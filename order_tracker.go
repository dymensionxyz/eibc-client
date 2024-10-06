package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pkg/errors"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	tmtypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"

	"github.com/dymensionxyz/eibc-client/store"
)

type orderTracker struct {
	hubClient       rpcclient.Client
	fullNodeClients []rpcclient.Client
	store           botStore
	logger          *zap.Logger
	bots            map[string]*orderFulfiller

	fomu            sync.Mutex
	fulfilledOrders map[string]struct{}
	validOrdersCh   chan []*demandOrder
	outputOrdersCh  chan<- []*demandOrder

	pool orderPool

	batchSize         int
	fulfillCriteria   *fulfillCriteria
	fulfilledOrdersCh chan *orderBatch
	subscriberID      string
}

// TODO: implement a store syncer to sync the store with the state of the order tracker

type botStore interface {
	GetOrders(ctx context.Context, opts ...store.OrderOption) ([]*store.Order, error)
	GetOrder(ctx context.Context, id string) (*store.Order, error)
	SaveManyOrders(ctx context.Context, orders []*store.Order) error
	UpdateManyOrders(ctx context.Context, orders []*store.Order) error
	DeleteOrder(ctx context.Context, id string) error
	GetBot(ctx context.Context, key string, opts ...store.BotOption) (*store.Bot, error)
	GetBots(ctx context.Context, opts ...store.BotOption) ([]*store.Bot, error)
	SaveBot(ctx context.Context, bot *store.Bot) error
	Close()
}

func newOrderTracker(
	hubClient rpcclient.Client,
	fullNodeClients []rpcclient.Client,
	store botStore,
	fulfilledOrdersCh chan *orderBatch,
	bots map[string]*orderFulfiller,
	subscriberID string,
	batchSize int,
	fCriteria *fulfillCriteria,
	ordersCh chan<- []*demandOrder,
	logger *zap.Logger,
) *orderTracker {
	return &orderTracker{
		hubClient:         hubClient,
		fullNodeClients:   fullNodeClients,
		store:             store,
		pool:              orderPool{orders: make(map[string]*demandOrder)},
		fulfilledOrdersCh: fulfilledOrdersCh,
		bots:              bots,
		batchSize:         batchSize,
		fulfillCriteria:   fCriteria,
		validOrdersCh:     make(chan []*demandOrder),
		outputOrdersCh:    ordersCh,
		logger:            logger.With(zap.String("module", "order-resolver")),
		subscriberID:      subscriberID,
		fulfilledOrders:   make(map[string]struct{}),
	}
}

func (or *orderTracker) start(ctx context.Context) error {
	if err := or.loadTrackedOrders(ctx); err != nil {
		return fmt.Errorf("failed to load orders: %w", err)
	}

	// TODO: consider that if the client is offline if might miss finalized orders, and the state might not be updated
	if err := or.waitForFinalizedOrder(ctx); err != nil {
		or.logger.Error("failed to wait for finalized order", zap.Error(err))
	}

	or.selectOrdersWorker(ctx)

	// go or.syncStore(ctx) TODO: do we really need all the orders in the store before fulfilling?
	go or.fulfilledOrdersWorker(ctx)

	return nil
}

// Demand orders are first added:
// - in sequencer mode, the first batch is sent to the output channel, and the rest of the orders are added to the pool
// - in p2p and settlement mode, all orders are added to the pool
// Then, the orders are periodically popped (fetched and deleted) from the pool and checked for validity.
// If the order is valid, it is sent to the output channel.
// If the order is not valid, it is added back to the pool.
// After the order validity deadline is expired, the order is removed permanently.
// Once an order is fulfilled, it is removed from the store
func (or *orderTracker) selectOrdersWorker(ctx context.Context) {
	toCheckOrdersCh := make(chan []*demandOrder, or.batchSize)
	go or.pullOrders(ctx, toCheckOrdersCh)
	go or.enqueueValidOrders(ctx, toCheckOrdersCh)
}

func (or *orderTracker) pullOrders(ctx context.Context, toCheckOrdersCh chan []*demandOrder) {
	ticker := time.NewTicker(500 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			orders := or.pool.popOrders(or.batchSize)
			if len(orders) == 0 {
				continue
			}
			// in "sequencer" mode send the orders directly to be fulfilled,
			// in other modes, send the orders to be checked for validity
			if or.fulfillCriteria.FulfillmentMode.Level == fulfillmentModeSequencer {
				or.outputOrdersCh <- orders
			} else {
				toCheckOrdersCh <- orders
			}
		}
	}
}

func (or *orderTracker) enqueueValidOrders(ctx context.Context, toCheckOrdersCh <-chan []*demandOrder) {
	for {
		select {
		case <-ctx.Done():
			return
		case orders := <-toCheckOrdersCh:
			validOrders, retryOrders := or.getValidAndRetryOrders(ctx, orders)
			if len(validOrders) > 0 {
				or.outputOrdersCh <- validOrders
			}
			if len(retryOrders) > 0 {
				or.pool.addOrder(retryOrders...)
			}
		}
	}
}

func (or *orderTracker) getValidAndRetryOrders(ctx context.Context, orders []*demandOrder) (validOrders, invalidOrders []*demandOrder) {
	for _, order := range orders {
		valid, err := or.isOrderValid(ctx, order)
		if err != nil {
			or.logger.Error("failed to check validation of block", zap.Error(err))
			continue
		}
		if valid {
			validOrders = append(validOrders, order)
			continue
		}
		if or.isOrderExpired(order) {
			or.logger.Debug("order has expired", zap.String("id", order.id))
			// order has expired, so delete it from the store (it's already deleted from the pool at this point)
			if err := or.store.DeleteOrder(ctx, order.id); err != nil {
				or.logger.Error("failed to delete order", zap.Error(err))
			}
			continue
		}
		or.logger.Debug("order is not valid yet", zap.String("id", order.id))
		// order is not valid yet, so add it back to the pool
		invalidOrders = append(invalidOrders, order)
	}
	return
}

func (or *orderTracker) isOrderValid(ctx context.Context, order *demandOrder) (valid bool, _ error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var numValidated int
	for _, node := range or.fullNodeClients {
		block, err := node.Block(ctx, &order.blockHeight)
		if err != nil || block == nil {
			// TODO: check for proper error
			if errors.Is(err, context.DeadlineExceeded) {
				return
			}
			or.logger.Error("failed to get block", zap.Error(err))
			continue
		}

		withSettlement := or.fulfillCriteria.FulfillmentMode.Level == fulfillmentModeSettlement
		settlementValidated := true // block.Block.SettlementValidated // TODO: implement

		if withSettlement {
			if !settlementValidated {
				continue
			}
		}
		numValidated++
	}

	valid = numValidated >= or.fulfillCriteria.FulfillmentMode.MinConfirmations
	return
}

func (or *orderTracker) isOrderExpired(order *demandOrder) bool {
	return time.Now().After(order.validDeadline)
}

func (or *orderTracker) addOrder(orders ...*demandOrder) {
	// - in mode "sequencer" we send a batch directly to be fulfilled,
	// and any orders that overflow the batch are added to the pool
	// - in mode "p2p" and "settlement" all orders are added to the pool
	if or.fulfillCriteria.FulfillmentMode.Level == fulfillmentModeSequencer {
		var (
			batchToSend []*demandOrder
			batchToPool []*demandOrder
		)
		// send one batch to fulfilling, add the rest to the pool
		for _, o := range orders {
			if len(batchToSend) < or.batchSize {
				batchToSend = append(batchToSend, o)
				continue
			}
			batchToPool = append(batchToPool, o)
		}
		or.outputOrdersCh <- batchToSend
		orders = batchToPool
	}
	or.pool.addOrder(orders...)
}

// sync the store with the pool every 30 seconds
func (or *orderTracker) syncStore(ctx context.Context) {
	for range time.NewTicker(time.Second * 30).C {
		poolOrders := or.pool.getOrders()
		storeOrders, err := or.store.GetOrders(ctx)
		if err != nil {
			or.logger.Error("failed to get orders", zap.Error(err))
			continue
		}
		storeGotOrders := make(map[string]struct{}, len(storeOrders))
		for _, o := range storeOrders {
			storeGotOrders[o.ID] = struct{}{}
		}

		toSaveOrders := make([]*store.Order, len(poolOrders))

		for i, o := range poolOrders {
			if _, ok := storeGotOrders[o.id]; ok {
				continue
			}
			toSaveOrders[i] = &store.Order{
				ID:            o.id,
				Amount:        o.amount.String(),
				Fee:           o.feeStr,
				RollappID:     o.rollappId,
				BlockHeight:   o.blockHeight,
				Status:        store.OrderStatusFulfilling, // always fulfilling status in the pool
				ValidDeadline: o.validDeadline.Unix(),
			}
		}
		if err := or.store.SaveManyOrders(ctx, toSaveOrders); err != nil {
			or.logger.Error("failed to save orders", zap.Error(err))
		}
	}
}

// upon startup, load the orders from the store: pending orders and fulfilled orders
func (or *orderTracker) loadTrackedOrders(ctx context.Context) error {
	// load fulfilled orders from the database
	fulfilledOrders, err := or.store.GetOrders(ctx, store.FilterByStatus(store.OrderStatusPendingFinalization))
	if err != nil {
		return fmt.Errorf("failed to get fulfilled orders: %w", err)
	}

	var (
		countFulfilled int
		countPending   int
	)

	or.fomu.Lock()
	for _, order := range fulfilledOrders {
		or.fulfilledOrders[order.ID] = struct{}{}
		countFulfilled++
	}
	or.fomu.Unlock()

	// load order pending fulfillment from the database
	pendingOrders, err := or.store.GetOrders(ctx, store.FilterByStatus(store.OrderStatusFulfilling))
	if err != nil {
		return fmt.Errorf("failed to get pending orders: %w", err)
	}

	var orders []*demandOrder
	for _, order := range pendingOrders {
		orders = append(orders, &demandOrder{
			id:            order.ID,
			feeStr:        order.Fee,
			rollappId:     order.RollappID,
			status:        string(order.Status),
			blockHeight:   order.BlockHeight,
			validDeadline: time.Unix(order.ValidDeadline, 0),
		})
		countPending++
	}

	or.pool.addOrder(orders...)

	or.logger.Info("loaded tracked orders", zap.Int("count-pending", countPending), zap.Int("count-fulfilled", countFulfilled))

	return nil
}

func (or *orderTracker) fulfilledOrdersWorker(ctx context.Context) {
	for {
		select {
		case batch := <-or.fulfilledOrdersCh:
			if err := or.addFulfilledOrders(ctx, batch); err != nil {
				or.logger.Error("failed to add fulfilled orders", zap.Error(err))
			}
		case <-ctx.Done():
			return
		}
	}
}

// addFulfilledOrders adds the fulfilled orders to the fulfilledOrders cache, and removes them from the orderPool.
// It also persists the state to the database.
func (or *orderTracker) addFulfilledOrders(ctx context.Context, batch *orderBatch) error {
	storeOrders := make([]*store.Order, len(batch.orders))
	or.fomu.Lock()
	for i, order := range batch.orders {
		if len(order.amount) == 0 {
			continue
		}
		// add to cache
		or.fulfilledOrders[order.id] = struct{}{}
		or.pool.removeOrder(order.id) // just in case it's still in the pool

		storeOrders[i] = &store.Order{
			ID:            order.id,
			Fulfiller:     batch.fulfiller,
			Amount:        order.amount[0].String(),
			Fee:           order.feeStr,
			RollappID:     order.rollappId,
			BlockHeight:   order.blockHeight,
			Status:        store.OrderStatusPendingFinalization,
			ValidDeadline: order.validDeadline.Unix(),
		}
	}
	or.fomu.Unlock()

	if err := or.store.SaveManyOrders(ctx, storeOrders); err != nil {
		return fmt.Errorf("failed to save orders: %w", err)
	}

	return nil
}

func (or *orderTracker) canFulfillOrder(order *demandOrder) bool {
	if or.isOrderFulfilled(order.id) {
		return false
	}
	// we are already processing this order
	if or.isOrderInPool(order.id) {
		return false
	}

	if !or.checkFeePercentage(order) {
		return false
	}

	return true
}

func (or *orderTracker) checkFeePercentage(order *demandOrder) bool {
	assetMinPercentage, ok := or.fulfillCriteria.MinFeePercentage.Asset[strings.ToLower(order.denom)]
	if !ok {
		return false
	}

	chainMinPercentage, ok := or.fulfillCriteria.MinFeePercentage.Chain[order.rollappId]
	if !ok {
		return false
	}

	feePercentage := order.feePercentage()
	okFee := feePercentage >= assetMinPercentage && feePercentage >= chainMinPercentage
	return okFee
}

func (or *orderTracker) isOrderFulfilled(id string) bool {
	or.fomu.Lock()
	defer or.fomu.Unlock()

	_, ok := or.fulfilledOrders[id]
	return ok
}

func (or *orderTracker) isOrderInPool(id string) bool {
	return or.pool.hasOrder(id)
}

const finalizedEvent = "dymensionxyz.dymension.eibc.EventDemandOrderPacketStatusUpdated"

func (or *orderTracker) waitForFinalizedOrder(ctx context.Context) error {
	// TODO: should filter by fulfiller (one of the bots)?
	query := fmt.Sprintf("%s.is_fulfilled='true' AND %s.new_packet_status='FINALIZED'", finalizedEvent, finalizedEvent)

	resCh, err := or.hubClient.Subscribe(ctx, or.subscriberID, query)
	if err != nil {
		return fmt.Errorf("failed to subscribe to demand orders: %w", err)
	}

	go func() {
		for {
			select {
			case res := <-resCh:
				if err := or.finalizeOrders(ctx, res); err != nil {
					or.logger.Error("failed to finalize order", zap.Error(err))
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// finalizeOrders finalizes the orders after the dispute period has passed. It updates the bot's balances and pending rewards.
func (or *orderTracker) finalizeOrders(ctx context.Context, res tmtypes.ResultEvent) error {
	ids := res.Events[finalizedEvent+".order_id"]

	if len(ids) == 0 {
		return nil
	}

	for _, id := range ids {
		if err := or.finalizeOrderWithID(ctx, id); err != nil {
			return fmt.Errorf("failed to finalize order with id %s: %w", id, err)
		}
	}

	return nil
}

func (or *orderTracker) finalizeOrderWithID(ctx context.Context, id string) error {
	or.fomu.Lock()
	defer or.fomu.Unlock()

	_, ok := or.fulfilledOrders[id]
	if !ok {
		return nil
	}

	order, err := or.store.GetOrder(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get order: %w", err)
	}

	b, err := or.store.GetBot(ctx, order.Fulfiller)
	if err != nil {
		return fmt.Errorf("failed to get bot: %w", err)
	}

	orderAmount, err := sdk.ParseCoinNormalized(order.Amount)
	if err != nil {
		return fmt.Errorf("failed to parse order amount: %w", err)
	}

	pendingRewards, err := sdk.ParseCoinsNormalized(strings.Join(b.PendingRewards, ","))
	if err != nil {
		return fmt.Errorf("failed to parse pending rewards: %w", err)
	}

	balances, err := sdk.ParseCoinsNormalized(strings.Join(b.Balances, ","))
	if err != nil {
		return fmt.Errorf("failed to parse balances: %w", err)
	}

	if pendingRewards.IsAnyGTE(sdk.NewCoins(orderAmount)) {
		pendingRewards = pendingRewards.Sub(orderAmount)
		balances = balances.Add(orderAmount)
		b.PendingRewards = store.CoinsToStrings(pendingRewards)
		b.Balances = store.CoinsToStrings(balances)
		or.bots[b.Name].accountSvc.setBalances(balances)
	}

	if err := or.store.SaveBot(ctx, b); err != nil {
		return fmt.Errorf("failed to update bot: %w", err)
	}

	if err := or.store.DeleteOrder(ctx, id); err != nil {
		return fmt.Errorf("failed to delete order: %w", err)
	}

	delete(or.fulfilledOrders, id)

	or.logger.Info("finalized order", zap.String("id", id))

	return nil
}

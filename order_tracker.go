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
	client rpcclient.Client
	store  botStore
	logger *zap.Logger

	fomu            sync.Mutex
	fulfilledOrders map[string]struct{}
	ordersCh        chan<- []*demandOrder

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
	DeleteOrder(ctx context.Context, id string) error
	GetBot(ctx context.Context, key string, opts ...store.BotOption) (*store.Bot, error)
	GetBots(ctx context.Context, opts ...store.BotOption) ([]*store.Bot, error)
	SaveBot(ctx context.Context, bot *store.Bot) error
	Close()
}

func newOrderTracker(
	client rpcclient.Client,
	store botStore,
	fulfilledOrdersCh chan *orderBatch,
	subscriberID string,
	batchSize int,
	fCriteria *fulfillCriteria,
	ordersCh chan<- []*demandOrder,
	logger *zap.Logger,
) *orderTracker {
	return &orderTracker{
		client:            client,
		store:             store,
		pool:              orderPool{orders: make(map[string]*demandOrder)},
		fulfilledOrdersCh: fulfilledOrdersCh,
		batchSize:         batchSize,
		fulfillCriteria:   fCriteria,
		ordersCh:          ordersCh,
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

	go or.fulfilledOrdersWorker(ctx)

	return nil
}

const (
	numWorkers = 5
	batchTime  = 2 * time.Second
)

func (or *orderTracker) selectOrdersWorker(ctx context.Context) {
	validOrdersCh := make(chan *demandOrder, or.batchSize)
	toCheckOrdersCh := make(chan *demandOrder, or.batchSize)

	for i := 0; i < numWorkers; i++ {
		go or.enqueueValidOrder(ctx, toCheckOrdersCh, validOrdersCh)
	}

	go or.pullOrders(toCheckOrdersCh)
	go or.pushValidOrders(validOrdersCh)
}

func (or *orderTracker) pullOrders(toCheckOrdersCh chan *demandOrder) {
	var ticker *time.Ticker
	if or.fulfillCriteria.FulfillmentMode.Level == fulfillmentModeSequencer {
		ticker = time.NewTicker(1 * time.Second)
	} else {
		ticker = time.NewTicker(5 * time.Second)
	}
	for {
		select {
		case <-ticker.C:
			// TODO: orders will be removed from the pool, what happens if fulfillment fails/succeeds?
			// pop a batch of orders from the pool
			// and send them to the order channel
			// channel will block until orders are processed
			// after unblocking, more orders will be popped from the pool
			for _, order := range or.pool.popOrders(or.batchSize) {
				toCheckOrdersCh <- order
			}
		}
	}
}

func (or *orderTracker) pushValidOrders(validOrdersCh <-chan *demandOrder) {
	batch := make([]*demandOrder, 0, or.batchSize)
	timer := time.NewTimer(batchTime)

	for {
		select {
		case order := <-validOrdersCh:
			batch = append(batch, order)
			if len(batch) >= or.batchSize {
				or.ordersCh <- batch
				batch = nil
			}
			timer.Reset(batchTime)
		case <-timer.C:
			if len(batch) > 0 {
				or.ordersCh <- batch
				batch = nil
			}
		}
	}
}

func (or *orderTracker) enqueueValidOrder(ctx context.Context, toCheckOrdersCh <-chan *demandOrder, validOrderCh chan<- *demandOrder) {
	for {
		select {
		case <-ctx.Done():
			return
		case order := <-toCheckOrdersCh:
			valid, err := or.isOrderValid(ctx, order)
			if err != nil {
				or.logger.Error("failed to check validation of block", zap.Error(err))
				return
			}

			if !valid {
				or.logger.Debug("order is not valid yet", zap.String("id", order.id))
				// order is not valid yet, so add it back to the pool
				or.addOrderToPool(order)
				return
			}
			validOrderCh <- order
		}
	}
}

func (or *orderTracker) isOrderValid(ctx context.Context, order *demandOrder) (valid bool, _ error) {
	// for SequencerFulfillment mode, we don't need to check if the block was validated
	if or.fulfillCriteria.FulfillmentMode.Level == fulfillmentModeSequencer {
		return true, nil
	}

	defer func() {
		if !valid && order.validDeadline.After(time.Now()) {
			or.logger.Debug("order deadline has passed", zap.String("id", order.id))
			// order is likely fraudulent, delete it
			if err := or.store.DeleteOrder(ctx, order.id); err != nil {
				or.logger.Error("failed to delete order", zap.Error(err))
			}
			return
		}
	}()

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_ /*block*/, err := or.client.Block(ctx, &order.blockHeight)
	if err != nil {
		// TODO: check for proper error
		if !errors.Is(err, context.DeadlineExceeded) {
			return
		}
		return false, fmt.Errorf("failed to get block: %w", err)
	}

	withSettlement := or.fulfillCriteria.FulfillmentMode.Level == fulfillmentModeSettlement
	settlementValidated := true // block.Block.SettlementValidated // TODO: implement

	if withSettlement {
		valid = settlementValidated
	}

	valid = true
	return
}

// TODO: cache every new order, but for P2P and Settlement, also save to DB
// when fulfilled, remove from fulfilledOrders and DB

func (or *orderTracker) syncStore() {
	// TODO
}

func (or *orderTracker) addOrderToPool(order ...*demandOrder) {
	or.pool.addOrder(order...)
}

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
			feeStr:        "",
			rollappId:     "",
			status:        "",
			blockHeight:   0,
			validDeadline: time.Time{},
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
		or.pool.removeOrder(order.id)

		storeOrders[i] = &store.Order{
			ID:            order.id,
			Fulfiller:     batch.fulfiller,
			Amount:        order.amount[0].String(),
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

	resCh, err := or.client.Subscribe(ctx, or.subscriberID, query)
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

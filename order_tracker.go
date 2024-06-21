package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/dymensionxyz/cosmosclient/cosmosclient"
	tmtypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"

	"github.com/dymensionxyz/eibc-client/store"
)

type orderTracker struct {
	client cosmosclient.Client
	store  botStore
	logger *zap.Logger

	comu              sync.Mutex
	currentOrders     map[string]struct{}
	tomu              sync.Mutex
	trackedOrders     map[string]struct{}
	denomsWhitelist   map[string]struct{}
	fulfilledOrdersCh chan *orderBatch
}

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
	client cosmosclient.Client,
	store botStore,
	fulfilledOrdersCh chan *orderBatch,
	denomsWhitelist map[string]struct{},
	logger *zap.Logger,
) *orderTracker {
	return &orderTracker{
		client:            client,
		store:             store,
		currentOrders:     make(map[string]struct{}),
		fulfilledOrdersCh: fulfilledOrdersCh,
		denomsWhitelist:   denomsWhitelist,
		logger:            logger.With(zap.String("module", "order-resolver")),
		trackedOrders:     make(map[string]struct{}),
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

	go func() {
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
	}()

	return nil
}

func (or *orderTracker) loadTrackedOrders(ctx context.Context) error {
	// load tracked orders from the database
	orders, err := or.store.GetOrders(ctx, store.FilterByStatus(store.OrderStatusPending))
	if err != nil {
		return fmt.Errorf("failed to get pending orders: %w", err)
	}

	or.tomu.Lock()
	for _, order := range orders {
		or.trackedOrders[order.ID] = struct{}{}
	}
	or.tomu.Unlock()
	or.logger.Info("loaded tracked orders", zap.Int("count", len(or.trackedOrders)))

	return nil
}

func (or *orderTracker) addFulfilledOrders(ctx context.Context, batch *orderBatch) error {
	storeOrders := make([]*store.Order, len(batch.orders))
	or.tomu.Lock()
	or.comu.Lock()
	for i, order := range batch.orders {
		if len(order.amount) == 0 {
			continue
		}
		// add to cache
		or.trackedOrders[order.id] = struct{}{}
		delete(or.currentOrders, order.id)

		storeOrders[i] = &store.Order{
			ID:        order.id,
			Fulfiller: batch.fulfiller,
			Amount:    order.amount[0].String(),
			Status:    store.OrderStatusPending,
		}
	}
	or.tomu.Unlock()
	or.comu.Unlock()

	if err := or.store.SaveManyOrders(ctx, storeOrders); err != nil {
		return fmt.Errorf("failed to save orders: %w", err)
	}

	return nil
}

func (or *orderTracker) canFulfillOrder(id, denom string) bool {
	// exclude orders whose denoms are not in the whitelist
	if _, found := or.denomsWhitelist[strings.ToLower(denom)]; !found {
		return false
	}

	if or.isOrderFulfilled(id) {
		return false
	}

	if or.isOrderCurrent(id) {
		return false
	}

	return true
}

func (or *orderTracker) isOrderFulfilled(id string) bool {
	or.tomu.Lock()
	defer or.tomu.Unlock()

	_, ok := or.trackedOrders[id]
	return ok
}

func (or *orderTracker) isOrderCurrent(id string) bool {
	or.comu.Lock()
	defer or.comu.Unlock()

	_, ok := or.currentOrders[id]
	if !ok {
		or.currentOrders[id] = struct{}{}
	}
	return ok
}

func (or *orderTracker) waitForFinalizedOrder(ctx context.Context) error {
	const query = "eibc.is_fulfilled='true' AND eibc.packet_status='FINALIZED'"

	resCh, err := or.client.RPC.Subscribe(ctx, "", query)
	if err != nil {
		return fmt.Errorf("failed to subscribe to demand orders: %w", err)
	}

	go func() {
		for {
			select {
			case res := <-resCh:
				if err := or.finalizeOrder(ctx, res); err != nil {
					or.logger.Error("failed to finalize order", zap.Error(err))
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (or *orderTracker) finalizeOrder(ctx context.Context, res tmtypes.ResultEvent) error {
	ids := res.Events["eibc.id"]

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
	or.tomu.Lock()
	defer or.tomu.Unlock()

	_, ok := or.trackedOrders[id]
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

	delete(or.trackedOrders, id)

	or.logger.Info("finalized order", zap.String("id", id))

	return nil
}

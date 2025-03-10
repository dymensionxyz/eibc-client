package eibc

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	tmtypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"
)

type orderEventer struct {
	rpc                           rpcclient.Client
	subscribedStateUpdateRollapps map[string]struct{}
	eventClient                   rpcclient.EventsClient
	logger                        *zap.Logger

	orderTracker *orderTracker
	subscriberID string
}

func newOrderEventer(
	client cosmosclient.Client,
	subscriberID string,
	orderTracker *orderTracker,
	logger *zap.Logger,
) *orderEventer {
	return &orderEventer{
		rpc:                           client.RPC,
		eventClient:                   client.WSEvents,
		subscriberID:                  subscriberID,
		subscribedStateUpdateRollapps: make(map[string]struct{}),
		logger:                        logger.With(zap.String("module", "order-eventer")),
		orderTracker:                  orderTracker,
	}
}

const (
	createdEvent    = "dymensionxyz.dymension.eibc.EventDemandOrderCreated"
	updatedFeeEvent = "dymensionxyz.dymension.eibc.EventDemandOrderFeeUpdated"
)

func (e *orderEventer) start(ctx context.Context) error {
	if err := e.rpc.Start(); err != nil {
		return fmt.Errorf("start rpc client: %w", err)
	}

	if err := e.subscribeToPendingDemandOrders(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to pending demand orders: %w", err)
	}

	if err := e.subscribeToUpdatedDemandOrders(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to updated demand orders: %w", err)
	}

	return nil
}

func (e *orderEventer) enqueueEventOrders(_ context.Context, eventName string, res tmtypes.ResultEvent) error {
	orders := e.parseOrdersFromEvents(eventName, res)
	if len(orders) == 0 {
		return nil
	}

	d := "updated"
	if eventName == createdEvent {
		d = "new"
	}
	e.orderTracker.trackOrders(orders...)

	rollaps := make([]string, 0, len(orders))
	ids := make([]string, 0, len(orders))
	for _, order := range orders {
		ids = append(ids, order.id)
		if slices.Contains(rollaps, order.rollappId) {
			continue
		}
		rollaps = append(rollaps, order.rollappId)
	}
	e.logger.Info(fmt.Sprintf("%s demand orders", d), zap.Strings("ids", ids), zap.Strings("rollapps", rollaps))

	return nil
}

func (e *orderEventer) parseOrdersFromEvents(eventName string, res tmtypes.ResultEvent) []*demandOrder {
	ids := res.Events[eventName+".order_id"]

	if len(ids) == 0 {
		return nil
	}

	prices := res.Events[eventName+".price"]
	amounts := res.Events[eventName+".amount"]
	fees := res.Events[eventName+".new_fee"]
	if eventName == createdEvent {
		fees = res.Events[eventName+".fee"]
	}
	rollapps := res.Events[eventName+".rollapp_id"]
	proofHeights := res.Events[eventName+".proof_height"]
	newOrders := make([]*demandOrder, 0, len(ids))

	for i, id := range ids {
		price, err := sdk.ParseCoinsNormalized(prices[i])
		if err != nil {
			e.logger.Error("failed to parse price", zap.Error(err))
			continue
		}

		if fees[i] == "" {
			continue
		}

		amount, ok := sdk.NewIntFromString(amounts[i])
		if !ok {
			e.logger.Error("failed to parse amount", zap.String("amount", amounts[i]))
			continue
		}

		fee, err := sdk.ParseCoinNormalized(fees[i])
		if err != nil {
			e.logger.Error("failed to parse fee", zap.Error(err))
			continue
		}

		proofHeight, err := strconv.ParseInt(proofHeights[i], 10, 64)
		if err != nil {
			e.logger.Error("failed to parse proof height", zap.Error(err))
			continue
		}

		if eventName == updatedFeeEvent {
			existOrder, ok := e.orderTracker.pool.getOrder(id)
			if ok {
				// update the fee and price of the order
				existOrder.fee = fee
				existOrder.price = price
				e.orderTracker.pool.upsertOrder(existOrder)
				continue
			}
		}

		order := &demandOrder{
			id:            id,
			denom:         fee.Denom,
			price:         price,
			amount:        amount,
			fee:           fee,
			rollappId:     rollapps[i],
			proofHeight:   proofHeight,
			validDeadline: time.Now().Add(e.orderTracker.validation.WaitTime),
			from:          "event",
		}

		if !e.orderTracker.canFulfillOrder(order) {
			continue
		}

		if err := e.orderTracker.findLPForOrder(order); err != nil {
			e.logger.Debug("failed to find LP for order", zap.Error(err), zap.String("order_id", order.id))
			continue
		}

		newOrders = append(newOrders, order)
	}

	return newOrders
}

func (e *orderEventer) subscribeToPendingDemandOrders(ctx context.Context) error {
	query := fmt.Sprintf("%s.packet_status='PENDING'", createdEvent)
	return e.subscribeToEvent(ctx, createdEvent, query, e.enqueueEventOrders)
}

func (e *orderEventer) subscribeToUpdatedDemandOrders(ctx context.Context) error {
	query := fmt.Sprintf("%s.packet_status='PENDING'", updatedFeeEvent)
	return e.subscribeToEvent(ctx, updatedFeeEvent, query, e.enqueueEventOrders)
}

func (e *orderEventer) subscribeToEvent(ctx context.Context, event string, query string, callback func(ctx context.Context, name string, event tmtypes.ResultEvent) error) error {
	resCh, err := e.eventClient.Subscribe(ctx, e.subscriberID, query)
	if err != nil {
		return fmt.Errorf("failed to subscribe to %s events: %w", event, err)
	}

	go func() {
		for {
			select {
			case res := <-resCh:
				if err := callback(ctx, event, res); err != nil {
					e.logger.Error(fmt.Sprintf("failed to process %s event", event), zap.Error(err))
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

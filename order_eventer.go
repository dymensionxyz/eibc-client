package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	tmtypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"
)

type orderEventer struct {
	client cosmosclient.Client
	logger *zap.Logger

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
		client:       client,
		subscriberID: subscriberID,
		logger:       logger.With(zap.String("module", "order-eventer")),
		orderTracker: orderTracker,
	}
}

func (e *orderEventer) start(ctx context.Context) error {
	if err := e.client.RPC.Start(); err != nil {
		return fmt.Errorf("start rpc client: %w", err)
	}

	if err := e.subscribeToPendingDemandOrders(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to pending demand orders: %w", err)
	}

	return nil
}

func (e *orderEventer) enqueueEventOrders(res tmtypes.ResultEvent) error {
	newOrders := e.parseOrdersFromEvents(res)
	if len(newOrders) == 0 {
		return nil
	}

	if e.logger.Level() <= zap.DebugLevel {
		ids := make([]string, 0, len(newOrders))
		for _, order := range newOrders {
			ids = append(ids, order.id)
		}
		e.logger.Debug("new demand orders", zap.Strings("ids", ids))
	} else {
		e.logger.Info("new demand orders", zap.Int("count", len(newOrders)))
	}

	e.orderTracker.addOrderToPool(newOrders...)

	/*batch := make([]*demandOrder, 0, e.batchSize)

	for _, order := range newOrders {
		batch = append(batch, order)

		if len(batch) >= e.batchSize || len(batch) == len(newOrders) {
			e.newOrders <- batch
			batch = make([]*demandOrder, 0, e.batchSize)
		}
	}

	if len(batch) == 0 {
		return nil
	}*/

	return nil
}

const createdEvent = "dymensionxyz.dymension.eibc.EventDemandOrderCreated"

func (e *orderEventer) parseOrdersFromEvents(res tmtypes.ResultEvent) []*demandOrder {
	ids := res.Events[createdEvent+".order_id"]

	if len(ids) == 0 {
		return nil
	}

	prices := res.Events[createdEvent+".price"]
	fees := res.Events[createdEvent+".fee"]
	statuses := res.Events[createdEvent+".packet_status"]
	rollapps := res.Events[createdEvent+".rollapp_id"]
	heights := res.Events[createdEvent+".block_height"]
	newOrders := make([]*demandOrder, 0, len(ids))

	for i, id := range ids {
		price, err := sdk.ParseCoinsNormalized(prices[i])
		if err != nil {
			e.logger.Error("failed to parse price", zap.Error(err))
			continue
		}

		fee, err := sdk.ParseCoinsNormalized(fees[i])
		if err != nil {
			e.logger.Error("failed to parse fee", zap.Error(err))
			continue
		}

		height, err := strconv.ParseInt(heights[i], 10, 64)
		if err != nil {
			e.logger.Error("failed to parse block height", zap.Error(err))
			continue
		}

		validationWaitTime := e.orderTracker.fulfillCriteria.FulfillmentMode.ValidationWaitTime
		validDeadline := time.Now().Add(validationWaitTime)

		order := &demandOrder{
			id:            id,
			denom:         fee.GetDenomByIndex(0),
			amount:        price,
			fee:           fee,
			status:        statuses[i],
			rollappId:     rollapps[i],
			blockHeight:   height,
			validDeadline: validDeadline,
		}

		if !e.orderTracker.canFulfillOrder(order) {
			continue
		}

		newOrders = append(newOrders, order)
	}

	return newOrders
}

func (e *orderEventer) subscribeToPendingDemandOrders(ctx context.Context) error {
	const query = createdEvent + ".is_fulfilled='false'"

	resCh, err := e.client.WSEvents.Subscribe(ctx, e.subscriberID, query)
	if err != nil {
		return fmt.Errorf("failed to subscribe to demand orders: %w", err)
	}

	go func() {
		for {
			select {
			case res := <-resCh:
				if err := e.enqueueEventOrders(res); err != nil {
					e.logger.Error("failed to enqueue event orders", zap.Error(err))
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

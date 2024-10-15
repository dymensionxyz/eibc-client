package eibc

import (
	"context"
	"fmt"
	"strconv"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	tmtypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"
)

type orderEventer struct {
	rpc         rpcclient.Client
	rollapps    []string
	eventClient rpcclient.EventsClient
	logger      *zap.Logger

	orderTracker *orderTracker
	subscriberID string
}

func newOrderEventer(
	client cosmosclient.Client,
	subscriberID string,
	rollapps []string,
	orderTracker *orderTracker,
	logger *zap.Logger,
) *orderEventer {
	return &orderEventer{
		rpc:          client.RPC,
		eventClient:  client.WSEvents,
		subscriberID: subscriberID,
		rollapps:     rollapps,
		logger:       logger.With(zap.String("module", "order-eventer")),
		orderTracker: orderTracker,
	}
}

const (
	createdEvent   = "dymensionxyz.dymension.eibc.EventDemandOrderCreated"
	finalizedEvent = "dymensionxyz.dymension.eibc.EventDemandOrderPacketStatusUpdated"
	stateInfoEvent = "state_update"
)

func (e *orderEventer) start(ctx context.Context) error {
	if err := e.rpc.Start(); err != nil {
		return fmt.Errorf("start rpc client: %w", err)
	}

	if err := e.subscribeToPendingDemandOrders(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to pending demand orders: %w", err)
	}

	// TODO: consider that if the client is offline if might miss finalized orders, and the state might not be updated
	if err := e.waitForFinalizedOrder(ctx); err != nil {
		return fmt.Errorf("failed to wait for finalized orders: %w", err)
	}

	for _, rollappID := range e.rollapps {
		if err := e.subscribeToStateUpdates(ctx, rollappID); err != nil {
			return fmt.Errorf("failed to subscribe to state updates: %w", err)
		}
	}

	return nil
}

func (e *orderEventer) enqueueEventOrders(_ context.Context, res tmtypes.ResultEvent) error {
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

	e.orderTracker.addOrder(newOrders...)

	return nil
}

func (e *orderEventer) parseOrdersFromEvents(res tmtypes.ResultEvent) []*demandOrder {
	ids := res.Events[createdEvent+".order_id"]

	if len(ids) == 0 {
		return nil
	}

	prices := res.Events[createdEvent+".price"]
	fees := res.Events[createdEvent+".fee"]
	statuses := res.Events[createdEvent+".packet_status"]
	packetKeys := res.Events[createdEvent+".packet_key"]
	rollapps := res.Events[createdEvent+".rollapp_id"]
	heights := res.Events[createdEvent+".proof_height"]
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
			packetKey:     packetKeys[i],
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
	query := fmt.Sprintf("%s.is_fulfilled='false'", createdEvent)
	return e.subscribeToEvent(ctx, "pending demand", query, e.enqueueEventOrders)
}

func (e *orderEventer) waitForFinalizedOrder(ctx context.Context) error {
	// TODO: should filter by fulfiller (one of the bots)?
	query := fmt.Sprintf("%s.is_fulfilled='true' AND %s.new_packet_status='FINALIZED'", finalizedEvent, finalizedEvent)
	return e.subscribeToEvent(ctx, "finalized", query, e.orderTracker.finalizeOrders)
}

func (e *orderEventer) subscribeToStateUpdates(ctx context.Context, rollappID string) error {
	query := fmt.Sprintf("%s.rollapp_id='%s'", stateInfoEvent, rollappID)
	return e.subscribeToEvent(ctx, "state update", query, e.orderTracker.checkEventFinalized)
}

func (e *orderEventer) subscribeToEvent(ctx context.Context, event string, query string, callback func(ctx context.Context, event tmtypes.ResultEvent) error) error {
	resCh, err := e.eventClient.Subscribe(ctx, e.subscriberID, query)
	if err != nil {
		return fmt.Errorf("failed to subscribe to %s events: %w", event, err)
	}

	go func() {
		for {
			select {
			case res := <-resCh:
				if err := callback(ctx, res); err != nil {
					e.logger.Error(fmt.Sprintf("failed to process %s event", event), zap.Error(err))
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

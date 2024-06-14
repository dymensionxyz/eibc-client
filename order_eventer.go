package main

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	tmtypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"
)

type orderEventer struct {
	client cosmosclient.Client
	logger *zap.Logger

	batchSize int
	newOrders chan []*demandOrder
	tracker   *orderTracker
}

func newOrderEventer(
	client cosmosclient.Client,
	tracker *orderTracker,
	batchSize int,
	newOrders chan []*demandOrder,
	logger *zap.Logger,
) *orderEventer {
	return &orderEventer{
		client:    client,
		batchSize: batchSize,
		logger:    logger.With(zap.String("module", "order-eventer")),
		newOrders: newOrders,
		tracker:   tracker,
	}
}

func (e *orderEventer) start(ctx context.Context) error {
	if err := e.subscribeToPendingDemandOrders(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to pending demand orders: %w", err)
	}

	return nil
}

func (e *orderEventer) enqueueEventOrders(res tmtypes.ResultEvent) error {
	newOrders, err := e.parseOrdersFromEvents(res)
	if err != nil {
		return fmt.Errorf("failed to parse orders from events: %w", err)
	}

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

	batch := make([]*demandOrder, 0, e.batchSize)

	for _, order := range newOrders {
		batch = append(batch, order)

		if len(batch) >= e.batchSize || len(batch) == len(newOrders) {
			e.newOrders <- batch
			batch = make([]*demandOrder, 0, e.batchSize)
		}
	}

	if len(batch) == 0 {
		return nil
	}

	return nil
}

func (e *orderEventer) parseOrdersFromEvents(res tmtypes.ResultEvent) ([]*demandOrder, error) {
	ids := res.Events["eibc.id"]

	if len(ids) == 0 {
		return nil, nil
	}

	prices := res.Events["eibc.price"]
	fees := res.Events["eibc.fee"]
	statuses := res.Events["eibc.packet_status"]
	newOrders := make([]*demandOrder, 0, len(ids))

	for i, id := range ids {
		price, err := sdk.ParseCoinNormalized(prices[i])
		if err != nil {
			return nil, fmt.Errorf("failed to parse price: %w", err)
		}

		if !e.tracker.canFulfillOrder(id, price.Denom) {
			continue
		}

		fee, err := sdk.ParseCoinNormalized(fees[i])
		if err != nil {
			return nil, fmt.Errorf("failed to parse fee: %w", err)
		}

		order := &demandOrder{
			id:     id,
			amount: sdk.NewCoins(price.Add(fee)),
			status: statuses[i],
		}
		newOrders = append(newOrders, order)
	}

	return newOrders, nil
}

func (e *orderEventer) subscribeToPendingDemandOrders(ctx context.Context) error {
	const query = "eibc.is_fulfilled='false'"

	resCh, err := e.client.RPC.Subscribe(ctx, "", query)
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

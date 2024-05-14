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

func (of *orderEventer) start(ctx context.Context) error {
	if err := of.subscribeToPendingDemandOrders(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to pending demand orders: %w", err)
	}

	return nil
}

func (of *orderEventer) enqueueEventOrders(res tmtypes.ResultEvent) error {
	newOrders, err := of.parseOrdersFromEvents(res)
	if err != nil {
		return fmt.Errorf("failed to parse orders from events: %w", err)
	}

	if of.logger.Level() <= zap.DebugLevel {
		ids := make([]string, 0, len(newOrders))
		for _, order := range newOrders {
			ids = append(ids, order.id)
		}
		of.logger.Debug("new demand orders", zap.Strings("count", ids))
	} else {
		of.logger.Info("new demand orders", zap.Int("count", len(newOrders)))
	}

	batch := make([]*demandOrder, 0, of.batchSize)

	for _, order := range newOrders {
		batch = append(batch, order)

		if len(batch) >= of.batchSize || len(batch) == len(newOrders) {
			of.newOrders <- batch
			batch = make([]*demandOrder, 0, of.batchSize)
		}
	}

	if len(batch) == 0 {
		return nil
	}

	return nil
}

func (of *orderEventer) parseOrdersFromEvents(res tmtypes.ResultEvent) ([]*demandOrder, error) {
	ids := res.Events["eibc.id"]

	if len(ids) == 0 {
		return nil, nil
	}

	prices := res.Events["eibc.price"]
	fees := res.Events["eibc.fee"]
	statuses := res.Events["eibc.packet_status"]
	newOrders := make([]*demandOrder, 0, len(ids))

	for i, id := range ids {
		if of.tracker.isOrderFulfilled(id) {
			continue
		}

		if of.tracker.isOrderCurrent(id) {
			continue
		}

		price, err := sdk.ParseCoinNormalized(prices[i])
		if err != nil {
			return nil, fmt.Errorf("failed to parse price: %w", err)
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

func (of *orderEventer) subscribeToPendingDemandOrders(ctx context.Context) error {
	const query = "eibc.is_fulfilled='false'"

	resCh, err := of.client.RPC.Subscribe(ctx, "", query)
	if err != nil {
		return fmt.Errorf("failed to subscribe to demand orders: %w", err)
	}

	go func() {
		for {
			select {
			case res := <-resCh:
				if err := of.enqueueEventOrders(res); err != nil {
					of.logger.Error("failed to enqueue event orders", zap.Error(err))
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

package main

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

type bot struct {
	name      string
	logger    *zap.Logger
	fulfiller *orderFulfiller
	newOrders chan []*demandOrder
	failedCh  chan<- []string
}

func newBot(
	name string,
	fulfiller *orderFulfiller,
	newOrders chan []*demandOrder,
	failedCh chan<- []string,
	logger *zap.Logger,
) *bot {
	return &bot{
		name:      name,
		fulfiller: fulfiller,
		newOrders: newOrders,
		failedCh:  failedCh,
		logger:    logger.With(zap.String("name", name)),
	}
}

func (b *bot) start(ctx context.Context) error {
	balances, err := b.fulfiller.accountSvc.getAccountBalances(ctx)
	if err != nil {
		return fmt.Errorf("failed to get account balances: %w", err)
	}
	b.logger.Info("starting bot...", zap.String("balances", balances.String()))

	// start order fulfiller
	b.fulfiller.fulfillOrders(ctx, b.newOrders, b.failedCh)
	return nil
}

package main

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

type bot struct {
	name   string
	logger *zap.Logger
	*orderFulfiller
	newOrders     chan []*demandOrder
	unfulfilledCh chan<- []string
}

func newBot(
	name string,
	fulfiller *orderFulfiller,
	failedCh chan<- []string,
	logger *zap.Logger,
) *bot {
	return &bot{
		name:           name,
		orderFulfiller: fulfiller,
		newOrders:      make(chan []*demandOrder),
		unfulfilledCh:  failedCh,
		logger:         logger.With(zap.String("name", name)),
	}
}

func (b *bot) start(ctx context.Context) error {
	if err := b.accountSvc.refreshBalances(ctx); err != nil {
		return fmt.Errorf("failed to get account balances: %w", err)
	}
	b.logger.Info("starting bot...", zap.String("balances", b.accountSvc.balances.String()))

	// start order fulfiller
	b.fulfillOrders(ctx, b.newOrders, b.unfulfilledCh)
	return nil
}

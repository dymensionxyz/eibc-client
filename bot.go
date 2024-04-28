package main

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

type bot struct {
	name         string
	logger       *zap.Logger
	fulfiller    *orderFulfiller
	newOrders    chan []*demandOrder
	doneOrderIDs chan<- []string
}

func newBot(
	name string,
	fulfiller *orderFulfiller,
	newOrders chan []*demandOrder,
	doneOrderIDs chan<- []string,
	logger *zap.Logger,
) *bot {
	return &bot{
		name:         name,
		fulfiller:    fulfiller,
		newOrders:    newOrders,
		doneOrderIDs: doneOrderIDs,
		logger:       logger.With(zap.String("name", name)),
	}
}

func (b *bot) start(ctx context.Context) error {
	b.logger.Info("starting bot...")

	// setup account
	if err := b.fulfiller.accountSvc.setupAccount(); err != nil { // TODO: wrap
		return fmt.Errorf("failed to setup account: %w", err)
	}

	// start order fulfiller
	b.fulfiller.fulfillOrders(ctx, b.newOrders, b.doneOrderIDs)
	return nil
}

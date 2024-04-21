package main

import (
	"context"
	"time"

	"go.uber.org/zap"
)

func (oc *orderClient) orderRefresher(ctx context.Context) {
	go oc.worker(ctx, "orderRefresher", oc.refreshInterval, func() bool {
		if err := oc.refreshPendingDemandOrders(ctx); err != nil {
			oc.logger.Error("failed to refresh demand orders", zap.Error(err))
		}
		return false // don't stop me now
	})
}

func (oc *orderClient) orderFulfiller(ctx context.Context) {
	go oc.worker(ctx, "orderFulfiller", oc.fulfillInterval, func() bool {
		if oc.denomSkipped("adym") { // TODO: const
			oc.logger.Info("DYM balance low, paused order fulfillments")
			return false
		}
		if err := oc.fulfillOrders(ctx); err != nil {
			oc.logger.Error("failed to fulfill orders", zap.Error(err))
		}
		return false // don't stop me now
	})
}

func (oc *orderClient) orderCleaner(ctx context.Context) {
	go oc.worker(ctx, "orderCleaner", oc.cleanupInterval, func() bool {
		if err := oc.cleanup(); err != nil {
			oc.logger.Error("failed to cleanup", zap.Error(err))
		}
		return false // don't stop me now
	})
}

func (oc *orderClient) disputePeriodUpdater(ctx context.Context) {
	go oc.worker(ctx, "disputePeriodUpdater", oc.disputePeriodInterval, func() bool {
		disputePeriod, err := oc.getDisputePeriodInBlocks(ctx)
		if err != nil {
			oc.logger.Error("failed to get dispute period", zap.Error(err))
			return false
		}
		// TODO: mutex lock
		if disputePeriod != oc.disputePeriod {
			oc.logger.Info("updating dispute period", zap.Uint64("blocks", disputePeriod))
			oc.disputePeriod = disputePeriod
		}
		return false // don't stop me now
	})
}

func (oc *orderClient) accountBalanceChecker(ctx context.Context) {
	go oc.worker(ctx, "accountBalanceChecker", oc.balanceCheckInterval, func() bool {
		if err := oc.checkBalances(ctx); err != nil {
			oc.logger.Error("failed to check balances", zap.Error(err))
		}

		return false // don't stop me now
	})
}

func (oc *orderClient) worker(ctx context.Context, name string, interval time.Duration, cb func() bool) {
	defer oc.logger.Info("stopping worker", zap.String("name", name))

	oc.logger.Info("starting worker", zap.String("name", name), zap.Duration("interval", interval))

	for c := time.Tick(interval); ; <-c {
		select {
		case <-ctx.Done():
			return
		default:
			if cb() {
				return
			}
		}
	}
}

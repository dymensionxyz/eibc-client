package main

import (
	"context"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/zap"
)

func (oc *orderClient) orderRefresher(ctx context.Context) {
	go oc.worker(ctx, "orderRefresher", oc.orderRefreshInterval, func() bool {
		if err := oc.refreshPendingDemandOrders(ctx); err != nil {
			oc.logger.Error("failed to refresh demand orders", zap.Error(err))
		}
		return false // don't stop me now
	})
}

func (oc *orderClient) orderFulfiller(ctx context.Context) {
	go oc.worker(ctx, "orderFulfiller", oc.orderFulfillInterval, func() bool {
		balance, err := oc.getAccountBalance(ctx, oc.account.GetAddress().String(), defaultGasDenom)
		if err != nil {
			oc.logger.Error("failed to get account balance", zap.Error(err))
			return false
		}

		minimumGasBalance, err := sdk.ParseCoinNormalized(oc.minimumGasBalance)
		if err != nil {
			oc.logger.Error("failed to parse minimum gas balance", zap.Error(err))
			return false
		}

		if balance.IsLT(minimumGasBalance) {
			oc.logger.Debug("gas balance low, paused order fulfillments")
			return false
		}

		if err := oc.fulfillOrders(ctx); err != nil {
			oc.logger.Error("failed to fulfill orders", zap.Error(err))
		}
		return false // don't stop me now
	})
}

func (oc *orderClient) orderCleaner(ctx context.Context) {
	go oc.worker(ctx, "orderCleaner", oc.orderCleanupInterval, func() bool {
		if err := oc.cleanup(); err != nil {
			oc.logger.Error("failed to cleanup", zap.Error(err))
		}
		return false // don't stop me now
	})
}

func (oc *orderClient) disputePeriodUpdater(ctx context.Context) {
	go oc.worker(ctx, "disputePeriodUpdater", oc.disputePeriodRefreshInterval, func() bool {
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

package main

import (
	"context"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/zap"
)

type whale struct {
	accountSvc *accountService
	logger     *zap.Logger
	topUpCh    <-chan topUpRequest
}

type topUpRequest struct {
	coins  sdk.Coins
	toAddr string
	res    chan []string
}

func newWhale(accountSvc *accountService, logger *zap.Logger, topUpCh <-chan topUpRequest) *whale {
	return &whale{
		accountSvc: accountSvc,
		logger:     logger.With(zap.String("module", "whale")),
		topUpCh:    topUpCh,
	}
}

func (w *whale) start(ctx context.Context) error {
	if err := w.accountSvc.setupAccount(); err != nil {
		return fmt.Errorf("failed to setup account: %w", err)
	}

	balances, err := w.accountSvc.getAccountBalances(ctx)
	if err != nil {
		return fmt.Errorf("failed to get account balances: %w", err)
	}

	w.logger.Info("starting service...",
		zap.String("account", w.accountSvc.accountName),
		zap.String("address", w.accountSvc.account.GetAddress().String()),
		zap.String("balances", balances.String()))

	go w.topUpBalances(ctx)
	return nil
}

func (w *whale) topUpBalances(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case req := <-w.topUpCh:
			toppedUp := w.topUp(ctx, req.coins, req.toAddr)
			req.res <- toppedUp
		}
	}
}

func (w *whale) topUp(ctx context.Context, coins sdk.Coins, toAddr string) []string {
	whaleBalances, err := w.accountSvc.getAccountBalances(ctx)
	if err != nil {
		w.logger.Error("failed to get account balances", zap.Error(err))
		return nil
	}

	canTopUp := sdk.NewCoins()
	for _, coin := range coins {
		balance := whaleBalances.AmountOf(coin.Denom)
		diff := balance.Sub(coin.Amount)
		if diff.IsPositive() {
			canTopUp = canTopUp.Add(coin)
		} else {
			w.logger.Info(
				"account doesn't have enough balance",
				zap.String("denom", coin.Denom),
				zap.String("balance", balance.String()),
				zap.String("required", coin.Amount.String()))
		}
	}

	if canTopUp.Empty() {
		w.logger.Info(
			"no denoms to top up",
			zap.String("to", toAddr),
			zap.String("coins", coins.String()))

		return nil
	}

	w.logger.Debug(
		"topping up account",
		zap.String("to", toAddr),
		zap.String("coins", canTopUp.String()),
	)

	if err = w.accountSvc.sendCoins(canTopUp, toAddr); err != nil {
		w.logger.Error("failed to top up account", zap.Error(err))
		return nil
	}

	toppedUp := make([]string, len(canTopUp))
	for i, coin := range canTopUp {
		toppedUp[i] = coin.Denom
	}

	return toppedUp
}

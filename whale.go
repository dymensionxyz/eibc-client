package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/zap"
)

type whale struct {
	accountSvc        *accountService
	logger            *zap.Logger
	topUpCh           <-chan topUpRequest
	balanceThresholds map[string]sdk.Coin
	admu              sync.Mutex
	alertedDenoms     map[string]struct{}
	slack             *slacker
	chainID, node     string
}

type topUpRequest struct {
	coins  sdk.Coins
	toAddr string
	res    chan []string
}

func newWhale(
	accountSvc *accountService,
	balanceThresholds map[string]sdk.Coin,
	logger *zap.Logger,
	slack *slacker,
	chainID, node string,
	topUpCh <-chan topUpRequest,
) *whale {
	return &whale{
		accountSvc:        accountSvc,
		logger:            logger.With(zap.String("module", "whale")),
		topUpCh:           topUpCh,
		slack:             slack,
		balanceThresholds: balanceThresholds,
		alertedDenoms:     make(map[string]struct{}),
		chainID:           chainID,
		node:              node,
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

		// if balance thresholds are defined,
		// we check if this coin is in the list
		threshold, ok := w.balanceThresholds[strings.ToLower(coin.Denom)]
		if !ok && len(w.balanceThresholds) > 0 {
			continue
		}

		diff := balance.Sub(coin.Amount)
		if diff.IsPositive() {
			// if the balance is greater than the required amount, remove the alert
			go w.removeAlerted(coin.Denom)

			canTopUp = canTopUp.Add(coin)
			whaleBalances = whaleBalances.Sub(coin)
			newBalance := whaleBalances.AmountOf(coin.Denom)

			// if balance thresholds are defined and the new balance is below the threshold,
			// we alert the user
			if len(w.balanceThresholds) > 0 && newBalance.LTE(threshold.Amount) {
				go w.alertLowBalance(ctx, coin, sdk.NewCoin(coin.Denom, newBalance))
			}
		} else {
			go w.alertLowBalance(ctx, coin, sdk.NewCoin(coin.Denom, balance))
		}
	}

	if canTopUp.Empty() {
		w.logger.Debug(
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

func (w *whale) alertLowBalance(ctx context.Context, coin, balance sdk.Coin) {
	w.admu.Lock()
	defer w.admu.Unlock()

	if _, ok := w.alertedDenoms[coin.Denom]; ok {
		return
	}
	w.alertedDenoms[coin.Denom] = struct{}{}

	w.logger.Warn(
		"account doesn't have enough balance",
		zap.String("balance", balance.String()),
		zap.String("required", coin.String()))

	_, err := w.slack.begOnSlack(
		ctx,
		w.accountSvc.account.GetAddress().String(),
		coin,
		balance,
		w.chainID,
		w.node,
	)
	if err != nil {
		w.logger.Error("failed to beg on slack", zap.Error(err))
	}
}

func (w *whale) removeAlerted(denom string) {
	w.admu.Lock()
	defer w.admu.Unlock()

	delete(w.alertedDenoms, denom)
}

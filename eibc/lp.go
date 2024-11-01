package eibc

import (
	"context"
	"sync"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/bank/types"
	"go.uber.org/zap"
)

type lp struct {
	address       string
	rollapps      map[string]rollappCriteria
	bmu           sync.Mutex
	balance       sdk.Coins
	reservedFunds sdk.Coins
}

type rollappCriteria struct {
	rollappID           string
	denoms              map[string]bool
	maxPrice            sdk.Coins
	minFeePercentage    sdk.Dec
	operatorFeeShare    sdk.Dec
	settlementValidated bool
}

func (l *lp) getBalance() sdk.Coins {
	l.bmu.Lock()
	defer l.bmu.Unlock()
	return l.balance
}

func (l *lp) hasBalance(amount sdk.Coins) bool {
	return l.spendableBalance().IsAllGTE(amount)
}

func (l *lp) setBalance(balance sdk.Coins) {
	l.bmu.Lock()
	defer l.bmu.Unlock()
	l.balance = balance
}

func (l *lp) reserveFunds(amount sdk.Coins) {
	l.bmu.Lock()
	defer l.bmu.Unlock()
	l.reservedFunds = l.reservedFunds.Add(amount...)
}

func (l *lp) releaseFunds(amount sdk.Coins) {
	l.bmu.Lock()
	defer l.bmu.Unlock()

	var fail bool
	l.reservedFunds, fail = l.reservedFunds.SafeSub(amount...)
	if fail {
		l.reservedFunds = sdk.NewCoins()
	}
}

func (l *lp) debitReservedFunds(amount sdk.Coins) {
	l.bmu.Lock()
	defer l.bmu.Unlock()
	var fail bool
	l.reservedFunds, fail = l.reservedFunds.SafeSub(amount...)
	if fail {
		l.reservedFunds = sdk.NewCoins()
	}

	l.balance, fail = l.balance.SafeSub(amount...)
	if fail {
		l.balance = sdk.NewCoins()
	}
}

func (l *lp) spendableBalance() sdk.Coins {
	l.bmu.Lock()
	defer l.bmu.Unlock()

	return l.balance.Sub(l.reservedFunds...)
}

func (or *orderTracker) releaseAllReservedOrdersFunds(demandOrder ...*demandOrder) {
	for _, order := range demandOrder {
		or.lpmu.Lock()
		if lp, ok := or.lps[order.lpAddress]; ok {
			lp.releaseFunds(order.amount)
		}
		or.lpmu.Unlock()
	}
}

func (or *orderTracker) debitAllReservedOrdersFunds(demandOrder ...*demandOrder) {
	for _, order := range demandOrder {
		or.lpmu.Lock()
		if lp, ok := or.lps[order.lpAddress]; ok {
			lp.debitReservedFunds(order.amount)
		}
		or.lpmu.Unlock()
	}
}

func (or *orderTracker) balanceRefresher(ctx context.Context) {
	t := time.NewTicker(or.balanceRefreshInterval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			or.refreshBalances(ctx)
		}
	}
}

func (or *orderTracker) refreshBalances(ctx context.Context) {
	for _, lp := range or.lps {
		resp, err := or.getBalances(ctx, &types.QuerySpendableBalancesRequest{
			Address: lp.address,
		})
		if err != nil {
			or.logger.Error("failed to get balances", zap.Error(err))
			continue
		}
		lp.setBalance(resp.Balances)
	}
}

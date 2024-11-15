package eibc

import (
	"context"
	"fmt"
	"sync"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/gogoproto/proto"
	"go.uber.org/zap"

	"github.com/dymensionxyz/eibc-client/types"
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

func (l *lp) hasBalance(amount sdk.Coins) bool {
	return l.spendableBalance().IsAllGTE(amount)
}

func (l *lp) getBalance() sdk.Coins {
	l.bmu.Lock()
	defer l.bmu.Unlock()
	return l.balance
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

func (or *orderTracker) loadLPs(ctx context.Context) error {
	grants, err := or.getLPGrants(ctx, &authz.QueryGranteeGrantsRequest{
		Grantee: or.policyAddress,
	})
	if err != nil {
		return fmt.Errorf("failed to get LP grants: %w", err)
	}

	or.lpmu.Lock()
	defer or.lpmu.Unlock()

	for _, grant := range grants.Grants {
		if grant.Authorization == nil {
			continue
		}

		g := new(types.FulfillOrderAuthorization)
		if err = proto.Unmarshal(grant.Authorization.Value, g); err != nil {
			return fmt.Errorf("failed to unmarshal grant: %w", err)
		}

		resp, err := or.getBalances(ctx, &banktypes.QuerySpendableBalancesRequest{
			Address: grant.Granter,
		})
		if err != nil {
			return fmt.Errorf("failed to get LP balances: %w", err)
		}

		lp := &lp{
			address:  grant.Granter,
			rollapps: make(map[string]rollappCriteria),
			balance:  resp.Balances,
		}

		for _, rollapp := range g.Rollapps {
			// check the operator fee is the minimum for what the operator wants
			if rollapp.OperatorFeeShare.Dec.LT(or.minOperatorFeeShare) {
				continue
			}

			denoms := make(map[string]bool)
			for _, denom := range rollapp.Denoms {
				denoms[denom] = true
			}
			lp.rollapps[rollapp.RollappId] = rollappCriteria{
				rollappID:           rollapp.RollappId,
				denoms:              denoms,
				maxPrice:            rollapp.MaxPrice,
				minFeePercentage:    rollapp.MinFeePercentage.Dec,
				operatorFeeShare:    rollapp.OperatorFeeShare.Dec,
				settlementValidated: rollapp.SettlementValidated,
			}
		}

		if len(lp.rollapps) == 0 {
			continue
		}

		or.lps[grant.Granter] = lp
	}

	return nil
}

func (or *orderTracker) releaseAllReservedOrdersFunds(demandOrder ...*demandOrder) {
	for _, order := range demandOrder {
		or.lpmu.Lock()
		if lp, ok := or.lps[order.lpAddress]; ok {
			lp.releaseFunds(order.price)
		}
		or.lpmu.Unlock()
	}
}

func (or *orderTracker) debitAllReservedOrdersFunds(demandOrder ...*demandOrder) {
	for _, order := range demandOrder {
		or.lpmu.Lock()
		if lp, ok := or.lps[order.lpAddress]; ok {
			lp.debitReservedFunds(order.price)
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
		resp, err := or.getBalances(ctx, &banktypes.QuerySpendableBalancesRequest{
			Address: lp.address,
		})
		if err != nil {
			or.logger.Error("failed to get balances", zap.Error(err))
			continue
		}
		lp.setBalance(resp.Balances)
	}
}

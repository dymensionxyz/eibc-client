package eibc

import (
	"context"
	"crypto/sha256"
	"encoding/json"
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
	Rollapps      map[string]rollappCriteria
	bmu           sync.Mutex
	balance       sdk.Coins
	reservedFunds sdk.Coins
	hash          string
}

// note: if spend limit is added, it will change the hash every time it's updated
type rollappCriteria struct {
	RollappID           string
	Denoms              map[string]bool
	MaxPrice            sdk.Coins
	MinFeePercentage    sdk.Dec
	OperatorFeeShare    sdk.Dec
	SettlementValidated bool
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

	var lpsUpdated bool

	currentLPCount := len(or.lps)

	for _, grant := range grants.Grants {
		if grant.Authorization == nil {
			continue
		}

		if grant.Granter == "" || grant.Grantee == "" {
			or.logger.Error("invalid grant", zap.Any("grant", grant))
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
			Rollapps: make(map[string]rollappCriteria),
			balance:  resp.Balances,
		}

		for _, rollapp := range g.Rollapps {
			// check if the rollapp is supported
			if !or.isRollappSupported(rollapp.RollappId) {
				continue
			}
			// check the operator fee is the minimum for what the operator wants
			if rollapp.OperatorFeeShare.Dec.LT(or.minOperatorFeeShare) {
				continue
			}

			denoms := make(map[string]bool)
			for _, denom := range rollapp.Denoms {
				denoms[denom] = true
			}
			lp.Rollapps[rollapp.RollappId] = rollappCriteria{
				RollappID:           rollapp.RollappId,
				Denoms:              denoms,
				MaxPrice:            rollapp.MaxPrice,
				MinFeePercentage:    rollapp.MinFeePercentage.Dec,
				OperatorFeeShare:    rollapp.OperatorFeeShare.Dec,
				SettlementValidated: rollapp.SettlementValidated,
			}
		}

		if len(lp.Rollapps) == 0 {
			continue
		}

		lp.setHash()

		l, ok := or.lps[grant.Granter]
		if ok && l.hash != lp.hash {
			or.logger.Info("LP updated", zap.String("address", grant.Granter))
			lpsUpdated = true
		}

		or.lps[grant.Granter] = lp
	}

	if lpsUpdated || (currentLPCount > 0 && len(or.lps) > currentLPCount) {
		or.logger.Info("LPs updated, resetting order polling pagination")
		or.resetPoller()
	}

	return nil
}

func (or *orderTracker) releaseAllReservedOrdersFunds(demandOrder ...*demandOrder) {
	or.lpmu.Lock()
	defer or.lpmu.Unlock()
	for _, order := range demandOrder {
		if lp, ok := or.lps[order.lpAddress]; ok {
			lp.releaseFunds(order.price)
		}
	}
}

func (or *orderTracker) debitAllReservedOrdersFunds(demandOrder ...*demandOrder) {
	or.lpmu.Lock()
	defer or.lpmu.Unlock()
	for _, order := range demandOrder {
		if lp, ok := or.lps[order.lpAddress]; ok {
			lp.debitReservedFunds(order.price)
		}
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
	or.lpmu.Lock()
	defer or.lpmu.Unlock()
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

func (l *lp) setHash() {
	jsn, err := json.Marshal(l.Rollapps) // only hash the rollapps
	if err != nil {
		panic(err)
	}
	hash := sha256.Sum256(jsn)
	l.hash = fmt.Sprintf("%x", hash[:])
}

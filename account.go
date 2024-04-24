package main

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"go.uber.org/zap"
)

func (oc *orderClient) setupAccount() error {
	account, err := oc.client.AccountRegistry.GetByName(oc.accountName)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}
	oc.account = mustConvertAccount(account.Record)

	oc.logger.Info("using account",
		zap.String("name", oc.accountName),
		zap.String("pub key", oc.account.GetPubKey().String()),
		zap.String("address", oc.account.GetAddress().String()),
		zap.String("keyring-backend", oc.client.AccountRegistry.Keyring.Backend()),
		zap.String("home-dir", oc.client.Context().HomeDir),
	)
	return nil
}

func (oc *orderClient) getAccountBalance(ctx context.Context, address, denom string) (*sdk.Coin, error) {
	resp, err := banktypes.NewQueryClient(oc.client.Context()).Balance(ctx, &banktypes.QueryBalanceRequest{
		Address: address,
		Denom:   denom,
	})
	if err != nil {
		return nil, err
	}
	return resp.Balance, nil
}

func (oc *orderClient) getAccountBalances(ctx context.Context, address string) (sdk.Coins, error) {
	resp, err := banktypes.NewQueryClient(oc.client.Context()).SpendableBalances(ctx, &banktypes.QuerySpendableBalancesRequest{
		Address: address,
	})
	if err != nil {
		return nil, err
	}
	return resp.Balances, nil
}

func mustConvertAccount(rec *keyring.Record) client.Account {
	address, err := rec.GetAddress()
	if err != nil {
		panic(fmt.Errorf("failed to get account address: %w", err))
	}
	pubKey, err := rec.GetPubKey()
	if err != nil {
		panic(fmt.Errorf("failed to get account pubkey: %w", err))
	}
	return types.NewBaseAccount(address, pubKey, 0, 0)
}

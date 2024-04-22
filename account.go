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
	if !oc.accountExists(oc.accountName) {
		return fmt.Errorf("account does not exist: %s", oc.accountName)
	} else {
		account, err := oc.client.AccountRegistry.GetByName(oc.accountName)
		if err != nil {
			return fmt.Errorf("failed to get account: %w", err)
		}
		oc.account = mustConvertAccount(account.Record)

	}
	oc.logger.Info("using account",
		zap.String("oc.accountName", oc.accountName),
		zap.String("account.PubKey", oc.account.GetPubKey().String()),
		zap.String("account.Address", oc.account.GetAddress().String()),
		zap.String("keyring-backend", oc.client.AccountRegistry.Keyring.Backend()),
		zap.String("home-dir", oc.client.Context().HomeDir),
	)
	return nil
}

func (oc *orderClient) accountExists(name string) bool {
	account, err := oc.client.AccountRegistry.GetByName(name)
	return err == nil && account.Record != nil
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

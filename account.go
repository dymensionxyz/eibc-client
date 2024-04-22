package main

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/go-bip39"
	hd2 "github.com/evmos/evmos/v12/crypto/hd"
	"go.uber.org/zap"
)

func (oc *orderClient) setupAccount() error {
	if !oc.accountExists(oc.accountName) {
		// create or import account
		if oc.config.Mnemonic != "" {
			defer func() { oc.config.Mnemonic = "" }() // clear mnemonic

			if err := oc.importAccount(oc.accountName, oc.config.Mnemonic); err != nil {
				return fmt.Errorf("failed to import account: %w", err)
			}
		} else {
			if err := oc.createAccount(oc.accountName); err != nil {
				return fmt.Errorf("failed to create account: %w", err)
			}
		}
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
		zap.String("keyring-backend", oc.config.KeyringBackend),
		zap.String("home-dir", oc.config.HomeDir),
	)
	return nil
}

func (oc *orderClient) createAccount(name string) error {
	mnemonic, err := oc.newMnemonic()
	if err != nil {
		return fmt.Errorf("failed to get mnemonic: %w", err)
	}

	account, err := oc.newAccount(name, mnemonic)
	if err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}
	oc.account = account

	oc.logger.Info("created account",
		zap.String("name", name),
		zap.String("address", oc.account.GetAddress().String()),
		zap.String("mnemonic", mnemonic))

	return nil
}

func (oc *orderClient) importAccount(name, mnemonic string) error {
	account, err := oc.newAccount(name, mnemonic)
	if err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}
	oc.account = account

	oc.logger.Info("imported account",
		zap.String("name", name),
		zap.String("address", oc.account.GetAddress().String()))

	return nil
}

func (oc *orderClient) newAccount(name, mnemonic string) (client.Account, error) {
	keyringAlgos, _ := oc.client.Context().Keyring.SupportedAlgorithms()
	algo, err := keyring.NewSigningAlgoFromString(string(hd2.EthSecp256k1.Name()), keyringAlgos)
	if err != nil {
		return nil, fmt.Errorf("failed to get signing algo: %w", err)
	}

	hdPath := hd.CreateHDPath(60, 0, 0).String()
	account, err := oc.client.Context().Keyring.NewAccount(name, mnemonic, "", hdPath, algo)
	if err != nil {
		return nil, fmt.Errorf("failed to create account: %w", err)
	}

	return mustConvertAccount(account), nil
}

func (oc *orderClient) newMnemonic() (string, error) {
	entropySeed, err := bip39.NewEntropy(mnemonicEntropySize)
	if err != nil {
		return "", fmt.Errorf("failed to get entropy: %w", err)
	}

	mnemonic, err := bip39.NewMnemonic(entropySeed)
	if err != nil {
		return "", fmt.Errorf("failed to get mnemonic: %w", err)
	}

	return mnemonic, nil
}

func (oc *orderClient) accountExists(name string) bool {
	account, err := oc.client.AccountRegistry.GetByName(name)
	return err == nil && account.Record != nil
}

func (oc *orderClient) checkBalance(ctx context.Context, minCoins sdk.Coins) (bool, error) {
	balances, err := oc.getAccountBalances(ctx, oc.account.GetAddress().String())
	if err != nil {
		return false, fmt.Errorf("failed to check account balance: %w", err)
	}

	oc.logger.Info("checking account balances",
		zap.String("account", oc.account.GetAddress().String()),
		zap.Any("balances", balances))

	for _, minCoin := range minCoins {
		balance := balances.AmountOf(minCoin.Denom)
		if balance.LT(minCoin.Amount) {
			oc.logger.Warn("insufficient account balance",
				zap.String("denom", minCoin.Denom),
				zap.String("amount", balance.String()),
				zap.String("min_amount", minCoin.String()))

			oc.alertDenom(ctx, minCoin)
			return false, nil
		} else {
			oc.resetAlertDenom(minCoin.Denom)
		}
	}
	return true, nil
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

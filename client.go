package main

import (
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	eibctypes "github.com/dymensionxyz/dymension/v3/x/eibc/types"
	rollapptypes "github.com/dymensionxyz/dymension/v3/x/rollapp/types"
	"github.com/ignite/cli/ignite/pkg/cosmosaccount"
	"go.uber.org/zap"
)

func (oc *orderClient) createAccount(name string) error {
	account, mnemonic, err := oc.client.AccountRegistry.Create(name)
	if err != nil {
		return fmt.Errorf("failed to create account: %w", err)
	}

	oc.account = mustConvertAccount(account)

	oc.logger.Info("created account",
		zap.String("name", name),
		zap.String("address", oc.account.GetAddress().String()),
		zap.String("mnemonic", mnemonic))

	return nil
}

func (oc *orderClient) importAccount(name, mnemonic string) error {
	account, err := oc.client.AccountRegistry.Import(name, mnemonic, "")
	if err != nil {
		return fmt.Errorf("failed to import account: %w", err)
	}

	oc.account = mustConvertAccount(account)

	oc.logger.Info("imported account",
		zap.String("name", name),
		zap.String("address", oc.account.GetAddress().String()),
		zap.String("mnemonic", mnemonic))

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

func mustConvertAccount(acc cosmosaccount.Account) client.Account {
	address, err := acc.Record.GetAddress()
	if err != nil {
		panic(fmt.Errorf("failed to get account address: %w", err))
	}
	pubKey, err := acc.Record.GetPubKey()
	if err != nil {
		panic(fmt.Errorf("failed to get account pubkey: %w", err))
	}
	return types.NewBaseAccount(address, pubKey, 0, 0)
}

func (oc *orderClient) getDemandOrdersByStatus(ctx context.Context, status string) ([]*eibctypes.DemandOrder, error) {
	queryClient := eibctypes.NewQueryClient(oc.client.Context())
	resp, err := queryClient.DemandOrdersByStatus(ctx, &eibctypes.QueryDemandOrdersByStatusRequest{
		Status: status,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get demand orders: %w", err)
	}
	return resp.DemandOrders, nil
}

func (oc *orderClient) subscribeToPendingDemandOrders(ctx context.Context) error {
	if err := oc.client.RPC.Start(); err != nil {
		return fmt.Errorf("failed to start rpc: %w", err)
	}

	const query = "eibc.is_fulfilled='false'"

	resCh, err := oc.client.RPC.Subscribe(ctx, "", query)
	if err != nil {
		return fmt.Errorf("failed to subscribe to demand orders: %w", err)
	}

	go func() {
		for {
			select {
			case res := <-resCh:
				oc.resToDemandOrder(res)
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (oc *orderClient) fulfillDemandOrders(demandOrderID ...string) error {
	msgs := make([]sdk.Msg, len(demandOrderID))

	for i, id := range demandOrderID {
		msgs[i] = &eibctypes.MsgFulfillOrder{
			OrderId:          id,
			FulfillerAddress: oc.account.GetAddress().String(),
		}
	}

	_, err := oc.client.BroadcastTx(oc.accountName, msgs...)
	if err != nil {
		return fmt.Errorf("failed to broadcast tx: %w", err)
	}

	return nil
}

func (oc *orderClient) getDisputePeriodInBlocks(ctx context.Context) (uint64, error) {
	queryClient := rollapptypes.NewQueryClient(oc.client.Context())
	resp, err := queryClient.Params(ctx, &rollapptypes.QueryParamsRequest{})
	if err != nil {
		return 0, fmt.Errorf("failed to get dispute period: %w", err)
	}
	return resp.Params.DisputePeriodInBlocks, nil
}

func (oc *orderClient) getLatestHeight(ctx context.Context) (int64, error) {
	status, err := oc.client.Status(ctx)
	if err != nil {
		return 0, err
	}
	return status.SyncInfo.LatestBlockHeight, nil
}

package eibc

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/cosmos-sdk/x/feegrant"
	"github.com/cosmos/cosmos-sdk/x/group"
	"github.com/dymensionxyz/cosmosclient/cosmosclient"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/status"

	"github.com/dymensionxyz/eibc-client/config"
)

func addAccount(client cosmosclient.Client, name string) (string, error) {
	acc, err := client.AccountRegistry.GetByName(name)
	if err == nil {
		address, err := acc.Record.GetAddress()
		if err != nil {
			return "", fmt.Errorf("failed to get account address: %w", err)
		}
		return address.String(), nil
	}

	acc, _, err = client.AccountRegistry.Create(name)
	if err != nil {
		return "", fmt.Errorf("failed to create account: %w", err)
	}

	address, err := acc.Record.GetAddress()
	if err != nil {
		return "", fmt.Errorf("failed to get account address: %w", err)
	}
	return address.String(), nil
}

func getFulfillerAccounts(client cosmosclient.Client) (accs []account, err error) {
	var accounts []account
	accounts, err = listAccounts(client)
	if err != nil {
		return
	}

	for _, acc := range accounts {
		if !strings.HasPrefix(acc.Name, config.FulfillerNamePrefix) {
			continue
		}
		accs = append(accs, acc)
	}
	return
}

func listAccounts(client cosmosclient.Client) ([]account, error) {
	accs, err := client.AccountRegistry.List()
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts: %w", err)
	}

	var accounts []account
	for _, acc := range accs {
		addr, err := acc.Record.GetAddress()
		if err != nil {
			return nil, fmt.Errorf("failed to get account address: %w", err)
		}
		acct := account{
			Name:    acc.Name,
			Address: addr.String(),
		}
		accounts = append(accounts, acct)
	}

	return accounts, nil
}

func createFulfillerAccounts(client cosmosclient.Client, count int) (accs []account, err error) {
	for range count {
		fulfillerName := fmt.Sprintf("%s%s", config.FulfillerNamePrefix, uuid.New().String()[0:5])
		addr, err := addAccount(client, fulfillerName)
		if err != nil {
			return nil, fmt.Errorf("failed to create account: %w", err)
		}
		acc := account{
			Name:    fulfillerName,
			Address: addr,
		}
		accs = append(accs, acc)
	}
	return
}

func sendCoinsMulti(client cosmosClient, coins sdk.Coins, fromName string, fromAddr sdk.AccAddress, toAddrStr ...string) error {
	outputs := make([]banktypes.Output, 0, len(toAddrStr))
	for _, addrStr := range toAddrStr {
		toAddr, err := sdk.AccAddressFromBech32(addrStr)
		if err != nil {
			return fmt.Errorf("failed to parse address: %w", err)
		}
		outputs = append(outputs, banktypes.NewOutput(toAddr, coins))
	}

	inputCoins, ok := coins.SafeMulInt(sdk.NewInt(int64(len(toAddrStr))))
	if !ok {
		return fmt.Errorf("failed to calculate input coins")
	}
	msg := banktypes.NewMsgMultiSend([]banktypes.Input{banktypes.NewInput(fromAddr, inputCoins)}, outputs)

	rsp, err := client.BroadcastTx(fromName, msg)
	if err != nil {
		return fmt.Errorf("failed to broadcast tx: %w", err)
	}

	if _, err = waitForTx(client, rsp.TxHash); err != nil {
		return fmt.Errorf("failed to wait for tx: %w", err)
	}

	return nil
}

func waitForTx(client cosmosClient, txHash string) (*tx.GetTxResponse, error) {
	serviceClient := tx.NewServiceClient(client.Context())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	ticker := time.NewTicker(time.Second)

	defer func() {
		cancel()
		ticker.Stop()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out waiting for tx %s", txHash)
		case <-ticker.C:
			resp, err := serviceClient.GetTx(ctx, &tx.GetTxRequest{Hash: txHash})
			if err != nil {
				continue
			}
			if resp.TxResponse.Code == 0 {
				return resp, nil
			} else {
				return nil, fmt.Errorf("tx failed with code %d: %s", resp.TxResponse.Code, resp.TxResponse.RawLog)
			}
		}
	}
}

func addFulfillerAccounts(scale int, clientConfig config.ClientConfig, logger *zap.Logger) ([]account, error) {
	cClient, err := cosmosclient.New(config.GetCosmosClientOptions(clientConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create cosmos client for fulfiller: %w", err)
	}

	accs, err := getFulfillerAccounts(cClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get fulfiller accounts: %w", err)
	}

	numFoundFulfillers := len(accs)

	fulfillersAccountsToCreate := max(0, scale) - numFoundFulfillers
	if fulfillersAccountsToCreate > 0 {
		logger.Info("creating fulfiller accounts", zap.Int("accounts", fulfillersAccountsToCreate))
	}

	newAccs, err := createFulfillerAccounts(cClient, fulfillersAccountsToCreate)
	if err != nil {
		return nil, fmt.Errorf("failed to create fulfiller accounts: %w", err)
	}

	accs = slices.Concat(accs, newAccs)

	if len(accs) < scale {
		return nil, fmt.Errorf("expected %d fulfiller accounts, got %d", scale, len(accs))
	}
	return accs, nil
}

func addFulfillersToGroup(operatorName, operatorAddress string, groupID int, client cosmosclient.Client, newAccs []account) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	c := group.NewQueryClient(client.Context())
	members, err := c.GroupMembers(ctx, &group.QueryGroupMembersRequest{
		GroupId: uint64(groupID),
	})
	if err != nil {
		return err
	}

	memberMap := make(map[string]struct{})

	memberUpdates := make([]group.MemberRequest, 0, len(members.Members))
	for _, m := range members.Members {
		memberUpdates = append(memberUpdates, group.MemberRequest{
			Address: m.Member.Address,
			Weight:  m.Member.Weight,
		})
		memberMap[m.Member.Address] = struct{}{}
	}

	countAddMembers := 0

	for _, acc := range newAccs {
		if _, ok := memberMap[acc.Address]; ok {
			continue
		}
		memberUpdates = append(memberUpdates, group.MemberRequest{
			Address: acc.Address,
			Weight:  "1",
		})
		countAddMembers++
	}

	if countAddMembers == 0 {
		return nil
	}

	msg := group.MsgUpdateGroupMembers{
		Admin:         operatorAddress,
		GroupId:       uint64(groupID),
		MemberUpdates: memberUpdates,
	}

	resp, err := client.BroadcastTx(operatorName, &msg)
	if err != nil {
		return err
	}

	if _, err = waitForTx(client, resp.TxHash); err != nil {
		return fmt.Errorf("failed to wait for tx: %w", err)
	}

	return nil
}

func primeAccounts(client cosmosClient, fromName string, fromAddr sdk.AccAddress, toAddr ...string) error {
	if err := sendCoinsMulti(client, sdk.NewCoins(sdk.NewCoin("adym", sdk.NewInt(1))), fromName, fromAddr, toAddr...); err != nil {
		return fmt.Errorf("failed to send coins: %w", err)
	}
	return nil
}

func accountExists(clientCtx client.Context, address string) (bool, error) {
	// Parse the address
	addr, err := sdk.AccAddressFromBech32(address)
	if err != nil {
		return false, fmt.Errorf("invalid address: %v", err)
	}

	// Create a query client for the auth module
	authClient := authtypes.NewQueryClient(clientCtx)

	// Prepare the request
	req := &authtypes.QueryAccountRequest{
		Address: addr.String(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Query the account
	res, err := authClient.Account(ctx, req)
	if err != nil {
		// Check if the error indicates that the account does not exist
		if grpcErrorCode(err) == "NotFound" {
			return false, nil
		}
		return false, fmt.Errorf("failed to query account: %v", err)
	}

	// If res.Account is not nil, account exists
	if res.Account != nil {
		return true, nil
	}

	return false, nil
}

func hasFeeGranted(client cosmosClient, granterAddr, granteeAddr string) (bool, error) {
	feeGrantClient := feegrant.NewQueryClient(client.Context())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := feeGrantClient.Allowances(
		ctx,
		&feegrant.QueryAllowancesRequest{
			Grantee: granteeAddr,
		},
	)
	if err != nil {
		return false, fmt.Errorf("failed to query fee grants: %w", err)
	}

	for _, allowance := range res.Allowances {
		if allowance.Granter == granterAddr {
			return true, nil
		}
	}

	// Check if the account has enough balance
	return false, nil
}

func addFeeGrantToFulfiller(client cosmosClient, fromName string, granterAddr sdk.AccAddress, granteeAddr ...sdk.AccAddress) error {
	msgs := make([]sdk.Msg, 0, len(granteeAddr))
	for _, addr := range granteeAddr {
		msg, err := feegrant.NewMsgGrantAllowance(
			&feegrant.BasicAllowance{},
			granterAddr,
			addr,
		)
		if err != nil {
			return fmt.Errorf("failed to create fee grant msg: %w", err)
		}
		msgs = append(msgs, msg)
	}
	rsp, err := client.BroadcastTx(fromName, msgs...)
	if err != nil {
		return fmt.Errorf("failed to broadcast tx: %w", err)
	}

	if _, err = waitForTx(client, rsp.TxHash); err != nil {
		return fmt.Errorf("failed to wait for tx: %w", err)
	}

	return nil
}

func grpcErrorCode(err error) string {
	if grpcStatus, ok := status.FromError(err); ok {
		return grpcStatus.Code().String()
	}
	return ""
}

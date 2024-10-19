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
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/status"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"

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

func getBotAccounts(client cosmosclient.Client) (accs []account, err error) {
	var accounts []account
	accounts, err = listAccounts(client)
	if err != nil {
		return
	}

	for _, acc := range accounts {
		if !strings.HasPrefix(acc.Name, config.BotNamePrefix) {
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

func createBotAccounts(client cosmosclient.Client, count int) (accs []account, err error) {
	for range count {
		botName := fmt.Sprintf("%s%s", config.BotNamePrefix, uuid.New().String()[0:5])
		addr, err := addAccount(client, botName)
		if err != nil {
			return nil, fmt.Errorf("failed to create account: %w", err)
		}
		acc := account{
			Name:    botName,
			Address: addr,
		}
		accs = append(accs, acc)
	}
	return
}

func sendCoins(client cosmosClient, coins sdk.Coins, fromName string, fromAddr sdk.AccAddress, toAddrStr string) error {
	toAddr, err := sdk.AccAddressFromBech32(toAddrStr)
	if err != nil {
		return fmt.Errorf("failed to parse address: %w", err)
	}

	msg := banktypes.NewMsgSend(
		fromAddr,
		toAddr,
		coins,
	)

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

func addBotAccounts(numBots int, clientConfig config.ClientConfig, logger *zap.Logger) ([]account, error) {
	cosmosClient, err := cosmosclient.New(config.GetCosmosClientOptions(clientConfig)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create cosmos client for bot: %w", err)
	}

	accs, err := getBotAccounts(cosmosClient)
	if err != nil {
		return nil, fmt.Errorf("failed to get bot accounts: %w", err)
	}

	numFoundBots := len(accs)

	botsAccountsToCreate := max(0, numBots) - numFoundBots
	if botsAccountsToCreate > 0 {
		logger.Info("creating bot accounts", zap.Int("accounts", botsAccountsToCreate))
	}

	newAccs, err := createBotAccounts(cosmosClient, botsAccountsToCreate)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot accounts: %w", err)
	}

	accs = slices.Concat(accs, newAccs)

	if len(accs) < numBots {
		return nil, fmt.Errorf("expected %d bot accounts, got %d", numBots, len(accs))
	}
	return accs, nil
}

func addBotsToGroup(operatorName, operatorAddress string, groupID int, client cosmosclient.Client, newAccs []account) error {
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

func primeAccount(client cosmosClient, fromName string, fromAddr sdk.AccAddress, toAddr string) error {
	if err := sendCoins(client, sdk.NewCoins(sdk.NewCoin("adym", sdk.NewInt(1))), fromName, fromAddr, toAddr); err != nil {
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

func addFeeGrantToBot(client cosmosClient, fromName string, granterAddr, granteeAddr sdk.AccAddress) error {
	msg, err := feegrant.NewMsgGrantAllowance(
		&feegrant.BasicAllowance{},
		granterAddr,
		granteeAddr,
	)
	if err != nil {
		return fmt.Errorf("failed to create fee grant msg: %w", err)
	}
	rsp, err := client.BroadcastTx(fromName, msg)
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

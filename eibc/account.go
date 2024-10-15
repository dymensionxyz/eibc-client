package eibc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/google/uuid"
	"github.com/ignite/cli/ignite/pkg/cosmosaccount"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"

	"github.com/dymensionxyz/eibc-client/config"
	"github.com/dymensionxyz/eibc-client/store"
)

type accountService struct {
	client            cosmosClient
	bankClient        bankClient
	accountRegistry   cosmosaccount.Registry
	store             accountStore
	logger            *zap.Logger
	account           client.Account
	balances          sdk.Coins
	minimumGasBalance sdk.Coin
	accountName       string
	homeDir           string
	topUpFactor       int
	topUpCh           chan<- topUpRequest
	asyncClient       bool
}

type accountStore interface {
	GetBot(ctx context.Context, address string, _ ...store.BotOption) (*store.Bot, error)
	SaveBot(ctx context.Context, bot *store.Bot) error
}

type bankClient interface {
	SpendableBalances(ctx context.Context, in *banktypes.QuerySpendableBalancesRequest, opts ...grpc.CallOption) (*banktypes.QuerySpendableBalancesResponse, error)
}

type option func(*accountService)

func withTopUpFactor(topUpFactor int) option {
	return func(s *accountService) {
		s.topUpFactor = topUpFactor
	}
}

func newAccountService(
	client cosmosclient.Client,
	store accountStore,
	logger *zap.Logger,
	accountName string,
	minimumGasBalance sdk.Coin,
	topUpCh chan topUpRequest,
	options ...option,
) (*accountService, error) {
	a := &accountService{
		client:            client,
		accountRegistry:   client.AccountRegistry,
		store:             store,
		logger:            logger.With(zap.String("module", "account-service")),
		accountName:       accountName,
		homeDir:           client.Context().HomeDir,
		bankClient:        banktypes.NewQueryClient(client.Context()),
		minimumGasBalance: minimumGasBalance,
		topUpCh:           topUpCh,
	}

	for _, opt := range options {
		opt(a)
	}

	if err := a.setupAccount(); err != nil {
		return nil, fmt.Errorf("failed to setup account: %w", err)
	}

	return a, nil
}

func addAccount(client cosmosclient.Client, name string) error {
	_, err := client.AccountRegistry.GetByName(name)
	if err == nil {
		return nil
	}

	_, _, err = client.AccountRegistry.Create(name)
	return err
}

func getBotAccounts(client cosmosclient.Client) (accs []string, err error) {
	var accounts []account
	accounts, err = listAccounts(client)
	if err != nil {
		return
	}

	for _, acc := range accounts {
		if !strings.HasPrefix(acc.Name, config.BotNamePrefix) {
			continue
		}
		accs = append(accs, acc.Name)
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

func createBotAccounts(client cosmosclient.Client, count int) (names []string, err error) {
	for range count {
		botName := fmt.Sprintf("%s%s", config.BotNamePrefix, uuid.New().String()[0:5])
		if err = addAccount(client, botName); err != nil {
			err = fmt.Errorf("failed to create account: %w", err)
			return
		}
		names = append(names, botName)
	}
	return
}

// TODO: if not found...?
func (a *accountService) setupAccount() error {
	acc, err := a.accountRegistry.GetByName(a.accountName)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}
	a.account = mustConvertAccount(acc.Record)

	a.logger.Debug("using account",
		zap.String("name", a.accountName),
		zap.String("pub key", a.account.GetPubKey().String()),
		zap.String("address", a.account.GetAddress().String()),
		zap.String("keyring-backend", a.accountRegistry.Keyring.Backend()),
		zap.String("home-dir", a.homeDir),
	)
	return nil
}

func (a *accountService) EnsureBalances(ctx context.Context, coins sdk.Coins) ([]string, error) {
	// check if gas balance is below minimum
	gasDiff := math.NewInt(0)
	if !a.minimumGasBalance.IsNil() && a.minimumGasBalance.IsPositive() {
		gasBalance := a.BalanceOf(a.minimumGasBalance.Denom)
		gasDiff = a.minimumGasBalance.Amount.Sub(gasBalance)
	}

	toTopUp := sdk.NewCoins()
	if gasDiff.IsPositive() {
		toTopUp = toTopUp.Add(a.minimumGasBalance) // add the whole amount instead of the difference
	}

	fundedDenoms := make([]string, 0, len(coins))

	// check if balance is below required
	for _, coin := range coins {
		balance := a.BalanceOf(coin.Denom)
		diff := coin.Amount.Sub(balance)
		if diff.IsPositive() {
			// add x times the coin amount to the top up
			// to avoid frequent top ups
			if a.topUpFactor > 0 {
				coin.Amount = coin.Amount.MulRaw(int64(a.topUpFactor))
			}
			toTopUp = toTopUp.Add(coin) // add the whole amount instead of the difference
		} else {
			fundedDenoms = append(fundedDenoms, coin.Denom)
		}
	}

	if toTopUp.Empty() {
		// shouldn't happen
		return fundedDenoms, nil
	}

	// blocking operation
	resCh := make(chan []string)
	topUpReq := topUpRequest{
		coins:  toTopUp,
		toAddr: a.account.GetAddress().String(),
		res:    resCh,
	}

	a.topUpCh <- topUpReq
	res := <-resCh
	a.logger.Debug("topped up denoms", zap.Strings("denoms", res))
	close(resCh)

	if err := a.RefreshBalances(ctx); err != nil {
		return nil, fmt.Errorf("failed to refresh account balances: %w", err)
	}

	for _, coin := range coins {
		balance := a.BalanceOf(coin.Denom)
		if balance.GTE(coin.Amount) {
			fundedDenoms = append(fundedDenoms, coin.Denom)
		}
	}

	return fundedDenoms, nil
}

func (a *accountService) SendCoins(ctx context.Context, coins sdk.Coins, toAddrStr string) error {
	toAddr, err := sdk.AccAddressFromBech32(toAddrStr)
	if err != nil {
		return fmt.Errorf("failed to parse address: %w", err)
	}

	msg := banktypes.NewMsgSend(
		a.account.GetAddress(),
		toAddr,
		coins,
	)
	start := time.Now()

	rsp, err := a.client.BroadcastTx(a.accountName, msg)
	if err != nil {
		return fmt.Errorf("failed to broadcast tx: %w", err)
	}

	if err = a.WaitForTx(rsp.TxHash); err != nil {
		return fmt.Errorf("failed to wait for tx: %w", err)
	}

	if err := a.RefreshBalances(ctx); err != nil {
		a.logger.Error("failed to refresh account balances", zap.Error(err))
	}

	a.logger.Debug("coins sent", zap.String("to", toAddrStr), zap.Duration("duration", time.Since(start)))

	return nil
}

func (a *accountService) WaitForTx(txHash string) error {
	if a.asyncClient {
		return nil
	}
	serviceClient := tx.NewServiceClient(a.client.Context())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	ticker := time.NewTicker(time.Second)

	defer func() {
		cancel()
		ticker.Stop()
	}()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for tx %s", txHash)
		case <-ticker.C:
			resp, err := serviceClient.GetTx(ctx, &tx.GetTxRequest{Hash: txHash})
			if err != nil {
				continue
			}
			if resp.TxResponse.Code == 0 {
				return nil
			} else {
				return fmt.Errorf("tx failed with code %d: %s", resp.TxResponse.Code, resp.TxResponse.RawLog)
			}
		}
	}
}

func (a *accountService) BalanceOf(denom string) sdk.Int {
	if a.balances == nil {
		return sdk.ZeroInt()
	}
	return a.balances.AmountOf(denom)
}

type fundsOption func(*fundsRequest)

type fundsRequest struct {
	fees sdk.Coins
}

func addFeeEarnings(rewards sdk.Coins) fundsOption {
	return func(r *fundsRequest) {
		r.fees = rewards
	}
}

func (a *accountService) RefreshBalances(ctx context.Context) error {
	balances, err := a.GetAccountBalances(ctx)
	if err != nil {
		return fmt.Errorf("failed to get account balances: %w", err)
	}
	a.balances = balances
	return nil
}

func (a *accountService) UpdateFunds(ctx context.Context, opts ...fundsOption) error {
	if err := a.RefreshBalances(ctx); err != nil {
		return fmt.Errorf("failed to refresh account balances: %w", err)
	}

	b, err := a.store.GetBot(ctx, a.account.GetAddress().String())
	if err != nil {
		return fmt.Errorf("failed to get bot: %w", err)
	}
	if b == nil {
		b = &store.Bot{
			Address: a.Address(),
			Name:    a.accountName,
		}
	}

	// set to active only the bots that will be used
	b.Active = true

	b.Balances = make([]string, 0, len(a.balances))
	for _, balance := range a.balances {
		b.Balances = append(b.Balances, balance.String())
	}

	pendingEarnings := b.PendingEarnings.ToCoins()

	req := &fundsRequest{}
	for _, opt := range opts {
		opt(req)
	}

	if len(req.fees) > 0 {
		pendingEarnings = pendingEarnings.Add(req.fees...)
		b.PendingEarnings = store.CoinsToStrings(pendingEarnings)
	}

	if err := a.store.SaveBot(ctx, b); err != nil {
		return fmt.Errorf("failed to update bot: %w", err)
	}

	return nil
}

func (a *accountService) Address() string {
	return a.account.GetAddress().String()
}

func (a *accountService) GetAccountBalances(ctx context.Context) (sdk.Coins, error) {
	resp, err := a.bankClient.SpendableBalances(ctx, &banktypes.QuerySpendableBalancesRequest{
		Address: a.Address(),
	})
	if err != nil {
		return nil, err
	}
	return resp.Balances, nil
}

func (a *accountService) GetBalances() sdk.Coins {
	return a.balances
}

func (a *accountService) SetBalances(coins sdk.Coins) {
	a.balances = coins
}

func (a *accountService) GetAccountName() string {
	return a.accountName
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

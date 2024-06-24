package main

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"

	"github.com/dymensionxyz/eibc-client/store"
)

type accountService struct {
	client            cosmosclient.Client
	store             accountStore
	logger            *zap.Logger
	account           client.Account
	balances          sdk.Coins
	minimumGasBalance sdk.Coin
	accountName       string
	topUpFactor       uint64
	topUpCh           chan<- topUpRequest
}

type accountStore interface {
	GetBot(ctx context.Context, address string, _ ...store.BotOption) (*store.Bot, error)
	SaveBot(ctx context.Context, bot *store.Bot) error
}

type option func(*accountService)

func withTopUpFactor(topUpFactor uint64) option {
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
		store:             store,
		logger:            logger.With(zap.String("module", "account-service")),
		accountName:       accountName,
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
		if !strings.HasPrefix(acc.Name, botNamePrefix) {
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
		botName := fmt.Sprintf("%s%s", botNamePrefix, uuid.New().String()[0:5])
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
	acc, err := a.client.AccountRegistry.GetByName(a.accountName)
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}
	a.account = mustConvertAccount(acc.Record)

	a.logger.Debug("using account",
		zap.String("name", a.accountName),
		zap.String("pub key", a.account.GetPubKey().String()),
		zap.String("address", a.account.GetAddress().String()),
		zap.String("keyring-backend", a.client.AccountRegistry.Keyring.Backend()),
		zap.String("home-dir", a.client.Context().HomeDir),
	)
	return nil
}

func (a *accountService) ensureBalances(coins sdk.Coins) []string {
	// check if gas balance is below minimum
	gasBalance := a.balanceOf(a.minimumGasBalance.Denom)
	gasDiff := a.minimumGasBalance.Amount.Sub(gasBalance)

	toTopUp := sdk.NewCoins()
	if gasDiff.IsPositive() {
		toTopUp = toTopUp.Add(a.minimumGasBalance) // add the whole amount instead of the difference
	}

	fundedDenoms := make([]string, 0, len(coins))

	// check if balance is below required
	for _, coin := range coins {
		balance := a.balanceOf(coin.Denom)
		diff := coin.Amount.Sub(balance)
		if diff.IsPositive() {
			// add x times the coin amount to the top up
			// to avoid frequent top ups
			coin.Amount = coin.Amount.MulRaw(int64(a.topUpFactor))
			toTopUp = toTopUp.Add(coin) // add the whole amount instead of the difference
		} else {
			fundedDenoms = append(fundedDenoms, coin.Denom)
		}
	}

	if toTopUp.Empty() {
		// shouldn't happen
		return fundedDenoms
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
	close(resCh)

	fundedDenoms = slices.Concat(fundedDenoms, res)
	return fundedDenoms
}

func (a *accountService) sendCoins(coins sdk.Coins, toAddrStr string) error {
	toAddr, err := sdk.AccAddressFromBech32(toAddrStr)
	if err != nil {
		return fmt.Errorf("failed to parse address: %w", err)
	}

	msg := banktypes.NewMsgSend(
		a.account.GetAddress(),
		toAddr,
		coins,
	)

	_, err = a.client.BroadcastTx(a.accountName, msg)
	return err
}

func (a *accountService) balanceOf(denom string) sdk.Int {
	return a.balances.AmountOf(denom)
}

type fundsOption func(*fundsRequest)

type fundsRequest struct {
	rewards []string
}

func addRewards(rewards ...string) fundsOption {
	return func(r *fundsRequest) {
		r.rewards = rewards
	}
}

func (a *accountService) refreshBalances(ctx context.Context) error {
	balances, err := a.getAccountBalances(ctx)
	if err != nil {
		return fmt.Errorf("failed to get account balances: %w", err)
	}
	a.balances = balances
	return nil
}

func (a *accountService) updateFunds(ctx context.Context, opts ...fundsOption) error {
	if err := a.refreshBalances(ctx); err != nil {
		return fmt.Errorf("failed to refresh account balances: %w", err)
	}

	b, err := a.store.GetBot(ctx, a.account.GetAddress().String())
	if err != nil {
		return fmt.Errorf("failed to get bot: %w", err)
	}
	if b == nil {
		b = &store.Bot{
			Address: a.account.GetAddress().String(),
			Name:    a.accountName,
		}
	}

	b.Balances = make([]string, 0, len(a.balances))
	for _, balance := range a.balances {
		b.Balances = append(b.Balances, balance.String())
	}

	pendingRewards := b.PendingRewards.ToCoins()

	req := &fundsRequest{}
	for _, opt := range opts {
		opt(req)
	}

	if len(req.rewards) > 0 {
		for _, r := range req.rewards {
			pr, err := sdk.ParseCoinNormalized(r)
			if err != nil {
				return fmt.Errorf("failed to parse reward coin: %w", err)
			}
			pendingRewards = pendingRewards.Add(pr)
		}

		b.PendingRewards = store.CoinsToStrings(pendingRewards)
	}

	if err := a.store.SaveBot(ctx, b); err != nil {
		return fmt.Errorf("failed to update bot: %w", err)
	}

	return nil
}

func (a *accountService) address() string {
	return a.account.GetAddress().String()
}

func (a *accountService) getAccountBalances(ctx context.Context) (sdk.Coins, error) {
	resp, err := banktypes.NewQueryClient(a.client.Context()).SpendableBalances(ctx, &banktypes.QuerySpendableBalancesRequest{
		Address: a.account.GetAddress().String(),
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

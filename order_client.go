package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/slack-go/slack"
	tmtypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"

	"github.com/dymensionxyz/dymension/v3/x/common/types"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"
)

type orderClient struct {
	client cosmosclient.Client
	slack  slacker
	logger *zap.Logger

	chainID string
	node    string

	domu              sync.Mutex
	demandOrders      map[string]*demandOrder // id:demandOrder
	maxOrdersPerTx    int
	minimumDymBalance string
	disputePeriod     uint64

	account     client.Account
	accountName string

	sdmu          sync.Mutex
	skipDenoms    map[string]struct{}
	admu          sync.Mutex
	alertedDenoms map[string]struct{}

	refreshInterval       time.Duration
	fulfillInterval       time.Duration
	cleanupInterval       time.Duration
	disputePeriodInterval time.Duration
	balanceCheckInterval  time.Duration
}

func newOrderClient(config Config) (*orderClient, error) {
	cosmosClient, err := cosmosclient.New(
		context.Background(),
		getCosmosClientOptions(config)...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cosmos client: %w", err)
	}

	sdkcfg := sdk.GetConfig()
	sdkcfg.SetBech32PrefixForAccount(hubAddressPrefix, pubKeyPrefix)

	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	oc := &orderClient{
		client: cosmosClient,
		slack: slacker{
			Client:    slack.New(config.SlackConfig.BotToken, slack.OptionAppLevelToken(config.SlackConfig.AppToken)),
			channelID: config.SlackConfig.ChannelID,
			enabled:   config.SlackConfig.Enabled,
		},
		accountName:           config.AccountName,
		demandOrders:          make(map[string]*demandOrder),
		skipDenoms:            make(map[string]struct{}),
		alertedDenoms:         make(map[string]struct{}),
		logger:                logger,
		chainID:               cosmosClient.Context().ChainID,
		node:                  config.NodeAddress,
		minimumDymBalance:     config.MinimumDymBalance,
		maxOrdersPerTx:        defaultMaxOrdersPerTx,
		refreshInterval:       defaultRefreshInterval,
		fulfillInterval:       defaultFulfillInterval,
		cleanupInterval:       defaultCleanupInterval,
		disputePeriodInterval: defaultDisputePeriodInterval,
		balanceCheckInterval:  defaultBalanceCheckInterval,
	}
	/*
		_, _, err = oc.slack.PostMessage("poor-bots", slack.MsgOptionText("test", false))
		if err != nil {
			return nil, fmt.Errorf("failed to post message: %w", err)
		}
	*/
	if !oc.accountExists(oc.accountName) {
		// create or import account
		if config.Mnemonic != "" {
			defer func() { config.Mnemonic = "" }() // clear mnemonic

			if err := oc.importAccount(oc.accountName, config.Mnemonic); err != nil {
				return nil, fmt.Errorf("failed to import account: %w", err)
			}
		} else {
			if err := oc.createAccount(oc.accountName); err != nil {
				return nil, fmt.Errorf("failed to create account: %w", err)
			}
		}
	} else {
		account, err := oc.client.AccountRegistry.GetByName(oc.accountName)
		if err != nil {
			return nil, fmt.Errorf("failed to get account: %w", err)
		}
		oc.account = mustConvertAccount(account)

	}
	logger.Info("using account",
		zap.String("oc.accountName", oc.accountName),
		zap.String("account.PubKey", oc.account.GetPubKey().String()),
		zap.String("account.Address", oc.account.GetAddress().String()),
		zap.String("keyring-backend", config.KeyringBackend),
		zap.String("home-dir", config.HomeDir),
	)

	return oc, nil
}

type demandOrder struct {
	id                string
	price             sdk.Coins
	fee               sdk.Coins
	fulfilledAtHeight uint64
	credited          bool
}

func (oc *orderClient) start() error {
	ctx := context.Background()

	if err := oc.subscribeToPendingDemandOrders(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to demand orders: %w", err)
	}

	oc.orderRefresher(ctx)
	oc.orderFulfiller(ctx)
	oc.orderCleaner(ctx)
	oc.disputePeriodUpdater(ctx)
	// oc.accountBalanceChecker(ctx)

	make(chan struct{}) <- struct{}{} // TODO: make nicer

	return nil
}

func (oc *orderClient) GetDemandOrders() map[string]*demandOrder {
	oc.domu.Lock()
	defer oc.domu.Unlock()
	return oc.demandOrders
}

func (oc *orderClient) refreshPendingDemandOrders(ctx context.Context) error {
	oc.logger.Debug("refreshing demand orders")

	res, err := oc.getDemandOrdersByStatus(ctx, types.Status_PENDING.String())
	if err != nil {
		return fmt.Errorf("failed to get demand orders: %w", err)
	}

	oc.domu.Lock()
	defer oc.domu.Unlock()

	newCount := 0

	for _, d := range res {
		if _, found := oc.demandOrders[d.Id]; found || d.IsFullfilled {
			continue
		}
		oc.demandOrders[d.Id] = &demandOrder{
			id:    d.Id,
			price: d.Price,
			fee:   d.Fee,
		}
		newCount++
	}

	if newCount > 0 {
		oc.logger.Info("new demand orders", zap.Int("count", newCount))
	}
	return nil
}

func (oc *orderClient) resToDemandOrder(res tmtypes.ResultEvent) {
	ids := res.Events["eibc.id"]

	if len(ids) == 0 {
		return
	}

	oc.logger.Info("received demand orders", zap.Int("count", len(ids)))

	// packetKeys := res.Events["eibc.packet_key"]
	prices := res.Events["eibc.price"]
	fees := res.Events["eibc.fee"]
	statuses := res.Events["eibc.packet_status"]
	// recipients := res.Events["transfer.recipient"]

	oc.domu.Lock()
	defer oc.domu.Unlock()

	for i, id := range ids {
		if statuses[i] != types.Status_PENDING.String() {
			continue
		}
		price, err := sdk.ParseCoinNormalized(prices[i])
		if err != nil {
			oc.logger.Error("failed to parse price", zap.Error(err))
			continue
		}
		fee, err := sdk.ParseCoinNormalized(fees[i])
		if err != nil {
			oc.logger.Error("failed to parse fee", zap.Error(err))
			continue
		}
		order := &demandOrder{
			id:    id,
			price: sdk.NewCoins(price),
			fee:   sdk.NewCoins(fee),
		}
		oc.demandOrders[id] = order
	}
}

func (oc *orderClient) fulfillOrders(ctx context.Context) error {
	toFulfillIDs := make([]string, 0, len(oc.demandOrders))

	oc.domu.Lock()
	defer oc.domu.Unlock()

outer:
	for _, order := range oc.demandOrders {
		// if no funds for at least one denom in the order or the order is already fulfilled, skip it
		if order.fulfilledAtHeight > 0 {
			continue
		}
		for _, price := range order.price {
			if _, skip := oc.skipDenoms[price.Denom]; skip {
				oc.logger.Debug("skipping order because of low funds",
					zap.String("id", order.id),
					zap.String("denom", price.Denom))
				continue outer
			}
		}
		// TODO: make it more efficient
		gotBalance, err := oc.checkBalance(ctx, order.price)
		if err != nil {
			return fmt.Errorf("failed to check balance: %w", err)
		}
		if !gotBalance {
			continue
		}

		toFulfillIDs = append(toFulfillIDs, order.id)
		if oc.maxOrdersPerTx > 0 && len(toFulfillIDs) >= oc.maxOrdersPerTx {
			break
		}
	}

	if len(toFulfillIDs) == 0 {
		return nil
	}

	oc.logger.Info("fulfilling orders", zap.Int("count", len(toFulfillIDs)), zap.Strings("ids", toFulfillIDs))

	if err := oc.fulfillDemandOrders(toFulfillIDs...); err != nil {
		return fmt.Errorf("failed to fulfill order: %w", err)
	}

	for _, id := range toFulfillIDs {
		latestHeight, err := oc.getLatestHeight(context.Background())
		if err != nil {
			return fmt.Errorf("failed to get latest height: %w", err)
		}
		oc.demandOrders[id].fulfilledAtHeight = uint64(latestHeight)
	}

	return nil
}

func (oc *orderClient) cleanup() error {
	oc.domu.Lock()
	defer oc.domu.Unlock()

	cleanupCount := 0

	for id, order := range oc.demandOrders {
		// cleanup fulfilled and credited demand orders
		// if order.credited {
		if order.fulfilledAtHeight > 0 {
			delete(oc.demandOrders, id)
			cleanupCount++
		}
	}

	if cleanupCount > 0 {
		oc.logger.Info("cleaned up fulfilled orders", zap.Int("count", cleanupCount))
	}
	return nil
}

func (oc *orderClient) checkBalances(ctx context.Context) error {
	// check dym balance
	minDym, err := sdk.ParseCoinNormalized(oc.minimumDymBalance)
	if err != nil {
		return fmt.Errorf("failed to parse minimum DYM balance for '%s': %w", oc.minimumDymBalance, err)
	}
	_, err = oc.checkBalance(ctx, sdk.NewCoins(minDym))
	if err != nil {
		return fmt.Errorf("failed to check balance: %w", err)
	}

	// check demand order balances
	orderCoins := sdk.NewCoins()

	oc.domu.Lock()
	defer oc.domu.Unlock()

	for _, ord := range oc.demandOrders {
		if ord.fulfilledAtHeight > 0 {
			continue
		}
		orderCoins = orderCoins.Add(ord.price...)
	}

	_, err = oc.checkBalance(ctx, orderCoins)
	if err != nil {
		oc.logger.Error("failed to check balance", zap.Error(err))
		return fmt.Errorf("failed to check balance: %w", err)
	}
	return nil
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
			oc.skipDenom(minCoin.Denom)
			return false, nil
		} else {
			oc.resetDenom(minCoin.Denom)
		}
	}
	return true, nil
}

func (oc *orderClient) denomSkipped(denom string) bool {
	oc.sdmu.Lock()
	defer oc.sdmu.Unlock()
	_, found := oc.skipDenoms[denom]
	return found
}

func (oc *orderClient) denomAlerted(denom string) bool {
	oc.admu.Lock()
	defer oc.admu.Unlock()
	_, found := oc.alertedDenoms[denom]
	return found
}

func (oc *orderClient) skipDenom(denom string) {
	oc.sdmu.Lock()
	defer oc.sdmu.Unlock()
	_, found := oc.skipDenoms[denom]
	if found {
		return
	}

	oc.skipDenoms[denom] = struct{}{}
}

func (oc *orderClient) resetDenom(denom string) {
	oc.resetSkipDenom(denom)
	oc.resetAlertDenom(denom)
}

func (oc *orderClient) resetSkipDenom(denom string) {
	oc.sdmu.Lock()
	defer oc.sdmu.Unlock()
	_, found := oc.skipDenoms[denom]
	if !found {
		return
	}

	delete(oc.skipDenoms, denom)
}

func (oc *orderClient) resetAlertDenom(denom string) {
	oc.admu.Lock()
	defer oc.admu.Unlock()
	_, found := oc.alertedDenoms[denom]
	if !found {
		return
	}

	delete(oc.alertedDenoms, denom)
}

func (oc *orderClient) alertDenom(ctx context.Context, coin sdk.Coin) {
	if oc.denomAlerted(coin.Denom) {
		return
	}

	oc.admu.Lock()
	defer oc.admu.Unlock()
	oc.alertedDenoms[coin.Denom] = struct{}{}

	if _, err := oc.begOnSlack(ctx, coin, oc.chainID, oc.node); err != nil {
		oc.logger.Error("failed to alert slack", zap.Error(err))
	}
}

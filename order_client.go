package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/slack-go/slack"
	"go.uber.org/zap"

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
	minimumGasBalance sdk.Coin
	disputePeriod     uint64
	alertedLowGas     bool

	account     client.Account
	accountName string

	orderRefreshInterval         time.Duration
	orderFulfillInterval         time.Duration
	orderCleanupInterval         time.Duration
	disputePeriodRefreshInterval time.Duration
}

func newOrderClient(ctx context.Context, config Config) (*orderClient, error) {
	sdkcfg := sdk.GetConfig()
	sdkcfg.SetBech32PrefixForAccount(hubAddressPrefix, pubKeyPrefix)

	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	minimumGasBalance, err := sdk.ParseCoinNormalized(config.MinimumGasBalance)
	if err != nil {
		return nil, fmt.Errorf("failed to parse minimum gas balance: %w", err)
	}

	// init cosmos client
	cosmosClient, err := cosmosclient.New(ctx, getCosmosClientOptions(config)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create cosmos client: %w", err)
	}

	oc := &orderClient{
		client: cosmosClient,
		slack: slacker{
			Client:    slack.New(config.SlackConfig.AppToken),
			channelID: config.SlackConfig.ChannelID,
			enabled:   config.SlackConfig.Enabled,
		},
		accountName:                  config.AccountName,
		demandOrders:                 make(map[string]*demandOrder),
		logger:                       logger,
		chainID:                      cosmosClient.Context().ChainID,
		node:                         config.NodeAddress,
		minimumGasBalance:            minimumGasBalance,
		maxOrdersPerTx:               config.MaxOrdersPerTx,
		orderRefreshInterval:         config.OrderRefreshInterval,
		orderFulfillInterval:         config.OrderFulfillInterval,
		orderCleanupInterval:         config.OrderCleanupInterval,
		disputePeriodRefreshInterval: config.DisputePeriodRefreshInterval,
	}

	return oc, nil
}

func (oc *orderClient) start(ctx context.Context) error {
	// setup account
	if err := oc.setupAccount(); err != nil {
		return fmt.Errorf("failed to setup account: %w", err)
	}

	// subscribe to demand orders
	if err := oc.subscribeToPendingDemandOrders(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to demand orders: %w", err)
	}

	// start workers
	oc.orderRefresher(ctx)
	oc.orderFulfiller(ctx)
	oc.orderCleaner(ctx)
	oc.disputePeriodUpdater(ctx)

	make(chan struct{}) <- struct{}{} // TODO: make nicer

	return nil
}

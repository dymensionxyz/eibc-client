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
	config Config

	chainID string
	node    string

	domu              sync.Mutex
	demandOrders      map[string]*demandOrder // id:demandOrder
	maxOrdersPerTx    int
	minimumGasBalance string
	disputePeriod     uint64

	account     client.Account
	accountName string

	admu          sync.Mutex
	alertedDenoms map[string]struct{}

	orderRefreshInterval         time.Duration
	orderFulfillInterval         time.Duration
	orderCleanupInterval         time.Duration
	disputePeriodRefreshInterval time.Duration
}

func newOrderClient(config Config) (*orderClient, error) {
	sdkcfg := sdk.GetConfig()
	sdkcfg.SetBech32PrefixForAccount(hubAddressPrefix, pubKeyPrefix)

	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	oc := &orderClient{
		// client: nil, set in start()
		config: config,
		slack: slacker{
			Client:    slack.New(config.SlackConfig.BotToken, slack.OptionAppLevelToken(config.SlackConfig.AppToken)),
			channelID: config.SlackConfig.ChannelID,
			enabled:   config.SlackConfig.Enabled,
		},
		accountName:   config.AccountName,
		demandOrders:  make(map[string]*demandOrder),
		alertedDenoms: make(map[string]struct{}),
		logger:        logger,
		// chainID:               "", set in start()
		node:                         config.NodeAddress,
		minimumGasBalance:            config.MinimumGasBalance,
		maxOrdersPerTx:               defaultMaxOrdersPerTx,
		orderRefreshInterval:         defaulOrdertRefreshInterval,
		orderFulfillInterval:         defaultOrderFulfillInterval,
		orderCleanupInterval:         defaultOrderCleanupInterval,
		disputePeriodRefreshInterval: defaultDisputePeriodRefreshInterval,
	}

	return oc, nil
}

func (oc *orderClient) start(ctx context.Context) error {
	// init cosmos client
	cosmosClient, err := cosmosclient.New(ctx, getCosmosClientOptions(oc.config)...)
	if err != nil {
		return fmt.Errorf("failed to create cosmos client: %w", err)
	}

	oc.client = cosmosClient
	oc.chainID = cosmosClient.Context().ChainID

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

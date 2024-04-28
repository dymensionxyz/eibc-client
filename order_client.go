package main

import (
	"context"
	"fmt"
	"math"
	"slices"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/zap"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"
)

type orderClient struct {
	slack  slacker
	logger *zap.Logger
	config Config
	bots   map[string]*bot
	whale  *whale

	orderFetcher *orderFetcher
}

func newOrderClient(ctx context.Context, config Config) (*orderClient, error) {
	sdkcfg := sdk.GetConfig()
	sdkcfg.SetBech32PrefixForAccount(hubAddressPrefix, pubKeyPrefix)

	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	minGasBalance, err := sdk.ParseCoinNormalized(config.MinimumGasBalance)
	if err != nil {
		return nil, fmt.Errorf("failed to parse minimum gas balance: %w", err)
	}

	// init cosmos client
	cosmosClient, err := cosmosclient.New(ctx, getCosmosClientOptions(config)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create cosmos client: %w", err)
	}

	orderCh := make(chan []*demandOrder, newOrderBufferSize)
	failedCh := make(chan []string, newOrderBufferSize) // TODO: make buffer size configurable
	ordFetcher := newOrderFetcher(cosmosClient, config.MaxOrdersPerTx, orderCh, failedCh, logger)
	topUpCh := make(chan topUpRequest, config.NumberOfBots) // TODO: make buffer size configurable
	bin := "dymd"                                           // TODO: from config

	accs, err := getBotAccounts(bin, config.HomeDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get bot accounts: %w", err)
	}

	numFoundBots := len(accs)
	logger.Info("found local bot accounts", zap.Int("accounts", numFoundBots))

	botsAccountsToCreate := int(math.Max(0, float64(config.NumberOfBots)-float64(numFoundBots)))
	newNames, err := createBotAccounts(bin, config.HomeDir, botsAccountsToCreate)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot accounts: %w", err)
	}

	accs = slices.Concat(accs, newNames)

	if len(accs) < config.NumberOfBots {
		return nil, fmt.Errorf("expected %d bot accounts, got %d", config.NumberOfBots, len(accs))
	}

	// create bots
	bots := make(map[string]*bot)
	for i := range config.NumberOfBots {
		b, err := buildBot(ctx, accs[i], logger, config, minGasBalance, orderCh, failedCh, topUpCh)
		if err != nil {
			return nil, fmt.Errorf("failed to create bot: %w", err)
		}
		bots[b.name] = b
	}

	whaleSvc, err := buildWhale(ctx, logger, config, minGasBalance, topUpCh)
	if err != nil {
		return nil, fmt.Errorf("failed to create whale: %w", err)
	}

	oc := &orderClient{
		/*slack: slacker{
			Client:    slack.New(config.SlackConfig.AppToken),
			channelID: config.SlackConfig.ChannelID,
			enabled:   config.SlackConfig.Enabled,
		},*/
		orderFetcher: ordFetcher,
		bots:         bots,
		whale:        whaleSvc,
		config:       config,
		logger:       logger,
	}

	return oc, nil
}

func (oc *orderClient) start(ctx context.Context) error {
	oc.logger.Info("starting demand order fetcher...")

	// start order fetcher
	if err := oc.orderFetcher.start(ctx, oc.config.OrderRefreshInterval, oc.config.OrderCleanupInterval); err != nil {
		return fmt.Errorf("failed to subscribe to demand orders: %w", err)
	}

	// start whale service
	if err := oc.whale.start(ctx); err != nil {
		return fmt.Errorf("failed to start whale service: %w", err)
	}
	//	oc.disputePeriodUpdater(ctx)

	oc.logger.Info("starting bots...")
	// start bots
	for _, b := range oc.bots {
		go func() {
			if err := b.start(ctx); err != nil {
				oc.logger.Error("failed to bot", zap.Error(err))
			}
		}()
	}

	make(chan struct{}) <- struct{}{} // TODO: make nicer

	return nil
}

// add command that creates all the bots to be used?

func buildBot(
	ctx context.Context,
	name string,
	logger *zap.Logger,
	config Config,
	minimumGasBalance sdk.Coin,
	orderCh chan []*demandOrder,
	failedCh chan []string,
	topUpCh chan topUpRequest,
) (*bot, error) {
	cosmosClient, err := cosmosclient.New(ctx, getCosmosClientOptions(config)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create cosmos client for bot: %s;err: %w", name, err)
	}

	accountSvc := newAccountService(
		cosmosClient,
		logger,
		name,
		minimumGasBalance,
		topUpCh,
		withTopUpFactor(config.BotTopUpFactor),
	)

	fulfiller := newOrderFulfiller(accountSvc, cosmosClient, logger)
	b := newBot(name, fulfiller, orderCh, failedCh, logger)

	return b, nil
}

func buildWhale(
	ctx context.Context,
	logger *zap.Logger,
	config Config,
	minimumGasBalance sdk.Coin,
	topUpCh chan topUpRequest,
) (*whale, error) {
	cosmosClient, err := cosmosclient.New(ctx, getCosmosClientOptions(config)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create cosmos client for whale: %w", err)
	}

	accountSvc := newAccountService(cosmosClient, logger, config.WhaleAccountName, minimumGasBalance, topUpCh)

	return newWhale(accountSvc, logger, topUpCh), nil
}

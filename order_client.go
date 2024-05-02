package main

import (
	"context"
	"fmt"
	"os"
	"slices"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"
)

type orderClient struct {
	logger *zap.Logger
	config Config
	bots   map[string]*bot
	whale  *whale

	orderFetcher *orderFetcher
}

func newOrderClient(ctx context.Context, config Config) (*orderClient, error) {
	sdkcfg := sdk.GetConfig()
	sdkcfg.SetBech32PrefixForAccount(hubAddressPrefix, pubKeyPrefix)

	logger, err := buildLogger(config.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	defer logger.Sync() // Ensure all logs are written

	minGasBalance, err := sdk.ParseCoinNormalized(config.MinimumGasBalance)
	if err != nil {
		return nil, fmt.Errorf("failed to parse minimum gas balance: %w", err)
	}

	// init cosmos client for order fetcher
	fetcherClientCfg := clientConfig{
		homeDir:        config.Bots.KeyringDir,
		nodeAddress:    config.NodeAddress,
		gasFees:        config.GasFees,
		gasPrices:      config.GasPrices,
		keyringBackend: config.Bots.KeyringBackend,
	}
	fetcherCosmosClient, err := cosmosclient.New(ctx, getCosmosClientOptions(fetcherClientCfg)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create cosmos client: %w", err)
	}

	orderCh := make(chan []*demandOrder, newOrderBufferSize)
	failedCh := make(chan []string, newOrderBufferSize) // TODO: make buffer size configurable
	ordFetcher := newOrderFetcher(fetcherCosmosClient, config.Bots.MaxOrdersPerTx, orderCh, failedCh, logger)
	topUpCh := make(chan topUpRequest, config.Bots.NumberOfBots) // TODO: make buffer size configurable
	bin := "dymd"                                                // TODO: from config

	slackClient := newSlacker(config.SlackConfig, logger)

	whaleSvc, err := buildWhale(
		ctx,
		logger,
		config.Whale,
		slackClient,
		config.NodeAddress,
		config.GasFees,
		config.GasPrices,
		minGasBalance,
		topUpCh,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create whale: %w", err)
	}

	accs, err := getBotAccounts(bin, config.Bots.KeyringDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get bot accounts: %w", err)
	}

	numFoundBots := len(accs)
	logger.Info("found local bot accounts", zap.Int("accounts", numFoundBots))

	botsAccountsToCreate := max(0, config.Bots.NumberOfBots) - numFoundBots
	if botsAccountsToCreate > 0 {
		logger.Info("creating bot accounts", zap.Int("accounts", botsAccountsToCreate))
	}

	newNames, err := createBotAccounts(bin, config.Bots.KeyringDir, botsAccountsToCreate)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot accounts: %w", err)
	}

	accs = slices.Concat(accs, newNames)

	if len(accs) < config.Bots.NumberOfBots {
		return nil, fmt.Errorf("expected %d bot accounts, got %d", config.Bots.NumberOfBots, len(accs))
	}

	// create bots
	bots := make(map[string]*bot)

	var botIdx int
	for botIdx = range config.Bots.NumberOfBots {
		b, err := buildBot(
			ctx,
			accs[botIdx],
			logger,
			config.Bots,
			config.NodeAddress,
			config.GasFees,
			config.GasPrices,
			minGasBalance,
			orderCh,
			failedCh,
			topUpCh,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create bot: %w", err)
		}
		bots[b.name] = b
	}

	// TODO: move to account.go
	whaleAcc, err := whaleSvc.accountSvc.client.AccountRegistry.GetByName(whaleSvc.accountSvc.accountName)
	if err != nil {
		return nil, fmt.Errorf("failed to get whale account: %w", err)
	}

	whaleAddr, err := whaleAcc.Record.GetAddress()
	if err != nil {
		return nil, fmt.Errorf("failed to get whale address: %w", err)
	}

	refundFromExtraBotsToWhale(ctx, config, botIdx, accs, whaleAddr.String(), config.GasFees, logger)

	oc := &orderClient{
		orderFetcher: ordFetcher,
		bots:         bots,
		whale:        whaleSvc,
		config:       config,
		logger:       logger,
	}

	return oc, nil
}

func refundFromExtraBotsToWhale(
	ctx context.Context,
	config Config,
	botIdx int,
	accs []string,
	whaleAddress string,
	gasFeesStr string,
	logger *zap.Logger,
) {
	refunded := sdk.NewCoins()

	gasFees, err := sdk.ParseCoinNormalized(gasFeesStr)
	if err != nil {
		logger.Error("failed to parse gas fees", zap.Error(err))
		return
	}

	// return funds from extra bots to whale
	for ; botIdx < len(accs); botIdx++ {
		b, err := buildBot(
			ctx,
			accs[botIdx],
			logger,
			config.Bots,
			config.NodeAddress,
			config.GasFees,
			config.GasPrices,
			sdk.Coin{}, nil, nil, nil,
		)
		if err != nil {
			logger.Error("failed to create bot", zap.Error(err))
			continue
		}

		// TODO: if bots have other denoms but low gas, use whale as the fee payer

		botBalances, err := b.fulfiller.accountSvc.getAccountBalances(ctx)
		if err != nil {
			logger.Error("failed to get bot balances", zap.Error(err))
			continue
		}

		if botBalances.AmountOf(gasFees.Denom).LT(gasFees.Amount) {
			continue
		}

		botBalances = botBalances.Sub(gasFees)

		if len(botBalances) == 0 {
			continue
		}

		if err = b.fulfiller.accountSvc.sendCoins(botBalances, whaleAddress); err != nil {
			logger.Error("failed to return funds to whale", zap.Error(err))
			continue
		}

		refunded = refunded.Add(botBalances...)
	}

	logger.Info("refunded funds from extra bots to whale",
		zap.Int("bots", len(accs)-config.Bots.NumberOfBots), zap.String("refunded", refunded.String()))
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
	config botConfig,
	nodeAddress, gasFees, gasPrices string,
	minimumGasBalance sdk.Coin,
	orderCh chan []*demandOrder,
	failedCh chan []string,
	topUpCh chan topUpRequest,
) (*bot, error) {
	clientCfg := clientConfig{
		homeDir:        config.KeyringDir,
		keyringBackend: config.KeyringBackend,
		nodeAddress:    nodeAddress,
		gasFees:        gasFees,
		gasPrices:      gasPrices,
	}

	cosmosClient, err := cosmosclient.New(ctx, getCosmosClientOptions(clientCfg)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create cosmos client for bot: %s;err: %w", name, err)
	}

	accountSvc, err := newAccountService(
		cosmosClient,
		logger,
		name,
		minimumGasBalance,
		topUpCh,
		withTopUpFactor(config.TopUpFactor),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create account service for bot: %s;err: %w", name, err)
	}

	fulfiller := newOrderFulfiller(accountSvc, cosmosClient, logger)
	b := newBot(name, fulfiller, orderCh, failedCh, logger)

	return b, nil
}

func buildWhale(
	ctx context.Context,
	logger *zap.Logger,
	config whaleConfig,
	slack *slacker,
	nodeAddress, gasFees, gasPrices string,
	minimumGasBalance sdk.Coin,
	topUpCh chan topUpRequest,
) (*whale, error) {
	clientCfg := clientConfig{
		homeDir:        config.KeyringDir,
		keyringBackend: config.KeyringBackend,
		nodeAddress:    nodeAddress,
		gasFees:        gasFees,
		gasPrices:      gasPrices,
	}

	balanceThresholdMap := make(map[string]sdk.Coin)
	for denom, threshold := range config.AllowedBalanceThresholds {
		coinStr := threshold + denom
		coin, err := sdk.ParseCoinNormalized(coinStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse threshold coin: %w", err)
		}

		balanceThresholdMap[denom] = coin
	}

	cosmosClient, err := cosmosclient.New(ctx, getCosmosClientOptions(clientCfg)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create cosmos client for whale: %w", err)
	}

	accountSvc, err := newAccountService(cosmosClient, logger, config.AccountName, minimumGasBalance, topUpCh)
	if err != nil {
		return nil, fmt.Errorf("failed to create account service for whale: %w", err)
	}

	return newWhale(
		accountSvc,
		balanceThresholdMap,
		logger,
		slack,
		cosmosClient.Context().ChainID,
		clientCfg.nodeAddress,
		topUpCh,
	), nil
}

func buildLogger(logLevel string) (*zap.Logger, error) {
	var level zapcore.Level
	if err := level.Set(logLevel); err != nil {
		return nil, fmt.Errorf("failed to set log level: %w", err)
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.Lock(os.Stdout),
		level,
	))

	return logger, nil
}

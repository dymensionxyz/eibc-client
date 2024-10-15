package eibc

import (
	"context"
	"fmt"
	"math/rand"
	"slices"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"go.uber.org/zap"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"

	"github.com/dymensionxyz/eibc-client/config"
	"github.com/dymensionxyz/eibc-client/store"
	"github.com/dymensionxyz/eibc-client/types"
)

type orderClient struct {
	logger *zap.Logger
	config config.Config
	bots   map[string]*orderFulfiller
	whale  *whale

	orderEventer *orderEventer
	orderPoller  *orderPoller
	orderTracker *orderTracker
}

func NewOrderClient(ctx context.Context, cfg config.Config, logger *zap.Logger) (*orderClient, error) {
	if cfg.FulfillCriteria.FulfillmentMode.MinConfirmations > len(cfg.FulfillCriteria.FulfillmentMode.FullNodes) {
		return nil, fmt.Errorf("min confirmations cannot be greater than the number of full nodes")
	}

	sdkcfg := sdk.GetConfig()
	sdkcfg.SetBech32PrefixForAccount(config.HubAddressPrefix, config.PubKeyPrefix)

	minGasBalance, err := sdk.ParseCoinNormalized(cfg.Gas.MinimumGasBalance)
	if err != nil {
		return nil, fmt.Errorf("failed to parse minimum gas balance: %w", err)
	}

	//nolint:gosec
	subscriberID := fmt.Sprintf("eibc-client-%d", rand.Int())

	orderCh := make(chan []*demandOrder, config.NewOrderBufferSize)

	db, err := store.NewDB(ctx, cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create db: %w", err)
	}

	fulfilledOrdersCh := make(chan *orderBatch, config.NewOrderBufferSize) // TODO: make buffer size configurable
	bstore := store.NewBotStore(db)

	// deactivate all bots, so that we set active only the ones we will use
	if err := bstore.DeactivateAllBots(ctx); err != nil {
		return nil, fmt.Errorf("failed to deactivate all bots: %w", err)
	}

	hubClient, err := getHubClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create hub client: %w", err)
	}

	fullNodeClient, err := getFullNodeClients(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create full node clients: %w", err)
	}

	// create bots
	bots := make(map[string]*orderFulfiller)

	queryClient := types.NewQueryClient(hubClient.Context())

	ordTracker := newOrderTracker(
		queryClient.StateInfo,
		hubClient.BroadcastTx,
		fullNodeClient,
		bstore,
		fulfilledOrdersCh,
		bots,
		subscriberID,
		cfg.Bots.MaxOrdersPerTx,
		&cfg.FulfillCriteria,
		orderCh,
		logger,
	)

	rollapps := make([]string, 0, len(cfg.FulfillCriteria.MinFeePercentage.Chain))
	for chain := range cfg.FulfillCriteria.MinFeePercentage.Chain {
		rollapps = append(rollapps, chain)
	}

	eventer := newOrderEventer(
		hubClient,
		subscriberID,
		rollapps,
		ordTracker,
		logger,
	)

	topUpCh := make(chan topUpRequest, cfg.Bots.NumberOfBots) // TODO: make buffer size configurable
	slackClient := newSlacker(cfg.SlackConfig, logger)

	whaleSvc, err := buildWhale(
		logger,
		cfg.Whale,
		slackClient,
		cfg.NodeAddress,
		cfg.Gas.Fees,
		cfg.Gas.Prices,
		minGasBalance,
		topUpCh,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create whale: %w", err)
	}

	botClientCfg := config.ClientConfig{
		HomeDir:        cfg.Bots.KeyringDir,
		KeyringBackend: cfg.Bots.KeyringBackend,
		NodeAddress:    cfg.NodeAddress,
		GasFees:        cfg.Gas.Fees,
		GasPrices:      cfg.Gas.Prices,
	}

	accs, err := addBotAccounts(cfg.Bots.NumberOfBots, botClientCfg, logger)
	if err != nil {
		return nil, err
	}

	var botIdx int
	for botIdx = range cfg.Bots.NumberOfBots {
		b, err := buildBot(
			accs[botIdx],
			logger,
			cfg.Bots,
			botClientCfg,
			bstore,
			minGasBalance,
			orderCh,
			fulfilledOrdersCh,
			topUpCh,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create bot: %w", err)
		}
		bots[b.accountSvc.GetAccountName()] = b
	}

	if !cfg.SkipRefund {
		refundFromExtraBotsToWhale(
			ctx,
			bstore,
			cfg,
			botIdx,
			accs,
			whaleSvc.accountSvc,
			cfg.Gas.Fees,
			logger,
		)
	}

	oc := &orderClient{
		orderEventer: eventer,
		orderTracker: ordTracker,
		bots:         bots,
		whale:        whaleSvc,
		config:       cfg,
		logger:       logger,
	}

	if cfg.OrderPolling.Enabled {
		oc.orderPoller = newOrderPoller(
			hubClient.Context().ChainID,
			ordTracker,
			cfg.OrderPolling,
			logger,
		)
	}

	return oc, nil
}

func getHubClient(cfg config.Config) (cosmosclient.Client, error) {
	// init cosmos client for order fetcher
	hubClientCfg := config.ClientConfig{
		HomeDir:        cfg.Bots.KeyringDir,
		NodeAddress:    cfg.NodeAddress,
		GasFees:        cfg.Gas.Fees,
		GasPrices:      cfg.Gas.Prices,
		KeyringBackend: cfg.Bots.KeyringBackend,
	}

	hubClient, err := cosmosclient.New(config.GetCosmosClientOptions(hubClientCfg)...)
	if err != nil {
		return cosmosclient.Client{}, fmt.Errorf("failed to create cosmos client: %w", err)
	}

	return hubClient, nil
}

func getFullNodeClients(cfg config.Config) (*nodeClient, error) {
	var expectedValidationLevel validationLevel

	switch cfg.FulfillCriteria.FulfillmentMode.Level {
	case config.FulfillmentModeSequencer:
		return &nodeClient{}, nil
	case config.FulfillmentModeP2P:
		expectedValidationLevel = validationLevelP2P
	case config.FulfillmentModeSettlement:
		expectedValidationLevel = validationLevelSettlement
	default:
		return nil, fmt.Errorf("unknown fulfillment mode: %s", cfg.FulfillCriteria.FulfillmentMode.Level)
	}

	client, err := newNodeClient(
		cfg.FulfillCriteria.FulfillmentMode.FullNodes,
		expectedValidationLevel,
		cfg.FulfillCriteria.FulfillmentMode.MinConfirmations,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create full node client: %w", err)
	}

	return client, nil
}

func (oc *orderClient) Start(ctx context.Context) error {
	oc.logger.Info("starting demand order fetcher...")
	// start order fetcher
	if err := oc.orderEventer.start(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to demand orders: %w", err)
	}

	// start order polling
	if oc.orderPoller != nil {
		oc.logger.Info("starting order polling...")
		oc.orderPoller.start(ctx)
	}

	// start whale service
	if err := oc.whale.start(ctx); err != nil {
		return fmt.Errorf("failed to start whale service: %w", err)
	}

	oc.logger.Info("starting bots...")

	// start bots
	for _, b := range oc.bots {
		go func() {
			if err := b.start(ctx); err != nil {
				oc.logger.Error("failed to bot", zap.Error(err))
			}
		}()
	}

	if err := oc.orderTracker.start(ctx); err != nil {
		return fmt.Errorf("failed to start order tracker: %w", err)
	}

	return nil
}

func (oc *orderClient) Stop() {
	oc.logger.Info("stopping order client")

	oc.orderTracker.store.Close()
}

func (oc *orderClient) GetWhale() *whale {
	return oc.whale
}

func (oc *orderClient) GetBotStore() botStore {
	return oc.orderTracker.store
}

func addBotAccounts(numBots int, botConfig config.ClientConfig, logger *zap.Logger) ([]string, error) {
	cosmosClient, err := cosmosclient.New(config.GetCosmosClientOptions(botConfig)...)
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

	newNames, err := createBotAccounts(cosmosClient, botsAccountsToCreate)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot accounts: %w", err)
	}

	accs = slices.Concat(accs, newNames)

	if len(accs) < numBots {
		return nil, fmt.Errorf("expected %d bot accounts, got %d", numBots, len(accs))
	}
	return accs, nil
}

func refundFromExtraBotsToWhale(
	ctx context.Context,
	store accountStore,
	cfg config.Config,
	startBotIdx int,
	accs []string,
	whaleAccSvc AccountSvc,
	gasFeesStr string,
	logger *zap.Logger,
) {
	logger.Info("refunding funds from idle bots to whale", zap.String("whale", whaleAccSvc.Address()))

	refunded := sdk.NewCoins()

	gasFees, err := sdk.ParseCoinNormalized(gasFeesStr)
	if err != nil {
		logger.Error("failed to parse gas fees", zap.Error(err))
		return
	}

	botClientCfg := config.ClientConfig{
		HomeDir:        cfg.Bots.KeyringDir,
		KeyringBackend: cfg.Bots.KeyringBackend,
		NodeAddress:    cfg.NodeAddress,
		GasFees:        cfg.Gas.Fees,
		GasPrices:      cfg.Gas.Prices,
	}

	// return funds from extra bots to whale
	for ; startBotIdx < len(accs); startBotIdx++ {
		b, err := buildBot(
			accs[startBotIdx],
			logger,
			cfg.Bots,
			botClientCfg,
			store,
			sdk.Coin{}, nil, nil, nil,
		)
		if err != nil {
			logger.Error("failed to create bot", zap.Error(err))
			continue
		}

		if err := b.accountSvc.UpdateFunds(ctx); err != nil {
			logger.Error("failed to update bot funds", zap.Error(err))
			continue
		}

		if b.accountSvc.GetBalances().IsZero() {
			continue
		}

		// if bot doesn't have enough DYM to pay the fees
		// transfer some from the whale
		if diff := gasFees.Amount.Sub(b.accountSvc.BalanceOf(gasFees.Denom)); diff.IsPositive() {
			logger.Info("topping up bot account", zap.String("bot", b.accountSvc.GetAccountName()),
				zap.String("denom", gasFees.Denom), zap.String("amount", diff.String()))
			topUp := sdk.NewCoin(gasFees.Denom, diff)
			if err := whaleAccSvc.SendCoins(ctx, sdk.NewCoins(topUp), b.accountSvc.Address()); err != nil {
				logger.Error("failed to top up bot account with gas", zap.Error(err))
				continue
			}
			// update bot balance with topped up gas amount
			b.accountSvc.SetBalances(b.accountSvc.GetBalances().Add(topUp))
		}
		// subtract gas fees from bot balance, so there is enough to pay for the fees
		b.accountSvc.SetBalances(b.accountSvc.GetBalances().Sub(gasFees))

		if err = b.accountSvc.SendCoins(ctx, b.accountSvc.GetBalances(), whaleAccSvc.Address()); err != nil {
			logger.Error("failed to return funds to whale", zap.Error(err))
			continue
		}

		if err := b.accountSvc.UpdateFunds(ctx); err != nil {
			logger.Error("failed to update bot funds", zap.Error(err))
			continue
		}

		refunded = refunded.Add(b.accountSvc.GetBalances()...)
	}

	if !refunded.Empty() {
		logger.Info("refunded funds from extra bots to whale",
			zap.Int("bots", len(accs)-cfg.Bots.NumberOfBots), zap.String("refunded", refunded.String()))
	}
}

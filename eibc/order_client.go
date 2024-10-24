package eibc

import (
	"context"
	"fmt"
	"math/rand"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	"go.uber.org/zap"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"

	"github.com/dymensionxyz/eibc-client/config"
)

type orderClient struct {
	logger *zap.Logger
	config config.Config
	bots   map[string]*orderFulfiller

	orderEventer *orderEventer
	orderPoller  *orderPoller
	orderTracker *orderTracker
}

func NewOrderClient(cfg config.Config, logger *zap.Logger) (*orderClient, error) {
	if cfg.Validation.MinConfirmations > len(cfg.Validation.FullNodes) {
		return nil, fmt.Errorf("min confirmations cannot be greater than the number of full nodes")
	}

	sdkcfg := sdk.GetConfig()
	sdkcfg.SetBech32PrefixForAccount(config.HubAddressPrefix, config.PubKeyPrefix)

	//nolint:gosec
	subscriberID := fmt.Sprintf("eibc-client-%d", rand.Int())

	orderCh := make(chan []*demandOrder, config.NewOrderBufferSize)
	fulfilledOrdersCh := make(chan *orderBatch, config.NewOrderBufferSize) // TODO: make buffer size configurable

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

	minOperatorFeeShare, err := sdk.NewDecFromStr(cfg.Operator.MinFeeShare)
	if err != nil {
		return nil, fmt.Errorf("failed to parse min operator fee share: %w", err)
	}

	ordTracker := newOrderTracker(
		hubClient,
		cfg.Bots.PolicyAddress,
		minOperatorFeeShare,
		fullNodeClient,
		fulfilledOrdersCh,
		bots,
		subscriberID,
		cfg.Bots.MaxOrdersPerTx,
		&cfg.Validation,
		orderCh,
		cfg.OrderPolling.Interval, // we can use the same interval for order polling and LP balance checking
		logger,
	)

	eventer := newOrderEventer(
		hubClient,
		subscriberID,
		ordTracker,
		logger,
	)

	operatorClientCfg := config.ClientConfig{
		HomeDir:        cfg.Operator.KeyringDir,
		KeyringBackend: cfg.Operator.KeyringBackend,
		NodeAddress:    cfg.NodeAddress,
		GasFees:        cfg.Gas.Fees,
		GasPrices:      cfg.Gas.Prices,
	}

	operatorClient, err := cosmosclient.New(config.GetCosmosClientOptions(operatorClientCfg)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create cosmos client for bot: %w", err)
	}

	operatorName := cfg.Operator.AccountName
	operatorAddress, err := operatorClient.Address(operatorName)
	if err != nil {
		return nil, err
	}

	botClientCfg := config.ClientConfig{
		HomeDir:        cfg.Bots.KeyringDir,
		KeyringBackend: cfg.Bots.KeyringBackend,
		NodeAddress:    cfg.NodeAddress,
		GasFees:        cfg.Gas.Fees,
		GasPrices:      cfg.Gas.Prices,
		FeeGranter:     operatorAddress.String(),
		FeePayer:       cfg.Bots.PolicyAddress,
	}

	accs, err := addBotAccounts(cfg.Bots.NumberOfBots, botClientCfg, logger)
	if err != nil {
		return nil, err
	}

	activeAccs := make([]account, 0, len(accs))
	granteeAddrs := make([]sdk.AccAddress, 0, len(accs))
	primeAddrs := make([]string, 0, len(accs))

	var botIdx int
	for botIdx = range cfg.Bots.NumberOfBots {
		acc := accs[botIdx]
		exist, err := accountExists(operatorClient.Context(), acc.Address)
		if err != nil {
			return nil, fmt.Errorf("failed to check if bot account exists: %w", err)
		}
		if !exist {
			primeAddrs = append(primeAddrs, acc.Address)
		}

		hasGrant, err := hasFeeGranted(operatorClient, operatorAddress.String(), acc.Address)
		if err != nil {
			return nil, fmt.Errorf("failed to check if fee is granted: %w", err)
		}
		if !hasGrant {
			_, granteeAddr, err := bech32.DecodeAndConvert(acc.Address)
			if err != nil {
				return nil, fmt.Errorf("failed to decode bot address: %w", err)
			}
			granteeAddrs = append(granteeAddrs, granteeAddr)
		}

		b, err := buildBot(
			acc,
			operatorAddress.String(),
			logger,
			cfg.Bots,
			botClientCfg,
			orderCh,
			fulfilledOrdersCh,
			ordTracker.releaseAllReservedOrdersFunds,
			ordTracker.debitAllReservedOrdersFunds,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create bot: %w", err)
		}
		bots[b.account.Name] = b
		activeAccs = append(activeAccs, acc)
	}

	if len(primeAddrs) > 0 {
		logger.Info("priming bot accounts", zap.Strings("addresses", primeAddrs))
		if err = primeAccounts(operatorClient, operatorName, operatorAddress, primeAddrs...); err != nil {
			return nil, fmt.Errorf("failed to prime bot account: %w", err)
		}
	}

	if len(granteeAddrs) > 0 {
		logger.Info("adding fee grant to bot accounts", zap.Strings("addresses", primeAddrs))
		if err = addFeeGrantToBot(operatorClient, operatorName, operatorAddress, granteeAddrs...); err != nil {
			return nil, fmt.Errorf("failed to add grant to bot: %w", err)
		}
	}

	err = addBotsToGroup(operatorName, operatorAddress.String(), cfg.Operator.GroupID, operatorClient, activeAccs)
	if err != nil {
		return nil, err
	}

	oc := &orderClient{
		orderEventer: eventer,
		orderTracker: ordTracker,
		bots:         bots,
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
	switch cfg.Validation.FallbackLevel {
	case config.ValidationModeSequencer:
		return &nodeClient{}, nil
	}

	client, err := newNodeClient(
		cfg.Validation.FullNodes,
		cfg.Validation.MinConfirmations,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create full node client: %w", err)
	}

	return client, nil
}

func (oc *orderClient) Start(ctx context.Context) error {
	oc.logger.Info("starting demand order tracker...")

	if err := oc.orderTracker.start(ctx); err != nil {
		return fmt.Errorf("failed to start order tracker: %w", err)
	}

	// start order fetcher
	oc.logger.Info("starting demand order eventer...")
	if err := oc.orderEventer.start(ctx); err != nil {
		return fmt.Errorf("failed to subscribe to demand orders: %w", err)
	}

	// start order polling
	if oc.orderPoller != nil {
		oc.logger.Info("starting order polling...")
		if err := oc.orderPoller.start(ctx); err != nil {
			return fmt.Errorf("failed to start order polling: %w", err)
		}
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

	<-make(chan bool)

	return nil
}

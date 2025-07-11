package eibc

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/dymensionxyz/cosmosclient/cosmosclient"
	"go.uber.org/zap"

	"github.com/dymensionxyz/eibc-client/config"
)

type orderClient struct {
	logger     *zap.Logger
	config     config.Config
	fulfillers map[string]*orderFulfiller

	orderEventer   *orderEventer
	orderPoller    *orderPoller
	orderTracker   *orderTracker
	orderFinalizer *orderFinalizer
}

func NewOrderClient(cfg config.Config, logger *zap.Logger) (*orderClient, error) {
	sdkcfg := sdk.GetConfig()
	sdkcfg.SetBech32PrefixForAccount(config.HubAddressPrefix, config.PubKeyPrefix)

	//nolint:gosec
	subscriberID := fmt.Sprintf("eibc-client-%d", rand.Int())
	orderCh := make(chan []*demandOrder, config.NewOrderBufferSize)
	processedCh := make(chan []orderFulfillResult, config.NewOrderBufferSize)

	hubClient, err := getHubClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create hub client: %w", err)
	}

	fullNodeClient, err := getFullNodeClients(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create full node clients: %w", err)
	}

	minOperatorFeeShare, err := math.LegacyNewDecFromStr(cfg.Operator.MinFeeShare)
	if err != nil {
		return nil, fmt.Errorf("failed to parse min operator fee share: %w", err)
	}

	tracker := newOrderTracker(
		hubClient,
		cfg.Fulfillers.PolicyAddress,
		minOperatorFeeShare,
		fullNodeClient,
		subscriberID,
		cfg.Fulfillers.Scale,
		cfg.Fulfillers.MaxOrdersPerTx,
		&cfg.Validation,
		orderCh,
		processedCh,
		cfg.OrderPolling.Interval, // we can use the same interval for order polling and LP balance checking
		cfg.Validation.Interval,
		func() {},
		logger,
	)
	oc := &orderClient{
		config:       cfg,
		fulfillers:   make(map[string]*orderFulfiller),
		orderTracker: tracker,
		logger:       logger,
	}

	operatorClientCfg := config.ClientConfig{
		HomeDir:        cfg.Operator.KeyringDir,
		KeyringBackend: cfg.Operator.KeyringBackend,
		NodeAddress:    cfg.NodeAddress,
		GasFees:        cfg.Gas.Fees,
		GasPrices:      cfg.Gas.Prices,
	}

	operatorClient, err := cosmosclient.New(config.GetCosmosClientOptions(operatorClientCfg)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create cosmos client for fulfiller: %w", err)
	}

	operatorName := cfg.Operator.AccountName
	operatorAddress, err := operatorClient.Address(operatorName)
	if err != nil {
		return nil, fmt.Errorf("failed to get operator address: %w", err)
	}

	if cfg.OrderPolling.Enabled {
		var rollapps []string
		for r := range cfg.Rollapps {
			rollapps = append(rollapps, r)
		}
		oc.orderPoller = newOrderPoller(
			hubClient.Context(),
			oc.orderTracker,
			cfg.OrderPolling,
			rollapps,
			operatorAddress.String(),
			logger,
		)
		oc.orderTracker.resetPoller = oc.orderPoller.resetOrderPolling
	}

	oc.orderEventer = newOrderEventer(
		hubClient,
		subscriberID,
		oc.orderTracker,
		logger,
	)

	fulfillerClientCfg := config.ClientConfig{
		HomeDir:        cfg.Fulfillers.KeyringDir,
		KeyringBackend: cfg.Fulfillers.KeyringBackend,
		NodeAddress:    cfg.NodeAddress,
		GasFees:        cfg.Gas.Fees,
		GasPrices:      cfg.Gas.Prices,
		FeeGranter:     operatorAddress.String(),
	}

	accs, err := addFulfillerAccounts(cfg.Fulfillers.Scale, fulfillerClientCfg, logger)
	if err != nil {
		return nil, err
	}

	activeAccs := make([]account, 0, len(accs))
	granteeAddrs := make([]sdk.AccAddress, 0, len(accs))
	primeAddrs := make([]string, 0, len(accs))

	var fulfillerIdx int
	for fulfillerIdx = range cfg.Fulfillers.Scale {
		acc := accs[fulfillerIdx]
		exist, err := accountExists(operatorClient.Context(), acc.Address)
		if err != nil {
			return nil, fmt.Errorf("failed to check if fulfiller account exists: %w", err)
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
				return nil, fmt.Errorf("failed to decode fulfiller address: %w", err)
			}
			granteeAddrs = append(granteeAddrs, granteeAddr)
		}

		cClient, err := cosmosclient.New(config.GetCosmosClientOptions(fulfillerClientCfg)...)
		if err != nil {
			return nil, fmt.Errorf("failed to create cosmos client for fulfiller: %s;err: %w", acc.Name, err)
		}

		f, err := newOrderFulfiller(
			acc,
			operatorAddress.String(),
			logger,
			cfg.Fulfillers.PolicyAddress,
			cClient,
			orderCh,
			processedCh,
			oc.orderTracker.releaseAllReservedOrdersFunds,
			oc.orderTracker.debitAllReservedOrdersFunds,
			cfg.Fulfillers.MaxOrdersPerTx,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create fulfiller: %w", err)
		}
		oc.fulfillers[f.account.Name] = f
		activeAccs = append(activeAccs, acc)
	}

	if len(primeAddrs) > 0 {
		logger.Info("priming fulfiller accounts", zap.Strings("addresses", primeAddrs))
		if err = primeAccounts(operatorClient, operatorName, operatorAddress, primeAddrs...); err != nil {
			return nil, fmt.Errorf("failed to prime fulfiller account: %w", err)
		}
	}

	if len(granteeAddrs) > 0 {
		logger.Info("adding fee grant to fulfiller accounts", zap.Strings("addresses", primeAddrs))
		if err = addFeeGrantToFulfiller(operatorClient, operatorName, operatorAddress, granteeAddrs...); err != nil {
			return nil, fmt.Errorf("failed to add grant to fulfiller: %w", err)
		}
	}

	err = addFulfillersToGroup(operatorName, operatorAddress.String(), cfg.Operator.GroupID, operatorClient, activeAccs)
	if err != nil {
		return nil, err
	}

	oc.orderFinalizer = newFinalizer(
		operatorName,
		operatorAddress.String(),
		oc.orderPoller,
		operatorClient,
	)

	return oc, nil
}

const (
	connectAttempts = 5
	connectSleep    = 10 * time.Second
)

func getHubClient(cfg config.Config) (hubClient cosmosclient.Client, err error) {
	// init cosmos client for order fetcher
	hubClientCfg := config.ClientConfig{
		HomeDir:        cfg.Fulfillers.KeyringDir,
		NodeAddress:    cfg.NodeAddress,
		GasFees:        cfg.Gas.Fees,
		GasPrices:      cfg.Gas.Prices,
		KeyringBackend: cfg.Fulfillers.KeyringBackend,
	}

	err = retry(connectAttempts, connectSleep, func() error {
		var retryErr error
		hubClient, retryErr = cosmosclient.New(config.GetCosmosClientOptions(hubClientCfg)...)
		if retryErr != nil {
			log.Printf("failed to obtain hub client, retrying in 10 seconds: %s", retryErr.Error())
		}
		return retryErr
	})
	if err != nil {
		return cosmosclient.Client{}, fmt.Errorf("failed to create cosmos client after retries: %w", err)
	}

	return hubClient, nil
}

func retry(attempts int, sleep time.Duration, f func() error) (err error) {
	for i := 0; i < attempts; i++ {
		err = f()
		if err == nil {
			return
		}
		if i < attempts-1 {
			time.Sleep(sleep)
		}
	}
	return fmt.Errorf("all retry attempts failed: %w", err)
}

func getFullNodeClients(cfg config.Config) (*nodeClient, error) {
	switch cfg.Validation.FallbackLevel {
	case config.ValidationModeSequencer:
		return &nodeClient{}, nil
	}

	client, err := newNodeClient(cfg.Rollapps)
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

	oc.logger.Info("starting fulfillers...")

	// start fulfillers
	for _, b := range oc.fulfillers {
		go func(b *orderFulfiller) {
			if err := b.start(ctx); err != nil {
				oc.logger.Error("failed to start fulfiller", zap.Error(err))
			}
		}(b)
	}

	if err := oc.orderFinalizer.start(ctx); err != nil {
		oc.logger.Error("failed to start orderFinalizer", zap.Error(err))
	}

	<-make(chan bool)

	return nil
}

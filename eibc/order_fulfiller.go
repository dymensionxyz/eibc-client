package eibc

import (
	"context"
	"fmt"
	"slices"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/dymensionxyz/cosmosclient/cosmosclient"
	"go.uber.org/zap"

	"github.com/dymensionxyz/eibc-client/config"
	"github.com/dymensionxyz/eibc-client/types"
)

type orderFulfiller struct {
	accountSvc          AccountSvc
	client              cosmosClient
	logger              *zap.Logger
	FulfillDemandOrders func(demandOrder ...*demandOrder) error

	newOrdersCh       chan []*demandOrder
	fulfilledOrdersCh chan<- *orderBatch
}

type cosmosClient interface {
	BroadcastTx(accountName string, msgs ...sdk.Msg) (cosmosclient.Response, error)
	Context() client.Context
}

type AccountSvc interface {
	Address() string
	GetAccountName() string
	GetBalances() sdk.Coins
	SetBalances(sdk.Coins)
	EnsureBalances(ctx context.Context, coins sdk.Coins) ([]string, error)
	SendCoins(ctx context.Context, coins sdk.Coins, toAddrStr string) error
	GetAccountBalances(ctx context.Context) (sdk.Coins, error)
	UpdateFunds(ctx context.Context, opts ...fundsOption) error
	BalanceOf(denom string) sdk.Int
	WaitForTx(txHash string) error
	RefreshBalances(ctx context.Context) error
}

func newOrderFulfiller(
	accountSvc AccountSvc,
	newOrdersCh chan []*demandOrder,
	fulfilledOrdersCh chan<- *orderBatch,
	client cosmosClient,
	logger *zap.Logger,
) *orderFulfiller {
	o := &orderFulfiller{
		accountSvc:        accountSvc,
		client:            client,
		fulfilledOrdersCh: fulfilledOrdersCh,
		newOrdersCh:       newOrdersCh,
		logger: logger.With(zap.String("module", "order-fulfiller"),
			zap.String("name", accountSvc.GetAccountName()), zap.String("address", accountSvc.Address())),
	}
	o.FulfillDemandOrders = o.fulfillDemandOrders
	return o
}

// add command that creates all the bots to be used?

func buildBot(
	name string,
	logger *zap.Logger,
	cfg config.BotConfig,
	clientCfg config.ClientConfig,
	store accountStore,
	minimumGasBalance sdk.Coin,
	newOrderCh chan []*demandOrder,
	fulfilledCh chan *orderBatch,
	topUpCh chan topUpRequest,
) (*orderFulfiller, error) {
	cosmosClient, err := cosmosclient.New(config.GetCosmosClientOptions(clientCfg)...)
	if err != nil {
		return nil, fmt.Errorf("failed to create cosmos client for bot: %s;err: %w", name, err)
	}

	as, err := newAccountService(
		cosmosClient,
		store,
		logger,
		name,
		minimumGasBalance,
		topUpCh,
		withTopUpFactor(cfg.TopUpFactor),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create account service for bot: %s;err: %w", name, err)
	}

	return newOrderFulfiller(as, newOrderCh, fulfilledCh, cosmosClient, logger), nil
}

func (ol *orderFulfiller) start(ctx context.Context) error {
	if err := ol.accountSvc.UpdateFunds(ctx); err != nil {
		return fmt.Errorf("failed to update account funds: %w", err)
	}

	ol.logger.Info("starting fulfiller...", zap.String("balances", ol.accountSvc.GetBalances().String()))

	ol.fulfillOrders(ctx)
	return nil
}

func (ol *orderFulfiller) fulfillOrders(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case orders := <-ol.newOrdersCh:
			if err := ol.processBatch(ctx, orders); err != nil {
				ol.logger.Error("failed to process batch", zap.Error(err))
			}
		}
	}
}

func (ol *orderFulfiller) processBatch(ctx context.Context, batch []*demandOrder) error {
	var (
		ids          []string
		demandOrders []*demandOrder
	)

	coins := sdk.NewCoins()

	for _, order := range batch {
		coins = coins.Add(order.amount...)
		coins = coins.Sub(order.fee...)
	}

	ol.logger.Debug("ensuring balances for orders")

	ensuredDenoms, err := ol.accountSvc.EnsureBalances(ctx, coins)
	if err != nil {
		return fmt.Errorf("failed to ensure balances: %w", err)
	}

	if len(ensuredDenoms) > 0 {
		ol.logger.Info("ensured balances for orders", zap.Strings("denoms", ensuredDenoms))
	}

	leftoverBatch := make([]string, 0, len(batch))
	demandOrders = make([]*demandOrder, 0, len(batch))

outer:
	for _, order := range batch {
		for _, price := range order.amount {
			if !slices.Contains(ensuredDenoms, price.Denom) {
				leftoverBatch = append(leftoverBatch, order.id)
				continue outer
			}

			ids = append(ids, order.id)
			demandOrders = append(demandOrders, order)
		}
	}

	if len(ids) == 0 {
		ol.logger.Debug(
			"no orders to fulfill",
			zap.String("bot-name", ol.accountSvc.GetAccountName()),
			zap.Int("leftover count", len(leftoverBatch)),
		)
		return nil
	}

	ol.logger.Info("fulfilling orders", zap.Int("count", len(ids)))

	if err := ol.FulfillDemandOrders(demandOrders...); err != nil {
		return fmt.Errorf("failed to fulfill orders: ids: %v; %w", ids, err)
	}

	ol.logger.Info("orders fulfilled", zap.Int("count", len(ids)))

	go func() {
		if len(ids) == 0 {
			return
		}

		fees := sdk.Coins{}

		for _, order := range batch {
			if slices.Contains(ids, order.id) {
				fees = fees.Add(order.fee...)
			}
		}

		// TODO: check if balances get updated before the new batch starts processing
		if err := ol.accountSvc.UpdateFunds(ctx, addFeeEarnings(fees)); err != nil {
			ol.logger.Error("failed to refresh balances", zap.Error(err))
		}

		ol.fulfilledOrdersCh <- &orderBatch{
			orders:    demandOrders,
			fulfiller: ol.accountSvc.Address(),
		}
	}()

	return nil
}

func (ol *orderFulfiller) fulfillDemandOrders(demandOrder ...*demandOrder) error {
	msgs := make([]sdk.Msg, len(demandOrder))

	for i, order := range demandOrder {
		msgs[i] = types.NewMsgFulfillOrder(ol.accountSvc.Address(), order.id, order.feeStr)
	}

	rsp, err := ol.client.BroadcastTx(ol.accountSvc.GetAccountName(), msgs...)
	if err != nil {
		return fmt.Errorf("failed to broadcast tx: %w", err)
	}

	ol.logger.Info("broadcast tx", zap.String("tx-hash", rsp.TxHash))

	if err = ol.accountSvc.WaitForTx(rsp.TxHash); err != nil {
		return fmt.Errorf("failed to wait for tx: %w", err)
	}

	return nil
}

package eibc

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/dymensionxyz/cosmosclient/cosmosclient"

	"github.com/stretchr/testify/require"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"

	"github.com/dymensionxyz/eibc-client/config"
)

func TestOrderClient(t *testing.T) {
	tests := []struct {
		name                              string
		config                            config.Config
		hubClient                         mockNodeClient
		fullNodeClient                    *nodeClient
		pollOrders                        []Order
		eventOrders                       []Order
		expectBotBalanceAfterFulfill      sdk.Coins
		expectOperatorBalanceAfterFulfill sdk.Coins
		expectBotBalanceAfterFinalize     sdk.Coins
		expectBotClaimableEarnings        sdk.Coins
		expectedValidationLevel           validationLevel
		expectFulfilledOrderIDs           []string
	}{
		{
			name: "sequencer mode, orders from poller: fulfilled",
			config: config.Config{
				OrderPolling: config.OrderPollingConfig{
					Interval: time.Second,
					Enabled:  true,
				},
				Operator: config.OperatorConfig{
					AllowedBalanceThresholds: map[string]string{
						"stake": "1000",
					},
				},
				Bots: config.BotConfig{
					NumberOfBots:   1,
					MaxOrdersPerTx: 10,
				},
			},
			hubClient: mockNodeClient{
				currentBlockHeight: 1,
			},
			fullNodeClient: nil,
			pollOrders: []Order{
				{
					EibcOrderId: "order1",
					Amount:      "100",
					Fee:         "10stake",
					RollappId:   "rollapp1",
					PacketKey:   "packet-key-1",
					BlockHeight: "1",
				}, {
					EibcOrderId: "order2",
					Amount:      "450",
					Fee:         "25stake",
					RollappId:   "rollapp1",
					PacketKey:   "packet-key-2",
					BlockHeight: "1",
				},
			},
			expectBotBalanceAfterFulfill:      sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(0))),
			expectOperatorBalanceAfterFulfill: sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(485))),
			expectBotBalanceAfterFinalize:     sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(550))),
			expectBotClaimableEarnings:        sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(35))),
			expectFulfilledOrderIDs:           []string{"order1", "order2"},
		}, {
			name: "p2p mode, orders from events: fulfilled",
			config: config.Config{
				Operator: config.OperatorConfig{
					AllowedBalanceThresholds: map[string]string{
						"stake": "1000",
					},
				},
				Bots: config.BotConfig{
					NumberOfBots:   1,
					MaxOrdersPerTx: 10,
				},
			},
			operatorBalance: sdk.NewCoins(
				sdk.NewCoin("stake", sdk.NewInt(1000)),
			),
			hubClient: mockNodeClient{
				currentBlockHeight: 1,
			},
			fullNodeClient: &nodeClient{
				locations:             []string{"location1", "location2", "location3"},
				minimumValidatedNodes: 2,
				get: mockValidGet(map[string]map[int64]*blockValidatedResponse{
					"location1": {
						1: {Result: validationLevelP2P},
					},
					"location2": {
						1: {Result: validationLevelP2P},
					},
					"location3": {
						1: {Result: validationLevelNone},
					},
				}),
			},
			eventOrders: []Order{
				{
					EibcOrderId: "order1",
					Amount:      "100stake",
					Fee:         "10stake",
					RollappId:   "rollapp1",
					PacketKey:   "packet-key-1",
					BlockHeight: "1",
				}, {
					EibcOrderId: "order2",
					Amount:      "450stake",
					Fee:         "25stake",
					RollappId:   "rollapp1",
					PacketKey:   "packet-key-2",
					BlockHeight: "1",
				},
			},
			expectBotBalanceAfterFulfill:      sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(0))),
			expectOperatorBalanceAfterFulfill: sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(485))),
			expectBotBalanceAfterFinalize:     sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(550))),
			expectBotClaimableEarnings:        sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(35))),
			expectFulfilledOrderIDs:           []string{"order1", "order2"},
			expectedValidationLevel:           validationLevelP2P,
		}, {
			name: "settlement mode, orders from events: fulfilled",
			config: config.Config{
				Operator: config.OperatorConfig{
					AllowedBalanceThresholds: map[string]string{
						"stake": "1000",
					},
				},
				Bots: config.BotConfig{
					NumberOfBots:   1,
					MaxOrdersPerTx: 10,
				},
			},
			operatorBalance: sdk.NewCoins(
				sdk.NewCoin("stake", sdk.NewInt(1000)),
			),
			hubClient: mockNodeClient{
				currentBlockHeight: 1,
			},
			fullNodeClient: &nodeClient{
				locations:             []string{"location1", "location2", "location3"},
				minimumValidatedNodes: 2,
				get: mockValidGet(map[string]map[int64]*blockValidatedResponse{
					"location1": {
						1: {Result: validationLevelSettlement},
					},
					"location2": {
						1: {Result: validationLevelP2P},
					},
					"location3": {
						1: {Result: validationLevelSettlement},
					},
				}),
			},
			eventOrders: []Order{
				{
					EibcOrderId: "order1",
					Amount:      "100stake",
					Fee:         "10stake",
					RollappId:   "rollapp1",
					PacketKey:   "packet-key-1",
					BlockHeight: "1",
				}, {
					EibcOrderId: "order2",
					Amount:      "450stake",
					Fee:         "25stake",
					RollappId:   "rollapp1",
					PacketKey:   "packet-key-2",
					BlockHeight: "1",
				},
			},
			expectBotBalanceAfterFulfill:      sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(0))),
			expectOperatorBalanceAfterFulfill: sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(485))),
			expectBotBalanceAfterFinalize:     sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(550))),
			expectBotClaimableEarnings:        sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(35))),
			expectFulfilledOrderIDs:           []string{"order1", "order2"},
			expectedValidationLevel:           validationLevelSettlement,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oc, err := setupTestOrderClient(
				tt.config,
				mockGetPollerOrders(tt.pollOrders),
				tt.hubClient,
				tt.fullNodeClient,
			)
			require.NoError(t, err)

			go func() {
				err = oc.Start(context.Background())
				require.NoError(t, err)
			}()

			if len(tt.eventOrders) > 0 {
				for _, order := range tt.eventOrders {
					oc.orderEventer.eventClient.(*mockNodeClient).addOrderCh <- coretypes.ResultEvent{
						Events: map[string][]string{
							createdEvent + ".order_id":      {order.EibcOrderId},
							createdEvent + ".price":         {order.Amount},
							createdEvent + ".packet_status": {"PENDING"},
							createdEvent + ".fee":           {order.Fee},
							createdEvent + ".rollapp_id":    {order.RollappId},
							createdEvent + ".packet_key":    {order.PacketKey},
							createdEvent + ".proof_height":  {order.BlockHeight},
						},
					}
				}
			}

			// wait a bit for the client to fulfill orders
			time.Sleep(2 * time.Second)

			// ======= after fulfilling =========
			// check operator balance
			operatorBalanceAfterFulfill := oc.operator.accountSvc.GetBalances()
			require.Equal(t, tt.expectOperatorBalanceAfterFulfill.String(), operatorBalanceAfterFulfill.String())

			// check pool: should be empty
			require.Empty(t, oc.orderTracker.pool.orders)

			// TODO: check LP balance
		})
	}
}

func setupTestOrderClient(
	cfg config.Config,
	pollOrders func() ([]Order, error),
	hubClient mockNodeClient,
	fullNodeClient *nodeClient,
) (*orderClient, error) {
	logger, _ := zap.NewDevelopment()
	orderCh := make(chan []*demandOrder, config.NewOrderBufferSize)
	fulfilledOrdersCh := make(chan *orderBatch, config.NewOrderBufferSize)

	// tracker
	trackerClient := hubClient

	// bots
	bots := make(map[string]*orderFulfiller)

	ordTracker := newOrderTracker(
		trackerClient,
		"policyAddress",
		sdk.NewDec(1),
		fullNodeClient,
		fulfilledOrdersCh,
		bots,
		"chainID",
		cfg.Bots.MaxOrdersPerTx,
		&cfg.Validation,
		orderCh,
		logger,
	)
	ordTracker.finalizedCheckerInterval = time.Second // override interval for testing

	// eventer
	eventerClient := hubClient
	eventerClient.finalizeOrderCh = make(chan coretypes.ResultEvent, 1)
	eventerClient.addOrderCh = make(chan coretypes.ResultEvent, 1)
	eventerClient.stateInfoCh = make(chan coretypes.ResultEvent, 1)

	eventer := newOrderEventer(
		cosmosclient.Client{
			RPC:      &eventerClient,
			WSEvents: &eventerClient,
		},
		"",
		ordTracker,
		logger,
	)

	operatorClient := hubClient

	chainID := "test-chain-id"

	balanceThresholdMap := make(map[string]sdk.Coin)
	for denom, threshold := range cfg.Operator.AllowedBalanceThresholds {
		coinStr := threshold + denom
		coin, err := sdk.ParseCoinNormalized(coinStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse threshold coin: %w", err)
		}
		balanceThresholdMap[denom] = coin
	}

	// operator
	operatorSvc := newOperator(
		balanceThresholdMap,
		logger,
		chainID,
		"",
	)

	for range cfg.Bots.NumberOfBots {
		botName := fmt.Sprintf("bot-%d", 0)

		hc := hubClient
		acc := account{
			Name:    botName,
			Address: "bot-address",
		}
		b := newOrderFulfiller(orderCh, fulfilledOrdersCh, &hc, acc, "", "", logger)
		b.FulfillDemandOrders = func(demandOrder ...*demandOrder) error {
			return nil
		}
		bots[b.account.Name] = b
	}

	// poller
	var poller *orderPoller
	if cfg.OrderPolling.Enabled {
		poller = newOrderPoller(
			chainID,
			ordTracker,
			cfg.OrderPolling,
			logger,
		)
		poller.getOrders = pollOrders
	}

	// order client
	oc := &orderClient{
		orderEventer: eventer,
		orderTracker: ordTracker,
		bots:         bots,
		operator:     operatorSvc,
		config:       cfg,
		logger:       logger,
		orderPoller:  poller,
	}

	return oc, nil
}

func mockGetPollerOrders(orders []Order) func() ([]Order, error) {
	return func() ([]Order, error) {
		// after polling once, remove orders
		defer func() {
			orders = nil
		}()
		return orders, nil
	}
}

func mockValidGet(resps map[string]map[int64]*blockValidatedResponse) getFn {
	return func(ctx context.Context, urlStr string) (*blockValidatedResponse, error) {
		for loc, rspMap := range resps {
			if strings.Contains(urlStr, loc) {
				u, _ := url.Parse(urlStr)
				h, _ := strconv.ParseInt(u.Query().Get("height"), 10, 64)

				if rspMap[h] != nil {
					return rspMap[h], nil
				}
			}
		}
		return &blockValidatedResponse{
			Result: validationLevelNone,
		}, nil
	}
}

type mockNodeClient struct {
	rpcclient.Client
	finalizeOrderCh    chan coretypes.ResultEvent
	addOrderCh         chan coretypes.ResultEvent
	stateInfoCh        chan coretypes.ResultEvent
	currentBlockHeight int64
}

func (m *mockNodeClient) Start() error { return nil }

func (m *mockNodeClient) Context() client.Context {
	return client.Context{}
}

func (m *mockNodeClient) BroadcastTx(_ string, msgs ...sdk.Msg) (cosmosclient.Response, error) {
	return cosmosclient.Response{
		TxResponse: &sdk.TxResponse{},
	}, nil
}

func (m *mockNodeClient) Subscribe(_ context.Context, _ string, query string, _ ...int) (out <-chan coretypes.ResultEvent, err error) {
	switch query {
	case fmt.Sprintf("%s.is_fulfilled='true' AND %s.new_packet_status='FINALIZED'", finalizedEvent, finalizedEvent):
		return m.finalizeOrderCh, nil
	case fmt.Sprintf("%s.is_fulfilled='false'", createdEvent):
		return m.addOrderCh, nil
	default:
		if strings.Contains(query, stateInfoEvent) {
			return m.stateInfoCh, nil
		}
	}
	return nil, fmt.Errorf("invalid query")
}

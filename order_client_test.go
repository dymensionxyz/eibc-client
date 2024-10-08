package main

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/dymensionxyz/cosmosclient/cosmosclient"
	"github.com/stretchr/testify/require"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/dymensionxyz/eibc-client/store"
)

func TestOrderClient(t *testing.T) {
	tests := []struct {
		name                           string
		config                         Config
		store                          *mockStore
		whaleBalance                   sdk.Coins
		hubClient                      mockNodeClient
		fullNodeClients                []rpcclient.Client
		pollOrders                     []Order
		eventOrders                    []Order
		expectBotBalanceAfterFulfill   sdk.Coins
		expectWhaleBalanceAfterFulfill sdk.Coins
		expectBotBalanceAfterFinalize  sdk.Coins
		expectFulfilledOrderIDs        []string
	}{
		{
			name: "sequencer mode, orders from poller: fulfilled",
			config: Config{
				OrderPolling: OrderPollingConfig{
					Interval: time.Second,
				},
				Whale: whaleConfig{
					AllowedBalanceThresholds: map[string]string{
						"stake": "1000",
					},
				},
				Bots: botConfig{
					NumberOfBots:   1,
					TopUpFactor:    1,
					MaxOrdersPerTx: 10,
				},
				DisputePeriodInBlocks: 10,
				FulfillCriteria: fulfillCriteria{
					MinFeePercentage: minFeePercentage{
						Asset: map[string]float32{
							"stake": 0.1,
						},
						Chain: map[string]float32{
							"rollapp1": 0.1,
						},
					},
					FulfillmentMode: fulfillmentMode{
						Level: fulfillmentModeSequencer,
					},
				},
			},
			store: &mockStore{},
			whaleBalance: sdk.NewCoins(
				sdk.NewCoin("stake", sdk.NewInt(1000)),
			),
			hubClient: mockNodeClient{
				successfulAttempt:  0,
				chainID:            "dymension",
				currentBlockHeight: 1,
			},
			fullNodeClients: []rpcclient.Client{},
			pollOrders: []Order{
				{
					EibcOrderId: "order1",
					Amount:      "100",
					Fee:         "10stake",
					RollappId:   "rollapp1",
					BlockHeight: "1",
				}, {
					EibcOrderId: "order2",
					Amount:      "450",
					Fee:         "25stake",
					RollappId:   "rollapp1",
					BlockHeight: "1",
				},
			},
			expectBotBalanceAfterFulfill:   sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(0))),
			expectWhaleBalanceAfterFulfill: sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(485))),
			expectBotBalanceAfterFinalize:  sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(550))),
			expectFulfilledOrderIDs:        []string{"order1", "order2"},
		}, {
			name: "p2p mode, orders from events: fulfilled",
			config: Config{
				OrderPolling: OrderPollingConfig{
					Interval: time.Second,
				},
				Whale: whaleConfig{
					AllowedBalanceThresholds: map[string]string{
						"stake": "1000",
					},
				},
				Bots: botConfig{
					NumberOfBots:   1,
					TopUpFactor:    1,
					MaxOrdersPerTx: 10,
				},
				DisputePeriodInBlocks: 10,
				FulfillCriteria: fulfillCriteria{
					MinFeePercentage: minFeePercentage{
						Asset: map[string]float32{
							"stake": 0.1,
						},
						Chain: map[string]float32{
							"rollapp1": 0.1,
						},
					},
					FulfillmentMode: fulfillmentMode{
						Level:              fulfillmentModeP2P,
						MinConfirmations:   1,
						ValidationWaitTime: 5 * time.Second,
					},
				},
			},
			store: &mockStore{},
			whaleBalance: sdk.NewCoins(
				sdk.NewCoin("stake", sdk.NewInt(1000)),
			),
			hubClient: mockNodeClient{
				successfulAttempt:  0,
				chainID:            "dymension",
				currentBlockHeight: 1,
			},
			fullNodeClients: []rpcclient.Client{
				&mockNodeClient{
					successfulAttempt: 0,
					chainID:           "rollapp-fullnode",
					blocks: map[int64]*coretypes.ResultBlock{
						1: {},
					},
				},
			},
			eventOrders: []Order{
				{
					EibcOrderId: "order1",
					Amount:      "100stake",
					Fee:         "10stake",
					RollappId:   "rollapp1",
					BlockHeight: "1",
				}, {
					EibcOrderId: "order2",
					Amount:      "450stake",
					Fee:         "25stake",
					RollappId:   "rollapp1",
					BlockHeight: "1",
				},
			},
			expectBotBalanceAfterFulfill:   sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(0))),
			expectWhaleBalanceAfterFulfill: sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(485))),
			expectBotBalanceAfterFinalize:  sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(550))),
			expectFulfilledOrderIDs:        []string{"order1", "order2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oc, err := setupTestOrderClient(
				tt.config,
				tt.store,
				tt.whaleBalance,
				mockGetPollerOrders(tt.pollOrders),
				tt.hubClient,
				tt.fullNodeClients...,
			)
			require.NoError(t, err)

			go func() {
				err = oc.start(context.Background())
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
							createdEvent + ".block_height":  {order.BlockHeight},
						},
					}
				}
			}

			// wait a bit for the client to fulfill orders
			time.Sleep(2 * time.Second)

			// ======= after fulfilling =========

			// check bot balance
			balanceAfterFulfill := oc.bots["bot-0"].accountSvc.getBalances()
			require.Equal(t, tt.expectBotBalanceAfterFulfill.String(), balanceAfterFulfill.String())

			// check whale balance
			whaleBalanceAfterFulfill := oc.whale.accountSvc.getBalances()
			require.Equal(t, tt.expectWhaleBalanceAfterFulfill.String(), whaleBalanceAfterFulfill.String())

			// check order store
			ctx := context.Background()
			orders, err := oc.orderTracker.store.GetOrders(ctx)
			require.NoError(t, err)

			ids := make([]string, 0, len(orders))
			for _, order := range orders {
				// check if order is pending finalization
				require.Equal(t, order.Status, store.OrderStatusPendingFinalization)
				// check if order exists in fulfilled map
				_, ok := oc.orderTracker.fulfilledOrders[order.ID]
				require.True(t, ok)
				ids = append(ids, order.ID)
			}
			require.ElementsMatch(t, tt.expectFulfilledOrderIDs, ids)

			// check pool
			require.Empty(t, oc.orderTracker.pool.orders)
			// ===================================

			// finalize orders

			// set current block height to 11
			// this is optional, as we wait for an event to finalize orders
			oc.orderTracker.hubClient.(*mockNodeClient).currentBlockHeight = 11

			for _, id := range ids {
				oc.orderTracker.hubClient.(*mockNodeClient).finalizeOrderCh <- coretypes.ResultEvent{
					Events: map[string][]string{
						finalizedEvent + ".order_id": {id},
					},
				}
			}

			// wait a bit for the client to finalize orders
			time.Sleep(2 * time.Second)

			// =========== after finalizing =========

			// check bot balance
			balanceAfterFinalize := oc.bots["bot-0"].accountSvc.getBalances()
			require.Equal(t, tt.expectBotBalanceAfterFinalize.String(), balanceAfterFinalize.String())

			// check store orders
			orders, err = oc.orderTracker.store.GetOrders(ctx)
			require.NoError(t, err)
			require.Empty(t, orders)

			oc.stop()
		})
	}
}

func setupTestOrderClient(
	config Config,
	bstore *mockStore,
	whaleBalance sdk.Coins,
	pollOrders func() ([]Order, error),
	hubClient mockNodeClient,
	fullNodeClients ...rpcclient.Client,
) (*orderClient, error) {
	if config.FulfillCriteria.FulfillmentMode.MinConfirmations > len(fullNodeClients) {
		return nil, fmt.Errorf("min confirmations cannot be greater than the number of full nodes")
	}

	logger, _ := zap.NewDevelopment()
	orderCh := make(chan []*demandOrder, newOrderBufferSize)
	fulfilledOrdersCh := make(chan *orderBatch, newOrderBufferSize)

	// tracker
	trackerClient := hubClient
	trackerClient.finalizeOrderCh = make(chan coretypes.ResultEvent, 1)

	bstore.bots = make(map[string]*store.Bot)

	// bots
	bots := make(map[string]*orderFulfiller)

	ordTracker := newOrderTracker(
		&trackerClient,
		trackerClient.BroadcastTx,
		fullNodeClients,
		bstore,
		fulfilledOrdersCh,
		bots,
		"",
		config.Bots.MaxOrdersPerTx,
		config.DisputePeriodInBlocks,
		&config.FulfillCriteria,
		orderCh,
		logger,
	)
	ordTracker.finalizedCheckerInterval = time.Second // override interval for testing

	// eventer
	eventerClient := hubClient
	eventerClient.addOrderCh = make(chan coretypes.ResultEvent, 1)

	eventer := newOrderEventer(
		cosmosclient.Client{
			RPC:      &eventerClient,
			WSEvents: &eventerClient,
		},
		"",
		ordTracker,
		logger,
	)

	topUpCh := make(chan topUpRequest, 1)

	whaleClient := hubClient

	was := &accountService{
		balances:    whaleBalance,
		accountName: "whale",
		client:      &whaleClient,
		account:     client.TestAccount{Address: sdk.AccAddress("whale")},
		asyncClient: true,
		logger:      logger,
	}
	bc := &mockBankClient{
		accountService: was,
	}
	was.bankClient = bc
	accountBalances[was.address()] = whaleBalance

	chainID := "test-chain-id"

	balanceThresholdMap := make(map[string]sdk.Coin)
	for denom, threshold := range config.Whale.AllowedBalanceThresholds {
		coinStr := threshold + denom
		coin, err := sdk.ParseCoinNormalized(coinStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse threshold coin: %w", err)
		}
		balanceThresholdMap[denom] = coin
	}

	// whale
	whaleSvc := newWhale(
		was,
		balanceThresholdMap,
		logger,
		&slacker{
			logger: logger,
		},
		chainID,
		"",
		topUpCh,
	)

	for range config.Bots.NumberOfBots {
		botName := fmt.Sprintf("bot-%d", 0)
		as := &accountService{
			accountName: botName,
			topUpCh:     topUpCh,
			store:       bstore,
			account:     client.TestAccount{Address: sdk.AccAddress(botName)},
			asyncClient: true,
			logger:      logger,
		}
		bc := &mockBankClient{
			accountService: as,
		}
		as.bankClient = bc
		hc := hubClient
		as.client = &hc
		b := newOrderFulfiller(as, orderCh, fulfilledOrdersCh, &hc, logger)
		b.FulfillDemandOrders = bc.mockFulfillDemandOrders
		bots[b.accountSvc.getAccountName()] = b
	}

	// poller
	poller := newOrderPoller(
		chainID,
		ordTracker,
		config.OrderPolling,
		logger,
	)
	poller.getOrders = pollOrders

	// order client
	oc := &orderClient{
		orderEventer: eventer,
		orderTracker: ordTracker,
		bots:         bots,
		whale:        whaleSvc,
		config:       config,
		logger:       logger,
		orderPoller:  poller,
		stopCh:       make(chan struct{}),
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

type mockNodeClient struct {
	rpcclient.Client
	sender             string
	successfulAttempt  int
	attemptCounter     int
	chainID            string
	blocks             map[int64]*coretypes.ResultBlock
	finalizeOrderCh    chan coretypes.ResultEvent
	addOrderCh         chan coretypes.ResultEvent
	currentBlockHeight int64
}

func (m *mockNodeClient) Start() error { return nil }

func (m *mockNodeClient) Context() client.Context {
	return client.Context{}
}

var accountBalances = make(map[string]sdk.Coins)

func (m *mockNodeClient) BroadcastTx(_ string, msgs ...sdk.Msg) (cosmosclient.Response, error) {
	for _, msg := range msgs {
		switch m := msg.(type) {
		case *types.MsgSend:
			for _, coin := range m.Amount {
				if accountBalances[m.FromAddress].AmountOf(coin.Denom).LT(coin.Amount) {
					return cosmosclient.Response{}, fmt.Errorf("insufficient balance")
				}
				accountBalances[m.FromAddress] = accountBalances[m.FromAddress].Sub(coin)
				accountBalances[m.ToAddress] = accountBalances[m.ToAddress].Add(coin)
			}
		}
	}
	return cosmosclient.Response{
		TxResponse: &sdk.TxResponse{},
	}, nil
}

func (m *mockNodeClient) Subscribe(_ context.Context, _ string, query string, _ ...int) (out <-chan coretypes.ResultEvent, err error) {
	switch query {
	case fmt.Sprintf("%s.is_fulfilled='true' AND %s.new_packet_status='FINALIZED'", finalizedEvent, finalizedEvent):
		return m.finalizeOrderCh, nil
	case createdEvent + ".is_fulfilled='false'":
		return m.addOrderCh, nil
	}
	return nil, fmt.Errorf("invalid query")
}

func (m *mockNodeClient) Block(_ context.Context, h *int64) (*coretypes.ResultBlock, error) {
	if h == nil {
		return &coretypes.ResultBlock{
			Block: &tmtypes.Block{
				Header: tmtypes.Header{
					Height: m.currentBlockHeight,
				},
			},
		}, nil
	}
	if len(m.blocks) == 0 {
		return nil, fmt.Errorf("no block")
	}
	m.attemptCounter++
	if m.attemptCounter < m.successfulAttempt {
		return nil, fmt.Errorf("failed to get block")
	}
	return m.blocks[*h], nil
}

func transformFullNodeClients(clients []*mockNodeClient) []rpcclient.Client {
	var c []rpcclient.Client
	for _, cl := range clients {
		c = append(c, cl)
	}
	return c
}

type mockStore struct {
	botStore
	bots   map[string]*store.Bot
	orders []*store.Order
	mu     sync.Mutex
}

func (m *mockStore) GetBot(_ context.Context, name string, opts ...store.BotOption) (*store.Bot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.bots[name], nil
}
func (m *mockStore) SaveBot(_ context.Context, bot *store.Bot) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.bots[bot.Address] = bot
	return nil
}

func (m *mockStore) GetOrders(context.Context, ...store.OrderOption) ([]*store.Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.orders, nil
}

func (m *mockStore) GetOrder(_ context.Context, id string) (*store.Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var order *store.Order
	for _, o := range m.orders {
		if o.ID == id {
			order = o
			break
		}
	}
	return order, nil
}

func (m *mockStore) UpdateManyOrders(_ context.Context, orders []*store.Order) error { return nil }
func (m *mockStore) SaveManyOrders(_ context.Context, orders []*store.Order) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.orders = append(m.orders, orders...)
	return nil
}
func (m *mockStore) DeleteOrder(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.orders = slices.DeleteFunc(m.orders, func(o *store.Order) bool {
		return o.ID == id
	})

	return nil
}

type mockBankClient struct {
	*accountService
}

func (m *mockBankClient) SpendableBalances(_ context.Context, in *types.QuerySpendableBalancesRequest, _ ...grpc.CallOption) (*types.QuerySpendableBalancesResponse, error) {
	return &types.QuerySpendableBalancesResponse{
		Balances: accountBalances[in.Address],
	}, nil
}

func (m *mockBankClient) mockFulfillDemandOrders(demandOrder ...*demandOrder) error {
	balances := m.getBalances()

	for _, order := range demandOrder {
		coins := order.amount.Sub(order.fee...)
		balances = balances.Sub(coins...)

		m.setBalances(balances)
		accountBalances[m.address()] = balances
	}
	return nil
}

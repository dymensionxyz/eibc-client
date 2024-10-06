package main

import (
	"context"
	"fmt"
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
			name: "sequencer mode, order from poller: fulfilled",
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
						Level: "sequencer",
						// MinConfirmations:   0,
						// ValidationWaitTime: 0,
					},
				},
			},
			store: &mockStore{},
			whaleBalance: sdk.NewCoins(
				sdk.NewCoin("stake", sdk.NewInt(1000)),
			),
			hubClient: mockNodeClient{
				successfulAttempt: 0,
				chainID:           "dymension",
				blocks:            nil,
			},
			fullNodeClients: []rpcclient.Client{
				//	&mockNodeClient{
				//		successfulAttempt: 0,
				//		chainID:           "rollapp-fullnode",
				//		blocks:            nil,
				//	},
			},
			pollOrders: []Order{
				{
					EibcOrderId: "order1",
					Amount:      "100",
					Fee:         "10stake",
					RollappId:   "rollapp1",
					BlockHeight: "1",
				},
			},
			expectBotBalanceAfterFulfill:   sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(0))),
			expectWhaleBalanceAfterFulfill: sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(910))),
			expectBotBalanceAfterFinalize:  sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(100))),
			expectFulfilledOrderIDs:        []string{"order1"},
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

			// TODO:
			// check store orders

			oc.stop()
		})
	}

	/*

		Expected order of events:
		1. "sequencer" mode
		- check bot balance
		- start order client
		- emit order event
		- order fulfilled
		- check order store (order should exist as Pending Finalization)
		- check pool (order should not exist)
		- check bot balance
		- emit finalized event
		- stop order client
		- check order store (order should not exist)
		- check bot balance

		2. "p2p" mode
		- check bot balance
		- start order client
		- emit order event
		-* full node client waits for block
		- order fulfilled
		- check order store (order should exist as Pending Finalization)
		- check pool (order should not exist)
		- check bot balance
		- emit finalized event
		- stop order client
		- check order store (order should not exist)
		- check bot balance

	*/
}

func setupTestOrderClient(
	config Config,
	bstore *mockStore,
	whaleBalance sdk.Coins,
	pollOrders func() ([]Order, error),
	hubClient mockNodeClient,
	fullNodeClients ...rpcclient.Client,
) (*orderClient, error) {
	if config.FulfillCriteria.FulfillmentMode.MinConfirmations > len(config.FulfillCriteria.FulfillmentMode.FullNodes) {
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
		fullNodeClients,
		bstore,
		fulfilledOrdersCh,
		bots,
		"",
		config.Bots.MaxOrdersPerTx,
		&config.FulfillCriteria,
		orderCh,
		logger,
	)

	// eventer
	eventer := newOrderEventer(
		cosmosclient.Client{
			RPC:      &hubClient,
			WSEvents: &hubClient,
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
	sender            string
	successfulAttempt int
	attemptCounter    int
	chainID           string
	blocks            map[int64]*coretypes.ResultBlock
	finalizeOrderCh   chan coretypes.ResultEvent
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
	return cosmosclient.Response{}, nil
}

func (m *mockNodeClient) Subscribe(context.Context, string, string, ...int) (out <-chan coretypes.ResultEvent, err error) {
	return m.finalizeOrderCh, nil
}

func (m *mockNodeClient) Block(_ context.Context, h *int64) (*coretypes.ResultBlock, error) {
	if len(m.blocks) == 0 {
		return nil, fmt.Errorf("no block")
	}
	m.attemptCounter++
	if m.attemptCounter < m.successfulAttempt {
		return nil, fmt.Errorf("failed to get block")
	}
	if h == nil {
		return nil, fmt.Errorf("block height is nil")
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
func (m *mockStore) DeleteOrder(context.Context, string) error { return nil }

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

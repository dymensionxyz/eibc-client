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
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/dymensionxyz/cosmosclient/cosmosclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/dymensionxyz/eibc-client/config"
	"github.com/dymensionxyz/eibc-client/types"
)

func TestOrderClient(t *testing.T) {
	type lpConfig struct {
		grant   *types.FulfillOrderAuthorization
		balance sdk.Coins
	}
	tests := []struct {
		name                      string
		config                    config.Config
		lpConfigs                 []lpConfig
		hubClient                 mockNodeClient
		fullNodeClient            *nodeClient
		pollOrders                []Order
		eventOrders               []Order
		updateOrders              []Order
		expectLPFulfilledOrderIDs map[string]string // orderID -> lpAddress
	}{
		{
			name: "p2p mode, orders from poller: fulfilled",
			config: config.Config{
				OrderPolling: config.OrderPollingConfig{
					Interval: 5 * time.Second,
					Enabled:  true,
				},
				Operator: config.OperatorConfig{
					MinFeeShare: "0.1",
				},
				Fulfillers: config.FulfillerConfig{
					Scale:     3,
					BatchSize: 4,
				},
				Validation: config.ValidationConfig{
					WaitTime: time.Second,
					Interval: time.Second,
				},
			},
			lpConfigs: []lpConfig{
				{
					grant: &types.FulfillOrderAuthorization{
						Rollapps: []*types.RollappCriteria{
							{
								RollappId:           "rollapp1",
								Denoms:              []string{"stake", "adym"},
								MinFeePercentage:    sdk.DecProto{Dec: sdk.MustNewDecFromStr("0.1")},
								MaxPrice:            sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(210)), sdk.NewCoin("adym", sdk.NewInt(150))),
								OperatorFeeShare:    sdk.DecProto{Dec: sdk.MustNewDecFromStr("0.1")},
								SpendLimit:          nil,
								SettlementValidated: false,
							}, {
								RollappId:        "rollapp2",
								Denoms:           []string{"stake", "adym"},
								MinFeePercentage: sdk.DecProto{Dec: sdk.MustNewDecFromStr("0.1")},
								MaxPrice:         sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(210)), sdk.NewCoin("adym", sdk.NewInt(150))),
								SpendLimit:       nil,
								OperatorFeeShare: sdk.DecProto{Dec: sdk.MustNewDecFromStr("0.1")},
							}},
					},
					balance: sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(201)), sdk.NewCoin("adym", sdk.NewInt(140))),
				}, {
					grant: &types.FulfillOrderAuthorization{
						Rollapps: []*types.RollappCriteria{{
							RollappId:           "rollapp2",
							Denoms:              []string{"adym"},
							MinFeePercentage:    sdk.DecProto{Dec: sdk.MustNewDecFromStr("0.1")},
							MaxPrice:            sdk.NewCoins(sdk.NewCoin("adym", sdk.NewInt(450))),
							SpendLimit:          nil,
							OperatorFeeShare:    sdk.DecProto{Dec: sdk.MustNewDecFromStr("0.2")},
							SettlementValidated: true,
						}},
					},
					balance: sdk.NewCoins(sdk.NewCoin("adym", sdk.NewInt(500))),
				}, {
					grant: &types.FulfillOrderAuthorization{
						Rollapps: []*types.RollappCriteria{{
							RollappId:           "rollapp1",
							Denoms:              []string{"stake"},
							MinFeePercentage:    sdk.DecProto{Dec: sdk.MustNewDecFromStr("0.1")},
							MaxPrice:            sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(450))),
							SpendLimit:          nil,
							OperatorFeeShare:    sdk.DecProto{Dec: sdk.MustNewDecFromStr("0.2")},
							SettlementValidated: false,
						}},
					},
					balance: sdk.NewCoins(sdk.NewCoin("stake", sdk.NewInt(200)), sdk.NewCoin("adym", sdk.NewInt(100))),
				},
			},
			hubClient: mockNodeClient{},
			fullNodeClient: &nodeClient{
				rollapps: map[string]config.RollappConfig{
					"rollapp1": {
						MinConfirmations: 2,
						FullNodes:        []string{"location1", "location2"},
					},
					"rollapp2": {
						MinConfirmations: 2,
						FullNodes:        []string{"location3", "location4"},
					},
				},
				get: mockValidGet(map[string]map[int64]*blockValidatedResponse{
					"location1": {
						1: {Result: validationLevelP2P, ChainID: "rollapp1"},
						2: {Result: validationLevelP2P, ChainID: "rollapp1"},
						5: {Result: validationLevelP2P, ChainID: "rollapp1"},
					},
					"location2": {
						1: {Result: validationLevelP2P, ChainID: "rollapp1"},
						2: {Result: validationLevelP2P, ChainID: "rollapp1"},
						5: {Result: validationLevelP2P, ChainID: "rollapp1"},
					},
					"location3": {
						3: {Result: validationLevelP2P, ChainID: "rollapp2"},
						4: {Result: validationLevelSettlement, ChainID: "rollapp2"},
						6: {Result: validationLevelP2P, ChainID: "rollapp2"},
					},
					"location4": {
						3: {Result: validationLevelP2P, ChainID: "rollapp2"},
						4: {Result: validationLevelSettlement, ChainID: "rollapp2"},
						6: {Result: validationLevelNone, ChainID: "rollapp2"},
					},
				}),
			},
			pollOrders: []Order{
				{
					EibcOrderId: "order1",
					Amount:      "92",
					Price:       "80",
					Fee:         "12stake",
					RollappId:   "rollapp1",
					ProofHeight: "1",
					BlockHeight: "1",
				}, {
					EibcOrderId: "order2",
					Amount:      "204",
					Price:       "202",
					Fee:         "2stake", // too low - won't fulfill
					RollappId:   "rollapp2",
					ProofHeight: "2",
					BlockHeight: "2",
				}, {
					EibcOrderId: "order5",
					Amount:      "251",
					Price:       "201",
					Fee:         "50stake",
					RollappId:   "rollapp1",
					ProofHeight: "5",
					BlockHeight: "5",
				},
			},
			eventOrders: []Order{
				{
					EibcOrderId: "order3",
					Amount:      "120",
					Price:       "100adym",
					Fee:         "20adym",
					RollappId:   "rollapp2",
					ProofHeight: "3",
				}, {
					EibcOrderId: "order4",
					Amount:      "285",
					Price:       "250adym",
					Fee:         "35adym",
					RollappId:   "rollapp2",
					ProofHeight: "4",
				}, {
					EibcOrderId: "order6",
					Amount:      "285",
					Price:       "250adym",
					Fee:         "35adym",
					RollappId:   "rollapp2",
					ProofHeight: "6",
				},
			},
			updateOrders: []Order{
				{
					EibcOrderId: "order2",
					Amount:      "227",
					Price:       "202adym",
					Fee:         "25stake", // update so it will fulfill
					RollappId:   "rollapp2",
					ProofHeight: "2",
					BlockHeight: "2",
				},
			},
			expectLPFulfilledOrderIDs: map[string]string{
				"order1": "lp-3-address", // lp3 (lp1 and lp3 selected because they fulfill for rollapp1, lp3 preferred because operator fee is higher)
				// "order2": "",          // not fulfilled (lp1 has not enough balance, lp2 does not fulfill stake orders, lp3 does not fulfill for rollapp2)
				"order3": "lp-1-address", // lp1 (both selected but lp1 preferred due to no settlement validation required)
				"order4": "lp-2-address", // lp2 (selected - max price is high enough)
				"order5": "lp-1-address", // lp1 (between lp1 and lp3, only lp1 has enough balance, lp2 doesn't fulfill adym orders)
				// "order6": "lp-2-address", // not fulfilled, only got 1/2 validation
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var fulfilledOrders []*demandOrder
			fulfillOrdersFn := func(demandOrder ...*demandOrder) error {
				fulfilledOrders = append(fulfilledOrders, demandOrder...)
				return nil
			}

			var grants []*authz.GrantAuthorization
			lpBalances := make(map[string]sdk.Coins)
			for i, g := range tt.lpConfigs {
				a, err := cdctypes.NewAnyWithValue(g.grant)
				if err != nil {
					t.Fatal(err)
				}
				lpAddr := fmt.Sprintf("lp-%d-address", i+1)
				grants = append(grants, &authz.GrantAuthorization{
					Granter:       lpAddr,
					Grantee:       "policyAddress",
					Authorization: a,
				})
				lpBalances[lpAddr] = g.balance
			}

			getLPGrants := func(ctx context.Context, in *authz.QueryGranteeGrantsRequest, opts ...grpc.CallOption) (*authz.QueryGranteeGrantsResponse, error) {
				return &authz.QueryGranteeGrantsResponse{
					Grants: grants,
				}, nil
			}

			oc, err := setupTestOrderClient(
				tt.config,
				mockGetPollerOrders(tt.pollOrders),
				tt.hubClient,
				tt.fullNodeClient,
				getLPGrants,
				fulfillOrdersFn,
				lpBalances,
				sdk.MustNewDecFromStr(tt.config.Operator.MinFeeShare),
			)
			require.NoError(t, err)

			go func() {
				err = oc.Start(context.Background())
				require.NoError(t, err)
			}()

			// orders from events will be picked up first
			for _, order := range tt.eventOrders {
				oc.orderEventer.eventClient.(*mockNodeClient).addOrderCh <- coretypes.ResultEvent{
					Events: map[string][]string{
						createdEvent + ".order_id":      {order.EibcOrderId},
						createdEvent + ".price":         {order.Price},
						createdEvent + ".amount":        {order.Amount},
						createdEvent + ".packet_status": {"PENDING"},
						createdEvent + ".fee":           {order.Fee},
						createdEvent + ".rollapp_id":    {order.RollappId},
						createdEvent + ".proof_height":  {order.ProofHeight},
					},
				}
			}

			for _, order := range tt.updateOrders {
				oc.orderEventer.eventClient.(*mockNodeClient).updateOrderCh <- coretypes.ResultEvent{
					Events: map[string][]string{
						updatedFeeEvent + ".order_id":      {order.EibcOrderId},
						updatedFeeEvent + ".price":         {order.Price},
						updatedFeeEvent + ".amount":        {order.Amount},
						updatedFeeEvent + ".packet_status": {"PENDING"},
						updatedFeeEvent + ".new_fee":       {order.Fee},
						updatedFeeEvent + ".rollapp_id":    {order.RollappId},
						updatedFeeEvent + ".proof_height":  {order.ProofHeight},
					},
				}
			}

			// wait a bit for the client to fulfill orders
			time.Sleep(3 * time.Second)

			// ======= after fulfilling =========
			require.Len(t, fulfilledOrders, len(tt.expectLPFulfilledOrderIDs))

			expectTotalLPSpent := map[string]sdk.Coins{}

			for _, o := range fulfilledOrders {
				lpAddr, ok := tt.expectLPFulfilledOrderIDs[o.id]
				require.True(t, ok)
				require.Equal(t, lpAddr, o.lpAddress)
				expectTotalLPSpent[o.lpAddress] = expectTotalLPSpent[o.lpAddress].Add(o.price...)
			}

			for _, lp := range oc.orderTracker.lps {
				assert.Truef(t, lp.reservedFunds.Empty(), "lp %s has reserved funds; got: %s", lp.address, lp.reservedFunds)
				expectBalance := lpBalances[lp.address].Sub(expectTotalLPSpent[lp.address]...)
				assert.Truef(t, expectBalance.IsEqual(lp.getBalance()),
					"lp %s balance is not correct; expect: %s, got: %s", lp.address, expectBalance, lp.getBalance())
			}
		})
	}
}

func setupTestOrderClient(
	cfg config.Config,
	pollOrders func() ([]Order, error),
	hubClient mockNodeClient,
	fullNodeClient *nodeClient,
	grantsFn getLPGrantsFn,
	fulfillOrdersFn func(demandOrder ...*demandOrder) error,
	lpBalances map[string]sdk.Coins,
	minOperatorFeeShare sdk.Dec,
) (*orderClient, error) {
	logger, _ := zap.NewDevelopment()
	orderCh := make(chan []*demandOrder, config.NewOrderBufferSize)

	// tracker
	trackerClient := hubClient

	// fulfillers
	fulfiller := make(map[string]*orderFulfiller)

	ordTracker := newOrderTracker(
		&trackerClient,
		"policyAddress",
		minOperatorFeeShare,
		fullNodeClient,
		"subscriber",
		cfg.Fulfillers.BatchSize,
		&cfg.Validation,
		orderCh,
		cfg.OrderPolling.Interval,
		cfg.Validation.Interval,
		func() {},
		logger,
	)
	ordTracker.getLPGrants = grantsFn
	// LPs always have enough balance
	ordTracker.getBalances = mockGetBalances(lpBalances)

	// eventer
	eventerClient := hubClient
	eventerClient.finalizeOrderCh = make(chan coretypes.ResultEvent, 1)
	eventerClient.addOrderCh = make(chan coretypes.ResultEvent, 1)
	eventerClient.updateOrderCh = make(chan coretypes.ResultEvent, 1)

	eventer := newOrderEventer(
		cosmosclient.Client{
			RPC:      &eventerClient,
			WSEvents: &eventerClient,
		},
		"",
		ordTracker,
		logger,
	)

	chainID := "test-chain-id"

	for i := range cfg.Fulfillers.Scale {
		fulfillerName := fmt.Sprintf("fulfiller-%d", i+1)

		hc := hubClient
		acc := account{
			Name:    fulfillerName,
			Address: fulfillerName + "-address",
		}
		b, err := newOrderFulfiller(
			acc,
			"operatorAddress",
			logger,
			"policyAddress",
			&hc,
			orderCh,
			ordTracker.releaseAllReservedOrdersFunds,
			ordTracker.debitAllReservedOrdersFunds,
		)
		if err != nil {
			return nil, err
		}
		b.FulfillDemandOrders = fulfillOrdersFn
		fulfiller[b.account.Name] = b
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
		fulfillers:   fulfiller,
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

func mockGetBalances(lpBalances map[string]sdk.Coins) getSpendableBalancesFn {
	return func(ctx context.Context, in *banktypes.QuerySpendableBalancesRequest, opts ...grpc.CallOption) (*banktypes.QuerySpendableBalancesResponse, error) {
		return &banktypes.QuerySpendableBalancesResponse{
			Balances: lpBalances[in.Address],
		}, nil
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
	finalizeOrderCh chan coretypes.ResultEvent
	addOrderCh      chan coretypes.ResultEvent
	updateOrderCh   chan coretypes.ResultEvent
}

func (m *mockNodeClient) Start() error {
	return nil
}

func (m *mockNodeClient) Context() client.Context {
	return client.Context{}
}

func (m *mockNodeClient) BroadcastTx(string, ...sdk.Msg) (cosmosclient.Response, error) {
	return cosmosclient.Response{TxResponse: &sdk.TxResponse{}}, nil
}

func (m *mockNodeClient) Subscribe(_ context.Context, _ string, query string, _ ...int) (out <-chan coretypes.ResultEvent, err error) {
	switch {
	case strings.Contains(query, createdEvent):
		return m.addOrderCh, nil
	case strings.Contains(query, updatedFeeEvent):
		return m.updateOrderCh, nil
	}
	return nil, fmt.Errorf("invalid query")
}

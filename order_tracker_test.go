package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	rpcclient "github.com/tendermint/tendermint/rpc/client"
	coretypes "github.com/tendermint/tendermint/rpc/core/types"
	"go.uber.org/zap"

	"github.com/dymensionxyz/eibc-client/store"
)

func Test_orderTracker_canFulfillOrder(t *testing.T) {
	type fields struct {
		currentOrders   map[string]*demandOrder
		trackedOrders   map[string]struct{}
		fulfillCriteria *fulfillCriteria
	}
	type args struct {
		order *demandOrder
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "can fulfill order",
			fields: fields{
				fulfillCriteria: &fulfillCriteria{
					MinFeePercentage: sampleMinFeePercentage,
				},
			},
			args: args{
				order: sampleDemandOrder,
			},
			want: true,
		}, {
			name: "cannot fulfill order: order already fulfilled",
			fields: fields{
				trackedOrders: map[string]struct{}{
					sampleDemandOrder.id: {},
				},
				fulfillCriteria: &fulfillCriteria{
					MinFeePercentage: sampleMinFeePercentage,
				},
			},
			args: args{
				order: sampleDemandOrder,
			},
			want: false,
		}, {
			name: "cannot fulfill order: order already being processed",
			fields: fields{
				currentOrders: map[string]*demandOrder{
					sampleDemandOrder.id: sampleDemandOrder,
				},
				fulfillCriteria: &fulfillCriteria{
					MinFeePercentage: sampleMinFeePercentage,
				},
			},
			args: args{
				order: sampleDemandOrder,
			},
			want: false,
		}, {
			name: "cannot fulfill order: asset fee percentage is lower than minimum",
			fields: fields{
				fulfillCriteria: &fulfillCriteria{
					MinFeePercentage: minFeePercentage{
						Asset: map[string]float32{
							sampleDemandOrder.denom: 0.2,
						},
						Chain: map[string]float32{
							sampleDemandOrder.rollappId: 0.1,
						},
					},
				},
			},
			args: args{
				order: sampleDemandOrder,
			},
			want: false,
		}, {
			name: "cannot fulfill order: chain fee percentage is lower than minimum",
			fields: fields{
				fulfillCriteria: &fulfillCriteria{
					MinFeePercentage: minFeePercentage{
						Asset: map[string]float32{
							sampleDemandOrder.denom: 0.1,
						},
						Chain: map[string]float32{
							sampleDemandOrder.rollappId: 0.2,
						},
					},
				},
			},
			args: args{
				order: sampleDemandOrder,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			or := &orderTracker{
				pool:            orderPool{orders: tt.fields.currentOrders},
				fulfilledOrders: tt.fields.trackedOrders,
				fulfillCriteria: tt.fields.fulfillCriteria,
			}
			if or.pool.orders == nil {
				or.pool.orders = make(map[string]*demandOrder)
			}
			if or.fulfilledOrders == nil {
				or.fulfilledOrders = make(map[string]struct{})
			}
			if got := or.canFulfillOrder(tt.args.order); got != tt.want {
				t.Errorf("canFulfillOrder() = %v, want %v", got, tt.want)
			}
		})
	}
}

var (
	sampleMinFeePercentage = minFeePercentage{
		Asset: map[string]float32{
			sampleDemandOrder.denom: 0.1,
		},
		Chain: map[string]float32{
			sampleDemandOrder.rollappId: 0.1,
		},
	}

	sampleDemandOrder = &demandOrder{
		id:        "order1",
		amount:    amount,
		fee:       fee,
		denom:     amount.GetDenomByIndex(0),
		rollappId: "rollappId",
	}

	amount, _ = sdk.ParseCoinsNormalized("10526097010000000000denom")
	fee, _    = sdk.ParseCoinsNormalized("15789145514999998denom")
)

func Test_worker_sequencerMode(t *testing.T) {
	tests := []struct {
		name             string
		fullNodeClients  []*mockCosmosClient
		store            mockStore
		fulfillmentLevel fulfillmentLevel
		minConfirmations int
		batchSize        int
		orderDeadline    time.Time
		orderIDs         []string
		expectedOrderIDs []string
	}{
		{
			name:             "fulfill orders in sequencer mode",
			fulfillmentLevel: fulfillmentModeSequencer,
			batchSize:        10,
			orderIDs:         generateOrderIDs(10),
			expectedOrderIDs: generateOrderIDs(10),
		}, {
			name:             "fulfill orders in sequencer mode: batshize smaller than order count",
			fulfillmentLevel: fulfillmentModeSequencer,
			batchSize:        7,
			orderIDs:         generateOrderIDs(10),
			expectedOrderIDs: generateOrderIDs(10),
		}, {
			name:             "fulfill orders in sequencer mode: batshize bigger than order count",
			fulfillmentLevel: fulfillmentModeSequencer,
			batchSize:        11,
			orderIDs:         generateOrderIDs(10),
			expectedOrderIDs: generateOrderIDs(10),
		}, {
			name:             "fulfill orders in sequencer mode: high order count",
			fulfillmentLevel: fulfillmentModeSequencer,
			batchSize:        7,
			orderIDs:         generateOrderIDs(100),
			expectedOrderIDs: generateOrderIDs(100),
		}, {
			name:             "fulfill orders in p2p mode",
			fulfillmentLevel: fulfillmentModeP2P,
			minConfirmations: 1,
			fullNodeClients: []*mockCosmosClient{{
				blocks: map[int64]*coretypes.ResultBlock{
					1: {}, 2: {}, 3: {}, 4: {}, 5: {}, 6: {}, 7: {}, 8: {}, 9: {}, 10: {},
				},
			}},
			batchSize:        10,
			orderDeadline:    time.Now().Add(time.Second * 3),
			orderIDs:         generateOrderIDs(10),
			expectedOrderIDs: generateOrderIDs(10),
		}, {
			name:             "fulfill orders in p2p mode: no blocks 6 and 9",
			fulfillmentLevel: fulfillmentModeP2P,
			minConfirmations: 1,
			fullNodeClients: []*mockCosmosClient{{
				blocks: map[int64]*coretypes.ResultBlock{
					1: {}, 2: {}, 3: {}, 4: {}, 5: {}, 7: {}, 8: {}, 10: {},
				},
			}},
			batchSize:        10,
			orderDeadline:    time.Now().Add(time.Second * 3),
			orderIDs:         generateOrderIDs(10),
			expectedOrderIDs: []string{"order1", "order2", "order3", "order4", "order5", "order7", "order8", "order10"},
		}, {
			name:             "fulfill orders in p2p mode: 2/3 nodes validated",
			fulfillmentLevel: fulfillmentModeP2P,
			minConfirmations: 2,
			fullNodeClients: []*mockCosmosClient{
				{
					blocks: map[int64]*coretypes.ResultBlock{
						1: {}, 2: {}, 3: nil, 4: {}, 5: {}, 6: {}, 7: nil, 8: {}, 9: {}, 10: {},
					},
				}, {
					blocks: map[int64]*coretypes.ResultBlock{
						1: {}, 2: nil, 3: {}, 4: nil, 5: {}, 6: nil, 7: {}, 8: nil, 9: {}, 10: nil,
					},
				}, {
					blocks: map[int64]*coretypes.ResultBlock{
						1: nil, 2: {}, 3: {}, 4: {}, 5: nil, 6: {}, 7: {}, 8: {}, 9: nil, 10: {},
					},
				},
			},
			batchSize:        10,
			orderDeadline:    time.Now().Add(time.Second * 3),
			orderIDs:         generateOrderIDs(10),
			expectedOrderIDs: generateOrderIDs(10),
		}, {
			name:             "fulfill orders in p2p mode: half orders 2/3 validated, other half 1/3 validated",
			fulfillmentLevel: fulfillmentModeP2P,
			minConfirmations: 2,
			fullNodeClients: []*mockCosmosClient{
				{
					blocks: map[int64]*coretypes.ResultBlock{
						1: {}, 2: {}, 3: nil, 4: {}, 5: {}, 6: {}, 7: nil, 8: {}, 9: {}, 10: nil,
					},
				}, {
					blocks: map[int64]*coretypes.ResultBlock{
						1: {}, 2: nil, 3: {}, 4: nil, 5: {}, 6: nil, 7: nil, 8: nil, 9: nil, 10: nil,
					},
				}, {
					blocks: map[int64]*coretypes.ResultBlock{
						1: nil, 2: {}, 3: {}, 4: {}, 5: nil, 6: nil, 7: {}, 8: nil, 9: nil, 10: {},
					},
				},
			},
			batchSize:        10,
			orderDeadline:    time.Now().Add(time.Second * 3),
			orderIDs:         generateOrderIDs(10),
			expectedOrderIDs: generateOrderIDs(5),
		}, {
			name:             "fulfill orders in p2p mode: orders hit deadline",
			fulfillmentLevel: fulfillmentModeP2P,
			minConfirmations: 1,
			fullNodeClients: []*mockCosmosClient{
				{
					blocks: map[int64]*coretypes.ResultBlock{},
				},
			},
			batchSize:        10,
			orderDeadline:    time.Now().Add(-time.Second * 1),
			orderIDs:         generateOrderIDs(10),
			expectedOrderIDs: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			fulfillOrderCh := make(chan []*demandOrder)

			ot := &orderTracker{
				hubClient:       &mockCosmosClient{},
				fullNodeClients: transformFullNodeClients(tt.fullNodeClients),
				store:           tt.store,
				logger:          zap.NewNop(),
				fulfilledOrders: make(map[string]struct{}),
				validOrdersCh:   make(chan []*demandOrder),
				outputOrdersCh:  fulfillOrderCh,
				pool:            orderPool{orders: make(map[string]*demandOrder)},
				batchSize:       tt.batchSize,
				fulfillCriteria: &fulfillCriteria{
					MinFeePercentage: sampleMinFeePercentage,
					FulfillmentMode: fulfillmentMode{
						Level:            tt.fulfillmentLevel,
						MinConfirmations: tt.minConfirmations,
					},
				},
			}

			// start order tracker
			_ = ot.start(ctx)

			// generate orders and add them to the pool
			go func() {
				orders := generateOrders(tt.orderIDs, tt.orderDeadline)
				ot.addOrder(orders...)
			}()

			// get orders to fulfill
			var ids []string
		loop:
			for {
				select {
				case <-time.NewTimer(time.Second * 5).C:
					break loop
				case toFulfillOrders := <-fulfillOrderCh:
					for _, o := range toFulfillOrders {
						ids = append(ids, o.id)
						if len(ids) == len(tt.expectedOrderIDs) {
							close(fulfillOrderCh)
							break loop
						}
					}
				}
			}

			assert.ElementsMatch(t, tt.expectedOrderIDs, ids)
		})
	}
}

func generateOrders(ids []string, deadline time.Time) (orders []*demandOrder) {
	for i, id := range ids {
		order := *sampleDemandOrder
		order.id = id
		order.validDeadline = deadline
		order.blockHeight = int64(i + 1)
		orders = append(orders, &order)
	}
	return
}
func generateOrderIDs(n int) (ids []string) {
	for i := 0; i < n; i++ {
		ids = append(ids, "order"+fmt.Sprint(i+1))
	}
	return
}

type mockCosmosClient struct {
	rpcclient.Client
	successfulAttempt int
	attemptCounter    int
	blocks            map[int64]*coretypes.ResultBlock
}

func (m *mockCosmosClient) Subscribe(context.Context, string, string, ...int) (out <-chan coretypes.ResultEvent, err error) {
	return make(chan coretypes.ResultEvent), nil
}

func (m *mockCosmosClient) Block(_ context.Context, h *int64) (*coretypes.ResultBlock, error) {
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

func transformFullNodeClients(clients []*mockCosmosClient) []rpcclient.Client {
	var c []rpcclient.Client
	for _, client := range clients {
		c = append(c, client)
	}
	return c
}

type mockStore struct {
	botStore
	orders []*store.Order
}

func (m mockStore) GetOrders(context.Context, ...store.OrderOption) ([]*store.Order, error) {
	return m.orders, nil
}

func (m mockStore) GetOrder(_ context.Context, id string) (*store.Order, error) {
	var order *store.Order
	for _, o := range m.orders {
		if o.ID == id {
			order = o
			break
		}
	}
	return order, nil
}

func (m mockStore) SaveManyOrders(context.Context, []*store.Order) error { return nil }
func (m mockStore) DeleteOrder(context.Context, string) error            { return nil }

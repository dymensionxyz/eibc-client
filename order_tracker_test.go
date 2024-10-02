package main

import (
	"context"
	"fmt"
	"testing"

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
		name      string
		store     mockStore
		batchSize int
		orderIDs  []string
	}{
		{
			name:      "fulfill orders in sequencer mode",
			batchSize: 7,
			orderIDs:  generateOrderIDs(10),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			fulfillOrderCh := make(chan []*demandOrder)

			ot := &orderTracker{
				client:          mockCosmosClient{},
				store:           tt.store,
				logger:          zap.NewNop(),
				fulfilledOrders: make(map[string]struct{}),
				ordersCh:        fulfillOrderCh,
				pool:            orderPool{orders: make(map[string]*demandOrder)},
				batchSize:       tt.batchSize,
				fulfillCriteria: &fulfillCriteria{
					MinFeePercentage: sampleMinFeePercentage,
					FulfillmentMode: fulfillmentMode{
						Level: fulfillmentModeSequencer,
					},
				},
			}

			// start order tracker
			_ = ot.start(ctx)

			// generate orders and add them to the pool
			go func() {
				orders := generateOrders(tt.orderIDs)
				ot.addOrderToPool(orders...)
			}()

			// get orders to fulfill
			var ids []string
			for toFulfillOrders := range fulfillOrderCh {
				for _, o := range toFulfillOrders {
					ids = append(ids, o.id)
					if len(ids) == len(tt.orderIDs) {
						close(fulfillOrderCh)
						break
					}
				}
			}

			assert.ElementsMatch(t, tt.orderIDs, ids)
		})
	}
}

func generateOrders(ids []string) (orders []*demandOrder) {
	for _, id := range ids {
		order := *sampleDemandOrder
		order.id = id
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
	block *coretypes.ResultBlock
}

func (m mockCosmosClient) Subscribe(context.Context, string, string, ...int) (out <-chan coretypes.ResultEvent, err error) {
	return make(chan coretypes.ResultEvent), nil
}

func (m mockCosmosClient) Block(context.Context, *int64) (*coretypes.ResultBlock, error) {
	return m.block, nil
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

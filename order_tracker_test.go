package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func Test_orderTracker_canFulfillOrder(t *testing.T) {
	type fields struct {
		currentOrders   map[string]*demandOrder
		trackedOrders   map[string]*demandOrder
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
				trackedOrders: map[string]*demandOrder{
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
				or.fulfilledOrders = make(map[string]*demandOrder)
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
		fullNodeClient   *nodeClient
		fulfillmentLevel fulfillmentLevel
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
			fullNodeClient: &nodeClient{
				minimumValidatedNodes:   1,
				expectedValidationLevel: validationLevelP2P,
				locations:               []string{"location1"},
				get: mockValidGet(map[string]map[int64]*blockValidatedResponse{
					"location1": {
						1:  {ValidationLevel: validationLevelP2P},
						2:  {ValidationLevel: validationLevelP2P},
						3:  {ValidationLevel: validationLevelP2P},
						4:  {ValidationLevel: validationLevelP2P},
						5:  {ValidationLevel: validationLevelP2P},
						6:  {ValidationLevel: validationLevelP2P},
						7:  {ValidationLevel: validationLevelP2P},
						8:  {ValidationLevel: validationLevelP2P},
						9:  {ValidationLevel: validationLevelP2P},
						10: {ValidationLevel: validationLevelP2P},
					},
				}),
			},
			batchSize:        10,
			orderDeadline:    time.Now().Add(time.Second * 3),
			orderIDs:         generateOrderIDs(10),
			expectedOrderIDs: generateOrderIDs(10),
		}, {
			name:             "fulfill orders in p2p mode: invalid blocks 6 and 9",
			fulfillmentLevel: fulfillmentModeP2P,
			fullNodeClient: &nodeClient{
				minimumValidatedNodes:   1,
				expectedValidationLevel: validationLevelP2P,
				locations:               []string{"location1"},
				get: mockValidGet(map[string]map[int64]*blockValidatedResponse{
					"location1": {
						1:  {ValidationLevel: validationLevelP2P},
						2:  {ValidationLevel: validationLevelP2P},
						3:  {ValidationLevel: validationLevelP2P},
						4:  {ValidationLevel: validationLevelP2P},
						5:  {ValidationLevel: validationLevelP2P},
						6:  {ValidationLevel: validationLevelNone},
						7:  {ValidationLevel: validationLevelP2P},
						8:  {ValidationLevel: validationLevelP2P},
						9:  {ValidationLevel: validationLevelNone},
						10: {ValidationLevel: validationLevelP2P},
					},
				}),
			},
			batchSize:        10,
			orderDeadline:    time.Now().Add(time.Second * 3),
			orderIDs:         generateOrderIDs(10),
			expectedOrderIDs: []string{"order1", "order2", "order3", "order4", "order5", "order7", "order8", "order10"},
		}, {
			name:             "fulfill orders in p2p mode: 2/3 nodes validated",
			fulfillmentLevel: fulfillmentModeP2P,
			fullNodeClient: &nodeClient{
				minimumValidatedNodes:   2,
				expectedValidationLevel: validationLevelP2P,
				locations:               []string{"location1", "location2", "location3"},
				get: mockValidGet(map[string]map[int64]*blockValidatedResponse{
					"location1": {
						1:  {ValidationLevel: validationLevelP2P},
						2:  {ValidationLevel: validationLevelP2P},
						3:  {ValidationLevel: validationLevelNone},
						4:  {ValidationLevel: validationLevelP2P},
						5:  {ValidationLevel: validationLevelP2P},
						6:  {ValidationLevel: validationLevelP2P},
						7:  {ValidationLevel: validationLevelNone},
						8:  {ValidationLevel: validationLevelP2P},
						9:  {ValidationLevel: validationLevelP2P},
						10: {ValidationLevel: validationLevelP2P},
					},
					"location2": {
						1:  {ValidationLevel: validationLevelP2P},
						2:  {ValidationLevel: validationLevelNone},
						3:  {ValidationLevel: validationLevelP2P},
						4:  {ValidationLevel: validationLevelNone},
						5:  {ValidationLevel: validationLevelP2P},
						6:  {ValidationLevel: validationLevelNone},
						7:  {ValidationLevel: validationLevelP2P},
						8:  {ValidationLevel: validationLevelNone},
						9:  {ValidationLevel: validationLevelP2P},
						10: {ValidationLevel: validationLevelNone},
					},
					"location3": {
						1:  {ValidationLevel: validationLevelNone},
						2:  {ValidationLevel: validationLevelP2P},
						3:  {ValidationLevel: validationLevelP2P},
						4:  {ValidationLevel: validationLevelP2P},
						5:  {ValidationLevel: validationLevelNone},
						6:  {ValidationLevel: validationLevelP2P},
						7:  {ValidationLevel: validationLevelP2P},
						8:  {ValidationLevel: validationLevelP2P},
						9:  {ValidationLevel: validationLevelNone},
						10: {ValidationLevel: validationLevelP2P},
					},
				}),
			},
			batchSize:        10,
			orderDeadline:    time.Now().Add(time.Second * 3),
			orderIDs:         generateOrderIDs(10),
			expectedOrderIDs: generateOrderIDs(10),
		}, {
			name:             "fulfill orders in settlement mode: half orders 2/3 validated, other half 1/3 validated",
			fulfillmentLevel: fulfillmentModeSettlement,
			fullNodeClient: &nodeClient{
				minimumValidatedNodes:   2,
				expectedValidationLevel: validationLevelSettlement,
				locations:               []string{"location1", "location2", "location3"},
				get: mockValidGet(map[string]map[int64]*blockValidatedResponse{
					"location1": {
						1:  {ValidationLevel: validationLevelSettlement},
						2:  {ValidationLevel: validationLevelSettlement},
						3:  {ValidationLevel: validationLevelNone},
						4:  {ValidationLevel: validationLevelSettlement},
						5:  {ValidationLevel: validationLevelSettlement},
						6:  {ValidationLevel: validationLevelSettlement},
						7:  {ValidationLevel: validationLevelNone},
						8:  {ValidationLevel: validationLevelSettlement},
						9:  {ValidationLevel: validationLevelSettlement},
						10: {ValidationLevel: validationLevelP2P},
					},
					"location2": {
						1:  {ValidationLevel: validationLevelSettlement},
						2:  {ValidationLevel: validationLevelNone},
						3:  {ValidationLevel: validationLevelSettlement},
						4:  {ValidationLevel: validationLevelP2P},
						5:  {ValidationLevel: validationLevelSettlement},
						6:  {ValidationLevel: validationLevelNone},
						7:  {ValidationLevel: validationLevelP2P},
						8:  {ValidationLevel: validationLevelNone},
						9:  {ValidationLevel: validationLevelP2P},
						10: {ValidationLevel: validationLevelNone},
					},
					"location3": {
						1:  {ValidationLevel: validationLevelNone},
						2:  {ValidationLevel: validationLevelSettlement},
						3:  {ValidationLevel: validationLevelSettlement},
						4:  {ValidationLevel: validationLevelSettlement},
						5:  {ValidationLevel: validationLevelNone},
						6:  {ValidationLevel: validationLevelP2P},
						7:  {ValidationLevel: validationLevelSettlement},
						8:  {ValidationLevel: validationLevelNone},
						9:  {ValidationLevel: validationLevelP2P},
						10: {ValidationLevel: validationLevelSettlement},
					},
				}),
			},
			batchSize:        10,
			orderDeadline:    time.Now().Add(time.Second * 3),
			orderIDs:         generateOrderIDs(10),
			expectedOrderIDs: generateOrderIDs(5),
		}, {
			name:             "fulfill orders in p2p mode: orders hit deadline",
			fulfillmentLevel: fulfillmentModeP2P,
			fullNodeClient: &nodeClient{
				minimumValidatedNodes:   1,
				expectedValidationLevel: validationLevelP2P,
				get: mockValidGet(map[string]map[int64]*blockValidatedResponse{
					"location1": {
						1: {ValidationLevel: validationLevelP2P},
					},
				}),
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
				fullNodeClient:  tt.fullNodeClient,
				store:           &mockStore{},
				logger:          zap.NewNop(),
				fulfilledOrders: make(map[string]*demandOrder),
				validOrdersCh:   make(chan []*demandOrder),
				outputOrdersCh:  fulfillOrderCh,
				pool:            orderPool{orders: make(map[string]*demandOrder)},
				batchSize:       tt.batchSize,
				fulfillCriteria: &fulfillCriteria{
					MinFeePercentage: sampleMinFeePercentage,
					FulfillmentMode: fulfillmentMode{
						Level: tt.fulfillmentLevel,
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

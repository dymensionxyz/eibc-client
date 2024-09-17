package main

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func Test_orderTracker_canFulfillOrder(t *testing.T) {
	type fields struct {
		currentOrders   map[string]struct{}
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
				currentOrders: map[string]struct{}{
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
				currentOrders:   tt.fields.currentOrders,
				trackedOrders:   tt.fields.trackedOrders,
				fulfillCriteria: tt.fields.fulfillCriteria,
			}
			if or.currentOrders == nil {
				or.currentOrders = make(map[string]struct{})
			}
			if or.trackedOrders == nil {
				or.trackedOrders = make(map[string]struct{})
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

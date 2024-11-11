package eibc

import (
	"fmt"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func Test_shuffleLPs(t *testing.T) {
	type args struct {
		lps []*lp
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Test 1",
			args: args{
				lps: []*lp{
					{
						address: "lpaddr1",
						rollapps: map[string]rollappCriteria{
							"rollapp1": {
								operatorFeeShare:    sdk.NewDec(1),
								settlementValidated: true,
							},
						},
					}, {
						address: "lpaddr2",
						rollapps: map[string]rollappCriteria{
							"rollapp1": {
								operatorFeeShare:    sdk.NewDec(1),
								settlementValidated: true,
							},
						},
					}, {
						address: "lpaddr3",
						rollapps: map[string]rollappCriteria{
							"rollapp1": {
								operatorFeeShare:    sdk.NewDec(1),
								settlementValidated: true,
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shuffleLPs(tt.args.lps)
			for _, lp := range tt.args.lps {
				fmt.Println(lp.address)
			}
		})
	}
}

package main

import (
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type orderBatch struct {
	orders    []*demandOrder
	fulfiller string
}

type demandOrder struct {
	id          string
	denom       string
	amount      sdk.Coins
	fee         sdk.Coins
	rollappId   string
	status      string
	blockHeight uint64
}

func (o *demandOrder) feePercentage() float32 {
	amount := o.amount.AmountOf(o.denom)
	if amount.IsZero() {
		return 0
	}

	price, _, err := big.ParseFloat(amount.String(), 10, 64, big.ToNearestEven)
	if err != nil {
		panic(err)
	}

	fee, _, err := big.ParseFloat(o.fee.AmountOf(o.denom).String(), 10, 64, big.ToNearestEven)
	if err != nil {
		panic(err)
	}

	feeProportion, _ := new(big.Float).Quo(fee, price).Float32()
	feePercent := feeProportion * 100
	return feePercent
}

type account struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

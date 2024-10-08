package main

import (
	"fmt"
	"math/big"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/dymensionxyz/eibc-client/store"
)

type orderBatch struct {
	orders    []*demandOrder
	fulfiller string
}

type demandOrder struct {
	id            string
	denom         string
	amount        sdk.Coins
	fee           sdk.Coins
	feeStr        string
	rollappId     string
	status        string
	blockHeight   int64
	validDeadline time.Time
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

	o.feeStr = o.fee.AmountOf(o.denom).String()

	fee, _, err := big.ParseFloat(o.feeStr, 10, 64, big.ToNearestEven)
	if err != nil {
		panic(err)
	}

	feeProportion, _ := new(big.Float).Quo(fee, price).Float32()
	feePercent := feeProportion * 100
	return feePercent
}

func fromStoreOrder(order *store.Order) (*demandOrder, error) {
	if order.Fee == "" {
		return nil, fmt.Errorf("fee is empty")
	}
	amount, err := sdk.ParseCoinsNormalized(order.Amount)
	if err != nil {
		return nil, fmt.Errorf("failed to parse amount: %w", err)
	}
	if amount.Empty() {
		return nil, fmt.Errorf("amount is empty")
	}
	fee, err := sdk.ParseCoinsNormalized(order.Fee + amount.GetDenomByIndex(0))
	if err != nil {
		return nil, fmt.Errorf("failed to parse fee: %w", err)
	}
	return &demandOrder{
		id:            order.ID,
		denom:         fee.GetDenomByIndex(0),
		amount:        amount,
		fee:           fee,
		rollappId:     order.RollappID,
		status:        string(order.Status),
		blockHeight:   order.BlockHeight,
		validDeadline: time.Unix(order.ValidDeadline, 0),
	}, nil
}

type account struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

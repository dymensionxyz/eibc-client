package eibc

import (
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type orderBatch struct {
	orders    []*demandOrder
	fulfiller string
}

type demandOrder struct {
	id                  string
	denom               string
	amount              sdk.Coins
	fee                 sdk.Coin
	rollappId           string
	status              string
	proofHeight         int64
	validDeadline       time.Time
	settlementValidated bool
	operatorFeePart     sdk.Dec
	lpAddress           string
}

type account struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

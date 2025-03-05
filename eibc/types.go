package eibc

import (
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type demandOrder struct {
	id                  string
	denom               string
	price               sdk.Coins
	amount              sdk.Int
	fee                 sdk.Coin
	rollappId           string
	proofHeight         int64
	validDeadline       time.Time
	settlementValidated bool
	operatorFeePart     math.LegacyDec
	lpAddress           string
	from                string
	checking, valid     bool
}

type hashableOrder struct {
	ID  string   `json:"id"`
	Fee sdk.Coin `json:"fee"`
}

type orderFulfillResult struct {
	orderID         string
	failedOrderHash string
}

type account struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

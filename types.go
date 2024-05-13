package main

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type orderBatch struct {
	orders    []*demandOrder
	fulfiller string
}

type demandOrder struct {
	id     string
	amount sdk.Coins
	status string
}

type account struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

package main

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type demandOrder struct {
	id              string
	price           sdk.Coins
	fee             sdk.Coins
	alertedLowFunds bool // TODO: remove
}

type account struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

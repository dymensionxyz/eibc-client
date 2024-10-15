package handlers

import (
	"net/http"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/dymensionxyz/eibc-client/eibc"
)

type WhaleHandler struct {
	whaleAccSvc       eibc.AccountSvc
	balanceThresholds map[string]sdk.Coin
}

func NewWhaleHandler(balanceThresholds map[string]sdk.Coin, whale eibc.AccountSvc) WhaleHandler {
	return WhaleHandler{
		whaleAccSvc:       whale,
		balanceThresholds: balanceThresholds,
	}
}

func (wh WhaleHandler) GetWhale(w http.ResponseWriter, r *http.Request) {
	// Implement your logic here
}

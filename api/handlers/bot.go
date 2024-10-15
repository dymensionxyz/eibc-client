package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/dymensionxyz/eibc-client/store"
)

type BotHandler struct {
	store botStore
}

type botStore interface {
	GetBots(ctx context.Context, opts ...store.BotOption) ([]*store.Bot, error)
	GetBot(ctx context.Context, address string, opts ...store.BotOption) (*store.Bot, error)
}

func NewBotHandler(store botStore) BotHandler {
	return BotHandler{store: store}
}

func (b BotHandler) ListBots(w http.ResponseWriter, r *http.Request) {
	bots, err := b.store.GetBots(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return bots as JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err = json.NewEncoder(w).Encode(bots); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	return
}

func (b BotHandler) GetBot(w http.ResponseWriter, r *http.Request) {
	// Get bot by address
	address := mux.Vars(r)["address"]
	if address == "" {
		http.Error(w, "missing bot address", http.StatusBadRequest)
		return
	}

	bot, err := b.store.GetBot(r.Context(), address, store.IncludePendingOrders())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return bot as JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err = json.NewEncoder(w).Encode(bot); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	return
}

func (b BotHandler) BotsFulfillingOrdersUpdates(w http.ResponseWriter, r *http.Request) {
	// WebSocket handler placeholder
}

func (b BotHandler) BotsEarningsPayments(w http.ResponseWriter, r *http.Request) {
	// WebSocket handler placeholder
}

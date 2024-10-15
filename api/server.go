package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/dymensionxyz/eibc-client/api/handlers"
)

type Server struct {
	bh         handlers.BotHandler
	wh         handlers.WhaleHandler
	oh         handlers.OrderHandler
	ch         handlers.ClientHandler
	listenAddr string
	logger     *zap.Logger
}

func NewServer(
	bh handlers.BotHandler,
	wh handlers.WhaleHandler,
	// oh handlers.OrderHandler,
	// ch handlers.ClientHandler,
	address string,
	logger *zap.Logger,
) Server {
	return Server{
		bh: bh,
		wh: wh,
		//	oh:         oh,
		//	ch:         ch,
		listenAddr: address,
		logger:     logger,
	}
}

func (s Server) Start() {
	router := mux.NewRouter()

	// Routes for bots
	router.HandleFunc("/bots/list", s.bh.ListBots).Methods("GET") // name, address, balances, pending payouts, claimable earnings
	router.HandleFunc("/bots/{address}", s.bh.GetBot).Methods("GET")
	//	router.HandleFunc("/bots/fulfilling_orders_updates", s.bh.BotsFulfillingOrdersUpdates) // WebSocket handler placeholder ?
	//	router.HandleFunc("/bots/earnings_payments", s.bh.BotsEarningsPayments)                // WebSocket handler placeholder ?

	// Routes for whale
	// router.HandleFunc("/whale/get", s.wh.GetWhale).Methods("GET") // name, address, balances, thresholds

	// Routes for orders
	// router.HandleFunc("/orders/updates", s.oh.IncomingOrdersUpdates) // WebSocket handler placeholder
	// router.HandleFunc("/orders/pool", s.oh.GetPoolOrders).Methods("GET")
	// router.HandleFunc("/orders/fulfilled", s.oh.GetFulfilledOrders).Methods("GET")
	// router.HandleFunc("/orders/fulfill_order", s.oh.FulfillSingleOrder).Methods("POST") // ?

	// Routes for client info
	// router.HandleFunc("/client/info", s.ch.Info).Methods("GET") // num orders fulfilled, time running, total funds, total balances, total earnings
	// router.HandleFunc("/client/scale_bots", s.ch.ScaleBots).Methods("POST")
	// router.HandleFunc("/client/add_denoms", s.ch.AddDenoms).Methods("POST")
	// router.HandleFunc("/client/add_rollapps", s.ch.AddRollapps).Methods("POST")
	// router.HandleFunc("/client/change_fulfill_criteria", s.ch.ChangeFulfillCriteria).Methods("POST")
	// router.HandleFunc("/client/change_fulfillment_mode", s.ch.ChangeFulfillmentMode).Methods("POST")

	// Start server
	s.logger.Info("Starting server", zap.String("port", "8000"))
	s.logger.Fatal("Server failed to start", zap.Error(http.ListenAndServe(s.listenAddr, router)))
}

package handlers

import "net/http"

type ClientHandler struct {
	// Implement your logic here
}

// Handler functions for client
func (c ClientHandler) Info(w http.ResponseWriter, r *http.Request) {
	// Implement your logic here
}

func (c ClientHandler) ScaleBots(w http.ResponseWriter, r *http.Request) {
	// Implement your logic here
}

func (c ClientHandler) AddDenoms(w http.ResponseWriter, r *http.Request) {
	// Implement your logic here
}

func (c ClientHandler) AddRollapps(w http.ResponseWriter, r *http.Request) {
	// Implement your logic here
}

func (c ClientHandler) ChangeFulfillCriteria(w http.ResponseWriter, r *http.Request) {
	// Implement your logic here
}

// TODO: per rollapp
func (c ClientHandler) ChangeFulfillmentMode(w http.ResponseWriter, r *http.Request) {
	// Implement your logic here
}

package store

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

type botStore struct {
	*mongo.Client
}

const (
	botDatabase     = "botstore"
	botCollection   = "bots"
	orderCollection = "orders"
)

func NewBotStore(client *mongo.Client) *botStore {
	return &botStore{client}
}

func (s *botStore) Close() {
	_ = s.Client.Disconnect(context.Background())
}

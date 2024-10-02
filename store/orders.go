package store

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson"
)

type Order struct {
	ID                      string `bson:"_id,omitempty"`
	Fulfiller               string
	Amount                  string
	FulfilledHeight         uint64
	ExpectedFinalizedHeight uint64
	Status                  OrderStatus
	ValidDeadline           int64
}

type OrderStatus string

const (
	OrderStatusFulfilling          OrderStatus = "fulfilling"
	OrderStatusPendingFinalization OrderStatus = "pending"
)

type OrderOption func(*orderFilter)

type orderFilter struct {
	status       OrderStatus
	fulfillerKey string
}

func FilterByStatus(status OrderStatus) OrderOption {
	return func(f *orderFilter) {
		f.status = status
	}
}

func FilterByFulfiller(fulfiller string) OrderOption {
	return func(f *orderFilter) {
		f.fulfillerKey = fulfiller
	}
}

func (s *botStore) GetOrder(ctx context.Context, id string) (*Order, error) {
	ordersCollection := s.Database(botDatabase).Collection(orderCollection)

	var order Order
	err := ordersCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&order)
	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	return &order, nil
}

func (s *botStore) GetOrders(ctx context.Context, opts ...OrderOption) ([]*Order, error) {
	ordersCollection := s.Database(botDatabase).Collection(orderCollection)

	var filter orderFilter
	for _, opt := range opts {
		opt(&filter)
	}

	query := bson.M{}
	if filter.status != "" {
		query["status"] = filter.status
	}
	if filter.fulfillerKey != "" {
		query["fulfillerkey"] = filter.fulfillerKey
	}

	cursor, err := ordersCollection.Find(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get orders: %w", err)
	}

	var orders []*Order
	if err = cursor.All(ctx, &orders); err != nil {
		return nil, fmt.Errorf("failed to get orders: %w", err)
	}

	return orders, nil
}

func (s *botStore) SaveOrder(ctx context.Context, order *Order) error {
	ordersCollection := s.Database(botDatabase).Collection(orderCollection)
	_, err := ordersCollection.InsertOne(ctx, order)
	if err != nil {
		return fmt.Errorf("failed to save order: %w", err)
	}

	return nil
}

func (s *botStore) SaveManyOrders(ctx context.Context, orders []*Order) error {
	ordersCollection := s.Database(botDatabase).Collection(orderCollection)
	records := make([]interface{}, len(orders))

	for i, order := range orders {
		records[i] = order
	}

	_, err := ordersCollection.InsertMany(ctx, records)
	if err != nil {
		return fmt.Errorf("failed to insert many orders: %w", err)
	}

	return nil
}

func (s *botStore) DeleteOrder(ctx context.Context, id string) error {
	ordersCollection := s.Database(botDatabase).Collection(orderCollection)
	_, err := ordersCollection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to update order: %w", err)
	}

	return nil
}

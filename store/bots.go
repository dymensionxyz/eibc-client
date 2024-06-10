package store

import (
	"context"
	"errors"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Bot struct {
	Address        string `bson:"_id,omitempty"`
	Name           string
	Balances       []string
	PendingRewards PendingRewards
	Orders         []*Order
}

type PendingRewards []string

func (pr PendingRewards) ToCoins() sdk.Coins {
	coins := sdk.NewCoins()
	for _, reward := range pr {
		coin, err := sdk.ParseCoinNormalized(reward)
		if err != nil {
			continue
		}
		coins = coins.Add(coin)
	}
	return coins
}

func CoinsToStrings(coins sdk.Coins) []string {
	coinStrings := make([]string, 0, len(coins))
	for _, coin := range coins {
		coinStrings = append(coinStrings, coin.String())
	}
	return coinStrings
}

type BotOption func(*botFilter)

func IncludePendingOrders() BotOption {
	return func(f *botFilter) {
		f.includePendingOrders = true
	}
}

func OnlyWithFunds() BotOption {
	return func(f *botFilter) {
		f.withFunds = true
	}
}

type botFilter struct {
	includePendingOrders bool
	withFunds            bool
}

func (s *botStore) GetBot(ctx context.Context, key string, opts ...BotOption) (*Bot, error) {
	botsCollection := s.Database(botDatabase).Collection(botCollection)

	var bot Bot
	err := botsCollection.FindOne(ctx, bson.M{"_id": key}).Decode(&bot)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get bot: %w", err)
	}

	var filter botFilter
	for _, opt := range opts {
		opt(&filter)
	}

	if filter.includePendingOrders {
		bot.Orders, err = s.GetOrders(ctx, FilterByFulfiller(key), FilterByStatus(OrderStatusPending))
		if err != nil {
			return nil, fmt.Errorf("failed to get bot orders: %w", err)
		}
	}

	return &bot, nil
}

func (s *botStore) GetBots(ctx context.Context, opts ...BotOption) ([]*Bot, error) {
	botsCollection := s.Database(botDatabase).Collection(botCollection)

	var filter botFilter
	for _, opt := range opts {
		opt(&filter)
	}

	f := bson.M{}

	if filter.withFunds {
		f["balances"] = bson.M{"$exists": true, "$ne": bson.A{}}
	}

	cursor, err := botsCollection.Find(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("failed to get all bots: %w", err)
	}

	var bots []*Bot
	if err = cursor.All(ctx, &bots); err != nil {
		return nil, fmt.Errorf("failed to get all bots: %w", err)
	}

	return bots, nil
}

func (s *botStore) SaveBot(ctx context.Context, bot *Bot) error {
	botsCollection := s.Database(botDatabase).Collection(botCollection)
	upsert := true
	_, err := botsCollection.ReplaceOne(ctx, bson.M{"_id": bot.Address}, bot, &options.ReplaceOptions{Upsert: &upsert})
	if err != nil {
		return fmt.Errorf("failed to update bot: %w", err)
	}

	return nil
}

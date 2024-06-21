package store_test

import (
	"context"
	"testing"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/dymensionxyz/eibc-client/store"
)

func TestBotStore(t *testing.T) {
	// Create a new Docker pool
	pool, err := dockertest.NewPool("")
	require.NoError(t, err)

	// Start a new MongoDB container
	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "mongo",
		Tag:        "4.2",
		Env:        []string{},
	}, func(config *docker.HostConfig) {
		// Set AutoRemove to true so that stopped container gets deleted
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	require.NoError(t, err)

	defer func() {
		require.NoError(t, pool.Purge(resource))
	}()

	var client *mongo.Client

	ctx := context.Background()

	err = pool.Retry(func() error {
		url := "mongodb://localhost:" + resource.GetPort("27017/tcp")
		client, err = mongo.Connect(ctx, options.Client().ApplyURI(url))
		if err != nil {
			return err
		}
		return client.Ping(ctx, nil)
	})
	require.NoError(t, err)

	key := "bot-1"
	t.Run("test bot creation", func(t *testing.T) {
		s := store.NewBotStore(client)
		bot := &store.Bot{
			Address: key,
			Balances: []string{
				"1000000000000adym",
			},
		}

		err = s.SaveBot(ctx, bot)
		require.NoError(t, err)

		b, err := s.GetBot(ctx, key)
		require.NoError(t, err)
		require.Equal(t, bot, b)
	})

	t.Run("test order creation", func(t *testing.T) {
		s := store.NewBotStore(client)
		id := "order-1"
		order := &store.Order{
			ID:                      id,
			Fulfiller:               key,
			Amount:                  "1000000000000adym",
			FulfilledHeight:         100,
			ExpectedFinalizedHeight: 200,
			Status:                  store.OrderStatusPending,
		}

		err = s.SaveOrder(ctx, order)
		require.NoError(t, err)

		o, err := s.GetOrder(ctx, id)
		require.NoError(t, err)
		require.Equal(t, order, o)
	})

	t.Run("test order retrieval by status", func(t *testing.T) {
		s := store.NewBotStore(client)
		id := "order-2"
		order := &store.Order{
			ID:                      id,
			Fulfiller:               key,
			Amount:                  "1000000000000adym",
			FulfilledHeight:         100,
			ExpectedFinalizedHeight: 200,
			Status:                  store.OrderStatusFinalized,
		}

		err = s.SaveOrder(ctx, order)
		require.NoError(t, err)

		o, err := s.GetOrders(ctx, store.FilterByStatus(store.OrderStatusFinalized))
		require.NoError(t, err)
		require.Len(t, o, 1)
		require.Equal(t, order, o[0])
	})

	t.Run("test get bot with pending orders", func(t *testing.T) {
		s := store.NewBotStore(client)

		botKey := "bot-2"
		bot := &store.Bot{
			Address: botKey,
			Balances: []string{
				"1000000000000adym",
			},
		}

		err = s.SaveBot(ctx, bot)
		require.NoError(t, err)

		id := "order-3"
		order := &store.Order{
			ID:                      id,
			Fulfiller:               botKey,
			Amount:                  "1000000000000adym",
			FulfilledHeight:         100,
			ExpectedFinalizedHeight: 200,
			Status:                  store.OrderStatusPending,
		}

		err = s.SaveOrder(ctx, order)
		require.NoError(t, err)

		b, err := s.GetBot(ctx, botKey, store.IncludePendingOrders())
		require.NoError(t, err)
		require.Len(t, b.Orders, 1)
		require.Equal(t, order, b.Orders[0])
	})
}

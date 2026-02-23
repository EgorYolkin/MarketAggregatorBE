package redis_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	testredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"market_aggregator/internal/domain"
	repo "market_aggregator/internal/infrastructure/repository/redis"
)

func TestRedisRepository_Integration(t *testing.T) {
	ctx := context.Background()

	// 1. Spinning up a Redis container
	redisContainer, err := testredis.Run(ctx,
		"redis:7-alpine",
	)
	if err != nil {
		t.Fatalf("failed to start redis container: %s", err)
	}
	defer func() {
		if e := redisContainer.Terminate(ctx); e != nil {
			t.Fatalf("failed to terminate redis container: %s", e)
		}
	}()

	uri, err := redisContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("failed to get connection string: %s", err)
	}

	// 2. Initializing client and repository
	opt, err := redis.ParseURL(uri)
	if err != nil {
		t.Fatalf("failed to parse redis url: %s", err)
	}
	client := redis.NewClient(opt)
	defer func() { _ = client.Close() }()

	r := repo.NewRepository(client)

	// --- TEST: Ping ---
	err = r.Ping(ctx)
	assert.NoError(t, err)

	// --- TEST: CacheRepository (AssetPrice) ---
	t.Run("Set and Get Asset", func(t *testing.T) {
		asset := domain.AssetPrice{
			Symbol:          "BTC",
			AggregatedPrice: 50000.0,
			Sources:         map[string]float64{"Binance": 50000.0, "CoinGecko": 50000.0},
			UpdatedAt:       time.Now().UTC().Truncate(time.Millisecond), // Redis JSON precision compatibility
		}

		err := r.SetAsset(ctx, asset)
		assert.NoError(t, err)

		fetched, err := r.GetAsset(ctx, "BTC")
		assert.NoError(t, err)
		assert.Equal(t, asset.Symbol, fetched.Symbol)
		assert.Equal(t, asset.AggregatedPrice, fetched.AggregatedPrice)
		assert.Equal(t, asset.Sources, fetched.Sources)
		// Compare times allowing timezone discrepancies
		assert.True(t, asset.UpdatedAt.Equal(fetched.UpdatedAt))
	})

	t.Run("Get All Assets", func(t *testing.T) {
		asset2 := domain.AssetPrice{
			Symbol:          "ETH",
			AggregatedPrice: 3000.0,
			Sources:         map[string]float64{"Binance": 3000.0},
			UpdatedAt:       time.Now().UTC(),
		}
		_ = r.SetAsset(ctx, asset2)

		assets, err := r.GetAllAssets(ctx)
		assert.NoError(t, err)
		assert.Len(t, assets, 2)
	})

	// --- TEST: WebhookRepository ---
	t.Run("Webhooks", func(t *testing.T) {
		sub1 := domain.WebhookSubscription{
			ID:               uuid.New(),
			Symbol:           "SOL",
			TargetURL:        "http://test.com",
			ThresholdPercent: 5.0,
			BasePrice:        100.0,
		}

		sub2 := domain.WebhookSubscription{
			ID:               uuid.New(),
			Symbol:           "SOL",
			TargetURL:        "http://test2.com",
			ThresholdPercent: 10.0,
			BasePrice:        100.0,
		}

		err := r.SaveSubscription(ctx, sub1)
		assert.NoError(t, err)
		err = r.SaveSubscription(ctx, sub2)
		assert.NoError(t, err)

		// Get all for SOL
		subs, err := r.GetSubscriptionsBySymbol(ctx, "SOL")
		assert.NoError(t, err)
		assert.Len(t, subs, 2)

		// Update base price
		err = r.UpdateBasePrice(ctx, sub1.ID.String(), 150.0)
		assert.NoError(t, err)

		// Verify update
		subsConfigured, _ := r.GetSubscriptionsBySymbol(ctx, "SOL")
		for _, s := range subsConfigured {
			if s.ID == sub1.ID {
				assert.Equal(t, 150.0, s.BasePrice)
			}
		}
	})
}

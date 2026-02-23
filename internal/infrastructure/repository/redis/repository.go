// Package redis implements the CacheRepository and WebhookRepository interfaces using Redis.
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"market_aggregator/internal/domain"

	"github.com/redis/go-redis/v9"
)

// Repository wraps a Redis client and implements both CacheRepository and WebhookRepository.
type Repository struct {
	client *redis.Client
}

// NewRepository creates a new Repository using the provided Redis client connection.
func NewRepository(client *redis.Client) *Repository {
	return &Repository{
		client: client,
	}
}

// ---------------------------------------------------------
// CacheRepository Implementation
// ---------------------------------------------------------

// Ping checks the availability of the Redis server.
func (r *Repository) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// SetAsset serializes an AssetPrice into JSON and stores it with a 30s TTL.
func (r *Repository) SetAsset(ctx context.Context, asset domain.AssetPrice) error {
	dto := FromDomain(asset)

	jsonBytes, err := json.Marshal(dto)
	if err != nil {
		return err
	}

	key := "asset:price:" + asset.Symbol
	// TTL is selected based on polling frequency (e.g., 30 seconds),
	// so that stale prices expire if the worker stops.
	ttl := 30 * time.Second

	return r.client.Set(ctx, key, jsonBytes, ttl).Err()
}

// GetAsset retrieves a single asset price from Redis by symbol.
// Returns domain.ErrAssetNotFound if the key has expired or was never set.
func (r *Repository) GetAsset(ctx context.Context, symbol string) (domain.AssetPrice, error) {
	key := "asset:price:" + symbol
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return domain.AssetPrice{}, domain.ErrAssetNotFound
		}
		return domain.AssetPrice{}, err
	}

	var dto AssetPriceDTO
	if err := json.Unmarshal([]byte(val), &dto); err != nil {
		return domain.AssetPrice{}, err
	}

	return dto.ToDomain(), nil
}

// GetAllAssets scans all asset:price:* keys and returns the full list of cached prices.
func (r *Repository) GetAllAssets(ctx context.Context) ([]domain.AssetPrice, error) {
	var assets []domain.AssetPrice
	var cursor uint64
	var keys []string
	var err error

	for {
		var scanKeys []string
		scanKeys, cursor, err = r.client.Scan(ctx, cursor, "asset:price:*", 10).Result()
		if err != nil {
			return nil, err
		}
		keys = append(keys, scanKeys...)
		if cursor == 0 {
			break
		}
	}

	if len(keys) == 0 {
		return assets, nil
	}

	// MGet for batch retrieval
	vals, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	for _, val := range vals {
		if val == nil {
			continue // Key might have expired between SCAN and MGET
		}
		strVal, ok := val.(string)
		if !ok {
			continue
		}

		var dto AssetPriceDTO
		if err := json.Unmarshal([]byte(strVal), &dto); err == nil {
			assets = append(assets, dto.ToDomain())
		}
	}

	return assets, nil
}

// ---------------------------------------------------------
// WebhookRepository Implementation
// ---------------------------------------------------------

// SaveSubscription persists a webhook subscription in a Redis hash keyed by symbol.
func (r *Repository) SaveSubscription(ctx context.Context, sub domain.WebhookSubscription) error {
	dto := WebhookFromDomain(sub)

	jsonBytes, err := json.Marshal(dto)
	if err != nil {
		return err
	}

	key := "webhooks:" + sub.Symbol
	return r.client.HSet(ctx, key, sub.ID.String(), jsonBytes).Err()
}

// GetSubscriptionsBySymbol returns all webhook subscriptions registered for a specific asset symbol.
func (r *Repository) GetSubscriptionsBySymbol(ctx context.Context, symbol string) ([]domain.WebhookSubscription, error) {
	key := "webhooks:" + symbol

	results, err := r.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var subscriptions []domain.WebhookSubscription
	for _, jsonStr := range results {
		var dto WebhookSubscriptionDTO
		if err := json.Unmarshal([]byte(jsonStr), &dto); err == nil {
			subscriptions = append(subscriptions, dto.ToDomain())
		}
	}

	return subscriptions, nil
}

// UpdateBasePrice locates a subscription by ID across all webhook hashes and updates its BasePrice.
func (r *Repository) UpdateBasePrice(ctx context.Context, id string, newPrice float64) error {
	var cursor uint64
	var keys []string
	var err error

	for {
		var scanKeys []string
		scanKeys, cursor, err = r.client.Scan(ctx, cursor, "webhooks:*", 10).Result()
		if err != nil {
			return err
		}
		keys = append(keys, scanKeys...)
		if cursor == 0 {
			break
		}
	}

	var foundSymbolKey string
	var foundJSON string

	for _, k := range keys {
		val, err := r.client.HGet(ctx, k, id).Result()
		if err == nil && val != "" {
			foundSymbolKey = k
			foundJSON = val
			break
		} else if err != nil && !errors.Is(err, redis.Nil) {
			return err
		}
	}

	if foundSymbolKey == "" {
		return nil
	}

	var dto WebhookSubscriptionDTO
	if err := json.Unmarshal([]byte(foundJSON), &dto); err != nil {
		return err
	}

	dto.BasePrice = newPrice

	updatedJSON, err := json.Marshal(dto)
	if err != nil {
		return err
	}

	return r.client.HSet(ctx, foundSymbolKey, id, updatedJSON).Err()
}

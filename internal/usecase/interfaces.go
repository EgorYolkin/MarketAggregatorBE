package usecase

import (
	"context"

	"market_aggregator/internal/domain"
)

// MarketDataProvider defines the abstraction for external price exchange adapters.
// Implementations are responsible for authentication, rate limiting, and data normalization.
type MarketDataProvider interface {
	// Name returns the provider identifier used in audit logs and price breakdowns.
	Name() string

	// FetchPrices retrieves real-time pricing for the requested symbols.
	// It should handle underlying API specificities and return errors for transient failures
	// to allow for higher-level retry mechanisms.
	FetchPrices(ctx context.Context, symbols []string) (map[string]float64, error)
}

// CacheRepository defines the persistence contract for aggregated asset price data.
type CacheRepository interface {
	// SetAsset persists the aggregated price with a strict TTL for freshness.
	SetAsset(ctx context.Context, asset domain.AssetPrice) error

	// GetAsset retrieves a single asset price or returns domain.ErrAssetNotFound.
	GetAsset(ctx context.Context, symbol string) (domain.AssetPrice, error)

	// GetAllAssets retrieves all currently cached assets. Useful for bulk API responses.
	GetAllAssets(ctx context.Context) ([]domain.AssetPrice, error)

	// Ping verifies the connectivity to the underlying storage engine.
	Ping(ctx context.Context) error
}

// WebhookRepository defines the persistence contract for alert subscriptions.
type WebhookRepository interface {
	// SaveSubscription persists a new webhook alert.
	SaveSubscription(ctx context.Context, sub domain.WebhookSubscription) error

	// GetSubscriptionsBySymbol retrieves all active alerts for a specific asset.
	GetSubscriptionsBySymbol(ctx context.Context, symbol string) ([]domain.WebhookSubscription, error)

	// UpdateBasePrice updates the baseline price for a specific subscription after an alert triggers.
	UpdateBasePrice(ctx context.Context, id string, newPrice float64) error
}

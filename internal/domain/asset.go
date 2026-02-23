// Package domain defines the core business entities and errors for the market aggregator.
package domain

import "time"

// AssetPrice represents a normalized and aggregated snapshot of an asset's market value.
// It serves as the primary domain entity for price tracking and alerting logic.
type AssetPrice struct {
	// Symbol is the unique ticker identifier (e.g., "BTC", "ETH").
	// Consistency across different data providers is handled at the usecase level.
	Symbol string

	// AggregatedPrice is the result of the median calculation across all available sources.
	// This approach ensures resilience against price manipulation or provider failures.
	AggregatedPrice float64

	// Sources contains a map of provider names to their last reported prices.
	// Used for transparency and debugging price discrepancies (e.g., {"Binance": 50000.1}).
	Sources map[string]float64

	// UpdatedAt is the UTC timestamp of the last successful aggregation cycle.
	// Used by the storage layer to enforce data freshness via TTL.
	UpdatedAt time.Time
}

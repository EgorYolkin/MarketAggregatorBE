// Package redis implements the CacheRepository and WebhookRepository interfaces using Redis.
package redis

import (
	"time"

	"market_aggregator/internal/domain"
)

// AssetPriceDTO is the serialized internal representation of an AssetPrice for Redis storage.
// It optimizes storage by flattening domain types where necessary.
type AssetPriceDTO struct {
	Symbol         string             `json:"symbol"`
	AggPrice       float64            `json:"agg_price"`
	ProviderPrices map[string]float64 `json:"provider_prices"`
	LastUpdated    time.Time          `json:"last_updated"`
}

// ToDomain converts the Redis DTO back into the business domain entity.
func (dto AssetPriceDTO) ToDomain() domain.AssetPrice {
	return domain.AssetPrice{
		Symbol:          dto.Symbol,
		AggregatedPrice: dto.AggPrice,
		Sources:         dto.ProviderPrices,
		UpdatedAt:       dto.LastUpdated,
	}
}

// FromDomain transforms a domain entity into a serializable DTO for persistence.
func FromDomain(a domain.AssetPrice) AssetPriceDTO {
	return AssetPriceDTO{
		Symbol:         a.Symbol,
		AggPrice:       a.AggregatedPrice,
		ProviderPrices: a.Sources,
		LastUpdated:    a.UpdatedAt,
	}
}

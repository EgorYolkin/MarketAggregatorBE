package redis

import (
	"market_aggregator/internal/domain"

	"github.com/google/uuid"
)

// WebhookSubscriptionDTO represents the flattened storage format for alert subscriptions.
// It is stored as a JSON blob within Redis HSETs, grouped by asset symbol.
type WebhookSubscriptionDTO struct {
	ID               uuid.UUID `json:"id"`
	Symbol           string    `json:"symbol"`
	TargetURL        string    `json:"target_url"`
	ThresholdPercent float64   `json:"threshold_percent"`
	BasePrice        float64   `json:"base_price"`
}

// ToDomain converts the storage representation back into a robust domain entity.
func (dto WebhookSubscriptionDTO) ToDomain() domain.WebhookSubscription {
	return domain.WebhookSubscription{
		ID:               dto.ID,
		Symbol:           dto.Symbol,
		TargetURL:        dto.TargetURL,
		ThresholdPercent: dto.ThresholdPercent,
		BasePrice:        dto.BasePrice,
	}
}

// WebhookFromDomain converts a business domain entity into a format suitable for the storage provider.
func WebhookFromDomain(sub domain.WebhookSubscription) WebhookSubscriptionDTO {
	return WebhookSubscriptionDTO{
		ID:               sub.ID,
		Symbol:           sub.Symbol,
		TargetURL:        sub.TargetURL,
		ThresholdPercent: sub.ThresholdPercent,
		BasePrice:        sub.BasePrice,
	}
}

package usecase

import (
	"context"
	"math"
	"time"

	"market_aggregator/internal/domain"
)

// WebhookService handles webhook triggers when prices change.
type WebhookService struct {
	repo      WebhookRepository
	eventChan chan<- WebhookEvent // Channel for sending events to workers
}

// NewWebhookService creates a new WebhookService with the given repository and event channel.
func NewWebhookService(repo WebhookRepository, evChan chan<- WebhookEvent) *WebhookService {
	return &WebhookService{
		repo:      repo,
		eventChan: evChan,
	}
}

// RegisterSubscription saves a new webhook subscription via the repository.
func (ws *WebhookService) RegisterSubscription(ctx context.Context, sub domain.WebhookSubscription) error {
	return ws.repo.SaveSubscription(ctx, sub)
}

// CheckTriggers receives a new aggregated price and compares it with the base price for all subscriptions.
func (ws *WebhookService) CheckTriggers(ctx context.Context, asset domain.AssetPrice) {
	// 1. Retrieve all subscriptions for the given asset
	subs, err := ws.repo.GetSubscriptionsBySymbol(ctx, asset.Symbol)
	if err != nil || len(subs) == 0 {
		return
	}

	// 2. Iterate and check mathematical conditions
	for _, sub := range subs {
		if sub.BasePrice <= 0 {
			continue // Protect against division by zero
		}

		// Formula: | (New - Base) / Base | * 100
		deltaPercent := math.Abs((asset.AggregatedPrice-sub.BasePrice)/sub.BasePrice) * 100.0

		// If threshold reached or exceeded
		if deltaPercent >= sub.ThresholdPercent {
			// Round to 4 decimal places for clean JSON output.
			rounded := math.Round(deltaPercent*10000) / 10000

			event := WebhookEvent{
				SubscriptionID: sub.ID.String(),
				TargetURL:      sub.TargetURL,
				Symbol:         asset.Symbol,
				OldPrice:       sub.BasePrice,
				NewPrice:       asset.AggregatedPrice,
				ChangePct:      rounded,
				Timestamp:      time.Now().UTC(),
			}

			// Send to channel (non-blocking select in case workers are busy)
			select {
			case ws.eventChan <- event:
				// Event sent to queue
			default:
				// Optional: log dropped events
			}
		}
	}
}

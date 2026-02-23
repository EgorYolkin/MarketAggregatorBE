// Package processors implements background workers: the polling loop and the webhook dispatcher.
package processors

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"market_aggregator/internal/usecase"
)

// WebhookRepository manages webhook subscription state updates.
type WebhookRepository interface {
	UpdateBasePrice(ctx context.Context, subID string, newPrice float64) error
}

// WebhookDispatcher consumes WebhookEvents from a channel and delivers them to target URLs.
type WebhookDispatcher struct {
	eventChan <-chan usecase.WebhookEvent
	repo      WebhookRepository
	client    *http.Client
}

// NewWebhookDispatcher creates a new WebhookDispatcher with a 5s HTTP client timeout.
func NewWebhookDispatcher(eventChan <-chan usecase.WebhookEvent, repo WebhookRepository) *WebhookDispatcher {
	return &WebhookDispatcher{
		eventChan: eventChan,
		repo:      repo,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Start begins the event loop, dispatching webhooks until ctx is cancelled.
func (d *WebhookDispatcher) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			slog.Info("webhook dispatcher stopped")
			return ctx.Err()
		case event := <-d.eventChan:
			go d.dispatch(ctx, event)
		}
	}
}

func (d *WebhookDispatcher) dispatch(ctx context.Context, event usecase.WebhookEvent) {
	payload, err := json.Marshal(event)
	if err != nil {
		slog.ErrorContext(ctx, "dispatcher: failed to marshal webhook event", "error", err)
		return
	}

	slog.InfoContext(ctx, "dispatcher: sending webhook",
		"subscription_id", event.SubscriptionID,
		"symbol", event.Symbol,
		"target_url", event.TargetURL,
		"change_pct", event.ChangePct,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, event.TargetURL, bytes.NewBuffer(payload))
	if err != nil {
		slog.ErrorContext(ctx, "dispatcher: failed to create request",
			"target_url", event.TargetURL,
			"error", err,
		)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req) //nolint:gosec // Target URL comes from a registered webhook subscription, not user input.
	if err != nil {
		slog.WarnContext(ctx, "dispatcher: webhook delivery failed",
			"subscription_id", event.SubscriptionID,
			"target_url", event.TargetURL,
			"error", err,
		)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		slog.InfoContext(ctx, "dispatcher: webhook delivered successfully",
			"subscription_id", event.SubscriptionID,
			"target_url", event.TargetURL,
			"status_code", resp.StatusCode,
		)
		if err := d.repo.UpdateBasePrice(ctx, event.SubscriptionID, event.NewPrice); err != nil {
			slog.ErrorContext(ctx, "dispatcher: failed to update base price after delivery",
				"subscription_id", event.SubscriptionID,
				"error", err,
			)
		}
	} else {
		slog.WarnContext(ctx, "dispatcher: webhook target returned non-2xx",
			"subscription_id", event.SubscriptionID,
			"target_url", event.TargetURL,
			"status_code", resp.StatusCode,
		)
	}
}

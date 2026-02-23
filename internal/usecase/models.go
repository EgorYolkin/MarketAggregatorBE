package usecase

import "time"

// fetchResult stores the response from a single provider for concurrent data collection.
type fetchResult struct {
	ProviderName string
	Prices       map[string]float64
	Err          error
}

// WebhookEvent represents a triggered price alert ready for delivery.
// It contains the historical context of the trigger for client-side processing.
// The struct is serialized directly into the webhook POST body by the dispatcher.
type WebhookEvent struct {
	// SubscriptionID links the event back to the original WebhookSubscription.
	// Mandatory for a client to correlate the alert with their registration.
	SubscriptionID string `json:"subscription_id"`

	// TargetURL is the destination endpoint for delivery.
	// Excluded from the JSON payload — used only for internal HTTP routing.
	TargetURL string `json:"-"`
	// Symbol is the asset ticker (e.g., "BTC").
	Symbol string `json:"symbol"`

	// OldPrice is the price at the time of subscription or last triggered alert.
	OldPrice float64 `json:"old_price"`

	// NewPrice is the current aggregated price that triggered the alert.
	NewPrice float64 `json:"new_price"`

	// ChangePct is the actual relative change captured, rounded to 4 decimal places.
	// Measured in percentage points (e.g., 5.1234).
	ChangePct float64 `json:"change_pct"`

	// Timestamp is the UTC time when the alert was triggered.
	Timestamp time.Time `json:"timestamp"`
}

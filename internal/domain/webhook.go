package domain

import "github.com/google/uuid"

// WebhookSubscription defines the business rules for a price alert requested by a client.
// It links an asset, a change threshold, and a target callback destination.
type WebhookSubscription struct {
	// ID is the unique identifier for the subscription, managed as a UUID v4.
	ID uuid.UUID

	// Symbol is the asset ticker to monitor (e.g., "BTC").
	Symbol string

	// TargetURL is the destination endpoint that will receive POST notifications.
	// Validated to be a well-formed absolute URL.
	TargetURL string

	// ThresholdPercent is the relative change required to trigger an alert.
	// Measured in percentage points (e.g., 5.0 indicates a 5% deviation from BasePrice).
	// Valid range: (0.0, 100.0].
	ThresholdPercent float64

	// BasePrice is the aggregated price at the moment of registration or last successful alert.
	// Used as the baseline for calculating the current deviation percentage.
	BasePrice float64
}

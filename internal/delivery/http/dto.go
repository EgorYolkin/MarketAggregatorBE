// Package http implements the HTTP delivery layer using Fiber.
// Package http implements the HTTP delivery layer DTOs.
package http //nolint:revive // intentional: delivery/http is a common Go package layout pattern.

import "time"

// AssetResponse defines the public HTTP response payload for asset price information.
// It provides both the aggregated value and the breakdown per provider.
type AssetResponse struct {
	// Symbol is the asset ticker (e.g., "BTC").
	Symbol string `json:"symbol"`

	// AggregatedPrice is the calculated median price across all active providers.
	AggregatedPrice float64 `json:"aggregated_price"`

	// Sources is a map of provider names to their reported prices for auditing.
	Sources map[string]float64 `json:"sources"`

	// UpdatedAt is the ISO-8601 timestamp of when the price was last aggregated.
	UpdatedAt time.Time `json:"updated_at"`
}

// AlertRequest defines the input payload for registering a new price alert.
// Success triggers a creation of a new WebhookSubscription in the domain layer.
type AlertRequest struct {
	// Symbol is the asset ticker to monitor (e.g., "BTC", "ETH").
	// Must be currently tracked by the system.
	Symbol string `json:"symbol" validate:"required,uppercase"`

	// TargetURL is a valid HTTP/HTTPS endpoint that will receive
	// POST notifications when the threshold is reached.
	TargetURL string `json:"target_url" validate:"required,url"`

	// ThresholdPercent specifies the relative price change required to trigger an alert.
	// Measured in percentage points (e.g., 5.0 for 5%). Must be greater than 0.
	ThresholdPercent float64 `json:"threshold_percent" validate:"required,gt=0"`
}

// ErrorResponse is a standardised error envelope for all non-2xx API responses.
// It provides a machine-readable code alongside a human-readable message.
type ErrorResponse struct {
	// Error is a short, human-readable description of the problem.
	Error string `json:"error"`

	// Code is an optional machine-readable error code for programmatic consumption.
	Code string `json:"code,omitempty"`
}

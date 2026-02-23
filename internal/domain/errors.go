package domain

import "errors"

var (
	// ErrAssetNotFound is returned when a requested asset symbol does not exist in the cache.
	ErrAssetNotFound = errors.New("asset not found in cache")
	// ErrSourceUnavailable is returned when an external data source cannot be reached.
	ErrSourceUnavailable = errors.New("data source is currently unavailable")
	// ErrInvalidThreshold is returned when a webhook threshold is not positive.
	ErrInvalidThreshold = errors.New("threshold percent must be greater than 0")
)

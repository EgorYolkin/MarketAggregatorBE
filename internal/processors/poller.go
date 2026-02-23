package processors

import (
	"context"
	"log/slog"
	"time"
)

// MarketDataUsecase defines the contract for the core aggregation logic.
type MarketDataUsecase interface {
	CollectAndAggregate(ctx context.Context, symbols []string) error
}

// PollingWorker periodically triggers data collection for configured asset symbols.
type PollingWorker struct {
	usecase  MarketDataUsecase
	symbols  []string      // List of assets from configuration
	interval time.Duration // Configurable polling interval
}

// NewPollingWorker creates a PollingWorker with the given symbols and tick interval.
func NewPollingWorker(u MarketDataUsecase, symbols []string, interval time.Duration) *PollingWorker {
	return &PollingWorker{
		usecase:  u,
		symbols:  symbols,
		interval: interval,
	}
}

// Start begins the polling loop. It runs an immediate first poll and then ticks at the configured interval.
func (w *PollingWorker) Start(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	slog.InfoContext(ctx, "poller: starting initial poll", "symbols", w.symbols)

	// Immediate first run without waiting for the first tick
	if err := w.usecase.CollectAndAggregate(ctx, w.symbols); err != nil {
		slog.ErrorContext(ctx, "poller: initial poll failed", "symbols", w.symbols, "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			slog.Info("poller: stopped")
			return ctx.Err()
		case <-ticker.C:
			slog.DebugContext(ctx, "poller: tick", "symbols", w.symbols)
			if err := w.usecase.CollectAndAggregate(ctx, w.symbols); err != nil {
				slog.ErrorContext(ctx, "poller: poll iteration failed", "symbols", w.symbols, "error", err)
			}
		}
	}
}

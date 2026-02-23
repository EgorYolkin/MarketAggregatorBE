// Package usecase implements the core business logic for market data aggregation and webhooks.
package usecase

import (
	"context"
	"log/slog"
	"math"
	"sort"
	"sync"
	"time"

	"market_aggregator/internal/domain"
)

const (
	// fetchTimeout is the maximum time allowed for all providers to respond.
	// Acts as a safety net on top of per-provider HTTP timeouts.
	fetchTimeout = 10 * time.Second

	// webhookTimeout is the maximum time allowed for a single webhook trigger cycle.
	// Prevents a hung callback target from leaking goroutines.
	webhookTimeout = 5 * time.Second
)

// MarketDataService orchestrates the specialized workflow of collection,
// normalization, and aggregation of market data from disparate sources.
type MarketDataService struct {
	providers []MarketDataProvider
	cache     CacheRepository
	webhook   *WebhookService
}

// NewMarketDataService creates a new service with the given providers, cache, and webhook subsystem.
func NewMarketDataService(p []MarketDataProvider, c CacheRepository, w *WebhookService) *MarketDataService {
	return &MarketDataService{
		providers: p,
		cache:     c,
		webhook:   w,
	}
}

// CollectAndAggregate triggers a full data acquisition cycle. It performs parallel fetching
// from all configured providers, calculates a resilient median price, and updates the cache.
// Significant price deviations will trigger asynchronous webhook notifications.
//
// The method enforces a hard timeout on the entire fetch phase to prevent a single
// hung provider from stalling the aggregation pipeline indefinitely.
func (s *MarketDataService) CollectAndAggregate(ctx context.Context, symbols []string) error {
	// Create a scoped context with a hard timeout for the fetch phase.
	fetchCtx, fetchCancel := context.WithTimeout(ctx, fetchTimeout)
	defer fetchCancel()

	results := make([]fetchResult, len(s.providers))
	var wg sync.WaitGroup

	// 1. Parallel polling of data sources
	for i, provider := range s.providers {
		wg.Add(1)
		go func(idx int, p MarketDataProvider) {
			defer wg.Done()

			prices, err := p.FetchPrices(fetchCtx, symbols)
			results[idx] = fetchResult{
				ProviderName: p.Name(),
				Prices:       prices,
				Err:          err,
			}
		}(i, provider)
	}
	wg.Wait()

	// 2. Filtering and grouping
	// Structure: map[Symbol]map[ProviderName]Price
	groupedPrices := make(map[string]map[string]float64)

	for _, res := range results {
		if res.Err != nil {
			slog.ErrorContext(ctx, "provider fetch failed",
				"provider", res.ProviderName,
				"error", res.Err,
			)
			continue
		}

		for sym, price := range res.Prices {
			if groupedPrices[sym] == nil {
				groupedPrices[sym] = make(map[string]float64)
			}
			groupedPrices[sym][res.ProviderName] = price
		}
	}

	// 3. Aggregation (median) and caching
	now := time.Now().UTC()
	for sym, sources := range groupedPrices {
		if len(sources) == 0 {
			continue
		}

		aggPrice := calculateMedian(sources)

		asset := domain.AssetPrice{
			Symbol:          sym,
			AggregatedPrice: aggPrice,
			Sources:         sources,
			UpdatedAt:       now,
		}

		if err := s.cache.SetAsset(ctx, asset); err != nil {
			slog.ErrorContext(ctx, "failed to cache aggregated price",
				"symbol", sym,
				"error", err,
			)
			continue
		}

		// 4. Webhook trigger check.
		// Run asynchronously with a bounded timeout so a slow callback
		// target cannot block the aggregation loop or leak goroutines.
		webhookCtx, webhookCancel := context.WithTimeout(context.Background(), webhookTimeout)
		go func(cancel context.CancelFunc, a domain.AssetPrice) {
			defer cancel()
			s.webhook.CheckTriggers(webhookCtx, a)
		}(webhookCancel, asset)
	}

	return nil
}

// GetAsset retrieves a single asset from the cache layer.
func (s *MarketDataService) GetAsset(ctx context.Context, symbol string) (domain.AssetPrice, error) {
	return s.cache.GetAsset(ctx, symbol)
}

// GetAllAssets retrieves all currently cached assets.
func (s *MarketDataService) GetAllAssets(ctx context.Context) ([]domain.AssetPrice, error) {
	return s.cache.GetAllAssets(ctx)
}

// calculateMedian extracts prices from the map and applies the median formula.
// Returns 0 for an empty input map (defensive guard).
//
// NOTE: float64 is used for prices as an acceptable trade-off for a crypto
// aggregator. For stablecoin-grade precision (e.g. USDT ≈ 1.0000) consider
// shopspring/decimal, but for volatile assets the ~15 significant digits of
// IEEE-754 double are more than sufficient.
func calculateMedian(sources map[string]float64) float64 {
	if len(sources) == 0 {
		return 0
	}

	prices := make([]float64, 0, len(sources))
	for _, p := range sources {
		// Guard against corrupted data from upstream adapters:
		// NaN propagates through arithmetic silently, Inf skews the median.
		if math.IsNaN(p) || math.IsInf(p, 0) {
			continue
		}
		prices = append(prices, p)
	}

	if len(prices) == 0 {
		return 0
	}

	sort.Float64s(prices)
	n := len(prices)

	if n%2 != 0 {
		return prices[n/2]
	}
	return (prices[n/2-1] + prices[n/2]) / 2.0
}

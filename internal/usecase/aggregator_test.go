package usecase

import (
	"context"
	"math"
	"testing"
	"time"

	"market_aggregator/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockMarketDataProvider
type MockMarketDataProvider struct {
	mock.Mock
}

func (m *MockMarketDataProvider) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockMarketDataProvider) FetchPrices(ctx context.Context, symbols []string) (map[string]float64, error) {
	args := m.Called(ctx, symbols)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]float64), args.Error(1)
}

// MockCacheRepository
type MockCacheRepository struct {
	mock.Mock
}

func (m *MockCacheRepository) SetAsset(ctx context.Context, asset domain.AssetPrice) error {
	args := m.Called(ctx, asset)
	return args.Error(0)
}

func (m *MockCacheRepository) GetAsset(ctx context.Context, symbol string) (domain.AssetPrice, error) {
	args := m.Called(ctx, symbol)
	return args.Get(0).(domain.AssetPrice), args.Error(1)
}

func (m *MockCacheRepository) GetAllAssets(ctx context.Context) ([]domain.AssetPrice, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AssetPrice), args.Error(1)
}

func (m *MockCacheRepository) Ping(_ context.Context) error {
	return nil
}

// MockWebhookRepository
type MockWebhookRepository struct {
	mock.Mock
}

func (m *MockWebhookRepository) SaveSubscription(ctx context.Context, sub domain.WebhookSubscription) error {
	args := m.Called(ctx, sub)
	return args.Error(0)
}

func (m *MockWebhookRepository) GetSubscriptionsBySymbol(ctx context.Context, symbol string) ([]domain.WebhookSubscription, error) {
	args := m.Called(ctx, symbol)
	return args.Get(0).([]domain.WebhookSubscription), args.Error(1)
}

func (m *MockWebhookRepository) UpdateBasePrice(ctx context.Context, id string, newPrice float64) error {
	args := m.Called(ctx, id, newPrice)
	return args.Error(0)
}

func TestCalculateMedian(t *testing.T) {
	tests := []struct {
		name     string
		prices   map[string]float64
		expected float64
	}{
		{"empty", map[string]float64{}, 0},
		{"single", map[string]float64{"a": 100}, 100},
		{"odd number", map[string]float64{"a": 100, "b": 200, "c": 300}, 200},
		{"even number", map[string]float64{"a": 100, "b": 200, "c": 300, "d": 400}, 250},
		{"unsorted", map[string]float64{"a": 300, "b": 100, "c": 200}, 200},
		{"with outliers", map[string]float64{"a": 100, "b": 101, "c": 102, "d": 10000}, 101.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := calculateMedian(tt.prices)
			assert.Equal(t, tt.expected, res)
		})
	}
}

func FuzzCalculateMedian(f *testing.F) {
	f.Add(100.0, 200.0, 300.0)
	f.Add(0.0, 0.0, 0.0)
	f.Add(-100.0, 50.0, 10.0)

	f.Fuzz(func(t *testing.T, a, b, c float64) {
		prices := map[string]float64{
			"a": a,
			"b": b,
			"c": c,
		}
		res := calculateMedian(prices)

		// Ensure it doesn't panic and result is not NaN for finite inputs
		if !math.IsNaN(a) && !math.IsNaN(b) && !math.IsNaN(c) &&
			!math.IsInf(a, 0) && !math.IsInf(b, 0) && !math.IsInf(c, 0) {
			assert.False(t, math.IsNaN(res))
		}
	})
}

func TestMarketDataService_CollectAndAggregate(t *testing.T) {
	p1 := new(MockMarketDataProvider)
	p2 := new(MockMarketDataProvider)
	cache := new(MockCacheRepository)
	repo := new(MockWebhookRepository)

	evChan := make(chan WebhookEvent, 10)
	ws := NewWebhookService(repo, evChan)

	p1.On("Name").Return("Prov1")
	p2.On("Name").Return("Prov2")

	// Prov1 returns data
	p1.On("FetchPrices", mock.Anything, []string{"BTC"}).Return(map[string]float64{"BTC": 50000.0}, nil)
	// Prov2 returns error
	p2.On("FetchPrices", mock.Anything, []string{"BTC"}).Return(nil, domain.ErrSourceUnavailable)

	cache.On("SetAsset", mock.Anything, mock.MatchedBy(func(a domain.AssetPrice) bool {
		return a.Symbol == "BTC" && a.AggregatedPrice == 50000.0 && len(a.Sources) == 1
	})).Return(nil)

	repo.On("GetSubscriptionsBySymbol", mock.Anything, "BTC").Return([]domain.WebhookSubscription{}, nil)

	s := NewMarketDataService([]MarketDataProvider{p1, p2}, cache, ws)

	err := s.CollectAndAggregate(context.Background(), []string{"BTC"})
	assert.NoError(t, err)

	// Wait briefly for webhook goroutine to finish
	time.Sleep(10 * time.Millisecond)

	p1.AssertExpectations(t)
	p2.AssertExpectations(t)
	cache.AssertExpectations(t)
	repo.AssertExpectations(t)
}

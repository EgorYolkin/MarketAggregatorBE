package usecase

import (
	"context"
	"testing"
	"time"

	"market_aggregator/internal/domain"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockWebhookRepo struct {
	mock.Mock
}

func (m *MockWebhookRepo) SaveSubscription(ctx context.Context, sub domain.WebhookSubscription) error {
	args := m.Called(ctx, sub)
	return args.Error(0)
}

func (m *MockWebhookRepo) GetSubscriptionsBySymbol(ctx context.Context, symbol string) ([]domain.WebhookSubscription, error) {
	args := m.Called(ctx, symbol)
	return args.Get(0).([]domain.WebhookSubscription), args.Error(1)
}

func (m *MockWebhookRepo) UpdateBasePrice(ctx context.Context, id string, newPrice float64) error {
	args := m.Called(ctx, id, newPrice)
	return args.Error(0)
}

func TestWebhookService_CheckTriggers(t *testing.T) {
	repo := new(MockWebhookRepo)
	eventChan := make(chan WebhookEvent, 10)
	ws := NewWebhookService(repo, eventChan)

	subs := []domain.WebhookSubscription{
		{
			ID:               uuid.New(),
			Symbol:           "BTC",
			TargetURL:        "http://test.com",
			ThresholdPercent: 5.0,
			BasePrice:        50000.0, // Trigger at <= 47500 or >= 52500
		},
		{
			ID:               uuid.New(),
			Symbol:           "BTC",
			TargetURL:        "http://test2.com",
			ThresholdPercent: 1.0,
			BasePrice:        0.0, // Division by zero protection
		},
	}

	repo.On("GetSubscriptionsBySymbol", mock.Anything, "BTC").Return(subs, nil)

	// Test 1: Price goes up, triggers first sub
	ws.CheckTriggers(context.Background(), domain.AssetPrice{
		Symbol:          "BTC",
		AggregatedPrice: 53000.0,
	})

	select {
	case event := <-eventChan:
		assert.Equal(t, "BTC", event.Symbol)
		assert.Equal(t, 53000.0, event.NewPrice)
		assert.Equal(t, subs[0].ID.String(), event.SubscriptionID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected an event to be sent")
	}

	// Make sure no more events
	assert.Equal(t, 0, len(eventChan))

	repo.AssertExpectations(t)
}

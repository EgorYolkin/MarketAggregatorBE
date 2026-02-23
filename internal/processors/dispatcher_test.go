package processors

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"market_aggregator/internal/usecase"

	"github.com/stretchr/testify/assert"
)

// MockWebhookRepository intercepts repository calls in the dispatcher phase.
type MockWebhookRepository struct {
	UpdateBasePriceFunc func(ctx context.Context, subID string, price float64) error
	UpdateCallsCount    int
}

func (m *MockWebhookRepository) UpdateBasePrice(ctx context.Context, subID string, price float64) error {
	m.UpdateCallsCount++
	if m.UpdateBasePriceFunc != nil {
		return m.UpdateBasePriceFunc(ctx, subID, price)
	}
	return nil
}

func TestWebhookDispatcher_DispatchSuccess(t *testing.T) {
	// 1. Spawning a test HTTP server (mocking the client server)
	serverHit := false
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		serverHit = true
		w.WriteHeader(http.StatusOK) // Respond with 200 OK
	}))
	defer testServer.Close()

	// 2. Mocking the repository to verify UpdateBasePrice calls
	mockRepo := &MockWebhookRepository{
		UpdateBasePriceFunc: func(_ context.Context, subID string, price float64) error {
			assert.Equal(t, "sub-123", subID)
			assert.Equal(t, 55000.0, price)
			return nil
		},
	}

	eventChan := make(chan usecase.WebhookEvent, 1)
	dispatcher := NewWebhookDispatcher(eventChan, mockRepo)

	// 3. Send event to the channel
	eventChan <- usecase.WebhookEvent{
		SubscriptionID: "sub-123",
		TargetURL:      testServer.URL, // Point to the test server's URL!
		Symbol:         "BTC",
		OldPrice:       50000.0,
		NewPrice:       55000.0,
	}

	// 4. Start the dispatcher in a goroutine
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = dispatcher.Start(ctx) }()

	// Give the dispatcher some time to process
	time.Sleep(50 * time.Millisecond)
	cancel() // Stop the worker

	// 5. Verify results
	assert.True(t, serverHit, "Webhook target URL was never called")
	assert.Equal(t, 1, mockRepo.UpdateCallsCount, "Base price was not updated in repository")
}

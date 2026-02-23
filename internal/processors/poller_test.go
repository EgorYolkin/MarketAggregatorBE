package processors

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockMarketDataUsecase mocks the actual Usecase layer dependency
type MockMarketDataUsecase struct {
	CollectFunc func(ctx context.Context, symbols []string) error
	calls       int
}

func (m *MockMarketDataUsecase) CollectAndAggregate(ctx context.Context, symbols []string) error {
	m.calls++
	if m.CollectFunc != nil {
		return m.CollectFunc(ctx, symbols)
	}
	return nil
}

func (m *MockMarketDataUsecase) CallsCount() int {
	return m.calls
}

func TestPollingWorker_StartAndCancel(t *testing.T) {
	mockUsecase := &MockMarketDataUsecase{
		CollectFunc: func(_ context.Context, _ []string) error {
			return nil
		},
	}

	worker := NewPollingWorker(mockUsecase, []string{"BTC"}, 10*time.Millisecond)

	// Create a context that expires after 25 milliseconds
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()

	// Start the worker. It will block until the context is cancelled.
	err := worker.Start(ctx)

	// Verify the worker exited due to context cancellation
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// Verify calls: 1 immediate + 2 on ticks (at 10ms and 20ms)
	assert.GreaterOrEqual(t, mockUsecase.CallsCount(), 2)
}

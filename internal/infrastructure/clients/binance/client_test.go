package binance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_FetchPrices(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockResponse := `[
			{"symbol":"BTCUSDT","price":"60000.00"},
			{"symbol":"ETHUSDT","price":"3000.00"},
			{"symbol":"SOLUSDT","price":"150.00"},
			{"symbol":"DOGEUSDT","price":"0.15"}
		]`

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != tickerPriceEndpoint {
				t.Errorf("expected path %q, got %q", tickerPriceEndpoint, r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(mockResponse))
		}))
		defer server.Close()

		client := NewClient(
			WithBaseURL(server.URL),
			WithHTTPClient(server.Client()),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		symbols := []string{"BTC", "ETH", "UNKNOWN"}
		prices, err := client.FetchPrices(ctx, symbols)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(prices) != 2 {
			t.Fatalf("expected 2 parsed prices, got %d", len(prices))
		}

		if val, err := prices["BTC"]; !err || val != 60000.00 {
			t.Errorf("expected BTC price 60000.00, got %v", val)
		}
		if val, err := prices["ETH"]; !err || val != 3000.00 {
			t.Errorf("expected ETH price 3000.00, got %v", val)
		}
	})

	t.Run("empty symbols", func(t *testing.T) {
		client := NewClient()
		prices, err := client.FetchPrices(context.Background(), []string{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(prices) != 0 {
			t.Fatalf("expected empty map, got %d items", len(prices))
		}
	})

	t.Run("integration test with real api", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}

		client := NewClient()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		prices, err := client.FetchPrices(ctx, []string{"BTC"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(prices) == 0 {
			t.Fatalf("expected BTC price, got empty map")
		}
		if prices["BTC"] <= 0 {
			t.Fatalf("expected BTC price > 0, got %f", prices["BTC"])
		}
	})
}

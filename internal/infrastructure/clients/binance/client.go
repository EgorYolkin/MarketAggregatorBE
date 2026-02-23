// Package binance implements a client for the Binance public market data API.
package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"market_aggregator/internal/domain"
)

const (
	defaultBaseURL      = "https://api.binance.com"
	tickerPriceEndpoint = "/api/v3/ticker/price"
)

// Client is a Binance API client.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// Option configures the Client.
type Option func(*Client)

// WithHTTPClient allows setting a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithBaseURL allows customizing the base URL.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

// NewClient creates a new Binance client.
func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:      10,
				IdleConnTimeout:   30 * time.Second,
				DisableKeepAlives: false,
			},
		},
		baseURL: defaultBaseURL,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Name returns the provider name used for attribution in aggregated results.
func (c *Client) Name() string {
	return "Binance"
}

// FetchPrices fetches current USDT prices for the specified base symbols.
func (c *Client) FetchPrices(ctx context.Context, symbols []string) (map[string]float64, error) {
	if len(symbols) == 0 {
		return make(map[string]float64), nil
	}

	var dtos []TickerResponse
	var errDo error

	for attempt := 1; attempt <= 3; attempt++ {
		endpoint := fmt.Sprintf("%s%s", c.baseURL, tickerPriceEndpoint)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		resp, err := c.httpClient.Do(req) //nolint:gosec // URL is constructed from a configured baseURL, not user input.
		if err != nil {
			errDo = err
		} else {
			if resp.StatusCode == http.StatusOK {
				if decodeErr := json.NewDecoder(resp.Body).Decode(&dtos); decodeErr != nil {
					_ = resp.Body.Close()
					return nil, fmt.Errorf("decode response: %w", decodeErr)
				}
				_ = resp.Body.Close()
				errDo = nil
				break
			}

			statusCode := resp.StatusCode
			_ = resp.Body.Close()
			if statusCode != http.StatusTooManyRequests && statusCode < http.StatusInternalServerError {
				return nil, fmt.Errorf("unexpected status code: %d", statusCode)
			}
			errDo = fmt.Errorf("server error: %d", statusCode)
		}

		if attempt < 3 {
			backoff := time.Duration(1<<attempt) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				continue
			}
		}
	}

	if errDo != nil {
		return nil, domain.ErrSourceUnavailable
	}

	result := make(map[string]float64, len(symbols))
	requestedSymbols := make(map[string]struct{}, len(symbols))

	for _, sym := range symbols {
		requestedSymbols[sym] = struct{}{}
	}

	for _, item := range dtos {
		if !strings.HasSuffix(item.Symbol, "USDT") {
			continue
		}

		baseSymbol := strings.TrimSuffix(item.Symbol, "USDT")
		if _, ok := requestedSymbols[baseSymbol]; !ok {
			continue
		}

		price, err := strconv.ParseFloat(item.Price, 64)
		if err != nil {
			continue
		}

		result[baseSymbol] = price
	}

	return result, nil
}

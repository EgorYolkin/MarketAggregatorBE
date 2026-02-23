// Package coingecko implements a client for the CoinGecko public market data API.
package coingecko

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"market_aggregator/internal/domain"
)

// Client queries the CoinGecko simple/price endpoint for USD-denominated prices.
type Client struct {
	httpClient *http.Client
	baseURL    string

	// symbolToID maps canonical uppercase tickers (e.g., "BTC") to CoinGecko slugs ("bitcoin").
	symbolToID map[string]string
	// idToSymbol is the reverse mapping used to convert API responses back to canonical symbols.
	idToSymbol map[string]string
}

// NewClient creates a CoinGecko client with a pre-configured symbol registry.
func NewClient() *Client {
	symbolToID := map[string]string{
		"BTC": "bitcoin",
		"ETH": "ethereum",
		"SOL": "solana",
		"ADA": "cardano",
		"XRP": "ripple",
	}

	idToSymbol := make(map[string]string)
	for k, v := range symbolToID {
		idToSymbol[v] = k
	}

	return &Client{
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:      10,
				IdleConnTimeout:   30 * time.Second,
				DisableKeepAlives: false,
			},
		},
		baseURL:    "https://api.coingecko.com/api/v3",
		symbolToID: symbolToID,
		idToSymbol: idToSymbol,
	}
}

// Name returns the provider name used for attribution in aggregated results.
func (c *Client) Name() string {
	return "CoinGecko"
}

// FetchPrices fetches current USD prices for the specified symbols via the /simple/price endpoint.
func (c *Client) FetchPrices(ctx context.Context, symbols []string) (map[string]float64, error) {
	var ids []string
	for _, sym := range symbols {
		if id, ok := c.symbolToID[sym]; ok {
			ids = append(ids, id)
		}
	}

	if len(ids) == 0 {
		return map[string]float64{}, nil
	}

	idsParam := strings.Join(ids, ",")
	endpoint := c.baseURL + "/simple/price?ids=" + idsParam + "&vs_currencies=usd"

	var dto Response
	var errDo error

	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}

		resp, err := c.httpClient.Do(req) //nolint:gosec // URL is constructed from a configured baseURL, not user input.
		if err != nil {
			errDo = err
		} else {
			if resp.StatusCode == http.StatusOK {
				if decodeErr := json.NewDecoder(resp.Body).Decode(&dto); decodeErr != nil {
					_ = resp.Body.Close()
					return nil, decodeErr
				}
				_ = resp.Body.Close()
				errDo = nil
				break
			}

			statusCode := resp.StatusCode
			_ = resp.Body.Close()
			if statusCode != http.StatusTooManyRequests && statusCode < http.StatusInternalServerError {
				return nil, domain.ErrSourceUnavailable
			}
			errDo = domain.ErrSourceUnavailable
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

	result := make(map[string]float64)
	for id, currencyMap := range dto {
		if price, ok := currencyMap["usd"]; ok {
			if sym, ok := c.idToSymbol[id]; ok {
				result[sym] = price
			}
		}
	}

	return result, nil
}

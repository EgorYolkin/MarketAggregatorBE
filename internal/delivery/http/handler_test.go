package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"market_aggregator/internal/delivery/http"
	"market_aggregator/internal/domain"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockMarketData struct {
	mock.Mock
}

func (m *MockMarketData) GetAsset(ctx context.Context, symbol string) (domain.AssetPrice, error) {
	args := m.Called(ctx, symbol)
	return args.Get(0).(domain.AssetPrice), args.Error(1)
}

func (m *MockMarketData) GetAllAssets(ctx context.Context) ([]domain.AssetPrice, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]domain.AssetPrice), args.Error(1)
}

type MockWebhook struct {
	mock.Mock
}

func (m *MockWebhook) RegisterSubscription(ctx context.Context, sub domain.WebhookSubscription) error {
	args := m.Called(ctx, sub)
	return args.Error(0)
}

type MockHealth struct {
	mock.Mock
}

func (m *MockHealth) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestHandler_GetAsset(t *testing.T) {
	app := fiber.New()
	mockMD := new(MockMarketData)
	h := http.NewHandler(mockMD, new(MockWebhook), new(MockHealth))
	app.Get("/v1/assets/:symbol", h.GetAsset)

	t.Run("Success", func(t *testing.T) {
		asset := domain.AssetPrice{
			Symbol:          "BTC",
			AggregatedPrice: 50000,
			Sources:         map[string]float64{"Prov1": 50000},
			UpdatedAt:       time.Now(),
		}
		mockMD.On("GetAsset", mock.Anything, "BTC").Return(asset, nil).Once()

		req := httptest.NewRequest("GET", "/v1/assets/BTC", nil)
		resp, _ := app.Test(req)

		assert.Equal(t, fiber.StatusOK, resp.StatusCode)

		var res http.AssetResponse
		_ = json.NewDecoder(resp.Body).Decode(&res)
		assert.Equal(t, "BTC", res.Symbol)
		assert.Equal(t, 50000.0, res.AggregatedPrice)
	})

	t.Run("Not Found", func(t *testing.T) {
		mockMD.On("GetAsset", mock.Anything, "ETH").Return(domain.AssetPrice{}, domain.ErrAssetNotFound).Once()

		req := httptest.NewRequest("GET", "/v1/assets/ETH", nil)
		resp, _ := app.Test(req)

		assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)
	})
}

func TestHandler_RegisterAlert(t *testing.T) {
	app := fiber.New()
	mockMD := new(MockMarketData)
	mockWH := new(MockWebhook)
	h := http.NewHandler(mockMD, mockWH, new(MockHealth))
	app.Post("/v1/alerts", h.RegisterAlert)

	t.Run("Success", func(t *testing.T) {
		reqBody := http.AlertRequest{
			Symbol:           "BTC",
			TargetURL:        "https://example.com/webhook",
			ThresholdPercent: 5.0,
		}
		body, _ := json.Marshal(reqBody)

		mockMD.On("GetAsset", mock.Anything, "BTC").Return(domain.AssetPrice{AggregatedPrice: 50000}, nil).Once()
		mockWH.On("RegisterSubscription", mock.Anything, mock.MatchedBy(func(s domain.WebhookSubscription) bool {
			return s.Symbol == "BTC" && s.ThresholdPercent == 5.0 && s.BasePrice == 50000.0
		})).Return(nil).Once()

		req := httptest.NewRequest("POST", "/v1/alerts", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)

		assert.Equal(t, fiber.StatusCreated, resp.StatusCode)
	})

	t.Run("Invalid Payload - Bad Threshold", func(t *testing.T) {
		reqBody := http.AlertRequest{
			Symbol:           "BTC",
			TargetURL:        "https://example.com/webhook",
			ThresholdPercent: -5.0,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/v1/alerts", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := app.Test(req)

		assert.Equal(t, fiber.StatusBadRequest, resp.StatusCode)
	})
}

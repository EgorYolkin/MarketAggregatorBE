package http_test

import (
	"net/http/httptest"
	"testing"

	"market_aggregator/internal/delivery/http"
	"market_aggregator/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRouter_Integration(t *testing.T) {
	mockMD := new(MockMarketData)
	mockWH := new(MockWebhook)
	mockH := new(MockHealth)

	handler := http.NewHandler(mockMD, mockWH, mockH)

	app := http.NewFiberApp(http.Config{
		LoggerFormat: "", // Disable logging in tests
	})

	http.MapV1Routes(app, handler)

	t.Run("HealthCheck Route", func(t *testing.T) {
		mockH.On("Ping", mock.Anything).Return(nil).Once()

		req := httptest.NewRequest("GET", "/v1/health", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("ListAssets Route", func(t *testing.T) {
		mockMD.On("GetAllAssets", mock.Anything).Return([]domain.AssetPrice{}, nil).Once()

		req := httptest.NewRequest("GET", "/v1/assets", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, 200, resp.StatusCode)
	})

	t.Run("Swagger Route Exists", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/swagger/index.html", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		// Usually returns 200 if Swagger UI is loaded correctly
		assert.Equal(t, 200, resp.StatusCode)
	})
}

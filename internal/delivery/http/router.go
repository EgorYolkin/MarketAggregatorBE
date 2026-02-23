// Package http implements the HTTP delivery layer using Fiber.
package http //nolint:revive // intentional: delivery/http is a common Go package layout pattern.

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/swagger"
)

// Config holds the configuration constraints for the HTTP server.
type Config struct {
	Port         string
	LoggerFormat string
}

// NewFiberApp initializes a new Fiber app with configured middlewares.
func NewFiberApp(_ Config) *fiber.App {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// Add recover middleware to prevent panics from breaking the application.
	app.Use(recover.New())

	// Structured JSON request logger using slog instead of Fiber's default text format.
	// This ensures all application output is unified as JSON.
	app.Use(slogRequestLogger())

	return app
}

// slogRequestLogger returns a Fiber middleware that logs every HTTP request via slog.
func slogRequestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Process the request
		err := c.Next()

		slog.Info("http request",
			"method", c.Method(),
			"path", c.Path(),
			"status", c.Response().StatusCode(),
			"latency_ms", float64(time.Since(start).Microseconds())/1000.0,
			"ip", c.IP(),
		)

		return err
	}
}

// MapV1Routes registers the /v1 API routes on the provided fiber Router.
func MapV1Routes(router fiber.Router, handler *Handler) {
	// Serve static OpenAPI specification
	router.Static("/swagger/openapi.yaml", "./api/openapi.yaml")
	// Enable Swagger UI
	router.Get("/swagger/*", swagger.New(swagger.Config{
		URL: "/swagger/openapi.yaml",
	}))

	v1 := router.Group("/v1")

	v1.Get("/health", handler.HealthCheck)
	v1.Get("/assets", handler.ListAssets)
	v1.Get("/assets/:symbol", handler.GetAsset)
	v1.Post("/alerts", handler.RegisterAlert)

	// Debug endpoint: accepts webhook delivery callbacks for local testing.
	// Use http://localhost:8080/test_alerts as target_url when registering alerts.
	router.Post("/test_alerts", func(c *fiber.Ctx) error {
		slog.Info("webhook received on /test_alerts", "payload", string(c.Body()))
		return c.JSON(fiber.Map{"status": "received"})
	})
}

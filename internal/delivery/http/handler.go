package http

import (
	"context"
	"errors"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"market_aggregator/internal/domain"
)

// validate is a package-level validator instance reused across all handler calls.
// Thread-safe by design.
var validate = validator.New(validator.WithRequiredStructEnabled())

// MarketDataService interfaces the core logic for retrieving asset data.
type MarketDataService interface {
	GetAsset(ctx context.Context, symbol string) (domain.AssetPrice, error)
	GetAllAssets(ctx context.Context) ([]domain.AssetPrice, error)
}

// WebhookService interfaces the logic for webhook registration.
type WebhookService interface {
	RegisterSubscription(ctx context.Context, sub domain.WebhookSubscription) error
}

// HealthChecker interfaces the ping check for readiness checks.
type HealthChecker interface {
	Ping(ctx context.Context) error
}

// Handler acts as the primary HTTP adapter for the system.
// It translates incoming RESTful requests into structured domain usecase calls
// and manages the serialization of domain entities into public-facing DTOs.
type Handler struct {
	marketData MarketDataService
	webhook    WebhookService
	health     HealthChecker
}

// NewHandler creates a new instance of Handler with defined dependencies.
func NewHandler(m MarketDataService, w WebhookService, h HealthChecker) *Handler {
	return &Handler{
		marketData: m,
		webhook:    w,
		health:     h,
	}
}

// HealthCheck verifies the readiness of the application by checking the state of its
// critical infrastructure dependencies. Returns 200 OK or 503 Service Unavailable.
func (h *Handler) HealthCheck(c *fiber.Ctx) error {
	if err := h.health.Ping(c.UserContext()); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(ErrorResponse{
			Error: "service unavailable: redis not responding",
		})
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"status": "ok",
	})
}

// ListAssets retrieves a snapshot of all currently tracked assets and their calculated median prices.
// Returns a 200 OK response with an array of AssetResponse DTOs.
func (h *Handler) ListAssets(c *fiber.Ctx) error {
	assets, err := h.marketData.GetAllAssets(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{
			Error: "failed to retrieve assets",
		})
	}

	responses := make([]AssetResponse, 0, len(assets))
	for _, a := range assets {
		responses = append(responses, AssetResponse{
			Symbol:          a.Symbol,
			AggregatedPrice: a.AggregatedPrice,
			Sources:         a.Sources,
			UpdatedAt:       a.UpdatedAt,
		})
	}

	return c.JSON(responses)
}

// GetAsset retrieves the full breakdown for a single specific asset identified by its symbol.
// Returns 200 OK or 404 Not Found if the asset is not tracked or has expired from the cache.
func (h *Handler) GetAsset(c *fiber.Ctx) error {
	symbol := c.Params("symbol")
	if symbol == "" {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "symbol is required"})
	}

	asset, err := h.marketData.GetAsset(c.UserContext(), symbol)
	if err != nil {
		if errors.Is(err, domain.ErrAssetNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{Error: "asset not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to retrieve asset"})
	}

	return c.JSON(AssetResponse{
		Symbol:          asset.Symbol,
		AggregatedPrice: asset.AggregatedPrice,
		Sources:         asset.Sources,
		UpdatedAt:       asset.UpdatedAt,
	})
}

// RegisterAlert accepts a client's request for price change notifications.
// It validates the payload using struct tags, persists the registration as a
// WebhookSubscription, and returns 201 Created on success.
func (h *Handler) RegisterAlert(c *fiber.Ctx) error {
	var req AlertRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{Error: "invalid request body"})
	}

	// Struct-level validation via go-playground/validator tags.
	if err := validate.Struct(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(ErrorResponse{
			Error: "validation failed: " + err.Error(),
			Code:  "VALIDATION_ERROR",
		})
	}

	// Current Price Lookup (to act as BasePrice)
	asset, err := h.marketData.GetAsset(c.UserContext(), req.Symbol)
	if err != nil {
		if errors.Is(err, domain.ErrAssetNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(ErrorResponse{Error: "asset not tracked or unavailable"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to fetch asset base price"})
	}

	sub := domain.WebhookSubscription{
		ID:               uuid.New(),
		Symbol:           req.Symbol,
		TargetURL:        req.TargetURL,
		ThresholdPercent: req.ThresholdPercent,
		BasePrice:        asset.AggregatedPrice,
	}

	if err := h.webhook.RegisterSubscription(c.UserContext(), sub); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(ErrorResponse{Error: "failed to register webhook"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message":         "webhook registered successfully",
		"subscription_id": sub.ID.String(),
		"base_price":      sub.BasePrice,
	})
}

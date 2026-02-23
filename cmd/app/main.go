/*
Package main entry point for the Market Aggregator Service.
The service is a high-performance price aggregator that:
1. Polls multiple external crypto exchanges (Binance, CoinGecko) concurrently.
2. Normalizes and aggregates data using a median-based approach to mitigate outliers.
3. Caches results in Redis with strict TTL for data freshness.
4. Monitors price triggers and dispatches webhook alerts to registered listeners.

The application follows Clean Architecture principles and ensures reliability through
exponential backoff retries and graceful shutdown orchestration.
*/
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"market_aggregator/internal/config"
	httpDelivery "market_aggregator/internal/delivery/http"
	"market_aggregator/internal/infrastructure/clients/binance"
	"market_aggregator/internal/infrastructure/clients/coingecko"
	redisRepo "market_aggregator/internal/infrastructure/repository/redis"
	"market_aggregator/internal/processors"
	"market_aggregator/internal/usecase"
	"market_aggregator/pkg/logger"

	"github.com/redis/go-redis/v9"
)

func main() {
	// 1. Load Configuration (Fail-Fast)
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatal("Fatal err reading config", "error", err)
	}

	// 2. Initialize Logger
	if err := logger.InitLogger(logger.Config{
		Level:           cfg.App.LogLevel,
		Format:          cfg.App.LogFormat,
		TimestampFormat: time.RFC3339,
		EnableConsole:   true,
	}); err != nil {
		logger.Fatal("Failed to initialize logger", "error", err)
	}
	logger.Log.Info("Starting Market Aggregator Service")

	// 3. Infrastructure: Redis
	redisAddr := fmt.Sprintf("%s:%s", cfg.Redis.Host, cfg.Redis.Port)
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Fatal("Failed to connect to Redis", "address", redisAddr, "error", err)
	}
	logger.Log.Info("Connected to Redis successfully")

	repo := redisRepo.NewRepository(redisClient)

	// 4. Infrastructure: API Clients
	binanceClient := binance.NewClient()
	coinGeckoClient := coingecko.NewClient()
	providers := []usecase.MarketDataProvider{binanceClient, coinGeckoClient}

	// 5. Usecases (Business Logic)
	webhookChan := make(chan usecase.WebhookEvent, 100)
	webhookService := usecase.NewWebhookService(repo, webhookChan)
	marketDataService := usecase.NewMarketDataService(providers, repo, webhookService)

	// 6. Processors (Workers)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	poller := processors.NewPollingWorker(marketDataService, cfg.Worker.Assets, cfg.Worker.PollInterval)
	dispatcher := processors.NewWebhookDispatcher(webhookChan, repo)

	go func() {
		logger.Log.Info("Starting Polling Worker")
		if err := poller.Start(ctx); err != nil && err != context.Canceled {
			logger.Log.Error("Polling worker stopped with error", "error", err)
		}
	}()

	go func() {
		logger.Log.Info("Starting Webhook Dispatcher Worker")
		if err := dispatcher.Start(ctx); err != nil && err != context.Canceled {
			logger.Log.Error("Dispatcher worker stopped with error", "error", err)
		}
	}()

	// 7. Delivery (HTTP Fiber API)
	handler := httpDelivery.NewHandler(marketDataService, webhookService, repo)

	app := httpDelivery.NewFiberApp(httpDelivery.Config{
		Port:         cfg.App.Port,
		LoggerFormat: "", // Use fiber default
	})

	httpDelivery.MapV1Routes(app, handler)

	// Run HTTP Server
	go func() {
		portAddr := fmt.Sprintf(":%s", cfg.App.Port)
		logger.Log.Info("Starting HTTP server", "address", portAddr)
		if err := app.Listen(portAddr); err != nil {
			logger.Fatal("HTTP server failed", "error", err)
		}
	}()

	// 8. Graceful Shutdown
	<-ctx.Done() // Block until signal
	logger.Log.Info("Shutting down gracefully...")

	// Shutdown Fiber app with 5s timeout
	if err := app.ShutdownWithTimeout(5 * time.Second); err != nil {
		logger.Log.Error("Failed to shutdown HTTP server gracefully", "error", err)
	}

	// Close Redis connection
	if err := redisClient.Close(); err != nil {
		logger.Log.Error("Failed to close redis connection", "error", err)
	}

	logger.Log.Info("Shutdown complete")
}

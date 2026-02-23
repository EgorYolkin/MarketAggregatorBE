# Multi-Source Market Data Aggregator

![Go Version](https://img.shields.io/badge/go-1.24-blue.svg)
![Lint Status](https://img.shields.io/badge/golangci--lint-passing-brightgreen.svg)
![Tests](https://img.shields.io/badge/tests-passing-brightgreen.svg)
![License](https://img.shields.io/badge/license-MIT-blue.svg)

A resilient Go-based service designed to collect, normalize, and aggregate cryptocurrency exchange rates from multiple sources (Binance, CoinGecko). The service provides "cleaned" median prices via a REST API and a robust Webhook system with configurable change thresholds.

---

## ✨ Key Features

*   **Clean Architecture**: Strict separation of concerns (Domain, Usecase, Infrastructure, Delivery), ensuring maintainability, testability, and easy swapping of components (e.g., changing Redis to another DB).
*   **Resilient Aggregation**: Uses the **Median** approach instead of the arithmetic mean to protect against market anomalies, manipulations, or flash crashes on individual exchanges.
*   **Concurrent Polling**: High-performance data collection using optimized concurrency patterns (sync.WaitGroup), ensuring total latency is limited by the slowest provider response time.
*   **Fault Tolerance**: Continued operation even if some providers are unreachable. Features **Exponential Backoff** retries for external API clients.
*   **Observability**: Fully structured JSON logging via `slog`, ready for ELK/Loki integration.
*   **Production-Ready DevOps**: Docker Multi-stage builds using `scratch`, minimal image footprint, non-root user execution, and built-in Healthchecks.

---

## 🧠 Technical Decisions

### Why Median?
Unlike the arithmetic Mean, the Median is robust against outliers. If a single data source begins reporting incorrect prices (due to API bugs or market manipulation), the median ignores these extremes, maintaining the accuracy of the aggregate price. This is a standard practice for reliable Oracle systems.

### Why Redis?
Redis was chosen as a high-performance in-memory store for price caching and reliable webhook subscription persistence. It allows the application to remain stateless (in terms of local memory) and scale horizontally with ease.

### Using `slog`
Migrating to the standard library's `slog` reduced external dependencies and provided native support for structured logging. Logs include request context, latency, and status codes for better traceabilty.

### Concurrency Pattern
Provider data fetching occurs in parallel. We spawn Fetch requests in separate goroutines and wait for all to complete. This minimizes service latency, as the total aggregation time is roughly equal to the longest network request.

---

## 🛡️ Resiliency & Failure Handling

*   **Provider Failure**: If an exchange is down, the service logs a `WARN`, excludes that source from the current calculation, and continues its duty.
*   **Network Issues**: Exponential backoff retries are implemented for all outgoing HTTP requests to external APIs.
*   **Service Restart**: Webhook subscriptions are persisted in Redis, ensuring no data loss during application restarts.
*   **Graceful Shutdown**: The service cleanly terminates active goroutines and closes Redis connections upon receiving SIGTERM/SIGINT signals.

---

## 📡 API Documentation

The API specification is available in OpenAPI format: [api/openapi.yaml](api/openapi.yaml).

### Swagger UI
When the service is running, interactive documentation is available at:
`http://localhost:8080/swagger/index.html`

### Main Endpoints
*   `GET /v1/health` — Service health check.
*   `GET /v1/assets` — List all tracked assets and their aggregated median prices.
*   `GET /v1/assets/:symbol` — Breakdown of price data for a specific asset (including individual provider prices).
*   `POST /v1/alerts` — Register a webhook for price change notifications.
    *   *Payload:* `{"symbol": "BTC", "target_url": "http://your-app.com/callback", "threshold_percent": 1.5}`

---

## 🛠 Development & Deployment

### Environment Configuration
Copy the sample config or set environment variables:
*   `PORT`: Server port (default: 8080).
*   `REDIS_HOST`: Redis host.
*   `POLL_INTERVAL`: Polling frequency (default: 30s).
*   `TRACKED_ASSETS`: Comma-separated list of tickers (e.g., BTC,ETH).

### Quick Start (Docker Compose)
```bash
make up     # Spin up the app and Redis
make down   # Stop and remove containers
```

### Development Commands
```bash
make build  # Compile the binary to bin/
make test   # Run Unit, Integration (Testcontainers), and E2E tests
make lint   # Code quality check (golangci-lint)
make clean  # Remove build artifacts
```

---

## 📊 Observability

*   **JSON Logs**: All logs are written to stdout in JSON format.
*   **Request Logging**: Every HTTP request is logged with method, path, IP address, processing time, and status.
*   **Worker Context**: Pollers and dispatchers include relevant context (symbol, subscription ID) in logs for streamlined debugging.

---

## 🔮 Future Improvements

1.  **Circuit Breaker**: Implement patterns (e.g., `sony/gobreaker`) to prevent cascading failures during prolonged external API outages.
2.  **Websocket Support**: Migrate from a Polling model to Websockets for sub-second price updates.
3.  **Metrics**: Integrate Prometheus to collect metrics (request counts, provider error rates, aggregation latency).
4.  **Admin UI**: A simple dashboard to monitor active subscriptions and provider health.

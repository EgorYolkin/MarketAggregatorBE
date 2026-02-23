.PHONY: run build test lint up down clean help

# Environment variables for local run
export REDIS_HOST ?= localhost
export REDIS_PORT ?= 6379

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

run: ## Run the application locally
	go run cmd/app/main.go

build: ## Compile the application
	go build -ldflags="-w -s" -o bin/market-aggregator cmd/app/main.go

test: ## Run tests with race detector
	go test -v -race ./...

lint: ## Run golangci-lint
	golangci-lint run ./...

up: ## Start docker-compose infrastructure (app and redis)
	docker-compose up -d --build

down: ## Stop docker-compose infrastructure
	docker-compose down

clean: ## Clean build binaries
	rm -rf bin/

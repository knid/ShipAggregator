# Makefile for TR-Only Shipping Aggregation Backend
# ================================================

# Variables
BINARY_API=shipping-api
BINARY_WORKER=shipping-worker
BINARY_MIGRATE=shipping-migrate
GO=go
GOFLAGS=-ldflags="-s -w"
DOCKER_COMPOSE=docker compose

# Database
DB_HOST?=localhost
DB_PORT?=5432
DB_USER?=shipping
DB_PASSWORD?=shipping
DB_NAME?=shipping
DATABASE_URL?=postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable

# Colors for output
GREEN=\033[0;32m
YELLOW=\033[0;33m
RED=\033[0;31m
NC=\033[0m # No Color

.PHONY: all dev run-api run-worker migrate-up migrate-down seed generate sqlc openapi test test-unit test-integration fmt lint build clean help

# Default target
all: build

# ==================== Development ====================

## dev: Start development environment with hot reload
dev:
	@echo "$(GREEN)Starting development environment...$(NC)"
	$(DOCKER_COMPOSE) up -d postgres redis minio
	@echo "$(YELLOW)Waiting for services to be ready...$(NC)"
	@sleep 3
	@$(MAKE) migrate-up
	air -c .air.toml

## run-api: Run the API server
run-api:
	@echo "$(GREEN)Starting API server...$(NC)"
	$(GO) run ./cmd/api/main.go

## run-worker: Run the background worker
run-worker:
	@echo "$(GREEN)Starting background worker...$(NC)"
	$(GO) run ./cmd/worker/main.go

# ==================== Database ====================

## migrate-up: Run database migrations up
migrate-up:
	@echo "$(GREEN)Running migrations up...$(NC)"
	$(GO) run ./cmd/migrate/main.go -direction up

## migrate-down: Rollback database migrations
migrate-down:
	@echo "$(YELLOW)Rolling back migrations...$(NC)"
	$(GO) run ./cmd/migrate/main.go -direction down

## migrate-create: Create a new migration (usage: make migrate-create name=migration_name)
migrate-create:
	@if [ -z "$(name)" ]; then \
		echo "$(RED)Error: Please provide a migration name. Usage: make migrate-create name=migration_name$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)Creating migration: $(name)$(NC)"
	@NEXT_NUM=$$(ls -1 migrations/*.up.sql 2>/dev/null | wc -l | xargs printf "%06d"); \
	touch migrations/$${NEXT_NUM}_$(name).up.sql; \
	touch migrations/$${NEXT_NUM}_$(name).down.sql; \
	echo "Created migrations/$${NEXT_NUM}_$(name).up.sql"; \
	echo "Created migrations/$${NEXT_NUM}_$(name).down.sql"

## seed: Seed the database with initial data
seed:
	@echo "$(GREEN)Seeding database...$(NC)"
	$(GO) run ./cmd/migrate/main.go -seed

## seed-reference: Seed reference data (cities, districts, neighborhoods) from JSON files
## Usage: make seed-reference [TRUNCATE=1]
## Place your JSON files in the data/ directory:
##   - data/provinces.json (or provinces.min.json)
##   - data/districts.json (or districts.min.json)
##   - data/neighborhoods.json
seed-reference:
	@echo "$(GREEN)Seeding reference data...$(NC)"
	@PROVINCES_FILE=$$(ls data/provinces*.json 2>/dev/null | head -1); \
	DISTRICTS_FILE=$$(ls data/districts*.json 2>/dev/null | head -1); \
	NEIGHBORHOODS_FILE=$$(ls data/neighborhoods*.json 2>/dev/null | head -1); \
	if [ -z "$$PROVINCES_FILE" ] || [ -z "$$DISTRICTS_FILE" ] || [ -z "$$NEIGHBORHOODS_FILE" ]; then \
		echo "$(RED)Error: Missing JSON files in data/ directory$(NC)"; \
		echo "Required files: provinces.json, districts.json, neighborhoods.json"; \
		exit 1; \
	fi; \
	TRUNCATE_FLAG=""; \
	if [ "$(TRUNCATE)" = "1" ]; then TRUNCATE_FLAG="-truncate"; fi; \
	$(GO) run ./cmd/seed/main.go \
		-db "$(DATABASE_URL)" \
		-provinces "$$PROVINCES_FILE" \
		-districts "$$DISTRICTS_FILE" \
		-neighborhoods "$$NEIGHBORHOODS_FILE" \
		$$TRUNCATE_FLAG

# ==================== Code Generation ====================

## generate: Run all code generation
generate: sqlc openapi
	@echo "$(GREEN)Running go generate...$(NC)"
	$(GO) generate ./...

## sqlc: Generate type-safe SQL code
sqlc:
	@echo "$(GREEN)Generating sqlc code...$(NC)"
	sqlc generate

## openapi: Generate OpenAPI server code
openapi:
	@echo "$(GREEN)Generating OpenAPI code...$(NC)"
	oapi-codegen -config oapi-codegen.yaml specs/001-core-platform/contracts/openapi.yaml

# ==================== Testing ====================

## test: Run all tests
test: test-unit test-integration
	@echo "$(GREEN)All tests completed$(NC)"

## test-unit: Run unit tests
test-unit:
	@echo "$(GREEN)Running unit tests...$(NC)"
	$(GO) test -v -race -short ./...

## test-integration: Run integration tests
test-integration:
	@echo "$(GREEN)Running integration tests...$(NC)"
	$(GO) test -v -race -run Integration ./tests/...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	$(GO) test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report generated: coverage.html$(NC)"

## test-contract: Run OpenAPI contract tests
test-contract:
	@echo "$(GREEN)Running contract tests...$(NC)"
	$(GO) test -v ./tests/contract/...

# ==================== Code Quality ====================

## fmt: Format code
fmt:
	@echo "$(GREEN)Formatting code...$(NC)"
	$(GO) fmt ./...
	gofumpt -l -w .

## lint: Run linters
lint:
	@echo "$(GREEN)Running linters...$(NC)"
	golangci-lint run ./...

## lint-fix: Run linters and fix issues
lint-fix:
	@echo "$(GREEN)Running linters with auto-fix...$(NC)"
	golangci-lint run --fix ./...

## vet: Run go vet
vet:
	@echo "$(GREEN)Running go vet...$(NC)"
	$(GO) vet ./...

# ==================== Build ====================

## build: Build all binaries
build: build-api build-worker build-migrate
	@echo "$(GREEN)Build completed$(NC)"

## build-api: Build API server binary
build-api:
	@echo "$(GREEN)Building API server...$(NC)"
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -o bin/$(BINARY_API) ./cmd/api

## build-worker: Build worker binary
build-worker:
	@echo "$(GREEN)Building worker...$(NC)"
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -o bin/$(BINARY_WORKER) ./cmd/worker

## build-migrate: Build migration tool binary
build-migrate:
	@echo "$(GREEN)Building migration tool...$(NC)"
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -o bin/$(BINARY_MIGRATE) ./cmd/migrate

# ==================== Docker ====================

## docker-build: Build Docker image
docker-build:
	@echo "$(GREEN)Building Docker image...$(NC)"
	docker build -t shipping-aggregator:latest .

## docker-up: Start all Docker services
docker-up:
	@echo "$(GREEN)Starting Docker services...$(NC)"
	$(DOCKER_COMPOSE) up -d

## docker-down: Stop all Docker services
docker-down:
	@echo "$(YELLOW)Stopping Docker services...$(NC)"
	$(DOCKER_COMPOSE) down

## docker-logs: View Docker logs
docker-logs:
	$(DOCKER_COMPOSE) logs -f

## docker-clean: Remove all Docker resources
docker-clean:
	@echo "$(RED)Removing all Docker resources...$(NC)"
	$(DOCKER_COMPOSE) down -v --rmi local

# ==================== Dependencies ====================

## deps: Download dependencies
deps:
	@echo "$(GREEN)Downloading dependencies...$(NC)"
	$(GO) mod download

## deps-tidy: Tidy dependencies
deps-tidy:
	@echo "$(GREEN)Tidying dependencies...$(NC)"
	$(GO) mod tidy

## deps-update: Update dependencies
deps-update:
	@echo "$(GREEN)Updating dependencies...$(NC)"
	$(GO) get -u ./...
	$(GO) mod tidy

## tools: Install development tools
tools:
	@echo "$(GREEN)Installing development tools...$(NC)"
	$(GO) install github.com/air-verse/air@latest
	$(GO) install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	$(GO) install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GO) install mvdan.cc/gofumpt@latest

# ==================== Cleanup ====================

## clean: Clean build artifacts
clean:
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
	rm -rf bin/
	rm -f coverage.out coverage.html

## clean-all: Clean everything including Docker volumes
clean-all: clean docker-clean
	@echo "$(RED)All artifacts cleaned$(NC)"

# ==================== Help ====================

## help: Show this help message
help:
	@echo "$(GREEN)Available targets:$(NC)"
	@echo ""
	@grep -E '^##' $(MAKEFILE_LIST) | sed -e 's/## //' | column -t -s ':'

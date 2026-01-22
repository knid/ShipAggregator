# Developer Quickstart Guide

**Feature Branch**: `001-core-platform`  
**Created**: 2026-01-22  
**Status**: Draft

---

## Overview

This guide helps developers get started with the TR-Only Shipping Aggregation Backend. It covers local development setup, running tests, and common development tasks.

---

## Prerequisites

### Required Software

| Software | Version | Purpose |
|----------|---------|---------|
| Go | 1.22+ | Primary language |
| Docker | 24+ | Local services |
| Docker Compose | 2.20+ | Service orchestration |
| Make | 4+ | Build automation |
| sqlc | 1.25+ | SQL code generation |
| golang-migrate | 4.17+ | Database migrations |

### Installation (macOS)

```bash
# Go
brew install go

# Docker Desktop
brew install --cask docker

# Development tools
brew install sqlc golang-migrate make

# Optional: Air for hot reload
go install github.com/cosmtrek/air@latest
```

### Installation (Linux)

```bash
# Go (download from golang.org)
wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Docker
curl -fsSL https://get.docker.com | sh

# sqlc
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

# golang-migrate
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

---

## Project Setup

### 1. Clone Repository

```bash
git clone https://github.com/your-org/shipaggregator.git
cd shipaggregator
git checkout 001-core-platform
```

### 2. Start Local Services

```bash
# Start PostgreSQL, Redis, MinIO
docker-compose up -d

# Verify services are running
docker-compose ps
```

**docker-compose.yml** (reference):

```yaml
version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: shipaggregator
      POSTGRES_PASSWORD: localdev
      POSTGRES_DB: shipaggregator
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

  minio:
    image: minio/minio:latest
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    ports:
      - "9000:9000"
      - "9001:9001"
    volumes:
      - minio_data:/data

volumes:
  postgres_data:
  minio_data:
```

### 3. Configure Environment

```bash
# Copy example environment file
cp .env.example .env

# Edit with your local settings
vim .env
```

**.env** (reference):

```bash
# Server
PORT=8080
ENV=development

# Database
DATABASE_URL=postgres://shipaggregator:localdev@localhost:5432/shipaggregator?sslmode=disable

# Redis
REDIS_URL=redis://localhost:6379/0

# Storage
S3_ENDPOINT=http://localhost:9000
S3_ACCESS_KEY=minioadmin
S3_SECRET_KEY=minioadmin
S3_BUCKET=shipaggregator
S3_REGION=us-east-1

# Encryption (generate with: openssl rand -hex 32)
ENCRYPTION_KEY=your-32-byte-hex-key-here

# Iyzico (sandbox)
IYZICO_API_KEY=sandbox-xxx
IYZICO_SECRET_KEY=sandbox-yyy
IYZICO_BASE_URL=https://sandbox-api.iyzipay.com

# Provider Sandboxes (optional for local dev)
ARAS_WSDL_URL=https://customerws.araskargo.com.tr/aaborsa/ArasBorsaWS.asmx?WSDL
ARAS_USERNAME=sandbox_user
ARAS_PASSWORD=sandbox_pass
```

### 4. Run Database Migrations

```bash
# Run all migrations
make migrate-up

# Or manually
migrate -path migrations -database "$DATABASE_URL" up
```

### 5. Seed Reference Data

```bash
# Seed providers, services, and Turkish reference data
make seed

# Or manually
go run cmd/seed/main.go
```

### 6. Generate Code

```bash
# Generate sqlc code
make sqlc

# Generate OpenAPI types
make openapi

# Generate all
make generate
```

---

## Running the Application

### API Server

```bash
# Run directly
go run cmd/api/main.go

# Or with hot reload
air -c .air.toml

# Or with make
make run-api
```

The API server starts at `http://localhost:8080`.

### Worker (Background Jobs)

```bash
# Run worker
go run cmd/worker/main.go

# Or with make
make run-worker
```

### Both (Development)

```bash
# Run API and worker together
make dev
```

---

## API Exploration

### Health Check

```bash
# Liveness
curl http://localhost:8080/health/live

# Readiness
curl http://localhost:8080/health/ready
```

### Create Test Organization

```bash
# Create organization
curl -X POST http://localhost:8080/v1/organizations \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test Company",
    "billing_email": "billing@test.com"
  }'
```

### Create API Key

```bash
# Create API key (requires admin session)
curl -X POST http://localhost:8080/v1/api-keys \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <session_token>" \
  -d '{
    "name": "Development Key",
    "scopes": ["shipment:read", "shipment:create", "quote:create"]
  }'
```

### Create Quote

```bash
curl -X POST http://localhost:8080/v1/quotes \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer sk_test_xxx" \
  -H "Idempotency-Key: quote-$(uuidgen)" \
  -d '{
    "origin": {
      "contact_name": "Sender Name",
      "phone": "+905321234567",
      "street_line_1": "Atatürk Cad. No:1",
      "neighborhood_code": "340101001",
      "district_code": "3401",
      "city_code": "34"
    },
    "destination": {
      "contact_name": "Recipient Name",
      "phone": "+905329876543",
      "street_line_1": "Kızılay Mah. No:5",
      "neighborhood_code": "060101001",
      "district_code": "0601",
      "city_code": "06"
    },
    "parcel": {
      "weight_grams": 2500,
      "length_cm": 30,
      "width_cm": 20,
      "height_cm": 15
    }
  }'
```

### OpenAPI Documentation

Access the OpenAPI spec at:

- JSON: `http://localhost:8080/v1/openapi.json`
- Swagger UI: `http://localhost:8080/docs` (if enabled)

---

## Testing

### Run All Tests

```bash
make test
```

### Unit Tests Only

```bash
make test-unit

# Or directly
go test -short ./...
```

### Integration Tests

```bash
# Requires Docker services running
make test-integration

# Or directly
go test -tags=integration ./tests/integration/...
```

### Contract Tests

```bash
# Validate API against OpenAPI spec
make test-contract
```

### E2E Tests

```bash
# Full flow tests
make test-e2e
```

### Coverage Report

```bash
make coverage

# View in browser
open coverage.html
```

---

## Common Development Tasks

### Adding a New Endpoint

1. **Define in OpenAPI spec** (`specs/001-core-platform/contracts/openapi.yaml`)

2. **Generate types**:

   ```bash
   make openapi
   ```

3. **Add handler** (`internal/api/handler/`):

   ```go
   func (h *ShipmentHandler) CreateShipment(w http.ResponseWriter, r *http.Request) {
       // Implementation
   }
   ```

4. **Register route** (`internal/api/router.go`):

   ```go
   r.Post("/shipments", h.shipment.CreateShipment)
   ```

5. **Add tests**

### Adding a New Database Table

1. **Create migration**:

   ```bash
   migrate create -ext sql -dir migrations -seq add_new_table
   ```

2. **Write SQL** (`migrations/000004_add_new_table.up.sql`):

   ```sql
   CREATE TABLE new_table (
       id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
       organization_id UUID NOT NULL REFERENCES organizations(id),
       -- columns
       created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
   );
   
   -- Enable RLS
   ALTER TABLE new_table ENABLE ROW LEVEL SECURITY;
   
   CREATE POLICY new_table_tenant_isolation ON new_table
       USING (organization_id = current_setting('app.current_organization_id')::uuid);
   ```

3. **Write down migration** (`migrations/000004_add_new_table.down.sql`):

   ```sql
   DROP TABLE new_table;
   ```

4. **Run migration**:

   ```bash
   make migrate-up
   ```

5. **Add sqlc queries** (`internal/repository/postgres/queries/new_table.sql`)

6. **Regenerate**:

   ```bash
   make sqlc
   ```

### Adding a New Provider Adapter

1. **Create adapter directory**:

   ```bash
   mkdir -p internal/provider/newprovider
   ```

2. **Implement interface** (`internal/provider/newprovider/adapter.go`):

   ```go
   type Adapter struct {
       client *soap.Client // or *rest.Client
       config Config
   }
   
   func (a *Adapter) ProviderCode() string { return "newprovider" }
   func (a *Adapter) APIType() string { return "soap" } // or "rest"
   
   // Implement all ProviderAdapter methods
   ```

3. **Add status mapping** (`internal/provider/newprovider/mapping.go`)

4. **Register in factory** (`internal/provider/factory.go`):

   ```go
   factory.Register("newprovider", func(cfg Config) ProviderAdapter {
       return newprovider.NewAdapter(cfg)
   })
   ```

5. **Add to capability matrix** (`specs/001-core-platform/contracts/provider-capabilities-matrix.md`)

6. **Add integration tests**

### Adding a New Background Job

1. **Define task** (`internal/worker/tasks/new_task.go`):

   ```go
   const TaskNewTask = "new:task"
   
   type NewTaskPayload struct {
       // fields
   }
   
   func NewNewTaskHandler(deps Dependencies) *NewTaskHandler {
       return &NewTaskHandler{deps: deps}
   }
   
   func (h *NewTaskHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
       // Implementation
   }
   ```

2. **Register handler** (`internal/worker/worker.go`):

   ```go
   mux.HandleFunc(tasks.TaskNewTask, newTaskHandler.ProcessTask)
   ```

3. **Enqueue from service**:

   ```go
   task := asynq.NewTask(tasks.TaskNewTask, payload)
   client.Enqueue(task, asynq.Queue("default"))
   ```

---

## Debugging

### Database Queries

```bash
# Connect to PostgreSQL
docker-compose exec postgres psql -U shipaggregator

# View recent queries (if pg_stat_statements enabled)
SELECT query, calls, mean_time FROM pg_stat_statements ORDER BY mean_time DESC LIMIT 10;
```

### Redis

```bash
# Connect to Redis
docker-compose exec redis redis-cli

# View keys
KEYS *

# View rate limit
ZRANGE ratelimit:org:xxx 0 -1 WITHSCORES
```

### Logs

```bash
# API logs
docker-compose logs -f api

# Worker logs
docker-compose logs -f worker

# All logs
docker-compose logs -f
```

### Tracing

If using Jaeger locally:

```bash
# Start Jaeger
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 4317:4317 \
  jaegertracing/all-in-one:latest

# View traces
open http://localhost:16686
```

---

## Code Style

### Formatting

```bash
# Format code
make fmt

# Or directly
gofmt -w .
goimports -w .
```

### Linting

```bash
# Run linter
make lint

# Or directly
golangci-lint run
```

### Pre-commit Hooks

```bash
# Install pre-commit
pip install pre-commit
pre-commit install

# Run manually
pre-commit run --all-files
```

---

## Makefile Reference

```makefile
# Development
make dev          # Run API + worker with hot reload
make run-api      # Run API server
make run-worker   # Run worker

# Database
make migrate-up   # Run migrations
make migrate-down # Rollback last migration
make seed         # Seed reference data

# Code Generation
make generate     # Generate all code
make sqlc         # Generate sqlc code
make openapi      # Generate OpenAPI types

# Testing
make test         # Run all tests
make test-unit    # Run unit tests
make test-integration  # Run integration tests
make test-contract     # Run contract tests
make test-e2e     # Run E2E tests
make coverage     # Generate coverage report

# Quality
make fmt          # Format code
make lint         # Run linter
make vet          # Run go vet

# Build
make build        # Build binaries
make docker       # Build Docker image

# Cleanup
make clean        # Clean build artifacts
make docker-clean # Remove Docker volumes
```

---

## Troubleshooting

### Database Connection Failed

```
Error: dial tcp 127.0.0.1:5432: connect: connection refused
```

**Solution**: Ensure PostgreSQL is running:

```bash
docker-compose up -d postgres
docker-compose logs postgres
```

### Redis Connection Failed

```
Error: dial tcp 127.0.0.1:6379: connect: connection refused
```

**Solution**: Ensure Redis is running:

```bash
docker-compose up -d redis
```

### Migration Failed

```
Error: Dirty database version X. Fix and force version.
```

**Solution**: Fix the migration and force version:

```bash
migrate -path migrations -database "$DATABASE_URL" force X
migrate -path migrations -database "$DATABASE_URL" up
```

### sqlc Generation Failed

```
Error: relation "xxx" does not exist
```

**Solution**: Run migrations first:

```bash
make migrate-up
make sqlc
```

### Provider Sandbox Not Working

**Solution**: Check credentials in `.env` and verify sandbox access with provider.

---

## Getting Help

- **Documentation**: `specs/001-core-platform/`
- **API Spec**: `specs/001-core-platform/contracts/openapi.yaml`
- **Constitution**: `.specify/memory/constitution.md`
- **Slack**: #shipaggregator-dev
- **Issues**: GitHub Issues

---

## Next Steps

1. Read the [Feature Specification](./spec.md)
2. Review the [Data Model](./data-model.md)
3. Explore the [API Contracts](./contracts/)
4. Check the [Implementation Plan](./plan.md)
5. Start with assigned tasks from [tasks.md](./tasks.md)

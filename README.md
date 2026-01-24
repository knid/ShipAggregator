# Shipping Aggregator

A multi-tenant shipping aggregation backend for Turkey-only logistics providers.

## Tech Stack

- **Go 1.25+** with [chi](https://github.com/go-chi/chi) router
- **PostgreSQL** with [pgx](https://github.com/jackc/pgx) driver and [sqlc](https://sqlc.dev/) for type-safe SQL
- **Redis** for caching and rate limiting
- **Asynq** for background job processing
- **OpenTelemetry** for observability
- **Docker Compose** for local development

## Supported Providers

- Aras Kargo
- Yurtiçi Kargo
- Sürat Kargo
- PTT Kargo
- Hepsijet
- MNG Kargo
- Kolaygelsin
- Sendeo
- Pakettaxi

## Getting Started

### Prerequisites

- Go 1.25+
- Docker & Docker Compose
- Make

### Setup

1. Clone the repository
2. Copy environment file:

   ```bash
   cp .env.example .env
   ```

3. Install development tools:

   ```bash
   make tools
   ```

4. Start development environment:

   ```bash
   make dev
   ```

## Available Commands

```bash
make dev              # Start development environment with hot reload
make run-api          # Run the API server
make run-worker       # Run the background worker
make migrate-up       # Run database migrations
make migrate-down     # Rollback migrations
make seed             # Seed database with initial data
make sqlc             # Generate type-safe SQL code
make test             # Run all tests
make lint             # Run linters
make build            # Build all binaries
make docker-up        # Start Docker services
make docker-down      # Stop Docker services
make help             # Show all available commands
```

## Project Structure

```
cmd/
├── api/          # API server entrypoint
├── migrate/      # Database migration tool
├── seed/         # Database seeding tool
└── worker/       # Background worker entrypoint

internal/
├── api/          # HTTP handlers, middleware, router
├── config/       # Configuration management
├── crypto/       # Encryption utilities
├── domain/       # Domain models
├── observability/# Logging, metrics, tracing
├── payment/      # Payment gateway integrations
├── provider/     # Shipping provider adapters
└── repository/   # Database access layer
```

## License

Proprietary

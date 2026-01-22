# Implementation Plan: TR-Only Shipping Aggregation Backend

**Branch**: `001-core-platform` | **Date**: 2026-01-22 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-core-platform/spec.md`

---

## Summary

This plan covers the complete backend implementation for a Turkish shipping aggregation platform supporting Marketplace and BYOA (Bring Your Own Agreement) business models. The system aggregates 9 cargo carriers (Aras, Yurtiçi, Sürat, PTT, Hepsijet, MNG, Kolaygelsin, Sendeo, Pakettaxi) under a unified REST API with comprehensive billing/wallet/ledger capabilities, webhook delivery, and provider abstraction.

**Technical Approach**: Go + chi router with PostgreSQL for persistence, Redis for caching/rate-limiting, Asynq for background jobs, and S3-compatible storage for documents. OpenAPI-first design with contract tests ensuring provider adapter correctness.

---

## Technical Context

**Language/Version**: Go 1.22+  
**Primary Dependencies**: chi (router), sqlc (type-safe SQL), pgx (PostgreSQL driver), asynq (job queue), go-redis, etree/gowsdl (SOAP), oapi-codegen (OpenAPI)  
**Storage**: PostgreSQL 15+ (primary), Redis 7+ (cache/rate-limit/jobs), S3-compatible (MinIO/AWS S3 for documents)  
**Testing**: go test, testify, dockertest (integration), schemathesis (OpenAPI contract tests)  
**Target Platform**: Linux containers (Docker/Kubernetes), Turkey region deployment  
**Project Type**: Single backend service (monolith with clean module boundaries)  
**Performance Goals**: 10,000 concurrent API requests, p99 < 500ms for non-carrier operations, carrier operations within carrier SLA (5-30s)  
**Constraints**: All data in Turkey, KVKK compliant, no floating-point for money, immutable ledger  
**Scale/Scope**: Initial: 1000 organizations, 100k shipments/month; Target: 10k organizations, 1M shipments/month

---

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Article | Requirement | Status | Implementation Notes |
|---------|-------------|--------|---------------------|
| I. Multi-Tenant Isolation | All queries include tenant context, RLS enforced | ✅ PASS | PostgreSQL RLS policies on all tenant tables, `organization_id` required in all repository methods |
| II. Least Privilege | Service accounts with minimum permissions | ✅ PASS | Scoped API keys, role-based permissions, admin elevation with audit |
| III. API Key Scopes & Rotation | Explicit scopes, 90-day max age, dual-key rotation | ✅ PASS | Defined in security.md, 24-hour grace period for rotation |
| IV. Idempotency | Idempotency-Key required for critical operations | ✅ PASS | Defined in idempotency.md, 24-hour record retention |
| V. Money Correctness | Integer kuruş, immutable ledger | ✅ PASS | All amounts in kuruş (int64), ledger_entries table with UPDATE/DELETE trigger prevention |
| VI. Financial Invariants | No negative balance, debits=credits, refunds ≤ original | ✅ PASS | DB constraints, nightly invariant checker job |
| VII. Reconciliation Pipeline | Daily reconciliation with Iyzico and carriers | ✅ PASS | Defined in iyzico.md, COD settlement reconciliation in spec |
| VIII. Outbox Pattern | Events written in same transaction as state change | ✅ PASS | outbox table defined in data-model.md |
| IX. Webhook Delivery | HMAC-SHA256 signing, retry with backoff, DLQ | ✅ PASS | Defined in events.md and security.md |
| X. Provider Abstraction | All carrier calls through ProviderAdapter interface | ✅ PASS | Defined in provider-adapter.md |
| XI. Provider Capability Matrix | Declared capabilities, routing respects constraints | ✅ PASS | Defined in provider-capabilities-matrix.md |
| XII. Observability | trace_id propagation, structured logging, metrics | ✅ PASS | All responses include trace_id, PII masking rules defined |
| XIII. PII Masking | No PII in logs, masking functions | ✅ PASS | Masking rules in security.md, 30-day log retention |
| XIV. KVKK Compliance | Data subject access/deletion, documented legal basis | ✅ PASS | Soft delete with 30-day retention, data inventory required |
| XV. Retention & Access Logging | Defined retention periods, access logging | ✅ PASS | Retention table in data-model.md, audit_logs table |
| XVI. Full Specification Coverage | No MVP shortcuts, full adapter interface | ✅ PASS | All 9 providers must implement full interface or declare unsupported |

**Gate Status**: ✅ ALL GATES PASS - Proceed to Phase 0

---

## Project Structure

### Documentation (this feature)

```text
specs/001-core-platform/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Database schema (exists, may need updates)
├── quickstart.md        # Phase 1 output - developer onboarding
├── contracts/           # API contracts (exists)
│   ├── openapi.yaml     # REST API specification
│   ├── events.md        # Webhook event schemas
│   ├── errors.md        # Error codes and formats
│   ├── idempotency.md   # Idempotency specification
│   ├── security.md      # Security specification
│   ├── provider-adapter.md        # Provider interface
│   ├── provider-capabilities-matrix.md  # Carrier capabilities
│   └── iyzico.md        # Payment integration
├── checklists/
│   └── requirements.md  # Requirements checklist
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
cmd/
├── api/
│   └── main.go                    # HTTP server entrypoint
├── worker/
│   └── main.go                    # Asynq worker entrypoint
└── migrate/
    └── main.go                    # Database migration tool

internal/
├── config/
│   └── config.go                  # Configuration loading
├── domain/                        # Domain models (no external deps)
│   ├── organization.go
│   ├── member.go
│   ├── apikey.go
│   ├── address.go
│   ├── provider.go
│   ├── quote.go
│   ├── shipment.go
│   ├── tracking.go
│   ├── webhook.go
│   ├── ledger.go
│   ├── topup.go
│   └── errors.go
├── repository/                    # Data access layer
│   ├── postgres/
│   │   ├── organization.go
│   │   ├── member.go
│   │   ├── apikey.go
│   │   ├── address.go
│   │   ├── provider.go
│   │   ├── quote.go
│   │   ├── shipment.go
│   │   ├── tracking.go
│   │   ├── webhook.go
│   │   ├── ledger.go
│   │   ├── topup.go
│   │   ├── idempotency.go
│   │   ├── outbox.go
│   │   └── audit.go
│   └── redis/
│       ├── cache.go
│       └── ratelimit.go
├── service/                       # Business logic
│   ├── auth/
│   │   ├── apikey.go
│   │   └── session.go
│   ├── organization/
│   │   └── service.go
│   ├── quote/
│   │   └── service.go
│   ├── shipment/
│   │   └── service.go
│   ├── billing/
│   │   ├── wallet.go
│   │   ├── topup.go
│   │   └── ledger.go
│   ├── webhook/
│   │   ├── publisher.go
│   │   └── delivery.go
│   └── tracking/
│       └── service.go
├── provider/                      # Provider adapters
│   ├── adapter.go                 # Interface definition
│   ├── factory.go                 # Adapter factory
│   ├── soap/
│   │   ├── client.go              # SOAP client utilities
│   │   └── xml.go                 # XML parsing utilities
│   ├── rest/
│   │   └── client.go              # REST client utilities
│   ├── aras/
│   │   ├── adapter.go
│   │   ├── types.go
│   │   └── mapping.go
│   ├── yurtici/
│   │   ├── adapter.go
│   │   ├── types.go
│   │   └── mapping.go
│   ├── surat/
│   │   └── ...
│   ├── ptt/
│   │   └── ...
│   ├── hepsijet/
│   │   └── ...
│   ├── mng/
│   │   └── ...
│   ├── kolaygelsin/
│   │   └── ...
│   ├── sendeo/
│   │   └── ...
│   └── pakettaxi/
│       └── ...
├── payment/                       # Payment integrations
│   └── iyzico/
│       ├── client.go
│       └── types.go
├── api/                           # HTTP handlers
│   ├── middleware/
│   │   ├── auth.go
│   │   ├── ratelimit.go
│   │   ├── tenant.go
│   │   ├── trace.go
│   │   └── idempotency.go
│   ├── handler/
│   │   ├── auth.go
│   │   ├── organization.go
│   │   ├── address.go
│   │   ├── provider.go
│   │   ├── quote.go
│   │   ├── shipment.go
│   │   ├── webhook.go
│   │   ├── billing.go
│   │   ├── reference.go
│   │   └── admin.go
│   ├── integration/               # Inbound webhooks from providers
│   │   ├── aras.go
│   │   ├── yurtici.go
│   │   ├── hepsijet.go
│   │   ├── mng.go
│   │   ├── kolaygelsin.go
│   │   ├── sendeo.go
│   │   ├── pakettaxi.go
│   │   └── iyzico.go
│   └── router.go
├── worker/                        # Background jobs
│   ├── tasks/
│   │   ├── webhook_delivery.go
│   │   ├── tracking_poll.go
│   │   ├── outbox_publisher.go
│   │   ├── idempotency_cleanup.go
│   │   ├── reconciliation.go
│   │   └── retention_cleanup.go
│   └── scheduler.go
├── crypto/                        # Encryption utilities
│   ├── aes.go                     # AES-256-GCM for credentials
│   └── hmac.go                    # HMAC for webhooks
└── observability/
    ├── logger.go
    ├── metrics.go
    └── tracer.go

pkg/
├── money/
│   └── money.go                   # Money type (int64 kuruş)
├── phone/
│   └── phone.go                   # E.164 phone validation
└── address/
    └── turkish.go                 # Turkish address validation

migrations/
├── 000001_initial_schema.up.sql
├── 000001_initial_schema.down.sql
├── 000002_seed_providers.up.sql
├── 000002_seed_providers.down.sql
├── 000003_seed_reference_data.up.sql
└── 000003_seed_reference_data.down.sql

tests/
├── contract/                      # OpenAPI contract tests
│   └── api_test.go
├── integration/                   # Integration tests
│   ├── shipment_test.go
│   ├── billing_test.go
│   ├── webhook_test.go
│   └── provider/
│       ├── aras_test.go
│       ├── yurtici_test.go
│       └── ...
└── e2e/                          # End-to-end flows
    ├── marketplace_flow_test.go
    ├── byoa_flow_test.go
    └── cod_flow_test.go
```

**Structure Decision**: Single Go module with clean internal package boundaries. The `internal/` directory enforces encapsulation. Provider adapters are isolated in `internal/provider/` with a common interface. Background jobs use Asynq for reliable execution with Redis as the broker.

---

## Module Boundaries and Package Layout

### Layer Responsibilities

| Layer | Package | Responsibility | Dependencies |
|-------|---------|----------------|--------------|
| **Domain** | `internal/domain` | Pure domain models, no I/O | None |
| **Repository** | `internal/repository` | Data access, SQL queries | domain, pgx |
| **Service** | `internal/service` | Business logic, orchestration | domain, repository, provider |
| **Provider** | `internal/provider` | Carrier API adapters | domain, http/soap clients |
| **API** | `internal/api` | HTTP handlers, middleware | service, domain |
| **Worker** | `internal/worker` | Background job handlers | service, repository |

### Dependency Rules

1. **Domain** has no dependencies on other internal packages
2. **Repository** depends only on domain
3. **Service** depends on domain and repository (via interfaces)
4. **Provider** depends on domain only
5. **API** depends on service (via interfaces)
6. **Worker** depends on service and repository

### Interface Boundaries

```go
// internal/service/shipment/service.go
type ShipmentService interface {
    CreateFromOffer(ctx context.Context, req CreateFromOfferRequest) (*domain.Shipment, error)
    PurchaseLabel(ctx context.Context, shipmentID uuid.UUID, req PurchaseLabelRequest) (*domain.Shipment, error)
    Cancel(ctx context.Context, shipmentID uuid.UUID) (*domain.Shipment, error)
    Get(ctx context.Context, shipmentID uuid.UUID) (*domain.Shipment, error)
    List(ctx context.Context, filter ListFilter) (*domain.ShipmentList, error)
}

// internal/provider/adapter.go
type ProviderAdapter interface {
    ProviderCode() string
    APIType() string // "soap" or "rest"
    VerifyCredentials(ctx context.Context, creds Credentials) (*VerificationResult, error)
    GetQuote(ctx context.Context, req QuoteRequest) (*QuoteResponse, error)
    CreateShipment(ctx context.Context, req CreateShipmentRequest) (*CreateShipmentResponse, error)
    CancelShipment(ctx context.Context, req CancelShipmentRequest) (*CancelShipmentResponse, error)
    GetLabel(ctx context.Context, req GetLabelRequest) (*GetLabelResponse, error)
    GetTracking(ctx context.Context, req GetTrackingRequest) (*GetTrackingResponse, error)
    ParseTrackingWebhook(ctx context.Context, payload []byte) ([]TrackingEvent, error)
    SchedulePickup(ctx context.Context, req SchedulePickupRequest) (*SchedulePickupResponse, error)
    CreateReturn(ctx context.Context, req CreateReturnRequest) (*CreateReturnResponse, error)
}
```

---

## ProviderAdapter Architecture

### Interface Design

See [`contracts/provider-adapter.md`](./contracts/provider-adapter.md) for the complete interface specification.

### Capability Matrix

See [`contracts/provider-capabilities-matrix.md`](./contracts/provider-capabilities-matrix.md) for the complete capability matrix.

### SOAP Client Strategy

For SOAP providers (Aras, Yurtiçi, Sürat, PTT, MNG):

1. **WSDL Handling**: Use `gowsdl` to generate Go types from WSDL at build time
2. **Manual Envelopes**: For complex cases, construct SOAP envelopes manually using `encoding/xml`
3. **XML Parsing**: Use `etree` for flexible XML manipulation when WSDL-generated types are insufficient
4. **Fault Mapping**: Map SOAP faults to domain errors with provider-specific error codes

```go
// internal/provider/soap/client.go
type SOAPClient struct {
    httpClient *http.Client
    endpoint   string
    timeout    time.Duration
    logger     *slog.Logger
}

func (c *SOAPClient) Call(ctx context.Context, action string, request, response interface{}) error {
    // 1. Marshal request to SOAP envelope
    // 2. Send HTTP POST with SOAPAction header
    // 3. Parse response or SOAP fault
    // 4. Map faults to ProviderError
}
```

### REST Client Strategy

For REST providers (Hepsijet, Kolaygelsin, Sendeo, Pakettaxi):

1. **HTTP Client**: Use standard `net/http` with configurable timeouts
2. **Retries**: Implement exponential backoff with jitter for transient errors
3. **Circuit Breaker**: Use `sony/gobreaker` for circuit breaker pattern

```go
// internal/provider/rest/client.go
type RESTClient struct {
    httpClient     *http.Client
    baseURL        string
    timeout        time.Duration
    circuitBreaker *gobreaker.CircuitBreaker
    logger         *slog.Logger
}

func (c *RESTClient) Do(ctx context.Context, method, path string, body, response interface{}) error {
    // 1. Build request with auth headers
    // 2. Execute through circuit breaker
    // 3. Retry on transient errors
    // 4. Parse response or error
}
```

### Credential Encryption and Verification

```go
// internal/crypto/aes.go
func EncryptCredentials(plaintext []byte, key []byte) (ciphertext, iv []byte, err error) {
    // AES-256-GCM encryption
}

func DecryptCredentials(ciphertext, iv, key []byte) ([]byte, error) {
    // AES-256-GCM decryption
}

// internal/service/provider/account.go
func (s *ProviderAccountService) Verify(ctx context.Context, accountID uuid.UUID) error {
    // 1. Load account with encrypted credentials
    // 2. Decrypt credentials using KMS-derived key
    // 3. Get adapter for provider
    // 4. Call adapter.VerifyCredentials()
    // 5. Update account status based on result
}
```

---

## Data Model Implementation Details

### Tables, Indexes, Unique Constraints

See [`data-model.md`](./data-model.md) for the complete schema. Key implementation notes:

1. **Primary Keys**: All tables use UUID v4 with `gen_random_uuid()`
2. **Tenant Isolation**: All tenant-scoped tables have `organization_id` with RLS policies
3. **Soft Deletes**: PII-containing tables use `deleted_at` timestamp
4. **Audit Trail**: `audit_logs` table captures all state changes

### Idempotency Store Design

```sql
CREATE TABLE idempotency_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    idempotency_key VARCHAR(100) NOT NULL,
    endpoint VARCHAR(100) NOT NULL,
    request_hash VARCHAR(64) NOT NULL,  -- SHA-256 of canonical JSON
    response_status INTEGER,             -- NULL while in-flight
    response_body JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    
    CONSTRAINT uq_idempotency_lookup 
        UNIQUE (organization_id, idempotency_key, endpoint)
);

CREATE INDEX idx_idempotency_expires ON idempotency_records(expires_at);
```

**Replay Behavior**:

1. **First Request**: Insert record with `response_status = NULL`, process, update with response
2. **Duplicate (same hash)**: Return cached response, set `X-Idempotency-Replayed: true`
3. **Duplicate (different hash)**: Return 422 `IDEMPOTENCY_REPLAYED` error
4. **In-Flight**: Use `SELECT FOR UPDATE` to block concurrent requests

### Ledger Invariants and Transaction Boundaries

```go
// internal/service/billing/ledger.go
func (s *LedgerService) Debit(ctx context.Context, req DebitRequest) (*domain.LedgerEntry, error) {
    return s.repo.WithTransaction(ctx, func(tx repository.Transaction) error {
        // 1. Lock organization row for update
        // 2. Calculate current balance from ledger
        // 3. Verify sufficient balance
        // 4. Insert ledger entry with balance_after
        // 5. Write to outbox for event publishing
    })
}
```

**Invariants Enforced**:

1. `balance_after_kurus` is computed and stored, not derived at read time
2. No UPDATE/DELETE on `ledger_entries` (trigger prevents)
3. Sum of credits = sum of debits per organization (nightly check)
4. Refund amount ≤ original charge amount (application check)

---

## Reliability Patterns

### Outbox for Webhook Events & Internal Events

```sql
CREATE TABLE outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type VARCHAR(50) NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    published_at TIMESTAMPTZ,
    retry_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_outbox_unpublished ON outbox(created_at) WHERE published_at IS NULL;
```

**Publisher Job** (runs every 5 seconds):

```go
// internal/worker/tasks/outbox_publisher.go
func (t *OutboxPublisherTask) Process(ctx context.Context) error {
    // 1. SELECT unpublished events ORDER BY created_at LIMIT 100 FOR UPDATE SKIP LOCKED
    // 2. For each event:
    //    a. Publish to internal event bus (Redis Streams or direct)
    //    b. Create webhook_events for subscribed endpoints
    //    c. Mark as published
}
```

### Exactly-Once-ish Billing + Label Purchase

```go
// internal/service/shipment/service.go
func (s *ShipmentService) PurchaseLabel(ctx context.Context, shipmentID uuid.UUID, req PurchaseLabelRequest) (*domain.Shipment, error) {
    // 1. Check idempotency record
    if cached := s.idempotency.Get(ctx, req.IdempotencyKey, "purchase-label"); cached != nil {
        return cached, nil
    }
    
    // 2. Start transaction
    return s.repo.WithTransaction(ctx, func(tx repository.Transaction) error {
        // 3. Lock shipment row
        shipment, err := tx.Shipments().GetForUpdate(ctx, shipmentID)
        
        // 4. Verify status is 'draft'
        if shipment.Status != domain.ShipmentStatusDraft {
            return domain.ErrInvalidShipmentStatus
        }
        
        // 5. Calculate and verify balance
        balance, err := tx.Ledger().GetBalance(ctx, shipment.OrganizationID)
        if balance < shipment.PriceChargedKurus {
            return domain.ErrInsufficientBalance
        }
        
        // 6. Call provider to create shipment and get label
        result, err := s.provider.CreateShipment(ctx, ...)
        
        // 7. Store label document
        doc, err := s.documents.Store(ctx, result.Label)
        
        // 8. Create ledger debit entry
        entry, err := tx.Ledger().Debit(ctx, ...)
        
        // 9. Update shipment status
        shipment.Status = domain.ShipmentStatusLabelPurchased
        shipment.ProviderShipmentID = result.ProviderShipmentID
        shipment.ProviderTrackingNumber = result.TrackingNumber
        shipment.LabelDocumentID = doc.ID
        
        // 10. Write to outbox
        tx.Outbox().Insert(ctx, "shipment", shipment.ID, "shipment.label_purchased", ...)
        
        // 11. Store idempotency record
        tx.Idempotency().Store(ctx, req.IdempotencyKey, "purchase-label", shipment)
        
        return nil
    })
}
```

---

## Webhook System

### HMAC Signing

```go
// internal/crypto/hmac.go
func SignWebhook(payload []byte, secret string, timestamp int64) string {
    message := fmt.Sprintf("%d.%s", timestamp, string(payload))
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write([]byte(message))
    return fmt.Sprintf("v1=%x", mac.Sum(nil))
}
```

### Timestamp Window and Replay Protection

- Timestamp must be within 5 minutes of current time
- Store delivered event IDs for 24 hours to detect replays

### Retry Schedule and DLQ

| Attempt | Delay | Cumulative |
|---------|-------|------------|
| 1 | Immediate | 0 |
| 2 | 1 minute | 1 min |
| 3 | 5 minutes | 6 min |
| 4 | 30 minutes | 36 min |
| 5 | 2 hours | 2h 36m |
| 6 | 8 hours | 10h 36m |
| 7 | 24 hours | 34h 36m |

After 7 failed attempts, move to DLQ.

### Replay Endpoints

```
POST /webhooks/deliveries/{id}/replay
```

Re-queues a DLQ entry for delivery.

---

## Observability

### trace_id + request_id

```go
// internal/api/middleware/trace.go
func TraceMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        traceID := r.Header.Get("X-Trace-ID")
        if traceID == "" {
            traceID = uuid.NewString()
        }
        requestID := uuid.NewString()
        
        ctx := context.WithValue(r.Context(), traceIDKey, traceID)
        ctx = context.WithValue(ctx, requestIDKey, requestID)
        
        w.Header().Set("X-Trace-ID", traceID)
        w.Header().Set("X-Request-ID", requestID)
        
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### PII Masking Rules

| Field | Masking |
|-------|---------|
| phone | `+9053****1234` |
| email | `ab***@example.com` |
| contact_name | `Meh***` |
| street_line_1 | `Ata***` |
| password | `[REDACTED]` |
| api_key | `sk_live_****xxxx` |
| credentials | `[ENCRYPTED]` |

### Provider Request/Response Log Policies

- Log all provider API calls to `provider_api_logs` table
- Mask PII in request/response previews
- Truncate bodies to 2000 characters
- Retain for 30 days
- Partition by `created_at` (daily)

---

## Test Strategy

### OpenAPI Contract Tests

```go
// tests/contract/api_test.go
func TestOpenAPIContract(t *testing.T) {
    // Use schemathesis or similar to validate all endpoints
    // against openapi.yaml specification
}
```

### Integration Tests per Provider

```go
// tests/integration/provider/aras_test.go
func TestArasAdapter_CreateShipment(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    adapter := aras.NewAdapter(aras.Config{
        WSDLURL:  os.Getenv("ARAS_WSDL_URL"),
        Username: os.Getenv("ARAS_USERNAME"),
        Password: os.Getenv("ARAS_PASSWORD"),
    })
    
    // Test with sandbox credentials
    result, err := adapter.CreateShipment(ctx, testShipmentRequest)
    require.NoError(t, err)
    assert.NotEmpty(t, result.TrackingNumber)
}
```

### End-to-End Flows

```go
// tests/e2e/marketplace_flow_test.go
func TestMarketplaceFlow(t *testing.T) {
    // 1. Create organization with funded wallet
    // 2. Create quote
    // 3. Select offer
    // 4. Create shipment
    // 5. Purchase label
    // 6. Verify wallet charged
    // 7. Verify ledger entry
    // 8. Verify webhook delivered
}

// tests/e2e/byoa_flow_test.go
func TestBYOAFlow(t *testing.T) {
    // 1. Create organization
    // 2. Create provider account with credentials
    // 3. Verify credentials
    // 4. Create quote with use_byoa=true
    // 5. Create shipment
    // 6. Purchase label (no wallet charge)
}

// tests/e2e/cod_flow_test.go
func TestCODFlow(t *testing.T) {
    // 1. Create COD shipment
    // 2. Simulate delivery with COD collection
    // 3. Verify COD settlement record
    // 4. Reconcile settlement
    // 5. Verify wallet credited
}
```

---

## Rollout

### Feature Flags per Provider & Capability

```go
// internal/config/features.go
type FeatureFlags struct {
    Providers map[string]ProviderFlags `json:"providers"`
}

type ProviderFlags struct {
    Enabled           bool `json:"enabled"`
    QuotesEnabled     bool `json:"quotes_enabled"`
    ShipmentsEnabled  bool `json:"shipments_enabled"`
    CODEnabled        bool `json:"cod_enabled"`
    ReturnsEnabled    bool `json:"returns_enabled"`
    PickupEnabled     bool `json:"pickup_enabled"`
    WebhooksEnabled   bool `json:"webhooks_enabled"`
    SandboxOnly       bool `json:"sandbox_only"`
}
```

### Migrations + Backfills

1. **Initial Schema**: Create all tables, indexes, constraints
2. **Seed Providers**: Insert provider and service records
3. **Seed Reference Data**: Load Turkish cities, districts, neighborhoods
4. **Enable RLS**: Apply row-level security policies
5. **Create Triggers**: Audit, immutability, updated_at triggers
6. **Create Views**: Computed views for balance, health

**Backfill Strategy**:

- Use `pg_dump` for reference data
- Idempotent migrations (check before insert)
- Zero-downtime migrations (add column, backfill, add constraint)

---

## Complexity Tracking

> No constitution violations requiring justification.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| N/A | N/A | N/A |

---

## Next Steps

1. **Phase 0**: Generate `research.md` with technology decisions and best practices
2. **Phase 1**: Update `data-model.md` if needed, generate `quickstart.md`
3. **Phase 2**: Break down into implementation tasks via `/speckit.tasks`

# Research Document: TR-Only Shipping Aggregation Backend

**Feature Branch**: `001-core-platform`  
**Created**: 2026-01-22  
**Status**: Complete

---

## Overview

This document consolidates research findings for technology choices, best practices, and implementation patterns for the shipping aggregation backend. All "NEEDS CLARIFICATION" items from the Technical Context have been resolved.

---

## Technology Decisions

### 1. Go Version and Module Structure

**Decision**: Go 1.22+

**Rationale**:

- Enhanced `net/http` routing with method-based patterns (Go 1.22 feature)
- Improved range-over-func for cleaner iteration patterns
- Better generics support for repository patterns
- Long-term support and stability

**Alternatives Considered**:

- Go 1.21: Lacks new routing features, would require more chi boilerplate
- Go 1.23 (future): Not yet released, prefer stable version

---

### 2. HTTP Router: chi

**Decision**: `go-chi/chi` v5

**Rationale**:

- Lightweight, idiomatic Go router
- Excellent middleware composition
- Context-based request handling
- No external dependencies
- Battle-tested in production at scale

**Alternatives Considered**:

- `gorilla/mux`: Archived, no longer maintained
- `gin`: More opinionated, heavier framework
- `fiber`: Uses fasthttp, not compatible with standard library
- Standard library (Go 1.22): Viable but chi provides better middleware patterns

---

### 3. Database Access: sqlc + pgx

**Decision**: `sqlc` for query generation, `pgx` v5 for driver

**Rationale**:

- **sqlc**: Compile-time SQL validation, type-safe Go code generation
- **pgx**: Native PostgreSQL driver with better performance than database/sql
- Explicit SQL queries (no ORM magic)
- Easy to audit and optimize queries
- Supports PostgreSQL-specific features (JSONB, arrays, RLS)

**Alternatives Considered**:

- GORM: Too much magic, hard to optimize, RLS complications
- sqlx: Good but manual struct mapping, no compile-time validation
- ent: Heavy code generation, overkill for this use case

**Implementation Pattern**:

```go
// queries/shipment.sql
-- name: GetShipment :one
SELECT * FROM shipments 
WHERE id = $1 AND organization_id = $2;

-- name: CreateShipment :one
INSERT INTO shipments (organization_id, ...) 
VALUES ($1, ...) 
RETURNING *;
```

---

### 4. Job Queue: Asynq

**Decision**: `hibiken/asynq`

**Rationale**:

- Redis-backed, simple and reliable
- Built-in retry with exponential backoff
- Scheduled tasks support
- Dead letter queue
- Prometheus metrics
- Active maintenance

**Alternatives Considered**:

- Temporal: Overkill for our use case, complex setup
- RabbitMQ: Additional infrastructure, more complex
- PostgreSQL-based (pgq, river): Adds load to primary database
- Custom implementation: Unnecessary complexity

**Task Types**:

```go
const (
    TaskWebhookDelivery    = "webhook:delivery"
    TaskTrackingPoll       = "tracking:poll"
    TaskOutboxPublish      = "outbox:publish"
    TaskIdempotencyCleanup = "idempotency:cleanup"
    TaskReconciliation     = "reconciliation:daily"
    TaskRetentionCleanup   = "retention:cleanup"
)
```

---

### 5. SOAP Client Strategy

**Decision**: Hybrid approach - gowsdl for type generation, manual envelopes for complex cases

**Rationale**:

- Turkish carrier SOAP APIs have inconsistent WSDL quality
- Some operations require custom XML namespaces
- Need flexibility for fault handling

**Implementation**:

```go
// For well-defined WSDL (Aras, Yurtiçi)
//go:generate gowsdl -o aras_types.go -p aras https://customerws.araskargo.com.tr/...

// For complex cases, manual envelope construction
func (a *ArasAdapter) buildCreateShipmentEnvelope(req CreateShipmentRequest) ([]byte, error) {
    envelope := &soap.Envelope{
        Body: soap.Body{
            Content: &arasCreateShipmentRequest{
                // Map fields
            },
        },
    }
    return xml.Marshal(envelope)
}
```

**XML Parsing Strategy**:

- Use `encoding/xml` for standard cases
- Use `beevik/etree` for dynamic XML manipulation
- Always validate response structure before parsing

---

### 6. REST Client Strategy

**Decision**: Standard `net/http` with custom client wrapper

**Rationale**:

- No need for heavy HTTP client libraries
- Full control over retry logic and circuit breaker
- Easy to mock for testing

**Implementation**:

```go
type RESTClient struct {
    client         *http.Client
    baseURL        string
    circuitBreaker *gobreaker.CircuitBreaker
    retryConfig    RetryConfig
}

type RetryConfig struct {
    MaxAttempts     int
    BaseDelay       time.Duration
    MaxDelay        time.Duration
    RetryableErrors []string
}

func (c *RESTClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
    return c.circuitBreaker.Execute(func() (interface{}, error) {
        return c.doWithRetry(ctx, req)
    })
}
```

**Circuit Breaker Settings**:

- Failure threshold: 5 consecutive failures
- Success threshold: 2 successes to close
- Timeout: 30 seconds in open state

---

### 7. Credential Encryption

**Decision**: AES-256-GCM with KMS-managed keys

**Rationale**:

- Industry standard authenticated encryption
- GCM provides integrity verification
- KMS integration for key management
- Supports key rotation without re-encryption

**Implementation**:

```go
// Key hierarchy:
// 1. Master Key (in KMS) -> never leaves KMS
// 2. Data Encryption Key (DEK) -> encrypted by master key, stored in DB
// 3. Credentials -> encrypted by DEK

type CredentialEncryptor struct {
    kms    KMSClient
    keyID  string
}

func (e *CredentialEncryptor) Encrypt(ctx context.Context, plaintext []byte) (EncryptedData, error) {
    // 1. Generate random DEK
    dek := make([]byte, 32)
    rand.Read(dek)
    
    // 2. Encrypt DEK with KMS
    encryptedDEK, err := e.kms.Encrypt(ctx, e.keyID, dek)
    
    // 3. Encrypt plaintext with DEK using AES-256-GCM
    block, _ := aes.NewCipher(dek)
    gcm, _ := cipher.NewGCM(block)
    nonce := make([]byte, gcm.NonceSize())
    rand.Read(nonce)
    ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
    
    return EncryptedData{
        Ciphertext:   ciphertext,
        EncryptedDEK: encryptedDEK,
    }, nil
}
```

---

### 8. Idempotency Implementation

**Decision**: Database-backed with request hash comparison

**Rationale**:

- Survives server restarts
- Works across multiple instances
- Supports in-flight request detection

**Implementation Details**:

```go
// Canonical JSON for hashing
func canonicalJSON(v interface{}) ([]byte, error) {
    // 1. Marshal to JSON
    // 2. Unmarshal to map[string]interface{}
    // 3. Sort keys recursively
    // 4. Marshal with no whitespace
}

// Hash computation
func hashRequest(body []byte) string {
    canonical, _ := canonicalJSON(body)
    hash := sha256.Sum256(canonical)
    return hex.EncodeToString(hash[:])
}

// Idempotency middleware
func IdempotencyMiddleware(repo IdempotencyRepository) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            key := r.Header.Get("Idempotency-Key")
            if key == "" {
                // Return 400 for required endpoints
            }
            
            // Check existing record
            record, err := repo.Get(ctx, orgID, key, endpoint)
            if record != nil {
                if record.RequestHash != hashRequest(body) {
                    // Return 422 IDEMPOTENCY_REPLAYED
                }
                // Return cached response
                w.Header().Set("X-Idempotency-Replayed", "true")
                w.WriteHeader(record.ResponseStatus)
                w.Write(record.ResponseBody)
                return
            }
            
            // Insert in-flight record
            repo.InsertInFlight(ctx, orgID, key, endpoint, requestHash)
            
            // Capture response
            rec := httptest.NewRecorder()
            next.ServeHTTP(rec, r)
            
            // Store response
            repo.UpdateResponse(ctx, orgID, key, endpoint, rec.Code, rec.Body.Bytes())
            
            // Write actual response
            copyResponse(w, rec)
        })
    }
}
```

---

### 9. Outbox Pattern Implementation

**Decision**: PostgreSQL table with polling publisher

**Rationale**:

- Guarantees exactly-once event publishing
- No additional infrastructure (uses existing PostgreSQL)
- Simple to implement and debug

**Implementation**:

```go
// Within transaction
func (s *ShipmentService) PurchaseLabel(ctx context.Context, ...) error {
    return s.db.WithTransaction(ctx, func(tx pgx.Tx) error {
        // ... business logic ...
        
        // Write to outbox in same transaction
        _, err := tx.Exec(ctx, `
            INSERT INTO outbox (aggregate_type, aggregate_id, event_type, payload)
            VALUES ($1, $2, $3, $4)
        `, "shipment", shipment.ID, "shipment.label_purchased", payload)
        
        return err
    })
}

// Publisher job (runs every 5 seconds)
func (p *OutboxPublisher) Process(ctx context.Context) error {
    rows, err := p.db.Query(ctx, `
        SELECT id, aggregate_type, aggregate_id, event_type, payload
        FROM outbox
        WHERE published_at IS NULL
        ORDER BY created_at
        LIMIT 100
        FOR UPDATE SKIP LOCKED
    `)
    
    for rows.Next() {
        // Publish event
        // Mark as published
    }
}
```

---

### 10. Webhook Delivery Strategy

**Decision**: Asynq tasks with exponential backoff

**Rationale**:

- Reliable delivery with automatic retries
- Built-in DLQ support
- Configurable retry schedule

**Implementation**:

```go
type WebhookDeliveryPayload struct {
    DeliveryID   uuid.UUID `json:"delivery_id"`
    EndpointURL  string    `json:"endpoint_url"`
    Secret       string    `json:"secret"`
    EventPayload []byte    `json:"event_payload"`
    Attempt      int       `json:"attempt"`
}

func (h *WebhookDeliveryHandler) ProcessTask(ctx context.Context, t *asynq.Task) error {
    var payload WebhookDeliveryPayload
    json.Unmarshal(t.Payload(), &payload)
    
    // Sign payload
    timestamp := time.Now().Unix()
    signature := signWebhook(payload.EventPayload, payload.Secret, timestamp)
    
    // Send request
    req, _ := http.NewRequestWithContext(ctx, "POST", payload.EndpointURL, bytes.NewReader(payload.EventPayload))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Webhook-Timestamp", strconv.FormatInt(timestamp, 10))
    req.Header.Set("X-Webhook-Signature", signature)
    
    resp, err := h.client.Do(req)
    if err != nil || resp.StatusCode >= 300 {
        // Update delivery record
        // Return error to trigger retry
        return fmt.Errorf("delivery failed: %v", err)
    }
    
    // Mark as delivered
    return nil
}

// Retry schedule
var webhookRetrySchedule = []time.Duration{
    1 * time.Minute,
    5 * time.Minute,
    30 * time.Minute,
    2 * time.Hour,
    8 * time.Hour,
    24 * time.Hour,
}
```

---

### 11. Rate Limiting Strategy

**Decision**: Redis-based sliding window

**Rationale**:

- Works across multiple instances
- Accurate rate limiting
- Low latency

**Implementation**:

```go
func (r *RateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, RateLimitInfo, error) {
    now := time.Now()
    windowStart := now.Add(-window)
    
    pipe := r.redis.Pipeline()
    
    // Remove old entries
    pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart.UnixNano(), 10))
    
    // Add current request
    pipe.ZAdd(ctx, key, redis.Z{Score: float64(now.UnixNano()), Member: now.UnixNano()})
    
    // Count requests in window
    pipe.ZCard(ctx, key)
    
    // Set expiry
    pipe.Expire(ctx, key, window)
    
    results, err := pipe.Exec(ctx)
    
    count := results[2].(*redis.IntCmd).Val()
    
    return count <= int64(limit), RateLimitInfo{
        Limit:     limit,
        Remaining: max(0, limit-int(count)),
        Reset:     now.Add(window),
    }, nil
}
```

---

### 12. Observability Stack

**Decision**: OpenTelemetry + Prometheus + structured logging

**Rationale**:

- OpenTelemetry for distributed tracing
- Prometheus for metrics (industry standard)
- Structured JSON logging for log aggregation

**Implementation**:

```go
// Tracing
func initTracer() (*sdktrace.TracerProvider, error) {
    exporter, err := otlptrace.New(ctx, otlptracegrpc.NewClient())
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.ServiceNameKey.String("shipaggregator"),
        )),
    )
    otel.SetTracerProvider(tp)
    return tp, nil
}

// Metrics
var (
    httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "http_requests_total",
        Help: "Total HTTP requests",
    }, []string{"method", "path", "status"})
    
    httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "http_request_duration_seconds",
        Help:    "HTTP request duration",
        Buckets: prometheus.DefBuckets,
    }, []string{"method", "path"})
    
    providerAPILatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "provider_api_latency_seconds",
        Help:    "Provider API call latency",
        Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
    }, []string{"provider", "operation"})
)

// Structured logging
type Logger struct {
    *slog.Logger
}

func (l *Logger) WithContext(ctx context.Context) *slog.Logger {
    return l.With(
        "trace_id", ctx.Value(traceIDKey),
        "organization_id", ctx.Value(orgIDKey),
    )
}
```

---

### 13. PII Masking Implementation

**Decision**: Centralized masking functions with field-based rules

**Rationale**:

- Consistent masking across all log outputs
- Easy to audit and update rules
- Supports nested structures

**Implementation**:

```go
type PIIMasker struct {
    rules map[string]MaskFunc
}

type MaskFunc func(string) string

var defaultMasker = &PIIMasker{
    rules: map[string]MaskFunc{
        "phone": func(s string) string {
            if len(s) < 8 {
                return "****"
            }
            return s[:4] + "****" + s[len(s)-2:]
        },
        "email": func(s string) string {
            parts := strings.Split(s, "@")
            if len(parts) != 2 {
                return "***@***"
            }
            return parts[0][:min(2, len(parts[0]))] + "***@" + parts[1]
        },
        "contact_name": func(s string) string {
            if len(s) < 3 {
                return "***"
            }
            return s[:3] + "***"
        },
        "street_line_1": func(s string) string {
            if len(s) < 3 {
                return "***"
            }
            return s[:3] + "***"
        },
        "street_line_2": func(s string) string {
            if len(s) < 3 {
                return "***"
            }
            return s[:3] + "***"
        },
        "password":    func(s string) string { return "[REDACTED]" },
        "api_key":     func(s string) string { return s[:8] + "****" + s[len(s)-4:] },
        "credentials": func(s string) string { return "[ENCRYPTED]" },
    },
}

func (m *PIIMasker) MaskStruct(v interface{}) interface{} {
    // Use reflection to walk struct and apply masking rules
}
```

---

### 14. Database Connection Pooling

**Decision**: pgxpool with tuned settings

**Rationale**:

- pgx provides native connection pooling
- Better performance than database/sql pool
- Supports prepared statements

**Configuration**:

```go
config, _ := pgxpool.ParseConfig(databaseURL)

// Pool settings
config.MaxConns = 50                    // Max connections
config.MinConns = 10                    // Min idle connections
config.MaxConnLifetime = 1 * time.Hour  // Max connection age
config.MaxConnIdleTime = 30 * time.Minute
config.HealthCheckPeriod = 1 * time.Minute

// Connection settings
config.ConnConfig.ConnectTimeout = 5 * time.Second

pool, _ := pgxpool.NewWithConfig(ctx, config)
```

---

### 15. Testing Strategy Details

**Decision**: Multi-layer testing with dockertest for integration

**Rationale**:

- Unit tests for business logic
- Integration tests with real PostgreSQL/Redis
- Contract tests for API compliance
- E2E tests for critical flows

**Implementation**:

```go
// Unit test example
func TestLedgerService_Debit(t *testing.T) {
    repo := mocks.NewMockLedgerRepository(t)
    service := billing.NewLedgerService(repo)
    
    repo.EXPECT().GetBalance(mock.Anything, orgID).Return(100000, nil)
    repo.EXPECT().CreateEntry(mock.Anything, mock.Anything).Return(&domain.LedgerEntry{}, nil)
    
    entry, err := service.Debit(ctx, DebitRequest{Amount: 50000})
    assert.NoError(t, err)
    assert.Equal(t, 50000, entry.BalanceAfterKurus)
}

// Integration test with dockertest
func TestShipmentRepository_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip()
    }
    
    pool, _ := dockertest.NewPool("")
    resource, _ := pool.RunWithOptions(&dockertest.RunOptions{
        Repository: "postgres",
        Tag:        "15",
        Env:        []string{"POSTGRES_PASSWORD=test"},
    })
    
    // Run migrations
    // Execute tests
    // Cleanup
}

// Contract test with schemathesis
func TestOpenAPIContract(t *testing.T) {
    // Load openapi.yaml
    // Start test server
    // Run schemathesis against all endpoints
}
```

---

## Best Practices Applied

### 1. Error Handling

- Use custom error types with error codes
- Wrap errors with context using `fmt.Errorf("...: %w", err)`
- Map all errors to RFC 7807 Problem Details at API boundary

### 2. Context Propagation

- Pass `context.Context` as first parameter to all functions
- Store request-scoped values (trace_id, org_id) in context
- Respect context cancellation in long-running operations

### 3. Graceful Shutdown

```go
func main() {
    // Start server
    server := &http.Server{...}
    go server.ListenAndServe()
    
    // Wait for interrupt
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit
    
    // Graceful shutdown
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    
    server.Shutdown(ctx)
    worker.Shutdown()
    pool.Close()
}
```

### 4. Configuration Management

- Use environment variables for all configuration
- Validate configuration at startup
- Fail fast on invalid configuration

### 5. Health Checks

```go
// Liveness: Is the process running?
GET /health/live -> 200 OK

// Readiness: Can the process handle requests?
GET /health/ready -> 200 OK (if DB and Redis connected)
```

---

## Security Best Practices

### 1. Input Validation

- Validate all inputs at API boundary
- Use strong typing (no `interface{}` for user input)
- Sanitize strings for SQL and HTML

### 2. Authentication

- Hash API keys with SHA-256 (not reversible)
- Use bcrypt for passwords (cost factor 12)
- Implement constant-time comparison for secrets

### 3. Authorization

- Check permissions at service layer
- Use RLS as defense in depth
- Log all authorization failures

### 4. Secrets Management

- Never log secrets
- Use KMS for encryption keys
- Rotate secrets regularly

---

## Performance Considerations

### 1. Database

- Use connection pooling
- Index all foreign keys and query predicates
- Use EXPLAIN ANALYZE for query optimization
- Consider read replicas for reporting queries

### 2. Caching

- Cache reference data (cities, districts) in Redis
- Cache provider capabilities
- Use cache-aside pattern with TTL

### 3. API

- Use cursor-based pagination
- Implement request coalescing for provider calls
- Use HTTP/2 for multiplexing

---

## Deployment Considerations

### 1. Container Image

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /api ./cmd/api

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /api /api
ENTRYPOINT ["/api"]
```

### 2. Kubernetes Resources

- Deployment with 3+ replicas
- HorizontalPodAutoscaler based on CPU/memory
- PodDisruptionBudget for availability
- NetworkPolicy for security

### 3. Database Migrations

- Use golang-migrate for migrations
- Run migrations as init container or separate job
- Always test rollback

---

## Resolved Clarifications

| Item | Resolution |
|------|------------|
| Go version | Go 1.22+ for enhanced routing |
| SOAP client approach | Hybrid: gowsdl + manual envelopes |
| Job queue | Asynq with Redis |
| Credential encryption | AES-256-GCM with KMS |
| Rate limiting | Redis sliding window |
| Observability | OpenTelemetry + Prometheus + slog |

---

## References

- [chi router documentation](https://go-chi.io/)
- [sqlc documentation](https://sqlc.dev/)
- [pgx documentation](https://github.com/jackc/pgx)
- [Asynq documentation](https://github.com/hibiken/asynq)
- [OpenTelemetry Go](https://opentelemetry.io/docs/instrumentation/go/)
- [RFC 7807 - Problem Details](https://tools.ietf.org/html/rfc7807)
- [Iyzico API Documentation](https://dev.iyzipay.com/)

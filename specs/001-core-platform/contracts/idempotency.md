# Idempotency Specification

**Feature Branch**: `001-core-platform`  
**Created**: 2026-01-22  
**Status**: Draft

---

## Overview

This document defines the idempotency mechanism for the ShipAggregator API. Idempotency ensures that retrying a request produces the same result as the original request, preventing duplicate operations.

---

## Idempotent Endpoints

The following endpoints require an `Idempotency-Key` header:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/quotes` | POST | Create quote |
| `/shipments` | POST | Create shipment |
| `/shipments/{id}/purchase-label` | POST | Purchase label |
| `/billing/topups` | POST | Create top-up |

---

## Idempotency Key

### Header Format

```http
Idempotency-Key: <unique-key>
```

### Key Requirements

| Property | Requirement |
|----------|-------------|
| Format | String, 1-100 characters |
| Allowed Characters | Alphanumeric, hyphens, underscores |
| Case Sensitivity | Case-sensitive |
| Uniqueness | Unique per organization per endpoint |
| Expiration | 24 hours |

### Recommended Key Formats

```
# UUID v4
Idempotency-Key: 550e8400-e29b-41d4-a716-446655440000

# Order reference
Idempotency-Key: order-12345-shipment

# Timestamp-based
Idempotency-Key: 2026-01-22T14:30:00Z-shipment-12345

# Hash-based
Idempotency-Key: sha256-abc123def456
```

---

## Request Flow

### First Request

```
Client                          Server
  |                               |
  |  POST /shipments              |
  |  Idempotency-Key: key-123     |
  |  Body: {...}                  |
  |------------------------------>|
  |                               |
  |                               | 1. Check idempotency store
  |                               | 2. Key not found
  |                               | 3. Hash request body
  |                               | 4. Process request
  |                               | 5. Store result with key
  |                               |
  |  201 Created                  |
  |  X-Idempotency-Replayed: false|
  |  Body: {shipment}             |
  |<------------------------------|
```

### Duplicate Request (Same Body)

```
Client                          Server
  |                               |
  |  POST /shipments              |
  |  Idempotency-Key: key-123     |
  |  Body: {...}                  |
  |------------------------------>|
  |                               |
  |                               | 1. Check idempotency store
  |                               | 2. Key found
  |                               | 3. Hash matches
  |                               | 4. Return stored result
  |                               |
  |  201 Created                  |
  |  X-Idempotency-Replayed: true |
  |  Body: {shipment}             |
  |<------------------------------|
```

### Duplicate Request (Different Body)

```
Client                          Server
  |                               |
  |  POST /shipments              |
  |  Idempotency-Key: key-123     |
  |  Body: {different...}         |
  |------------------------------>|
  |                               |
  |                               | 1. Check idempotency store
  |                               | 2. Key found
  |                               | 3. Hash mismatch!
  |                               | 4. Return error
  |                               |
  |  422 Unprocessable Entity     |
  |  Body: {error}                |
  |<------------------------------|
```

---

## Response Headers

### X-Idempotency-Replayed

Indicates whether the response is from a replayed request.

| Value | Meaning |
|-------|---------|
| `false` | Original request processed |
| `true` | Cached response returned |

### X-Idempotency-Key

Echoes the idempotency key used.

```http
HTTP/1.1 201 Created
X-Idempotency-Key: order-12345-shipment
X-Idempotency-Replayed: false
Content-Type: application/json

{
  "id": "shp_550e8400-e29b-41d4-a716-446655440000",
  ...
}
```

---

## Error Handling

### Missing Idempotency Key

```http
HTTP/1.1 400 Bad Request
Content-Type: application/problem+json

{
  "type": "https://api.shipaggregator.com/errors/missing-idempotency-key",
  "title": "Missing Idempotency Key",
  "status": 400,
  "detail": "Idempotency-Key header is required for this endpoint",
  "code": "MISSING_IDEMPOTENCY_KEY",
  "trace_id": "abc123"
}
```

### Invalid Idempotency Key Format

```http
HTTP/1.1 400 Bad Request
Content-Type: application/problem+json

{
  "type": "https://api.shipaggregator.com/errors/invalid-idempotency-key",
  "title": "Invalid Idempotency Key",
  "status": 400,
  "detail": "Idempotency key must be 1-100 alphanumeric characters, hyphens, or underscores",
  "code": "INVALID_IDEMPOTENCY_KEY",
  "trace_id": "abc123"
}
```

### Idempotency Key Reused with Different Body

```http
HTTP/1.1 422 Unprocessable Entity
Content-Type: application/problem+json

{
  "type": "https://api.shipaggregator.com/errors/idempotency-replayed",
  "title": "Idempotency Key Conflict",
  "status": 422,
  "detail": "Idempotency key 'order-12345' was already used with a different request body",
  "code": "IDEMPOTENCY_REPLAYED",
  "trace_id": "abc123",
  "idempotency_key": "order-12345",
  "original_request_at": "2026-01-22T14:00:00Z"
}
```

---

## Storage Schema

### idempotency_records Table

```sql
CREATE TABLE idempotency_records (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  organization_id UUID NOT NULL REFERENCES organizations(id),
  idempotency_key VARCHAR(100) NOT NULL,
  endpoint VARCHAR(100) NOT NULL,
  request_hash VARCHAR(64) NOT NULL,  -- SHA-256 of request body
  response_status INTEGER NOT NULL,
  response_body JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at TIMESTAMPTZ NOT NULL,
  
  CONSTRAINT uq_idempotency_lookup 
    UNIQUE (organization_id, idempotency_key, endpoint)
);

CREATE INDEX idx_idempotency_expires ON idempotency_records(expires_at);
```

---

## Request Body Hashing

### Algorithm

1. Serialize request body to canonical JSON (sorted keys, no whitespace)
2. Compute SHA-256 hash
3. Encode as lowercase hexadecimal

### Canonical JSON Rules

- Object keys sorted alphabetically
- No whitespace between elements
- Numbers without trailing zeros
- Strings escaped consistently

### Example

**Original Request:**

```json
{
  "destination": {
    "city_code": "06",
    "contact_name": "John"
  },
  "origin": {
    "city_code": "34",
    "contact_name": "Jane"
  }
}
```

**Canonical JSON:**

```json
{"destination":{"city_code":"06","contact_name":"John"},"origin":{"city_code":"34","contact_name":"Jane"}}
```

**SHA-256 Hash:**

```
e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
```

---

## Concurrency Handling

### Race Condition Prevention

Use database-level locking to prevent race conditions:

```sql
-- Attempt to insert idempotency record
INSERT INTO idempotency_records (
  organization_id, idempotency_key, endpoint, request_hash,
  response_status, response_body, expires_at
)
VALUES ($1, $2, $3, $4, $5, $6, NOW() + INTERVAL '24 hours')
ON CONFLICT (organization_id, idempotency_key, endpoint) DO NOTHING
RETURNING id;

-- If no row returned, record exists - check hash
SELECT request_hash, response_status, response_body
FROM idempotency_records
WHERE organization_id = $1 
  AND idempotency_key = $2 
  AND endpoint = $3
  AND expires_at > NOW();
```

### In-Flight Request Handling

For requests still being processed:

1. Insert record with `response_status = NULL`
2. Use `SELECT FOR UPDATE` to lock the row
3. Update with actual response when complete
4. Concurrent requests wait on lock

```sql
-- Mark request as in-flight
INSERT INTO idempotency_records (
  organization_id, idempotency_key, endpoint, request_hash,
  response_status, response_body, expires_at
)
VALUES ($1, $2, $3, $4, NULL, '{}', NOW() + INTERVAL '24 hours')
ON CONFLICT (organization_id, idempotency_key, endpoint) DO NOTHING
RETURNING id;

-- Lock and check status
SELECT id, request_hash, response_status, response_body
FROM idempotency_records
WHERE organization_id = $1 
  AND idempotency_key = $2 
  AND endpoint = $3
FOR UPDATE;

-- If response_status is NULL, wait and retry
-- If response_status is set, return cached response
```

---

## Expiration and Cleanup

### Expiration Policy

- Records expire 24 hours after creation
- Expired records are not returned
- Cleanup job runs hourly

### Cleanup Job

```sql
-- Delete expired records
DELETE FROM idempotency_records
WHERE expires_at < NOW() - INTERVAL '1 hour';
```

---

## Client Implementation

### Recommended Pattern

```python
import hashlib
import uuid
import time

def create_shipment_with_retry(client, shipment_data, max_retries=3):
    # Generate idempotency key once
    idempotency_key = str(uuid.uuid4())
    
    for attempt in range(max_retries):
        try:
            response = client.post(
                '/shipments',
                json=shipment_data,
                headers={'Idempotency-Key': idempotency_key}
            )
            
            if response.status_code == 201:
                return response.json()
            elif response.status_code == 422:
                error = response.json()
                if error['code'] == 'IDEMPOTENCY_REPLAYED':
                    # Key was reused with different body - this is a bug
                    raise ValueError("Idempotency key reused with different body")
                raise ValidationError(error)
            elif response.status_code >= 500:
                # Server error - retry with same key
                if attempt < max_retries - 1:
                    time.sleep(2 ** attempt)  # Exponential backoff
                    continue
                raise ServerError(response)
            else:
                raise APIError(response)
                
        except ConnectionError:
            # Network error - retry with same key
            if attempt < max_retries - 1:
                time.sleep(2 ** attempt)
                continue
            raise
    
    raise MaxRetriesExceeded()
```

### Key Generation Strategies

**Order-Based (Recommended for E-commerce):**

```python
def generate_idempotency_key(order_id, operation):
    return f"order-{order_id}-{operation}"

# Usage
key = generate_idempotency_key("12345", "create-shipment")
# Result: "order-12345-create-shipment"
```

**UUID-Based (General Purpose):**

```python
import uuid

def generate_idempotency_key():
    return str(uuid.uuid4())

# Result: "550e8400-e29b-41d4-a716-446655440000"
```

**Content-Based (Deterministic):**

```python
import hashlib
import json

def generate_idempotency_key(request_body):
    canonical = json.dumps(request_body, sort_keys=True, separators=(',', ':'))
    return hashlib.sha256(canonical.encode()).hexdigest()[:32]

# Result: "e3b0c44298fc1c149afbf4c8996fb924"
```

---

## Testing

### Test Cases

| Scenario | Expected Behavior |
|----------|-------------------|
| First request | Process and return 201 |
| Duplicate with same body | Return cached 201, `X-Idempotency-Replayed: true` |
| Duplicate with different body | Return 422 `IDEMPOTENCY_REPLAYED` |
| Missing key on required endpoint | Return 400 `MISSING_IDEMPOTENCY_KEY` |
| Invalid key format | Return 400 `INVALID_IDEMPOTENCY_KEY` |
| Key after expiration | Process as new request |
| Concurrent identical requests | One processes, others wait and return cached |

### Sandbox Testing

Use these test keys in sandbox to trigger specific behaviors:

| Key Pattern | Behavior |
|-------------|----------|
| `test-duplicate-*` | Always returns cached response |
| `test-conflict-*` | Always returns `IDEMPOTENCY_REPLAYED` |
| `test-expired-*` | Simulates expired key |

---

## Monitoring

### Metrics

| Metric | Description |
|--------|-------------|
| `idempotency.requests.total` | Total idempotent requests |
| `idempotency.cache_hits` | Requests served from cache |
| `idempotency.cache_misses` | New requests processed |
| `idempotency.conflicts` | Hash mismatch errors |
| `idempotency.records.count` | Active records in store |

### Alerts

| Condition | Alert |
|-----------|-------|
| Conflict rate > 1% | High idempotency conflicts |
| Cache hit rate > 50% | Unusual retry pattern |
| Record count growth > 10%/hour | Potential key leak |

---

## Best Practices

### Do

- ✅ Generate unique keys per logical operation
- ✅ Reuse the same key when retrying failed requests
- ✅ Include operation context in key (e.g., order ID)
- ✅ Store keys with your order/transaction records
- ✅ Handle `IDEMPOTENCY_REPLAYED` as a bug in your code

### Don't

- ❌ Generate new keys on retry
- ❌ Reuse keys across different operations
- ❌ Use sequential/predictable keys
- ❌ Share keys across organizations
- ❌ Rely on idempotency for business logic deduplication

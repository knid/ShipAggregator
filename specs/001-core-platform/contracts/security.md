# Security Specification

**Feature Branch**: `001-core-platform`  
**Created**: 2026-01-22  
**Status**: Draft

---

## Overview

This document defines the security architecture, authentication mechanisms, authorization model, and data protection measures for the ShipAggregator API.

---

## Authentication

### API Key Authentication

API keys are used for programmatic access to the API.

#### Key Format

```
sk_live_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
sk_test_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

| Component | Description |
|-----------|-------------|
| `sk` | Secret key prefix |
| `live`/`test` | Environment indicator |
| `xxx...` | 32-character random string (Base62) |

#### Key Generation

```python
import secrets
import string

def generate_api_key(environment: str) -> str:
    prefix = f"sk_{environment}_"
    charset = string.ascii_letters + string.digits
    random_part = ''.join(secrets.choice(charset) for _ in range(32))
    return prefix + random_part
```

#### Key Storage

- Full key shown only once at creation
- Store SHA-256 hash in database
- Store first 8 characters as prefix for identification

```sql
INSERT INTO api_keys (
  organization_id, name, key_prefix, key_hash, scopes
) VALUES (
  $1, $2, 
  LEFT($3, 8),  -- key_prefix
  encode(sha256($3::bytea), 'hex'),  -- key_hash
  $4
);
```

#### Key Rotation

1. Generate new key
2. Store new key hash
3. Move old key hash to `previous_key_hash`
4. Set `previous_key_expires_at` to NOW() + 24 hours
5. Both keys valid during grace period

#### Request Authentication

```http
GET /v1/shipments HTTP/1.1
Host: api.shipaggregator.com
Authorization: Bearer sk_live_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

#### Validation Flow

```
1. Extract token from Authorization header
2. Compute SHA-256 hash of token
3. Query api_keys by key_hash OR previous_key_hash
4. Verify key is active and not expired
5. Verify organization is active
6. Check IP allowlist (if configured)
7. Check rate limits
8. Set request context (organization_id, scopes)
```

---

### Session Authentication

Session-based authentication for dashboard users.

#### Login Flow

```
1. POST /auth/login with email + password
2. Verify password against bcrypt hash
3. Generate session token (32 bytes, Base64)
4. Store session with SHA-256 hash of token
5. Set session cookie (HttpOnly, Secure, SameSite=Strict)
6. Return session token in response body
```

#### Session Token Format

```
sess_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

#### Session Storage

```sql
INSERT INTO sessions (
  member_id, token_hash, ip_address, user_agent, expires_at
) VALUES (
  $1,
  encode(sha256($2::bytea), 'hex'),
  $3, $4,
  NOW() + INTERVAL '24 hours'
);
```

#### Session Validation

```
1. Extract session token from cookie or Authorization header
2. Compute SHA-256 hash
3. Query sessions by token_hash
4. Verify session not expired
5. Verify member is active
6. Update last_activity_at
7. Set request context (member_id, organization_id, permissions)
```

#### Session Expiration

| Condition | Expiration |
|-----------|------------|
| Default | 24 hours from creation |
| Inactivity | 2 hours without activity |
| Explicit logout | Immediate |
| Password change | All sessions invalidated |

---

## Authorization

### Permission Model

Permissions follow the format: `resource:action`

#### Available Permissions

| Permission | Description |
|------------|-------------|
| `organization:read` | View organization details |
| `organization:update` | Update organization settings |
| `member:read` | View members |
| `member:create` | Invite members |
| `member:update` | Update member roles |
| `member:delete` | Remove members |
| `role:read` | View roles |
| `role:create` | Create custom roles |
| `role:update` | Update custom roles |
| `role:delete` | Delete custom roles |
| `api_key:read` | View API keys |
| `api_key:create` | Create API keys |
| `api_key:update` | Update API keys |
| `api_key:delete` | Revoke API keys |
| `api_key:rotate` | Rotate API keys |
| `address:read` | View addresses |
| `address:create` | Create addresses |
| `address:update` | Update addresses |
| `address:delete` | Delete addresses |
| `provider_account:read` | View BYOA accounts |
| `provider_account:create` | Create BYOA accounts |
| `provider_account:update` | Update BYOA accounts |
| `provider_account:delete` | Delete BYOA accounts |
| `quote:read` | View quotes |
| `quote:create` | Create quotes |
| `shipment:read` | View shipments |
| `shipment:create` | Create shipments |
| `shipment:update` | Update shipments |
| `shipment:cancel` | Cancel shipments |
| `shipment:label` | Purchase labels |
| `shipment:pickup` | Schedule pickups |
| `shipment:return` | Create returns |
| `webhook:read` | View webhooks |
| `webhook:create` | Create webhooks |
| `webhook:update` | Update webhooks |
| `webhook:delete` | Delete webhooks |
| `billing:read` | View billing/balance |
| `billing:topup` | Create top-ups |
| `billing:refund` | Request refunds |
| `admin:audit` | View audit logs |
| `admin:provider_logs` | View provider logs |
| `admin:health` | View provider health |
| `admin:dlq` | Manage DLQ |

#### System Roles

| Role | Permissions |
|------|-------------|
| `owner` | All permissions |
| `admin` | All except `organization:delete` |
| `operator` | `shipment:*`, `quote:*`, `address:*`, `webhook:read` |
| `viewer` | `*:read` only |

### API Key Scopes

API keys can be restricted to specific scopes:

```json
{
  "scopes": [
    "shipment:read",
    "shipment:create",
    "quote:create"
  ]
}
```

If `scopes` is empty, the key has all permissions of the creating user.

### Authorization Check

```python
def check_permission(context, required_permission):
    # For API key auth
    if context.auth_type == 'api_key':
        if context.scopes and required_permission not in context.scopes:
            raise ForbiddenError(f"API key missing scope: {required_permission}")
    
    # For session auth
    if context.auth_type == 'session':
        if required_permission not in context.permissions:
            raise ForbiddenError(f"Permission denied: {required_permission}")
```

---

## Multi-Tenant Isolation

### Row-Level Security

All tenant-scoped tables enforce organization isolation:

```sql
-- Enable RLS
ALTER TABLE shipments ENABLE ROW LEVEL SECURITY;

-- Policy for SELECT
CREATE POLICY shipments_select ON shipments
  FOR SELECT
  USING (organization_id = current_setting('app.current_organization_id')::uuid);

-- Policy for INSERT
CREATE POLICY shipments_insert ON shipments
  FOR INSERT
  WITH CHECK (organization_id = current_setting('app.current_organization_id')::uuid);

-- Policy for UPDATE
CREATE POLICY shipments_update ON shipments
  FOR UPDATE
  USING (organization_id = current_setting('app.current_organization_id')::uuid);

-- Policy for DELETE
CREATE POLICY shipments_delete ON shipments
  FOR DELETE
  USING (organization_id = current_setting('app.current_organization_id')::uuid);
```

### Request Context

Set organization context at request start:

```sql
-- Set at beginning of request
SET LOCAL app.current_organization_id = 'org-uuid-here';
SET LOCAL app.current_actor_id = 'actor-uuid-here';
SET LOCAL app.current_actor_type = 'member';  -- or 'api_key', 'system'
SET LOCAL app.trace_id = 'trace-id-here';
```

---

## Webhook Security

### Signature Generation

Webhooks are signed using HMAC-SHA256:

```python
import hmac
import hashlib
import time

def sign_webhook(payload: bytes, secret: str, timestamp: int) -> str:
    message = f"{timestamp}.{payload.decode('utf-8')}"
    signature = hmac.new(
        secret.encode('utf-8'),
        message.encode('utf-8'),
        hashlib.sha256
    ).hexdigest()
    return f"v1={signature}"
```

### Webhook Headers

```http
POST /webhook-endpoint HTTP/1.1
Content-Type: application/json
X-Webhook-Id: evt_550e8400-e29b-41d4-a716-446655440000
X-Webhook-Timestamp: 1706015400
X-Webhook-Signature: v1=5d41402abc4b2a76b9719d911017c592
```

### Signature Verification

```python
import hmac
import hashlib
import time

def verify_webhook_signature(
    payload: bytes,
    signature_header: str,
    timestamp_header: str,
    secret: str,
    tolerance_seconds: int = 300
) -> bool:
    # Parse timestamp
    timestamp = int(timestamp_header)
    
    # Check timestamp is within tolerance
    current_time = int(time.time())
    if abs(current_time - timestamp) > tolerance_seconds:
        return False  # Replay attack protection
    
    # Extract signature
    if not signature_header.startswith('v1='):
        return False
    expected_signature = signature_header[3:]
    
    # Compute expected signature
    message = f"{timestamp}.{payload.decode('utf-8')}"
    computed_signature = hmac.new(
        secret.encode('utf-8'),
        message.encode('utf-8'),
        hashlib.sha256
    ).hexdigest()
    
    # Constant-time comparison
    return hmac.compare_digest(computed_signature, expected_signature)
```

### Replay Protection

- Timestamp must be within 5 minutes of current time
- Store processed webhook IDs to detect duplicates
- Reject webhooks with duplicate IDs within 24 hours

---

## Data Protection

### Encryption at Rest

| Data Type | Encryption |
|-----------|------------|
| Database | AES-256 (managed by cloud provider) |
| Object Storage | AES-256 (managed by cloud provider) |
| BYOA Credentials | AES-256-GCM (application-level) |
| API Key Secrets | SHA-256 hash only (not reversible) |

### BYOA Credential Encryption

```python
from cryptography.hazmat.primitives.ciphers.aead import AESGCM
import os

def encrypt_credentials(credentials: dict, key: bytes) -> tuple[bytes, bytes]:
    """Encrypt BYOA credentials with AES-256-GCM."""
    plaintext = json.dumps(credentials).encode('utf-8')
    iv = os.urandom(12)  # 96-bit IV for GCM
    aesgcm = AESGCM(key)
    ciphertext = aesgcm.encrypt(iv, plaintext, None)
    return ciphertext, iv

def decrypt_credentials(ciphertext: bytes, iv: bytes, key: bytes) -> dict:
    """Decrypt BYOA credentials."""
    aesgcm = AESGCM(key)
    plaintext = aesgcm.decrypt(iv, ciphertext, None)
    return json.loads(plaintext.decode('utf-8'))
```

### Encryption Key Management

- Master key stored in cloud KMS (AWS KMS, GCP KMS, etc.)
- Data encryption keys (DEK) derived per organization
- Key rotation supported without re-encryption

### Encryption in Transit

- TLS 1.2+ required for all connections
- HSTS enabled with 1-year max-age
- Certificate pinning recommended for mobile clients

---

## PII Handling

### PII Fields

| Table | PII Fields |
|-------|------------|
| `members` | email, name |
| `addresses` | contact_name, phone, email, street_line_1, street_line_2 |
| `shipments` | origin_address, destination_address |
| `provider_api_logs` | request_body_preview, response_body_preview |
| `webhook_deliveries` | response_body_preview |

### PII Masking in Logs

```python
def mask_pii(data: dict) -> dict:
    """Mask PII fields in log data."""
    pii_fields = ['phone', 'email', 'contact_name', 'street_line_1', 'street_line_2']
    masked = data.copy()
    
    for field in pii_fields:
        if field in masked:
            value = masked[field]
            if field == 'phone':
                masked[field] = value[:4] + '****' + value[-2:]
            elif field == 'email':
                parts = value.split('@')
                masked[field] = parts[0][:2] + '***@' + parts[1]
            else:
                masked[field] = value[:3] + '***'
    
    return masked
```

### Soft Delete and Retention

- PII-containing records use soft delete (`deleted_at` timestamp)
- Hard delete after 30 days
- Audit logs retained for 2 years (PII masked)

---

## Rate Limiting

### Default Limits

| Scope | Limit |
|-------|-------|
| Per API Key | 100 requests/minute |
| Per Organization | 1000 requests/minute |
| Per IP (unauthenticated) | 10 requests/minute |

### Rate Limit Headers

```http
HTTP/1.1 200 OK
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1706015460
```

### Rate Limit Algorithm

Token bucket algorithm with Redis:

```python
import time
import redis

def check_rate_limit(redis_client, key: str, limit: int, window: int) -> tuple[bool, dict]:
    """
    Check rate limit using sliding window.
    Returns (allowed, headers).
    """
    now = time.time()
    window_start = now - window
    
    pipe = redis_client.pipeline()
    pipe.zremrangebyscore(key, 0, window_start)
    pipe.zadd(key, {str(now): now})
    pipe.zcard(key)
    pipe.expire(key, window)
    _, _, count, _ = pipe.execute()
    
    remaining = max(0, limit - count)
    reset_time = int(now) + window
    
    headers = {
        'X-RateLimit-Limit': limit,
        'X-RateLimit-Remaining': remaining,
        'X-RateLimit-Reset': reset_time
    }
    
    return count <= limit, headers
```

### Endpoint-Specific Limits

| Endpoint | Limit | Reason |
|----------|-------|--------|
| `POST /auth/login` | 5/minute per IP | Brute force protection |
| `POST /quotes` | 50/minute | Provider API costs |
| `POST /shipments` | 100/minute | Standard |
| `POST /billing/topups` | 10/minute | Payment fraud prevention |

---

## IP Allowlisting

### API Key IP Restriction

```json
{
  "ip_allowlist": [
    "203.0.113.0/24",
    "198.51.100.50"
  ]
}
```

### Validation

```python
import ipaddress

def check_ip_allowlist(client_ip: str, allowlist: list[str]) -> bool:
    """Check if client IP is in allowlist."""
    if not allowlist:
        return True  # No restriction
    
    client = ipaddress.ip_address(client_ip)
    
    for entry in allowlist:
        try:
            network = ipaddress.ip_network(entry, strict=False)
            if client in network:
                return True
        except ValueError:
            continue
    
    return False
```

---

## Security Headers

### Response Headers

```http
Strict-Transport-Security: max-age=31536000; includeSubDomains; preload
X-Content-Type-Options: nosniff
X-Frame-Options: DENY
X-XSS-Protection: 1; mode=block
Content-Security-Policy: default-src 'none'; frame-ancestors 'none'
Referrer-Policy: strict-origin-when-cross-origin
Cache-Control: no-store
```

### CORS Configuration

```http
Access-Control-Allow-Origin: https://dashboard.shipaggregator.com
Access-Control-Allow-Methods: GET, POST, PATCH, DELETE, OPTIONS
Access-Control-Allow-Headers: Authorization, Content-Type, Idempotency-Key
Access-Control-Max-Age: 86400
Access-Control-Allow-Credentials: true
```

---

## Audit Logging

### Logged Events

| Event | Logged Data |
|-------|-------------|
| Authentication | Success/failure, IP, user agent |
| Authorization failure | Resource, action, actor |
| Resource creation | Resource type, ID, actor |
| Resource modification | Resource type, ID, old/new values, actor |
| Resource deletion | Resource type, ID, actor |
| API key operations | Create, rotate, revoke |
| Sensitive data access | BYOA credentials decryption |

### Audit Log Format

```json
{
  "id": "aud_550e8400-e29b-41d4-a716-446655440000",
  "timestamp": "2026-01-22T14:30:00Z",
  "organization_id": "org_123",
  "actor_type": "member",
  "actor_id": "mem_456",
  "action": "shipment.create",
  "resource_type": "shipment",
  "resource_id": "shp_789",
  "ip_address": "203.0.113.50",
  "user_agent": "Mozilla/5.0...",
  "trace_id": "abc123",
  "changes": {
    "status": {"old": null, "new": "draft"}
  }
}
```

---

## Vulnerability Management

### Input Validation

- All inputs validated against schema
- SQL injection prevented via parameterized queries
- XSS prevented via output encoding
- Path traversal prevented via allowlist validation

### Dependency Security

- Automated dependency scanning (Dependabot, Snyk)
- Security patches applied within 7 days (critical) / 30 days (high)
- No dependencies with known critical vulnerabilities

### Penetration Testing

- Annual third-party penetration test
- Quarterly automated security scans
- Bug bounty program for responsible disclosure

---

## Incident Response

### Security Incident Classification

| Severity | Description | Response Time |
|----------|-------------|---------------|
| Critical | Data breach, system compromise | 1 hour |
| High | Authentication bypass, privilege escalation | 4 hours |
| Medium | Information disclosure, DoS | 24 hours |
| Low | Minor vulnerability, hardening | 7 days |

### Incident Response Steps

1. **Detection**: Automated alerts or manual report
2. **Containment**: Isolate affected systems
3. **Investigation**: Determine scope and impact
4. **Eradication**: Remove threat
5. **Recovery**: Restore normal operations
6. **Lessons Learned**: Post-incident review

### Customer Notification

- Data breach affecting customer data: Within 72 hours
- Service disruption: Real-time status page updates
- Security advisory: Email to affected customers

---

## Compliance

### Data Residency

- All data stored in Turkey (TR region)
- No cross-border data transfer without consent
- KVKK (Turkish Data Protection Law) compliant

### Retention Periods

| Data Type | Retention |
|-----------|-----------|
| Shipment records | 7 years |
| Payment records | 10 years |
| Audit logs | 2 years |
| Application logs | 30 days |
| Soft-deleted PII | 30 days |

### Access Controls

- Principle of least privilege
- Regular access reviews (quarterly)
- Privileged access requires MFA
- Production access logged and audited

# Error Handling Specification

**Feature Branch**: `001-core-platform`  
**Created**: 2026-01-22  
**Status**: Draft

---

## Overview

This document defines the error handling strategy, error codes, and response formats for the ShipAggregator API. All errors follow RFC 7807 (Problem Details for HTTP APIs) with extensions.

---

## Error Response Format

All error responses use `application/problem+json` content type:

```json
{
  "type": "https://api.shipaggregator.com/errors/validation-failed",
  "title": "Validation Failed",
  "status": 422,
  "detail": "One or more fields failed validation",
  "instance": "/v1/shipments",
  "code": "VALIDATION_FAILED",
  "trace_id": "abc123def456",
  "errors": [
    {
      "field": "origin.phone",
      "message": "Invalid phone number format. Expected E.164 format (+905XXXXXXXXX)"
    }
  ]
}
```

### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string (URI) | Yes | URI reference identifying the error type |
| `title` | string | Yes | Short, human-readable summary |
| `status` | integer | Yes | HTTP status code |
| `detail` | string | No | Human-readable explanation specific to this occurrence |
| `instance` | string (URI) | No | URI reference identifying the specific occurrence |
| `code` | string | Yes | Machine-readable error code |
| `trace_id` | string | Yes | Request trace ID for debugging |
| `errors` | array | No | Field-level validation errors |

---

## Error Codes

### Authentication Errors (4xx)

#### AUTH_INVALID

Authentication credentials are missing or invalid.

| Property | Value |
|----------|-------|
| HTTP Status | 401 |
| Type URI | `https://api.shipaggregator.com/errors/auth-invalid` |
| Title | Authentication Required |

**Causes:**

- Missing `Authorization` header
- Invalid API key format
- Expired API key
- Revoked API key
- Invalid session token

**Example:**

```json
{
  "type": "https://api.shipaggregator.com/errors/auth-invalid",
  "title": "Authentication Required",
  "status": 401,
  "detail": "The provided API key is invalid or has been revoked",
  "code": "AUTH_INVALID",
  "trace_id": "abc123"
}
```

---

#### FORBIDDEN

Authenticated but not authorized for the requested action.

| Property | Value |
|----------|-------|
| HTTP Status | 403 |
| Type URI | `https://api.shipaggregator.com/errors/forbidden` |
| Title | Forbidden |

**Causes:**

- Insufficient permissions for the action
- API key missing required scope
- Resource belongs to different organization
- Account suspended

**Example:**

```json
{
  "type": "https://api.shipaggregator.com/errors/forbidden",
  "title": "Forbidden",
  "status": 403,
  "detail": "API key does not have 'shipment:create' scope required for this action",
  "code": "FORBIDDEN",
  "trace_id": "abc123"
}
```

---

#### RATE_LIMITED

Request rate limit exceeded.

| Property | Value |
|----------|-------|
| HTTP Status | 429 |
| Type URI | `https://api.shipaggregator.com/errors/rate-limited` |
| Title | Rate Limit Exceeded |

**Headers:**

- `Retry-After`: Seconds until rate limit resets
- `X-RateLimit-Limit`: Request limit per window
- `X-RateLimit-Remaining`: Remaining requests in window
- `X-RateLimit-Reset`: Unix timestamp when limit resets

**Example:**

```json
{
  "type": "https://api.shipaggregator.com/errors/rate-limited",
  "title": "Rate Limit Exceeded",
  "status": 429,
  "detail": "Rate limit of 100 requests per minute exceeded. Retry after 45 seconds.",
  "code": "RATE_LIMITED",
  "trace_id": "abc123",
  "retry_after": 45
}
```

---

### Validation Errors (4xx)

#### VALIDATION_FAILED

Request body or parameters failed validation.

| Property | Value |
|----------|-------|
| HTTP Status | 422 |
| Type URI | `https://api.shipaggregator.com/errors/validation-failed` |
| Title | Validation Failed |

**Example:**

```json
{
  "type": "https://api.shipaggregator.com/errors/validation-failed",
  "title": "Validation Failed",
  "status": 422,
  "detail": "One or more fields failed validation",
  "code": "VALIDATION_FAILED",
  "trace_id": "abc123",
  "errors": [
    {
      "field": "origin.phone",
      "message": "Invalid phone number format. Expected E.164 format (+905XXXXXXXXX)",
      "code": "INVALID_FORMAT"
    },
    {
      "field": "parcel.weight_grams",
      "message": "Must be between 1 and 150000",
      "code": "OUT_OF_RANGE"
    },
    {
      "field": "destination.neighborhood_code",
      "message": "Neighborhood code '999999' not found",
      "code": "INVALID_REFERENCE"
    }
  ]
}
```

**Field Error Codes:**

| Code | Description |
|------|-------------|
| `REQUIRED` | Field is required but missing |
| `INVALID_FORMAT` | Field format is invalid |
| `INVALID_TYPE` | Field type is incorrect |
| `OUT_OF_RANGE` | Value outside allowed range |
| `TOO_SHORT` | String shorter than minimum |
| `TOO_LONG` | String longer than maximum |
| `INVALID_REFERENCE` | Referenced entity not found |
| `INVALID_ENUM` | Value not in allowed enum |
| `DUPLICATE` | Value must be unique |

---

### Resource Errors (4xx)

#### NOT_FOUND

Requested resource does not exist.

| Property | Value |
|----------|-------|
| HTTP Status | 404 |
| Type URI | `https://api.shipaggregator.com/errors/not-found` |
| Title | Not Found |

**Example:**

```json
{
  "type": "https://api.shipaggregator.com/errors/not-found",
  "title": "Not Found",
  "status": 404,
  "detail": "Shipment with ID 'shp_550e8400-e29b-41d4-a716-446655440000' not found",
  "code": "NOT_FOUND",
  "trace_id": "abc123",
  "resource_type": "shipment",
  "resource_id": "shp_550e8400-e29b-41d4-a716-446655440000"
}
```

---

#### CONFLICT

Request conflicts with current resource state.

| Property | Value |
|----------|-------|
| HTTP Status | 409 |
| Type URI | `https://api.shipaggregator.com/errors/conflict` |
| Title | Conflict |

**Causes:**

- Shipment already has label purchased
- Shipment cannot be cancelled in current status
- Member email already exists
- Role in use by members

**Example:**

```json
{
  "type": "https://api.shipaggregator.com/errors/conflict",
  "title": "Conflict",
  "status": 409,
  "detail": "Cannot cancel shipment in 'delivered' status",
  "code": "CONFLICT",
  "trace_id": "abc123",
  "current_status": "delivered",
  "allowed_statuses": ["draft", "label_purchased"]
}
```

---

#### GONE

Resource existed but is no longer available.

| Property | Value |
|----------|-------|
| HTTP Status | 410 |
| Type URI | `https://api.shipaggregator.com/errors/gone` |
| Title | Gone |

**Causes:**

- Quote expired
- Offer expired
- Payment session expired

**Example:**

```json
{
  "type": "https://api.shipaggregator.com/errors/gone",
  "title": "Gone",
  "status": 410,
  "detail": "Quote expired at 2026-01-22T14:00:00Z. Please create a new quote.",
  "code": "QUOTE_EXPIRED",
  "trace_id": "abc123",
  "expired_at": "2026-01-22T14:00:00Z"
}
```

---

### Idempotency Errors (4xx)

#### IDEMPOTENCY_REPLAYED

Idempotency key was already used with different request body.

| Property | Value |
|----------|-------|
| HTTP Status | 422 |
| Type URI | `https://api.shipaggregator.com/errors/idempotency-replayed` |
| Title | Idempotency Key Conflict |

**Example:**

```json
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

### Business Logic Errors (4xx)

#### INSUFFICIENT_BALANCE

Wallet balance insufficient for operation.

| Property | Value |
|----------|-------|
| HTTP Status | 402 |
| Type URI | `https://api.shipaggregator.com/errors/insufficient-balance` |
| Title | Insufficient Balance |

**Example:**

```json
{
  "type": "https://api.shipaggregator.com/errors/insufficient-balance",
  "title": "Insufficient Balance",
  "status": 402,
  "detail": "Wallet balance of 25.00 TRY is insufficient for charge of 45.50 TRY",
  "code": "INSUFFICIENT_BALANCE",
  "trace_id": "abc123",
  "current_balance": "25.00",
  "required_amount": "45.50",
  "currency": "TRY"
}
```

---

#### COD_NOT_SUPPORTED

COD not supported for the requested route/provider.

| Property | Value |
|----------|-------|
| HTTP Status | 422 |
| Type URI | `https://api.shipaggregator.com/errors/cod-not-supported` |
| Title | COD Not Supported |

**Example:**

```json
{
  "type": "https://api.shipaggregator.com/errors/cod-not-supported",
  "title": "COD Not Supported",
  "status": 422,
  "detail": "Provider 'hepsijet' does not support COD for service 'express'",
  "code": "COD_NOT_SUPPORTED",
  "trace_id": "abc123",
  "provider_code": "hepsijet",
  "service_code": "express"
}
```

---

#### PROVIDER_CAPABILITY_MISSING

Provider lacks required capability.

| Property | Value |
|----------|-------|
| HTTP Status | 422 |
| Type URI | `https://api.shipaggregator.com/errors/provider-capability-missing` |
| Title | Provider Capability Missing |

**Example:**

```json
{
  "type": "https://api.shipaggregator.com/errors/provider-capability-missing",
  "title": "Provider Capability Missing",
  "status": 422,
  "detail": "Provider 'ptt' does not support pickup scheduling",
  "code": "PROVIDER_CAPABILITY_MISSING",
  "trace_id": "abc123",
  "provider_code": "ptt",
  "missing_capability": "pickup_scheduling"
}
```

---

#### PARCEL_EXCEEDS_LIMITS

Parcel dimensions or weight exceed provider limits.

| Property | Value |
|----------|-------|
| HTTP Status | 422 |
| Type URI | `https://api.shipaggregator.com/errors/parcel-exceeds-limits` |
| Title | Parcel Exceeds Limits |

**Example:**

```json
{
  "type": "https://api.shipaggregator.com/errors/parcel-exceeds-limits",
  "title": "Parcel Exceeds Limits",
  "status": 422,
  "detail": "Parcel weight of 35000g exceeds provider limit of 30000g",
  "code": "PARCEL_EXCEEDS_LIMITS",
  "trace_id": "abc123",
  "provider_code": "aras",
  "service_code": "standard",
  "exceeded_limits": [
    {
      "dimension": "weight_grams",
      "value": 35000,
      "limit": 30000
    }
  ]
}
```

---

#### ADDRESS_NOT_SERVICEABLE

Address is not serviceable by any provider.

| Property | Value |
|----------|-------|
| HTTP Status | 422 |
| Type URI | `https://api.shipaggregator.com/errors/address-not-serviceable` |
| Title | Address Not Serviceable |

**Example:**

```json
{
  "type": "https://api.shipaggregator.com/errors/address-not-serviceable",
  "title": "Address Not Serviceable",
  "status": 422,
  "detail": "Destination district '8101' (Artvin/Ardanuç) is not serviceable",
  "code": "ADDRESS_NOT_SERVICEABLE",
  "trace_id": "abc123",
  "address_type": "destination",
  "city_code": "81",
  "district_code": "8101"
}
```

---

### Provider Errors (5xx)

#### PROVIDER_DOWN

Provider API is unavailable.

| Property | Value |
|----------|-------|
| HTTP Status | 502 |
| Type URI | `https://api.shipaggregator.com/errors/provider-down` |
| Title | Provider Unavailable |

**Example:**

```json
{
  "type": "https://api.shipaggregator.com/errors/provider-down",
  "title": "Provider Unavailable",
  "status": 502,
  "detail": "Provider 'aras' is currently unavailable. Please try again later.",
  "code": "PROVIDER_DOWN",
  "trace_id": "abc123",
  "provider_code": "aras",
  "retry_after": 60
}
```

---

#### PROVIDER_REJECTED

Provider rejected the request.

| Property | Value |
|----------|-------|
| HTTP Status | 502 |
| Type URI | `https://api.shipaggregator.com/errors/provider-rejected` |
| Title | Provider Rejected Request |

**Example:**

```json
{
  "type": "https://api.shipaggregator.com/errors/provider-rejected",
  "title": "Provider Rejected Request",
  "status": 502,
  "detail": "Provider 'yurtici' rejected the shipment creation request",
  "code": "PROVIDER_REJECTED",
  "trace_id": "abc123",
  "provider_code": "yurtici",
  "provider_error_code": "INVALID_SENDER_ADDRESS",
  "provider_error_message": "Gönderici adresi geçersiz"
}
```

---

#### PROVIDER_TIMEOUT

Provider API request timed out.

| Property | Value |
|----------|-------|
| HTTP Status | 504 |
| Type URI | `https://api.shipaggregator.com/errors/provider-timeout` |
| Title | Provider Timeout |

**Example:**

```json
{
  "type": "https://api.shipaggregator.com/errors/provider-timeout",
  "title": "Provider Timeout",
  "status": 504,
  "detail": "Request to provider 'mng' timed out after 30 seconds",
  "code": "PROVIDER_TIMEOUT",
  "trace_id": "abc123",
  "provider_code": "mng",
  "timeout_seconds": 30
}
```

---

### Server Errors (5xx)

#### INTERNAL_ERROR

Unexpected server error.

| Property | Value |
|----------|-------|
| HTTP Status | 500 |
| Type URI | `https://api.shipaggregator.com/errors/internal-error` |
| Title | Internal Server Error |

**Example:**

```json
{
  "type": "https://api.shipaggregator.com/errors/internal-error",
  "title": "Internal Server Error",
  "status": 500,
  "detail": "An unexpected error occurred. Please contact support with trace ID.",
  "code": "INTERNAL_ERROR",
  "trace_id": "abc123"
}
```

---

## Error Code Summary

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `AUTH_INVALID` | 401 | Authentication failed |
| `FORBIDDEN` | 403 | Permission denied |
| `RATE_LIMITED` | 429 | Rate limit exceeded |
| `VALIDATION_FAILED` | 422 | Request validation failed |
| `NOT_FOUND` | 404 | Resource not found |
| `CONFLICT` | 409 | State conflict |
| `QUOTE_EXPIRED` | 410 | Quote/offer expired |
| `IDEMPOTENCY_REPLAYED` | 422 | Idempotency key reused |
| `INSUFFICIENT_BALANCE` | 402 | Wallet balance too low |
| `COD_NOT_SUPPORTED` | 422 | COD not available |
| `PROVIDER_CAPABILITY_MISSING` | 422 | Provider lacks capability |
| `PARCEL_EXCEEDS_LIMITS` | 422 | Parcel too large/heavy |
| `ADDRESS_NOT_SERVICEABLE` | 422 | Address not serviceable |
| `PROVIDER_DOWN` | 502 | Provider unavailable |
| `PROVIDER_REJECTED` | 502 | Provider rejected request |
| `PROVIDER_TIMEOUT` | 504 | Provider request timeout |
| `INTERNAL_ERROR` | 500 | Unexpected server error |

---

## Error Handling Best Practices

### For API Consumers

1. **Always check `code` field** for programmatic error handling
2. **Display `detail` to users** for human-readable messages
3. **Log `trace_id`** for support requests
4. **Handle `errors` array** for field-level validation feedback
5. **Respect `Retry-After`** header for rate limiting
6. **Implement exponential backoff** for 5xx errors

### Retry Strategy

| Error Code | Retry? | Strategy |
|------------|--------|----------|
| `AUTH_INVALID` | No | Fix credentials |
| `FORBIDDEN` | No | Check permissions |
| `RATE_LIMITED` | Yes | Wait for `Retry-After` |
| `VALIDATION_FAILED` | No | Fix request |
| `NOT_FOUND` | No | Resource doesn't exist |
| `CONFLICT` | Maybe | Check current state |
| `QUOTE_EXPIRED` | No | Create new quote |
| `IDEMPOTENCY_REPLAYED` | No | Use new key or same request |
| `INSUFFICIENT_BALANCE` | No | Top up wallet |
| `PROVIDER_DOWN` | Yes | Exponential backoff |
| `PROVIDER_REJECTED` | No | Check provider error |
| `PROVIDER_TIMEOUT` | Yes | Exponential backoff |
| `INTERNAL_ERROR` | Yes | Exponential backoff |

### Exponential Backoff

```
delay = min(base_delay * 2^attempt, max_delay) + random_jitter

base_delay = 1 second
max_delay = 60 seconds
max_attempts = 5
jitter = random(0, 1000) milliseconds
```

---

## Localization

Error messages are returned in English by default. Use `Accept-Language` header for localized messages:

```http
Accept-Language: tr-TR
```

Supported languages:

- `en` (default)
- `tr`

**Example (Turkish):**

```json
{
  "type": "https://api.shipaggregator.com/errors/insufficient-balance",
  "title": "Yetersiz Bakiye",
  "status": 402,
  "detail": "25.00 TRY bakiye, 45.50 TRY ücret için yetersiz",
  "code": "INSUFFICIENT_BALANCE",
  "trace_id": "abc123"
}
```

---

## Logging and Monitoring

### Error Logging

All errors are logged with:

- Trace ID
- Request ID
- Timestamp
- Error code
- HTTP status
- Request path
- Request method
- Organization ID (if authenticated)
- Actor ID (if authenticated)
- Stack trace (for 5xx errors)

### Alerting Thresholds

| Error Code | Alert Threshold |
|------------|-----------------|
| `PROVIDER_DOWN` | > 5 in 5 minutes |
| `PROVIDER_TIMEOUT` | > 10 in 5 minutes |
| `INTERNAL_ERROR` | > 1 in 1 minute |
| `RATE_LIMITED` | > 100 in 1 minute (per org) |

---

## Testing Errors

Use these test values to trigger specific errors in sandbox:

| Trigger | Error Code |
|---------|------------|
| Phone: `+905001111111` | `VALIDATION_FAILED` |
| City code: `99` | `ADDRESS_NOT_SERVICEABLE` |
| Weight: `999999` | `PARCEL_EXCEEDS_LIMITS` |
| Provider: `test_down` | `PROVIDER_DOWN` |
| Provider: `test_timeout` | `PROVIDER_TIMEOUT` |
| COD amount: `99999.99` | `COD_NOT_SUPPORTED` |

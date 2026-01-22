# Webhook Events Specification

**Feature Branch**: `001-core-platform`  
**Created**: 2026-01-22  
**Status**: Draft

---

## Overview

This document defines the webhook event types, payload schemas, and delivery semantics for outbound customer webhooks. Events are triggered by system state changes and delivered to customer-registered endpoints.

---

## Event Envelope

All webhook events are wrapped in a standard envelope:

```json
{
  "id": "evt_550e8400-e29b-41d4-a716-446655440000",
  "type": "shipment.delivered",
  "api_version": "2026-01-01",
  "created_at": "2026-01-22T14:30:00Z",
  "organization_id": "org_123e4567-e89b-12d3-a456-426614174000",
  "data": {
    // Event-specific payload
  }
}
```

### Envelope Fields

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique event identifier (prefixed with `evt_`) |
| `type` | string | Event type (e.g., `shipment.delivered`) |
| `api_version` | string | API version for payload schema |
| `created_at` | string (RFC3339) | Event creation timestamp |
| `organization_id` | string | Organization that owns the resource |
| `data` | object | Event-specific payload |

---

## Event Types

### Shipment Events

#### `shipment.created`

Triggered when a new shipment is created.

**Payload:**

```json
{
  "id": "evt_550e8400-e29b-41d4-a716-446655440000",
  "type": "shipment.created",
  "api_version": "2026-01-01",
  "created_at": "2026-01-22T14:30:00Z",
  "organization_id": "org_123e4567-e89b-12d3-a456-426614174000",
  "data": {
    "shipment": {
      "id": "shp_789e0123-e89b-12d3-a456-426614174000",
      "provider_code": "aras",
      "service_code": "standard",
      "status": "draft",
      "origin": {
        "contact_name": "Sender Name",
        "city_code": "34",
        "city_name": "İstanbul",
        "district_code": "3401",
        "district_name": "Kadıköy"
      },
      "destination": {
        "contact_name": "Recipient Name",
        "city_code": "06",
        "city_name": "Ankara",
        "district_code": "0601",
        "district_name": "Çankaya"
      },
      "parcel": {
        "weight_grams": 2500,
        "length_cm": 30,
        "width_cm": 20,
        "height_cm": 15
      },
      "cod_amount": "150.00",
      "is_marketplace": true,
      "created_at": "2026-01-22T14:30:00Z"
    }
  }
}
```

---

#### `shipment.label_purchased`

Triggered when a shipping label is purchased.

**Payload:**

```json
{
  "id": "evt_550e8400-e29b-41d4-a716-446655440001",
  "type": "shipment.label_purchased",
  "api_version": "2026-01-01",
  "created_at": "2026-01-22T14:35:00Z",
  "organization_id": "org_123e4567-e89b-12d3-a456-426614174000",
  "data": {
    "shipment": {
      "id": "shp_789e0123-e89b-12d3-a456-426614174000",
      "provider_code": "aras",
      "service_code": "standard",
      "status": "label_purchased",
      "provider_tracking_number": "1234567890",
      "provider_barcode": "ARAS1234567890",
      "label_document_id": "doc_abc12345-e89b-12d3-a456-426614174000",
      "label_format": "pdf",
      "price_charged": "45.50"
    },
    "charge": {
      "ledger_entry_id": "led_def67890-e89b-12d3-a456-426614174000",
      "amount": "45.50",
      "currency": "TRY",
      "balance_after": "954.50"
    }
  }
}
```

---

#### `shipment.status_changed`

Triggered when shipment status changes (from tracking updates).

**Payload:**

```json
{
  "id": "evt_550e8400-e29b-41d4-a716-446655440002",
  "type": "shipment.status_changed",
  "api_version": "2026-01-01",
  "created_at": "2026-01-22T16:00:00Z",
  "organization_id": "org_123e4567-e89b-12d3-a456-426614174000",
  "data": {
    "shipment": {
      "id": "shp_789e0123-e89b-12d3-a456-426614174000",
      "provider_code": "aras",
      "provider_tracking_number": "1234567890",
      "previous_status": "label_purchased",
      "status": "picked_up"
    },
    "tracking_event": {
      "id": "trk_111e2222-e89b-12d3-a456-426614174000",
      "status_code": "picked_up",
      "status_description": "Shipment picked up from sender",
      "location_city": "İstanbul",
      "location_detail": "Kadıköy Transfer Center",
      "occurred_at": "2026-01-22T15:45:00Z",
      "source": "webhook"
    }
  }
}
```

---

#### `shipment.in_transit`

Triggered when shipment enters transit.

**Payload:**

```json
{
  "id": "evt_550e8400-e29b-41d4-a716-446655440003",
  "type": "shipment.in_transit",
  "api_version": "2026-01-01",
  "created_at": "2026-01-22T18:00:00Z",
  "organization_id": "org_123e4567-e89b-12d3-a456-426614174000",
  "data": {
    "shipment": {
      "id": "shp_789e0123-e89b-12d3-a456-426614174000",
      "provider_code": "aras",
      "provider_tracking_number": "1234567890",
      "status": "in_transit"
    },
    "tracking_event": {
      "id": "trk_222e3333-e89b-12d3-a456-426614174000",
      "status_code": "in_transit",
      "status_description": "Shipment in transit to destination",
      "location_city": "İstanbul",
      "location_detail": "Main Sorting Center",
      "occurred_at": "2026-01-22T17:30:00Z",
      "source": "webhook"
    }
  }
}
```

---

#### `shipment.out_for_delivery`

Triggered when shipment is out for delivery.

**Payload:**

```json
{
  "id": "evt_550e8400-e29b-41d4-a716-446655440004",
  "type": "shipment.out_for_delivery",
  "api_version": "2026-01-01",
  "created_at": "2026-01-23T09:00:00Z",
  "organization_id": "org_123e4567-e89b-12d3-a456-426614174000",
  "data": {
    "shipment": {
      "id": "shp_789e0123-e89b-12d3-a456-426614174000",
      "provider_code": "aras",
      "provider_tracking_number": "1234567890",
      "status": "out_for_delivery"
    },
    "tracking_event": {
      "id": "trk_333e4444-e89b-12d3-a456-426614174000",
      "status_code": "out_for_delivery",
      "status_description": "Shipment out for delivery",
      "location_city": "Ankara",
      "location_detail": "Çankaya Distribution Center",
      "occurred_at": "2026-01-23T08:30:00Z",
      "source": "webhook"
    }
  }
}
```

---

#### `shipment.delivered`

Triggered when shipment is successfully delivered.

**Payload:**

```json
{
  "id": "evt_550e8400-e29b-41d4-a716-446655440005",
  "type": "shipment.delivered",
  "api_version": "2026-01-01",
  "created_at": "2026-01-23T11:30:00Z",
  "organization_id": "org_123e4567-e89b-12d3-a456-426614174000",
  "data": {
    "shipment": {
      "id": "shp_789e0123-e89b-12d3-a456-426614174000",
      "provider_code": "aras",
      "provider_tracking_number": "1234567890",
      "status": "delivered",
      "delivered_at": "2026-01-23T11:15:00Z"
    },
    "tracking_event": {
      "id": "trk_444e5555-e89b-12d3-a456-426614174000",
      "status_code": "delivered",
      "status_description": "Delivered to recipient",
      "location_city": "Ankara",
      "location_detail": "Delivered to: MEHMET YILMAZ",
      "occurred_at": "2026-01-23T11:15:00Z",
      "source": "webhook"
    },
    "delivery": {
      "recipient_name": "MEHMET YILMAZ",
      "signature_captured": true,
      "pod_document_id": "doc_pod12345-e89b-12d3-a456-426614174000"
    }
  }
}
```

---

#### `shipment.delivery_failed`

Triggered when delivery attempt fails.

**Payload:**

```json
{
  "id": "evt_550e8400-e29b-41d4-a716-446655440006",
  "type": "shipment.delivery_failed",
  "api_version": "2026-01-01",
  "created_at": "2026-01-23T12:00:00Z",
  "organization_id": "org_123e4567-e89b-12d3-a456-426614174000",
  "data": {
    "shipment": {
      "id": "shp_789e0123-e89b-12d3-a456-426614174000",
      "provider_code": "aras",
      "provider_tracking_number": "1234567890",
      "status": "delivery_failed"
    },
    "tracking_event": {
      "id": "trk_555e6666-e89b-12d3-a456-426614174000",
      "status_code": "delivery_failed",
      "status_description": "Delivery failed - recipient not available",
      "location_city": "Ankara",
      "location_detail": "Çankaya",
      "occurred_at": "2026-01-23T11:45:00Z",
      "source": "webhook"
    },
    "failure": {
      "reason_code": "recipient_unavailable",
      "reason_description": "Recipient not available at delivery address",
      "attempt_number": 1,
      "next_attempt_scheduled": "2026-01-24T09:00:00Z"
    }
  }
}
```

**Failure Reason Codes:**

| Code | Description |
|------|-------------|
| `recipient_unavailable` | Recipient not available at address |
| `address_not_found` | Delivery address could not be located |
| `refused_by_recipient` | Recipient refused delivery |
| `cod_payment_refused` | Recipient refused COD payment |
| `damaged_package` | Package damaged during transit |
| `incorrect_address` | Address information incorrect |
| `access_restricted` | Cannot access delivery location |
| `other` | Other reason (see description) |

---

#### `shipment.returning`

Triggered when shipment is being returned to sender.

**Payload:**

```json
{
  "id": "evt_550e8400-e29b-41d4-a716-446655440007",
  "type": "shipment.returning",
  "api_version": "2026-01-01",
  "created_at": "2026-01-25T10:00:00Z",
  "organization_id": "org_123e4567-e89b-12d3-a456-426614174000",
  "data": {
    "shipment": {
      "id": "shp_789e0123-e89b-12d3-a456-426614174000",
      "provider_code": "aras",
      "provider_tracking_number": "1234567890",
      "status": "returning"
    },
    "tracking_event": {
      "id": "trk_666e7777-e89b-12d3-a456-426614174000",
      "status_code": "returning",
      "status_description": "Shipment returning to sender",
      "location_city": "Ankara",
      "occurred_at": "2026-01-25T09:30:00Z",
      "source": "webhook"
    },
    "return": {
      "reason": "delivery_failed_max_attempts",
      "return_tracking_number": "1234567890-R"
    }
  }
}
```

---

#### `shipment.returned_to_sender`

Triggered when shipment is returned to sender.

**Payload:**

```json
{
  "id": "evt_550e8400-e29b-41d4-a716-446655440008",
  "type": "shipment.returned_to_sender",
  "api_version": "2026-01-01",
  "created_at": "2026-01-27T14:00:00Z",
  "organization_id": "org_123e4567-e89b-12d3-a456-426614174000",
  "data": {
    "shipment": {
      "id": "shp_789e0123-e89b-12d3-a456-426614174000",
      "provider_code": "aras",
      "provider_tracking_number": "1234567890",
      "status": "returned_to_sender"
    },
    "tracking_event": {
      "id": "trk_777e8888-e89b-12d3-a456-426614174000",
      "status_code": "returned_to_sender",
      "status_description": "Returned to sender",
      "location_city": "İstanbul",
      "location_detail": "Kadıköy",
      "occurred_at": "2026-01-27T13:30:00Z",
      "source": "webhook"
    }
  }
}
```

---

#### `shipment.cancelled`

Triggered when shipment is cancelled.

**Payload:**

```json
{
  "id": "evt_550e8400-e29b-41d4-a716-446655440009",
  "type": "shipment.cancelled",
  "api_version": "2026-01-01",
  "created_at": "2026-01-22T15:00:00Z",
  "organization_id": "org_123e4567-e89b-12d3-a456-426614174000",
  "data": {
    "shipment": {
      "id": "shp_789e0123-e89b-12d3-a456-426614174000",
      "provider_code": "aras",
      "provider_tracking_number": "1234567890",
      "status": "cancelled",
      "cancelled_at": "2026-01-22T15:00:00Z"
    },
    "cancellation": {
      "reason": "Customer requested cancellation",
      "cancelled_by": "member",
      "refund_eligible": true,
      "refund_amount": "45.50"
    }
  }
}
```

---

#### `shipment.exception`

Triggered when an exception occurs during shipment processing.

**Payload:**

```json
{
  "id": "evt_550e8400-e29b-41d4-a716-446655440010",
  "type": "shipment.exception",
  "api_version": "2026-01-01",
  "created_at": "2026-01-24T10:00:00Z",
  "organization_id": "org_123e4567-e89b-12d3-a456-426614174000",
  "data": {
    "shipment": {
      "id": "shp_789e0123-e89b-12d3-a456-426614174000",
      "provider_code": "aras",
      "provider_tracking_number": "1234567890",
      "status": "exception"
    },
    "exception": {
      "code": "customs_hold",
      "description": "Package held at customs for inspection",
      "action_required": true,
      "action_description": "Contact carrier for customs documentation"
    }
  }
}
```

**Exception Codes:**

| Code | Description |
|------|-------------|
| `customs_hold` | Held at customs |
| `security_hold` | Security inspection required |
| `weather_delay` | Weather-related delay |
| `carrier_issue` | Carrier operational issue |
| `lost_package` | Package reported lost |
| `damaged_in_transit` | Package damaged during transit |
| `address_correction_needed` | Address correction required |
| `other` | Other exception |

---

### Billing Events

#### `billing.topup_completed`

Triggered when wallet top-up is completed.

**Payload:**

```json
{
  "id": "evt_550e8400-e29b-41d4-a716-446655440011",
  "type": "billing.topup_completed",
  "api_version": "2026-01-01",
  "created_at": "2026-01-22T10:00:00Z",
  "organization_id": "org_123e4567-e89b-12d3-a456-426614174000",
  "data": {
    "topup": {
      "id": "top_aaa11111-e89b-12d3-a456-426614174000",
      "amount": "1000.00",
      "currency": "TRY",
      "payment_provider": "iyzico",
      "payment_id": "iyz_123456789",
      "completed_at": "2026-01-22T10:00:00Z"
    },
    "wallet": {
      "balance_before": "500.00",
      "balance_after": "1500.00",
      "currency": "TRY"
    },
    "ledger_entry": {
      "id": "led_bbb22222-e89b-12d3-a456-426614174000",
      "entry_type": "credit",
      "amount": "1000.00"
    }
  }
}
```

---

#### `billing.topup_failed`

Triggered when wallet top-up fails.

**Payload:**

```json
{
  "id": "evt_550e8400-e29b-41d4-a716-446655440012",
  "type": "billing.topup_failed",
  "api_version": "2026-01-01",
  "created_at": "2026-01-22T10:05:00Z",
  "organization_id": "org_123e4567-e89b-12d3-a456-426614174000",
  "data": {
    "topup": {
      "id": "top_ccc33333-e89b-12d3-a456-426614174000",
      "amount": "1000.00",
      "currency": "TRY",
      "payment_provider": "iyzico",
      "status": "failed"
    },
    "failure": {
      "reason_code": "card_declined",
      "reason_description": "Card was declined by issuing bank"
    }
  }
}
```

---

#### `billing.balance_low`

Triggered when wallet balance falls below threshold.

**Payload:**

```json
{
  "id": "evt_550e8400-e29b-41d4-a716-446655440013",
  "type": "billing.balance_low",
  "api_version": "2026-01-01",
  "created_at": "2026-01-22T16:00:00Z",
  "organization_id": "org_123e4567-e89b-12d3-a456-426614174000",
  "data": {
    "wallet": {
      "balance": "50.00",
      "currency": "TRY",
      "threshold": "100.00"
    },
    "trigger": {
      "transaction_type": "shipment",
      "transaction_id": "shp_789e0123-e89b-12d3-a456-426614174000",
      "amount_charged": "45.50"
    }
  }
}
```

---

#### `billing.refund_completed`

Triggered when a refund is processed.

**Payload:**

```json
{
  "id": "evt_550e8400-e29b-41d4-a716-446655440014",
  "type": "billing.refund_completed",
  "api_version": "2026-01-01",
  "created_at": "2026-01-23T14:00:00Z",
  "organization_id": "org_123e4567-e89b-12d3-a456-426614174000",
  "data": {
    "refund": {
      "id": "ref_ddd44444-e89b-12d3-a456-426614174000",
      "amount": "45.50",
      "currency": "TRY",
      "reason": "Shipment cancelled before pickup",
      "original_ledger_entry_id": "led_eee55555-e89b-12d3-a456-426614174000"
    },
    "wallet": {
      "balance_before": "954.50",
      "balance_after": "1000.00",
      "currency": "TRY"
    },
    "ledger_entry": {
      "id": "led_fff66666-e89b-12d3-a456-426614174000",
      "entry_type": "credit",
      "amount": "45.50"
    }
  }
}
```

---

#### `billing.cod_settled`

Triggered when COD amount is settled to wallet.

**Payload:**

```json
{
  "id": "evt_550e8400-e29b-41d4-a716-446655440015",
  "type": "billing.cod_settled",
  "api_version": "2026-01-01",
  "created_at": "2026-01-30T10:00:00Z",
  "organization_id": "org_123e4567-e89b-12d3-a456-426614174000",
  "data": {
    "settlement": {
      "id": "cod_ggg77777-e89b-12d3-a456-426614174000",
      "shipment_id": "shp_789e0123-e89b-12d3-a456-426614174000",
      "provider_code": "aras",
      "expected_amount": "150.00",
      "actual_amount": "150.00",
      "carrier_settlement_id": "ARAS-COD-2026-001234",
      "settled_at": "2026-01-30T10:00:00Z"
    },
    "wallet": {
      "balance_before": "1000.00",
      "balance_after": "1150.00",
      "currency": "TRY"
    },
    "ledger_entry": {
      "id": "led_hhh88888-e89b-12d3-a456-426614174000",
      "entry_type": "credit",
      "amount": "150.00"
    }
  }
}
```

---

### Provider Account Events

#### `provider_account.verified`

Triggered when BYOA credentials are verified.

**Payload:**

```json
{
  "id": "evt_550e8400-e29b-41d4-a716-446655440016",
  "type": "provider_account.verified",
  "api_version": "2026-01-01",
  "created_at": "2026-01-22T09:00:00Z",
  "organization_id": "org_123e4567-e89b-12d3-a456-426614174000",
  "data": {
    "provider_account": {
      "id": "pac_iii99999-e89b-12d3-a456-426614174000",
      "provider_code": "aras",
      "name": "My Aras Account",
      "status": "verified",
      "verified_at": "2026-01-22T09:00:00Z"
    }
  }
}
```

---

#### `provider_account.verification_failed`

Triggered when BYOA credential verification fails.

**Payload:**

```json
{
  "id": "evt_550e8400-e29b-41d4-a716-446655440017",
  "type": "provider_account.verification_failed",
  "api_version": "2026-01-01",
  "created_at": "2026-01-22T09:05:00Z",
  "organization_id": "org_123e4567-e89b-12d3-a456-426614174000",
  "data": {
    "provider_account": {
      "id": "pac_jjj00000-e89b-12d3-a456-426614174000",
      "provider_code": "yurtici",
      "name": "My Yurtiçi Account",
      "status": "verification_failed"
    },
    "failure": {
      "reason_code": "invalid_credentials",
      "reason_description": "Username or password is incorrect"
    }
  }
}
```

---

## Event Type Summary

| Event Type | Description | Trigger |
|------------|-------------|---------|
| `shipment.created` | Shipment created | POST /shipments |
| `shipment.label_purchased` | Label purchased | POST /shipments/{id}/purchase-label |
| `shipment.status_changed` | Status changed | Tracking update |
| `shipment.in_transit` | In transit | Tracking update |
| `shipment.out_for_delivery` | Out for delivery | Tracking update |
| `shipment.delivered` | Delivered | Tracking update |
| `shipment.delivery_failed` | Delivery failed | Tracking update |
| `shipment.returning` | Returning to sender | Tracking update |
| `shipment.returned_to_sender` | Returned | Tracking update |
| `shipment.cancelled` | Cancelled | POST /shipments/{id}/cancel |
| `shipment.exception` | Exception occurred | Tracking update |
| `billing.topup_completed` | Top-up completed | Iyzico callback |
| `billing.topup_failed` | Top-up failed | Iyzico callback |
| `billing.balance_low` | Balance below threshold | Wallet debit |
| `billing.refund_completed` | Refund processed | Refund approval |
| `billing.cod_settled` | COD settled | COD reconciliation |
| `provider_account.verified` | BYOA verified | Verification success |
| `provider_account.verification_failed` | BYOA failed | Verification failure |

---

## Webhook Delivery

### Request Format

```http
POST /your-webhook-endpoint HTTP/1.1
Host: your-domain.com
Content-Type: application/json
X-Webhook-Id: evt_550e8400-e29b-41d4-a716-446655440000
X-Webhook-Timestamp: 1706015400
X-Webhook-Signature: v1=5d41402abc4b2a76b9719d911017c592
User-Agent: ShipAggregator-Webhook/1.0

{
  "id": "evt_550e8400-e29b-41d4-a716-446655440000",
  "type": "shipment.delivered",
  ...
}
```

### Headers

| Header | Description |
|--------|-------------|
| `X-Webhook-Id` | Unique event identifier |
| `X-Webhook-Timestamp` | Unix timestamp of delivery attempt |
| `X-Webhook-Signature` | HMAC-SHA256 signature |
| `Content-Type` | Always `application/json` |
| `User-Agent` | `ShipAggregator-Webhook/1.0` |

### Signature Verification

See [security.md](./security.md) for signature verification details.

### Expected Response

- **Success**: HTTP 2xx status code
- **Failure**: Any non-2xx status code triggers retry

### Retry Policy

| Attempt | Delay |
|---------|-------|
| 1 | Immediate |
| 2 | 1 minute |
| 3 | 5 minutes |
| 4 | 30 minutes |
| 5 | 2 hours |
| 6 | 8 hours |
| 7 | 24 hours |

After 7 failed attempts, the delivery is moved to DLQ (Dead Letter Queue).

### Timeout

- Connection timeout: 5 seconds
- Read timeout: 30 seconds

---

## Subscription Management

### Available Event Types for Subscription

```json
[
  "shipment.created",
  "shipment.label_purchased",
  "shipment.status_changed",
  "shipment.in_transit",
  "shipment.out_for_delivery",
  "shipment.delivered",
  "shipment.delivery_failed",
  "shipment.returning",
  "shipment.returned_to_sender",
  "shipment.cancelled",
  "shipment.exception",
  "billing.topup_completed",
  "billing.topup_failed",
  "billing.balance_low",
  "billing.refund_completed",
  "billing.cod_settled",
  "provider_account.verified",
  "provider_account.verification_failed"
]
```

### Wildcard Subscriptions

- `shipment.*` - All shipment events
- `billing.*` - All billing events
- `provider_account.*` - All provider account events
- `*` - All events

---

## Best Practices

### For Consumers

1. **Verify signatures** before processing events
2. **Respond quickly** (within 30 seconds)
3. **Process asynchronously** - acknowledge receipt, process later
4. **Handle duplicates** - events may be delivered more than once
5. **Use idempotency** - track processed event IDs
6. **Monitor DLQ** - replay failed deliveries

### Event Ordering

Events are delivered in approximate order but **not guaranteed** to be in strict order. Use `created_at` timestamps for ordering if needed.

### Idempotency

Each event has a unique `id`. Store processed event IDs to handle duplicate deliveries:

```sql
CREATE TABLE processed_webhook_events (
  event_id VARCHAR(100) PRIMARY KEY,
  processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Before processing
INSERT INTO processed_webhook_events (event_id)
VALUES ('evt_550e8400-e29b-41d4-a716-446655440000')
ON CONFLICT (event_id) DO NOTHING
RETURNING event_id;

-- If no row returned, event was already processed
```

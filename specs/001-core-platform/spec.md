# Feature Specification: TR-Only Shipping Aggregation Backend

**Feature Branch**: `001-core-platform`  
**Created**: 2026-01-22  
**Status**: Draft  
**Input**: User description: "TR-only Shipping Aggregation Backend (Full Product, no UI) supporting Marketplace and BYOA business models with comprehensive Auth, Reference Data, Provider Framework, Quotes, Shipments, Tracking, Webhooks, and Billing/Wallet/Ledger capabilities."

---

## Executive Summary

This specification defines a complete shipping aggregation backend for the Turkish market. The system aggregates multiple cargo carriers (Aras, Yurtiçi, Sürat, PTT, Hepsijet, MNG, Kolaygelsin, Sendeo, Pakettaxi) under a unified API, supporting two business models:

1. **Marketplace Model**: Customers purchase shipping labels at aggregated rates, paying from a prepaid wallet. The system records revenue, cost, and margin in an immutable ledger.

2. **BYOA (Bring Your Own Agreement) Model**: Customers connect their own carrier contracts/credentials. The system routes shipments using customer-owned accounts while optionally charging SaaS usage fees.

The platform provides no user interface—all operations are performed via RESTful API endpoints authenticated via API keys or dashboard sessions.

---

## User Scenarios & Testing

### User Story 1 - Marketplace Shipment Creation (Priority: P1)

A customer with a funded wallet creates a shipment using marketplace rates. They request quotes, select an offer, create a shipment, purchase a label, and track delivery.

**Why this priority**: This is the core revenue-generating flow. Without marketplace shipments, there is no business.

**Independent Test**: Can be fully tested by creating a quote, selecting an offer, creating a shipment, purchasing a label, and verifying the wallet was charged correctly. Delivers immediate value as the primary business transaction.

**Acceptance Scenarios**:

1. **Given** a customer with 500 TRY wallet balance, **When** they POST /quotes with valid origin/destination addresses and parcel details, **Then** they receive a quote_id and can GET /quotes/{quote_id}/offers to see normalized offers from eligible providers with full price breakdowns.

2. **Given** a valid quote with offers, **When** the customer POST /shipments with offer_id and Idempotency-Key, **Then** a shipment is created in `draft` status and the offer price is reserved (not yet charged).

3. **Given** a draft shipment, **When** the customer POST /shipments/{id}/purchase-label with Idempotency-Key, **Then** the label is fetched from the provider, stored as a document, the wallet is charged, a ledger entry is created, and the shipment transitions to `label_purchased` status.

4. **Given** a shipment with purchased label, **When** the customer GET /shipments/{id}/label, **Then** they receive the label document in the requested format (PDF/ZPL/PNG).

5. **Given** insufficient wallet balance (100 TRY) and a shipment costing 150 TRY, **When** the customer POST /shipments/{id}/purchase-label, **Then** they receive a 402 error with code `INSUFFICIENT_BALANCE` and the shipment remains in `draft` status.

---

### User Story 2 - BYOA Shipment Creation (Priority: P1)

A customer with their own carrier contract connects their credentials, verifies the account, and creates shipments using their own rates.

**Why this priority**: BYOA is a key differentiator enabling enterprise customers to use existing carrier relationships while benefiting from the aggregation platform.

**Independent Test**: Can be fully tested by creating a provider account, verifying credentials, creating a shipment, and confirming it routes through the customer's own carrier account.

**Acceptance Scenarios**:

1. **Given** a customer organization, **When** they POST /provider-accounts with provider_code "aras" and their SOAP credentials, **Then** a provider account is created in `pending_verification` status.

2. **Given** a pending provider account, **When** they POST /provider-accounts/{id}/verify, **Then** the system calls the carrier's test endpoint, and on success transitions to `verified` status.

3. **Given** a verified BYOA account for Aras, **When** the customer POST /quotes with use_byoa=true, **Then** offers include rates from their own Aras contract (not marketplace rates).

4. **Given** a BYOA shipment, **When** the label is purchased, **Then** no wallet charge occurs for the shipping cost (only optional SaaS fee if configured), and the shipment is created using the customer's carrier credentials.

5. **Given** invalid BYOA credentials, **When** verification is attempted, **Then** the account transitions to `verification_failed` status with error details.

---

### User Story 3 - COD Shipment with Settlement Reconciliation (Priority: P2)

A customer creates a Cash-on-Delivery shipment. Upon delivery, the carrier collects payment from the recipient. The system tracks COD collection and reconciles settlements.

**Why this priority**: COD is essential for Turkish e-commerce where card penetration is lower. Settlement reconciliation ensures financial accuracy.

**Independent Test**: Can be tested by creating a COD shipment, simulating delivery with COD collection, and verifying settlement tracking entries.

**Acceptance Scenarios**:

1. **Given** a quote request with cod_amount=250.00 TRY, **When** offers are returned, **Then** only providers supporting COD are included, and each offer shows cod_fee in the price breakdown.

2. **Given** a COD shipment delivered successfully, **When** the carrier reports COD collected via tracking webhook, **Then** a COD settlement record is created with status `pending_settlement`.

3. **Given** pending COD settlements, **When** the carrier sends settlement data (daily/weekly), **Then** POST /billing/cod/settlements/reconcile matches settlements to shipments and updates statuses.

4. **Given** a reconciled COD settlement, **When** the settlement amount is credited, **Then** a ledger entry is created crediting the customer's wallet with the COD amount minus any fees.

5. **Given** a COD shipment where recipient refuses payment, **When** the carrier reports delivery failed, **Then** the shipment transitions to `delivery_failed` and no COD settlement is expected.

---

### User Story 4 - Wallet Top-Up via Iyzico (Priority: P2)

A customer tops up their wallet balance using Iyzico payment gateway.

**Why this priority**: Wallet funding is required for marketplace operations. Iyzico is the primary payment processor for Turkish market.

**Independent Test**: Can be tested by initiating a top-up, completing payment via Iyzico sandbox, and verifying wallet balance increase.

**Acceptance Scenarios**:

1. **Given** a customer wanting to add 1000 TRY, **When** they POST /billing/topups with amount and Idempotency-Key, **Then** an Iyzico payment form URL is returned and a topup record is created in `pending` status.

2. **Given** a pending topup, **When** Iyzico sends a success callback/webhook, **Then** POST /billing/topups/{id}/confirm processes the confirmation, creates a ledger credit entry, and updates wallet balance.

3. **Given** a successful topup, **When** the customer GET /billing/balance, **Then** the new balance reflects the credited amount.

4. **Given** a failed Iyzico payment, **When** the failure callback is received, **Then** the topup transitions to `failed` status and no ledger entry is created.

5. **Given** a duplicate topup request with same Idempotency-Key, **When** processed, **Then** the original response is returned without creating a new topup.

---

### User Story 5 - Tracking Updates and Customer Webhooks (Priority: P2)

Carriers send tracking updates. The system normalizes events and notifies customers via webhooks.

**Why this priority**: Real-time tracking is essential for customer experience and operational visibility.

**Independent Test**: Can be tested by receiving a carrier tracking webhook, verifying event normalization, and confirming customer webhook delivery.

**Acceptance Scenarios**:

1. **Given** a shipment in transit, **When** Hepsijet POST /integrations/hepsijet/tracking-webhook with delivery status, **Then** the event is normalized to standard status, stored, and shipment status is updated.

2. **Given** a customer with webhook endpoint subscribed to `shipment.status_changed`, **When** a tracking update changes shipment status, **Then** a signed webhook is sent to their endpoint within 30 seconds.

3. **Given** a webhook delivery failure (5xx response), **When** retry logic executes, **Then** retries occur at 1m, 5m, 30m, 2h, 8h intervals with exponential backoff.

4. **Given** 5 consecutive webhook failures, **When** the final retry fails, **Then** the delivery moves to DLQ and the customer can GET /webhooks/deliveries to see failed deliveries.

5. **Given** a DLQ entry, **When** the customer POST /webhooks/deliveries/{id}/replay, **Then** the webhook is re-sent and delivery attempt is logged.

---

### User Story 6 - API Key Management (Priority: P2)

Customers manage API keys with scopes, rotation, and rate limits.

**Why this priority**: Secure API access is fundamental for all integrations.

**Independent Test**: Can be tested by creating an API key, using it to authenticate, rotating it, and verifying old key is invalidated.

**Acceptance Scenarios**:

1. **Given** an authenticated dashboard session, **When** POST /api-keys with name and scopes, **Then** a new API key is created and the secret is returned (only shown once).

2. **Given** an API key with scope `shipment:read`, **When** used to POST /shipments, **Then** a 403 Forbidden error is returned with code `FORBIDDEN`.

3. **Given** an active API key, **When** POST /api-keys/{id}/rotate, **Then** a new secret is generated, old secret remains valid for 24-hour grace period, then is invalidated.

4. **Given** an API key exceeding rate limit (100 req/min), **When** the 101st request arrives, **Then** a 429 response is returned with Retry-After header.

5. **Given** a deleted API key, **When** used for authentication, **Then** a 401 Unauthorized error is returned with code `AUTH_INVALID`.

---

### User Story 7 - Shipment Cancellation and Refund (Priority: P3)

A customer cancels a shipment before pickup and receives a refund.

**Why this priority**: Cancellation handling is important for customer satisfaction but less frequent than creation flows.

**Independent Test**: Can be tested by creating a shipment, purchasing label, cancelling before pickup, and verifying refund.

**Acceptance Scenarios**:

1. **Given** a shipment in `label_purchased` status not yet picked up, **When** POST /shipments/{id}/cancel, **Then** the carrier cancellation API is called, shipment transitions to `cancelled`, and a refund is initiated.

2. **Given** a successful cancellation, **When** refund is processed, **Then** a ledger entry credits the wallet with the original charge amount (minus any non-refundable fees).

3. **Given** a shipment already in `in_transit` status, **When** cancellation is attempted, **Then** a 409 Conflict error is returned with code `CONFLICT` indicating shipment cannot be cancelled.

4. **Given** a carrier that rejects cancellation, **When** the API returns error, **Then** the shipment remains in current status and error details are returned.

---

### User Story 8 - Return Shipment Creation (Priority: P3)

A customer creates a return shipment for a previously delivered package.

**Why this priority**: Returns are common in e-commerce but secondary to forward logistics.

**Independent Test**: Can be tested by creating a return for an existing shipment and verifying reverse logistics flow.

**Acceptance Scenarios**:

1. **Given** a delivered shipment, **When** POST /shipments/{id}/return with return reason, **Then** a new return shipment is created linked to the original, with origin/destination swapped.

2. **Given** a return shipment, **When** label is purchased, **Then** the return label is generated and wallet is charged for return shipping cost.

3. **Given** a provider that doesn't support returns, **When** return is requested, **Then** a 400 error is returned with code `PROVIDER_CAPABILITY_MISSING`.

---

### User Story 9 - Pickup Scheduling (Priority: P3)

A customer schedules a pickup for shipments ready at their location.

**Why this priority**: Pickup scheduling improves operational efficiency but many customers use drop-off.

**Independent Test**: Can be tested by creating shipments and scheduling a pickup, then verifying pickup confirmation.

**Acceptance Scenarios**:

1. **Given** shipments with purchased labels at same origin, **When** POST /shipments/{id}/pickup with preferred date/time, **Then** a pickup request is sent to the carrier and pickup is scheduled.

2. **Given** a scheduled pickup, **When** the carrier confirms, **Then** shipment status includes pickup_scheduled_at timestamp.

3. **Given** a provider without pickup API, **When** pickup is requested, **Then** a 400 error indicates pickup must be arranged directly with carrier.

---

### User Story 10 - Admin Operations (Priority: P3)

Operations team monitors system health, reviews logs, and handles exceptions.

**Why this priority**: Admin capabilities are essential for operations but not customer-facing.

**Independent Test**: Can be tested by accessing admin endpoints and verifying audit log entries.

**Acceptance Scenarios**:

1. **Given** an admin user, **When** GET /admin/audit-logs with filters, **Then** paginated audit entries are returned showing all system actions.

2. **Given** a provider experiencing issues, **When** GET /admin/health/providers, **Then** health status for each provider is returned including latency and error rates.

3. **Given** failed webhook deliveries in DLQ, **When** POST /admin/webhooks/dlq/reprocess, **Then** all DLQ entries are re-queued for delivery.

4. **Given** a shipment with missing tracking, **When** POST /admin/shipments/{id}/rebuild-tracking, **Then** tracking is re-fetched from carrier and events are reconstructed.

---

### Edge Cases

- **Quote expiry**: Quotes expire after 30 minutes. Attempting to create shipment from expired quote returns 410 Gone with code `QUOTE_EXPIRED`.
- **Provider timeout**: If carrier API times out during label purchase, the operation is retried with same idempotency key. If all retries fail, shipment remains in `draft` and error is returned.
- **Concurrent requests**: Two simultaneous purchase-label requests with same idempotency key result in one success and one returning the cached response.
- **Partial provider outage**: If one provider is down during quote, offers from other providers are still returned. The unavailable provider is marked in response.
- **Address validation failure**: Invalid Turkish addresses (non-existent district/neighborhood codes) are rejected at quote time with detailed validation errors.
- **Cross-tenant access**: Attempting to access another organization's shipment returns 404 (not 403) to prevent information leakage.
- **Webhook endpoint unreachable**: After DLQ, customer is notified via email (if configured) about accumulated failed deliveries.
- **Idempotency key reuse with different payload**: Returns 409 Conflict with code `IDEMPOTENCY_REPLAYED` and original response.

---

## Requirements

### Functional Requirements - Authentication & Access

- **FR-AUTH-001**: System MUST support dashboard authentication via email/password for ops/admin users with session-based tokens.
- **FR-AUTH-002**: System MUST support API key authentication via `Authorization: Bearer <api_key>` header for customer integrations.
- **FR-AUTH-003**: API keys MUST have configurable scopes limiting access to specific operations (e.g., `shipment:create`, `shipment:read`, `quote:create`, `billing:read`).
- **FR-AUTH-004**: API keys MUST support rotation with a 24-hour dual-key grace period where both old and new keys are valid.
- **FR-AUTH-005**: System MUST enforce rate limits per API key (default: 100 requests/minute, configurable per key).
- **FR-AUTH-006**: System MUST support optional IP allowlist per API key, rejecting requests from non-whitelisted IPs.
- **FR-AUTH-007**: API keys MUST have a maximum age of 90 days, after which rotation is required.
- **FR-AUTH-008**: System MUST log all authentication attempts (success/failure) with IP, user agent, and timestamp.

### Functional Requirements - Organizations & Members

- **FR-ORG-001**: Each customer operates within an Organization that owns all resources (shipments, addresses, API keys, etc.).
- **FR-ORG-002**: Organizations MUST have members with assigned roles determining permissions.
- **FR-ORG-003**: System MUST support custom roles with granular permissions (view, create, update, delete per resource type).
- **FR-ORG-004**: Organization data MUST be strictly isolated; queries MUST include tenant context.
- **FR-ORG-005**: System MUST support member invitation, role assignment, and removal.

### Functional Requirements - Reference Data (TR)

- **FR-REF-001**: System MUST provide endpoint to list all Turkish cities with stable city_code identifiers.
- **FR-REF-002**: System MUST provide endpoint to list districts (ilçe) for a given city_code.
- **FR-REF-003**: System MUST provide endpoint to list neighborhoods (mahalle) for a given district_code.
- **FR-REF-004**: System MUST provide endpoint to list postal codes for a given district_code.
- **FR-REF-005**: Reference data MUST be synchronized with carrier reference data to ensure address compatibility.
- **FR-REF-006**: Address validation MUST verify city/district/neighborhood combinations exist and are serviceable.

### Functional Requirements - Addresses

- **FR-ADDR-001**: Customers MUST be able to save addresses for reuse (sender/recipient address book).
- **FR-ADDR-002**: Addresses MUST include: contact_name, company_name (optional), phone (E.164), email (optional), street_line_1, street_line_2 (optional), neighborhood_code, district_code, city_code, postal_code.
- **FR-ADDR-003**: System MUST normalize addresses by validating against reference data and standardizing format.
- **FR-ADDR-004**: System MUST support address soft-delete to preserve historical shipment references.

### Functional Requirements - Provider Framework

- **FR-PROV-001**: System MUST maintain a provider catalog listing all integrated carriers with metadata (name, code, logo_url, website, support_contact).
- **FR-PROV-002**: Each provider MUST have defined services (service levels) with attributes: service_code, name, estimated_days_min, estimated_days_max, supports_cod, supports_insurance, supports_return, max_weight_grams, max_length_cm, max_width_cm, max_height_cm.
- **FR-PROV-003**: System MUST track provider capabilities in a matrix: supports_tracking_webhook, supports_tracking_poll, supports_pickup_scheduling, supports_label_pdf, supports_label_zpl, supports_label_png, supported_regions.
- **FR-PROV-004**: Provider accounts (BYOA) MUST store encrypted credentials specific to each provider's authentication method (SOAP username/password, REST API key, OAuth tokens).
- **FR-PROV-005**: Provider account verification MUST call a test endpoint on the carrier to validate credentials before allowing shipment creation.
- **FR-PROV-006**: System MUST support marketplace rate plans (internal configuration) defining pricing tiers for each provider/service combination.

### Functional Requirements - Quotes & Offers

- **FR-QUOTE-001**: Quote requests MUST include: origin address, destination address, parcel details (weight_grams, length_cm, width_cm, height_cm), optional cod_amount, optional declared_value.
- **FR-QUOTE-002**: System MUST validate addresses against TR reference data before requesting carrier quotes.
- **FR-QUOTE-003**: System MUST fan-out quote requests to all eligible providers based on capability matrix (weight limits, COD support, region coverage).
- **FR-QUOTE-004**: For marketplace quotes, system MUST apply marketplace rate plans to calculate customer-facing prices.
- **FR-QUOTE-005**: For BYOA quotes (use_byoa=true), system MUST request rates using customer's provider account credentials.
- **FR-QUOTE-006**: Offers MUST include normalized price breakdown: base_price, fuel_surcharge, vat_amount, cod_fee (if applicable), other_fees, total_price (all in TRY, string decimal format).
- **FR-QUOTE-007**: Quotes MUST have an expiry time (default: 30 minutes) after which offers are invalid.
- **FR-QUOTE-008**: System MUST support quote refresh (POST /quotes/{id}/refresh) to get updated offers for same parameters.
- **FR-QUOTE-009**: Quote requests MUST require Idempotency-Key header to prevent duplicate quote creation.

### Functional Requirements - Shipments & Labels

- **FR-SHIP-001**: Shipments MAY be created from a quote offer (offer_id) or directly with full shipment details.
- **FR-SHIP-002**: Shipment creation MUST require Idempotency-Key header.
- **FR-SHIP-003**: Shipments MUST follow explicit state machine: `draft` → `label_purchased` → `pickup_scheduled` (optional) → `picked_up` → `in_transit` → `out_for_delivery` → `delivered` | `delivery_failed` | `returned_to_sender`. Also: `draft` → `cancelled`, `label_purchased` → `cancelled`.
- **FR-SHIP-004**: Label purchase (POST /shipments/{id}/purchase-label) MUST: call carrier API to create shipment and get label, store label artifact, charge wallet (marketplace) or record usage (BYOA), create ledger entry, update shipment status.
- **FR-SHIP-005**: Label purchase MUST require Idempotency-Key header.
- **FR-SHIP-006**: Labels MUST be stored as documents with support for multiple formats (PDF, ZPL, PNG) based on carrier capability.
- **FR-SHIP-007**: Shipment cancellation MUST call carrier cancellation API and only succeed if carrier confirms cancellation.
- **FR-SHIP-008**: Shipment updates (PATCH) MUST be limited by status—only certain fields editable in certain states.
- **FR-SHIP-009**: Return shipment creation MUST link to original shipment and swap origin/destination.
- **FR-SHIP-010**: Pickup scheduling MUST send request to carrier and record scheduled pickup time.

### Functional Requirements - Tracking

- **FR-TRACK-001**: System MUST receive tracking webhooks from carriers at provider-specific endpoints (POST /integrations/{provider_code}/tracking-webhook).
- **FR-TRACK-002**: For carriers without webhook support, system MUST poll tracking status via scheduled jobs.
- **FR-TRACK-003**: Tracking events MUST be normalized to standard status codes: `created`, `label_generated`, `picked_up`, `in_transit`, `at_hub`, `out_for_delivery`, `delivered`, `delivery_attempted`, `delivery_failed`, `returning`, `returned_to_sender`, `cancelled`, `exception`.
- **FR-TRACK-004**: Each tracking event MUST include: timestamp, status_code, status_description, location (if available), raw_provider_status.
- **FR-TRACK-005**: Shipment status MUST be deterministically updated based on tracking events (latest event determines status).
- **FR-TRACK-006**: Tracking updates MUST trigger outbound customer webhooks for subscribed events.

### Functional Requirements - Customer Webhooks (Outbound)

- **FR-HOOK-001**: Customers MUST be able to register webhook endpoints with URL and optional secret for signature verification.
- **FR-HOOK-002**: Customers MUST be able to create subscriptions linking endpoints to event types (e.g., `shipment.created`, `shipment.status_changed`, `shipment.delivered`, `billing.topup_completed`).
- **FR-HOOK-003**: Webhook payloads MUST be signed with HMAC-SHA256 using the endpoint's secret, included in `X-Signature-256` header.
- **FR-HOOK-004**: Webhook payloads MUST include timestamp in `X-Timestamp` header; receivers should reject payloads older than 5 minutes (replay protection).
- **FR-HOOK-005**: Failed webhook deliveries (non-2xx response) MUST retry with exponential backoff: 1 minute, 5 minutes, 30 minutes, 2 hours, 8 hours.
- **FR-HOOK-006**: After 5 failed attempts, webhook delivery MUST move to Dead Letter Queue (DLQ).
- **FR-HOOK-007**: Customers MUST be able to view DLQ entries and replay failed deliveries.
- **FR-HOOK-008**: Webhook delivery attempts MUST be logged with request/response metadata (PII masked in logs).
- **FR-HOOK-009**: Customers MUST be able to test webhook endpoints with sample payloads.

### Functional Requirements - Billing / Wallet / Ledger

- **FR-BILL-001**: Each organization MUST have a wallet with balance in TRY.
- **FR-BILL-002**: All financial transactions MUST be recorded as immutable ledger entries with: entry_id, organization_id, entry_type (credit/debit), amount (integer kuruş), currency, reference_type, reference_id, description, created_at, actor_id.
- **FR-BILL-003**: Wallet balance MUST always be computed from ledger entries (sum of credits minus sum of debits), never stored as mutable field.
- **FR-BILL-004**: Marketplace label purchases MUST debit wallet at purchase time (not shipment creation).
- **FR-BILL-005**: Insufficient balance MUST prevent label purchase with clear error message.
- **FR-BILL-006**: Wallet top-ups via Iyzico MUST: create pending topup record, redirect to Iyzico payment form, process callback/webhook on completion, create ledger credit entry.
- **FR-BILL-007**: Top-up requests MUST require Idempotency-Key header.
- **FR-BILL-008**: Refunds MUST create ledger credit entries linked to original debit entry.
- **FR-BILL-009**: COD settlements MUST be tracked with: shipment_id, expected_amount, actual_amount, carrier_settlement_id, status (pending, settled, disputed), settled_at.
- **FR-BILL-010**: COD reconciliation MUST match carrier settlement reports to internal records and flag discrepancies.
- **FR-BILL-011**: System MUST provide transaction history with filtering by date range, type, and reference.

### Functional Requirements - Admin/Ops

- **FR-ADMIN-001**: System MUST maintain audit logs for all state-changing operations with: actor_id, action, resource_type, resource_id, old_value, new_value, timestamp, ip_address.
- **FR-ADMIN-002**: System MUST log all provider API calls with: provider_code, operation, request (masked), response (masked), latency_ms, success, error_code.
- **FR-ADMIN-003**: System MUST provide provider health endpoint showing: availability, average_latency, error_rate, last_successful_call for each provider.
- **FR-ADMIN-004**: System MUST allow admin to reprocess webhook DLQ entries.
- **FR-ADMIN-005**: System MUST allow admin to rebuild tracking for a shipment by re-fetching from carrier.
- **FR-ADMIN-006**: Admin endpoints MUST require elevated permissions (admin role).

### Functional Requirements - Idempotency

- **FR-IDEM-001**: Idempotency-Key header MUST be required for: POST /quotes, POST /shipments, POST /shipments/{id}/purchase-label, POST /billing/topups.
- **FR-IDEM-002**: Idempotency keys MUST be scoped to organization (same key from different orgs are independent).
- **FR-IDEM-003**: Idempotency records MUST be stored for minimum 24 hours.
- **FR-IDEM-004**: Duplicate requests with same idempotency key and same payload MUST return original response without re-execution.
- **FR-IDEM-005**: Requests with same idempotency key but different payload MUST return 409 Conflict with code `IDEMPOTENCY_REPLAYED`.

### Functional Requirements - Error Handling

- **FR-ERR-001**: All error responses MUST use application/problem+json format with: type, title, status, detail, instance, code, trace_id, errors[] (for validation).
- **FR-ERR-002**: System MUST use standard error codes: AUTH_INVALID, FORBIDDEN, RATE_LIMITED, VALIDATION_FAILED, NOT_FOUND, CONFLICT, PROVIDER_DOWN, PROVIDER_REJECTED, INSUFFICIENT_BALANCE, IDEMPOTENCY_REPLAYED, COD_NOT_SUPPORTED, PROVIDER_CAPABILITY_MISSING, QUOTE_EXPIRED.
- **FR-ERR-003**: Validation errors MUST include field-level details in errors[] array with field path and message.
- **FR-ERR-004**: All responses MUST include trace_id for debugging and support correlation.

### Functional Requirements - Pagination

- **FR-PAGE-001**: List endpoints MUST use cursor-based pagination with query parameters: cursor (opaque string), limit (default 20, max 100).
- **FR-PAGE-002**: Paginated responses MUST include: items[] array, next_cursor (null if no more pages).

---

### Key Entities

- **Organization**: Multi-tenant container owning all customer resources. Attributes: id, name, billing_email, created_at, settings.
- **Member**: User belonging to an organization with role. Attributes: id, organization_id, email, name, role_id, status, invited_at, joined_at.
- **Role**: Permission set assignable to members. Attributes: id, organization_id, name, permissions[], is_system (built-in vs custom).
- **ApiKey**: Authentication credential for API access. Attributes: id, organization_id, name, key_hash, scopes[], rate_limit, ip_allowlist[], expires_at, last_used_at, created_at.
- **Address**: Saved address for reuse. Attributes: id, organization_id, label, contact_name, company_name, phone, email, street_line_1, street_line_2, neighborhood_code, district_code, city_code, postal_code, is_default_sender, is_default_recipient.
- **Provider**: Integrated carrier. Attributes: code, name, logo_url, website, support_email, support_phone, is_active.
- **ProviderService**: Service level offered by provider. Attributes: id, provider_code, service_code, name, estimated_days_min, estimated_days_max, supports_cod, supports_insurance, supports_return, max_weight_grams, max_dimensions.
- **ProviderAccount**: BYOA credentials. Attributes: id, organization_id, provider_code, name, credentials_encrypted, status (pending_verification, verified, verification_failed, suspended), verified_at, last_used_at.
- **Quote**: Rate request with offers. Attributes: id, organization_id, origin_address, destination_address, parcel, cod_amount, declared_value, use_byoa, expires_at, created_at.
- **Offer**: Price quote from provider. Attributes: id, quote_id, provider_code, service_code, provider_account_id (for BYOA), price_breakdown, total_price, estimated_days, expires_at.
- **Shipment**: Shipping transaction. Attributes: id, organization_id, offer_id, provider_code, service_code, provider_account_id, provider_shipment_id, provider_tracking_number, origin_address, destination_address, parcel, cod_amount, status, label_document_id, price_charged, created_at, updated_at.
- **TrackingEvent**: Status update from carrier. Attributes: id, shipment_id, provider_code, status_code, status_description, location, occurred_at, raw_data, created_at.
- **Document**: Stored file (labels, POD). Attributes: id, organization_id, type (label, pod, invoice), format (pdf, zpl, png), storage_path, content_hash, created_at.
- **WebhookEndpoint**: Customer's webhook URL. Attributes: id, organization_id, url, secret_hash, is_active, created_at.
- **WebhookSubscription**: Event subscription. Attributes: id, endpoint_id, event_type, is_active, created_at.
- **WebhookDelivery**: Delivery attempt record. Attributes: id, subscription_id, event_id, payload, status (pending, delivered, failed, dlq), attempts, last_attempt_at, next_retry_at, response_status, response_body_preview.
- **LedgerEntry**: Immutable financial record. Attributes: id, organization_id, entry_type (credit, debit), amount_kurus, currency, reference_type, reference_id, description, created_at, actor_id.
- **Topup**: Wallet funding request. Attributes: id, organization_id, amount_kurus, status (pending, completed, failed, cancelled), iyzico_payment_id, idempotency_key, created_at, completed_at.
- **CodSettlement**: COD collection tracking. Attributes: id, shipment_id, expected_amount_kurus, actual_amount_kurus, carrier_settlement_id, status (pending, settled, disputed), settled_at.

---

## Success Criteria

### Measurable Outcomes

- **SC-001**: Customers can complete the full shipment flow (quote → create → purchase label → track) in under 5 minutes for any supported carrier.
- **SC-002**: System supports 10,000 concurrent API requests with p99 latency under 500ms for non-carrier-dependent operations.
- **SC-003**: Carrier API calls complete within carrier SLA (typically 5-30 seconds) with automatic retry on transient failures.
- **SC-004**: 99.9% of webhook deliveries succeed within 1 hour of event occurrence (excluding customer endpoint issues).
- **SC-005**: Financial reconciliation achieves 100% accuracy—ledger balance always matches sum of transactions.
- **SC-006**: Zero cross-tenant data leakage verified by automated security tests.
- **SC-007**: All carrier integrations pass contract tests verifying correct request/response mapping.
- **SC-008**: System handles carrier outages gracefully, returning partial results and clear error messages.
- **SC-009**: Idempotency prevents 100% of duplicate charges and shipment creations in retry scenarios.
- **SC-010**: Tracking updates propagate to customer webhooks within 60 seconds of carrier notification.

---

## Assumptions

1. **Currency**: All monetary values are in Turkish Lira (TRY). No multi-currency support required initially.
2. **Region**: Service is limited to Turkey (TR). International shipping is out of scope.
3. **Carriers**: Initial carrier set is Aras, Yurtiçi, Sürat, PTT, Hepsijet, MNG, Kolaygelsin, Sendeo, Pakettaxi. Additional carriers can be added via the provider adapter pattern.
4. **Authentication**: Dashboard auth uses session cookies; API auth uses Bearer tokens. No OAuth/SSO required initially.
5. **Payment**: Iyzico is the sole payment processor for wallet top-ups. Bank transfer reconciliation is manual/out of scope.
6. **Label Formats**: PDF is universally supported; ZPL/PNG depend on carrier capability.
7. **Tracking Webhooks**: Not all carriers support push notifications; polling is the fallback.
8. **Rate Limits**: Default 100 req/min per API key is sufficient for most integrations; enterprise customers can request increases.
9. **Data Retention**: Per constitution—shipment data 7 years, payment records 10 years, logs 30-90 days.
10. **PII Handling**: Phone numbers stored in E.164 format; addresses stored with reference codes; logs mask PII per constitution.

---

## Out of Scope

1. User interface (web/mobile dashboard)
2. International shipping
3. Multi-currency support
4. OAuth/SSO authentication
5. Bank transfer payment processing
6. Carrier contract negotiation
7. Physical logistics operations
8. Customer support ticketing
9. Analytics/reporting dashboards
10. Mobile SDKs

---

## Dependencies

1. **Carrier APIs**: Availability and stability of Aras, Yurtiçi, Sürat, PTT, Hepsijet, MNG, Kolaygelsin, Sendeo, Pakettaxi APIs.
2. **Iyzico**: Payment gateway availability for wallet top-ups.
3. **Turkish Address Database**: Accurate and up-to-date city/district/neighborhood reference data.
4. **Infrastructure**: Database, cache, message queue, object storage for labels.

---

## Risks

1. **Carrier API Changes**: Carriers may change APIs without notice. Mitigation: Version adapters, monitor for breaking changes, maintain sandbox environments.
2. **Carrier Outages**: Individual carriers may experience downtime. Mitigation: Circuit breakers, graceful degradation, multi-carrier redundancy.
3. **Rate Limit Exhaustion**: High-volume customers may hit carrier rate limits. Mitigation: Request queuing, rate limit monitoring, carrier relationship management.
4. **COD Settlement Delays**: Carriers may delay COD settlements. Mitigation: Reconciliation alerts, settlement tracking, carrier SLA monitoring.
5. **Regulatory Changes**: Turkish shipping/payment regulations may change. Mitigation: Legal monitoring, flexible configuration.

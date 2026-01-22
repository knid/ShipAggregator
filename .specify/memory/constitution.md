<!--
Sync Impact Report
==================
Version change: 0.0.0 → 1.0.0 (initial ratification)
Modified principles: N/A (initial version)
Added sections: 16 Articles, Governance, Compliance & Retention
Removed sections: N/A
Templates requiring updates: ⚠ pending (no dependent templates exist yet)
Follow-up TODOs: None
-->

# ShipAggregator Constitution

A TR-only shipping aggregation backend supporting Marketplace and BYOA (Bring Your Own Agreement) models with COD and iyzico payment integration.

## Core Principles

### Article I. Multi-Tenant Isolation

**Rule**: Every tenant's data, credentials, and operations MUST be strictly isolated. Cross-tenant data access is forbidden. All database queries MUST include tenant context. Tenant identifiers MUST be validated at the API gateway before any downstream processing.

**Rationale**: Shipping aggregation handles sensitive business data including carrier credentials, shipment details, and financial transactions. A single tenant leak could expose competitor pricing, customer addresses, or payment information. Isolation is the foundation of trust in a multi-tenant SaaS.

**Enforcement Checks**:

- [ ] Unit tests verify all repository methods require tenant_id parameter
- [ ] Integration tests attempt cross-tenant access and assert 403/404 responses
- [ ] PR check: static analysis flags any raw SQL without tenant predicate
- [ ] Database-level RLS (Row Level Security) policies enforced where supported
- [ ] Audit log review for any tenant_id mismatch warnings

### Article II. Least Privilege Access Control

**Rule**: Service accounts, API keys, and internal components MUST operate with minimum required permissions. No component may request or receive broader access than its function demands. Admin operations require explicit elevation with audit trail.

**Rationale**: Blast radius containment. If a component is compromised, damage is limited to its permission scope. Prevents accidental data modification by services that only need read access.

**Enforcement Checks**:

- [ ] IAM policy review in PR for any new service account
- [ ] Integration tests verify service A cannot call service B's admin endpoints
- [ ] Quarterly access audit comparing actual usage vs granted permissions
- [ ] CI check: new permission grants require justification comment

### Article III. API Key Scopes and Rotation

**Rule**: All API keys MUST have explicit scopes (e.g., `shipment:create`, `label:read`, `billing:admin`). Keys MUST support rotation without downtime (dual-key window). Maximum key age is 90 days for production, 30 days for sandbox. Rate limits MUST be enforced per key, per tenant, and globally.

**Rationale**: Scoped keys limit damage from leaks. Rotation ensures compromised keys have limited lifetime. Rate limiting prevents abuse and ensures fair resource allocation across tenants.

**Enforcement Checks**:

- [ ] Unit tests verify scope enforcement on all protected endpoints
- [ ] Integration tests confirm rate limit responses (429) with correct headers
- [ ] Automated alerts for keys approaching expiration
- [ ] PR check: new endpoints must declare required scopes in OpenAPI spec
- [ ] Load tests validate rate limiter behavior under burst traffic

### Article IV. Idempotency for Critical Operations

**Rule**: All create, purchase, and billing operations MUST be idempotent. Clients MUST provide an `Idempotency-Key` header. The system MUST store idempotency records for minimum 24 hours. Duplicate requests within the window MUST return the original response without re-execution.

**Rationale**: Network failures, retries, and client bugs can cause duplicate requests. Without idempotency, a shipment could be purchased twice, a customer billed multiple times, or inventory double-decremented. Idempotency is non-negotiable for financial correctness.

**Enforcement Checks**:

- [ ] Unit tests verify idempotency key storage and retrieval
- [ ] Integration tests send duplicate requests and assert identical responses
- [ ] PR check: new mutating endpoints must implement IdempotencyGuard middleware
- [ ] Chaos tests: inject network failures and verify no duplicate side effects
- [ ] Monitoring alert for idempotency key collisions from different payloads

### Article V. Money Correctness and Immutable Ledger

**Rule**: All monetary values MUST be stored as integer minor units (kuruş for TRY). No floating-point arithmetic for money. All financial state changes MUST be recorded as immutable ledger entries (append-only). Balance is always computed from ledger, never stored as mutable field. Ledger entries MUST include: amount, currency, operation_type, reference_id, timestamp, actor_id.

**Rationale**: Floating-point errors accumulate and cause reconciliation nightmares. Mutable balances hide the audit trail. An immutable ledger provides complete financial history, enables point-in-time reconstruction, and satisfies audit requirements.

**Enforcement Checks**:

- [ ] Static analysis: flag any `float`/`double` type in money-related code paths
- [ ] Unit tests verify ledger append-only behavior (no UPDATE/DELETE)
- [ ] Integration tests reconcile computed balance against expected after N operations
- [ ] PR check: money-handling code requires finance team review label
- [ ] Daily reconciliation job comparing ledger totals to external payment provider

### Article VI. Strict Financial Invariants

**Rule**: The system MUST enforce: (1) No negative balances without explicit credit facility, (2) Sum of all debits equals sum of all credits per tenant, (3) Every charge has a corresponding shipment or service reference, (4) Refunds cannot exceed original charge amount, (5) Currency conversion rates are locked at transaction time and stored with the entry.

**Rationale**: Financial invariants catch bugs before they become accounting problems. Violations indicate either bugs or fraud attempts. Early detection prevents costly manual reconciliation.

**Enforcement Checks**:

- [ ] Database constraints enforce non-negative balances (where applicable)
- [ ] Nightly invariant checker job with PagerDuty alert on violation
- [ ] Unit tests for each invariant with boundary conditions
- [ ] Integration tests attempt invariant violations and assert rejection
- [ ] Quarterly audit samples random transactions for invariant compliance

### Article VII. Reconciliation Pipeline

**Rule**: Automated reconciliation MUST run daily comparing: (1) Internal ledger vs payment provider (iyzico) settlements, (2) Internal shipment costs vs carrier invoices, (3) Expected COD collections vs actual deposits. Discrepancies exceeding 1 TRY MUST generate alerts. Unresolved discrepancies MUST block affected tenant billing.

**Rationale**: External systems (carriers, payment providers) are sources of truth for actual money movement. Reconciliation catches integration bugs, carrier billing errors, and fraud. Blocking billing on discrepancies prevents compounding errors.

**Enforcement Checks**:

- [ ] Integration tests with mock provider data verify reconciliation logic
- [ ] Staging environment runs reconciliation against sandbox providers
- [ ] Alert thresholds configured and tested with synthetic discrepancies
- [ ] Runbook exists for each discrepancy type with resolution steps
- [ ] Monthly review of reconciliation metrics and false positive rate

### Article VIII. Outbox Pattern for Event Delivery

**Rule**: All domain events MUST be written to an outbox table within the same database transaction as the state change. A separate publisher process reads the outbox and dispatches to the message broker. Events MUST NOT be published directly from application code. Outbox entries include: event_type, payload, created_at, published_at, retry_count.

**Rationale**: Dual-write problem: if we update DB then publish event, a crash between them loses the event. If we publish then update, a crash causes phantom events. Outbox pattern guarantees exactly-once semantics between DB state and event publication.

**Enforcement Checks**:

- [ ] Static analysis: flag any direct message broker publish calls outside outbox publisher
- [ ] Unit tests verify event written to outbox within transaction boundary
- [ ] Integration tests: kill publisher mid-batch, verify no lost events on restart
- [ ] PR check: new domain events must have outbox integration
- [ ] Monitoring: outbox lag alert if unpublished events exceed 5 minutes

### Article IX. Webhook Delivery Guarantees

**Rule**: Webhooks MUST be signed with HMAC-SHA256 using per-tenant secrets. Failed deliveries MUST retry with exponential backoff (1m, 5m, 30m, 2h, 8h). After 5 failures, events move to Dead Letter Queue (DLQ). Tenants MUST have access to DLQ contents and manual replay. Webhook payloads MUST include event_id, event_type, timestamp, and idempotency-safe structure.

**Rationale**: Webhooks are the primary integration mechanism for tenant systems. Unreliable webhooks break downstream automation. HMAC prevents spoofing. DLQ with replay ensures no event is permanently lost due to temporary tenant endpoint issues.

**Enforcement Checks**:

- [ ] Unit tests verify HMAC signature generation and validation
- [ ] Integration tests simulate endpoint failures and verify retry schedule
- [ ] Integration tests verify DLQ insertion after max retries
- [ ] Load tests confirm webhook throughput under high event volume
- [ ] Tenant-facing API for DLQ listing and replay with integration tests

### Article X. Provider Abstraction Layer

**Rule**: All carrier API calls MUST go through a ProviderAdapter interface. Direct HTTP calls to carrier endpoints are forbidden outside adapter implementations. Each adapter MUST implement the full capability matrix or explicitly declare unsupported operations. Adapter selection is driven by capability requirements, not hardcoded carrier names.

**Rationale**: Carriers have wildly different APIs (REST, SOAP, FTP). Abstraction enables: (1) Adding new carriers without core changes, (2) Consistent error handling and retry logic, (3) Capability-based routing for optimal carrier selection, (4) Isolated testing with mock adapters.

**Enforcement Checks**:

- [ ] Static analysis: flag HTTP client usage outside adapter package
- [ ] Unit tests for each adapter method with recorded responses
- [ ] Integration tests against carrier sandbox environments (where available)
- [ ] PR check: new carrier requires full adapter implementation + capability declaration
- [ ] Capability matrix documentation auto-generated from adapter metadata

### Article XI. Provider Capability Matrix

**Rule**: Each carrier adapter MUST declare capabilities: supports_cod, supports_return, supports_multi_parcel, supports_insurance, max_weight_kg, max_dimensions_cm, supported_regions, label_formats, tracking_webhook_available, estimated_delivery_sla. The routing engine MUST use this matrix for carrier selection. Capabilities MUST be versioned and updated when carrier APIs change.

**Rationale**: Not all carriers support all features. Attempting unsupported operations wastes API calls and creates poor UX. Capability matrix enables intelligent routing and clear error messages when no carrier matches requirements.

**Enforcement Checks**:

- [ ] Schema validation for capability declarations (all required fields present)
- [ ] Unit tests verify routing engine respects capability constraints
- [ ] Integration tests confirm declared capabilities match actual carrier behavior
- [ ] Automated capability drift detection comparing declared vs observed
- [ ] Documentation generated from capability matrix for tenant API docs

### Article XII. Observability and Tracing

**Rule**: Every inbound request MUST have a trace_id (generated if not provided). trace_id MUST propagate to all downstream calls (DB, cache, carrier APIs, internal services). All log entries MUST include trace_id, tenant_id, and operation context. Metrics MUST be emitted for: request latency (p50, p95, p99), error rates by type, carrier API latency, queue depths.

**Rationale**: Distributed systems fail in distributed ways. Without tracing, debugging production issues requires correlating timestamps across services—slow and error-prone. Metrics enable SLO tracking and capacity planning.

**Enforcement Checks**:

- [ ] Middleware automatically injects trace_id; unit tests verify propagation
- [ ] Integration tests verify trace_id appears in logs across service boundaries
- [ ] PR check: new log statements must use structured logger with context
- [ ] Alerting configured for p99 latency and error rate thresholds
- [ ] Dashboard review in sprint retrospectives

### Article XIII. PII Masking and Data Minimization

**Rule**: Logs MUST NOT contain: full addresses, phone numbers, email addresses, payment details, or carrier credentials. PII in logs MUST be masked (e.g., `+90532***1234`, `a]***@example.com`). Debug-level logs with full PII are permitted only in development environment with explicit flag. Production log retention: 30 days for application logs, 90 days for audit logs.

**Rationale**: Logs are often accessed by broader teams and may be exported to third-party observability platforms. PII in logs creates KVKK compliance risk and potential data breach surface. Masking preserves debuggability while protecting privacy.

**Enforcement Checks**:

- [ ] Unit tests verify masking functions for all PII types
- [ ] Integration tests grep production-like logs for PII patterns and fail if found
- [ ] PR check: new log statements reviewed for PII exposure
- [ ] Log aggregator configured with PII detection alerts
- [ ] Quarterly log audit sampling for compliance verification

### Article XIV. KVKK Compliance Foundations

**Rule**: Personal data processing MUST have documented legal basis. Data subject access requests MUST be fulfillable within 30 days. Right to deletion MUST be implementable (soft delete with retention, then hard delete). Cross-border data transfer MUST be documented and compliant. Consent records MUST be immutable and timestamped.

**Rationale**: KVKK (Turkish GDPR equivalent) carries significant penalties for non-compliance. Building compliance into the architecture is cheaper than retrofitting. Access request fulfillment requires knowing where all PII lives.

**Enforcement Checks**:

- [ ] Data inventory document listing all PII fields and their legal basis
- [ ] Integration tests verify data subject export endpoint returns all PII
- [ ] Integration tests verify deletion cascade across all PII storage
- [ ] PR check: new PII fields require data inventory update
- [ ] Annual KVKK compliance review with legal team

### Article XV. Retention and Access Logging

**Rule**: All data types MUST have defined retention periods: shipment data (7 years for tax), payment records (10 years), application logs (30 days), audit logs (2 years), deleted PII (30 days soft delete, then purge). All access to PII MUST be logged with: accessor_id, access_type, timestamp, data_category. Retention enforcement MUST be automated.

**Rationale**: Retention requirements come from tax law, KVKK, and business needs. Over-retention increases breach impact and storage costs. Under-retention causes legal and audit issues. Access logging enables breach investigation and deters insider threats.

**Enforcement Checks**:

- [ ] Automated retention job with unit tests for each data type
- [ ] Integration tests verify data is purged after retention period
- [ ] Access log middleware with unit tests for PII endpoints
- [ ] Monitoring for retention job failures
- [ ] Quarterly retention compliance report

### Article XVI. Full Specification Coverage

**Rule**: No "MVP shortcuts" are permitted. Every carrier integration MUST implement the full adapter interface or explicitly mark operations as unsupported with clear error messages. Every API endpoint MUST have: OpenAPI documentation, request/response validation, error code documentation, integration tests, and load test coverage. Test harness MUST cover: happy path, validation errors, carrier errors, timeout handling, and idempotency.

**Rationale**: Shortcuts create technical debt that compounds. Undocumented behavior becomes de facto API contract. Missing tests allow regressions. A shipping aggregator's value is reliability—partial implementations erode trust.

**Enforcement Checks**:

- [ ] PR check: new endpoints require OpenAPI spec update
- [ ] PR check: new endpoints require integration test coverage
- [ ] CI gate: OpenAPI spec validation (no undocumented endpoints)
- [ ] Coverage threshold: 80% line coverage, 100% endpoint coverage
- [ ] Carrier integration checklist template enforced for new adapters

## Compliance and Retention Summary

| Data Category | Retention Period | Legal Basis |
|---------------|------------------|-------------|
| Shipment records | 7 years | Tax law (VUK) |
| Payment/billing records | 10 years | Commercial code |
| Carrier credentials | Until revoked + 90 days | Contract performance |
| Application logs | 30 days | Legitimate interest |
| Audit logs | 2 years | KVKK compliance |
| Soft-deleted PII | 30 days | Deletion verification |
| Webhook delivery logs | 90 days | Debugging/support |

## Governance

### Amendment Process

1. Proposed amendments MUST be documented in a PR with rationale
2. Amendments affecting security or compliance require security team review
3. Amendments affecting financial operations require finance team review
4. All amendments require at least two approvals from senior engineers
5. Breaking changes to existing articles require migration plan

### Version Policy

- **MAJOR**: Removal or fundamental redefinition of an article
- **MINOR**: New article added or significant expansion of existing article
- **PATCH**: Clarifications, typo fixes, enforcement check updates

### Compliance Review

- All PRs MUST include constitution compliance checklist
- Quarterly review of enforcement check effectiveness
- Annual full constitution review with stakeholders
- Violations logged and reviewed in sprint retrospectives

### Runtime Guidance

For day-to-day development decisions not covered by this constitution, refer to the project's technical decision records (TDRs) and API design guidelines in the documentation.

**Version**: 1.0.0 | **Ratified**: 2026-01-22 | **Last Amended**: 2026-01-22

# Tasks: TR-Only Shipping Aggregation Backend

**Input**: Design documents from `/specs/001-core-platform/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

**Tests**: Tests are NOT included unless explicitly requested.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Single project**: `cmd/`, `internal/`, `pkg/`, `migrations/` at repository root
- Paths follow the structure defined in plan.md

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [x] T001 Initialize Go module with `go mod init` and create go.mod at repository root
- [x] T002 Create project directory structure per plan.md (cmd/, internal/, pkg/, migrations/, tests/)
- [x] T003 [P] Create Makefile with targets: dev, run-api, run-worker, migrate-up, migrate-down, seed, generate, sqlc, openapi, test, test-unit, test-integration, fmt, lint, build
- [x] T004 [P] Create docker-compose.yml with PostgreSQL 15, Redis 7, MinIO services
- [x] T005 [P] Create .env.example with all required environment variables
- [x] T006 [P] Create .air.toml for hot reload configuration
- [x] T007 [P] Create .golangci.yml for linter configuration
- [x] T008 [P] Create Dockerfile for production build (multi-stage)
- [x] T009 Add primary dependencies to go.mod: chi, pgx, sqlc, asynq, go-redis, oapi-codegen, testify

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

### Database Schema & Migrations

- [x] T010 Create migrations/000001_initial_schema.up.sql with organizations, members, roles, sessions tables
- [x] T011 Create migrations/000001_initial_schema.down.sql with rollback for initial schema
- [x] T012 Create migrations/000002_api_keys.up.sql with api_keys table and indexes
- [x] T013 Create migrations/000002_api_keys.down.sql with rollback
- [x] T014 Create migrations/000003_reference_data.up.sql with ref_cities, ref_districts, ref_neighborhoods, ref_postal_codes tables
- [x] T015 Create migrations/000003_reference_data.down.sql with rollback
- [x] T016 Create migrations/000004_addresses.up.sql with addresses table and RLS policies
- [x] T017 Create migrations/000004_addresses.down.sql with rollback
- [x] T018 Create migrations/000005_providers.up.sql with providers, provider_services, provider_accounts, marketplace_rate_plans tables
- [x] T019 Create migrations/000005_providers.down.sql with rollback
- [x] T020 Create migrations/000006_quotes.up.sql with quotes, offers tables
- [x] T021 Create migrations/000006_quotes.down.sql with rollback
- [x] T022 Create migrations/000007_shipments.up.sql with shipments, tracking_events, documents tables
- [x] T023 Create migrations/000007_shipments.down.sql with rollback
- [x] T024 Create migrations/000008_webhooks.up.sql with webhook_endpoints, webhook_subscriptions, webhook_events, webhook_deliveries tables
- [x] T025 Create migrations/000008_webhooks.down.sql with rollback
- [x] T026 Create migrations/000009_billing.up.sql with ledger_entries, topups, refunds, cod_settlements tables
- [x] T027 Create migrations/000009_billing.down.sql with rollback
- [x] T028 Create migrations/000010_infrastructure.up.sql with idempotency_records, audit_logs, provider_api_logs, outbox tables
- [x] T029 Create migrations/000010_infrastructure.down.sql with rollback
- [x] T030 Create migrations/000011_triggers.up.sql with updated_at, ledger_immutability, audit_log triggers
- [x] T031 Create migrations/000011_triggers.down.sql with rollback
- [x] T032 Create migrations/000012_views.up.sql with v_wallet_balances, v_provider_health views
- [x] T033 Create migrations/000012_views.down.sql with rollback
- [x] T034 Create migrations/000013_seed_providers.up.sql with provider seed data (aras, yurtici, surat, ptt, hepsijet, mng, kolaygelsin, sendeo, pakettaxi)
- [x] T035 Create migrations/000013_seed_providers.down.sql with rollback
- [x] T036 Create migrations/000014_seed_roles.up.sql with system roles (owner, admin, operator, viewer)
- [x] T037 Create migrations/000014_seed_roles.down.sql with rollback

### Configuration & Utilities

- [ ] T038 Implement internal/config/config.go with environment variable loading and validation
- [ ] T039 [P] Implement pkg/money/money.go with integer kuruş type and arithmetic operations
- [ ] T040 [P] Implement pkg/phone/phone.go with E.164 Turkish phone validation
- [ ] T041 [P] Implement pkg/address/turkish.go with Turkish address validation utilities

### Database Layer

- [ ] T042 Create sqlc.yaml configuration file at repository root
- [ ] T043 Create internal/repository/postgres/queries/organizations.sql with CRUD queries
- [ ] T044 Create internal/repository/postgres/queries/members.sql with CRUD queries
- [ ] T045 Create internal/repository/postgres/queries/roles.sql with CRUD queries
- [ ] T046 Create internal/repository/postgres/queries/sessions.sql with CRUD queries
- [ ] T047 Create internal/repository/postgres/queries/api_keys.sql with CRUD queries
- [ ] T048 Create internal/repository/postgres/queries/addresses.sql with CRUD queries
- [ ] T049 Create internal/repository/postgres/queries/providers.sql with read queries
- [ ] T050 Create internal/repository/postgres/queries/provider_accounts.sql with CRUD queries
- [ ] T051 Create internal/repository/postgres/queries/quotes.sql with CRUD queries
- [ ] T052 Create internal/repository/postgres/queries/offers.sql with CRUD queries
- [ ] T053 Create internal/repository/postgres/queries/shipments.sql with CRUD queries
- [ ] T054 Create internal/repository/postgres/queries/tracking_events.sql with CRUD queries
- [ ] T055 Create internal/repository/postgres/queries/documents.sql with CRUD queries
- [ ] T056 Create internal/repository/postgres/queries/webhook_endpoints.sql with CRUD queries
- [ ] T057 Create internal/repository/postgres/queries/webhook_subscriptions.sql with CRUD queries
- [ ] T058 Create internal/repository/postgres/queries/webhook_deliveries.sql with CRUD queries
- [ ] T059 Create internal/repository/postgres/queries/ledger_entries.sql with insert and read queries
- [ ] T060 Create internal/repository/postgres/queries/topups.sql with CRUD queries
- [ ] T061 Create internal/repository/postgres/queries/cod_settlements.sql with CRUD queries
- [ ] T062 Create internal/repository/postgres/queries/idempotency.sql with CRUD queries
- [ ] T063 Create internal/repository/postgres/queries/audit_logs.sql with insert and read queries
- [ ] T064 Create internal/repository/postgres/queries/outbox.sql with CRUD queries
- [ ] T065 Create internal/repository/postgres/queries/reference_data.sql with read queries for cities, districts, neighborhoods, postal_codes
- [ ] T066 Run sqlc generate to create Go code from SQL queries

### Domain Models

- [ ] T067 [P] Implement internal/domain/organization.go with Organization, OrganizationStatus types
- [ ] T068 [P] Implement internal/domain/member.go with Member, MemberStatus types
- [ ] T069 [P] Implement internal/domain/apikey.go with ApiKey, ApiKeyScope types
- [ ] T070 [P] Implement internal/domain/address.go with Address type
- [ ] T071 [P] Implement internal/domain/provider.go with Provider, ProviderService, ProviderAccount types
- [ ] T072 [P] Implement internal/domain/quote.go with Quote, Offer, PriceBreakdown types
- [ ] T073 [P] Implement internal/domain/shipment.go with Shipment, ShipmentStatus, Parcel types
- [ ] T074 [P] Implement internal/domain/tracking.go with TrackingEvent, TrackingStatus types
- [ ] T075 [P] Implement internal/domain/webhook.go with WebhookEndpoint, WebhookSubscription, WebhookDelivery types
- [ ] T076 [P] Implement internal/domain/ledger.go with LedgerEntry, EntryType types
- [ ] T077 [P] Implement internal/domain/topup.go with Topup, TopupStatus types
- [ ] T078 [P] Implement internal/domain/errors.go with domain error types (ErrNotFound, ErrInsufficientBalance, etc.)

### Crypto & Security

- [ ] T079 Implement internal/crypto/aes.go with AES-256-GCM encryption/decryption for credentials
- [ ] T080 Implement internal/crypto/hmac.go with HMAC-SHA256 signing for webhooks

### Observability

- [ ] T081 Implement internal/observability/logger.go with structured slog logger and PII masking
- [ ] T082 Implement internal/observability/metrics.go with Prometheus metrics (http_requests_total, http_request_duration, provider_api_latency)
- [ ] T083 Implement internal/observability/tracer.go with OpenTelemetry tracer setup

### Redis Layer

- [ ] T084 Implement internal/repository/redis/cache.go with generic cache operations
- [ ] T085 Implement internal/repository/redis/ratelimit.go with sliding window rate limiter

### HTTP Infrastructure

- [ ] T086 Implement internal/api/middleware/trace.go with trace_id and request_id propagation
- [ ] T087 Implement internal/api/middleware/auth.go with API key and session authentication
- [ ] T088 Implement internal/api/middleware/tenant.go with organization context injection and RLS setup
- [ ] T089 Implement internal/api/middleware/ratelimit.go with per-key rate limiting
- [ ] T090 Implement internal/api/middleware/idempotency.go with idempotency key handling
- [ ] T091 Implement internal/api/router.go with chi router setup and middleware chain
- [ ] T092 Implement internal/api/handler/health.go with /health/live and /health/ready endpoints
- [ ] T093 Implement internal/api/handler/errors.go with RFC 7807 Problem Details error responses

### Provider Framework

- [ ] T094 Implement internal/provider/adapter.go with ProviderAdapter interface definition
- [ ] T095 Implement internal/provider/factory.go with adapter factory and registration
- [ ] T096 Implement internal/provider/soap/client.go with SOAP client utilities
- [ ] T097 Implement internal/provider/soap/xml.go with XML parsing utilities
- [ ] T098 Implement internal/provider/rest/client.go with REST client with circuit breaker and retry

### Worker Infrastructure

- [ ] T099 Implement internal/worker/scheduler.go with Asynq scheduler setup
- [ ] T100 Implement internal/worker/tasks/types.go with task type constants

### Entry Points

- [ ] T101 Implement cmd/api/main.go with HTTP server entrypoint
- [ ] T102 Implement cmd/worker/main.go with Asynq worker entrypoint
- [ ] T103 Implement cmd/migrate/main.go with database migration tool

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Marketplace Shipment Creation (Priority: P1) 🎯 MVP

**Goal**: Enable customers to create shipments using marketplace rates with wallet payment

**Independent Test**: Create quote → select offer → create shipment → purchase label → verify wallet charged

### Implementation for User Story 1

- [ ] T104 [P] [US1] Implement internal/service/auth/apikey.go with API key validation and scope checking
- [ ] T105 [P] [US1] Implement internal/service/auth/session.go with session management
- [ ] T106 [US1] Implement internal/api/handler/auth.go with POST /auth/login, POST /auth/logout, GET /auth/me endpoints
- [ ] T107 [US1] Implement internal/service/organization/service.go with organization CRUD operations
- [ ] T108 [US1] Implement internal/api/handler/organization.go with GET/PATCH /orgs/current, member management endpoints
- [ ] T109 [US1] Implement internal/api/handler/reference.go with GET /ref/cities, /ref/districts, /ref/neighborhoods, /ref/postal-codes endpoints
- [ ] T110 [US1] Implement internal/api/handler/address.go with CRUD endpoints for /addresses
- [ ] T111 [US1] Implement internal/api/handler/provider.go with GET /providers, GET /providers/{code}, GET /providers/{code}/services endpoints
- [ ] T112 [US1] Implement internal/service/quote/service.go with quote creation, offer generation, and expiry handling
- [ ] T113 [US1] Implement internal/api/handler/quote.go with POST /quotes, GET /quotes/{id}, GET /quotes/{id}/offers, POST /quotes/{id}/refresh endpoints
- [ ] T114 [US1] Implement internal/service/billing/wallet.go with balance calculation from ledger
- [ ] T115 [US1] Implement internal/service/billing/ledger.go with debit/credit operations and invariant checks
- [ ] T116 [US1] Implement internal/service/shipment/service.go with CreateFromOffer, PurchaseLabel, Get, List operations
- [ ] T117 [US1] Implement internal/api/handler/shipment.go with POST /shipments, GET /shipments, GET /shipments/{id}, POST /shipments/{id}/purchase-label endpoints
- [ ] T118 [US1] Implement internal/api/handler/document.go with GET /documents/{id} endpoint
- [ ] T119 [US1] Implement internal/api/handler/billing.go with GET /billing/balance, GET /billing/ledger endpoints
- [ ] T120 [US1] Implement internal/provider/aras/adapter.go with Aras SOAP adapter (GetQuote, CreateShipment, GetLabel)
- [ ] T121 [US1] Implement internal/provider/aras/types.go with Aras-specific request/response types
- [ ] T122 [US1] Implement internal/provider/aras/mapping.go with status code mapping
- [ ] T123 [US1] Implement internal/provider/yurtici/adapter.go with Yurtiçi SOAP adapter
- [ ] T124 [US1] Implement internal/provider/yurtici/types.go with Yurtiçi-specific types
- [ ] T125 [US1] Implement internal/provider/yurtici/mapping.go with status code mapping
- [ ] T126 [US1] Implement internal/provider/hepsijet/adapter.go with Hepsijet REST adapter
- [ ] T127 [US1] Implement internal/provider/hepsijet/types.go with Hepsijet-specific types
- [ ] T128 [US1] Implement internal/provider/hepsijet/mapping.go with status code mapping
- [ ] T129 [US1] Register routes for US1 endpoints in internal/api/router.go

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently

---

## Phase 4: User Story 2 - BYOA Shipment Creation (Priority: P1)

**Goal**: Enable customers to use their own carrier credentials for shipments

**Independent Test**: Create provider account → verify credentials → create BYOA quote → create shipment

### Implementation for User Story 2

- [ ] T130 [P] [US2] Implement internal/service/provider/account.go with provider account CRUD and credential encryption
- [ ] T131 [US2] Implement internal/service/provider/verification.go with credential verification via carrier API
- [ ] T132 [US2] Implement internal/api/handler/provider_account.go with CRUD endpoints for /provider-accounts
- [ ] T133 [US2] Implement POST /provider-accounts/{id}/verify endpoint in internal/api/handler/provider_account.go
- [ ] T134 [US2] Update internal/service/quote/service.go to support use_byoa flag and BYOA pricing
- [ ] T135 [US2] Update internal/service/shipment/service.go to handle BYOA shipments (no wallet charge for shipping)
- [ ] T136 [US2] Register routes for US2 endpoints in internal/api/router.go

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently

---

## Phase 5: User Story 3 - COD Shipment with Settlement (Priority: P2)

**Goal**: Enable COD shipments with settlement tracking and reconciliation

**Independent Test**: Create COD shipment → simulate delivery → verify settlement record created

### Implementation for User Story 3

- [ ] T137 [P] [US3] Implement internal/service/billing/cod.go with COD settlement tracking and reconciliation
- [ ] T138 [US3] Update internal/service/quote/service.go to filter providers by COD support and add cod_fee
- [ ] T139 [US3] Update internal/service/shipment/service.go to handle COD shipments and create settlement records
- [ ] T140 [US3] Implement internal/api/handler/billing.go GET /billing/cod/settlements endpoint
- [ ] T141 [US3] Implement internal/api/handler/billing.go POST /billing/cod/settlements/reconcile endpoint (admin)
- [ ] T142 [US3] Register routes for US3 endpoints in internal/api/router.go

**Checkpoint**: COD shipments and settlement tracking functional

---

## Phase 6: User Story 4 - Wallet Top-Up via Iyzico (Priority: P2)

**Goal**: Enable wallet funding through Iyzico payment gateway

**Independent Test**: Initiate top-up → complete Iyzico payment → verify wallet balance increased

### Implementation for User Story 4

- [ ] T143 [P] [US4] Implement internal/payment/iyzico/client.go with Iyzico API client
- [ ] T144 [P] [US4] Implement internal/payment/iyzico/types.go with Iyzico request/response types
- [ ] T145 [US4] Implement internal/service/billing/topup.go with top-up initiation and confirmation
- [ ] T146 [US4] Implement internal/api/handler/billing.go POST /billing/topups endpoint
- [ ] T147 [US4] Implement internal/api/handler/billing.go POST /billing/topups/{id}/confirm endpoint
- [ ] T148 [US4] Implement internal/api/handler/billing.go GET /billing/topups endpoint
- [ ] T149 [US4] Implement internal/api/integration/iyzico.go with Iyzico webhook handler
- [ ] T150 [US4] Register routes for US4 endpoints in internal/api/router.go

**Checkpoint**: Wallet top-up via Iyzico functional

---

## Phase 7: User Story 5 - Tracking Updates and Webhooks (Priority: P2)

**Goal**: Receive carrier tracking updates and notify customers via webhooks

**Independent Test**: Receive carrier webhook → verify event normalized → verify customer webhook delivered

### Implementation for User Story 5

- [ ] T151 [P] [US5] Implement internal/service/tracking/service.go with tracking event processing and normalization
- [ ] T152 [P] [US5] Implement internal/service/webhook/publisher.go with outbox-based event publishing
- [ ] T153 [US5] Implement internal/service/webhook/delivery.go with webhook delivery logic and signing
- [ ] T154 [US5] Implement internal/api/handler/webhook.go with CRUD endpoints for /webhooks/endpoints
- [ ] T155 [US5] Implement internal/api/handler/webhook.go with CRUD endpoints for /webhooks/subscriptions
- [ ] T156 [US5] Implement internal/api/handler/webhook.go with GET /webhooks/deliveries, POST /webhooks/deliveries/{id}/replay endpoints
- [ ] T157 [US5] Implement internal/api/handler/webhook.go POST /webhooks/endpoints/{id}/test endpoint
- [ ] T158 [US5] Implement internal/api/integration/aras.go with Aras tracking webhook handler
- [ ] T159 [US5] Implement internal/api/integration/yurtici.go with Yurtiçi tracking webhook handler
- [ ] T160 [US5] Implement internal/api/integration/hepsijet.go with Hepsijet tracking webhook handler
- [ ] T161 [US5] Implement internal/worker/tasks/webhook_delivery.go with webhook delivery task
- [ ] T162 [US5] Implement internal/worker/tasks/outbox_publisher.go with outbox polling and publishing
- [ ] T163 [US5] Implement internal/worker/tasks/tracking_poll.go with tracking poll for non-webhook providers
- [ ] T164 [US5] Implement GET /shipments/{id}/tracking endpoint in internal/api/handler/shipment.go
- [ ] T165 [US5] Register routes for US5 endpoints in internal/api/router.go

**Checkpoint**: Tracking updates and customer webhooks functional

---

## Phase 8: User Story 6 - API Key Management (Priority: P2)

**Goal**: Enable customers to manage API keys with scopes, rotation, and rate limits

**Independent Test**: Create API key → use for auth → rotate → verify old key invalidated after grace period

### Implementation for User Story 6

- [ ] T166 [P] [US6] Update internal/service/auth/apikey.go with key rotation and grace period logic
- [ ] T167 [US6] Implement internal/api/handler/apikey.go with CRUD endpoints for /api-keys
- [ ] T168 [US6] Implement internal/api/handler/apikey.go POST /api-keys/{id}/rotate endpoint
- [ ] T169 [US6] Register routes for US6 endpoints in internal/api/router.go

**Checkpoint**: API key management with rotation functional

---

## Phase 9: User Story 7 - Shipment Cancellation and Refund (Priority: P3)

**Goal**: Enable shipment cancellation with automatic refund processing

**Independent Test**: Create shipment → purchase label → cancel → verify refund credited

### Implementation for User Story 7

- [ ] T170 [P] [US7] Update internal/service/shipment/service.go with Cancel operation and carrier API call
- [ ] T171 [US7] Implement internal/service/billing/refund.go with refund processing and ledger credit
- [ ] T172 [US7] Implement POST /shipments/{id}/cancel endpoint in internal/api/handler/shipment.go
- [ ] T173 [US7] Implement POST /billing/refunds endpoint in internal/api/handler/billing.go
- [ ] T174 [US7] Register routes for US7 endpoints in internal/api/router.go

**Checkpoint**: Shipment cancellation and refunds functional

---

## Phase 10: User Story 8 - Return Shipment Creation (Priority: P3)

**Goal**: Enable return shipment creation for delivered packages

**Independent Test**: Create original shipment → mark delivered → create return → verify addresses swapped

### Implementation for User Story 8

- [ ] T175 [P] [US8] Update internal/service/shipment/service.go with CreateReturn operation
- [ ] T176 [US8] Implement POST /shipments/{id}/return endpoint in internal/api/handler/shipment.go
- [ ] T177 [US8] Register routes for US8 endpoints in internal/api/router.go

**Checkpoint**: Return shipment creation functional

---

## Phase 11: User Story 9 - Pickup Scheduling (Priority: P3)

**Goal**: Enable pickup scheduling for shipments

**Independent Test**: Create shipment → purchase label → schedule pickup → verify pickup_scheduled_at set

### Implementation for User Story 9

- [ ] T178 [P] [US9] Update internal/service/shipment/service.go with SchedulePickup operation
- [ ] T179 [US9] Implement POST /shipments/{id}/pickup endpoint in internal/api/handler/shipment.go
- [ ] T180 [US9] Register routes for US9 endpoints in internal/api/router.go

**Checkpoint**: Pickup scheduling functional

---

## Phase 12: User Story 10 - Admin Operations (Priority: P3)

**Goal**: Enable admin monitoring and exception handling

**Independent Test**: Access admin endpoints → verify audit logs → rebuild tracking

### Implementation for User Story 10

- [ ] T181 [P] [US10] Implement internal/repository/postgres/audit.go with audit log repository
- [ ] T182 [P] [US10] Implement internal/repository/postgres/provider_logs.go with provider API log repository
- [ ] T183 [US10] Implement internal/api/handler/admin.go with GET /admin/audit-logs endpoint
- [ ] T184 [US10] Implement internal/api/handler/admin.go with GET /admin/provider-logs endpoint
- [ ] T185 [US10] Implement internal/api/handler/admin.go with GET /admin/health/providers endpoint
- [ ] T186 [US10] Implement internal/api/handler/admin.go with POST /admin/webhooks/dlq/reprocess endpoint
- [ ] T187 [US10] Implement internal/api/handler/admin.go with POST /admin/shipments/{id}/rebuild-tracking endpoint
- [ ] T188 [US10] Implement internal/api/handler/admin.go with POST /integrations/{provider}/poll-tracking endpoint
- [ ] T189 [US10] Register routes for US10 endpoints in internal/api/router.go

**Checkpoint**: Admin operations functional

---

## Phase 13: Additional Provider Adapters

**Purpose**: Implement remaining carrier adapters

- [ ] T190 [P] Implement internal/provider/surat/adapter.go with Sürat SOAP adapter
- [ ] T191 [P] Implement internal/provider/surat/types.go with Sürat-specific types
- [ ] T192 [P] Implement internal/provider/surat/mapping.go with status code mapping
- [ ] T193 [P] Implement internal/provider/ptt/adapter.go with PTT SOAP adapter
- [ ] T194 [P] Implement internal/provider/ptt/types.go with PTT-specific types
- [ ] T195 [P] Implement internal/provider/ptt/mapping.go with status code mapping
- [ ] T196 [P] Implement internal/provider/mng/adapter.go with MNG SOAP adapter
- [ ] T197 [P] Implement internal/provider/mng/types.go with MNG-specific types
- [ ] T198 [P] Implement internal/provider/mng/mapping.go with status code mapping
- [ ] T199 [P] Implement internal/provider/kolaygelsin/adapter.go with Kolaygelsin REST adapter
- [ ] T200 [P] Implement internal/provider/kolaygelsin/types.go with Kolaygelsin-specific types
- [ ] T201 [P] Implement internal/provider/kolaygelsin/mapping.go with status code mapping
- [ ] T202 [P] Implement internal/provider/sendeo/adapter.go with Sendeo REST adapter
- [ ] T203 [P] Implement internal/provider/sendeo/types.go with Sendeo-specific types
- [ ] T204 [P] Implement internal/provider/sendeo/mapping.go with status code mapping
- [ ] T205 [P] Implement internal/provider/pakettaxi/adapter.go with Pakettaxi REST adapter
- [ ] T206 [P] Implement internal/provider/pakettaxi/types.go with Pakettaxi-specific types
- [ ] T207 [P] Implement internal/provider/pakettaxi/mapping.go with status code mapping
- [ ] T208 [P] Implement internal/api/integration/mng.go with MNG tracking webhook handler
- [ ] T209 [P] Implement internal/api/integration/kolaygelsin.go with Kolaygelsin tracking webhook handler
- [ ] T210 [P] Implement internal/api/integration/sendeo.go with Sendeo tracking webhook handler
- [ ] T211 [P] Implement internal/api/integration/pakettaxi.go with Pakettaxi tracking webhook handler
- [ ] T212 Register all provider adapters in internal/provider/factory.go

**Checkpoint**: All 9 carrier adapters implemented

---

## Phase 14: Background Jobs & Maintenance

**Purpose**: Implement scheduled jobs and maintenance tasks

- [ ] T213 [P] Implement internal/worker/tasks/idempotency_cleanup.go with 24-hour record cleanup
- [ ] T214 [P] Implement internal/worker/tasks/reconciliation.go with daily financial reconciliation
- [ ] T215 [P] Implement internal/worker/tasks/retention_cleanup.go with data retention enforcement
- [ ] T216 Configure scheduler in internal/worker/scheduler.go with job schedules

**Checkpoint**: All background jobs implemented

---

## Phase 15: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T217 [P] Add OpenAPI spec serving at GET /v1/openapi.json in internal/api/router.go
- [ ] T218 [P] Implement GET /billing/transactions combined view in internal/api/handler/billing.go
- [ ] T219 [P] Implement GET /providers/{code}/dropoff-points endpoint in internal/api/handler/provider.go
- [ ] T220 [P] Implement internal/api/handler/role.go with CRUD endpoints for /roles
- [ ] T221 Code cleanup and refactoring across all packages
- [ ] T222 Add request/response logging middleware in internal/api/middleware/logging.go
- [ ] T223 Add panic recovery middleware in internal/api/middleware/recovery.go
- [ ] T224 Run quickstart.md validation scenarios
- [ ] T225 Security hardening review and fixes

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-12)**: All depend on Foundational phase completion
  - User stories can then proceed in parallel (if staffed)
  - Or sequentially in priority order (P1 → P2 → P3)
- **Additional Providers (Phase 13)**: Can start after Phase 3 (US1) is complete
- **Background Jobs (Phase 14)**: Can start after Phase 5 (US3) is complete
- **Polish (Phase 15)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P1)**: Can start after Foundational (Phase 2) - Shares provider framework with US1
- **User Story 3 (P2)**: Can start after Foundational (Phase 2) - Extends billing from US1
- **User Story 4 (P2)**: Can start after Foundational (Phase 2) - Extends billing from US1
- **User Story 5 (P2)**: Can start after Foundational (Phase 2) - Requires shipment from US1
- **User Story 6 (P2)**: Can start after Foundational (Phase 2) - Extends auth from US1
- **User Story 7 (P3)**: Requires US1 complete (shipment creation)
- **User Story 8 (P3)**: Requires US1 complete (shipment creation)
- **User Story 9 (P3)**: Requires US1 complete (shipment creation)
- **User Story 10 (P3)**: Can start after Foundational (Phase 2) - Independent admin features

### Within Each User Story

- Models before services
- Services before handlers
- Core implementation before integration
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel
- All Foundational tasks marked [P] can run in parallel (within Phase 2)
- Once Foundational phase completes, all user stories can start in parallel (if team capacity allows)
- All provider adapters marked [P] can run in parallel
- Different user stories can be worked on in parallel by different team members

---

## Parallel Example: User Story 1

```bash
# Launch all domain models together:
Task: "Implement internal/domain/organization.go"
Task: "Implement internal/domain/member.go"
Task: "Implement internal/domain/apikey.go"
Task: "Implement internal/domain/address.go"
Task: "Implement internal/domain/provider.go"
Task: "Implement internal/domain/quote.go"
Task: "Implement internal/domain/shipment.go"

# Launch all provider adapters together:
Task: "Implement internal/provider/aras/adapter.go"
Task: "Implement internal/provider/yurtici/adapter.go"
Task: "Implement internal/provider/hepsijet/adapter.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: Test User Story 1 independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test independently → Deploy/Demo
4. Add User Stories 3-6 → Test independently → Deploy/Demo
5. Add User Stories 7-10 → Test independently → Deploy/Demo
6. Add remaining providers → Full carrier coverage
7. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (Marketplace)
   - Developer B: User Story 2 (BYOA)
   - Developer C: User Story 5 (Webhooks)
3. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence

# Data Model Specification

**Feature Branch**: `001-core-platform`  
**Created**: 2026-01-22  
**Status**: Draft

---

## Overview

This document defines the database schema for the TR-Only Shipping Aggregation Backend. The schema follows these principles from the constitution:

1. **Multi-tenant isolation**: All tenant-scoped tables include `organization_id` with enforced foreign key and RLS policies.
2. **Immutable ledger**: Financial transactions are append-only with no UPDATE/DELETE operations.
3. **Money as integers**: All monetary values stored as integer kuruş (1 TRY = 100 kuruş) to avoid floating-point errors.
4. **Soft deletes**: PII-containing records use soft delete with 30-day retention before hard delete.
5. **Audit trail**: State changes are logged in audit tables.

---

## Naming Conventions

- Table names: `snake_case`, plural (e.g., `organizations`, `shipments`)
- Column names: `snake_case` (e.g., `created_at`, `organization_id`)
- Primary keys: `id` (UUID)
- Foreign keys: `{referenced_table_singular}_id` (e.g., `organization_id`, `shipment_id`)
- Timestamps: `{action}_at` (e.g., `created_at`, `updated_at`, `deleted_at`)
- Boolean flags: `is_{condition}` (e.g., `is_active`, `is_default`)
- Enums: Stored as `VARCHAR` with CHECK constraints for flexibility

---

## Tables

### organizations

Multi-tenant root entity. All customer resources belong to an organization.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| name | VARCHAR(255) | NOT NULL | Organization display name |
| slug | VARCHAR(100) | NOT NULL, UNIQUE | URL-safe identifier |
| billing_email | VARCHAR(255) | NOT NULL | Primary billing contact |
| settings | JSONB | NOT NULL, DEFAULT '{}' | Organization-level settings |
| status | VARCHAR(20) | NOT NULL, DEFAULT 'active', CHECK (status IN ('active', 'suspended', 'closed')) | Account status |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last update timestamp |
| deleted_at | TIMESTAMPTZ | NULL | Soft delete timestamp |

**Indexes**:

- `idx_organizations_slug` UNIQUE ON (slug) WHERE deleted_at IS NULL
- `idx_organizations_status` ON (status)

---

### members

Users belonging to organizations with role-based access.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| organization_id | UUID | NOT NULL, REFERENCES organizations(id) | Parent organization |
| email | VARCHAR(255) | NOT NULL | Member email (login identifier) |
| name | VARCHAR(255) | NOT NULL | Display name |
| password_hash | VARCHAR(255) | NULL | Bcrypt hash (NULL for invited) |
| role_id | UUID | NOT NULL, REFERENCES roles(id) | Assigned role |
| status | VARCHAR(20) | NOT NULL, DEFAULT 'invited', CHECK (status IN ('invited', 'active', 'suspended', 'removed')) | Member status |
| invited_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Invitation timestamp |
| joined_at | TIMESTAMPTZ | NULL | Activation timestamp |
| last_login_at | TIMESTAMPTZ | NULL | Last successful login |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last update timestamp |

**Indexes**:

- `idx_members_org_email` UNIQUE ON (organization_id, email) WHERE status != 'removed'
- `idx_members_organization_id` ON (organization_id)
- `idx_members_email` ON (email)

---

### roles

Permission sets assignable to members.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| organization_id | UUID | NULL, REFERENCES organizations(id) | NULL for system roles |
| name | VARCHAR(100) | NOT NULL | Role name |
| description | VARCHAR(500) | NULL | Role description |
| permissions | JSONB | NOT NULL, DEFAULT '[]' | Array of permission strings |
| is_system | BOOLEAN | NOT NULL, DEFAULT FALSE | System role (non-editable) |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last update timestamp |

**Indexes**:

- `idx_roles_org_name` UNIQUE ON (organization_id, name)
- `idx_roles_organization_id` ON (organization_id)

**System Roles** (seeded):

- `owner`: Full access to all resources
- `admin`: Full access except billing/organization deletion
- `operator`: Shipment and quote management
- `viewer`: Read-only access

---

### api_keys

API authentication credentials.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| organization_id | UUID | NOT NULL, REFERENCES organizations(id) | Parent organization |
| name | VARCHAR(100) | NOT NULL | Key name/description |
| key_prefix | VARCHAR(8) | NOT NULL | First 8 chars for identification |
| key_hash | VARCHAR(255) | NOT NULL | SHA-256 hash of full key |
| previous_key_hash | VARCHAR(255) | NULL | Previous key hash during rotation |
| previous_key_expires_at | TIMESTAMPTZ | NULL | When previous key becomes invalid |
| scopes | JSONB | NOT NULL, DEFAULT '[]' | Array of scope strings |
| rate_limit_per_minute | INTEGER | NOT NULL, DEFAULT 100 | Requests per minute limit |
| ip_allowlist | JSONB | NOT NULL, DEFAULT '[]' | Array of allowed IPs/CIDRs |
| expires_at | TIMESTAMPTZ | NULL | Key expiration (NULL = no expiry) |
| last_used_at | TIMESTAMPTZ | NULL | Last successful use |
| is_active | BOOLEAN | NOT NULL, DEFAULT TRUE | Active status |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last update timestamp |
| revoked_at | TIMESTAMPTZ | NULL | Revocation timestamp |

**Indexes**:

- `idx_api_keys_key_hash` ON (key_hash) WHERE revoked_at IS NULL
- `idx_api_keys_previous_key_hash` ON (previous_key_hash) WHERE previous_key_hash IS NOT NULL
- `idx_api_keys_organization_id` ON (organization_id)
- `idx_api_keys_key_prefix` ON (key_prefix)

---

### sessions

Dashboard authentication sessions.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Session identifier |
| member_id | UUID | NOT NULL, REFERENCES members(id) | Authenticated member |
| token_hash | VARCHAR(255) | NOT NULL | SHA-256 hash of session token |
| ip_address | INET | NOT NULL | Client IP address |
| user_agent | VARCHAR(500) | NULL | Client user agent |
| expires_at | TIMESTAMPTZ | NOT NULL | Session expiration |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |
| last_activity_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last activity timestamp |

**Indexes**:

- `idx_sessions_token_hash` ON (token_hash)
- `idx_sessions_member_id` ON (member_id)
- `idx_sessions_expires_at` ON (expires_at)

---

### ref_cities

Turkish cities reference data.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| code | VARCHAR(10) | PRIMARY KEY | City code (plate code, e.g., "34") |
| name | VARCHAR(100) | NOT NULL | City name |
| name_normalized | VARCHAR(100) | NOT NULL | Lowercase, ASCII normalized |
| is_active | BOOLEAN | NOT NULL, DEFAULT TRUE | Serviceable status |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last sync timestamp |

**Indexes**:

- `idx_ref_cities_name_normalized` ON (name_normalized)

---

### ref_districts

Turkish districts (ilçe) reference data.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| code | VARCHAR(20) | PRIMARY KEY | District code |
| city_code | VARCHAR(10) | NOT NULL, REFERENCES ref_cities(code) | Parent city |
| name | VARCHAR(100) | NOT NULL | District name |
| name_normalized | VARCHAR(100) | NOT NULL | Lowercase, ASCII normalized |
| is_active | BOOLEAN | NOT NULL, DEFAULT TRUE | Serviceable status |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last sync timestamp |

**Indexes**:

- `idx_ref_districts_city_code` ON (city_code)
- `idx_ref_districts_name_normalized` ON (name_normalized)

---

### ref_neighborhoods

Turkish neighborhoods (mahalle) reference data.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| code | VARCHAR(30) | PRIMARY KEY | Neighborhood code |
| district_code | VARCHAR(20) | NOT NULL, REFERENCES ref_districts(code) | Parent district |
| name | VARCHAR(150) | NOT NULL | Neighborhood name |
| name_normalized | VARCHAR(150) | NOT NULL | Lowercase, ASCII normalized |
| postal_code | VARCHAR(10) | NULL | Associated postal code |
| is_active | BOOLEAN | NOT NULL, DEFAULT TRUE | Serviceable status |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last sync timestamp |

**Indexes**:

- `idx_ref_neighborhoods_district_code` ON (district_code)
- `idx_ref_neighborhoods_postal_code` ON (postal_code)
- `idx_ref_neighborhoods_name_normalized` ON (name_normalized)

---

### ref_postal_codes

Turkish postal codes reference data.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| code | VARCHAR(10) | PRIMARY KEY | Postal code |
| district_code | VARCHAR(20) | NOT NULL, REFERENCES ref_districts(code) | Parent district |
| is_active | BOOLEAN | NOT NULL, DEFAULT TRUE | Serviceable status |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last sync timestamp |

**Indexes**:

- `idx_ref_postal_codes_district_code` ON (district_code)

---

### addresses

Saved addresses for organizations.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| organization_id | UUID | NOT NULL, REFERENCES organizations(id) | Parent organization |
| label | VARCHAR(100) | NULL | User-defined label |
| contact_name | VARCHAR(255) | NOT NULL | Contact person name |
| company_name | VARCHAR(255) | NULL | Company/business name |
| phone | VARCHAR(20) | NOT NULL | Phone in E.164 format |
| email | VARCHAR(255) | NULL | Contact email |
| street_line_1 | VARCHAR(500) | NOT NULL | Primary street address |
| street_line_2 | VARCHAR(500) | NULL | Secondary address line |
| neighborhood_code | VARCHAR(30) | NOT NULL, REFERENCES ref_neighborhoods(code) | Neighborhood reference |
| district_code | VARCHAR(20) | NOT NULL, REFERENCES ref_districts(code) | District reference |
| city_code | VARCHAR(10) | NOT NULL, REFERENCES ref_cities(code) | City reference |
| postal_code | VARCHAR(10) | NULL | Postal code |
| latitude | DECIMAL(10, 8) | NULL | GPS latitude |
| longitude | DECIMAL(11, 8) | NULL | GPS longitude |
| is_default_sender | BOOLEAN | NOT NULL, DEFAULT FALSE | Default sender address |
| is_default_recipient | BOOLEAN | NOT NULL, DEFAULT FALSE | Default recipient address |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last update timestamp |
| deleted_at | TIMESTAMPTZ | NULL | Soft delete timestamp |

**Indexes**:

- `idx_addresses_organization_id` ON (organization_id) WHERE deleted_at IS NULL
- `idx_addresses_city_code` ON (city_code)
- `idx_addresses_district_code` ON (district_code)

---

### providers

Integrated carrier providers.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| code | VARCHAR(50) | PRIMARY KEY | Provider code (e.g., "aras", "hepsijet") |
| name | VARCHAR(100) | NOT NULL | Display name |
| logo_url | VARCHAR(500) | NULL | Logo image URL |
| website | VARCHAR(255) | NULL | Provider website |
| support_email | VARCHAR(255) | NULL | Support contact email |
| support_phone | VARCHAR(50) | NULL | Support contact phone |
| api_type | VARCHAR(20) | NOT NULL, CHECK (api_type IN ('soap', 'rest')) | API protocol type |
| api_base_url | VARCHAR(500) | NOT NULL | Base URL for API calls |
| api_sandbox_url | VARCHAR(500) | NULL | Sandbox environment URL |
| is_active | BOOLEAN | NOT NULL, DEFAULT TRUE | Provider availability |
| capabilities | JSONB | NOT NULL, DEFAULT '{}' | Capability flags |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last update timestamp |

---

### provider_services

Service levels offered by providers.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| provider_code | VARCHAR(50) | NOT NULL, REFERENCES providers(code) | Parent provider |
| service_code | VARCHAR(50) | NOT NULL | Service code (provider-specific) |
| name | VARCHAR(100) | NOT NULL | Service display name |
| description | VARCHAR(500) | NULL | Service description |
| estimated_days_min | INTEGER | NOT NULL | Minimum delivery days |
| estimated_days_max | INTEGER | NOT NULL | Maximum delivery days |
| supports_cod | BOOLEAN | NOT NULL, DEFAULT FALSE | COD support |
| supports_insurance | BOOLEAN | NOT NULL, DEFAULT FALSE | Insurance support |
| supports_return | BOOLEAN | NOT NULL, DEFAULT FALSE | Return shipment support |
| max_weight_grams | INTEGER | NOT NULL | Maximum weight in grams |
| max_length_cm | INTEGER | NOT NULL | Maximum length in cm |
| max_width_cm | INTEGER | NOT NULL | Maximum width in cm |
| max_height_cm | INTEGER | NOT NULL | Maximum height in cm |
| label_formats | JSONB | NOT NULL, DEFAULT '["pdf"]' | Supported label formats |
| is_active | BOOLEAN | NOT NULL, DEFAULT TRUE | Service availability |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last update timestamp |

**Indexes**:

- `idx_provider_services_provider_code` ON (provider_code)
- `idx_provider_services_provider_service` UNIQUE ON (provider_code, service_code)

---

### provider_accounts

BYOA credentials for customer-owned carrier accounts.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| organization_id | UUID | NOT NULL, REFERENCES organizations(id) | Parent organization |
| provider_code | VARCHAR(50) | NOT NULL, REFERENCES providers(code) | Carrier provider |
| name | VARCHAR(100) | NOT NULL | Account name/label |
| credentials_encrypted | BYTEA | NOT NULL | Encrypted credentials blob |
| credentials_iv | BYTEA | NOT NULL | Encryption IV |
| status | VARCHAR(30) | NOT NULL, DEFAULT 'pending_verification', CHECK (status IN ('pending_verification', 'verified', 'verification_failed', 'suspended')) | Account status |
| verification_error | VARCHAR(500) | NULL | Last verification error |
| verified_at | TIMESTAMPTZ | NULL | Verification timestamp |
| last_used_at | TIMESTAMPTZ | NULL | Last successful use |
| is_default | BOOLEAN | NOT NULL, DEFAULT FALSE | Default account for provider |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last update timestamp |
| deleted_at | TIMESTAMPTZ | NULL | Soft delete timestamp |

**Indexes**:

- `idx_provider_accounts_organization_id` ON (organization_id) WHERE deleted_at IS NULL
- `idx_provider_accounts_org_provider` ON (organization_id, provider_code) WHERE deleted_at IS NULL

---

### marketplace_rate_plans

Internal pricing configuration for marketplace model.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| provider_code | VARCHAR(50) | NOT NULL, REFERENCES providers(code) | Carrier provider |
| service_code | VARCHAR(50) | NOT NULL | Service code |
| name | VARCHAR(100) | NOT NULL | Rate plan name |
| base_price_kurus | INTEGER | NOT NULL | Base price in kuruş |
| price_per_kg_kurus | INTEGER | NOT NULL, DEFAULT 0 | Per-kg surcharge |
| price_per_desi_kurus | INTEGER | NOT NULL, DEFAULT 0 | Per-desi surcharge |
| fuel_surcharge_percent | DECIMAL(5, 2) | NOT NULL, DEFAULT 0 | Fuel surcharge % |
| cod_fee_percent | DECIMAL(5, 2) | NOT NULL, DEFAULT 0 | COD fee % |
| cod_fee_min_kurus | INTEGER | NOT NULL, DEFAULT 0 | Minimum COD fee |
| vat_percent | DECIMAL(5, 2) | NOT NULL, DEFAULT 20.00 | VAT percentage |
| effective_from | DATE | NOT NULL | Rate effective date |
| effective_to | DATE | NULL | Rate end date (NULL = current) |
| is_active | BOOLEAN | NOT NULL, DEFAULT TRUE | Rate plan active |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last update timestamp |

**Indexes**:

- `idx_marketplace_rate_plans_provider_service` ON (provider_code, service_code)
- `idx_marketplace_rate_plans_effective` ON (effective_from, effective_to)

---

### quotes

Rate request records.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| organization_id | UUID | NOT NULL, REFERENCES organizations(id) | Parent organization |
| idempotency_key | VARCHAR(100) | NOT NULL | Client idempotency key |
| origin_address | JSONB | NOT NULL | Origin address snapshot |
| destination_address | JSONB | NOT NULL | Destination address snapshot |
| parcel | JSONB | NOT NULL | Parcel details (weight, dimensions) |
| cod_amount_kurus | INTEGER | NULL | COD amount if applicable |
| declared_value_kurus | INTEGER | NULL | Declared value for insurance |
| use_byoa | BOOLEAN | NOT NULL, DEFAULT FALSE | Use BYOA accounts |
| provider_account_ids | JSONB | NULL | Specific BYOA accounts to use |
| status | VARCHAR(20) | NOT NULL, DEFAULT 'pending', CHECK (status IN ('pending', 'completed', 'failed', 'expired')) | Quote status |
| expires_at | TIMESTAMPTZ | NOT NULL | Quote expiration |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |

**Indexes**:

- `idx_quotes_organization_id` ON (organization_id)
- `idx_quotes_idempotency` UNIQUE ON (organization_id, idempotency_key)
- `idx_quotes_expires_at` ON (expires_at)

---

### offers

Price quotes from providers.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| quote_id | UUID | NOT NULL, REFERENCES quotes(id) | Parent quote |
| provider_code | VARCHAR(50) | NOT NULL, REFERENCES providers(code) | Carrier provider |
| service_code | VARCHAR(50) | NOT NULL | Service code |
| provider_account_id | UUID | NULL, REFERENCES provider_accounts(id) | BYOA account (if applicable) |
| is_marketplace | BOOLEAN | NOT NULL | Marketplace vs BYOA pricing |
| base_price_kurus | INTEGER | NOT NULL | Base shipping price |
| fuel_surcharge_kurus | INTEGER | NOT NULL, DEFAULT 0 | Fuel surcharge |
| cod_fee_kurus | INTEGER | NOT NULL, DEFAULT 0 | COD handling fee |
| insurance_fee_kurus | INTEGER | NOT NULL, DEFAULT 0 | Insurance fee |
| other_fees_kurus | INTEGER | NOT NULL, DEFAULT 0 | Other fees |
| subtotal_kurus | INTEGER | NOT NULL | Subtotal before VAT |
| vat_kurus | INTEGER | NOT NULL | VAT amount |
| total_price_kurus | INTEGER | NOT NULL | Total price |
| cost_price_kurus | INTEGER | NULL | Our cost (marketplace only) |
| margin_kurus | INTEGER | NULL | Margin (marketplace only) |
| estimated_days_min | INTEGER | NOT NULL | Minimum delivery days |
| estimated_days_max | INTEGER | NOT NULL | Maximum delivery days |
| provider_quote_id | VARCHAR(100) | NULL | Provider's quote reference |
| expires_at | TIMESTAMPTZ | NOT NULL | Offer expiration |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |

**Indexes**:

- `idx_offers_quote_id` ON (quote_id)
- `idx_offers_provider_code` ON (provider_code)

---

### shipments

Core shipment records.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| organization_id | UUID | NOT NULL, REFERENCES organizations(id) | Parent organization |
| idempotency_key | VARCHAR(100) | NOT NULL | Client idempotency key |
| offer_id | UUID | NULL, REFERENCES offers(id) | Source offer (if from quote) |
| provider_code | VARCHAR(50) | NOT NULL, REFERENCES providers(code) | Carrier provider |
| service_code | VARCHAR(50) | NOT NULL | Service code |
| provider_account_id | UUID | NULL, REFERENCES provider_accounts(id) | BYOA account (if applicable) |
| is_marketplace | BOOLEAN | NOT NULL | Marketplace vs BYOA |
| provider_shipment_id | VARCHAR(100) | NULL | Provider's shipment ID |
| provider_tracking_number | VARCHAR(100) | NULL | Provider's tracking number |
| provider_barcode | VARCHAR(100) | NULL | Provider's barcode |
| origin_address | JSONB | NOT NULL | Origin address snapshot |
| destination_address | JSONB | NOT NULL | Destination address snapshot |
| parcel | JSONB | NOT NULL | Parcel details |
| cod_amount_kurus | INTEGER | NULL | COD amount |
| declared_value_kurus | INTEGER | NULL | Declared value |
| status | VARCHAR(30) | NOT NULL, DEFAULT 'draft', CHECK (status IN ('draft', 'label_purchased', 'pickup_scheduled', 'picked_up', 'in_transit', 'at_hub', 'out_for_delivery', 'delivered', 'delivery_failed', 'returning', 'returned_to_sender', 'cancelled', 'exception')) | Shipment status |
| label_document_id | UUID | NULL, REFERENCES documents(id) | Label document |
| label_format | VARCHAR(10) | NULL, CHECK (label_format IN ('pdf', 'zpl', 'png')) | Label format |
| price_charged_kurus | INTEGER | NULL | Amount charged to wallet |
| cost_kurus | INTEGER | NULL | Our cost (marketplace) |
| pickup_scheduled_at | TIMESTAMPTZ | NULL | Scheduled pickup time |
| picked_up_at | TIMESTAMPTZ | NULL | Actual pickup time |
| delivered_at | TIMESTAMPTZ | NULL | Delivery timestamp |
| cancelled_at | TIMESTAMPTZ | NULL | Cancellation timestamp |
| cancellation_reason | VARCHAR(500) | NULL | Cancellation reason |
| return_shipment_id | UUID | NULL, REFERENCES shipments(id) | Linked return shipment |
| original_shipment_id | UUID | NULL, REFERENCES shipments(id) | Original shipment (for returns) |
| metadata | JSONB | NOT NULL, DEFAULT '{}' | Custom metadata |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last update timestamp |

**Indexes**:

- `idx_shipments_organization_id` ON (organization_id)
- `idx_shipments_idempotency` UNIQUE ON (organization_id, idempotency_key)
- `idx_shipments_provider_tracking` ON (provider_code, provider_tracking_number)
- `idx_shipments_status` ON (status)
- `idx_shipments_created_at` ON (created_at)
- `idx_shipments_offer_id` ON (offer_id)

---

### tracking_events

Shipment tracking history.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| shipment_id | UUID | NOT NULL, REFERENCES shipments(id) | Parent shipment |
| provider_code | VARCHAR(50) | NOT NULL | Source provider |
| status_code | VARCHAR(30) | NOT NULL | Normalized status code |
| status_description | VARCHAR(500) | NOT NULL | Human-readable description |
| location_city | VARCHAR(100) | NULL | Event location city |
| location_detail | VARCHAR(500) | NULL | Detailed location |
| occurred_at | TIMESTAMPTZ | NOT NULL | When event occurred |
| raw_status_code | VARCHAR(100) | NULL | Provider's raw status |
| raw_data | JSONB | NULL | Full provider response |
| source | VARCHAR(20) | NOT NULL, CHECK (source IN ('webhook', 'poll', 'manual')) | Event source |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Record creation |

**Indexes**:

- `idx_tracking_events_shipment_id` ON (shipment_id)
- `idx_tracking_events_occurred_at` ON (occurred_at)
- `idx_tracking_events_shipment_occurred` ON (shipment_id, occurred_at DESC)

---

### documents

Stored files (labels, POD, etc.).

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| organization_id | UUID | NOT NULL, REFERENCES organizations(id) | Parent organization |
| type | VARCHAR(30) | NOT NULL, CHECK (type IN ('label', 'pod', 'invoice', 'customs', 'other')) | Document type |
| format | VARCHAR(10) | NOT NULL, CHECK (format IN ('pdf', 'zpl', 'png', 'jpg', 'xml')) | File format |
| filename | VARCHAR(255) | NOT NULL | Original filename |
| storage_path | VARCHAR(500) | NOT NULL | Object storage path |
| storage_bucket | VARCHAR(100) | NOT NULL | Storage bucket name |
| content_type | VARCHAR(100) | NOT NULL | MIME type |
| size_bytes | INTEGER | NOT NULL | File size |
| content_hash | VARCHAR(64) | NOT NULL | SHA-256 hash |
| metadata | JSONB | NOT NULL, DEFAULT '{}' | Additional metadata |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |

**Indexes**:

- `idx_documents_organization_id` ON (organization_id)
- `idx_documents_content_hash` ON (content_hash)

---

### webhook_endpoints

Customer webhook endpoint registrations.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| organization_id | UUID | NOT NULL, REFERENCES organizations(id) | Parent organization |
| url | VARCHAR(500) | NOT NULL | Webhook URL |
| secret_hash | VARCHAR(255) | NOT NULL | HMAC secret hash |
| description | VARCHAR(255) | NULL | Endpoint description |
| is_active | BOOLEAN | NOT NULL, DEFAULT TRUE | Active status |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last update timestamp |

**Indexes**:

- `idx_webhook_endpoints_organization_id` ON (organization_id)

---

### webhook_subscriptions

Event type subscriptions for endpoints.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| endpoint_id | UUID | NOT NULL, REFERENCES webhook_endpoints(id) ON DELETE CASCADE | Parent endpoint |
| event_type | VARCHAR(100) | NOT NULL | Event type (e.g., "shipment.delivered") |
| is_active | BOOLEAN | NOT NULL, DEFAULT TRUE | Subscription active |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |

**Indexes**:

- `idx_webhook_subscriptions_endpoint_id` ON (endpoint_id)
- `idx_webhook_subscriptions_event_type` ON (event_type)
- `idx_webhook_subscriptions_endpoint_event` UNIQUE ON (endpoint_id, event_type)

---

### webhook_events

Generated webhook events.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| organization_id | UUID | NOT NULL, REFERENCES organizations(id) | Parent organization |
| event_type | VARCHAR(100) | NOT NULL | Event type |
| resource_type | VARCHAR(50) | NOT NULL | Resource type (shipment, billing, etc.) |
| resource_id | UUID | NOT NULL | Resource identifier |
| payload | JSONB | NOT NULL | Event payload |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |

**Indexes**:

- `idx_webhook_events_organization_id` ON (organization_id)
- `idx_webhook_events_event_type` ON (event_type)
- `idx_webhook_events_created_at` ON (created_at)

---

### webhook_deliveries

Webhook delivery attempts.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| event_id | UUID | NOT NULL, REFERENCES webhook_events(id) | Source event |
| subscription_id | UUID | NOT NULL, REFERENCES webhook_subscriptions(id) | Target subscription |
| endpoint_url | VARCHAR(500) | NOT NULL | Delivery URL (snapshot) |
| status | VARCHAR(20) | NOT NULL, DEFAULT 'pending', CHECK (status IN ('pending', 'delivered', 'failed', 'dlq')) | Delivery status |
| attempts | INTEGER | NOT NULL, DEFAULT 0 | Attempt count |
| last_attempt_at | TIMESTAMPTZ | NULL | Last attempt timestamp |
| next_retry_at | TIMESTAMPTZ | NULL | Next retry timestamp |
| response_status | INTEGER | NULL | HTTP response status |
| response_body_preview | VARCHAR(1000) | NULL | Response body (truncated) |
| response_time_ms | INTEGER | NULL | Response time |
| error_message | VARCHAR(500) | NULL | Error message |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last update timestamp |

**Indexes**:

- `idx_webhook_deliveries_event_id` ON (event_id)
- `idx_webhook_deliveries_subscription_id` ON (subscription_id)
- `idx_webhook_deliveries_status` ON (status)
- `idx_webhook_deliveries_next_retry` ON (next_retry_at) WHERE status = 'pending'

---

### ledger_entries

Immutable financial transaction log.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| organization_id | UUID | NOT NULL, REFERENCES organizations(id) | Parent organization |
| entry_type | VARCHAR(10) | NOT NULL, CHECK (entry_type IN ('credit', 'debit')) | Entry type |
| amount_kurus | INTEGER | NOT NULL, CHECK (amount_kurus > 0) | Amount in kuruş (always positive) |
| currency | VARCHAR(3) | NOT NULL, DEFAULT 'TRY' | Currency code |
| balance_after_kurus | INTEGER | NOT NULL | Running balance after entry |
| reference_type | VARCHAR(50) | NOT NULL | Reference type (topup, shipment, refund, etc.) |
| reference_id | UUID | NOT NULL | Reference record ID |
| description | VARCHAR(500) | NOT NULL | Human-readable description |
| metadata | JSONB | NOT NULL, DEFAULT '{}' | Additional metadata |
| actor_id | UUID | NULL | User/system that created entry |
| actor_type | VARCHAR(20) | NOT NULL, DEFAULT 'system', CHECK (actor_type IN ('member', 'api_key', 'system')) | Actor type |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp (immutable) |

**Indexes**:

- `idx_ledger_entries_organization_id` ON (organization_id)
- `idx_ledger_entries_reference` ON (reference_type, reference_id)
- `idx_ledger_entries_created_at` ON (created_at)
- `idx_ledger_entries_org_created` ON (organization_id, created_at DESC)

**Constraints**:

- No UPDATE or DELETE operations allowed (enforced via triggers/policies)

---

### topups

Wallet funding requests.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| organization_id | UUID | NOT NULL, REFERENCES organizations(id) | Parent organization |
| idempotency_key | VARCHAR(100) | NOT NULL | Client idempotency key |
| amount_kurus | INTEGER | NOT NULL, CHECK (amount_kurus > 0) | Top-up amount |
| currency | VARCHAR(3) | NOT NULL, DEFAULT 'TRY' | Currency |
| status | VARCHAR(20) | NOT NULL, DEFAULT 'pending', CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'cancelled')) | Status |
| payment_provider | VARCHAR(20) | NOT NULL, DEFAULT 'iyzico' | Payment provider |
| payment_id | VARCHAR(100) | NULL | Provider payment ID |
| payment_url | VARCHAR(500) | NULL | Payment form URL |
| payment_token | VARCHAR(255) | NULL | Provider token |
| ledger_entry_id | UUID | NULL, REFERENCES ledger_entries(id) | Created ledger entry |
| failure_reason | VARCHAR(500) | NULL | Failure reason |
| completed_at | TIMESTAMPTZ | NULL | Completion timestamp |
| expires_at | TIMESTAMPTZ | NOT NULL | Payment expiration |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last update timestamp |

**Indexes**:

- `idx_topups_organization_id` ON (organization_id)
- `idx_topups_idempotency` UNIQUE ON (organization_id, idempotency_key)
- `idx_topups_payment_id` ON (payment_provider, payment_id)
- `idx_topups_status` ON (status)

---

### refunds

Refund records.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| organization_id | UUID | NOT NULL, REFERENCES organizations(id) | Parent organization |
| original_ledger_entry_id | UUID | NOT NULL, REFERENCES ledger_entries(id) | Original charge |
| refund_ledger_entry_id | UUID | NULL, REFERENCES ledger_entries(id) | Refund credit entry |
| amount_kurus | INTEGER | NOT NULL, CHECK (amount_kurus > 0) | Refund amount |
| reason | VARCHAR(500) | NOT NULL | Refund reason |
| status | VARCHAR(20) | NOT NULL, DEFAULT 'pending', CHECK (status IN ('pending', 'approved', 'completed', 'rejected')) | Status |
| approved_by | UUID | NULL, REFERENCES members(id) | Approver |
| approved_at | TIMESTAMPTZ | NULL | Approval timestamp |
| completed_at | TIMESTAMPTZ | NULL | Completion timestamp |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last update timestamp |

**Indexes**:

- `idx_refunds_organization_id` ON (organization_id)
- `idx_refunds_original_entry` ON (original_ledger_entry_id)
- `idx_refunds_status` ON (status)

---

### cod_settlements

COD collection tracking.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| organization_id | UUID | NOT NULL, REFERENCES organizations(id) | Parent organization |
| shipment_id | UUID | NOT NULL, REFERENCES shipments(id) | Related shipment |
| provider_code | VARCHAR(50) | NOT NULL | Carrier provider |
| expected_amount_kurus | INTEGER | NOT NULL | Expected COD amount |
| actual_amount_kurus | INTEGER | NULL | Actual collected amount |
| carrier_settlement_id | VARCHAR(100) | NULL | Carrier's settlement reference |
| carrier_settlement_date | DATE | NULL | Carrier settlement date |
| status | VARCHAR(20) | NOT NULL, DEFAULT 'pending', CHECK (status IN ('pending', 'collected', 'settled', 'disputed', 'written_off')) | Settlement status |
| ledger_entry_id | UUID | NULL, REFERENCES ledger_entries(id) | Credit ledger entry |
| discrepancy_kurus | INTEGER | NULL | Amount discrepancy |
| discrepancy_reason | VARCHAR(500) | NULL | Discrepancy explanation |
| settled_at | TIMESTAMPTZ | NULL | Settlement timestamp |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |
| updated_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last update timestamp |

**Indexes**:

- `idx_cod_settlements_organization_id` ON (organization_id)
- `idx_cod_settlements_shipment_id` ON (shipment_id)
- `idx_cod_settlements_status` ON (status)
- `idx_cod_settlements_carrier` ON (provider_code, carrier_settlement_id)

---

### idempotency_records

Idempotency key storage.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| organization_id | UUID | NOT NULL, REFERENCES organizations(id) | Parent organization |
| idempotency_key | VARCHAR(100) | NOT NULL | Client-provided key |
| endpoint | VARCHAR(100) | NOT NULL | API endpoint |
| request_hash | VARCHAR(64) | NOT NULL | SHA-256 of request body |
| response_status | INTEGER | NOT NULL | HTTP response status |
| response_body | JSONB | NOT NULL | Response body |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |
| expires_at | TIMESTAMPTZ | NOT NULL | Record expiration |

**Indexes**:

- `idx_idempotency_records_lookup` UNIQUE ON (organization_id, idempotency_key, endpoint)
- `idx_idempotency_records_expires` ON (expires_at)

---

### audit_logs

System audit trail.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| organization_id | UUID | NULL, REFERENCES organizations(id) | Organization (NULL for system) |
| actor_id | UUID | NULL | Actor identifier |
| actor_type | VARCHAR(20) | NOT NULL, CHECK (actor_type IN ('member', 'api_key', 'system', 'provider')) | Actor type |
| action | VARCHAR(100) | NOT NULL | Action performed |
| resource_type | VARCHAR(50) | NOT NULL | Resource type |
| resource_id | UUID | NULL | Resource identifier |
| old_value | JSONB | NULL | Previous state |
| new_value | JSONB | NULL | New state |
| ip_address | INET | NULL | Client IP |
| user_agent | VARCHAR(500) | NULL | Client user agent |
| trace_id | VARCHAR(64) | NULL | Request trace ID |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Timestamp |

**Indexes**:

- `idx_audit_logs_organization_id` ON (organization_id)
- `idx_audit_logs_actor` ON (actor_type, actor_id)
- `idx_audit_logs_resource` ON (resource_type, resource_id)
- `idx_audit_logs_created_at` ON (created_at)
- `idx_audit_logs_trace_id` ON (trace_id)

**Partitioning**: Consider partitioning by created_at (monthly) for large-scale deployments.

---

### provider_api_logs

Provider API call logs.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| organization_id | UUID | NULL, REFERENCES organizations(id) | Organization (NULL for system) |
| provider_code | VARCHAR(50) | NOT NULL | Provider code |
| operation | VARCHAR(100) | NOT NULL | Operation name |
| request_method | VARCHAR(10) | NOT NULL | HTTP method |
| request_url | VARCHAR(500) | NOT NULL | Request URL |
| request_headers | JSONB | NULL | Request headers (masked) |
| request_body_preview | VARCHAR(2000) | NULL | Request body (masked, truncated) |
| response_status | INTEGER | NULL | HTTP response status |
| response_headers | JSONB | NULL | Response headers |
| response_body_preview | VARCHAR(2000) | NULL | Response body (masked, truncated) |
| latency_ms | INTEGER | NOT NULL | Response time |
| success | BOOLEAN | NOT NULL | Operation success |
| error_code | VARCHAR(100) | NULL | Error code |
| error_message | VARCHAR(500) | NULL | Error message |
| trace_id | VARCHAR(64) | NULL | Request trace ID |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Timestamp |

**Indexes**:

- `idx_provider_api_logs_provider` ON (provider_code)
- `idx_provider_api_logs_created_at` ON (created_at)
- `idx_provider_api_logs_trace_id` ON (trace_id)
- `idx_provider_api_logs_success` ON (success, created_at)

**Partitioning**: Partition by created_at (daily) and set retention policy (30 days).

---

### outbox

Transactional outbox for event publishing.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| id | UUID | PRIMARY KEY, DEFAULT gen_random_uuid() | Unique identifier |
| aggregate_type | VARCHAR(50) | NOT NULL | Aggregate type |
| aggregate_id | UUID | NOT NULL | Aggregate identifier |
| event_type | VARCHAR(100) | NOT NULL | Event type |
| payload | JSONB | NOT NULL | Event payload |
| published_at | TIMESTAMPTZ | NULL | Publication timestamp |
| retry_count | INTEGER | NOT NULL, DEFAULT 0 | Retry attempts |
| created_at | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |

**Indexes**:

- `idx_outbox_unpublished` ON (created_at) WHERE published_at IS NULL
- `idx_outbox_aggregate` ON (aggregate_type, aggregate_id)

---

## Views

### v_wallet_balances

Computed wallet balances from ledger.

```sql
CREATE VIEW v_wallet_balances AS
SELECT 
    organization_id,
    COALESCE(SUM(CASE WHEN entry_type = 'credit' THEN amount_kurus ELSE 0 END), 0) -
    COALESCE(SUM(CASE WHEN entry_type = 'debit' THEN amount_kurus ELSE 0 END), 0) AS balance_kurus,
    MAX(created_at) AS last_transaction_at
FROM ledger_entries
GROUP BY organization_id;
```

### v_provider_health

Provider health metrics (last 1 hour).

```sql
CREATE VIEW v_provider_health AS
SELECT 
    provider_code,
    COUNT(*) AS total_calls,
    SUM(CASE WHEN success THEN 1 ELSE 0 END) AS successful_calls,
    ROUND(100.0 * SUM(CASE WHEN success THEN 1 ELSE 0 END) / COUNT(*), 2) AS success_rate,
    ROUND(AVG(latency_ms), 2) AS avg_latency_ms,
    PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency_ms) AS p95_latency_ms,
    MAX(CASE WHEN success THEN created_at END) AS last_success_at
FROM provider_api_logs
WHERE created_at > NOW() - INTERVAL '1 hour'
GROUP BY provider_code;
```

---

## Row-Level Security (RLS)

Enable RLS on all tenant-scoped tables:

```sql
-- Example for shipments table
ALTER TABLE shipments ENABLE ROW LEVEL SECURITY;

CREATE POLICY shipments_tenant_isolation ON shipments
    USING (organization_id = current_setting('app.current_organization_id')::uuid);

CREATE POLICY shipments_tenant_insert ON shipments
    FOR INSERT
    WITH CHECK (organization_id = current_setting('app.current_organization_id')::uuid);
```

Apply similar policies to: addresses, api_keys, quotes, offers, shipments, documents, webhook_endpoints, webhook_subscriptions, webhook_deliveries, ledger_entries, topups, refunds, cod_settlements, provider_accounts.

---

## Triggers

### updated_at Trigger

Auto-update `updated_at` on row modification:

```sql
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply to all tables with updated_at column
CREATE TRIGGER trg_organizations_updated_at
    BEFORE UPDATE ON organizations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
```

### Ledger Immutability Trigger

Prevent UPDATE/DELETE on ledger_entries:

```sql
CREATE OR REPLACE FUNCTION prevent_ledger_modification()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'Ledger entries are immutable. UPDATE and DELETE operations are not allowed.';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_ledger_immutable
    BEFORE UPDATE OR DELETE ON ledger_entries
    FOR EACH ROW EXECUTE FUNCTION prevent_ledger_modification();
```

### Audit Log Trigger

Auto-create audit log entries:

```sql
CREATE OR REPLACE FUNCTION audit_log_trigger()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO audit_logs (
        organization_id,
        actor_id,
        actor_type,
        action,
        resource_type,
        resource_id,
        old_value,
        new_value,
        trace_id
    ) VALUES (
        COALESCE(NEW.organization_id, OLD.organization_id),
        current_setting('app.current_actor_id', true)::uuid,
        COALESCE(current_setting('app.current_actor_type', true), 'system'),
        TG_OP,
        TG_TABLE_NAME,
        COALESCE(NEW.id, OLD.id),
        CASE WHEN TG_OP IN ('UPDATE', 'DELETE') THEN to_jsonb(OLD) END,
        CASE WHEN TG_OP IN ('INSERT', 'UPDATE') THEN to_jsonb(NEW) END,
        current_setting('app.trace_id', true)
    );
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;
```

---

## Data Retention

Per constitution requirements:

| Data Category | Retention | Implementation |
|---------------|-----------|----------------|
| Shipment records | 7 years | No auto-delete; archive after 2 years |
| Payment/billing records | 10 years | No auto-delete |
| Application logs | 30 days | Partition + drop old partitions |
| Audit logs | 2 years | Partition + archive |
| Soft-deleted PII | 30 days | Scheduled hard delete job |
| Webhook delivery logs | 90 days | Partition + drop old partitions |
| Provider API logs | 30 days | Partition + drop old partitions |
| Idempotency records | 24 hours | Scheduled cleanup job |

---

## Migration Strategy

1. **Initial Schema**: Create all tables, indexes, constraints
2. **Seed Data**: Insert system roles, providers, reference data
3. **Enable RLS**: Apply row-level security policies
4. **Create Triggers**: Add audit, immutability, updated_at triggers
5. **Create Views**: Add computed views
6. **Verify Constraints**: Run constraint validation queries

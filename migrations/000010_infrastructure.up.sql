-- Migration: 000010_infrastructure
-- Description: Create idempotency_records, audit_logs, provider_api_logs, and outbox tables
-- Created: 2026-01-22

-- ============================================================================
-- IDEMPOTENCY_RECORDS TABLE
-- Idempotency key storage for preventing duplicate operations
-- ============================================================================
CREATE TABLE idempotency_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    idempotency_key VARCHAR(100) NOT NULL,
    endpoint VARCHAR(100) NOT NULL,
    request_hash VARCHAR(64) NOT NULL,
    response_status INTEGER NOT NULL,
    response_body JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

-- Idempotency lookup index (unique per organization + key + endpoint)
CREATE UNIQUE INDEX idx_idempotency_records_lookup ON idempotency_records(organization_id, idempotency_key, endpoint);

-- Expiration index for cleanup
CREATE INDEX idx_idempotency_records_expires ON idempotency_records(expires_at);

COMMENT ON TABLE idempotency_records IS 'Idempotency key storage';
COMMENT ON COLUMN idempotency_records.idempotency_key IS 'Client-provided idempotency key';
COMMENT ON COLUMN idempotency_records.endpoint IS 'API endpoint (e.g., "POST /shipments")';
COMMENT ON COLUMN idempotency_records.request_hash IS 'SHA-256 hash of canonical request body';
COMMENT ON COLUMN idempotency_records.response_status IS 'HTTP response status code';
COMMENT ON COLUMN idempotency_records.response_body IS 'Cached response body';
COMMENT ON COLUMN idempotency_records.expires_at IS 'Record expiration (24 hours from creation)';

-- ============================================================================
-- AUDIT_LOGS TABLE
-- System audit trail for compliance and debugging
-- ============================================================================
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NULL REFERENCES organizations(id) ON DELETE SET NULL,
    actor_id UUID NULL,
    actor_type VARCHAR(20) NOT NULL,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id UUID NULL,
    old_value JSONB NULL,
    new_value JSONB NULL,
    ip_address INET NULL,
    user_agent VARCHAR(500) NULL,
    trace_id VARCHAR(64) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_audit_logs_actor_type CHECK (actor_type IN ('member', 'api_key', 'system', 'provider'))
);

-- Organization lookup index
CREATE INDEX idx_audit_logs_organization_id ON audit_logs(organization_id);

-- Actor lookup index
CREATE INDEX idx_audit_logs_actor ON audit_logs(actor_type, actor_id);

-- Resource lookup index
CREATE INDEX idx_audit_logs_resource ON audit_logs(resource_type, resource_id);

-- Creation date index for listing
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at);

-- Trace ID lookup index
CREATE INDEX idx_audit_logs_trace_id ON audit_logs(trace_id);

COMMENT ON TABLE audit_logs IS 'System audit trail';
COMMENT ON COLUMN audit_logs.organization_id IS 'Organization (NULL for system-level actions)';
COMMENT ON COLUMN audit_logs.actor_id IS 'Actor identifier (member ID, API key ID, etc.)';
COMMENT ON COLUMN audit_logs.actor_type IS 'Actor type: member, api_key, system, provider';
COMMENT ON COLUMN audit_logs.action IS 'Action performed (e.g., "INSERT", "UPDATE", "DELETE", "login")';
COMMENT ON COLUMN audit_logs.resource_type IS 'Resource type (e.g., "shipment", "member")';
COMMENT ON COLUMN audit_logs.old_value IS 'Previous state (for UPDATE/DELETE)';
COMMENT ON COLUMN audit_logs.new_value IS 'New state (for INSERT/UPDATE)';
COMMENT ON COLUMN audit_logs.trace_id IS 'Request trace ID for correlation';

-- ============================================================================
-- PROVIDER_API_LOGS TABLE
-- Provider API call logs for debugging and monitoring
-- ============================================================================
CREATE TABLE provider_api_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NULL REFERENCES organizations(id) ON DELETE SET NULL,
    provider_code VARCHAR(50) NOT NULL,
    operation VARCHAR(100) NOT NULL,
    request_method VARCHAR(10) NOT NULL,
    request_url VARCHAR(500) NOT NULL,
    request_headers JSONB NULL,
    request_body_preview VARCHAR(2000) NULL,
    response_status INTEGER NULL,
    response_headers JSONB NULL,
    response_body_preview VARCHAR(2000) NULL,
    latency_ms INTEGER NOT NULL,
    success BOOLEAN NOT NULL,
    error_code VARCHAR(100) NULL,
    error_message VARCHAR(500) NULL,
    trace_id VARCHAR(64) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Provider lookup index
CREATE INDEX idx_provider_api_logs_provider ON provider_api_logs(provider_code);

-- Creation date index for listing
CREATE INDEX idx_provider_api_logs_created_at ON provider_api_logs(created_at);

-- Trace ID lookup index
CREATE INDEX idx_provider_api_logs_trace_id ON provider_api_logs(trace_id);

-- Success + date index for error monitoring
CREATE INDEX idx_provider_api_logs_success ON provider_api_logs(success, created_at);

COMMENT ON TABLE provider_api_logs IS 'Provider API call logs';
COMMENT ON COLUMN provider_api_logs.operation IS 'Operation name (e.g., "CreateShipment", "GetTracking")';
COMMENT ON COLUMN provider_api_logs.request_headers IS 'Request headers (sensitive values masked)';
COMMENT ON COLUMN provider_api_logs.request_body_preview IS 'Request body (masked, truncated to 2000 chars)';
COMMENT ON COLUMN provider_api_logs.response_body_preview IS 'Response body (masked, truncated to 2000 chars)';
COMMENT ON COLUMN provider_api_logs.latency_ms IS 'Response time in milliseconds';
COMMENT ON COLUMN provider_api_logs.success IS 'Whether the operation succeeded';

-- ============================================================================
-- OUTBOX TABLE
-- Transactional outbox for reliable event publishing
-- ============================================================================
CREATE TABLE outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type VARCHAR(50) NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    published_at TIMESTAMPTZ NULL,
    retry_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Unpublished events index for polling
CREATE INDEX idx_outbox_unpublished ON outbox(created_at) WHERE published_at IS NULL;

-- Aggregate lookup index
CREATE INDEX idx_outbox_aggregate ON outbox(aggregate_type, aggregate_id);

COMMENT ON TABLE outbox IS 'Transactional outbox for event publishing';
COMMENT ON COLUMN outbox.aggregate_type IS 'Aggregate type (e.g., "shipment", "billing")';
COMMENT ON COLUMN outbox.aggregate_id IS 'Aggregate identifier';
COMMENT ON COLUMN outbox.event_type IS 'Event type (e.g., "shipment.delivered")';
COMMENT ON COLUMN outbox.payload IS 'Event payload as JSON';
COMMENT ON COLUMN outbox.published_at IS 'Publication timestamp (NULL = unpublished)';
COMMENT ON COLUMN outbox.retry_count IS 'Number of publish attempts';

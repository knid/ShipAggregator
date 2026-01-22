-- Migration: 000008_webhooks
-- Description: Create webhook_endpoints, webhook_subscriptions, webhook_events, and webhook_deliveries tables
-- Created: 2026-01-22

-- ============================================================================
-- WEBHOOK_ENDPOINTS TABLE
-- Customer webhook endpoint registrations
-- ============================================================================
CREATE TABLE webhook_endpoints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    url VARCHAR(500) NOT NULL,
    secret_hash VARCHAR(255) NOT NULL,
    description VARCHAR(255) NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Organization lookup index
CREATE INDEX idx_webhook_endpoints_organization_id ON webhook_endpoints(organization_id);

COMMENT ON TABLE webhook_endpoints IS 'Customer webhook endpoint registrations';
COMMENT ON COLUMN webhook_endpoints.url IS 'Webhook URL (HTTPS required in production)';
COMMENT ON COLUMN webhook_endpoints.secret_hash IS 'HMAC secret hash for signature verification';

-- ============================================================================
-- WEBHOOK_SUBSCRIPTIONS TABLE
-- Event type subscriptions for endpoints
-- ============================================================================
CREATE TABLE webhook_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    endpoint_id UUID NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Endpoint lookup index
CREATE INDEX idx_webhook_subscriptions_endpoint_id ON webhook_subscriptions(endpoint_id);

-- Event type lookup index
CREATE INDEX idx_webhook_subscriptions_event_type ON webhook_subscriptions(event_type);

-- Unique subscription per endpoint + event type
CREATE UNIQUE INDEX idx_webhook_subscriptions_endpoint_event ON webhook_subscriptions(endpoint_id, event_type);

COMMENT ON TABLE webhook_subscriptions IS 'Event type subscriptions for endpoints';
COMMENT ON COLUMN webhook_subscriptions.event_type IS 'Event type (e.g., "shipment.delivered", "billing.topup.completed")';

-- ============================================================================
-- WEBHOOK_EVENTS TABLE
-- Generated webhook events
-- ============================================================================
CREATE TABLE webhook_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    event_type VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id UUID NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Organization lookup index
CREATE INDEX idx_webhook_events_organization_id ON webhook_events(organization_id);

-- Event type lookup index
CREATE INDEX idx_webhook_events_event_type ON webhook_events(event_type);

-- Creation date index for listing
CREATE INDEX idx_webhook_events_created_at ON webhook_events(created_at);

COMMENT ON TABLE webhook_events IS 'Generated webhook events';
COMMENT ON COLUMN webhook_events.resource_type IS 'Resource type (shipment, billing, etc.)';
COMMENT ON COLUMN webhook_events.resource_id IS 'Resource identifier';
COMMENT ON COLUMN webhook_events.payload IS 'Event payload as JSON';

-- ============================================================================
-- WEBHOOK_DELIVERIES TABLE
-- Webhook delivery attempts with retry tracking
-- ============================================================================
CREATE TABLE webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES webhook_events(id) ON DELETE CASCADE,
    subscription_id UUID NOT NULL REFERENCES webhook_subscriptions(id) ON DELETE CASCADE,
    endpoint_url VARCHAR(500) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMPTZ NULL,
    next_retry_at TIMESTAMPTZ NULL,
    response_status INTEGER NULL,
    response_body_preview VARCHAR(1000) NULL,
    response_time_ms INTEGER NULL,
    error_message VARCHAR(500) NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_webhook_deliveries_status CHECK (status IN ('pending', 'delivered', 'failed', 'dlq'))
);

-- Event lookup index
CREATE INDEX idx_webhook_deliveries_event_id ON webhook_deliveries(event_id);

-- Subscription lookup index
CREATE INDEX idx_webhook_deliveries_subscription_id ON webhook_deliveries(subscription_id);

-- Status index for filtering
CREATE INDEX idx_webhook_deliveries_status ON webhook_deliveries(status);

-- Next retry index for pending deliveries
CREATE INDEX idx_webhook_deliveries_next_retry ON webhook_deliveries(next_retry_at) WHERE status = 'pending';

COMMENT ON TABLE webhook_deliveries IS 'Webhook delivery attempts';
COMMENT ON COLUMN webhook_deliveries.endpoint_url IS 'Delivery URL (snapshot at time of delivery)';
COMMENT ON COLUMN webhook_deliveries.status IS 'Delivery status: pending, delivered, failed, dlq';
COMMENT ON COLUMN webhook_deliveries.attempts IS 'Number of delivery attempts';
COMMENT ON COLUMN webhook_deliveries.next_retry_at IS 'Next retry timestamp (for pending deliveries)';
COMMENT ON COLUMN webhook_deliveries.response_body_preview IS 'Response body (truncated to 1000 chars)';
COMMENT ON COLUMN webhook_deliveries.response_time_ms IS 'Response time in milliseconds';

-- ============================================================================
-- ROW-LEVEL SECURITY
-- Enable RLS on webhook tables
-- ============================================================================
ALTER TABLE webhook_endpoints ENABLE ROW LEVEL SECURITY;

CREATE POLICY webhook_endpoints_tenant_isolation ON webhook_endpoints
    USING (organization_id = current_setting('app.current_organization_id', true)::uuid);

CREATE POLICY webhook_endpoints_tenant_insert ON webhook_endpoints
    FOR INSERT
    WITH CHECK (organization_id = current_setting('app.current_organization_id', true)::uuid);

-- Subscriptions inherit tenant isolation through endpoints
ALTER TABLE webhook_subscriptions ENABLE ROW LEVEL SECURITY;

CREATE POLICY webhook_subscriptions_tenant_isolation ON webhook_subscriptions
    USING (
        endpoint_id IN (
            SELECT id FROM webhook_endpoints 
            WHERE organization_id = current_setting('app.current_organization_id', true)::uuid
        )
    );

CREATE POLICY webhook_subscriptions_tenant_insert ON webhook_subscriptions
    FOR INSERT
    WITH CHECK (
        endpoint_id IN (
            SELECT id FROM webhook_endpoints 
            WHERE organization_id = current_setting('app.current_organization_id', true)::uuid
        )
    );

ALTER TABLE webhook_events ENABLE ROW LEVEL SECURITY;

CREATE POLICY webhook_events_tenant_isolation ON webhook_events
    USING (organization_id = current_setting('app.current_organization_id', true)::uuid);

CREATE POLICY webhook_events_tenant_insert ON webhook_events
    FOR INSERT
    WITH CHECK (organization_id = current_setting('app.current_organization_id', true)::uuid);

-- Deliveries inherit tenant isolation through events
ALTER TABLE webhook_deliveries ENABLE ROW LEVEL SECURITY;

CREATE POLICY webhook_deliveries_tenant_isolation ON webhook_deliveries
    USING (
        event_id IN (
            SELECT id FROM webhook_events 
            WHERE organization_id = current_setting('app.current_organization_id', true)::uuid
        )
    );

CREATE POLICY webhook_deliveries_tenant_insert ON webhook_deliveries
    FOR INSERT
    WITH CHECK (
        event_id IN (
            SELECT id FROM webhook_events 
            WHERE organization_id = current_setting('app.current_organization_id', true)::uuid
        )
    );

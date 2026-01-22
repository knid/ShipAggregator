-- Migration: 000002_api_keys
-- Description: Create api_keys table for API authentication credentials
-- Created: 2026-01-22

-- ============================================================================
-- API_KEYS TABLE
-- API authentication credentials with scopes, rate limits, and rotation support.
-- ============================================================================
CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    key_prefix VARCHAR(8) NOT NULL,
    key_hash VARCHAR(255) NOT NULL,
    previous_key_hash VARCHAR(255) NULL,
    previous_key_expires_at TIMESTAMPTZ NULL,
    scopes JSONB NOT NULL DEFAULT '[]',
    rate_limit_per_minute INTEGER NOT NULL DEFAULT 100,
    ip_allowlist JSONB NOT NULL DEFAULT '[]',
    expires_at TIMESTAMPTZ NULL,
    last_used_at TIMESTAMPTZ NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ NULL
);

-- Key hash lookup index (for authentication, only non-revoked keys)
CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash) WHERE revoked_at IS NULL;

-- Previous key hash lookup index (for rotation grace period)
CREATE INDEX idx_api_keys_previous_key_hash ON api_keys(previous_key_hash) WHERE previous_key_hash IS NOT NULL;

-- Organization lookup index
CREATE INDEX idx_api_keys_organization_id ON api_keys(organization_id);

-- Key prefix lookup index (for identification in logs/UI)
CREATE INDEX idx_api_keys_key_prefix ON api_keys(key_prefix);

COMMENT ON TABLE api_keys IS 'API authentication credentials with scopes and rate limits';
COMMENT ON COLUMN api_keys.key_prefix IS 'First 8 characters of the key for identification';
COMMENT ON COLUMN api_keys.key_hash IS 'SHA-256 hash of the full API key';
COMMENT ON COLUMN api_keys.previous_key_hash IS 'Previous key hash during rotation grace period';
COMMENT ON COLUMN api_keys.previous_key_expires_at IS 'When the previous key becomes invalid (24-hour grace period)';
COMMENT ON COLUMN api_keys.scopes IS 'Array of scope strings defining permissions';
COMMENT ON COLUMN api_keys.rate_limit_per_minute IS 'Maximum requests per minute for this key';
COMMENT ON COLUMN api_keys.ip_allowlist IS 'Array of allowed IP addresses/CIDRs (empty = all allowed)';
COMMENT ON COLUMN api_keys.expires_at IS 'Key expiration timestamp (NULL = no expiry, max 90 days recommended)';
COMMENT ON COLUMN api_keys.revoked_at IS 'Timestamp when key was revoked';

-- ============================================================================
-- ROW-LEVEL SECURITY
-- Enable RLS on api_keys table
-- ============================================================================
ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;

CREATE POLICY api_keys_tenant_isolation ON api_keys
    USING (organization_id = current_setting('app.current_organization_id', true)::uuid);

CREATE POLICY api_keys_tenant_insert ON api_keys
    FOR INSERT
    WITH CHECK (organization_id = current_setting('app.current_organization_id', true)::uuid);

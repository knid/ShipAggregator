-- Migration: 000004_addresses
-- Description: Create addresses table for saved organization addresses
-- Created: 2026-01-22

-- ============================================================================
-- ADDRESSES TABLE
-- Saved addresses for organizations with Turkish location references.
-- ============================================================================
CREATE TABLE addresses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    label VARCHAR(100) NULL,
    contact_name VARCHAR(255) NOT NULL,
    company_name VARCHAR(255) NULL,
    phone VARCHAR(20) NOT NULL,
    email VARCHAR(255) NULL,
    street_line_1 VARCHAR(500) NOT NULL,
    street_line_2 VARCHAR(500) NULL,
    neighborhood_code VARCHAR(30) NOT NULL REFERENCES ref_neighborhoods(code),
    district_code VARCHAR(20) NOT NULL REFERENCES ref_districts(code),
    city_code VARCHAR(10) NOT NULL REFERENCES ref_cities(code),
    postal_code VARCHAR(10) NULL,
    latitude DECIMAL(10, 8) NULL,
    longitude DECIMAL(11, 8) NULL,
    is_default_sender BOOLEAN NOT NULL DEFAULT FALSE,
    is_default_recipient BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

-- Organization lookup index (only non-deleted addresses)
CREATE INDEX idx_addresses_organization_id ON addresses(organization_id) WHERE deleted_at IS NULL;

-- City lookup index
CREATE INDEX idx_addresses_city_code ON addresses(city_code);

-- District lookup index
CREATE INDEX idx_addresses_district_code ON addresses(district_code);

COMMENT ON TABLE addresses IS 'Saved addresses for organizations';
COMMENT ON COLUMN addresses.label IS 'User-defined label (e.g., "Warehouse", "HQ")';
COMMENT ON COLUMN addresses.phone IS 'Phone number in E.164 format (e.g., +905321234567)';
COMMENT ON COLUMN addresses.neighborhood_code IS 'Reference to Turkish neighborhood (mahalle)';
COMMENT ON COLUMN addresses.is_default_sender IS 'Default sender address for this organization';
COMMENT ON COLUMN addresses.is_default_recipient IS 'Default recipient address for this organization';

-- ============================================================================
-- ROW-LEVEL SECURITY
-- Enable RLS on addresses table
-- ============================================================================
ALTER TABLE addresses ENABLE ROW LEVEL SECURITY;

CREATE POLICY addresses_tenant_isolation ON addresses
    USING (organization_id = current_setting('app.current_organization_id', true)::uuid);

CREATE POLICY addresses_tenant_insert ON addresses
    FOR INSERT
    WITH CHECK (organization_id = current_setting('app.current_organization_id', true)::uuid);

-- Migration: 000005_providers
-- Description: Create providers, provider_services, provider_accounts, and marketplace_rate_plans tables
-- Created: 2026-01-22

-- ============================================================================
-- PROVIDERS TABLE
-- Integrated carrier providers (Aras, Yurtiçi, Sürat, PTT, Hepsijet, MNG, Kolaygelsin, Sendeo, Pakettaxi)
-- ============================================================================
CREATE TABLE providers (
    code VARCHAR(50) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    logo_url VARCHAR(500) NULL,
    website VARCHAR(255) NULL,
    support_email VARCHAR(255) NULL,
    support_phone VARCHAR(50) NULL,
    api_type VARCHAR(20) NOT NULL,
    api_base_url VARCHAR(500) NOT NULL,
    api_sandbox_url VARCHAR(500) NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    capabilities JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_providers_api_type CHECK (api_type IN ('soap', 'rest'))
);

COMMENT ON TABLE providers IS 'Integrated carrier providers';
COMMENT ON COLUMN providers.code IS 'Provider code (e.g., "aras", "hepsijet")';
COMMENT ON COLUMN providers.api_type IS 'API protocol type: soap or rest';
COMMENT ON COLUMN providers.capabilities IS 'Capability flags as JSON (e.g., {"cod": true, "returns": true})';

-- ============================================================================
-- PROVIDER_SERVICES TABLE
-- Service levels offered by providers (e.g., standard, express, same-day)
-- ============================================================================
CREATE TABLE provider_services (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_code VARCHAR(50) NOT NULL REFERENCES providers(code) ON DELETE CASCADE,
    service_code VARCHAR(50) NOT NULL,
    name VARCHAR(100) NOT NULL,
    description VARCHAR(500) NULL,
    estimated_days_min INTEGER NOT NULL,
    estimated_days_max INTEGER NOT NULL,
    supports_cod BOOLEAN NOT NULL DEFAULT FALSE,
    supports_insurance BOOLEAN NOT NULL DEFAULT FALSE,
    supports_return BOOLEAN NOT NULL DEFAULT FALSE,
    max_weight_grams INTEGER NOT NULL,
    max_length_cm INTEGER NOT NULL,
    max_width_cm INTEGER NOT NULL,
    max_height_cm INTEGER NOT NULL,
    label_formats JSONB NOT NULL DEFAULT '["pdf"]',
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Provider lookup index
CREATE INDEX idx_provider_services_provider_code ON provider_services(provider_code);

-- Unique service code per provider
CREATE UNIQUE INDEX idx_provider_services_provider_service ON provider_services(provider_code, service_code);

COMMENT ON TABLE provider_services IS 'Service levels offered by providers';
COMMENT ON COLUMN provider_services.service_code IS 'Provider-specific service code';
COMMENT ON COLUMN provider_services.estimated_days_min IS 'Minimum delivery days';
COMMENT ON COLUMN provider_services.estimated_days_max IS 'Maximum delivery days';
COMMENT ON COLUMN provider_services.label_formats IS 'Supported label formats (e.g., ["pdf", "zpl"])';

-- ============================================================================
-- PROVIDER_ACCOUNTS TABLE
-- BYOA (Bring Your Own Agreement) credentials for customer-owned carrier accounts
-- ============================================================================
CREATE TABLE provider_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    provider_code VARCHAR(50) NOT NULL REFERENCES providers(code),
    name VARCHAR(100) NOT NULL,
    credentials_encrypted BYTEA NOT NULL,
    credentials_iv BYTEA NOT NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'pending_verification',
    verification_error VARCHAR(500) NULL,
    verified_at TIMESTAMPTZ NULL,
    last_used_at TIMESTAMPTZ NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL,
    
    CONSTRAINT chk_provider_accounts_status CHECK (status IN ('pending_verification', 'verified', 'verification_failed', 'suspended'))
);

-- Organization lookup index (only non-deleted accounts)
CREATE INDEX idx_provider_accounts_organization_id ON provider_accounts(organization_id) WHERE deleted_at IS NULL;

-- Organization + provider lookup index
CREATE INDEX idx_provider_accounts_org_provider ON provider_accounts(organization_id, provider_code) WHERE deleted_at IS NULL;

COMMENT ON TABLE provider_accounts IS 'BYOA credentials for customer-owned carrier accounts';
COMMENT ON COLUMN provider_accounts.credentials_encrypted IS 'AES-256-GCM encrypted credentials blob';
COMMENT ON COLUMN provider_accounts.credentials_iv IS 'Encryption initialization vector';
COMMENT ON COLUMN provider_accounts.status IS 'Account verification status';
COMMENT ON COLUMN provider_accounts.is_default IS 'Default account for this provider within the organization';

-- ============================================================================
-- MARKETPLACE_RATE_PLANS TABLE
-- Internal pricing configuration for marketplace model
-- ============================================================================
CREATE TABLE marketplace_rate_plans (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_code VARCHAR(50) NOT NULL REFERENCES providers(code),
    service_code VARCHAR(50) NOT NULL,
    name VARCHAR(100) NOT NULL,
    base_price_kurus INTEGER NOT NULL,
    price_per_kg_kurus INTEGER NOT NULL DEFAULT 0,
    price_per_desi_kurus INTEGER NOT NULL DEFAULT 0,
    fuel_surcharge_percent DECIMAL(5, 2) NOT NULL DEFAULT 0,
    cod_fee_percent DECIMAL(5, 2) NOT NULL DEFAULT 0,
    cod_fee_min_kurus INTEGER NOT NULL DEFAULT 0,
    vat_percent DECIMAL(5, 2) NOT NULL DEFAULT 20.00,
    effective_from DATE NOT NULL,
    effective_to DATE NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Provider + service lookup index
CREATE INDEX idx_marketplace_rate_plans_provider_service ON marketplace_rate_plans(provider_code, service_code);

-- Effective date range index
CREATE INDEX idx_marketplace_rate_plans_effective ON marketplace_rate_plans(effective_from, effective_to);

COMMENT ON TABLE marketplace_rate_plans IS 'Internal pricing configuration for marketplace model';
COMMENT ON COLUMN marketplace_rate_plans.base_price_kurus IS 'Base price in kuruş (1 TRY = 100 kuruş)';
COMMENT ON COLUMN marketplace_rate_plans.price_per_kg_kurus IS 'Per-kilogram surcharge in kuruş';
COMMENT ON COLUMN marketplace_rate_plans.price_per_desi_kurus IS 'Per-desi (volumetric) surcharge in kuruş';
COMMENT ON COLUMN marketplace_rate_plans.fuel_surcharge_percent IS 'Fuel surcharge percentage';
COMMENT ON COLUMN marketplace_rate_plans.cod_fee_percent IS 'COD fee percentage of COD amount';
COMMENT ON COLUMN marketplace_rate_plans.cod_fee_min_kurus IS 'Minimum COD fee in kuruş';
COMMENT ON COLUMN marketplace_rate_plans.vat_percent IS 'VAT percentage (default 20% for Turkey)';
COMMENT ON COLUMN marketplace_rate_plans.effective_from IS 'Rate effective start date';
COMMENT ON COLUMN marketplace_rate_plans.effective_to IS 'Rate effective end date (NULL = current)';

-- ============================================================================
-- ROW-LEVEL SECURITY
-- Enable RLS on provider_accounts table
-- ============================================================================
ALTER TABLE provider_accounts ENABLE ROW LEVEL SECURITY;

CREATE POLICY provider_accounts_tenant_isolation ON provider_accounts
    USING (organization_id = current_setting('app.current_organization_id', true)::uuid);

CREATE POLICY provider_accounts_tenant_insert ON provider_accounts
    FOR INSERT
    WITH CHECK (organization_id = current_setting('app.current_organization_id', true)::uuid);

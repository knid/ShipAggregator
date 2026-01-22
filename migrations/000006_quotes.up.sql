-- Migration: 000006_quotes
-- Description: Create quotes and offers tables for rate requests
-- Created: 2026-01-22

-- ============================================================================
-- QUOTES TABLE
-- Rate request records with address snapshots and parcel details
-- ============================================================================
CREATE TABLE quotes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    idempotency_key VARCHAR(100) NOT NULL,
    origin_address JSONB NOT NULL,
    destination_address JSONB NOT NULL,
    parcel JSONB NOT NULL,
    cod_amount_kurus INTEGER NULL,
    declared_value_kurus INTEGER NULL,
    use_byoa BOOLEAN NOT NULL DEFAULT FALSE,
    provider_account_ids JSONB NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_quotes_status CHECK (status IN ('pending', 'completed', 'failed', 'expired'))
);

-- Organization lookup index
CREATE INDEX idx_quotes_organization_id ON quotes(organization_id);

-- Idempotency lookup index (unique per organization)
CREATE UNIQUE INDEX idx_quotes_idempotency ON quotes(organization_id, idempotency_key);

-- Expiration index for cleanup
CREATE INDEX idx_quotes_expires_at ON quotes(expires_at);

COMMENT ON TABLE quotes IS 'Rate request records';
COMMENT ON COLUMN quotes.idempotency_key IS 'Client-provided idempotency key';
COMMENT ON COLUMN quotes.origin_address IS 'Origin address snapshot as JSON';
COMMENT ON COLUMN quotes.destination_address IS 'Destination address snapshot as JSON';
COMMENT ON COLUMN quotes.parcel IS 'Parcel details (weight, dimensions) as JSON';
COMMENT ON COLUMN quotes.cod_amount_kurus IS 'COD amount in kuruş (if applicable)';
COMMENT ON COLUMN quotes.declared_value_kurus IS 'Declared value for insurance in kuruş';
COMMENT ON COLUMN quotes.use_byoa IS 'Use BYOA (Bring Your Own Agreement) accounts';
COMMENT ON COLUMN quotes.provider_account_ids IS 'Specific BYOA account IDs to use (JSON array)';
COMMENT ON COLUMN quotes.expires_at IS 'Quote expiration timestamp (typically 30 minutes)';

-- ============================================================================
-- OFFERS TABLE
-- Price quotes from providers for a given quote request
-- ============================================================================
CREATE TABLE offers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    quote_id UUID NOT NULL REFERENCES quotes(id) ON DELETE CASCADE,
    provider_code VARCHAR(50) NOT NULL REFERENCES providers(code),
    service_code VARCHAR(50) NOT NULL,
    provider_account_id UUID NULL REFERENCES provider_accounts(id),
    is_marketplace BOOLEAN NOT NULL,
    base_price_kurus INTEGER NOT NULL,
    fuel_surcharge_kurus INTEGER NOT NULL DEFAULT 0,
    cod_fee_kurus INTEGER NOT NULL DEFAULT 0,
    insurance_fee_kurus INTEGER NOT NULL DEFAULT 0,
    other_fees_kurus INTEGER NOT NULL DEFAULT 0,
    subtotal_kurus INTEGER NOT NULL,
    vat_kurus INTEGER NOT NULL,
    total_price_kurus INTEGER NOT NULL,
    cost_price_kurus INTEGER NULL,
    margin_kurus INTEGER NULL,
    estimated_days_min INTEGER NOT NULL,
    estimated_days_max INTEGER NOT NULL,
    provider_quote_id VARCHAR(100) NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Quote lookup index
CREATE INDEX idx_offers_quote_id ON offers(quote_id);

-- Provider lookup index
CREATE INDEX idx_offers_provider_code ON offers(provider_code);

COMMENT ON TABLE offers IS 'Price quotes from providers';
COMMENT ON COLUMN offers.is_marketplace IS 'TRUE for marketplace pricing, FALSE for BYOA';
COMMENT ON COLUMN offers.base_price_kurus IS 'Base shipping price in kuruş';
COMMENT ON COLUMN offers.fuel_surcharge_kurus IS 'Fuel surcharge in kuruş';
COMMENT ON COLUMN offers.cod_fee_kurus IS 'COD handling fee in kuruş';
COMMENT ON COLUMN offers.insurance_fee_kurus IS 'Insurance fee in kuruş';
COMMENT ON COLUMN offers.other_fees_kurus IS 'Other fees in kuruş';
COMMENT ON COLUMN offers.subtotal_kurus IS 'Subtotal before VAT in kuruş';
COMMENT ON COLUMN offers.vat_kurus IS 'VAT amount in kuruş';
COMMENT ON COLUMN offers.total_price_kurus IS 'Total price including VAT in kuruş';
COMMENT ON COLUMN offers.cost_price_kurus IS 'Our cost (marketplace only) in kuruş';
COMMENT ON COLUMN offers.margin_kurus IS 'Margin (marketplace only) in kuruş';
COMMENT ON COLUMN offers.provider_quote_id IS 'Provider''s quote reference ID';

-- ============================================================================
-- ROW-LEVEL SECURITY
-- Enable RLS on quotes and offers tables
-- ============================================================================
ALTER TABLE quotes ENABLE ROW LEVEL SECURITY;

CREATE POLICY quotes_tenant_isolation ON quotes
    USING (organization_id = current_setting('app.current_organization_id', true)::uuid);

CREATE POLICY quotes_tenant_insert ON quotes
    FOR INSERT
    WITH CHECK (organization_id = current_setting('app.current_organization_id', true)::uuid);

-- Offers inherit tenant isolation through quotes
ALTER TABLE offers ENABLE ROW LEVEL SECURITY;

CREATE POLICY offers_tenant_isolation ON offers
    USING (
        quote_id IN (
            SELECT id FROM quotes 
            WHERE organization_id = current_setting('app.current_organization_id', true)::uuid
        )
    );

CREATE POLICY offers_tenant_insert ON offers
    FOR INSERT
    WITH CHECK (
        quote_id IN (
            SELECT id FROM quotes 
            WHERE organization_id = current_setting('app.current_organization_id', true)::uuid
        )
    );

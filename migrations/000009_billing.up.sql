-- Migration: 000009_billing
-- Description: Create ledger_entries, topups, refunds, and cod_settlements tables
-- Created: 2026-01-22

-- ============================================================================
-- LEDGER_ENTRIES TABLE
-- Immutable financial transaction log (append-only)
-- ============================================================================
CREATE TABLE ledger_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    entry_type VARCHAR(10) NOT NULL,
    amount_kurus INTEGER NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'TRY',
    balance_after_kurus INTEGER NOT NULL,
    reference_type VARCHAR(50) NOT NULL,
    reference_id UUID NOT NULL,
    description VARCHAR(500) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    actor_id UUID NULL,
    actor_type VARCHAR(20) NOT NULL DEFAULT 'system',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_ledger_entries_entry_type CHECK (entry_type IN ('credit', 'debit')),
    CONSTRAINT chk_ledger_entries_amount CHECK (amount_kurus > 0),
    CONSTRAINT chk_ledger_entries_actor_type CHECK (actor_type IN ('member', 'api_key', 'system'))
);

-- Organization lookup index
CREATE INDEX idx_ledger_entries_organization_id ON ledger_entries(organization_id);

-- Reference lookup index
CREATE INDEX idx_ledger_entries_reference ON ledger_entries(reference_type, reference_id);

-- Creation date index for listing
CREATE INDEX idx_ledger_entries_created_at ON ledger_entries(created_at);

-- Composite index for organization timeline queries
CREATE INDEX idx_ledger_entries_org_created ON ledger_entries(organization_id, created_at DESC);

COMMENT ON TABLE ledger_entries IS 'Immutable financial transaction log';
COMMENT ON COLUMN ledger_entries.entry_type IS 'Entry type: credit (add funds) or debit (subtract funds)';
COMMENT ON COLUMN ledger_entries.amount_kurus IS 'Amount in kuruş (always positive)';
COMMENT ON COLUMN ledger_entries.balance_after_kurus IS 'Running balance after this entry';
COMMENT ON COLUMN ledger_entries.reference_type IS 'Reference type (topup, shipment, refund, cod_settlement, etc.)';
COMMENT ON COLUMN ledger_entries.reference_id IS 'Reference record ID';
COMMENT ON COLUMN ledger_entries.actor_id IS 'User/API key that created this entry';
COMMENT ON COLUMN ledger_entries.actor_type IS 'Actor type: member, api_key, or system';

-- ============================================================================
-- TOPUPS TABLE
-- Wallet funding requests via payment gateway
-- ============================================================================
CREATE TABLE topups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    idempotency_key VARCHAR(100) NOT NULL,
    amount_kurus INTEGER NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'TRY',
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    payment_provider VARCHAR(20) NOT NULL DEFAULT 'iyzico',
    payment_id VARCHAR(100) NULL,
    payment_url VARCHAR(500) NULL,
    payment_token VARCHAR(255) NULL,
    ledger_entry_id UUID NULL REFERENCES ledger_entries(id),
    failure_reason VARCHAR(500) NULL,
    completed_at TIMESTAMPTZ NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_topups_amount CHECK (amount_kurus > 0),
    CONSTRAINT chk_topups_status CHECK (status IN ('pending', 'processing', 'completed', 'failed', 'cancelled'))
);

-- Organization lookup index
CREATE INDEX idx_topups_organization_id ON topups(organization_id);

-- Idempotency lookup index (unique per organization)
CREATE UNIQUE INDEX idx_topups_idempotency ON topups(organization_id, idempotency_key);

-- Payment provider lookup index
CREATE INDEX idx_topups_payment_id ON topups(payment_provider, payment_id);

-- Status index for filtering
CREATE INDEX idx_topups_status ON topups(status);

COMMENT ON TABLE topups IS 'Wallet funding requests';
COMMENT ON COLUMN topups.idempotency_key IS 'Client-provided idempotency key';
COMMENT ON COLUMN topups.amount_kurus IS 'Top-up amount in kuruş';
COMMENT ON COLUMN topups.payment_provider IS 'Payment provider (default: iyzico)';
COMMENT ON COLUMN topups.payment_id IS 'Provider''s payment ID';
COMMENT ON COLUMN topups.payment_url IS 'Payment form URL for redirect';
COMMENT ON COLUMN topups.payment_token IS 'Provider token for callback verification';
COMMENT ON COLUMN topups.ledger_entry_id IS 'Created ledger entry (after completion)';
COMMENT ON COLUMN topups.expires_at IS 'Payment expiration timestamp';

-- ============================================================================
-- REFUNDS TABLE
-- Refund records for cancelled shipments or disputes
-- ============================================================================
CREATE TABLE refunds (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    original_ledger_entry_id UUID NOT NULL REFERENCES ledger_entries(id),
    refund_ledger_entry_id UUID NULL REFERENCES ledger_entries(id),
    amount_kurus INTEGER NOT NULL,
    reason VARCHAR(500) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    approved_by UUID NULL REFERENCES members(id),
    approved_at TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_refunds_amount CHECK (amount_kurus > 0),
    CONSTRAINT chk_refunds_status CHECK (status IN ('pending', 'approved', 'completed', 'rejected'))
);

-- Organization lookup index
CREATE INDEX idx_refunds_organization_id ON refunds(organization_id);

-- Original entry lookup index
CREATE INDEX idx_refunds_original_entry ON refunds(original_ledger_entry_id);

-- Status index for filtering
CREATE INDEX idx_refunds_status ON refunds(status);

COMMENT ON TABLE refunds IS 'Refund records';
COMMENT ON COLUMN refunds.original_ledger_entry_id IS 'Original charge ledger entry';
COMMENT ON COLUMN refunds.refund_ledger_entry_id IS 'Refund credit ledger entry (after completion)';
COMMENT ON COLUMN refunds.amount_kurus IS 'Refund amount in kuruş';
COMMENT ON COLUMN refunds.reason IS 'Refund reason';
COMMENT ON COLUMN refunds.approved_by IS 'Member who approved the refund';

-- ============================================================================
-- COD_SETTLEMENTS TABLE
-- COD (Cash on Delivery) collection tracking
-- ============================================================================
CREATE TABLE cod_settlements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    shipment_id UUID NOT NULL REFERENCES shipments(id),
    provider_code VARCHAR(50) NOT NULL,
    expected_amount_kurus INTEGER NOT NULL,
    actual_amount_kurus INTEGER NULL,
    carrier_settlement_id VARCHAR(100) NULL,
    carrier_settlement_date DATE NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    ledger_entry_id UUID NULL REFERENCES ledger_entries(id),
    discrepancy_kurus INTEGER NULL,
    discrepancy_reason VARCHAR(500) NULL,
    settled_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_cod_settlements_status CHECK (status IN ('pending', 'collected', 'settled', 'disputed', 'written_off'))
);

-- Organization lookup index
CREATE INDEX idx_cod_settlements_organization_id ON cod_settlements(organization_id);

-- Shipment lookup index
CREATE INDEX idx_cod_settlements_shipment_id ON cod_settlements(shipment_id);

-- Status index for filtering
CREATE INDEX idx_cod_settlements_status ON cod_settlements(status);

-- Carrier settlement lookup index
CREATE INDEX idx_cod_settlements_carrier ON cod_settlements(provider_code, carrier_settlement_id);

COMMENT ON TABLE cod_settlements IS 'COD collection tracking';
COMMENT ON COLUMN cod_settlements.expected_amount_kurus IS 'Expected COD amount in kuruş';
COMMENT ON COLUMN cod_settlements.actual_amount_kurus IS 'Actual collected amount in kuruş';
COMMENT ON COLUMN cod_settlements.carrier_settlement_id IS 'Carrier''s settlement reference';
COMMENT ON COLUMN cod_settlements.carrier_settlement_date IS 'Carrier settlement date';
COMMENT ON COLUMN cod_settlements.ledger_entry_id IS 'Credit ledger entry (after settlement)';
COMMENT ON COLUMN cod_settlements.discrepancy_kurus IS 'Amount discrepancy (actual - expected)';
COMMENT ON COLUMN cod_settlements.discrepancy_reason IS 'Explanation for discrepancy';

-- ============================================================================
-- ROW-LEVEL SECURITY
-- Enable RLS on billing tables
-- ============================================================================
ALTER TABLE ledger_entries ENABLE ROW LEVEL SECURITY;

CREATE POLICY ledger_entries_tenant_isolation ON ledger_entries
    USING (organization_id = current_setting('app.current_organization_id', true)::uuid);

CREATE POLICY ledger_entries_tenant_insert ON ledger_entries
    FOR INSERT
    WITH CHECK (organization_id = current_setting('app.current_organization_id', true)::uuid);

ALTER TABLE topups ENABLE ROW LEVEL SECURITY;

CREATE POLICY topups_tenant_isolation ON topups
    USING (organization_id = current_setting('app.current_organization_id', true)::uuid);

CREATE POLICY topups_tenant_insert ON topups
    FOR INSERT
    WITH CHECK (organization_id = current_setting('app.current_organization_id', true)::uuid);

ALTER TABLE refunds ENABLE ROW LEVEL SECURITY;

CREATE POLICY refunds_tenant_isolation ON refunds
    USING (organization_id = current_setting('app.current_organization_id', true)::uuid);

CREATE POLICY refunds_tenant_insert ON refunds
    FOR INSERT
    WITH CHECK (organization_id = current_setting('app.current_organization_id', true)::uuid);

ALTER TABLE cod_settlements ENABLE ROW LEVEL SECURITY;

CREATE POLICY cod_settlements_tenant_isolation ON cod_settlements
    USING (organization_id = current_setting('app.current_organization_id', true)::uuid);

CREATE POLICY cod_settlements_tenant_insert ON cod_settlements
    FOR INSERT
    WITH CHECK (organization_id = current_setting('app.current_organization_id', true)::uuid);

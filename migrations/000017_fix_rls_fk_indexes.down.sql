-- Migration: 000017_fix_rls_fk_indexes (rollback)
-- Description: Reverse RLS, FK, and index fixes from the up migration
-- Created: 2026-02-05

-- ============================================================================
-- REMOVE INDEXES (Fix #44 rollback)
-- ============================================================================
DROP INDEX IF EXISTS idx_addresses_neighborhood_code;
DROP INDEX IF EXISTS idx_provider_api_logs_organization_id;
DROP INDEX IF EXISTS idx_offers_provider_account_id;
DROP INDEX IF EXISTS idx_shipments_original_shipment_id;
DROP INDEX IF EXISTS idx_shipments_return_shipment_id;
DROP INDEX IF EXISTS idx_shipments_label_document_id;
DROP INDEX IF EXISTS idx_shipments_provider_account_id;
DROP INDEX IF EXISTS idx_cod_settlements_ledger_entry_id;
DROP INDEX IF EXISTS idx_refunds_approved_by;
DROP INDEX IF EXISTS idx_refunds_refund_ledger_entry_id;
DROP INDEX IF EXISTS idx_topups_ledger_entry_id;
DROP INDEX IF EXISTS idx_members_role_id;

-- ============================================================================
-- REVERT FK ON DELETE CLAUSES (Fix #42 rollback)
-- Restore to original NO ACTION (PostgreSQL default)
-- ============================================================================

-- cod_settlements.ledger_entry_id -> revert to NO ACTION
ALTER TABLE cod_settlements
    DROP CONSTRAINT IF EXISTS cod_settlements_ledger_entry_id_fkey;
ALTER TABLE cod_settlements
    ADD CONSTRAINT cod_settlements_ledger_entry_id_fkey
    FOREIGN KEY (ledger_entry_id) REFERENCES ledger_entries(id);

-- topups.ledger_entry_id -> revert to NO ACTION
ALTER TABLE topups
    DROP CONSTRAINT IF EXISTS topups_ledger_entry_id_fkey;
ALTER TABLE topups
    ADD CONSTRAINT topups_ledger_entry_id_fkey
    FOREIGN KEY (ledger_entry_id) REFERENCES ledger_entries(id);

-- refunds.approved_by -> revert to NO ACTION
ALTER TABLE refunds
    DROP CONSTRAINT IF EXISTS refunds_approved_by_fkey;
ALTER TABLE refunds
    ADD CONSTRAINT refunds_approved_by_fkey
    FOREIGN KEY (approved_by) REFERENCES members(id);

-- refunds.refund_ledger_entry_id -> revert to NO ACTION
ALTER TABLE refunds
    DROP CONSTRAINT IF EXISTS refunds_refund_ledger_entry_id_fkey;
ALTER TABLE refunds
    ADD CONSTRAINT refunds_refund_ledger_entry_id_fkey
    FOREIGN KEY (refund_ledger_entry_id) REFERENCES ledger_entries(id);

-- shipments.original_shipment_id -> revert to NO ACTION
ALTER TABLE shipments
    DROP CONSTRAINT IF EXISTS fk_shipments_original_shipment;
ALTER TABLE shipments
    ADD CONSTRAINT fk_shipments_original_shipment
    FOREIGN KEY (original_shipment_id) REFERENCES shipments(id);

-- shipments.return_shipment_id -> revert to NO ACTION
ALTER TABLE shipments
    DROP CONSTRAINT IF EXISTS fk_shipments_return_shipment;
ALTER TABLE shipments
    ADD CONSTRAINT fk_shipments_return_shipment
    FOREIGN KEY (return_shipment_id) REFERENCES shipments(id);

-- shipments.label_document_id -> revert to NO ACTION
ALTER TABLE shipments
    DROP CONSTRAINT IF EXISTS shipments_label_document_id_fkey;
ALTER TABLE shipments
    ADD CONSTRAINT shipments_label_document_id_fkey
    FOREIGN KEY (label_document_id) REFERENCES documents(id);

-- shipments.provider_account_id -> revert to NO ACTION
ALTER TABLE shipments
    DROP CONSTRAINT IF EXISTS shipments_provider_account_id_fkey;
ALTER TABLE shipments
    ADD CONSTRAINT shipments_provider_account_id_fkey
    FOREIGN KEY (provider_account_id) REFERENCES provider_accounts(id);

-- shipments.offer_id -> revert to NO ACTION
ALTER TABLE shipments
    DROP CONSTRAINT IF EXISTS shipments_offer_id_fkey;
ALTER TABLE shipments
    ADD CONSTRAINT shipments_offer_id_fkey
    FOREIGN KEY (offer_id) REFERENCES offers(id);

-- offers.provider_account_id -> revert to NO ACTION
ALTER TABLE offers
    DROP CONSTRAINT IF EXISTS offers_provider_account_id_fkey;
ALTER TABLE offers
    ADD CONSTRAINT offers_provider_account_id_fkey
    FOREIGN KEY (provider_account_id) REFERENCES provider_accounts(id);

-- ============================================================================
-- REVERT audit_logs FK (Fix #24 rollback)
-- The original migration (000010) already had ON DELETE SET NULL, so we
-- restore that same constraint. No actual change from the up migration.
-- ============================================================================
ALTER TABLE audit_logs
    DROP CONSTRAINT IF EXISTS audit_logs_organization_id_fkey;
ALTER TABLE audit_logs
    ADD CONSTRAINT audit_logs_organization_id_fkey
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE SET NULL;

-- ============================================================================
-- REMOVE RLS POLICIES (Fix #9 rollback)
-- ============================================================================

-- Remove RLS from idempotency_records
DROP POLICY IF EXISTS idempotency_records_tenant_insert ON idempotency_records;
DROP POLICY IF EXISTS idempotency_records_tenant_isolation ON idempotency_records;
ALTER TABLE idempotency_records DISABLE ROW LEVEL SECURITY;

-- Remove RLS from provider_api_logs
DROP POLICY IF EXISTS provider_api_logs_tenant_insert ON provider_api_logs;
DROP POLICY IF EXISTS provider_api_logs_tenant_isolation ON provider_api_logs;
ALTER TABLE provider_api_logs DISABLE ROW LEVEL SECURITY;

-- Remove RLS from audit_logs
DROP POLICY IF EXISTS audit_logs_tenant_insert ON audit_logs;
DROP POLICY IF EXISTS audit_logs_tenant_isolation ON audit_logs;
ALTER TABLE audit_logs DISABLE ROW LEVEL SECURITY;

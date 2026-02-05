-- Migration: 000017_fix_rls_fk_indexes
-- Description: Fix missing RLS on infrastructure tables, add ON DELETE clauses to
--              foreign keys, and add indexes on unindexed FK columns.
-- Created: 2026-02-05
--
-- This migration addresses four issues:
--   Fix #9  - Missing RLS on audit_logs, provider_api_logs, idempotency_records
--   Fix #24 - Ensure audit_logs.organization_id uses ON DELETE SET NULL (defensive)
--   Fix #42 - Add ON DELETE SET NULL to nullable FK columns that lack an explicit clause
--   Fix #44 - Add indexes on commonly-queried FK columns that are missing them

-- ============================================================================
-- FIX #9: MISSING RLS ON INFRASTRUCTURE TABLES
-- audit_logs, provider_api_logs, and idempotency_records were created without
-- row-level security, unlike every other tenant-scoped table in the schema.
-- ============================================================================

-- Enable RLS on audit_logs
ALTER TABLE audit_logs ENABLE ROW LEVEL SECURITY;

CREATE POLICY audit_logs_tenant_isolation ON audit_logs
    USING (
        organization_id IS NULL  -- System-level audit entries accessible when no tenant context
        OR organization_id = current_setting('app.current_organization_id', true)::uuid
    );

CREATE POLICY audit_logs_tenant_insert ON audit_logs
    FOR INSERT
    WITH CHECK (
        organization_id IS NULL
        OR organization_id = current_setting('app.current_organization_id', true)::uuid
    );

-- Enable RLS on provider_api_logs
ALTER TABLE provider_api_logs ENABLE ROW LEVEL SECURITY;

CREATE POLICY provider_api_logs_tenant_isolation ON provider_api_logs
    USING (
        organization_id IS NULL  -- System-level API logs accessible when no tenant context
        OR organization_id = current_setting('app.current_organization_id', true)::uuid
    );

CREATE POLICY provider_api_logs_tenant_insert ON provider_api_logs
    FOR INSERT
    WITH CHECK (
        organization_id IS NULL
        OR organization_id = current_setting('app.current_organization_id', true)::uuid
    );

-- Enable RLS on idempotency_records
ALTER TABLE idempotency_records ENABLE ROW LEVEL SECURITY;

CREATE POLICY idempotency_records_tenant_isolation ON idempotency_records
    USING (organization_id = current_setting('app.current_organization_id', true)::uuid);

CREATE POLICY idempotency_records_tenant_insert ON idempotency_records
    FOR INSERT
    WITH CHECK (organization_id = current_setting('app.current_organization_id', true)::uuid);

-- ============================================================================
-- FIX #24: ENSURE audit_logs.organization_id IS ON DELETE SET NULL
-- The original migration (000010) already specifies ON DELETE SET NULL, but we
-- defensively re-apply it in case the constraint was altered outside migrations.
-- ============================================================================

-- Drop and recreate the FK to guarantee SET NULL behavior
ALTER TABLE audit_logs
    DROP CONSTRAINT IF EXISTS audit_logs_organization_id_fkey;

ALTER TABLE audit_logs
    ADD CONSTRAINT audit_logs_organization_id_fkey
    FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE SET NULL;

-- ============================================================================
-- FIX #42: ADD ON DELETE CLAUSES TO FOREIGN KEYS MISSING THEM
-- Nullable FK columns get SET NULL; the default NO ACTION is appropriate for
-- non-nullable columns (members.role_id, cod_settlements.shipment_id, etc.)
-- since they cannot be set to NULL and CASCADE would be too destructive.
-- ============================================================================

-- offers.provider_account_id -> SET NULL
ALTER TABLE offers
    DROP CONSTRAINT IF EXISTS offers_provider_account_id_fkey;
ALTER TABLE offers
    ADD CONSTRAINT offers_provider_account_id_fkey
    FOREIGN KEY (provider_account_id) REFERENCES provider_accounts(id) ON DELETE SET NULL;

-- shipments.offer_id -> SET NULL
ALTER TABLE shipments
    DROP CONSTRAINT IF EXISTS shipments_offer_id_fkey;
ALTER TABLE shipments
    ADD CONSTRAINT shipments_offer_id_fkey
    FOREIGN KEY (offer_id) REFERENCES offers(id) ON DELETE SET NULL;

-- shipments.provider_account_id -> SET NULL
ALTER TABLE shipments
    DROP CONSTRAINT IF EXISTS shipments_provider_account_id_fkey;
ALTER TABLE shipments
    ADD CONSTRAINT shipments_provider_account_id_fkey
    FOREIGN KEY (provider_account_id) REFERENCES provider_accounts(id) ON DELETE SET NULL;

-- shipments.label_document_id -> SET NULL
ALTER TABLE shipments
    DROP CONSTRAINT IF EXISTS shipments_label_document_id_fkey;
ALTER TABLE shipments
    ADD CONSTRAINT shipments_label_document_id_fkey
    FOREIGN KEY (label_document_id) REFERENCES documents(id) ON DELETE SET NULL;

-- shipments.return_shipment_id -> SET NULL (named constraint from migration 000007)
ALTER TABLE shipments
    DROP CONSTRAINT IF EXISTS fk_shipments_return_shipment;
ALTER TABLE shipments
    ADD CONSTRAINT fk_shipments_return_shipment
    FOREIGN KEY (return_shipment_id) REFERENCES shipments(id) ON DELETE SET NULL;

-- shipments.original_shipment_id -> SET NULL (named constraint from migration 000007)
ALTER TABLE shipments
    DROP CONSTRAINT IF EXISTS fk_shipments_original_shipment;
ALTER TABLE shipments
    ADD CONSTRAINT fk_shipments_original_shipment
    FOREIGN KEY (original_shipment_id) REFERENCES shipments(id) ON DELETE SET NULL;

-- refunds.refund_ledger_entry_id -> SET NULL
ALTER TABLE refunds
    DROP CONSTRAINT IF EXISTS refunds_refund_ledger_entry_id_fkey;
ALTER TABLE refunds
    ADD CONSTRAINT refunds_refund_ledger_entry_id_fkey
    FOREIGN KEY (refund_ledger_entry_id) REFERENCES ledger_entries(id) ON DELETE SET NULL;

-- refunds.approved_by -> SET NULL
ALTER TABLE refunds
    DROP CONSTRAINT IF EXISTS refunds_approved_by_fkey;
ALTER TABLE refunds
    ADD CONSTRAINT refunds_approved_by_fkey
    FOREIGN KEY (approved_by) REFERENCES members(id) ON DELETE SET NULL;

-- topups.ledger_entry_id -> SET NULL
ALTER TABLE topups
    DROP CONSTRAINT IF EXISTS topups_ledger_entry_id_fkey;
ALTER TABLE topups
    ADD CONSTRAINT topups_ledger_entry_id_fkey
    FOREIGN KEY (ledger_entry_id) REFERENCES ledger_entries(id) ON DELETE SET NULL;

-- cod_settlements.ledger_entry_id -> SET NULL
ALTER TABLE cod_settlements
    DROP CONSTRAINT IF EXISTS cod_settlements_ledger_entry_id_fkey;
ALTER TABLE cod_settlements
    ADD CONSTRAINT cod_settlements_ledger_entry_id_fkey
    FOREIGN KEY (ledger_entry_id) REFERENCES ledger_entries(id) ON DELETE SET NULL;

-- ============================================================================
-- FIX #44: ADD INDEXES ON UNINDEXED FOREIGN KEY COLUMNS
-- These FK columns are commonly used in JOINs and WHERE clauses but lack
-- indexes, causing sequential scans on related queries.
-- ============================================================================

-- members.role_id (used in role assignment queries and JOIN to roles)
CREATE INDEX IF NOT EXISTS idx_members_role_id ON members(role_id);

-- topups.ledger_entry_id (used to find the ledger entry for a completed topup)
CREATE INDEX IF NOT EXISTS idx_topups_ledger_entry_id ON topups(ledger_entry_id);

-- refunds.refund_ledger_entry_id (used to find the credit entry for a refund)
CREATE INDEX IF NOT EXISTS idx_refunds_refund_ledger_entry_id ON refunds(refund_ledger_entry_id);

-- refunds.approved_by (used to find refunds approved by a specific member)
CREATE INDEX IF NOT EXISTS idx_refunds_approved_by ON refunds(approved_by);

-- cod_settlements.ledger_entry_id (used to find the ledger entry for a settlement)
CREATE INDEX IF NOT EXISTS idx_cod_settlements_ledger_entry_id ON cod_settlements(ledger_entry_id);

-- shipments.provider_account_id (used in BYOA queries filtering by account)
CREATE INDEX IF NOT EXISTS idx_shipments_provider_account_id ON shipments(provider_account_id);

-- shipments.label_document_id (used when looking up the label document for a shipment)
CREATE INDEX IF NOT EXISTS idx_shipments_label_document_id ON shipments(label_document_id);

-- shipments.return_shipment_id (used to find linked return shipments)
CREATE INDEX IF NOT EXISTS idx_shipments_return_shipment_id ON shipments(return_shipment_id);

-- shipments.original_shipment_id (used to find the original shipment from a return)
CREATE INDEX IF NOT EXISTS idx_shipments_original_shipment_id ON shipments(original_shipment_id);

-- offers.provider_account_id (used in BYOA offer queries)
CREATE INDEX IF NOT EXISTS idx_offers_provider_account_id ON offers(provider_account_id);

-- provider_api_logs.organization_id (used to filter API logs by organization)
CREATE INDEX IF NOT EXISTS idx_provider_api_logs_organization_id ON provider_api_logs(organization_id);

-- addresses.neighborhood_code (used in location-based address lookups)
CREATE INDEX IF NOT EXISTS idx_addresses_neighborhood_code ON addresses(neighborhood_code);

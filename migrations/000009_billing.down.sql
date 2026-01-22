-- Migration: 000009_billing (rollback)
-- Description: Drop ledger_entries, topups, refunds, and cod_settlements tables
-- Created: 2026-01-22

-- Drop RLS policies first
DROP POLICY IF EXISTS cod_settlements_tenant_insert ON cod_settlements;
DROP POLICY IF EXISTS cod_settlements_tenant_isolation ON cod_settlements;
DROP POLICY IF EXISTS refunds_tenant_insert ON refunds;
DROP POLICY IF EXISTS refunds_tenant_isolation ON refunds;
DROP POLICY IF EXISTS topups_tenant_insert ON topups;
DROP POLICY IF EXISTS topups_tenant_isolation ON topups;
DROP POLICY IF EXISTS ledger_entries_tenant_insert ON ledger_entries;
DROP POLICY IF EXISTS ledger_entries_tenant_isolation ON ledger_entries;

-- Drop tables in reverse order of creation (respecting foreign key dependencies)
DROP TABLE IF EXISTS cod_settlements;
DROP TABLE IF EXISTS refunds;
DROP TABLE IF EXISTS topups;
DROP TABLE IF EXISTS ledger_entries;

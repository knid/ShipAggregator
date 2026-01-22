-- Migration: 000006_quotes (rollback)
-- Description: Drop quotes and offers tables
-- Created: 2026-01-22

-- Drop RLS policies first
DROP POLICY IF EXISTS offers_tenant_insert ON offers;
DROP POLICY IF EXISTS offers_tenant_isolation ON offers;
DROP POLICY IF EXISTS quotes_tenant_insert ON quotes;
DROP POLICY IF EXISTS quotes_tenant_isolation ON quotes;

-- Drop tables in reverse order of creation (respecting foreign key dependencies)
DROP TABLE IF EXISTS offers;
DROP TABLE IF EXISTS quotes;

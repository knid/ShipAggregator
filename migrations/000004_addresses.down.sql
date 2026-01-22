-- Migration: 000004_addresses (rollback)
-- Description: Drop addresses table
-- Created: 2026-01-22

-- Drop RLS policies first
DROP POLICY IF EXISTS addresses_tenant_insert ON addresses;
DROP POLICY IF EXISTS addresses_tenant_isolation ON addresses;

-- Drop the table
DROP TABLE IF EXISTS addresses;

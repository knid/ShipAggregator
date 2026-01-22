-- Migration: 000002_api_keys (rollback)
-- Description: Drop api_keys table
-- Created: 2026-01-22

-- Drop RLS policies first
DROP POLICY IF EXISTS api_keys_tenant_insert ON api_keys;
DROP POLICY IF EXISTS api_keys_tenant_isolation ON api_keys;

-- Drop the table
DROP TABLE IF EXISTS api_keys;

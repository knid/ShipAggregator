-- Migration: 000005_providers (rollback)
-- Description: Drop providers, provider_services, provider_accounts, and marketplace_rate_plans tables
-- Created: 2026-01-22

-- Drop RLS policies first
DROP POLICY IF EXISTS provider_accounts_tenant_insert ON provider_accounts;
DROP POLICY IF EXISTS provider_accounts_tenant_isolation ON provider_accounts;

-- Drop tables in reverse order of creation (respecting foreign key dependencies)
DROP TABLE IF EXISTS marketplace_rate_plans;
DROP TABLE IF EXISTS provider_accounts;
DROP TABLE IF EXISTS provider_services;
DROP TABLE IF EXISTS providers;

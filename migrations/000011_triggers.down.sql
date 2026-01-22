-- Migration: 000011_triggers (rollback)
-- Description: Drop updated_at, ledger_immutability, and audit_log triggers
-- Created: 2026-01-22

-- Drop audit triggers
DROP TRIGGER IF EXISTS trg_refunds_audit ON refunds;
DROP TRIGGER IF EXISTS trg_topups_audit ON topups;
DROP TRIGGER IF EXISTS trg_webhook_endpoints_audit ON webhook_endpoints;
DROP TRIGGER IF EXISTS trg_shipments_audit ON shipments;
DROP TRIGGER IF EXISTS trg_provider_accounts_audit ON provider_accounts;
DROP TRIGGER IF EXISTS trg_api_keys_audit ON api_keys;
DROP TRIGGER IF EXISTS trg_roles_audit ON roles;
DROP TRIGGER IF EXISTS trg_members_audit ON members;
DROP TRIGGER IF EXISTS trg_organizations_audit ON organizations;

-- Drop ledger immutability trigger
DROP TRIGGER IF EXISTS trg_ledger_immutable ON ledger_entries;

-- Drop updated_at triggers
DROP TRIGGER IF EXISTS trg_cod_settlements_updated_at ON cod_settlements;
DROP TRIGGER IF EXISTS trg_refunds_updated_at ON refunds;
DROP TRIGGER IF EXISTS trg_topups_updated_at ON topups;
DROP TRIGGER IF EXISTS trg_webhook_deliveries_updated_at ON webhook_deliveries;
DROP TRIGGER IF EXISTS trg_webhook_endpoints_updated_at ON webhook_endpoints;
DROP TRIGGER IF EXISTS trg_shipments_updated_at ON shipments;
DROP TRIGGER IF EXISTS trg_marketplace_rate_plans_updated_at ON marketplace_rate_plans;
DROP TRIGGER IF EXISTS trg_provider_accounts_updated_at ON provider_accounts;
DROP TRIGGER IF EXISTS trg_provider_services_updated_at ON provider_services;
DROP TRIGGER IF EXISTS trg_providers_updated_at ON providers;
DROP TRIGGER IF EXISTS trg_addresses_updated_at ON addresses;
DROP TRIGGER IF EXISTS trg_api_keys_updated_at ON api_keys;
DROP TRIGGER IF EXISTS trg_roles_updated_at ON roles;
DROP TRIGGER IF EXISTS trg_members_updated_at ON members;
DROP TRIGGER IF EXISTS trg_organizations_updated_at ON organizations;

-- Drop trigger functions
DROP FUNCTION IF EXISTS audit_log_trigger();
DROP FUNCTION IF EXISTS prevent_ledger_modification();
DROP FUNCTION IF EXISTS update_updated_at();

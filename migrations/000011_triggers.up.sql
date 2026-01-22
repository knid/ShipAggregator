-- Migration: 000011_triggers
-- Description: Create updated_at, ledger_immutability, and audit_log triggers
-- Created: 2026-01-22

-- ============================================================================
-- UPDATED_AT TRIGGER FUNCTION
-- Auto-update updated_at column on row modification
-- ============================================================================
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION update_updated_at() IS 'Auto-update updated_at column on row modification';

-- Apply updated_at trigger to all tables with updated_at column
CREATE TRIGGER trg_organizations_updated_at
    BEFORE UPDATE ON organizations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_members_updated_at
    BEFORE UPDATE ON members
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_roles_updated_at
    BEFORE UPDATE ON roles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_api_keys_updated_at
    BEFORE UPDATE ON api_keys
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_addresses_updated_at
    BEFORE UPDATE ON addresses
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_providers_updated_at
    BEFORE UPDATE ON providers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_provider_services_updated_at
    BEFORE UPDATE ON provider_services
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_provider_accounts_updated_at
    BEFORE UPDATE ON provider_accounts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_marketplace_rate_plans_updated_at
    BEFORE UPDATE ON marketplace_rate_plans
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_shipments_updated_at
    BEFORE UPDATE ON shipments
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_webhook_endpoints_updated_at
    BEFORE UPDATE ON webhook_endpoints
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_webhook_deliveries_updated_at
    BEFORE UPDATE ON webhook_deliveries
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_topups_updated_at
    BEFORE UPDATE ON topups
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_refunds_updated_at
    BEFORE UPDATE ON refunds
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_cod_settlements_updated_at
    BEFORE UPDATE ON cod_settlements
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

-- ============================================================================
-- LEDGER IMMUTABILITY TRIGGER FUNCTION
-- Prevent UPDATE/DELETE on ledger_entries (append-only)
-- ============================================================================
CREATE OR REPLACE FUNCTION prevent_ledger_modification()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'Ledger entries are immutable. UPDATE and DELETE operations are not allowed.';
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION prevent_ledger_modification() IS 'Prevent UPDATE/DELETE on ledger_entries';

-- Apply immutability trigger to ledger_entries
CREATE TRIGGER trg_ledger_immutable
    BEFORE UPDATE OR DELETE ON ledger_entries
    FOR EACH ROW EXECUTE FUNCTION prevent_ledger_modification();

-- ============================================================================
-- AUDIT LOG TRIGGER FUNCTION
-- Auto-create audit log entries for tracked tables
-- ============================================================================
CREATE OR REPLACE FUNCTION audit_log_trigger()
RETURNS TRIGGER AS $$
DECLARE
    v_organization_id UUID;
    v_actor_id UUID;
    v_actor_type VARCHAR(20);
    v_trace_id VARCHAR(64);
BEGIN
    -- Get organization_id from the record (if available)
    IF TG_OP = 'DELETE' THEN
        v_organization_id := CASE 
            WHEN TG_TABLE_NAME IN ('organizations') THEN OLD.id
            ELSE OLD.organization_id
        END;
    ELSE
        v_organization_id := CASE 
            WHEN TG_TABLE_NAME IN ('organizations') THEN NEW.id
            ELSE NEW.organization_id
        END;
    END IF;
    
    -- Get actor info from session settings (set by application)
    BEGIN
        v_actor_id := current_setting('app.current_actor_id', true)::uuid;
    EXCEPTION WHEN OTHERS THEN
        v_actor_id := NULL;
    END;
    
    v_actor_type := COALESCE(current_setting('app.current_actor_type', true), 'system');
    v_trace_id := current_setting('app.trace_id', true);
    
    -- Insert audit log entry
    INSERT INTO audit_logs (
        organization_id,
        actor_id,
        actor_type,
        action,
        resource_type,
        resource_id,
        old_value,
        new_value,
        trace_id
    ) VALUES (
        v_organization_id,
        v_actor_id,
        v_actor_type,
        TG_OP,
        TG_TABLE_NAME,
        COALESCE(NEW.id, OLD.id),
        CASE WHEN TG_OP IN ('UPDATE', 'DELETE') THEN to_jsonb(OLD) END,
        CASE WHEN TG_OP IN ('INSERT', 'UPDATE') THEN to_jsonb(NEW) END,
        v_trace_id
    );
    
    RETURN COALESCE(NEW, OLD);
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION audit_log_trigger() IS 'Auto-create audit log entries for tracked tables';

-- Apply audit trigger to critical tables
-- Note: We apply AFTER triggers to ensure the operation succeeded before logging

CREATE TRIGGER trg_organizations_audit
    AFTER INSERT OR UPDATE OR DELETE ON organizations
    FOR EACH ROW EXECUTE FUNCTION audit_log_trigger();

CREATE TRIGGER trg_members_audit
    AFTER INSERT OR UPDATE OR DELETE ON members
    FOR EACH ROW EXECUTE FUNCTION audit_log_trigger();

CREATE TRIGGER trg_roles_audit
    AFTER INSERT OR UPDATE OR DELETE ON roles
    FOR EACH ROW EXECUTE FUNCTION audit_log_trigger();

CREATE TRIGGER trg_api_keys_audit
    AFTER INSERT OR UPDATE OR DELETE ON api_keys
    FOR EACH ROW EXECUTE FUNCTION audit_log_trigger();

CREATE TRIGGER trg_provider_accounts_audit
    AFTER INSERT OR UPDATE OR DELETE ON provider_accounts
    FOR EACH ROW EXECUTE FUNCTION audit_log_trigger();

CREATE TRIGGER trg_shipments_audit
    AFTER INSERT OR UPDATE OR DELETE ON shipments
    FOR EACH ROW EXECUTE FUNCTION audit_log_trigger();

CREATE TRIGGER trg_webhook_endpoints_audit
    AFTER INSERT OR UPDATE OR DELETE ON webhook_endpoints
    FOR EACH ROW EXECUTE FUNCTION audit_log_trigger();

CREATE TRIGGER trg_topups_audit
    AFTER INSERT OR UPDATE OR DELETE ON topups
    FOR EACH ROW EXECUTE FUNCTION audit_log_trigger();

CREATE TRIGGER trg_refunds_audit
    AFTER INSERT OR UPDATE OR DELETE ON refunds
    FOR EACH ROW EXECUTE FUNCTION audit_log_trigger();

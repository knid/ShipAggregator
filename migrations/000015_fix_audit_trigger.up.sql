-- Migration: 000015_fix_audit_trigger
-- Description: Fix audit_log_trigger to handle organizations table correctly
-- Created: 2026-01-23

-- ============================================================================
-- FIX AUDIT LOG TRIGGER FUNCTION
-- The previous version failed because it tried to access organization_id
-- on the organizations table which doesn't have that column
-- ============================================================================
CREATE OR REPLACE FUNCTION audit_log_trigger()
RETURNS TRIGGER AS $$
DECLARE
    v_organization_id UUID;
    v_actor_id UUID;
    v_actor_type VARCHAR(20);
    v_trace_id VARCHAR(64);
    v_record RECORD;
BEGIN
    -- Determine which record to use based on operation
    IF TG_OP = 'DELETE' THEN
        v_record := OLD;
    ELSE
        v_record := NEW;
    END IF;
    
    -- Get organization_id from the record
    -- For organizations table, use the id field
    -- For other tables, use organization_id field
    IF TG_TABLE_NAME = 'organizations' THEN
        v_organization_id := v_record.id;
    ELSE
        -- Use dynamic SQL to safely check if organization_id exists
        BEGIN
            EXECUTE format('SELECT ($1).organization_id') INTO v_organization_id USING v_record;
        EXCEPTION WHEN undefined_column THEN
            v_organization_id := NULL;
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

COMMENT ON FUNCTION audit_log_trigger() IS 'Auto-create audit log entries for tracked tables (fixed for organizations table)';

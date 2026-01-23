-- Migration: 000015_fix_audit_trigger (down)
-- Description: Revert to original audit_log_trigger function
-- Created: 2026-01-23

-- Restore original function (this will fail on organizations table)
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

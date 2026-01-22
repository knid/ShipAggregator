-- Migration: 000007_shipments (rollback)
-- Description: Drop shipments, tracking_events, and documents tables
-- Created: 2026-01-22

-- Drop RLS policies first
DROP POLICY IF EXISTS tracking_events_tenant_insert ON tracking_events;
DROP POLICY IF EXISTS tracking_events_tenant_isolation ON tracking_events;
DROP POLICY IF EXISTS shipments_tenant_insert ON shipments;
DROP POLICY IF EXISTS shipments_tenant_isolation ON shipments;
DROP POLICY IF EXISTS documents_tenant_insert ON documents;
DROP POLICY IF EXISTS documents_tenant_isolation ON documents;

-- Drop tables in reverse order of creation (respecting foreign key dependencies)
DROP TABLE IF EXISTS tracking_events;
DROP TABLE IF EXISTS shipments;
DROP TABLE IF EXISTS documents;

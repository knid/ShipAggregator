-- Migration: 000008_webhooks (rollback)
-- Description: Drop webhook_endpoints, webhook_subscriptions, webhook_events, and webhook_deliveries tables
-- Created: 2026-01-22

-- Drop RLS policies first
DROP POLICY IF EXISTS webhook_deliveries_tenant_insert ON webhook_deliveries;
DROP POLICY IF EXISTS webhook_deliveries_tenant_isolation ON webhook_deliveries;
DROP POLICY IF EXISTS webhook_events_tenant_insert ON webhook_events;
DROP POLICY IF EXISTS webhook_events_tenant_isolation ON webhook_events;
DROP POLICY IF EXISTS webhook_subscriptions_tenant_insert ON webhook_subscriptions;
DROP POLICY IF EXISTS webhook_subscriptions_tenant_isolation ON webhook_subscriptions;
DROP POLICY IF EXISTS webhook_endpoints_tenant_insert ON webhook_endpoints;
DROP POLICY IF EXISTS webhook_endpoints_tenant_isolation ON webhook_endpoints;

-- Drop tables in reverse order of creation (respecting foreign key dependencies)
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhook_events;
DROP TABLE IF EXISTS webhook_subscriptions;
DROP TABLE IF EXISTS webhook_endpoints;

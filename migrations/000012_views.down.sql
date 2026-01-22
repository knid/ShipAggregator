-- Migration: 000012_views (rollback)
-- Description: Drop v_wallet_balances and v_provider_health views
-- Created: 2026-01-22

-- Drop views
DROP VIEW IF EXISTS v_webhook_delivery_stats;
DROP VIEW IF EXISTS v_shipment_summary;
DROP VIEW IF EXISTS v_organization_stats;
DROP VIEW IF EXISTS v_provider_health;
DROP VIEW IF EXISTS v_wallet_balances;

-- Migration: 000013_seed_providers (rollback)
-- Description: Remove provider seed data
-- Created: 2026-01-22

-- Delete in reverse order of insertion (respecting foreign key dependencies)
DELETE FROM marketplace_rate_plans WHERE provider_code IN ('aras', 'yurtici');
DELETE FROM provider_services WHERE provider_code IN ('aras', 'yurtici');
DELETE FROM providers WHERE code IN ('aras', 'yurtici');

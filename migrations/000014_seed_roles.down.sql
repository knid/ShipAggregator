-- Migration: 000014_seed_roles (rollback)
-- Description: Remove system roles seed data
-- Created: 2026-01-22

-- Delete system roles by their fixed UUIDs
DELETE FROM roles WHERE id IN (
    '00000000-0000-0000-0000-000000000001',  -- owner
    '00000000-0000-0000-0000-000000000002',  -- admin
    '00000000-0000-0000-0000-000000000003',  -- operator
    '00000000-0000-0000-0000-000000000004'   -- viewer
);

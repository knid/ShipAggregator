-- Migration: 000016_add_providers_read_permission
-- Description: Add providers:read permission to all system roles
-- Created: 2026-01-23
-- 
-- This migration adds the missing providers:read permission to all system roles.
-- The /v1/providers endpoint requires this permission to list available shipping providers.

-- Update owner role - add providers:read permission
UPDATE roles
SET permissions = permissions || '["providers:read"]'::jsonb,
    updated_at = NOW()
WHERE id = '00000000-0000-0000-0000-000000000001'
  AND is_system = TRUE
  AND NOT permissions ? 'providers:read';

-- Update admin role - add providers:read permission
UPDATE roles
SET permissions = permissions || '["providers:read"]'::jsonb,
    updated_at = NOW()
WHERE id = '00000000-0000-0000-0000-000000000002'
  AND is_system = TRUE
  AND NOT permissions ? 'providers:read';

-- Update operator role - add providers:read permission
UPDATE roles
SET permissions = permissions || '["providers:read"]'::jsonb,
    updated_at = NOW()
WHERE id = '00000000-0000-0000-0000-000000000003'
  AND is_system = TRUE
  AND NOT permissions ? 'providers:read';

-- Update viewer role - add providers:read permission
UPDATE roles
SET permissions = permissions || '["providers:read"]'::jsonb,
    updated_at = NOW()
WHERE id = '00000000-0000-0000-0000-000000000004'
  AND is_system = TRUE
  AND NOT permissions ? 'providers:read';

-- Migration: 000016_add_providers_read_permission (rollback)
-- Description: Remove providers:read permission from all system roles
-- Created: 2026-01-23

-- Remove providers:read from owner role
UPDATE roles
SET permissions = permissions - 'providers:read',
    updated_at = NOW()
WHERE id = '00000000-0000-0000-0000-000000000001'
  AND is_system = TRUE;

-- Remove providers:read from admin role
UPDATE roles
SET permissions = permissions - 'providers:read',
    updated_at = NOW()
WHERE id = '00000000-0000-0000-0000-000000000002'
  AND is_system = TRUE;

-- Remove providers:read from operator role
UPDATE roles
SET permissions = permissions - 'providers:read',
    updated_at = NOW()
WHERE id = '00000000-0000-0000-0000-000000000003'
  AND is_system = TRUE;

-- Remove providers:read from viewer role
UPDATE roles
SET permissions = permissions - 'providers:read',
    updated_at = NOW()
WHERE id = '00000000-0000-0000-0000-000000000004'
  AND is_system = TRUE;

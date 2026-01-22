-- Roles CRUD queries
-- Table: roles

-- name: GetRoleByID :one
SELECT id, organization_id, name, description, permissions, is_system, created_at, updated_at
FROM roles
WHERE id = $1;

-- name: GetRoleByName :one
SELECT id, organization_id, name, description, permissions, is_system, created_at, updated_at
FROM roles
WHERE name = $1 AND (organization_id = sqlc.narg('organization_id')::uuid OR organization_id IS NULL);

-- name: ListRolesByOrganization :many
SELECT id, organization_id, name, description, permissions, is_system, created_at, updated_at
FROM roles
WHERE organization_id = $1 OR organization_id IS NULL
ORDER BY is_system DESC, name ASC;

-- name: ListSystemRoles :many
SELECT id, organization_id, name, description, permissions, is_system, created_at, updated_at
FROM roles
WHERE organization_id IS NULL AND is_system = TRUE
ORDER BY name ASC;

-- name: ListCustomRolesByOrganization :many
SELECT id, organization_id, name, description, permissions, is_system, created_at, updated_at
FROM roles
WHERE organization_id = $1 AND is_system = FALSE
ORDER BY name ASC;

-- name: CreateRole :one
INSERT INTO roles (
    organization_id,
    name,
    description,
    permissions,
    is_system
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING id, organization_id, name, description, permissions, is_system, created_at, updated_at;

-- name: UpdateRole :one
UPDATE roles
SET
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    permissions = COALESCE(sqlc.narg('permissions'), permissions),
    updated_at = NOW()
WHERE id = $1 AND is_system = FALSE
RETURNING id, organization_id, name, description, permissions, is_system, created_at, updated_at;

-- name: DeleteRole :exec
DELETE FROM roles
WHERE id = $1 AND is_system = FALSE;

-- name: CheckRoleNameExists :one
SELECT EXISTS(
    SELECT 1 FROM roles
    WHERE name = $1 
      AND (organization_id = sqlc.narg('organization_id')::uuid OR (organization_id IS NULL AND sqlc.narg('organization_id')::uuid IS NULL))
      AND id != COALESCE(sqlc.narg('exclude_id')::uuid, '00000000-0000-0000-0000-000000000000'::uuid)
) as exists;

-- name: CountMembersWithRole :one
SELECT COUNT(*) as total
FROM members
WHERE role_id = $1 AND status != 'removed';

-- name: GetSystemRoleByName :one
SELECT id, organization_id, name, description, permissions, is_system, created_at, updated_at
FROM roles
WHERE name = $1 AND organization_id IS NULL AND is_system = TRUE;

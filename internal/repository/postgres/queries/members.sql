-- Members CRUD queries
-- Table: members

-- name: GetMemberByID :one
SELECT m.id, m.organization_id, m.email, m.name, m.password_hash, m.role_id, m.status,
       m.invited_at, m.joined_at, m.last_login_at, m.created_at, m.updated_at
FROM members m
WHERE m.id = $1 AND m.status != 'removed';

-- name: GetMemberByEmail :one
SELECT m.id, m.organization_id, m.email, m.name, m.password_hash, m.role_id, m.status,
       m.invited_at, m.joined_at, m.last_login_at, m.created_at, m.updated_at
FROM members m
WHERE m.email = $1 AND m.status != 'removed';

-- name: GetMemberByOrgAndEmail :one
SELECT m.id, m.organization_id, m.email, m.name, m.password_hash, m.role_id, m.status,
       m.invited_at, m.joined_at, m.last_login_at, m.created_at, m.updated_at
FROM members m
WHERE m.organization_id = $1 AND m.email = $2 AND m.status != 'removed';

-- name: ListMembersByOrganization :many
SELECT m.id, m.organization_id, m.email, m.name, m.password_hash, m.role_id, m.status,
       m.invited_at, m.joined_at, m.last_login_at, m.created_at, m.updated_at
FROM members m
WHERE m.organization_id = $1 AND m.status != 'removed'
  AND (sqlc.narg('status')::varchar IS NULL OR m.status = sqlc.narg('status')::varchar)
ORDER BY m.created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: CountMembersByOrganization :one
SELECT COUNT(*) as total
FROM members m
WHERE m.organization_id = $1 AND m.status != 'removed'
  AND (sqlc.narg('status')::varchar IS NULL OR m.status = sqlc.narg('status')::varchar);

-- name: CreateMember :one
INSERT INTO members (
    organization_id,
    email,
    name,
    password_hash,
    role_id,
    status,
    invited_at
) VALUES (
    $1, $2, $3, $4, $5, $6, NOW()
)
RETURNING id, organization_id, email, name, password_hash, role_id, status,
          invited_at, joined_at, last_login_at, created_at, updated_at;

-- name: UpdateMember :one
UPDATE members
SET
    name = COALESCE(sqlc.narg('name'), name),
    email = COALESCE(sqlc.narg('email'), email),
    role_id = COALESCE(sqlc.narg('role_id'), role_id),
    status = COALESCE(sqlc.narg('status'), status),
    updated_at = NOW()
WHERE id = $1 AND status != 'removed'
RETURNING id, organization_id, email, name, password_hash, role_id, status,
          invited_at, joined_at, last_login_at, created_at, updated_at;

-- name: UpdateMemberPassword :one
UPDATE members
SET
    password_hash = $2,
    updated_at = NOW()
WHERE id = $1 AND status != 'removed'
RETURNING id, organization_id, email, name, password_hash, role_id, status,
          invited_at, joined_at, last_login_at, created_at, updated_at;

-- name: ActivateMember :one
UPDATE members
SET
    status = 'active',
    password_hash = $2,
    joined_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND status = 'invited'
RETURNING id, organization_id, email, name, password_hash, role_id, status,
          invited_at, joined_at, last_login_at, created_at, updated_at;

-- name: UpdateMemberLastLogin :exec
UPDATE members
SET
    last_login_at = NOW(),
    updated_at = NOW()
WHERE id = $1;

-- name: SuspendMember :one
UPDATE members
SET
    status = 'suspended',
    updated_at = NOW()
WHERE id = $1 AND status = 'active'
RETURNING id, organization_id, email, name, password_hash, role_id, status,
          invited_at, joined_at, last_login_at, created_at, updated_at;

-- name: ReactivateMember :one
UPDATE members
SET
    status = 'active',
    updated_at = NOW()
WHERE id = $1 AND status = 'suspended'
RETURNING id, organization_id, email, name, password_hash, role_id, status,
          invited_at, joined_at, last_login_at, created_at, updated_at;

-- name: RemoveMember :one
UPDATE members
SET
    status = 'removed',
    updated_at = NOW()
WHERE id = $1 AND status != 'removed'
RETURNING id, organization_id, email, name, password_hash, role_id, status,
          invited_at, joined_at, last_login_at, created_at, updated_at;

-- name: CheckMemberEmailExists :one
SELECT EXISTS(
    SELECT 1 FROM members
    WHERE organization_id = $1 AND email = $2 AND status != 'removed'
      AND id != COALESCE(sqlc.narg('exclude_id')::uuid, '00000000-0000-0000-0000-000000000000'::uuid)
) as exists;

-- name: GetMemberWithRole :one
SELECT m.id, m.organization_id, m.email, m.name, m.password_hash, m.role_id, m.status,
       m.invited_at, m.joined_at, m.last_login_at, m.created_at, m.updated_at,
       r.name as role_name, r.permissions as role_permissions, r.is_system as role_is_system
FROM members m
JOIN roles r ON m.role_id = r.id
WHERE m.id = $1 AND m.status != 'removed';

-- name: CheckGlobalEmailExists :one
SELECT EXISTS(
    SELECT 1 FROM members
    WHERE email = $1 AND status != 'removed'
) as exists;

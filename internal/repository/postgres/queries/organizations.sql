-- Organizations CRUD queries
-- Table: organizations

-- name: GetOrganizationByID :one
SELECT id, name, slug, billing_email, settings, status, created_at, updated_at, deleted_at
FROM organizations
WHERE id = $1 AND deleted_at IS NULL;

-- name: GetOrganizationBySlug :one
SELECT id, name, slug, billing_email, settings, status, created_at, updated_at, deleted_at
FROM organizations
WHERE slug = $1 AND deleted_at IS NULL;

-- name: ListOrganizations :many
SELECT id, name, slug, billing_email, settings, status, created_at, updated_at, deleted_at
FROM organizations
WHERE deleted_at IS NULL
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status')::varchar)
ORDER BY created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: CountOrganizations :one
SELECT COUNT(*) as total
FROM organizations
WHERE deleted_at IS NULL
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status')::varchar);

-- name: CreateOrganization :one
INSERT INTO organizations (
    name,
    slug,
    billing_email,
    settings,
    status
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING id, name, slug, billing_email, settings, status, created_at, updated_at, deleted_at;

-- name: UpdateOrganization :one
UPDATE organizations
SET
    name = COALESCE(sqlc.narg('name'), name),
    slug = COALESCE(sqlc.narg('slug'), slug),
    billing_email = COALESCE(sqlc.narg('billing_email'), billing_email),
    settings = COALESCE(sqlc.narg('settings'), settings),
    status = COALESCE(sqlc.narg('status'), status),
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, name, slug, billing_email, settings, status, created_at, updated_at, deleted_at;

-- name: SoftDeleteOrganization :one
UPDATE organizations
SET
    deleted_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, name, slug, billing_email, settings, status, created_at, updated_at, deleted_at;

-- name: UpdateOrganizationStatus :one
UPDATE organizations
SET
    status = $2,
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, name, slug, billing_email, settings, status, created_at, updated_at, deleted_at;

-- name: CheckOrganizationSlugExists :one
SELECT EXISTS(
    SELECT 1 FROM organizations
    WHERE slug = $1 AND deleted_at IS NULL AND id != COALESCE(sqlc.narg('exclude_id')::uuid, '00000000-0000-0000-0000-000000000000'::uuid)
) as exists;

-- API Keys CRUD queries
-- Table: api_keys

-- name: GetAPIKeyByID :one
SELECT id, organization_id, name, key_prefix, key_hash, previous_key_hash, previous_key_expires_at,
       scopes, rate_limit_per_minute, ip_allowlist, expires_at, last_used_at, is_active,
       created_at, updated_at, revoked_at
FROM api_keys
WHERE id = $1 AND revoked_at IS NULL;

-- name: GetAPIKeyByHash :one
SELECT id, organization_id, name, key_prefix, key_hash, previous_key_hash, previous_key_expires_at,
       scopes, rate_limit_per_minute, ip_allowlist, expires_at, last_used_at, is_active,
       created_at, updated_at, revoked_at
FROM api_keys
WHERE key_hash = $1 AND revoked_at IS NULL AND is_active = TRUE
  AND (expires_at IS NULL OR expires_at > NOW());

-- name: GetAPIKeyByPreviousHash :one
SELECT id, organization_id, name, key_prefix, key_hash, previous_key_hash, previous_key_expires_at,
       scopes, rate_limit_per_minute, ip_allowlist, expires_at, last_used_at, is_active,
       created_at, updated_at, revoked_at
FROM api_keys
WHERE previous_key_hash = $1 AND revoked_at IS NULL AND is_active = TRUE
  AND previous_key_expires_at > NOW()
  AND (expires_at IS NULL OR expires_at > NOW());

-- name: ListAPIKeysByOrganization :many
SELECT id, organization_id, name, key_prefix, key_hash, previous_key_hash, previous_key_expires_at,
       scopes, rate_limit_per_minute, ip_allowlist, expires_at, last_used_at, is_active,
       created_at, updated_at, revoked_at
FROM api_keys
WHERE organization_id = $1 AND revoked_at IS NULL
ORDER BY created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: CountAPIKeysByOrganization :one
SELECT COUNT(*) as total
FROM api_keys
WHERE organization_id = $1 AND revoked_at IS NULL;

-- name: CreateAPIKey :one
INSERT INTO api_keys (
    organization_id,
    name,
    key_prefix,
    key_hash,
    scopes,
    rate_limit_per_minute,
    ip_allowlist,
    expires_at,
    is_active
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING id, organization_id, name, key_prefix, key_hash, previous_key_hash, previous_key_expires_at,
          scopes, rate_limit_per_minute, ip_allowlist, expires_at, last_used_at, is_active,
          created_at, updated_at, revoked_at;

-- name: UpdateAPIKey :one
UPDATE api_keys
SET
    name = COALESCE(sqlc.narg('name'), name),
    scopes = COALESCE(sqlc.narg('scopes'), scopes),
    rate_limit_per_minute = COALESCE(sqlc.narg('rate_limit_per_minute'), rate_limit_per_minute),
    ip_allowlist = COALESCE(sqlc.narg('ip_allowlist'), ip_allowlist),
    expires_at = COALESCE(sqlc.narg('expires_at'), expires_at),
    is_active = COALESCE(sqlc.narg('is_active'), is_active),
    updated_at = NOW()
WHERE id = $1 AND revoked_at IS NULL
RETURNING id, organization_id, name, key_prefix, key_hash, previous_key_hash, previous_key_expires_at,
          scopes, rate_limit_per_minute, ip_allowlist, expires_at, last_used_at, is_active,
          created_at, updated_at, revoked_at;

-- name: RotateAPIKey :one
UPDATE api_keys
SET
    previous_key_hash = key_hash,
    previous_key_expires_at = $3,
    key_hash = $2,
    key_prefix = $4,
    updated_at = NOW()
WHERE id = $1 AND revoked_at IS NULL
RETURNING id, organization_id, name, key_prefix, key_hash, previous_key_hash, previous_key_expires_at,
          scopes, rate_limit_per_minute, ip_allowlist, expires_at, last_used_at, is_active,
          created_at, updated_at, revoked_at;

-- name: UpdateAPIKeyLastUsed :exec
UPDATE api_keys
SET last_used_at = NOW()
WHERE id = $1;

-- name: RevokeAPIKey :one
UPDATE api_keys
SET
    revoked_at = NOW(),
    is_active = FALSE,
    updated_at = NOW()
WHERE id = $1 AND revoked_at IS NULL
RETURNING id, organization_id, name, key_prefix, key_hash, previous_key_hash, previous_key_expires_at,
          scopes, rate_limit_per_minute, ip_allowlist, expires_at, last_used_at, is_active,
          created_at, updated_at, revoked_at;

-- name: DeactivateAPIKey :one
UPDATE api_keys
SET
    is_active = FALSE,
    updated_at = NOW()
WHERE id = $1 AND revoked_at IS NULL
RETURNING id, organization_id, name, key_prefix, key_hash, previous_key_hash, previous_key_expires_at,
          scopes, rate_limit_per_minute, ip_allowlist, expires_at, last_used_at, is_active,
          created_at, updated_at, revoked_at;

-- name: ActivateAPIKey :one
UPDATE api_keys
SET
    is_active = TRUE,
    updated_at = NOW()
WHERE id = $1 AND revoked_at IS NULL
RETURNING id, organization_id, name, key_prefix, key_hash, previous_key_hash, previous_key_expires_at,
          scopes, rate_limit_per_minute, ip_allowlist, expires_at, last_used_at, is_active,
          created_at, updated_at, revoked_at;

-- name: ClearExpiredPreviousKeys :exec
UPDATE api_keys
SET
    previous_key_hash = NULL,
    previous_key_expires_at = NULL,
    updated_at = NOW()
WHERE previous_key_expires_at IS NOT NULL AND previous_key_expires_at <= NOW();

-- name: ListExpiredAPIKeys :many
SELECT id, organization_id, name, key_prefix, expires_at
FROM api_keys
WHERE revoked_at IS NULL AND expires_at IS NOT NULL AND expires_at <= NOW();

-- Provider Accounts CRUD queries
-- Table: provider_accounts

-- name: GetProviderAccountByID :one
SELECT id, organization_id, provider_code, name, credentials_encrypted, credentials_iv,
       status, verification_error, verified_at, last_used_at, is_default,
       created_at, updated_at, deleted_at
FROM provider_accounts
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListProviderAccountsByOrganization :many
SELECT id, organization_id, provider_code, name, credentials_encrypted, credentials_iv,
       status, verification_error, verified_at, last_used_at, is_default,
       created_at, updated_at, deleted_at
FROM provider_accounts
WHERE organization_id = $1 AND deleted_at IS NULL
  AND (sqlc.narg('provider_code')::varchar IS NULL OR provider_code = sqlc.narg('provider_code')::varchar)
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status')::varchar)
ORDER BY provider_code, is_default DESC, created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: CountProviderAccountsByOrganization :one
SELECT COUNT(*) as total
FROM provider_accounts
WHERE organization_id = $1 AND deleted_at IS NULL
  AND (sqlc.narg('provider_code')::varchar IS NULL OR provider_code = sqlc.narg('provider_code')::varchar)
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status')::varchar);

-- name: GetDefaultProviderAccount :one
SELECT id, organization_id, provider_code, name, credentials_encrypted, credentials_iv,
       status, verification_error, verified_at, last_used_at, is_default,
       created_at, updated_at, deleted_at
FROM provider_accounts
WHERE organization_id = $1 AND provider_code = $2 AND is_default = TRUE 
  AND deleted_at IS NULL AND status = 'verified'
LIMIT 1;

-- name: GetVerifiedProviderAccount :one
SELECT id, organization_id, provider_code, name, credentials_encrypted, credentials_iv,
       status, verification_error, verified_at, last_used_at, is_default,
       created_at, updated_at, deleted_at
FROM provider_accounts
WHERE id = $1 AND deleted_at IS NULL AND status = 'verified';

-- name: ListVerifiedProviderAccountsByOrganization :many
SELECT id, organization_id, provider_code, name, credentials_encrypted, credentials_iv,
       status, verification_error, verified_at, last_used_at, is_default,
       created_at, updated_at, deleted_at
FROM provider_accounts
WHERE organization_id = $1 AND deleted_at IS NULL AND status = 'verified'
ORDER BY provider_code, is_default DESC, created_at DESC;

-- name: CreateProviderAccount :one
INSERT INTO provider_accounts (
    organization_id,
    provider_code,
    name,
    credentials_encrypted,
    credentials_iv,
    status,
    is_default
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING id, organization_id, provider_code, name, credentials_encrypted, credentials_iv,
          status, verification_error, verified_at, last_used_at, is_default,
          created_at, updated_at, deleted_at;

-- name: UpdateProviderAccount :one
UPDATE provider_accounts
SET
    name = COALESCE(sqlc.narg('name'), name),
    credentials_encrypted = COALESCE(sqlc.narg('credentials_encrypted'), credentials_encrypted),
    credentials_iv = COALESCE(sqlc.narg('credentials_iv'), credentials_iv),
    is_default = COALESCE(sqlc.narg('is_default'), is_default),
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, organization_id, provider_code, name, credentials_encrypted, credentials_iv,
          status, verification_error, verified_at, last_used_at, is_default,
          created_at, updated_at, deleted_at;

-- name: UpdateProviderAccountStatus :one
UPDATE provider_accounts
SET
    status = $2,
    verification_error = $3,
    verified_at = CASE WHEN $2 = 'verified' THEN NOW() ELSE verified_at END,
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, organization_id, provider_code, name, credentials_encrypted, credentials_iv,
          status, verification_error, verified_at, last_used_at, is_default,
          created_at, updated_at, deleted_at;

-- name: UpdateProviderAccountLastUsed :exec
UPDATE provider_accounts
SET last_used_at = NOW()
WHERE id = $1;

-- name: SoftDeleteProviderAccount :one
UPDATE provider_accounts
SET
    deleted_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, organization_id, provider_code, name, credentials_encrypted, credentials_iv,
          status, verification_error, verified_at, last_used_at, is_default,
          created_at, updated_at, deleted_at;

-- name: ClearDefaultProviderAccount :exec
UPDATE provider_accounts
SET
    is_default = FALSE,
    updated_at = NOW()
WHERE organization_id = $1 AND provider_code = $2 AND is_default = TRUE AND deleted_at IS NULL;

-- name: SetDefaultProviderAccount :one
UPDATE provider_accounts
SET
    is_default = TRUE,
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL AND status = 'verified'
RETURNING id, organization_id, provider_code, name, credentials_encrypted, credentials_iv,
          status, verification_error, verified_at, last_used_at, is_default,
          created_at, updated_at, deleted_at;

-- name: SuspendProviderAccount :one
UPDATE provider_accounts
SET
    status = 'suspended',
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, organization_id, provider_code, name, credentials_encrypted, credentials_iv,
          status, verification_error, verified_at, last_used_at, is_default,
          created_at, updated_at, deleted_at;

-- name: GetProviderAccountsByIDs :many
SELECT id, organization_id, provider_code, name, credentials_encrypted, credentials_iv,
       status, verification_error, verified_at, last_used_at, is_default,
       created_at, updated_at, deleted_at
FROM provider_accounts
WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL AND status = 'verified';

-- name: HardDeleteExpiredProviderAccounts :exec
DELETE FROM provider_accounts
WHERE deleted_at IS NOT NULL AND deleted_at < NOW() - INTERVAL '30 days';

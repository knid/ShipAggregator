-- Idempotency CRUD queries
-- Table: idempotency_records

-- name: GetIdempotencyRecord :one
SELECT id, organization_id, idempotency_key, endpoint, request_hash,
       response_status, response_body, created_at, expires_at
FROM idempotency_records
WHERE organization_id = $1 AND idempotency_key = $2 AND endpoint = $3;

-- name: GetIdempotencyRecordByID :one
SELECT id, organization_id, idempotency_key, endpoint, request_hash,
       response_status, response_body, created_at, expires_at
FROM idempotency_records
WHERE id = $1;

-- name: GetIdempotencyRecordForUpdate :one
SELECT id, organization_id, idempotency_key, endpoint, request_hash,
       response_status, response_body, created_at, expires_at
FROM idempotency_records
WHERE organization_id = $1 AND idempotency_key = $2 AND endpoint = $3
FOR UPDATE;

-- name: CreateIdempotencyRecord :one
INSERT INTO idempotency_records (
    organization_id,
    idempotency_key,
    endpoint,
    request_hash,
    response_status,
    response_body,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING id, organization_id, idempotency_key, endpoint, request_hash,
          response_status, response_body, created_at, expires_at;

-- name: DeleteExpiredIdempotencyRecords :exec
DELETE FROM idempotency_records
WHERE expires_at < NOW();

-- name: CheckIdempotencyRecordExists :one
SELECT EXISTS(
    SELECT 1 FROM idempotency_records
    WHERE organization_id = $1 AND idempotency_key = $2 AND endpoint = $3
) as exists;

-- name: ListIdempotencyRecordsByOrganization :many
SELECT id, organization_id, idempotency_key, endpoint, request_hash,
       response_status, response_body, created_at, expires_at
FROM idempotency_records
WHERE organization_id = $1
ORDER BY created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: CountIdempotencyRecordsByOrganization :one
SELECT COUNT(*) as total
FROM idempotency_records
WHERE organization_id = $1;

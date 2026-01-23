-- Topups CRUD queries
-- Table: topups

-- name: GetTopupByID :one
SELECT id, organization_id, idempotency_key, amount_kurus, currency, status,
       payment_provider, payment_id, payment_url, payment_token, ledger_entry_id,
       failure_reason, completed_at, expires_at, created_at, updated_at
FROM topups
WHERE id = $1;

-- name: GetTopupByIdempotencyKey :one
SELECT id, organization_id, idempotency_key, amount_kurus, currency, status,
       payment_provider, payment_id, payment_url, payment_token, ledger_entry_id,
       failure_reason, completed_at, expires_at, created_at, updated_at
FROM topups
WHERE organization_id = $1 AND idempotency_key = $2;

-- name: GetTopupByPaymentID :one
SELECT id, organization_id, idempotency_key, amount_kurus, currency, status,
       payment_provider, payment_id, payment_url, payment_token, ledger_entry_id,
       failure_reason, completed_at, expires_at, created_at, updated_at
FROM topups
WHERE payment_provider = $1 AND payment_id = $2;

-- name: GetTopupByPaymentToken :one
SELECT id, organization_id, idempotency_key, amount_kurus, currency, status,
       payment_provider, payment_id, payment_url, payment_token, ledger_entry_id,
       failure_reason, completed_at, expires_at, created_at, updated_at
FROM topups
WHERE payment_token = $1;

-- name: ListTopupsByOrganization :many
SELECT id, organization_id, idempotency_key, amount_kurus, currency, status,
       payment_provider, payment_id, payment_url, payment_token, ledger_entry_id,
       failure_reason, completed_at, expires_at, created_at, updated_at
FROM topups
WHERE organization_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status')::varchar)
ORDER BY created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: CountTopupsByOrganization :one
SELECT COUNT(*) as total
FROM topups
WHERE organization_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status')::varchar);

-- name: CreateTopup :one
INSERT INTO topups (
    organization_id,
    idempotency_key,
    amount_kurus,
    currency,
    status,
    payment_provider,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING id, organization_id, idempotency_key, amount_kurus, currency, status,
          payment_provider, payment_id, payment_url, payment_token, ledger_entry_id,
          failure_reason, completed_at, expires_at, created_at, updated_at;

-- name: UpdateTopupPaymentInitiated :one
UPDATE topups
SET
    status = 'processing',
    payment_id = $2,
    payment_url = $3,
    payment_token = $4,
    updated_at = NOW()
WHERE id = $1 AND status = 'pending'
RETURNING id, organization_id, idempotency_key, amount_kurus, currency, status,
          payment_provider, payment_id, payment_url, payment_token, ledger_entry_id,
          failure_reason, completed_at, expires_at, created_at, updated_at;

-- name: UpdateTopupCompleted :one
UPDATE topups
SET
    status = 'completed',
    ledger_entry_id = $2,
    completed_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND status = 'processing'
RETURNING id, organization_id, idempotency_key, amount_kurus, currency, status,
          payment_provider, payment_id, payment_url, payment_token, ledger_entry_id,
          failure_reason, completed_at, expires_at, created_at, updated_at;

-- name: UpdateTopupFailed :one
UPDATE topups
SET
    status = 'failed',
    failure_reason = $2,
    updated_at = NOW()
WHERE id = $1 AND status IN ('pending', 'processing')
RETURNING id, organization_id, idempotency_key, amount_kurus, currency, status,
          payment_provider, payment_id, payment_url, payment_token, ledger_entry_id,
          failure_reason, completed_at, expires_at, created_at, updated_at;

-- name: UpdateTopupCancelled :one
UPDATE topups
SET
    status = 'cancelled',
    updated_at = NOW()
WHERE id = $1 AND status IN ('pending', 'processing')
RETURNING id, organization_id, idempotency_key, amount_kurus, currency, status,
          payment_provider, payment_id, payment_url, payment_token, ledger_entry_id,
          failure_reason, completed_at, expires_at, created_at, updated_at;

-- name: ExpirePendingTopups :exec
UPDATE topups
SET
    status = 'failed',
    failure_reason = 'Payment expired',
    updated_at = NOW()
WHERE status IN ('pending', 'processing') AND expires_at <= NOW();

-- name: GetTopupSummaryByOrganization :one
SELECT
    COUNT(*) as total_topups,
    COALESCE(SUM(CASE WHEN status = 'completed' THEN amount_kurus ELSE 0 END), 0) as total_completed_kurus,
    COALESCE(SUM(CASE WHEN status = 'pending' OR status = 'processing' THEN amount_kurus ELSE 0 END), 0) as total_pending_kurus
FROM topups
WHERE organization_id = $1;

-- name: ListTopupsByDateRange :many
SELECT id, organization_id, idempotency_key, amount_kurus, currency, status,
       payment_provider, payment_id, payment_url, payment_token, ledger_entry_id,
       failure_reason, completed_at, expires_at, created_at, updated_at
FROM topups
WHERE created_at >= $1 AND created_at < $2
ORDER BY created_at DESC;

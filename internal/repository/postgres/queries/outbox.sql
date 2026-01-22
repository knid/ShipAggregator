-- Outbox CRUD queries (Transactional Outbox Pattern)
-- Table: outbox

-- name: GetOutboxMessageByID :one
SELECT id, aggregate_type, aggregate_id, event_type, payload,
       published_at, retry_count, created_at
FROM outbox
WHERE id = $1;

-- name: ListPendingOutboxMessages :many
SELECT id, aggregate_type, aggregate_id, event_type, payload,
       published_at, retry_count, created_at
FROM outbox
WHERE published_at IS NULL
ORDER BY created_at ASC
LIMIT sqlc.arg('limit_count')::int
FOR UPDATE SKIP LOCKED;

-- name: ListOutboxMessagesByAggregateID :many
SELECT id, aggregate_type, aggregate_id, event_type, payload,
       published_at, retry_count, created_at
FROM outbox
WHERE aggregate_type = $1 AND aggregate_id = $2
ORDER BY created_at DESC
LIMIT sqlc.arg('limit_count')::int;

-- name: ListPublishedOutboxMessages :many
SELECT id, aggregate_type, aggregate_id, event_type, payload,
       published_at, retry_count, created_at
FROM outbox
WHERE published_at IS NOT NULL
ORDER BY published_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: CountPendingOutboxMessages :one
SELECT COUNT(*) as count
FROM outbox
WHERE published_at IS NULL;

-- name: CreateOutboxMessage :one
INSERT INTO outbox (
    aggregate_type,
    aggregate_id,
    event_type,
    payload
) VALUES (
    $1, $2, $3, $4
)
RETURNING id, aggregate_type, aggregate_id, event_type, payload,
          published_at, retry_count, created_at;

-- name: CreateOutboxMessagesBatch :copyfrom
INSERT INTO outbox (
    aggregate_type,
    aggregate_id,
    event_type,
    payload
) VALUES (
    $1, $2, $3, $4
);

-- name: UpdateOutboxMessagePublished :one
UPDATE outbox
SET
    published_at = NOW(),
    retry_count = retry_count + 1
WHERE id = $1 AND published_at IS NULL
RETURNING id, aggregate_type, aggregate_id, event_type, payload,
          published_at, retry_count, created_at;

-- name: UpdateOutboxMessageRetry :one
UPDATE outbox
SET retry_count = retry_count + 1
WHERE id = $1
RETURNING id, aggregate_type, aggregate_id, event_type, payload,
          published_at, retry_count, created_at;

-- name: DeletePublishedOutboxMessages :exec
DELETE FROM outbox
WHERE published_at IS NOT NULL AND published_at < NOW() - INTERVAL '7 days';

-- name: DeleteOldOutboxMessages :exec
DELETE FROM outbox
WHERE created_at < NOW() - INTERVAL '30 days';

-- name: GetOutboxStats :one
SELECT 
    COUNT(*) FILTER (WHERE published_at IS NULL) as pending_count,
    COUNT(*) FILTER (WHERE published_at IS NOT NULL) as published_count,
    MAX(created_at) FILTER (WHERE published_at IS NULL) as oldest_pending,
    AVG(retry_count) FILTER (WHERE published_at IS NOT NULL) as avg_retries
FROM outbox;

-- name: ListFailedOutboxMessages :many
SELECT id, aggregate_type, aggregate_id, event_type, payload,
       published_at, retry_count, created_at
FROM outbox
WHERE published_at IS NULL AND retry_count >= $1
ORDER BY created_at ASC
LIMIT sqlc.arg('limit_count')::int;

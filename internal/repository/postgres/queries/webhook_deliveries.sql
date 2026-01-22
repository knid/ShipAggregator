-- Webhook Deliveries CRUD queries
-- Tables: webhook_events, webhook_deliveries

-- ============================================================================
-- WEBHOOK EVENTS
-- ============================================================================

-- name: GetWebhookEventByID :one
SELECT id, organization_id, event_type, resource_type, resource_id, payload, created_at
FROM webhook_events
WHERE id = $1;

-- name: ListWebhookEventsByOrganization :many
SELECT id, organization_id, event_type, resource_type, resource_id, payload, created_at
FROM webhook_events
WHERE organization_id = $1
  AND (sqlc.narg('event_type')::varchar IS NULL OR event_type = sqlc.narg('event_type')::varchar)
  AND (sqlc.narg('resource_type')::varchar IS NULL OR resource_type = sqlc.narg('resource_type')::varchar)
ORDER BY created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: CountWebhookEventsByOrganization :one
SELECT COUNT(*) as total
FROM webhook_events
WHERE organization_id = $1
  AND (sqlc.narg('event_type')::varchar IS NULL OR event_type = sqlc.narg('event_type')::varchar)
  AND (sqlc.narg('resource_type')::varchar IS NULL OR resource_type = sqlc.narg('resource_type')::varchar);

-- name: CreateWebhookEvent :one
INSERT INTO webhook_events (
    organization_id,
    event_type,
    resource_type,
    resource_id,
    payload
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING id, organization_id, event_type, resource_type, resource_id, payload, created_at;

-- name: DeleteOldWebhookEvents :exec
DELETE FROM webhook_events
WHERE created_at < NOW() - INTERVAL '90 days';

-- ============================================================================
-- WEBHOOK DELIVERIES
-- ============================================================================

-- name: GetWebhookDeliveryByID :one
SELECT id, event_id, subscription_id, endpoint_url, status, attempts,
       last_attempt_at, next_retry_at, response_status, response_body_preview,
       response_time_ms, error_message, created_at, updated_at
FROM webhook_deliveries
WHERE id = $1;

-- name: ListWebhookDeliveriesByEvent :many
SELECT id, event_id, subscription_id, endpoint_url, status, attempts,
       last_attempt_at, next_retry_at, response_status, response_body_preview,
       response_time_ms, error_message, created_at, updated_at
FROM webhook_deliveries
WHERE event_id = $1
ORDER BY created_at DESC;

-- name: ListWebhookDeliveriesByOrganization :many
SELECT d.id, d.event_id, d.subscription_id, d.endpoint_url, d.status, d.attempts,
       d.last_attempt_at, d.next_retry_at, d.response_status, d.response_body_preview,
       d.response_time_ms, d.error_message, d.created_at, d.updated_at,
       e.event_type, e.resource_type, e.resource_id
FROM webhook_deliveries d
JOIN webhook_events e ON d.event_id = e.id
WHERE e.organization_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR d.status = sqlc.narg('status')::varchar)
ORDER BY d.created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: CountWebhookDeliveriesByOrganization :one
SELECT COUNT(*) as total
FROM webhook_deliveries d
JOIN webhook_events e ON d.event_id = e.id
WHERE e.organization_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR d.status = sqlc.narg('status')::varchar);

-- name: ListPendingWebhookDeliveries :many
SELECT id, event_id, subscription_id, endpoint_url, status, attempts,
       last_attempt_at, next_retry_at, response_status, response_body_preview,
       response_time_ms, error_message, created_at, updated_at
FROM webhook_deliveries
WHERE status = 'pending' AND (next_retry_at IS NULL OR next_retry_at <= NOW())
ORDER BY COALESCE(next_retry_at, created_at) ASC
LIMIT sqlc.arg('limit_count')::int
FOR UPDATE SKIP LOCKED;

-- name: ListDLQWebhookDeliveries :many
SELECT d.id, d.event_id, d.subscription_id, d.endpoint_url, d.status, d.attempts,
       d.last_attempt_at, d.next_retry_at, d.response_status, d.response_body_preview,
       d.response_time_ms, d.error_message, d.created_at, d.updated_at,
       e.organization_id, e.event_type, e.resource_type, e.resource_id
FROM webhook_deliveries d
JOIN webhook_events e ON d.event_id = e.id
WHERE d.status = 'dlq'
  AND (sqlc.narg('organization_id')::uuid IS NULL OR e.organization_id = sqlc.narg('organization_id')::uuid)
ORDER BY d.created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: CreateWebhookDelivery :one
INSERT INTO webhook_deliveries (
    event_id,
    subscription_id,
    endpoint_url,
    status
) VALUES (
    $1, $2, $3, $4
)
RETURNING id, event_id, subscription_id, endpoint_url, status, attempts,
          last_attempt_at, next_retry_at, response_status, response_body_preview,
          response_time_ms, error_message, created_at, updated_at;

-- name: UpdateWebhookDeliverySuccess :one
UPDATE webhook_deliveries
SET
    status = 'delivered',
    attempts = attempts + 1,
    last_attempt_at = NOW(),
    next_retry_at = NULL,
    response_status = $2,
    response_body_preview = $3,
    response_time_ms = $4,
    error_message = NULL,
    updated_at = NOW()
WHERE id = $1
RETURNING id, event_id, subscription_id, endpoint_url, status, attempts,
          last_attempt_at, next_retry_at, response_status, response_body_preview,
          response_time_ms, error_message, created_at, updated_at;

-- name: UpdateWebhookDeliveryFailure :one
UPDATE webhook_deliveries
SET
    status = CASE WHEN attempts + 1 >= 7 THEN 'dlq' ELSE 'pending' END,
    attempts = attempts + 1,
    last_attempt_at = NOW(),
    next_retry_at = CASE 
        WHEN attempts + 1 >= 7 THEN NULL
        WHEN attempts = 0 THEN NOW() + INTERVAL '1 minute'
        WHEN attempts = 1 THEN NOW() + INTERVAL '5 minutes'
        WHEN attempts = 2 THEN NOW() + INTERVAL '30 minutes'
        WHEN attempts = 3 THEN NOW() + INTERVAL '2 hours'
        WHEN attempts = 4 THEN NOW() + INTERVAL '8 hours'
        ELSE NOW() + INTERVAL '24 hours'
    END,
    response_status = $2,
    response_body_preview = $3,
    response_time_ms = $4,
    error_message = $5,
    updated_at = NOW()
WHERE id = $1
RETURNING id, event_id, subscription_id, endpoint_url, status, attempts,
          last_attempt_at, next_retry_at, response_status, response_body_preview,
          response_time_ms, error_message, created_at, updated_at;

-- name: ReplayWebhookDelivery :one
UPDATE webhook_deliveries
SET
    status = 'pending',
    next_retry_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND status = 'dlq'
RETURNING id, event_id, subscription_id, endpoint_url, status, attempts,
          last_attempt_at, next_retry_at, response_status, response_body_preview,
          response_time_ms, error_message, created_at, updated_at;

-- name: ReplayDLQWebhookDeliveries :exec
UPDATE webhook_deliveries
SET
    status = 'pending',
    next_retry_at = NOW(),
    updated_at = NOW()
WHERE status = 'dlq'
  AND event_id IN (
      SELECT id FROM webhook_events WHERE organization_id = sqlc.narg('organization_id')::uuid
  );

-- name: DeleteOldWebhookDeliveries :exec
DELETE FROM webhook_deliveries
WHERE created_at < NOW() - INTERVAL '90 days';

-- name: CountWebhookDeliveriesByStatus :many
SELECT status, COUNT(*) as count
FROM webhook_deliveries d
JOIN webhook_events e ON d.event_id = e.id
WHERE e.organization_id = $1
GROUP BY status;

-- Webhook Subscriptions CRUD queries
-- Table: webhook_subscriptions

-- name: GetWebhookSubscriptionByID :one
SELECT id, endpoint_id, event_type, is_active, created_at
FROM webhook_subscriptions
WHERE id = $1;

-- name: ListWebhookSubscriptionsByEndpoint :many
SELECT id, endpoint_id, event_type, is_active, created_at
FROM webhook_subscriptions
WHERE endpoint_id = $1
ORDER BY event_type ASC;

-- name: ListActiveWebhookSubscriptionsByEndpoint :many
SELECT id, endpoint_id, event_type, is_active, created_at
FROM webhook_subscriptions
WHERE endpoint_id = $1 AND is_active = TRUE
ORDER BY event_type ASC;

-- name: ListWebhookSubscriptionsByEventType :many
SELECT s.id, s.endpoint_id, s.event_type, s.is_active, s.created_at,
       e.organization_id, e.url, e.secret_hash, e.is_active as endpoint_is_active
FROM webhook_subscriptions s
JOIN webhook_endpoints e ON s.endpoint_id = e.id
WHERE s.event_type = $1 AND s.is_active = TRUE AND e.is_active = TRUE;

-- name: ListWebhookSubscriptionsByOrganizationAndEventType :many
SELECT s.id, s.endpoint_id, s.event_type, s.is_active, s.created_at,
       e.organization_id, e.url, e.secret_hash
FROM webhook_subscriptions s
JOIN webhook_endpoints e ON s.endpoint_id = e.id
WHERE e.organization_id = $1 AND s.event_type = $2 AND s.is_active = TRUE AND e.is_active = TRUE;

-- name: CreateWebhookSubscription :one
INSERT INTO webhook_subscriptions (
    endpoint_id,
    event_type,
    is_active
) VALUES (
    $1, $2, $3
)
RETURNING id, endpoint_id, event_type, is_active, created_at;

-- name: UpdateWebhookSubscription :one
UPDATE webhook_subscriptions
SET is_active = $2
WHERE id = $1
RETURNING id, endpoint_id, event_type, is_active, created_at;

-- name: DeleteWebhookSubscription :exec
DELETE FROM webhook_subscriptions
WHERE id = $1;

-- name: DeleteWebhookSubscriptionsByEndpoint :exec
DELETE FROM webhook_subscriptions
WHERE endpoint_id = $1;

-- name: ActivateWebhookSubscription :one
UPDATE webhook_subscriptions
SET is_active = TRUE
WHERE id = $1
RETURNING id, endpoint_id, event_type, is_active, created_at;

-- name: DeactivateWebhookSubscription :one
UPDATE webhook_subscriptions
SET is_active = FALSE
WHERE id = $1
RETURNING id, endpoint_id, event_type, is_active, created_at;

-- name: CheckWebhookSubscriptionExists :one
SELECT EXISTS(
    SELECT 1 FROM webhook_subscriptions
    WHERE endpoint_id = $1 AND event_type = $2
) as exists;

-- name: GetWebhookSubscriptionByEndpointAndEventType :one
SELECT id, endpoint_id, event_type, is_active, created_at
FROM webhook_subscriptions
WHERE endpoint_id = $1 AND event_type = $2;

-- name: ListDistinctEventTypes :many
SELECT DISTINCT event_type
FROM webhook_subscriptions
ORDER BY event_type ASC;

-- name: CountWebhookSubscriptionsByEndpoint :one
SELECT COUNT(*) as total
FROM webhook_subscriptions
WHERE endpoint_id = $1;

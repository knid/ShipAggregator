-- Webhook Endpoints CRUD queries
-- Table: webhook_endpoints

-- name: GetWebhookEndpointByID :one
SELECT id, organization_id, url, secret_hash, description, is_active, created_at, updated_at
FROM webhook_endpoints
WHERE id = $1;

-- name: ListWebhookEndpointsByOrganization :many
SELECT id, organization_id, url, secret_hash, description, is_active, created_at, updated_at
FROM webhook_endpoints
WHERE organization_id = $1
ORDER BY created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: ListActiveWebhookEndpointsByOrganization :many
SELECT id, organization_id, url, secret_hash, description, is_active, created_at, updated_at
FROM webhook_endpoints
WHERE organization_id = $1 AND is_active = TRUE
ORDER BY created_at DESC;

-- name: CountWebhookEndpointsByOrganization :one
SELECT COUNT(*) as total
FROM webhook_endpoints
WHERE organization_id = $1;

-- name: CreateWebhookEndpoint :one
INSERT INTO webhook_endpoints (
    organization_id,
    url,
    secret_hash,
    description,
    is_active
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING id, organization_id, url, secret_hash, description, is_active, created_at, updated_at;

-- name: UpdateWebhookEndpoint :one
UPDATE webhook_endpoints
SET
    url = COALESCE(sqlc.narg('url'), url),
    secret_hash = COALESCE(sqlc.narg('secret_hash'), secret_hash),
    description = COALESCE(sqlc.narg('description'), description),
    is_active = COALESCE(sqlc.narg('is_active'), is_active),
    updated_at = NOW()
WHERE id = $1
RETURNING id, organization_id, url, secret_hash, description, is_active, created_at, updated_at;

-- name: DeleteWebhookEndpoint :exec
DELETE FROM webhook_endpoints
WHERE id = $1;

-- name: ActivateWebhookEndpoint :one
UPDATE webhook_endpoints
SET
    is_active = TRUE,
    updated_at = NOW()
WHERE id = $1
RETURNING id, organization_id, url, secret_hash, description, is_active, created_at, updated_at;

-- name: DeactivateWebhookEndpoint :one
UPDATE webhook_endpoints
SET
    is_active = FALSE,
    updated_at = NOW()
WHERE id = $1
RETURNING id, organization_id, url, secret_hash, description, is_active, created_at, updated_at;

-- name: CheckWebhookEndpointURLExists :one
SELECT EXISTS(
    SELECT 1 FROM webhook_endpoints
    WHERE organization_id = $1 AND url = $2
      AND id != COALESCE(sqlc.narg('exclude_id')::uuid, '00000000-0000-0000-0000-000000000000'::uuid)
) as exists;

-- name: GetWebhookEndpointWithSubscriptions :many
SELECT e.id, e.organization_id, e.url, e.secret_hash, e.description, e.is_active,
       e.created_at, e.updated_at,
       s.id as subscription_id, s.event_type, s.is_active as subscription_is_active
FROM webhook_endpoints e
LEFT JOIN webhook_subscriptions s ON e.id = s.endpoint_id
WHERE e.id = $1
ORDER BY s.event_type;

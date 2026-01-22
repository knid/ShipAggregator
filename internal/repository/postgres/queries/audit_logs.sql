-- Audit Logs queries (INSERT and READ only - immutable table)
-- Table: audit_logs

-- name: GetAuditLogByID :one
SELECT id, organization_id, actor_id, actor_type, action, resource_type, resource_id,
       old_value, new_value, ip_address, user_agent, trace_id, created_at
FROM audit_logs
WHERE id = $1;

-- name: ListAuditLogsByOrganization :many
SELECT id, organization_id, actor_id, actor_type, action, resource_type, resource_id,
       old_value, new_value, ip_address, user_agent, trace_id, created_at
FROM audit_logs
WHERE organization_id = $1
  AND (sqlc.narg('actor_id')::uuid IS NULL OR actor_id = sqlc.narg('actor_id')::uuid)
  AND (sqlc.narg('actor_type')::varchar IS NULL OR actor_type = sqlc.narg('actor_type')::varchar)
  AND (sqlc.narg('action')::varchar IS NULL OR action = sqlc.narg('action')::varchar)
  AND (sqlc.narg('resource_type')::varchar IS NULL OR resource_type = sqlc.narg('resource_type')::varchar)
  AND (sqlc.narg('resource_id')::uuid IS NULL OR resource_id = sqlc.narg('resource_id')::uuid)
  AND (sqlc.narg('created_after')::timestamptz IS NULL OR created_at >= sqlc.narg('created_after')::timestamptz)
  AND (sqlc.narg('created_before')::timestamptz IS NULL OR created_at <= sqlc.narg('created_before')::timestamptz)
ORDER BY created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: CountAuditLogsByOrganization :one
SELECT COUNT(*) as total
FROM audit_logs
WHERE organization_id = $1
  AND (sqlc.narg('actor_id')::uuid IS NULL OR actor_id = sqlc.narg('actor_id')::uuid)
  AND (sqlc.narg('actor_type')::varchar IS NULL OR actor_type = sqlc.narg('actor_type')::varchar)
  AND (sqlc.narg('action')::varchar IS NULL OR action = sqlc.narg('action')::varchar)
  AND (sqlc.narg('resource_type')::varchar IS NULL OR resource_type = sqlc.narg('resource_type')::varchar)
  AND (sqlc.narg('resource_id')::uuid IS NULL OR resource_id = sqlc.narg('resource_id')::uuid)
  AND (sqlc.narg('created_after')::timestamptz IS NULL OR created_at >= sqlc.narg('created_after')::timestamptz)
  AND (sqlc.narg('created_before')::timestamptz IS NULL OR created_at <= sqlc.narg('created_before')::timestamptz);

-- name: ListAuditLogsByResource :many
SELECT id, organization_id, actor_id, actor_type, action, resource_type, resource_id,
       old_value, new_value, ip_address, user_agent, trace_id, created_at
FROM audit_logs
WHERE resource_type = $1 AND resource_id = $2
ORDER BY created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: ListAuditLogsByActor :many
SELECT id, organization_id, actor_id, actor_type, action, resource_type, resource_id,
       old_value, new_value, ip_address, user_agent, trace_id, created_at
FROM audit_logs
WHERE actor_id = $1 AND actor_type = $2
ORDER BY created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: ListAuditLogsByTraceID :many
SELECT id, organization_id, actor_id, actor_type, action, resource_type, resource_id,
       old_value, new_value, ip_address, user_agent, trace_id, created_at
FROM audit_logs
WHERE trace_id = $1
ORDER BY created_at ASC;

-- name: CreateAuditLog :one
INSERT INTO audit_logs (
    organization_id,
    actor_id,
    actor_type,
    action,
    resource_type,
    resource_id,
    old_value,
    new_value,
    ip_address,
    user_agent,
    trace_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING id, organization_id, actor_id, actor_type, action, resource_type, resource_id,
          old_value, new_value, ip_address, user_agent, trace_id, created_at;

-- name: CreateAuditLogsBatch :copyfrom
INSERT INTO audit_logs (
    organization_id,
    actor_id,
    actor_type,
    action,
    resource_type,
    resource_id,
    old_value,
    new_value,
    ip_address,
    user_agent,
    trace_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
);

-- name: GetAuditLogSummaryByOrganization :many
SELECT 
    action,
    resource_type,
    COUNT(*) as count
FROM audit_logs
WHERE organization_id = $1
  AND created_at >= $2
  AND created_at <= $3
GROUP BY action, resource_type
ORDER BY count DESC;

-- name: GetAuditLogActivityByActor :many
SELECT 
    actor_id,
    actor_type,
    COUNT(*) as action_count,
    MAX(created_at) as last_activity
FROM audit_logs
WHERE organization_id = $1
  AND created_at >= $2
GROUP BY actor_id, actor_type
ORDER BY action_count DESC
LIMIT sqlc.arg('limit_count')::int;

-- name: DeleteOldAuditLogs :exec
DELETE FROM audit_logs
WHERE created_at < NOW() - INTERVAL '365 days';

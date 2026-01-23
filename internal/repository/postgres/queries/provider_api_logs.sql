-- Provider API Logs queries (INSERT and READ only)
-- Table: provider_api_logs

-- name: GetProviderApiLogByID :one
SELECT id, organization_id, provider_code, operation, request_method, request_url,
       request_headers, request_body_preview, response_status, response_headers,
       response_body_preview, latency_ms, success, error_code, error_message,
       trace_id, created_at
FROM provider_api_logs
WHERE id = $1;

-- name: ListProviderApiLogs :many
SELECT id, organization_id, provider_code, operation, request_method, request_url,
       request_headers, request_body_preview, response_status, response_headers,
       response_body_preview, latency_ms, success, error_code, error_message,
       trace_id, created_at
FROM provider_api_logs
WHERE (sqlc.narg('organization_id')::uuid IS NULL OR organization_id = sqlc.narg('organization_id')::uuid)
  AND (sqlc.narg('provider_code')::varchar IS NULL OR provider_code = sqlc.narg('provider_code')::varchar)
  AND (sqlc.narg('operation')::varchar IS NULL OR operation = sqlc.narg('operation')::varchar)
  AND (sqlc.narg('success')::boolean IS NULL OR success = sqlc.narg('success')::boolean)
  AND (sqlc.narg('created_after')::timestamptz IS NULL OR created_at >= sqlc.narg('created_after')::timestamptz)
  AND (sqlc.narg('created_before')::timestamptz IS NULL OR created_at <= sqlc.narg('created_before')::timestamptz)
ORDER BY created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: CountProviderApiLogs :one
SELECT COUNT(*) as total
FROM provider_api_logs
WHERE (sqlc.narg('organization_id')::uuid IS NULL OR organization_id = sqlc.narg('organization_id')::uuid)
  AND (sqlc.narg('provider_code')::varchar IS NULL OR provider_code = sqlc.narg('provider_code')::varchar)
  AND (sqlc.narg('operation')::varchar IS NULL OR operation = sqlc.narg('operation')::varchar)
  AND (sqlc.narg('success')::boolean IS NULL OR success = sqlc.narg('success')::boolean)
  AND (sqlc.narg('created_after')::timestamptz IS NULL OR created_at >= sqlc.narg('created_after')::timestamptz)
  AND (sqlc.narg('created_before')::timestamptz IS NULL OR created_at <= sqlc.narg('created_before')::timestamptz);

-- name: ListProviderApiLogsByProvider :many
SELECT id, organization_id, provider_code, operation, request_method, request_url,
       request_headers, request_body_preview, response_status, response_headers,
       response_body_preview, latency_ms, success, error_code, error_message,
       trace_id, created_at
FROM provider_api_logs
WHERE provider_code = $1
ORDER BY created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: ListProviderApiLogsByTraceID :many
SELECT id, organization_id, provider_code, operation, request_method, request_url,
       request_headers, request_body_preview, response_status, response_headers,
       response_body_preview, latency_ms, success, error_code, error_message,
       trace_id, created_at
FROM provider_api_logs
WHERE trace_id = $1
ORDER BY created_at ASC;

-- name: CreateProviderApiLog :one
INSERT INTO provider_api_logs (
    organization_id,
    provider_code,
    operation,
    request_method,
    request_url,
    request_headers,
    request_body_preview,
    response_status,
    response_headers,
    response_body_preview,
    latency_ms,
    success,
    error_code,
    error_message,
    trace_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
)
RETURNING id, organization_id, provider_code, operation, request_method, request_url,
          request_headers, request_body_preview, response_status, response_headers,
          response_body_preview, latency_ms, success, error_code, error_message,
          trace_id, created_at;

-- name: GetProviderApiLogStats :many
SELECT 
    provider_code,
    COUNT(*) as total_calls,
    SUM(CASE WHEN success THEN 1 ELSE 0 END) as successful_calls,
    SUM(CASE WHEN NOT success THEN 1 ELSE 0 END) as failed_calls,
    ROUND(AVG(latency_ms)::numeric, 2) as avg_latency_ms,
    MAX(latency_ms) as max_latency_ms,
    MIN(latency_ms) as min_latency_ms
FROM provider_api_logs
WHERE created_at >= $1
  AND created_at <= $2
GROUP BY provider_code
ORDER BY total_calls DESC;

-- name: GetProviderApiLogStatsByOperation :many
SELECT 
    provider_code,
    operation,
    COUNT(*) as total_calls,
    SUM(CASE WHEN success THEN 1 ELSE 0 END) as successful_calls,
    ROUND(100.0 * SUM(CASE WHEN success THEN 1 ELSE 0 END) / COUNT(*), 2) as success_rate,
    ROUND(AVG(latency_ms)::numeric, 2) as avg_latency_ms
FROM provider_api_logs
WHERE provider_code = $1
  AND created_at >= $2
  AND created_at <= $3
GROUP BY provider_code, operation
ORDER BY total_calls DESC;

-- name: GetProviderApiLogErrorSummary :many
SELECT 
    provider_code,
    error_code,
    COUNT(*) as error_count,
    MAX(created_at) as last_occurrence
FROM provider_api_logs
WHERE success = false
  AND created_at >= $1
GROUP BY provider_code, error_code
ORDER BY error_count DESC
LIMIT sqlc.arg('limit_count')::int;

-- name: DeleteOldProviderApiLogs :exec
DELETE FROM provider_api_logs
WHERE created_at < NOW() - INTERVAL '30 days';

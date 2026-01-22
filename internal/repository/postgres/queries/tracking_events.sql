-- Tracking Events CRUD queries
-- Table: tracking_events

-- name: GetTrackingEventByID :one
SELECT id, shipment_id, provider_code, status_code, status_description,
       location_city, location_detail, occurred_at, raw_status_code, raw_data,
       source, created_at
FROM tracking_events
WHERE id = $1;

-- name: ListTrackingEventsByShipment :many
SELECT id, shipment_id, provider_code, status_code, status_description,
       location_city, location_detail, occurred_at, raw_status_code, raw_data,
       source, created_at
FROM tracking_events
WHERE shipment_id = $1
ORDER BY occurred_at DESC;

-- name: ListTrackingEventsByShipmentAsc :many
SELECT id, shipment_id, provider_code, status_code, status_description,
       location_city, location_detail, occurred_at, raw_status_code, raw_data,
       source, created_at
FROM tracking_events
WHERE shipment_id = $1
ORDER BY occurred_at ASC;

-- name: GetLatestTrackingEvent :one
SELECT id, shipment_id, provider_code, status_code, status_description,
       location_city, location_detail, occurred_at, raw_status_code, raw_data,
       source, created_at
FROM tracking_events
WHERE shipment_id = $1
ORDER BY occurred_at DESC
LIMIT 1;

-- name: CreateTrackingEvent :one
INSERT INTO tracking_events (
    shipment_id,
    provider_code,
    status_code,
    status_description,
    location_city,
    location_detail,
    occurred_at,
    raw_status_code,
    raw_data,
    source
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
)
RETURNING id, shipment_id, provider_code, status_code, status_description,
          location_city, location_detail, occurred_at, raw_status_code, raw_data,
          source, created_at;

-- name: CreateTrackingEventsBatch :copyfrom
INSERT INTO tracking_events (
    shipment_id,
    provider_code,
    status_code,
    status_description,
    location_city,
    location_detail,
    occurred_at,
    raw_status_code,
    raw_data,
    source
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10
);

-- name: CountTrackingEventsByShipment :one
SELECT COUNT(*) as total
FROM tracking_events
WHERE shipment_id = $1;

-- name: CheckTrackingEventExists :one
SELECT EXISTS(
    SELECT 1 FROM tracking_events
    WHERE shipment_id = $1 
      AND provider_code = $2 
      AND raw_status_code = $3 
      AND occurred_at = $4
) as exists;

-- name: ListTrackingEventsByStatusCode :many
SELECT id, shipment_id, provider_code, status_code, status_description,
       location_city, location_detail, occurred_at, raw_status_code, raw_data,
       source, created_at
FROM tracking_events
WHERE shipment_id = $1 AND status_code = $2
ORDER BY occurred_at DESC;

-- name: DeleteTrackingEventsByShipment :exec
DELETE FROM tracking_events
WHERE shipment_id = $1;

-- name: GetTrackingEventsByDateRange :many
SELECT id, shipment_id, provider_code, status_code, status_description,
       location_city, location_detail, occurred_at, raw_status_code, raw_data,
       source, created_at
FROM tracking_events
WHERE shipment_id = $1
  AND occurred_at >= $2
  AND occurred_at <= $3
ORDER BY occurred_at DESC;

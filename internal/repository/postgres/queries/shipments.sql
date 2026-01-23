-- Shipments CRUD queries
-- Table: shipments

-- name: GetShipmentByID :one
SELECT id, organization_id, idempotency_key, offer_id, provider_code, service_code,
       provider_account_id, is_marketplace, provider_shipment_id, provider_tracking_number,
       provider_barcode, origin_address, destination_address, parcel, cod_amount_kurus,
       declared_value_kurus, status, label_document_id, label_format, price_charged_kurus,
       cost_kurus, pickup_scheduled_at, picked_up_at, delivered_at, cancelled_at,
       cancellation_reason, return_shipment_id, original_shipment_id, metadata,
       created_at, updated_at
FROM shipments
WHERE id = $1;

-- name: GetShipmentByIdempotencyKey :one
SELECT id, organization_id, idempotency_key, offer_id, provider_code, service_code,
       provider_account_id, is_marketplace, provider_shipment_id, provider_tracking_number,
       provider_barcode, origin_address, destination_address, parcel, cod_amount_kurus,
       declared_value_kurus, status, label_document_id, label_format, price_charged_kurus,
       cost_kurus, pickup_scheduled_at, picked_up_at, delivered_at, cancelled_at,
       cancellation_reason, return_shipment_id, original_shipment_id, metadata,
       created_at, updated_at
FROM shipments
WHERE organization_id = $1 AND idempotency_key = $2;

-- name: GetShipmentByTrackingNumber :one
SELECT id, organization_id, idempotency_key, offer_id, provider_code, service_code,
       provider_account_id, is_marketplace, provider_shipment_id, provider_tracking_number,
       provider_barcode, origin_address, destination_address, parcel, cod_amount_kurus,
       declared_value_kurus, status, label_document_id, label_format, price_charged_kurus,
       cost_kurus, pickup_scheduled_at, picked_up_at, delivered_at, cancelled_at,
       cancellation_reason, return_shipment_id, original_shipment_id, metadata,
       created_at, updated_at
FROM shipments
WHERE provider_code = $1 AND provider_tracking_number = $2;

-- name: GetShipmentForUpdate :one
SELECT id, organization_id, idempotency_key, offer_id, provider_code, service_code,
       provider_account_id, is_marketplace, provider_shipment_id, provider_tracking_number,
       provider_barcode, origin_address, destination_address, parcel, cod_amount_kurus,
       declared_value_kurus, status, label_document_id, label_format, price_charged_kurus,
       cost_kurus, pickup_scheduled_at, picked_up_at, delivered_at, cancelled_at,
       cancellation_reason, return_shipment_id, original_shipment_id, metadata,
       created_at, updated_at
FROM shipments
WHERE id = $1
FOR UPDATE;

-- name: ListShipmentsByOrganization :many
SELECT id, organization_id, idempotency_key, offer_id, provider_code, service_code,
       provider_account_id, is_marketplace, provider_shipment_id, provider_tracking_number,
       provider_barcode, origin_address, destination_address, parcel, cod_amount_kurus,
       declared_value_kurus, status, label_document_id, label_format, price_charged_kurus,
       cost_kurus, pickup_scheduled_at, picked_up_at, delivered_at, cancelled_at,
       cancellation_reason, return_shipment_id, original_shipment_id, metadata,
       created_at, updated_at
FROM shipments
WHERE organization_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status')::varchar)
  AND (sqlc.narg('provider_code')::varchar IS NULL OR provider_code = sqlc.narg('provider_code')::varchar)
  AND (sqlc.narg('created_after')::timestamptz IS NULL OR created_at >= sqlc.narg('created_after')::timestamptz)
  AND (sqlc.narg('created_before')::timestamptz IS NULL OR created_at <= sqlc.narg('created_before')::timestamptz)
ORDER BY created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: CountShipmentsByOrganization :one
SELECT COUNT(*) as total
FROM shipments
WHERE organization_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status')::varchar)
  AND (sqlc.narg('provider_code')::varchar IS NULL OR provider_code = sqlc.narg('provider_code')::varchar)
  AND (sqlc.narg('created_after')::timestamptz IS NULL OR created_at >= sqlc.narg('created_after')::timestamptz)
  AND (sqlc.narg('created_before')::timestamptz IS NULL OR created_at <= sqlc.narg('created_before')::timestamptz);

-- name: CreateShipment :one
INSERT INTO shipments (
    organization_id,
    idempotency_key,
    offer_id,
    provider_code,
    service_code,
    provider_account_id,
    is_marketplace,
    origin_address,
    destination_address,
    parcel,
    cod_amount_kurus,
    declared_value_kurus,
    status,
    metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
RETURNING id, organization_id, idempotency_key, offer_id, provider_code, service_code,
          provider_account_id, is_marketplace, provider_shipment_id, provider_tracking_number,
          provider_barcode, origin_address, destination_address, parcel, cod_amount_kurus,
          declared_value_kurus, status, label_document_id, label_format, price_charged_kurus,
          cost_kurus, pickup_scheduled_at, picked_up_at, delivered_at, cancelled_at,
          cancellation_reason, return_shipment_id, original_shipment_id, metadata,
          created_at, updated_at;

-- name: UpdateShipmentAfterLabelPurchase :one
UPDATE shipments
SET
    provider_shipment_id = $2,
    provider_tracking_number = $3,
    provider_barcode = $4,
    label_document_id = $5,
    label_format = $6,
    price_charged_kurus = $7,
    cost_kurus = $8,
    status = 'label_purchased',
    updated_at = NOW()
WHERE id = $1 AND status = 'draft'
RETURNING id, organization_id, idempotency_key, offer_id, provider_code, service_code,
          provider_account_id, is_marketplace, provider_shipment_id, provider_tracking_number,
          provider_barcode, origin_address, destination_address, parcel, cod_amount_kurus,
          declared_value_kurus, status, label_document_id, label_format, price_charged_kurus,
          cost_kurus, pickup_scheduled_at, picked_up_at, delivered_at, cancelled_at,
          cancellation_reason, return_shipment_id, original_shipment_id, metadata,
          created_at, updated_at;

-- name: UpdateShipmentStatus :one
UPDATE shipments
SET
    status = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING id, organization_id, idempotency_key, offer_id, provider_code, service_code,
          provider_account_id, is_marketplace, provider_shipment_id, provider_tracking_number,
          provider_barcode, origin_address, destination_address, parcel, cod_amount_kurus,
          declared_value_kurus, status, label_document_id, label_format, price_charged_kurus,
          cost_kurus, pickup_scheduled_at, picked_up_at, delivered_at, cancelled_at,
          cancellation_reason, return_shipment_id, original_shipment_id, metadata,
          created_at, updated_at;

-- name: UpdateShipmentPickupScheduled :one
UPDATE shipments
SET
    pickup_scheduled_at = $2,
    status = 'pickup_scheduled',
    updated_at = NOW()
WHERE id = $1 AND status = 'label_purchased'
RETURNING id, organization_id, idempotency_key, offer_id, provider_code, service_code,
          provider_account_id, is_marketplace, provider_shipment_id, provider_tracking_number,
          provider_barcode, origin_address, destination_address, parcel, cod_amount_kurus,
          declared_value_kurus, status, label_document_id, label_format, price_charged_kurus,
          cost_kurus, pickup_scheduled_at, picked_up_at, delivered_at, cancelled_at,
          cancellation_reason, return_shipment_id, original_shipment_id, metadata,
          created_at, updated_at;

-- name: UpdateShipmentPickedUp :one
UPDATE shipments
SET
    picked_up_at = $2,
    status = 'picked_up',
    updated_at = NOW()
WHERE id = $1
RETURNING id, organization_id, idempotency_key, offer_id, provider_code, service_code,
          provider_account_id, is_marketplace, provider_shipment_id, provider_tracking_number,
          provider_barcode, origin_address, destination_address, parcel, cod_amount_kurus,
          declared_value_kurus, status, label_document_id, label_format, price_charged_kurus,
          cost_kurus, pickup_scheduled_at, picked_up_at, delivered_at, cancelled_at,
          cancellation_reason, return_shipment_id, original_shipment_id, metadata,
          created_at, updated_at;

-- name: UpdateShipmentDelivered :one
UPDATE shipments
SET
    delivered_at = $2,
    status = 'delivered',
    updated_at = NOW()
WHERE id = $1
RETURNING id, organization_id, idempotency_key, offer_id, provider_code, service_code,
          provider_account_id, is_marketplace, provider_shipment_id, provider_tracking_number,
          provider_barcode, origin_address, destination_address, parcel, cod_amount_kurus,
          declared_value_kurus, status, label_document_id, label_format, price_charged_kurus,
          cost_kurus, pickup_scheduled_at, picked_up_at, delivered_at, cancelled_at,
          cancellation_reason, return_shipment_id, original_shipment_id, metadata,
          created_at, updated_at;

-- name: UpdateShipmentCancelled :one
UPDATE shipments
SET
    cancelled_at = NOW(),
    cancellation_reason = $2,
    status = 'cancelled',
    updated_at = NOW()
WHERE id = $1 AND status IN ('draft', 'label_purchased', 'pickup_scheduled')
RETURNING id, organization_id, idempotency_key, offer_id, provider_code, service_code,
          provider_account_id, is_marketplace, provider_shipment_id, provider_tracking_number,
          provider_barcode, origin_address, destination_address, parcel, cod_amount_kurus,
          declared_value_kurus, status, label_document_id, label_format, price_charged_kurus,
          cost_kurus, pickup_scheduled_at, picked_up_at, delivered_at, cancelled_at,
          cancellation_reason, return_shipment_id, original_shipment_id, metadata,
          created_at, updated_at;

-- name: UpdateShipmentReturnLink :one
UPDATE shipments
SET
    return_shipment_id = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING id, organization_id, idempotency_key, offer_id, provider_code, service_code,
          provider_account_id, is_marketplace, provider_shipment_id, provider_tracking_number,
          provider_barcode, origin_address, destination_address, parcel, cod_amount_kurus,
          declared_value_kurus, status, label_document_id, label_format, price_charged_kurus,
          cost_kurus, pickup_scheduled_at, picked_up_at, delivered_at, cancelled_at,
          cancellation_reason, return_shipment_id, original_shipment_id, metadata,
          created_at, updated_at;

-- name: UpdateShipmentMetadata :one
UPDATE shipments
SET
    metadata = $2,
    updated_at = NOW()
WHERE id = $1
RETURNING id, organization_id, idempotency_key, offer_id, provider_code, service_code,
          provider_account_id, is_marketplace, provider_shipment_id, provider_tracking_number,
          provider_barcode, origin_address, destination_address, parcel, cod_amount_kurus,
          declared_value_kurus, status, label_document_id, label_format, price_charged_kurus,
          cost_kurus, pickup_scheduled_at, picked_up_at, delivered_at, cancelled_at,
          cancellation_reason, return_shipment_id, original_shipment_id, metadata,
          created_at, updated_at;

-- name: ListShipmentsForTrackingPoll :many
SELECT id, organization_id, provider_code, provider_tracking_number, status
FROM shipments
WHERE status IN ('label_purchased', 'pickup_scheduled', 'picked_up', 'in_transit', 'at_hub', 'out_for_delivery')
  AND provider_tracking_number IS NOT NULL
  AND (sqlc.narg('provider_code')::varchar IS NULL OR provider_code = sqlc.narg('provider_code')::varchar)
ORDER BY updated_at ASC
LIMIT sqlc.arg('limit_count')::int;

-- name: ListCODShipmentsPendingSettlement :many
SELECT s.id, s.organization_id, s.provider_code, s.provider_tracking_number, 
       s.cod_amount_kurus, s.status, s.delivered_at
FROM shipments s
LEFT JOIN cod_settlements cs ON s.id = cs.shipment_id
WHERE s.cod_amount_kurus IS NOT NULL 
  AND s.cod_amount_kurus > 0
  AND s.status = 'delivered'
  AND cs.id IS NULL;

-- name: CreateReturnShipment :one
INSERT INTO shipments (
    organization_id,
    idempotency_key,
    provider_code,
    service_code,
    provider_account_id,
    is_marketplace,
    origin_address,
    destination_address,
    parcel,
    status,
    original_shipment_id,
    metadata
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
RETURNING id, organization_id, idempotency_key, offer_id, provider_code, service_code,
          provider_account_id, is_marketplace, provider_shipment_id, provider_tracking_number,
          provider_barcode, origin_address, destination_address, parcel, cod_amount_kurus,
          declared_value_kurus, status, label_document_id, label_format, price_charged_kurus,
          cost_kurus, pickup_scheduled_at, picked_up_at, delivered_at, cancelled_at,
          cancellation_reason, return_shipment_id, original_shipment_id, metadata,
          created_at, updated_at;

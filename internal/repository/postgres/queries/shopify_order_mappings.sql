-- Shopify Order Mappings CRUD queries
-- Table: shopify_order_mappings

-- name: GetShopifyOrderMappingByID :one
SELECT id, organization_id, shopify_store_id, shopify_order_id, shopify_order_number,
       shipment_id, status, sync_error, shopify_customer_name,
       shopify_order_total_kurus, shopify_line_item_count,
       shopify_fulfillment_id, fulfillment_synced_at, raw_order,
       created_at, updated_at
FROM shopify_order_mappings
WHERE id = $1;

-- name: GetShopifyOrderMappingByOrderID :one
SELECT id, organization_id, shopify_store_id, shopify_order_id, shopify_order_number,
       shipment_id, status, sync_error, shopify_customer_name,
       shopify_order_total_kurus, shopify_line_item_count,
       shopify_fulfillment_id, fulfillment_synced_at, raw_order,
       created_at, updated_at
FROM shopify_order_mappings
WHERE shopify_store_id = $1 AND shopify_order_id = $2;

-- name: GetShopifyOrderMappingByShipmentID :one
SELECT id, organization_id, shopify_store_id, shopify_order_id, shopify_order_number,
       shipment_id, status, sync_error, shopify_customer_name,
       shopify_order_total_kurus, shopify_line_item_count,
       shopify_fulfillment_id, fulfillment_synced_at, raw_order,
       created_at, updated_at
FROM shopify_order_mappings
WHERE shipment_id = $1;

-- name: ListShopifyOrderMappingsByStore :many
SELECT id, organization_id, shopify_store_id, shopify_order_id, shopify_order_number,
       shipment_id, status, sync_error, shopify_customer_name,
       shopify_order_total_kurus, shopify_line_item_count,
       shopify_fulfillment_id, fulfillment_synced_at,
       created_at, updated_at
FROM shopify_order_mappings
WHERE shopify_store_id = $1
  AND (sqlc.narg('status_filter')::text IS NULL OR status = sqlc.narg('status_filter')::text)
ORDER BY created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: CountShopifyOrderMappingsByStore :one
SELECT COUNT(*) as total
FROM shopify_order_mappings
WHERE shopify_store_id = $1
  AND (sqlc.narg('status_filter')::text IS NULL OR status = sqlc.narg('status_filter')::text);

-- name: CreateShopifyOrderMapping :one
INSERT INTO shopify_order_mappings (
    organization_id, shopify_store_id, shopify_order_id, shopify_order_number,
    status, shopify_customer_name, shopify_order_total_kurus,
    shopify_line_item_count, raw_order
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9
)
ON CONFLICT (shopify_store_id, shopify_order_id) DO NOTHING
RETURNING id, organization_id, shopify_store_id, shopify_order_id, shopify_order_number,
          shipment_id, status, sync_error, shopify_customer_name,
          shopify_order_total_kurus, shopify_line_item_count,
          shopify_fulfillment_id, fulfillment_synced_at, raw_order,
          created_at, updated_at;

-- name: UpdateShopifyOrderMappingStatus :exec
UPDATE shopify_order_mappings
SET
    status = $2,
    sync_error = $3,
    updated_at = NOW()
WHERE id = $1;

-- name: LinkShopifyOrderToShipment :exec
UPDATE shopify_order_mappings
SET
    shipment_id = $2,
    status = $3,
    updated_at = NOW()
WHERE id = $1;

-- name: UpdateShopifyOrderFulfillment :exec
UPDATE shopify_order_mappings
SET
    shopify_fulfillment_id = $2,
    status = 'fulfillment_synced',
    fulfillment_synced_at = NOW(),
    updated_at = NOW()
WHERE id = $1;

-- name: ListPendingFulfillmentSyncs :many
SELECT m.id, m.organization_id, m.shopify_store_id, m.shopify_order_id,
       m.shopify_order_number, m.shipment_id, m.status,
       m.created_at, m.updated_at
FROM shopify_order_mappings m
WHERE m.status = 'label_purchased'
  AND m.shipment_id IS NOT NULL
  AND m.shopify_fulfillment_id IS NULL
ORDER BY m.created_at ASC
LIMIT sqlc.arg('limit_count')::int;

-- name: CountShopifyOrdersByStatus :many
SELECT status, COUNT(*) as count
FROM shopify_order_mappings
WHERE shopify_store_id = $1
GROUP BY status;

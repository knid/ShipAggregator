-- Shopify Shipping Rules CRUD queries
-- Table: shopify_shipping_rules

-- name: GetShopifyShippingRuleByID :one
SELECT id, organization_id, shopify_store_id, name, priority, is_active,
       condition_destination_city_codes, condition_min_weight_grams, condition_max_weight_grams,
       condition_min_order_total_kurus, condition_max_order_total_kurus,
       condition_has_cod, condition_tags,
       action_provider_code, action_service_code, action_provider_account_id,
       created_at, updated_at
FROM shopify_shipping_rules
WHERE id = $1;

-- name: ListActiveShopifyShippingRules :many
SELECT id, organization_id, shopify_store_id, name, priority, is_active,
       condition_destination_city_codes, condition_min_weight_grams, condition_max_weight_grams,
       condition_min_order_total_kurus, condition_max_order_total_kurus,
       condition_has_cod, condition_tags,
       action_provider_code, action_service_code, action_provider_account_id,
       created_at, updated_at
FROM shopify_shipping_rules
WHERE organization_id = $1
  AND is_active = TRUE
  AND (shopify_store_id IS NULL OR shopify_store_id = sqlc.narg('store_id')::uuid)
ORDER BY priority ASC;

-- name: ListShopifyShippingRulesByOrganization :many
SELECT id, organization_id, shopify_store_id, name, priority, is_active,
       condition_destination_city_codes, condition_min_weight_grams, condition_max_weight_grams,
       condition_min_order_total_kurus, condition_max_order_total_kurus,
       condition_has_cod, condition_tags,
       action_provider_code, action_service_code, action_provider_account_id,
       created_at, updated_at
FROM shopify_shipping_rules
WHERE organization_id = $1
ORDER BY priority ASC;

-- name: CreateShopifyShippingRule :one
INSERT INTO shopify_shipping_rules (
    organization_id, shopify_store_id, name, priority, is_active,
    condition_destination_city_codes, condition_min_weight_grams, condition_max_weight_grams,
    condition_min_order_total_kurus, condition_max_order_total_kurus,
    condition_has_cod, condition_tags,
    action_provider_code, action_service_code, action_provider_account_id
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
)
RETURNING id, organization_id, shopify_store_id, name, priority, is_active,
          condition_destination_city_codes, condition_min_weight_grams, condition_max_weight_grams,
          condition_min_order_total_kurus, condition_max_order_total_kurus,
          condition_has_cod, condition_tags,
          action_provider_code, action_service_code, action_provider_account_id,
          created_at, updated_at;

-- name: UpdateShopifyShippingRule :one
UPDATE shopify_shipping_rules
SET
    name = COALESCE(sqlc.narg('name'), name),
    priority = COALESCE(sqlc.narg('priority'), priority),
    is_active = COALESCE(sqlc.narg('is_active'), is_active),
    condition_destination_city_codes = COALESCE(sqlc.narg('condition_destination_city_codes'), condition_destination_city_codes),
    condition_min_weight_grams = COALESCE(sqlc.narg('condition_min_weight_grams'), condition_min_weight_grams),
    condition_max_weight_grams = COALESCE(sqlc.narg('condition_max_weight_grams'), condition_max_weight_grams),
    condition_min_order_total_kurus = COALESCE(sqlc.narg('condition_min_order_total_kurus'), condition_min_order_total_kurus),
    condition_max_order_total_kurus = COALESCE(sqlc.narg('condition_max_order_total_kurus'), condition_max_order_total_kurus),
    condition_has_cod = COALESCE(sqlc.narg('condition_has_cod'), condition_has_cod),
    condition_tags = COALESCE(sqlc.narg('condition_tags'), condition_tags),
    action_provider_code = COALESCE(sqlc.narg('action_provider_code'), action_provider_code),
    action_service_code = COALESCE(sqlc.narg('action_service_code'), action_service_code),
    action_provider_account_id = COALESCE(sqlc.narg('action_provider_account_id'), action_provider_account_id),
    updated_at = NOW()
WHERE id = $1 AND organization_id = $2
RETURNING id, organization_id, shopify_store_id, name, priority, is_active,
          condition_destination_city_codes, condition_min_weight_grams, condition_max_weight_grams,
          condition_min_order_total_kurus, condition_max_order_total_kurus,
          condition_has_cod, condition_tags,
          action_provider_code, action_service_code, action_provider_account_id,
          created_at, updated_at;

-- name: DeleteShopifyShippingRule :exec
DELETE FROM shopify_shipping_rules
WHERE id = $1 AND organization_id = $2;

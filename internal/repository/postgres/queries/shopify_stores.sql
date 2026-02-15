-- Shopify Stores CRUD queries
-- Table: shopify_stores

-- name: GetShopifyStoreByID :one
SELECT id, organization_id, shop_domain, access_token_encrypted, access_token_iv,
       scopes, shopify_store_id, store_name, store_email, currency,
       status, sync_mode, installed_at, uninstalled_at, last_sync_at,
       created_at, updated_at
FROM shopify_stores
WHERE id = $1;

-- name: GetShopifyStoreByShopDomain :one
SELECT id, organization_id, shop_domain, access_token_encrypted, access_token_iv,
       scopes, shopify_store_id, store_name, store_email, currency,
       status, sync_mode, installed_at, uninstalled_at, last_sync_at,
       created_at, updated_at
FROM shopify_stores
WHERE shop_domain = $1 AND status != 'uninstalled'
LIMIT 1;

-- name: ListShopifyStoresByOrganization :many
SELECT id, organization_id, shop_domain, scopes, shopify_store_id,
       store_name, store_email, currency, status, sync_mode,
       installed_at, uninstalled_at, last_sync_at, created_at, updated_at
FROM shopify_stores
WHERE organization_id = $1 AND status != 'uninstalled'
ORDER BY created_at DESC;

-- name: CreateShopifyStore :one
INSERT INTO shopify_stores (
    organization_id, shop_domain, access_token_encrypted, access_token_iv,
    scopes, shopify_store_id, store_name, store_email, currency,
    status, sync_mode
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING id, organization_id, shop_domain, access_token_encrypted, access_token_iv,
          scopes, shopify_store_id, store_name, store_email, currency,
          status, sync_mode, installed_at, uninstalled_at, last_sync_at,
          created_at, updated_at;

-- name: UpdateShopifyStore :one
UPDATE shopify_stores
SET
    sync_mode = COALESCE(sqlc.narg('sync_mode'), sync_mode),
    status = COALESCE(sqlc.narg('status'), status),
    store_name = COALESCE(sqlc.narg('store_name'), store_name),
    store_email = COALESCE(sqlc.narg('store_email'), store_email),
    last_sync_at = COALESCE(sqlc.narg('last_sync_at'), last_sync_at),
    updated_at = NOW()
WHERE id = $1
RETURNING id, organization_id, shop_domain, access_token_encrypted, access_token_iv,
          scopes, shopify_store_id, store_name, store_email, currency,
          status, sync_mode, installed_at, uninstalled_at, last_sync_at,
          created_at, updated_at;

-- name: MarkShopifyStoreUninstalled :exec
UPDATE shopify_stores
SET
    status = 'uninstalled',
    uninstalled_at = NOW(),
    access_token_encrypted = NULL,
    access_token_iv = NULL,
    updated_at = NOW()
WHERE shop_domain = $1 AND status != 'uninstalled';

-- name: UpdateShopifyStoreLastSync :exec
UPDATE shopify_stores
SET last_sync_at = NOW(), updated_at = NOW()
WHERE id = $1;

-- name: DeleteShopifyStore :exec
UPDATE shopify_stores
SET
    status = 'uninstalled',
    uninstalled_at = NOW(),
    access_token_encrypted = NULL,
    access_token_iv = NULL,
    updated_at = NOW()
WHERE id = $1 AND organization_id = $2;

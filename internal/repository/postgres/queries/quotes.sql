-- Quotes CRUD queries
-- Table: quotes

-- name: GetQuoteByID :one
SELECT id, organization_id, idempotency_key, origin_address, destination_address,
       parcel, cod_amount_kurus, declared_value_kurus, use_byoa, provider_account_ids,
       status, expires_at, created_at, cod_collection_type, cod_installments
FROM quotes
WHERE id = $1;

-- name: GetQuoteByIdempotencyKey :one
SELECT id, organization_id, idempotency_key, origin_address, destination_address,
       parcel, cod_amount_kurus, declared_value_kurus, use_byoa, provider_account_ids,
       status, expires_at, created_at, cod_collection_type, cod_installments
FROM quotes
WHERE organization_id = $1 AND idempotency_key = $2;

-- name: GetActiveQuoteByID :one
SELECT id, organization_id, idempotency_key, origin_address, destination_address,
       parcel, cod_amount_kurus, declared_value_kurus, use_byoa, provider_account_ids,
       status, expires_at, created_at, cod_collection_type, cod_installments
FROM quotes
WHERE id = $1 AND status = 'completed' AND expires_at > NOW();

-- name: ListQuotesByOrganization :many
SELECT id, organization_id, idempotency_key, origin_address, destination_address,
       parcel, cod_amount_kurus, declared_value_kurus, use_byoa, provider_account_ids,
       status, expires_at, created_at, cod_collection_type, cod_installments
FROM quotes
WHERE organization_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status')::varchar)
ORDER BY created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: CountQuotesByOrganization :one
SELECT COUNT(*) as total
FROM quotes
WHERE organization_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status')::varchar);

-- name: CreateQuote :one
INSERT INTO quotes (
    organization_id,
    idempotency_key,
    origin_address,
    destination_address,
    parcel,
    cod_amount_kurus,
    declared_value_kurus,
    use_byoa,
    provider_account_ids,
    status,
    expires_at,
    cod_collection_type,
    cod_installments
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
)
RETURNING id, organization_id, idempotency_key, origin_address, destination_address,
          parcel, cod_amount_kurus, declared_value_kurus, use_byoa, provider_account_ids,
          status, expires_at, created_at, cod_collection_type, cod_installments;

-- name: UpdateQuoteStatus :one
UPDATE quotes
SET status = $2
WHERE id = $1
RETURNING id, organization_id, idempotency_key, origin_address, destination_address,
          parcel, cod_amount_kurus, declared_value_kurus, use_byoa, provider_account_ids,
          status, expires_at, created_at, cod_collection_type, cod_installments;

-- name: ExpireQuotes :exec
UPDATE quotes
SET status = 'expired'
WHERE status IN ('pending', 'completed') AND expires_at <= NOW();

-- name: DeleteExpiredQuotes :exec
DELETE FROM quotes
WHERE expires_at < NOW() - INTERVAL '7 days';

-- name: GetQuoteWithOffers :many
SELECT q.id, q.organization_id, q.idempotency_key, q.origin_address, q.destination_address,
       q.parcel, q.cod_amount_kurus, q.declared_value_kurus, q.use_byoa, q.provider_account_ids,
       q.status, q.expires_at, q.created_at, q.cod_collection_type, q.cod_installments,
       o.id as offer_id, o.provider_code, o.service_code, o.provider_account_id, o.is_marketplace,
       o.base_price_kurus, o.fuel_surcharge_kurus, o.cod_fee_kurus, o.insurance_fee_kurus,
       o.other_fees_kurus, o.subtotal_kurus, o.vat_kurus, o.total_price_kurus,
       o.cost_price_kurus, o.margin_kurus, o.estimated_days_min, o.estimated_days_max,
       o.provider_quote_id, o.expires_at as offer_expires_at
FROM quotes q
LEFT JOIN offers o ON q.id = o.quote_id
WHERE q.id = $1
ORDER BY o.total_price_kurus ASC;

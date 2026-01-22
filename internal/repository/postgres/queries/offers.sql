-- Offers CRUD queries
-- Table: offers

-- name: GetOfferByID :one
SELECT id, quote_id, provider_code, service_code, provider_account_id, is_marketplace,
       base_price_kurus, fuel_surcharge_kurus, cod_fee_kurus, insurance_fee_kurus,
       other_fees_kurus, subtotal_kurus, vat_kurus, total_price_kurus,
       cost_price_kurus, margin_kurus, estimated_days_min, estimated_days_max,
       provider_quote_id, expires_at, created_at
FROM offers
WHERE id = $1;

-- name: GetActiveOfferByID :one
SELECT id, quote_id, provider_code, service_code, provider_account_id, is_marketplace,
       base_price_kurus, fuel_surcharge_kurus, cod_fee_kurus, insurance_fee_kurus,
       other_fees_kurus, subtotal_kurus, vat_kurus, total_price_kurus,
       cost_price_kurus, margin_kurus, estimated_days_min, estimated_days_max,
       provider_quote_id, expires_at, created_at
FROM offers
WHERE id = $1 AND expires_at > NOW();

-- name: ListOffersByQuote :many
SELECT id, quote_id, provider_code, service_code, provider_account_id, is_marketplace,
       base_price_kurus, fuel_surcharge_kurus, cod_fee_kurus, insurance_fee_kurus,
       other_fees_kurus, subtotal_kurus, vat_kurus, total_price_kurus,
       cost_price_kurus, margin_kurus, estimated_days_min, estimated_days_max,
       provider_quote_id, expires_at, created_at
FROM offers
WHERE quote_id = $1
ORDER BY total_price_kurus ASC;

-- name: ListActiveOffersByQuote :many
SELECT id, quote_id, provider_code, service_code, provider_account_id, is_marketplace,
       base_price_kurus, fuel_surcharge_kurus, cod_fee_kurus, insurance_fee_kurus,
       other_fees_kurus, subtotal_kurus, vat_kurus, total_price_kurus,
       cost_price_kurus, margin_kurus, estimated_days_min, estimated_days_max,
       provider_quote_id, expires_at, created_at
FROM offers
WHERE quote_id = $1 AND expires_at > NOW()
ORDER BY total_price_kurus ASC;

-- name: ListOffersByQuoteAndProvider :many
SELECT id, quote_id, provider_code, service_code, provider_account_id, is_marketplace,
       base_price_kurus, fuel_surcharge_kurus, cod_fee_kurus, insurance_fee_kurus,
       other_fees_kurus, subtotal_kurus, vat_kurus, total_price_kurus,
       cost_price_kurus, margin_kurus, estimated_days_min, estimated_days_max,
       provider_quote_id, expires_at, created_at
FROM offers
WHERE quote_id = $1 AND provider_code = $2
ORDER BY total_price_kurus ASC;

-- name: GetCheapestOfferByQuote :one
SELECT id, quote_id, provider_code, service_code, provider_account_id, is_marketplace,
       base_price_kurus, fuel_surcharge_kurus, cod_fee_kurus, insurance_fee_kurus,
       other_fees_kurus, subtotal_kurus, vat_kurus, total_price_kurus,
       cost_price_kurus, margin_kurus, estimated_days_min, estimated_days_max,
       provider_quote_id, expires_at, created_at
FROM offers
WHERE quote_id = $1 AND expires_at > NOW()
ORDER BY total_price_kurus ASC
LIMIT 1;

-- name: GetFastestOfferByQuote :one
SELECT id, quote_id, provider_code, service_code, provider_account_id, is_marketplace,
       base_price_kurus, fuel_surcharge_kurus, cod_fee_kurus, insurance_fee_kurus,
       other_fees_kurus, subtotal_kurus, vat_kurus, total_price_kurus,
       cost_price_kurus, margin_kurus, estimated_days_min, estimated_days_max,
       provider_quote_id, expires_at, created_at
FROM offers
WHERE quote_id = $1 AND expires_at > NOW()
ORDER BY estimated_days_max ASC, total_price_kurus ASC
LIMIT 1;

-- name: CreateOffer :one
INSERT INTO offers (
    quote_id,
    provider_code,
    service_code,
    provider_account_id,
    is_marketplace,
    base_price_kurus,
    fuel_surcharge_kurus,
    cod_fee_kurus,
    insurance_fee_kurus,
    other_fees_kurus,
    subtotal_kurus,
    vat_kurus,
    total_price_kurus,
    cost_price_kurus,
    margin_kurus,
    estimated_days_min,
    estimated_days_max,
    provider_quote_id,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19
)
RETURNING id, quote_id, provider_code, service_code, provider_account_id, is_marketplace,
          base_price_kurus, fuel_surcharge_kurus, cod_fee_kurus, insurance_fee_kurus,
          other_fees_kurus, subtotal_kurus, vat_kurus, total_price_kurus,
          cost_price_kurus, margin_kurus, estimated_days_min, estimated_days_max,
          provider_quote_id, expires_at, created_at;

-- name: CreateOffersBatch :copyfrom
INSERT INTO offers (
    quote_id,
    provider_code,
    service_code,
    provider_account_id,
    is_marketplace,
    base_price_kurus,
    fuel_surcharge_kurus,
    cod_fee_kurus,
    insurance_fee_kurus,
    other_fees_kurus,
    subtotal_kurus,
    vat_kurus,
    total_price_kurus,
    cost_price_kurus,
    margin_kurus,
    estimated_days_min,
    estimated_days_max,
    provider_quote_id,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19
);

-- name: CountOffersByQuote :one
SELECT COUNT(*) as total
FROM offers
WHERE quote_id = $1;

-- name: CountActiveOffersByQuote :one
SELECT COUNT(*) as total
FROM offers
WHERE quote_id = $1 AND expires_at > NOW();

-- name: DeleteOffersByQuote :exec
DELETE FROM offers
WHERE quote_id = $1;

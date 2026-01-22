-- Providers read queries
-- Tables: providers, provider_services, marketplace_rate_plans

-- name: GetProviderByCode :one
SELECT code, name, logo_url, website, support_email, support_phone,
       api_type, api_base_url, api_sandbox_url, is_active, capabilities,
       created_at, updated_at
FROM providers
WHERE code = $1;

-- name: ListProviders :many
SELECT code, name, logo_url, website, support_email, support_phone,
       api_type, api_base_url, api_sandbox_url, is_active, capabilities,
       created_at, updated_at
FROM providers
WHERE (sqlc.narg('is_active')::boolean IS NULL OR is_active = sqlc.narg('is_active')::boolean)
ORDER BY name ASC;

-- name: ListActiveProviders :many
SELECT code, name, logo_url, website, support_email, support_phone,
       api_type, api_base_url, api_sandbox_url, is_active, capabilities,
       created_at, updated_at
FROM providers
WHERE is_active = TRUE
ORDER BY name ASC;

-- name: GetProviderService :one
SELECT id, provider_code, service_code, name, description,
       estimated_days_min, estimated_days_max, supports_cod, supports_insurance,
       supports_return, max_weight_grams, max_length_cm, max_width_cm, max_height_cm,
       label_formats, is_active, created_at, updated_at
FROM provider_services
WHERE provider_code = $1 AND service_code = $2;

-- name: GetProviderServiceByID :one
SELECT id, provider_code, service_code, name, description,
       estimated_days_min, estimated_days_max, supports_cod, supports_insurance,
       supports_return, max_weight_grams, max_length_cm, max_width_cm, max_height_cm,
       label_formats, is_active, created_at, updated_at
FROM provider_services
WHERE id = $1;

-- name: ListProviderServices :many
SELECT id, provider_code, service_code, name, description,
       estimated_days_min, estimated_days_max, supports_cod, supports_insurance,
       supports_return, max_weight_grams, max_length_cm, max_width_cm, max_height_cm,
       label_formats, is_active, created_at, updated_at
FROM provider_services
WHERE provider_code = $1
  AND (sqlc.narg('is_active')::boolean IS NULL OR is_active = sqlc.narg('is_active')::boolean)
ORDER BY name ASC;

-- name: ListActiveProviderServices :many
SELECT id, provider_code, service_code, name, description,
       estimated_days_min, estimated_days_max, supports_cod, supports_insurance,
       supports_return, max_weight_grams, max_length_cm, max_width_cm, max_height_cm,
       label_formats, is_active, created_at, updated_at
FROM provider_services
WHERE provider_code = $1 AND is_active = TRUE
ORDER BY name ASC;

-- name: ListProviderServicesWithCOD :many
SELECT id, provider_code, service_code, name, description,
       estimated_days_min, estimated_days_max, supports_cod, supports_insurance,
       supports_return, max_weight_grams, max_length_cm, max_width_cm, max_height_cm,
       label_formats, is_active, created_at, updated_at
FROM provider_services
WHERE provider_code = $1 AND is_active = TRUE AND supports_cod = TRUE
ORDER BY name ASC;

-- name: ListProviderServicesWithReturn :many
SELECT id, provider_code, service_code, name, description,
       estimated_days_min, estimated_days_max, supports_cod, supports_insurance,
       supports_return, max_weight_grams, max_length_cm, max_width_cm, max_height_cm,
       label_formats, is_active, created_at, updated_at
FROM provider_services
WHERE provider_code = $1 AND is_active = TRUE AND supports_return = TRUE
ORDER BY name ASC;

-- name: GetMarketplaceRatePlan :one
SELECT id, provider_code, service_code, name, base_price_kurus, price_per_kg_kurus,
       price_per_desi_kurus, fuel_surcharge_percent, cod_fee_percent, cod_fee_min_kurus,
       vat_percent, effective_from, effective_to, is_active, created_at, updated_at
FROM marketplace_rate_plans
WHERE provider_code = $1 AND service_code = $2 AND is_active = TRUE
  AND effective_from <= CURRENT_DATE
  AND (effective_to IS NULL OR effective_to >= CURRENT_DATE)
ORDER BY effective_from DESC
LIMIT 1;

-- name: ListMarketplaceRatePlans :many
SELECT id, provider_code, service_code, name, base_price_kurus, price_per_kg_kurus,
       price_per_desi_kurus, fuel_surcharge_percent, cod_fee_percent, cod_fee_min_kurus,
       vat_percent, effective_from, effective_to, is_active, created_at, updated_at
FROM marketplace_rate_plans
WHERE provider_code = $1
ORDER BY service_code, effective_from DESC;

-- name: ListCurrentMarketplaceRatePlans :many
SELECT id, provider_code, service_code, name, base_price_kurus, price_per_kg_kurus,
       price_per_desi_kurus, fuel_surcharge_percent, cod_fee_percent, cod_fee_min_kurus,
       vat_percent, effective_from, effective_to, is_active, created_at, updated_at
FROM marketplace_rate_plans
WHERE is_active = TRUE
  AND effective_from <= CURRENT_DATE
  AND (effective_to IS NULL OR effective_to >= CURRENT_DATE)
ORDER BY provider_code, service_code;

-- name: GetProviderWithServices :many
SELECT p.code, p.name, p.logo_url, p.website, p.support_email, p.support_phone,
       p.api_type, p.api_base_url, p.api_sandbox_url, p.is_active as provider_is_active, p.capabilities,
       ps.id as service_id, ps.service_code, ps.name as service_name, ps.description as service_description,
       ps.estimated_days_min, ps.estimated_days_max, ps.supports_cod, ps.supports_insurance,
       ps.supports_return, ps.max_weight_grams, ps.max_length_cm, ps.max_width_cm, ps.max_height_cm,
       ps.label_formats, ps.is_active as service_is_active
FROM providers p
LEFT JOIN provider_services ps ON p.code = ps.provider_code AND ps.is_active = TRUE
WHERE p.code = $1;

-- name: CheckProviderServiceExists :one
SELECT EXISTS(
    SELECT 1 FROM provider_services
    WHERE provider_code = $1 AND service_code = $2 AND is_active = TRUE
) as exists;

-- name: GetProviderCapabilities :one
SELECT capabilities
FROM providers
WHERE code = $1;

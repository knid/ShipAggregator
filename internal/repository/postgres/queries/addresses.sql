-- Addresses CRUD queries
-- Table: addresses

-- name: GetAddressByID :one
SELECT id, organization_id, label, contact_name, company_name, phone, email,
       street_line_1, street_line_2, neighborhood_code, district_code, city_code,
       postal_code, latitude, longitude, is_default_sender, is_default_recipient,
       created_at, updated_at, deleted_at
FROM addresses
WHERE id = $1 AND deleted_at IS NULL;

-- name: ListAddressesByOrganization :many
SELECT id, organization_id, label, contact_name, company_name, phone, email,
       street_line_1, street_line_2, neighborhood_code, district_code, city_code,
       postal_code, latitude, longitude, is_default_sender, is_default_recipient,
       created_at, updated_at, deleted_at
FROM addresses
WHERE organization_id = $1 AND deleted_at IS NULL
ORDER BY 
    CASE WHEN is_default_sender THEN 0 ELSE 1 END,
    CASE WHEN is_default_recipient THEN 0 ELSE 1 END,
    created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: CountAddressesByOrganization :one
SELECT COUNT(*) as total
FROM addresses
WHERE organization_id = $1 AND deleted_at IS NULL;

-- name: GetDefaultSenderAddress :one
SELECT id, organization_id, label, contact_name, company_name, phone, email,
       street_line_1, street_line_2, neighborhood_code, district_code, city_code,
       postal_code, latitude, longitude, is_default_sender, is_default_recipient,
       created_at, updated_at, deleted_at
FROM addresses
WHERE organization_id = $1 AND is_default_sender = TRUE AND deleted_at IS NULL
LIMIT 1;

-- name: GetDefaultRecipientAddress :one
SELECT id, organization_id, label, contact_name, company_name, phone, email,
       street_line_1, street_line_2, neighborhood_code, district_code, city_code,
       postal_code, latitude, longitude, is_default_sender, is_default_recipient,
       created_at, updated_at, deleted_at
FROM addresses
WHERE organization_id = $1 AND is_default_recipient = TRUE AND deleted_at IS NULL
LIMIT 1;

-- name: CreateAddress :one
INSERT INTO addresses (
    organization_id,
    label,
    contact_name,
    company_name,
    phone,
    email,
    street_line_1,
    street_line_2,
    neighborhood_code,
    district_code,
    city_code,
    postal_code,
    latitude,
    longitude,
    is_default_sender,
    is_default_recipient
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
)
RETURNING id, organization_id, label, contact_name, company_name, phone, email,
          street_line_1, street_line_2, neighborhood_code, district_code, city_code,
          postal_code, latitude, longitude, is_default_sender, is_default_recipient,
          created_at, updated_at, deleted_at;

-- name: UpdateAddress :one
UPDATE addresses
SET
    label = COALESCE(sqlc.narg('label'), label),
    contact_name = COALESCE(sqlc.narg('contact_name'), contact_name),
    company_name = COALESCE(sqlc.narg('company_name'), company_name),
    phone = COALESCE(sqlc.narg('phone'), phone),
    email = COALESCE(sqlc.narg('email'), email),
    street_line_1 = COALESCE(sqlc.narg('street_line_1'), street_line_1),
    street_line_2 = COALESCE(sqlc.narg('street_line_2'), street_line_2),
    neighborhood_code = COALESCE(sqlc.narg('neighborhood_code'), neighborhood_code),
    district_code = COALESCE(sqlc.narg('district_code'), district_code),
    city_code = COALESCE(sqlc.narg('city_code'), city_code),
    postal_code = COALESCE(sqlc.narg('postal_code'), postal_code),
    latitude = COALESCE(sqlc.narg('latitude'), latitude),
    longitude = COALESCE(sqlc.narg('longitude'), longitude),
    is_default_sender = COALESCE(sqlc.narg('is_default_sender'), is_default_sender),
    is_default_recipient = COALESCE(sqlc.narg('is_default_recipient'), is_default_recipient),
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, organization_id, label, contact_name, company_name, phone, email,
          street_line_1, street_line_2, neighborhood_code, district_code, city_code,
          postal_code, latitude, longitude, is_default_sender, is_default_recipient,
          created_at, updated_at, deleted_at;

-- name: SoftDeleteAddress :one
UPDATE addresses
SET
    deleted_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, organization_id, label, contact_name, company_name, phone, email,
          street_line_1, street_line_2, neighborhood_code, district_code, city_code,
          postal_code, latitude, longitude, is_default_sender, is_default_recipient,
          created_at, updated_at, deleted_at;

-- name: ClearDefaultSenderByOrganization :exec
UPDATE addresses
SET
    is_default_sender = FALSE,
    updated_at = NOW()
WHERE organization_id = $1 AND is_default_sender = TRUE AND deleted_at IS NULL;

-- name: ClearDefaultRecipientByOrganization :exec
UPDATE addresses
SET
    is_default_recipient = FALSE,
    updated_at = NOW()
WHERE organization_id = $1 AND is_default_recipient = TRUE AND deleted_at IS NULL;

-- name: SetDefaultSender :one
UPDATE addresses
SET
    is_default_sender = TRUE,
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, organization_id, label, contact_name, company_name, phone, email,
          street_line_1, street_line_2, neighborhood_code, district_code, city_code,
          postal_code, latitude, longitude, is_default_sender, is_default_recipient,
          created_at, updated_at, deleted_at;

-- name: SetDefaultRecipient :one
UPDATE addresses
SET
    is_default_recipient = TRUE,
    updated_at = NOW()
WHERE id = $1 AND deleted_at IS NULL
RETURNING id, organization_id, label, contact_name, company_name, phone, email,
          street_line_1, street_line_2, neighborhood_code, district_code, city_code,
          postal_code, latitude, longitude, is_default_sender, is_default_recipient,
          created_at, updated_at, deleted_at;

-- name: SearchAddressesByOrganization :many
SELECT id, organization_id, label, contact_name, company_name, phone, email,
       street_line_1, street_line_2, neighborhood_code, district_code, city_code,
       postal_code, latitude, longitude, is_default_sender, is_default_recipient,
       created_at, updated_at, deleted_at
FROM addresses
WHERE organization_id = $1 AND deleted_at IS NULL
  AND (
    label ILIKE '%' || $2 || '%'
    OR contact_name ILIKE '%' || $2 || '%'
    OR company_name ILIKE '%' || $2 || '%'
    OR street_line_1 ILIKE '%' || $2 || '%'
  )
ORDER BY created_at DESC
LIMIT sqlc.arg('limit_count')::int;

-- name: HardDeleteExpiredAddresses :exec
DELETE FROM addresses
WHERE deleted_at IS NOT NULL AND deleted_at < NOW() - INTERVAL '30 days';

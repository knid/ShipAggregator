-- Reference Data read queries
-- Tables: ref_cities, ref_districts, ref_neighborhoods, ref_postal_codes

-- ============================================================================
-- CITIES
-- ============================================================================

-- name: GetCityByCode :one
SELECT code, name, name_normalized, is_active, updated_at
FROM ref_cities
WHERE code = $1;

-- name: ListCities :many
SELECT code, name, name_normalized, is_active, updated_at
FROM ref_cities
WHERE is_active = TRUE
ORDER BY name ASC;

-- name: ListAllCities :many
SELECT code, name, name_normalized, is_active, updated_at
FROM ref_cities
ORDER BY name ASC;

-- name: SearchCitiesByName :many
SELECT code, name, name_normalized, is_active, updated_at
FROM ref_cities
WHERE name_normalized ILIKE '%' || $1 || '%' AND is_active = TRUE
ORDER BY name ASC
LIMIT sqlc.arg('limit_count')::int;

-- ============================================================================
-- DISTRICTS
-- ============================================================================

-- name: GetDistrictByCode :one
SELECT code, city_code, name, name_normalized, is_active, updated_at
FROM ref_districts
WHERE code = $1;

-- name: ListDistrictsByCity :many
SELECT code, city_code, name, name_normalized, is_active, updated_at
FROM ref_districts
WHERE city_code = $1 AND is_active = TRUE
ORDER BY name ASC;

-- name: SearchDistrictsByName :many
SELECT d.code, d.city_code, d.name, d.name_normalized, d.is_active, d.updated_at,
       c.name as city_name
FROM ref_districts d
JOIN ref_cities c ON d.city_code = c.code
WHERE d.name_normalized ILIKE '%' || $1 || '%' AND d.is_active = TRUE
ORDER BY c.name, d.name ASC
LIMIT sqlc.arg('limit_count')::int;

-- name: GetDistrictWithCity :one
SELECT d.code, d.city_code, d.name, d.name_normalized, d.is_active, d.updated_at,
       c.name as city_name
FROM ref_districts d
JOIN ref_cities c ON d.city_code = c.code
WHERE d.code = $1;

-- ============================================================================
-- NEIGHBORHOODS
-- ============================================================================

-- name: GetNeighborhoodByCode :one
SELECT code, district_code, name, name_normalized, postal_code, is_active, updated_at
FROM ref_neighborhoods
WHERE code = $1;

-- name: ListNeighborhoodsByDistrict :many
SELECT code, district_code, name, name_normalized, postal_code, is_active, updated_at
FROM ref_neighborhoods
WHERE district_code = $1 AND is_active = TRUE
ORDER BY name ASC;

-- name: GetNeighborhoodByPostalCode :one
SELECT code, district_code, name, name_normalized, postal_code, is_active, updated_at
FROM ref_neighborhoods
WHERE postal_code = $1 AND is_active = TRUE
LIMIT 1;

-- name: SearchNeighborhoodsByName :many
SELECT n.code, n.district_code, n.name, n.name_normalized, n.postal_code, n.is_active, n.updated_at,
       d.name as district_name, c.name as city_name
FROM ref_neighborhoods n
JOIN ref_districts d ON n.district_code = d.code
JOIN ref_cities c ON d.city_code = c.code
WHERE n.name_normalized ILIKE '%' || $1 || '%' AND n.is_active = TRUE
ORDER BY c.name, d.name, n.name ASC
LIMIT sqlc.arg('limit_count')::int;

-- name: GetNeighborhoodWithHierarchy :one
SELECT n.code, n.district_code, n.name, n.name_normalized, n.postal_code, n.is_active, n.updated_at,
       d.code as district_code_full, d.name as district_name,
       c.code as city_code, c.name as city_name
FROM ref_neighborhoods n
JOIN ref_districts d ON n.district_code = d.code
JOIN ref_cities c ON d.city_code = c.code
WHERE n.code = $1;

-- ============================================================================
-- POSTAL CODES
-- ============================================================================

-- name: GetPostalCodeByCode :one
SELECT code, district_code, is_active, updated_at
FROM ref_postal_codes
WHERE code = $1;

-- name: ListPostalCodesByDistrict :many
SELECT code, district_code, is_active, updated_at
FROM ref_postal_codes
WHERE district_code = $1 AND is_active = TRUE
ORDER BY code ASC;

-- ============================================================================
-- COMBINED QUERIES
-- ============================================================================

-- name: GetLocationByPostalCode :one
SELECT n.code as neighborhood_code, n.name as neighborhood_name, n.postal_code,
       d.code as district_code, d.name as district_name,
       c.code as city_code, c.name as city_name
FROM ref_neighborhoods n
JOIN ref_districts d ON n.district_code = d.code
JOIN ref_cities c ON d.city_code = c.code
WHERE n.postal_code = $1 AND n.is_active = TRUE
LIMIT 1;

-- name: SearchLocations :many
SELECT n.code as neighborhood_code, n.name as neighborhood_name, n.postal_code,
       d.code as district_code, d.name as district_name,
       c.code as city_code, c.name as city_name
FROM ref_neighborhoods n
JOIN ref_districts d ON n.district_code = d.code
JOIN ref_cities c ON d.city_code = c.code
WHERE (n.name_normalized ILIKE '%' || $1 || '%'
   OR d.name_normalized ILIKE '%' || $1 || '%'
   OR c.name_normalized ILIKE '%' || $1 || '%')
   AND n.is_active = TRUE
ORDER BY c.name, d.name, n.name ASC
LIMIT sqlc.arg('limit_count')::int;

-- name: GetReferenceDataCounts :one
SELECT 
    (SELECT COUNT(*) FROM ref_cities WHERE is_active = TRUE) as city_count,
    (SELECT COUNT(*) FROM ref_districts WHERE is_active = TRUE) as district_count,
    (SELECT COUNT(*) FROM ref_neighborhoods WHERE is_active = TRUE) as neighborhood_count,
    (SELECT COUNT(*) FROM ref_postal_codes WHERE is_active = TRUE) as postal_code_count;

-- name: ValidateLocationCodes :one
SELECT EXISTS(
    SELECT 1 FROM ref_neighborhoods n
    JOIN ref_districts d ON n.district_code = d.code
    JOIN ref_cities c ON d.city_code = c.code
    WHERE c.code = $1 AND d.code = $2 AND n.code = $3
      AND c.is_active = TRUE AND d.is_active = TRUE AND n.is_active = TRUE
) as is_valid;

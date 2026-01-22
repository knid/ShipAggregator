-- Migration: 000003_reference_data
-- Description: Create reference data tables for Turkish cities, districts, neighborhoods, and postal codes
-- Created: 2026-01-22

-- ============================================================================
-- REF_CITIES TABLE
-- Turkish cities reference data (81 provinces)
-- ============================================================================
CREATE TABLE ref_cities (
    code VARCHAR(10) PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    name_normalized VARCHAR(100) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Normalized name index for search
CREATE INDEX idx_ref_cities_name_normalized ON ref_cities(name_normalized);

COMMENT ON TABLE ref_cities IS 'Turkish cities (provinces) reference data';
COMMENT ON COLUMN ref_cities.code IS 'City plate code (e.g., "34" for Istanbul)';
COMMENT ON COLUMN ref_cities.name_normalized IS 'Lowercase, ASCII normalized name for search';

-- ============================================================================
-- REF_DISTRICTS TABLE
-- Turkish districts (ilçe) reference data
-- ============================================================================
CREATE TABLE ref_districts (
    code VARCHAR(20) PRIMARY KEY,
    city_code VARCHAR(10) NOT NULL REFERENCES ref_cities(code) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    name_normalized VARCHAR(100) NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- City lookup index
CREATE INDEX idx_ref_districts_city_code ON ref_districts(city_code);

-- Normalized name index for search
CREATE INDEX idx_ref_districts_name_normalized ON ref_districts(name_normalized);

COMMENT ON TABLE ref_districts IS 'Turkish districts (ilçe) reference data';
COMMENT ON COLUMN ref_districts.code IS 'District code (city_code + district identifier)';

-- ============================================================================
-- REF_NEIGHBORHOODS TABLE
-- Turkish neighborhoods (mahalle) reference data
-- ============================================================================
CREATE TABLE ref_neighborhoods (
    code VARCHAR(30) PRIMARY KEY,
    district_code VARCHAR(20) NOT NULL REFERENCES ref_districts(code) ON DELETE CASCADE,
    name VARCHAR(150) NOT NULL,
    name_normalized VARCHAR(150) NOT NULL,
    postal_code VARCHAR(10) NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- District lookup index
CREATE INDEX idx_ref_neighborhoods_district_code ON ref_neighborhoods(district_code);

-- Postal code lookup index
CREATE INDEX idx_ref_neighborhoods_postal_code ON ref_neighborhoods(postal_code);

-- Normalized name index for search
CREATE INDEX idx_ref_neighborhoods_name_normalized ON ref_neighborhoods(name_normalized);

COMMENT ON TABLE ref_neighborhoods IS 'Turkish neighborhoods (mahalle) reference data';
COMMENT ON COLUMN ref_neighborhoods.postal_code IS 'Associated postal code (may be NULL)';

-- ============================================================================
-- REF_POSTAL_CODES TABLE
-- Turkish postal codes reference data
-- ============================================================================
CREATE TABLE ref_postal_codes (
    code VARCHAR(10) PRIMARY KEY,
    district_code VARCHAR(20) NOT NULL REFERENCES ref_districts(code) ON DELETE CASCADE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- District lookup index
CREATE INDEX idx_ref_postal_codes_district_code ON ref_postal_codes(district_code);

COMMENT ON TABLE ref_postal_codes IS 'Turkish postal codes reference data';
COMMENT ON COLUMN ref_postal_codes.code IS '5-digit Turkish postal code';

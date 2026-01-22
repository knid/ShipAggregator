-- Migration: 000003_reference_data (rollback)
-- Description: Drop reference data tables
-- Created: 2026-01-22

-- Drop tables in reverse order of creation (respecting foreign key dependencies)
DROP TABLE IF EXISTS ref_postal_codes;
DROP TABLE IF EXISTS ref_neighborhoods;
DROP TABLE IF EXISTS ref_districts;
DROP TABLE IF EXISTS ref_cities;

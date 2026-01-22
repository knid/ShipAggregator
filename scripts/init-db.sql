-- =============================================================================
-- PostgreSQL Initialization Script for TR-Only Shipping Aggregation Backend
-- =============================================================================
-- This script runs when the PostgreSQL container is first created

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Create application user with limited privileges (for production)
-- In development, we use the default user
-- DO $$
-- BEGIN
--     IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'shipping_app') THEN
--         CREATE ROLE shipping_app WITH LOGIN PASSWORD 'shipping_app_password';
--     END IF;
-- END
-- $$;

-- Grant privileges
-- GRANT CONNECT ON DATABASE shipping TO shipping_app;
-- GRANT USAGE ON SCHEMA public TO shipping_app;
-- ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO shipping_app;
-- ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO shipping_app;

-- Log initialization
DO $$
BEGIN
    RAISE NOTICE 'Database initialized successfully at %', NOW();
END
$$;

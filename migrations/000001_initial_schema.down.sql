-- Migration: 000001_initial_schema (rollback)
-- Description: Drop core tables for organizations, members, roles, and sessions
-- Created: 2026-01-22

-- Drop RLS policies first
DROP POLICY IF EXISTS sessions_tenant_insert ON sessions;
DROP POLICY IF EXISTS sessions_tenant_isolation ON sessions;
DROP POLICY IF EXISTS roles_tenant_insert ON roles;
DROP POLICY IF EXISTS roles_tenant_isolation ON roles;
DROP POLICY IF EXISTS members_tenant_insert ON members;
DROP POLICY IF EXISTS members_tenant_isolation ON members;

-- Drop tables in reverse order of creation (respecting foreign key dependencies)
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS members;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS organizations;

-- Note: We don't drop the pgcrypto extension as it may be used by other schemas

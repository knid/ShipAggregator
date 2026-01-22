-- Migration: 000001_initial_schema
-- Description: Create core tables for organizations, members, roles, and sessions
-- Created: 2026-01-22

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ============================================================================
-- ORGANIZATIONS TABLE
-- Multi-tenant root entity. All customer resources belong to an organization.
-- ============================================================================
CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL,
    billing_email VARCHAR(255) NOT NULL,
    settings JSONB NOT NULL DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL,
    
    CONSTRAINT chk_organizations_status CHECK (status IN ('active', 'suspended', 'closed'))
);

-- Unique slug constraint (only for non-deleted organizations)
CREATE UNIQUE INDEX idx_organizations_slug ON organizations(slug) WHERE deleted_at IS NULL;

-- Status index for filtering
CREATE INDEX idx_organizations_status ON organizations(status);

COMMENT ON TABLE organizations IS 'Multi-tenant root entity. All customer resources belong to an organization.';
COMMENT ON COLUMN organizations.slug IS 'URL-safe identifier for the organization';
COMMENT ON COLUMN organizations.settings IS 'Organization-level settings as JSON';
COMMENT ON COLUMN organizations.status IS 'Account status: active, suspended, or closed';

-- ============================================================================
-- ROLES TABLE
-- Permission sets assignable to members.
-- Note: Created before members table due to foreign key dependency
-- ============================================================================
CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description VARCHAR(500) NULL,
    permissions JSONB NOT NULL DEFAULT '[]',
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Unique role name per organization (NULL organization_id for system roles)
CREATE UNIQUE INDEX idx_roles_org_name ON roles(organization_id, name);

-- Organization lookup index
CREATE INDEX idx_roles_organization_id ON roles(organization_id);

COMMENT ON TABLE roles IS 'Permission sets assignable to members';
COMMENT ON COLUMN roles.organization_id IS 'NULL for system roles, organization_id for custom roles';
COMMENT ON COLUMN roles.permissions IS 'Array of permission strings';
COMMENT ON COLUMN roles.is_system IS 'System roles cannot be edited or deleted';

-- ============================================================================
-- MEMBERS TABLE
-- Users belonging to organizations with role-based access.
-- ============================================================================
CREATE TABLE members (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NULL,
    role_id UUID NOT NULL REFERENCES roles(id),
    status VARCHAR(20) NOT NULL DEFAULT 'invited',
    invited_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    joined_at TIMESTAMPTZ NULL,
    last_login_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_members_status CHECK (status IN ('invited', 'active', 'suspended', 'removed'))
);

-- Unique email per organization (excluding removed members)
CREATE UNIQUE INDEX idx_members_org_email ON members(organization_id, email) WHERE status != 'removed';

-- Organization lookup index
CREATE INDEX idx_members_organization_id ON members(organization_id);

-- Email lookup index (for login)
CREATE INDEX idx_members_email ON members(email);

COMMENT ON TABLE members IS 'Users belonging to organizations with role-based access';
COMMENT ON COLUMN members.password_hash IS 'Bcrypt hash of password, NULL for invited members';
COMMENT ON COLUMN members.status IS 'Member status: invited, active, suspended, or removed';

-- ============================================================================
-- SESSIONS TABLE
-- Dashboard authentication sessions.
-- ============================================================================
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    member_id UUID NOT NULL REFERENCES members(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL,
    ip_address INET NOT NULL,
    user_agent VARCHAR(500) NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_activity_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Token hash lookup index
CREATE INDEX idx_sessions_token_hash ON sessions(token_hash);

-- Member lookup index
CREATE INDEX idx_sessions_member_id ON sessions(member_id);

-- Expiration index for cleanup
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

COMMENT ON TABLE sessions IS 'Dashboard authentication sessions';
COMMENT ON COLUMN sessions.token_hash IS 'SHA-256 hash of session token';
COMMENT ON COLUMN sessions.expires_at IS 'Session expiration timestamp';

-- ============================================================================
-- ROW-LEVEL SECURITY
-- Enable RLS on tenant-scoped tables
-- ============================================================================

-- Enable RLS on members table
ALTER TABLE members ENABLE ROW LEVEL SECURITY;

CREATE POLICY members_tenant_isolation ON members
    USING (organization_id = current_setting('app.current_organization_id', true)::uuid);

CREATE POLICY members_tenant_insert ON members
    FOR INSERT
    WITH CHECK (organization_id = current_setting('app.current_organization_id', true)::uuid);

-- Enable RLS on roles table (for custom roles only, system roles are accessible to all)
ALTER TABLE roles ENABLE ROW LEVEL SECURITY;

CREATE POLICY roles_tenant_isolation ON roles
    USING (
        organization_id IS NULL  -- System roles accessible to all
        OR organization_id = current_setting('app.current_organization_id', true)::uuid
    );

CREATE POLICY roles_tenant_insert ON roles
    FOR INSERT
    WITH CHECK (
        organization_id IS NULL  -- Allow system role creation (admin only)
        OR organization_id = current_setting('app.current_organization_id', true)::uuid
    );

-- Enable RLS on sessions table
ALTER TABLE sessions ENABLE ROW LEVEL SECURITY;

CREATE POLICY sessions_tenant_isolation ON sessions
    USING (
        member_id IN (
            SELECT id FROM members 
            WHERE organization_id = current_setting('app.current_organization_id', true)::uuid
        )
    );

CREATE POLICY sessions_tenant_insert ON sessions
    FOR INSERT
    WITH CHECK (
        member_id IN (
            SELECT id FROM members 
            WHERE organization_id = current_setting('app.current_organization_id', true)::uuid
        )
    );

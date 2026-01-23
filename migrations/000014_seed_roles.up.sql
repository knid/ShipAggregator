-- Migration: 000014_seed_roles
-- Description: Seed system roles (owner, admin, operator, viewer)
-- Created: 2026-01-22

-- ============================================================================
-- SYSTEM ROLES SEED DATA
-- Insert system-level roles that are available to all organizations
-- These roles have organization_id = NULL and is_system = TRUE
-- ============================================================================

-- Owner Role - Full access to all resources
INSERT INTO roles (id, organization_id, name, description, permissions, is_system)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    NULL,
    'owner',
    'Full access to all resources including organization management and billing',
    '[
        "organization:read",
        "organization:update",
        "organization:delete",
        "members:read",
        "members:create",
        "members:update",
        "members:delete",
        "roles:read",
        "roles:create",
        "roles:update",
        "roles:delete",
        "api_keys:read",
        "api_keys:create",
        "api_keys:update",
        "api_keys:delete",
        "api_keys:rotate",
        "addresses:read",
        "addresses:create",
        "addresses:update",
        "addresses:delete",
        "providers:read",
        "provider_accounts:read",
        "provider_accounts:create",
        "provider_accounts:update",
        "provider_accounts:delete",
        "provider_accounts:verify",
        "quotes:read",
        "quotes:create",
        "shipments:read",
        "shipments:create",
        "shipments:update",
        "shipments:cancel",
        "shipments:return",
        "shipments:pickup",
        "shipments:label",
        "tracking:read",
        "webhooks:read",
        "webhooks:create",
        "webhooks:update",
        "webhooks:delete",
        "webhooks:test",
        "billing:read",
        "billing:topup",
        "billing:refund",
        "billing:cod_reconcile",
        "admin:audit_logs",
        "admin:provider_logs",
        "admin:health"
    ]'::jsonb,
    TRUE
);

-- Admin Role - Full access except organization deletion and some billing operations
INSERT INTO roles (id, organization_id, name, description, permissions, is_system)
VALUES (
    '00000000-0000-0000-0000-000000000002',
    NULL,
    'admin',
    'Full access except organization deletion and critical billing operations',
    '[
        "organization:read",
        "organization:update",
        "members:read",
        "members:create",
        "members:update",
        "members:delete",
        "roles:read",
        "roles:create",
        "roles:update",
        "roles:delete",
        "api_keys:read",
        "api_keys:create",
        "api_keys:update",
        "api_keys:delete",
        "api_keys:rotate",
        "addresses:read",
        "addresses:create",
        "addresses:update",
        "addresses:delete",
        "providers:read",
        "provider_accounts:read",
        "provider_accounts:create",
        "provider_accounts:update",
        "provider_accounts:delete",
        "provider_accounts:verify",
        "quotes:read",
        "quotes:create",
        "shipments:read",
        "shipments:create",
        "shipments:update",
        "shipments:cancel",
        "shipments:return",
        "shipments:pickup",
        "shipments:label",
        "tracking:read",
        "webhooks:read",
        "webhooks:create",
        "webhooks:update",
        "webhooks:delete",
        "webhooks:test",
        "billing:read",
        "billing:topup",
        "admin:audit_logs",
        "admin:provider_logs",
        "admin:health"
    ]'::jsonb,
    TRUE
);

-- Operator Role - Shipment and quote management
INSERT INTO roles (id, organization_id, name, description, permissions, is_system)
VALUES (
    '00000000-0000-0000-0000-000000000003',
    NULL,
    'operator',
    'Shipment and quote management, limited administrative access',
    '[
        "organization:read",
        "members:read",
        "addresses:read",
        "addresses:create",
        "addresses:update",
        "providers:read",
        "provider_accounts:read",
        "quotes:read",
        "quotes:create",
        "shipments:read",
        "shipments:create",
        "shipments:update",
        "shipments:cancel",
        "shipments:return",
        "shipments:pickup",
        "shipments:label",
        "tracking:read",
        "webhooks:read",
        "billing:read"
    ]'::jsonb,
    TRUE
);

-- Viewer Role - Read-only access
INSERT INTO roles (id, organization_id, name, description, permissions, is_system)
VALUES (
    '00000000-0000-0000-0000-000000000004',
    NULL,
    'viewer',
    'Read-only access to all resources',
    '[
        "organization:read",
        "members:read",
        "roles:read",
        "api_keys:read",
        "addresses:read",
        "providers:read",
        "provider_accounts:read",
        "quotes:read",
        "shipments:read",
        "tracking:read",
        "webhooks:read",
        "billing:read"
    ]'::jsonb,
    TRUE
);

-- ============================================================================
-- PERMISSION REFERENCE
-- ============================================================================
-- 
-- Organization Permissions:
--   organization:read    - View organization details
--   organization:update  - Update organization settings
--   organization:delete  - Delete organization (owner only)
--
-- Member Permissions:
--   members:read    - View members
--   members:create  - Invite new members
--   members:update  - Update member roles/status
--   members:delete  - Remove members
--
-- Role Permissions:
--   roles:read    - View roles
--   roles:create  - Create custom roles
--   roles:update  - Update custom roles
--   roles:delete  - Delete custom roles
--
-- API Key Permissions:
--   api_keys:read    - View API keys
--   api_keys:create  - Create API keys
--   api_keys:update  - Update API key settings
--   api_keys:delete  - Revoke API keys
--   api_keys:rotate  - Rotate API keys
--
-- Address Permissions:
--   addresses:read    - View addresses
--   addresses:create  - Create addresses
--   addresses:update  - Update addresses
--   addresses:delete  - Delete addresses
--
-- Provider Permissions:
--   providers:read  - View available shipping providers and services
--
-- Provider Account Permissions:
--   provider_accounts:read    - View BYOA accounts
--   provider_accounts:create  - Create BYOA accounts
--   provider_accounts:update  - Update BYOA accounts
--   provider_accounts:delete  - Delete BYOA accounts
--   provider_accounts:verify  - Verify BYOA credentials
--
-- Quote Permissions:
--   quotes:read    - View quotes
--   quotes:create  - Create quotes
--
-- Shipment Permissions:
--   shipments:read    - View shipments
--   shipments:create  - Create shipments
--   shipments:update  - Update shipment metadata
--   shipments:cancel  - Cancel shipments
--   shipments:return  - Create return shipments
--   shipments:pickup  - Schedule pickups
--   shipments:label   - Purchase labels
--
-- Tracking Permissions:
--   tracking:read  - View tracking events
--
-- Webhook Permissions:
--   webhooks:read    - View webhook endpoints
--   webhooks:create  - Create webhook endpoints
--   webhooks:update  - Update webhook endpoints
--   webhooks:delete  - Delete webhook endpoints
--   webhooks:test    - Test webhook endpoints
--
-- Billing Permissions:
--   billing:read          - View balance and transactions
--   billing:topup         - Initiate wallet top-ups
--   billing:refund        - Request refunds
--   billing:cod_reconcile - Reconcile COD settlements
--
-- Admin Permissions:
--   admin:audit_logs    - View audit logs
--   admin:provider_logs - View provider API logs
--   admin:health        - View provider health status
-- ============================================================================

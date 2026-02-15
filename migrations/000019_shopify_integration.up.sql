-- Shopify integration tables
-- Tables: shopify_stores, shopify_order_mappings, shopify_shipping_rules

-- shopify_stores: Connected Shopify stores per organization
CREATE TABLE IF NOT EXISTS shopify_stores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    shop_domain TEXT NOT NULL,
    access_token_encrypted BYTEA,
    access_token_iv BYTEA,
    scopes TEXT,
    shopify_store_id BIGINT,
    store_name TEXT,
    store_email TEXT,
    currency TEXT DEFAULT 'TRY',
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'uninstalled')),
    sync_mode TEXT NOT NULL DEFAULT 'manual' CHECK (sync_mode IN ('manual', 'auto')),
    installed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    uninstalled_at TIMESTAMPTZ,
    last_sync_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Only one active/paused store per org+domain
CREATE UNIQUE INDEX idx_shopify_stores_org_domain_active
    ON shopify_stores (organization_id, shop_domain)
    WHERE status != 'uninstalled';

-- Webhook routing by shop domain
CREATE INDEX idx_shopify_stores_shop_domain ON shopify_stores (shop_domain);

-- shopify_order_mappings: Links Shopify orders to e-port shipments
CREATE TABLE IF NOT EXISTS shopify_order_mappings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    shopify_store_id UUID NOT NULL REFERENCES shopify_stores(id),
    shopify_order_id BIGINT NOT NULL,
    shopify_order_number TEXT,
    shipment_id UUID REFERENCES shipments(id),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN (
        'pending', 'processing', 'draft_created', 'label_purchased',
        'fulfillment_synced', 'cancelled', 'failed', 'skipped'
    )),
    sync_error TEXT,
    shopify_customer_name TEXT,
    shopify_order_total_kurus BIGINT,
    shopify_line_item_count INT,
    shopify_fulfillment_id BIGINT,
    fulfillment_synced_at TIMESTAMPTZ,
    raw_order JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Prevent duplicate order processing
CREATE UNIQUE INDEX idx_shopify_order_mappings_store_order
    ON shopify_order_mappings (shopify_store_id, shopify_order_id);

-- Look up mapping by shipment for fulfillment sync
CREATE INDEX idx_shopify_order_mappings_shipment
    ON shopify_order_mappings (shipment_id)
    WHERE shipment_id IS NOT NULL;

-- List by org and status
CREATE INDEX idx_shopify_order_mappings_org_status
    ON shopify_order_mappings (organization_id, status);

-- shopify_shipping_rules: Configurable rules for auto provider selection
CREATE TABLE IF NOT EXISTS shopify_shipping_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    shopify_store_id UUID REFERENCES shopify_stores(id),
    name TEXT NOT NULL,
    priority INT NOT NULL DEFAULT 0,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    -- Conditions (all nullable, AND logic when set)
    condition_destination_city_codes TEXT[],
    condition_min_weight_grams INT,
    condition_max_weight_grams INT,
    condition_min_order_total_kurus BIGINT,
    condition_max_order_total_kurus BIGINT,
    condition_has_cod BOOLEAN,
    condition_tags TEXT[],
    -- Actions
    action_provider_code TEXT NOT NULL,
    action_service_code TEXT,
    action_provider_account_id UUID REFERENCES provider_accounts(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- List active rules by org, priority ordered
CREATE INDEX idx_shopify_shipping_rules_org_active
    ON shopify_shipping_rules (organization_id, priority)
    WHERE is_active = TRUE;

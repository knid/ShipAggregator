-- Migration: 000007_shipments
-- Description: Create shipments, tracking_events, and documents tables
-- Created: 2026-01-22

-- ============================================================================
-- DOCUMENTS TABLE
-- Stored files (labels, POD, invoices, etc.)
-- Note: Created before shipments due to foreign key dependency
-- ============================================================================
CREATE TABLE documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    type VARCHAR(30) NOT NULL,
    format VARCHAR(10) NOT NULL,
    filename VARCHAR(255) NOT NULL,
    storage_path VARCHAR(500) NOT NULL,
    storage_bucket VARCHAR(100) NOT NULL,
    content_type VARCHAR(100) NOT NULL,
    size_bytes INTEGER NOT NULL,
    content_hash VARCHAR(64) NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_documents_type CHECK (type IN ('label', 'pod', 'invoice', 'customs', 'other')),
    CONSTRAINT chk_documents_format CHECK (format IN ('pdf', 'zpl', 'png', 'jpg', 'xml'))
);

-- Organization lookup index
CREATE INDEX idx_documents_organization_id ON documents(organization_id);

-- Content hash index for deduplication
CREATE INDEX idx_documents_content_hash ON documents(content_hash);

COMMENT ON TABLE documents IS 'Stored files (labels, POD, invoices, etc.)';
COMMENT ON COLUMN documents.type IS 'Document type: label, pod, invoice, customs, other';
COMMENT ON COLUMN documents.format IS 'File format: pdf, zpl, png, jpg, xml';
COMMENT ON COLUMN documents.storage_path IS 'Object storage path (S3/MinIO)';
COMMENT ON COLUMN documents.storage_bucket IS 'Storage bucket name';
COMMENT ON COLUMN documents.content_hash IS 'SHA-256 hash of file content';

-- ============================================================================
-- SHIPMENTS TABLE
-- Core shipment records with status tracking
-- ============================================================================
CREATE TABLE shipments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    idempotency_key VARCHAR(100) NOT NULL,
    offer_id UUID NULL REFERENCES offers(id),
    provider_code VARCHAR(50) NOT NULL REFERENCES providers(code),
    service_code VARCHAR(50) NOT NULL,
    provider_account_id UUID NULL REFERENCES provider_accounts(id),
    is_marketplace BOOLEAN NOT NULL,
    provider_shipment_id VARCHAR(100) NULL,
    provider_tracking_number VARCHAR(100) NULL,
    provider_barcode VARCHAR(100) NULL,
    origin_address JSONB NOT NULL,
    destination_address JSONB NOT NULL,
    parcel JSONB NOT NULL,
    cod_amount_kurus INTEGER NULL,
    declared_value_kurus INTEGER NULL,
    status VARCHAR(30) NOT NULL DEFAULT 'draft',
    label_document_id UUID NULL REFERENCES documents(id),
    label_format VARCHAR(10) NULL,
    price_charged_kurus INTEGER NULL,
    cost_kurus INTEGER NULL,
    pickup_scheduled_at TIMESTAMPTZ NULL,
    picked_up_at TIMESTAMPTZ NULL,
    delivered_at TIMESTAMPTZ NULL,
    cancelled_at TIMESTAMPTZ NULL,
    cancellation_reason VARCHAR(500) NULL,
    return_shipment_id UUID NULL,
    original_shipment_id UUID NULL,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_shipments_status CHECK (status IN (
        'draft', 'label_purchased', 'pickup_scheduled', 'picked_up', 
        'in_transit', 'at_hub', 'out_for_delivery', 'delivered', 
        'delivery_failed', 'returning', 'returned_to_sender', 
        'cancelled', 'exception'
    )),
    CONSTRAINT chk_shipments_label_format CHECK (label_format IS NULL OR label_format IN ('pdf', 'zpl', 'png'))
);

-- Self-referential foreign keys for return shipments
ALTER TABLE shipments 
    ADD CONSTRAINT fk_shipments_return_shipment 
    FOREIGN KEY (return_shipment_id) REFERENCES shipments(id);

ALTER TABLE shipments 
    ADD CONSTRAINT fk_shipments_original_shipment 
    FOREIGN KEY (original_shipment_id) REFERENCES shipments(id);

-- Organization lookup index
CREATE INDEX idx_shipments_organization_id ON shipments(organization_id);

-- Idempotency lookup index (unique per organization)
CREATE UNIQUE INDEX idx_shipments_idempotency ON shipments(organization_id, idempotency_key);

-- Provider tracking number lookup index
CREATE INDEX idx_shipments_provider_tracking ON shipments(provider_code, provider_tracking_number);

-- Status index for filtering
CREATE INDEX idx_shipments_status ON shipments(status);

-- Creation date index for listing
CREATE INDEX idx_shipments_created_at ON shipments(created_at);

-- Offer lookup index
CREATE INDEX idx_shipments_offer_id ON shipments(offer_id);

COMMENT ON TABLE shipments IS 'Core shipment records';
COMMENT ON COLUMN shipments.idempotency_key IS 'Client-provided idempotency key';
COMMENT ON COLUMN shipments.is_marketplace IS 'TRUE for marketplace, FALSE for BYOA';
COMMENT ON COLUMN shipments.provider_shipment_id IS 'Provider''s internal shipment ID';
COMMENT ON COLUMN shipments.provider_tracking_number IS 'Provider''s tracking number';
COMMENT ON COLUMN shipments.provider_barcode IS 'Provider''s barcode for label';
COMMENT ON COLUMN shipments.origin_address IS 'Origin address snapshot as JSON';
COMMENT ON COLUMN shipments.destination_address IS 'Destination address snapshot as JSON';
COMMENT ON COLUMN shipments.parcel IS 'Parcel details (weight, dimensions) as JSON';
COMMENT ON COLUMN shipments.price_charged_kurus IS 'Amount charged to wallet in kuruş';
COMMENT ON COLUMN shipments.cost_kurus IS 'Our cost (marketplace only) in kuruş';
COMMENT ON COLUMN shipments.return_shipment_id IS 'Linked return shipment (if created)';
COMMENT ON COLUMN shipments.original_shipment_id IS 'Original shipment (for return shipments)';

-- ============================================================================
-- TRACKING_EVENTS TABLE
-- Shipment tracking history with normalized status codes
-- ============================================================================
CREATE TABLE tracking_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    shipment_id UUID NOT NULL REFERENCES shipments(id) ON DELETE CASCADE,
    provider_code VARCHAR(50) NOT NULL,
    status_code VARCHAR(30) NOT NULL,
    status_description VARCHAR(500) NOT NULL,
    location_city VARCHAR(100) NULL,
    location_detail VARCHAR(500) NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    raw_status_code VARCHAR(100) NULL,
    raw_data JSONB NULL,
    source VARCHAR(20) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT chk_tracking_events_source CHECK (source IN ('webhook', 'poll', 'manual'))
);

-- Shipment lookup index
CREATE INDEX idx_tracking_events_shipment_id ON tracking_events(shipment_id);

-- Occurred at index for ordering
CREATE INDEX idx_tracking_events_occurred_at ON tracking_events(occurred_at);

-- Composite index for shipment timeline queries
CREATE INDEX idx_tracking_events_shipment_occurred ON tracking_events(shipment_id, occurred_at DESC);

COMMENT ON TABLE tracking_events IS 'Shipment tracking history';
COMMENT ON COLUMN tracking_events.status_code IS 'Normalized status code (matches shipment status values)';
COMMENT ON COLUMN tracking_events.status_description IS 'Human-readable description';
COMMENT ON COLUMN tracking_events.raw_status_code IS 'Provider''s original status code';
COMMENT ON COLUMN tracking_events.raw_data IS 'Full provider response as JSON';
COMMENT ON COLUMN tracking_events.source IS 'Event source: webhook, poll, or manual';

-- ============================================================================
-- ROW-LEVEL SECURITY
-- Enable RLS on shipments, tracking_events, and documents tables
-- ============================================================================
ALTER TABLE documents ENABLE ROW LEVEL SECURITY;

CREATE POLICY documents_tenant_isolation ON documents
    USING (organization_id = current_setting('app.current_organization_id', true)::uuid);

CREATE POLICY documents_tenant_insert ON documents
    FOR INSERT
    WITH CHECK (organization_id = current_setting('app.current_organization_id', true)::uuid);

ALTER TABLE shipments ENABLE ROW LEVEL SECURITY;

CREATE POLICY shipments_tenant_isolation ON shipments
    USING (organization_id = current_setting('app.current_organization_id', true)::uuid);

CREATE POLICY shipments_tenant_insert ON shipments
    FOR INSERT
    WITH CHECK (organization_id = current_setting('app.current_organization_id', true)::uuid);

-- Tracking events inherit tenant isolation through shipments
ALTER TABLE tracking_events ENABLE ROW LEVEL SECURITY;

CREATE POLICY tracking_events_tenant_isolation ON tracking_events
    USING (
        shipment_id IN (
            SELECT id FROM shipments 
            WHERE organization_id = current_setting('app.current_organization_id', true)::uuid
        )
    );

CREATE POLICY tracking_events_tenant_insert ON tracking_events
    FOR INSERT
    WITH CHECK (
        shipment_id IN (
            SELECT id FROM shipments 
            WHERE organization_id = current_setting('app.current_organization_id', true)::uuid
        )
    );

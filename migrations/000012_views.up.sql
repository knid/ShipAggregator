-- Migration: 000012_views
-- Description: Create v_wallet_balances and v_provider_health views
-- Created: 2026-01-22

-- ============================================================================
-- V_WALLET_BALANCES VIEW
-- Computed wallet balances from ledger entries
-- ============================================================================
CREATE VIEW v_wallet_balances AS
SELECT 
    organization_id,
    COALESCE(SUM(CASE WHEN entry_type = 'credit' THEN amount_kurus ELSE 0 END), 0) -
    COALESCE(SUM(CASE WHEN entry_type = 'debit' THEN amount_kurus ELSE 0 END), 0) AS balance_kurus,
    MAX(created_at) AS last_transaction_at
FROM ledger_entries
GROUP BY organization_id;

COMMENT ON VIEW v_wallet_balances IS 'Computed wallet balances from ledger entries';
COMMENT ON COLUMN v_wallet_balances.balance_kurus IS 'Current balance in kuruş (credits - debits)';
COMMENT ON COLUMN v_wallet_balances.last_transaction_at IS 'Timestamp of last transaction';

-- ============================================================================
-- V_PROVIDER_HEALTH VIEW
-- Provider health metrics from the last 1 hour
-- ============================================================================
CREATE VIEW v_provider_health AS
SELECT 
    provider_code,
    COUNT(*) AS total_calls,
    SUM(CASE WHEN success THEN 1 ELSE 0 END) AS successful_calls,
    ROUND(100.0 * SUM(CASE WHEN success THEN 1 ELSE 0 END) / NULLIF(COUNT(*), 0), 2) AS success_rate,
    ROUND(AVG(latency_ms), 2) AS avg_latency_ms,
    PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency_ms) AS p95_latency_ms,
    MAX(CASE WHEN success THEN created_at END) AS last_success_at
FROM provider_api_logs
WHERE created_at > NOW() - INTERVAL '1 hour'
GROUP BY provider_code;

COMMENT ON VIEW v_provider_health IS 'Provider health metrics from the last 1 hour';
COMMENT ON COLUMN v_provider_health.total_calls IS 'Total API calls in the last hour';
COMMENT ON COLUMN v_provider_health.successful_calls IS 'Successful API calls in the last hour';
COMMENT ON COLUMN v_provider_health.success_rate IS 'Success rate percentage';
COMMENT ON COLUMN v_provider_health.avg_latency_ms IS 'Average latency in milliseconds';
COMMENT ON COLUMN v_provider_health.p95_latency_ms IS '95th percentile latency in milliseconds';
COMMENT ON COLUMN v_provider_health.last_success_at IS 'Timestamp of last successful call';

-- ============================================================================
-- V_ORGANIZATION_STATS VIEW
-- Organization statistics for dashboard
-- ============================================================================
CREATE VIEW v_organization_stats AS
SELECT 
    o.id AS organization_id,
    o.name AS organization_name,
    o.status AS organization_status,
    COALESCE(wb.balance_kurus, 0) AS balance_kurus,
    COALESCE(s.total_shipments, 0) AS total_shipments,
    COALESCE(s.active_shipments, 0) AS active_shipments,
    COALESCE(s.delivered_shipments, 0) AS delivered_shipments,
    COALESCE(m.member_count, 0) AS member_count,
    COALESCE(ak.api_key_count, 0) AS api_key_count
FROM organizations o
LEFT JOIN v_wallet_balances wb ON wb.organization_id = o.id
LEFT JOIN (
    SELECT 
        organization_id,
        COUNT(*) AS total_shipments,
        SUM(CASE WHEN status NOT IN ('delivered', 'cancelled', 'returned_to_sender') THEN 1 ELSE 0 END) AS active_shipments,
        SUM(CASE WHEN status = 'delivered' THEN 1 ELSE 0 END) AS delivered_shipments
    FROM shipments
    GROUP BY organization_id
) s ON s.organization_id = o.id
LEFT JOIN (
    SELECT organization_id, COUNT(*) AS member_count
    FROM members
    WHERE status = 'active'
    GROUP BY organization_id
) m ON m.organization_id = o.id
LEFT JOIN (
    SELECT organization_id, COUNT(*) AS api_key_count
    FROM api_keys
    WHERE is_active = TRUE AND revoked_at IS NULL
    GROUP BY organization_id
) ak ON ak.organization_id = o.id
WHERE o.deleted_at IS NULL;

COMMENT ON VIEW v_organization_stats IS 'Organization statistics for dashboard';

-- ============================================================================
-- V_SHIPMENT_SUMMARY VIEW
-- Shipment summary with provider and pricing info
-- ============================================================================
CREATE VIEW v_shipment_summary AS
SELECT 
    s.id,
    s.organization_id,
    s.idempotency_key,
    s.provider_code,
    p.name AS provider_name,
    s.service_code,
    s.is_marketplace,
    s.provider_tracking_number,
    s.status,
    s.origin_address->>'city_code' AS origin_city,
    s.destination_address->>'city_code' AS destination_city,
    s.parcel->>'weight_grams' AS weight_grams,
    s.cod_amount_kurus,
    s.price_charged_kurus,
    s.cost_kurus,
    s.price_charged_kurus - COALESCE(s.cost_kurus, 0) AS margin_kurus,
    s.created_at,
    s.updated_at,
    s.delivered_at,
    s.cancelled_at
FROM shipments s
JOIN providers p ON p.code = s.provider_code;

COMMENT ON VIEW v_shipment_summary IS 'Shipment summary with provider and pricing info';

-- ============================================================================
-- V_WEBHOOK_DELIVERY_STATS VIEW
-- Webhook delivery statistics per endpoint
-- ============================================================================
CREATE VIEW v_webhook_delivery_stats AS
SELECT 
    we.id AS endpoint_id,
    we.organization_id,
    we.url,
    we.is_active,
    COUNT(wd.id) AS total_deliveries,
    SUM(CASE WHEN wd.status = 'delivered' THEN 1 ELSE 0 END) AS successful_deliveries,
    SUM(CASE WHEN wd.status = 'failed' THEN 1 ELSE 0 END) AS failed_deliveries,
    SUM(CASE WHEN wd.status = 'dlq' THEN 1 ELSE 0 END) AS dlq_deliveries,
    SUM(CASE WHEN wd.status = 'pending' THEN 1 ELSE 0 END) AS pending_deliveries,
    ROUND(100.0 * SUM(CASE WHEN wd.status = 'delivered' THEN 1 ELSE 0 END) / NULLIF(COUNT(wd.id), 0), 2) AS success_rate,
    ROUND(AVG(wd.response_time_ms), 2) AS avg_response_time_ms,
    MAX(CASE WHEN wd.status = 'delivered' THEN wd.last_attempt_at END) AS last_successful_delivery
FROM webhook_endpoints we
LEFT JOIN webhook_subscriptions ws ON ws.endpoint_id = we.id
LEFT JOIN webhook_deliveries wd ON wd.subscription_id = ws.id
GROUP BY we.id, we.organization_id, we.url, we.is_active;

COMMENT ON VIEW v_webhook_delivery_stats IS 'Webhook delivery statistics per endpoint';

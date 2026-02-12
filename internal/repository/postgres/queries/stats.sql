-- Stats queries for dashboard statistics
-- These queries aggregate data for the dashboard overview

-- name: GetShipmentStats :one
SELECT
    COUNT(*)::bigint AS total_shipments,
    COALESCE(SUM(CASE WHEN status = 'draft' THEN 1 ELSE 0 END), 0)::bigint AS draft_count,
    COALESCE(SUM(CASE WHEN status = 'label_purchased' THEN 1 ELSE 0 END), 0)::bigint AS label_purchased_count,
    COALESCE(SUM(CASE WHEN status = 'in_transit' THEN 1 ELSE 0 END), 0)::bigint AS in_transit_count,
    COALESCE(SUM(CASE WHEN status = 'delivered' THEN 1 ELSE 0 END), 0)::bigint AS delivered_count,
    COALESCE(SUM(CASE WHEN status = 'cancelled' THEN 1 ELSE 0 END), 0)::bigint AS cancelled_count,
    COALESCE(SUM(CASE WHEN status = 'returned' THEN 1 ELSE 0 END), 0)::bigint AS returned_count,
    COALESCE(SUM(price_charged_kurus), 0)::bigint AS total_spent_kurus
FROM shipments
WHERE organization_id = $1
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR created_at >= sqlc.narg('date_from')::timestamptz)
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR created_at <= sqlc.narg('date_to')::timestamptz);

-- name: GetShipmentStatsByProvider :many
SELECT
    provider_code,
    COUNT(*)::bigint AS shipment_count
FROM shipments
WHERE organization_id = $1
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR created_at >= sqlc.narg('date_from')::timestamptz)
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR created_at <= sqlc.narg('date_to')::timestamptz)
GROUP BY provider_code
ORDER BY shipment_count DESC;

-- name: GetBillingStats :one
SELECT
    COALESCE(SUM(CASE WHEN entry_type = 'credit' THEN amount_kurus ELSE 0 END), 0)::bigint AS total_credits_kurus,
    COALESCE(SUM(CASE WHEN entry_type = 'debit' THEN amount_kurus ELSE 0 END), 0)::bigint AS total_debits_kurus
FROM ledger_entries
WHERE organization_id = $1
  AND (sqlc.narg('date_from')::timestamptz IS NULL OR created_at >= sqlc.narg('date_from')::timestamptz)
  AND (sqlc.narg('date_to')::timestamptz IS NULL OR created_at <= sqlc.narg('date_to')::timestamptz);

-- name: GetPendingCODTotal :one
SELECT COALESCE(SUM(expected_amount_kurus), 0)::bigint AS pending_cod_kurus
FROM cod_settlements
WHERE organization_id = $1
  AND status = 'pending';

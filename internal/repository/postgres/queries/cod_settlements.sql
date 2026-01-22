-- COD Settlements CRUD queries
-- Table: cod_settlements

-- name: GetCODSettlementByID :one
SELECT id, organization_id, shipment_id, provider_code, expected_amount_kurus,
       actual_amount_kurus, carrier_settlement_id, carrier_settlement_date, status,
       ledger_entry_id, discrepancy_kurus, discrepancy_reason, settled_at,
       created_at, updated_at
FROM cod_settlements
WHERE id = $1;

-- name: GetCODSettlementByShipment :one
SELECT id, organization_id, shipment_id, provider_code, expected_amount_kurus,
       actual_amount_kurus, carrier_settlement_id, carrier_settlement_date, status,
       ledger_entry_id, discrepancy_kurus, discrepancy_reason, settled_at,
       created_at, updated_at
FROM cod_settlements
WHERE shipment_id = $1;

-- name: GetCODSettlementByCarrierID :one
SELECT id, organization_id, shipment_id, provider_code, expected_amount_kurus,
       actual_amount_kurus, carrier_settlement_id, carrier_settlement_date, status,
       ledger_entry_id, discrepancy_kurus, discrepancy_reason, settled_at,
       created_at, updated_at
FROM cod_settlements
WHERE provider_code = $1 AND carrier_settlement_id = $2;

-- name: ListCODSettlementsByOrganization :many
SELECT id, organization_id, shipment_id, provider_code, expected_amount_kurus,
       actual_amount_kurus, carrier_settlement_id, carrier_settlement_date, status,
       ledger_entry_id, discrepancy_kurus, discrepancy_reason, settled_at,
       created_at, updated_at
FROM cod_settlements
WHERE organization_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status')::varchar)
  AND (sqlc.narg('provider_code')::varchar IS NULL OR provider_code = sqlc.narg('provider_code')::varchar)
ORDER BY created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: CountCODSettlementsByOrganization :one
SELECT COUNT(*) as total
FROM cod_settlements
WHERE organization_id = $1
  AND (sqlc.narg('status')::varchar IS NULL OR status = sqlc.narg('status')::varchar)
  AND (sqlc.narg('provider_code')::varchar IS NULL OR provider_code = sqlc.narg('provider_code')::varchar);

-- name: ListPendingCODSettlements :many
SELECT id, organization_id, shipment_id, provider_code, expected_amount_kurus,
       actual_amount_kurus, carrier_settlement_id, carrier_settlement_date, status,
       ledger_entry_id, discrepancy_kurus, discrepancy_reason, settled_at,
       created_at, updated_at
FROM cod_settlements
WHERE status = 'pending'
  AND (sqlc.narg('provider_code')::varchar IS NULL OR provider_code = sqlc.narg('provider_code')::varchar)
ORDER BY created_at ASC
LIMIT sqlc.arg('limit_count')::int;

-- name: ListCODSettlementsForReconciliation :many
SELECT id, organization_id, shipment_id, provider_code, expected_amount_kurus,
       actual_amount_kurus, carrier_settlement_id, carrier_settlement_date, status,
       ledger_entry_id, discrepancy_kurus, discrepancy_reason, settled_at,
       created_at, updated_at
FROM cod_settlements
WHERE status IN ('pending', 'collected')
  AND provider_code = $1
  AND carrier_settlement_date = $2
ORDER BY created_at ASC;

-- name: CreateCODSettlement :one
INSERT INTO cod_settlements (
    organization_id,
    shipment_id,
    provider_code,
    expected_amount_kurus,
    status
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING id, organization_id, shipment_id, provider_code, expected_amount_kurus,
          actual_amount_kurus, carrier_settlement_id, carrier_settlement_date, status,
          ledger_entry_id, discrepancy_kurus, discrepancy_reason, settled_at,
          created_at, updated_at;

-- name: UpdateCODSettlementCollected :one
UPDATE cod_settlements
SET
    status = 'collected',
    actual_amount_kurus = $2,
    carrier_settlement_id = $3,
    carrier_settlement_date = $4,
    discrepancy_kurus = $2 - expected_amount_kurus,
    updated_at = NOW()
WHERE id = $1 AND status = 'pending'
RETURNING id, organization_id, shipment_id, provider_code, expected_amount_kurus,
          actual_amount_kurus, carrier_settlement_id, carrier_settlement_date, status,
          ledger_entry_id, discrepancy_kurus, discrepancy_reason, settled_at,
          created_at, updated_at;

-- name: UpdateCODSettlementSettled :one
UPDATE cod_settlements
SET
    status = 'settled',
    ledger_entry_id = $2,
    settled_at = NOW(),
    updated_at = NOW()
WHERE id = $1 AND status = 'collected'
RETURNING id, organization_id, shipment_id, provider_code, expected_amount_kurus,
          actual_amount_kurus, carrier_settlement_id, carrier_settlement_date, status,
          ledger_entry_id, discrepancy_kurus, discrepancy_reason, settled_at,
          created_at, updated_at;

-- name: UpdateCODSettlementDisputed :one
UPDATE cod_settlements
SET
    status = 'disputed',
    discrepancy_reason = $2,
    updated_at = NOW()
WHERE id = $1 AND status IN ('pending', 'collected')
RETURNING id, organization_id, shipment_id, provider_code, expected_amount_kurus,
          actual_amount_kurus, carrier_settlement_id, carrier_settlement_date, status,
          ledger_entry_id, discrepancy_kurus, discrepancy_reason, settled_at,
          created_at, updated_at;

-- name: UpdateCODSettlementWrittenOff :one
UPDATE cod_settlements
SET
    status = 'written_off',
    discrepancy_reason = $2,
    updated_at = NOW()
WHERE id = $1 AND status = 'disputed'
RETURNING id, organization_id, shipment_id, provider_code, expected_amount_kurus,
          actual_amount_kurus, carrier_settlement_id, carrier_settlement_date, status,
          ledger_entry_id, discrepancy_kurus, discrepancy_reason, settled_at,
          created_at, updated_at;

-- name: GetCODSettlementSummaryByOrganization :one
SELECT 
    COUNT(*) as total_settlements,
    COALESCE(SUM(expected_amount_kurus), 0) as total_expected_kurus,
    COALESCE(SUM(CASE WHEN status = 'settled' THEN actual_amount_kurus ELSE 0 END), 0) as total_settled_kurus,
    COALESCE(SUM(CASE WHEN status = 'pending' THEN expected_amount_kurus ELSE 0 END), 0) as total_pending_kurus,
    COALESCE(SUM(CASE WHEN status = 'disputed' THEN expected_amount_kurus ELSE 0 END), 0) as total_disputed_kurus
FROM cod_settlements
WHERE organization_id = $1;

-- name: GetCODSettlementSummaryByProvider :many
SELECT 
    provider_code,
    COUNT(*) as total_settlements,
    COALESCE(SUM(expected_amount_kurus), 0) as total_expected_kurus,
    COALESCE(SUM(CASE WHEN status = 'settled' THEN actual_amount_kurus ELSE 0 END), 0) as total_settled_kurus,
    COALESCE(SUM(CASE WHEN status = 'pending' THEN expected_amount_kurus ELSE 0 END), 0) as total_pending_kurus
FROM cod_settlements
WHERE organization_id = $1
GROUP BY provider_code
ORDER BY provider_code;

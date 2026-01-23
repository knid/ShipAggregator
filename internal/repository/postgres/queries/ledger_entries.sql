-- Ledger Entries queries (INSERT and READ only - immutable table)
-- Table: ledger_entries

-- name: GetLedgerEntryByID :one
SELECT id, organization_id, entry_type, amount_kurus, currency, balance_after_kurus,
       reference_type, reference_id, description, metadata, actor_id, actor_type, created_at
FROM ledger_entries
WHERE id = $1;

-- name: GetLedgerEntryByReference :one
SELECT id, organization_id, entry_type, amount_kurus, currency, balance_after_kurus,
       reference_type, reference_id, description, metadata, actor_id, actor_type, created_at
FROM ledger_entries
WHERE reference_type = $1 AND reference_id = $2
LIMIT 1;

-- name: ListLedgerEntriesByOrganization :many
SELECT id, organization_id, entry_type, amount_kurus, currency, balance_after_kurus,
       reference_type, reference_id, description, metadata, actor_id, actor_type, created_at
FROM ledger_entries
WHERE organization_id = $1
  AND (sqlc.narg('entry_type')::varchar IS NULL OR entry_type = sqlc.narg('entry_type')::varchar)
  AND (sqlc.narg('reference_type')::varchar IS NULL OR reference_type = sqlc.narg('reference_type')::varchar)
  AND (sqlc.narg('created_after')::timestamptz IS NULL OR created_at >= sqlc.narg('created_after')::timestamptz)
  AND (sqlc.narg('created_before')::timestamptz IS NULL OR created_at <= sqlc.narg('created_before')::timestamptz)
ORDER BY created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: CountLedgerEntriesByOrganization :one
SELECT COUNT(*) as total
FROM ledger_entries
WHERE organization_id = $1
  AND (sqlc.narg('entry_type')::varchar IS NULL OR entry_type = sqlc.narg('entry_type')::varchar)
  AND (sqlc.narg('reference_type')::varchar IS NULL OR reference_type = sqlc.narg('reference_type')::varchar)
  AND (sqlc.narg('created_after')::timestamptz IS NULL OR created_at >= sqlc.narg('created_after')::timestamptz)
  AND (sqlc.narg('created_before')::timestamptz IS NULL OR created_at <= sqlc.narg('created_before')::timestamptz);

-- name: GetWalletBalance :one
SELECT COALESCE(
    (SELECT balance_after_kurus FROM ledger_entries 
     WHERE organization_id = $1 
     ORDER BY created_at DESC 
     LIMIT 1),
    0
) as balance_kurus;

-- name: GetWalletBalanceWithLock :one
SELECT COALESCE(
    (SELECT balance_after_kurus FROM ledger_entries 
     WHERE organization_id = $1 
     ORDER BY created_at DESC 
     LIMIT 1
     FOR UPDATE),
    0
) as balance_kurus;

-- name: GetLatestLedgerEntry :one
SELECT id, organization_id, entry_type, amount_kurus, currency, balance_after_kurus,
       reference_type, reference_id, description, metadata, actor_id, actor_type, created_at
FROM ledger_entries
WHERE organization_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: CreateLedgerEntry :one
INSERT INTO ledger_entries (
    organization_id,
    entry_type,
    amount_kurus,
    currency,
    balance_after_kurus,
    reference_type,
    reference_id,
    description,
    metadata,
    actor_id,
    actor_type
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING id, organization_id, entry_type, amount_kurus, currency, balance_after_kurus,
          reference_type, reference_id, description, metadata, actor_id, actor_type, created_at;

-- name: GetLedgerSummaryByOrganization :one
SELECT 
    COALESCE(SUM(CASE WHEN entry_type = 'credit' THEN amount_kurus ELSE 0 END), 0) as total_credits_kurus,
    COALESCE(SUM(CASE WHEN entry_type = 'debit' THEN amount_kurus ELSE 0 END), 0) as total_debits_kurus,
    COUNT(*) as total_entries
FROM ledger_entries
WHERE organization_id = $1;

-- name: GetLedgerSummaryByDateRange :one
SELECT 
    COALESCE(SUM(CASE WHEN entry_type = 'credit' THEN amount_kurus ELSE 0 END), 0) as total_credits_kurus,
    COALESCE(SUM(CASE WHEN entry_type = 'debit' THEN amount_kurus ELSE 0 END), 0) as total_debits_kurus,
    COUNT(*) as total_entries
FROM ledger_entries
WHERE organization_id = $1
  AND created_at >= $2
  AND created_at <= $3;

-- name: ListLedgerEntriesByReferenceType :many
SELECT id, organization_id, entry_type, amount_kurus, currency, balance_after_kurus,
       reference_type, reference_id, description, metadata, actor_id, actor_type, created_at
FROM ledger_entries
WHERE organization_id = $1 AND reference_type = $2
ORDER BY created_at DESC
LIMIT sqlc.arg('limit_count')::int
OFFSET sqlc.arg('offset_count')::int;

-- name: CheckLedgerInvariant :one
SELECT
    COALESCE(SUM(CASE WHEN le.entry_type = 'credit' THEN le.amount_kurus ELSE 0 END), 0) as total_credits,
    COALESCE(SUM(CASE WHEN le.entry_type = 'debit' THEN le.amount_kurus ELSE 0 END), 0) as total_debits,
    COALESCE(SUM(CASE WHEN le.entry_type = 'credit' THEN le.amount_kurus ELSE 0 END), 0) -
    COALESCE(SUM(CASE WHEN le.entry_type = 'debit' THEN le.amount_kurus ELSE 0 END), 0) as computed_balance,
    (SELECT le2.balance_after_kurus FROM ledger_entries le2 WHERE le2.organization_id = $1 ORDER BY le2.created_at DESC LIMIT 1) as stored_balance
FROM ledger_entries le
WHERE le.organization_id = $1;

-- name: ListAllOrganizationBalances :many
SELECT
    organization_id,
    (SELECT balance_after_kurus FROM ledger_entries le2
     WHERE le2.organization_id = le.organization_id
     ORDER BY created_at DESC LIMIT 1) as balance_kurus,
    MAX(created_at) as last_transaction_at
FROM ledger_entries le
GROUP BY organization_id
ORDER BY balance_kurus DESC;

-- name: GetLedgerTotalsByDateRange :one
SELECT
    COALESCE(SUM(CASE WHEN entry_type = 'credit' THEN amount_kurus ELSE 0 END), 0) as total_credits_kurus,
    COALESCE(SUM(CASE WHEN entry_type = 'debit' THEN amount_kurus ELSE 0 END), 0) as total_debits_kurus,
    COUNT(*) as total_entries
FROM ledger_entries
WHERE created_at >= $1 AND created_at < $2;

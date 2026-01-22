-- Migration: 000010_infrastructure (rollback)
-- Description: Drop idempotency_records, audit_logs, provider_api_logs, and outbox tables
-- Created: 2026-01-22

-- Drop tables (no foreign key dependencies between these tables)
DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS provider_api_logs;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS idempotency_records;

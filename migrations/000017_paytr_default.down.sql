-- Revert default payment_provider to 'iyzico'
ALTER TABLE topups ALTER COLUMN payment_provider SET DEFAULT 'iyzico';

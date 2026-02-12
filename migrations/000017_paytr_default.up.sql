-- Change default payment_provider from 'iyzico' to 'paytr'
ALTER TABLE topups ALTER COLUMN payment_provider SET DEFAULT 'paytr';

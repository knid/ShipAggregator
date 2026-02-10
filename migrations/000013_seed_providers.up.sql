-- Migration: 000013_seed_providers
-- Description: Seed provider data (aras, yurtici)
-- Created: 2026-01-22

-- ============================================================================
-- PROVIDERS SEED DATA
-- Insert carrier providers with their capabilities
-- ============================================================================

-- Aras Kargo (SOAP)
INSERT INTO providers (code, name, logo_url, website, support_email, support_phone, api_type, api_base_url, api_sandbox_url, is_active, capabilities)
VALUES (
    'aras',
    'Aras Kargo',
    '/assets/providers/aras.png',
    'https://www.araskargo.com.tr',
    'destek@araskargo.com.tr',
    '444 25 52',
    'soap',
    'https://customerws.araskargo.com.tr/aaborsa/ArasBorsaWS.asmx',
    'https://customerws-test.araskargo.com.tr/aaborsa/ArasBorsaWS.asmx',
    TRUE,
    '{"cod": true, "insurance": true, "returns": true, "pickup": true, "tracking_webhook": true, "tracking_poll": true, "pod_image": true, "max_cod_amount_kurus": 5000000}'
);

-- Yurtiçi Kargo (SOAP)
INSERT INTO providers (code, name, logo_url, website, support_email, support_phone, api_type, api_base_url, api_sandbox_url, is_active, capabilities)
VALUES (
    'yurtici',
    'Yurtiçi Kargo',
    '/assets/providers/yurtici.png',
    'https://www.yurticikargo.com',
    'destek@yurticikargo.com',
    '444 99 99',
    'soap',
    'https://webservices.yurticikargo.com/KOPSWebServices/ShippingOrderDispatcherServices',
    'https://testwebservices.yurticikargo.com/KOPSWebServices/ShippingOrderDispatcherServices',
    TRUE,
    '{"cod": true, "insurance": true, "returns": true, "pickup": true, "tracking_webhook": true, "tracking_poll": true, "pod_image": true, "max_cod_amount_kurus": 3000000}'
);

-- ============================================================================
-- PROVIDER_SERVICES SEED DATA
-- Insert service levels for each provider
-- ============================================================================

-- Aras Kargo Services
INSERT INTO provider_services (provider_code, service_code, name, description, estimated_days_min, estimated_days_max, supports_cod, supports_insurance, supports_return, max_weight_grams, max_length_cm, max_width_cm, max_height_cm, label_formats, is_active)
VALUES
    ('aras', 'standard', 'Standart Kargo', 'Standart teslimat süresi', 2, 4, TRUE, TRUE, TRUE, 30000, 150, 100, 100, '["pdf", "zpl"]', TRUE),
    ('aras', 'express', 'Hızlı Kargo', 'Hızlı teslimat', 1, 2, TRUE, TRUE, TRUE, 30000, 150, 100, 100, '["pdf", "zpl"]', TRUE);

-- Yurtiçi Kargo Services
INSERT INTO provider_services (provider_code, service_code, name, description, estimated_days_min, estimated_days_max, supports_cod, supports_insurance, supports_return, max_weight_grams, max_length_cm, max_width_cm, max_height_cm, label_formats, is_active)
VALUES
    ('yurtici', 'standard', 'Standart', 'Standart teslimat', 2, 4, TRUE, TRUE, TRUE, 30000, 120, 80, 80, '["pdf", "zpl", "png"]', TRUE),
    ('yurtici', 'express', 'Express', 'Hızlı teslimat', 1, 2, TRUE, TRUE, TRUE, 30000, 120, 80, 80, '["pdf", "zpl", "png"]', TRUE),
    ('yurtici', 'economy', 'Ekonomik', 'Ekonomik teslimat', 3, 5, TRUE, FALSE, TRUE, 30000, 120, 80, 80, '["pdf", "zpl", "png"]', TRUE);

-- ============================================================================
-- MARKETPLACE_RATE_PLANS SEED DATA
-- Insert initial marketplace rate plans (placeholder rates - to be updated)
-- ============================================================================

-- Aras Kargo Rate Plans
INSERT INTO marketplace_rate_plans (provider_code, service_code, name, base_price_kurus, price_per_kg_kurus, price_per_desi_kurus, fuel_surcharge_percent, cod_fee_percent, cod_fee_min_kurus, vat_percent, effective_from, is_active)
VALUES
    ('aras', 'standard', 'Aras Standart', 2500, 100, 50, 5.00, 2.00, 500, 20.00, '2026-01-01', TRUE),
    ('aras', 'express', 'Aras Hızlı', 4000, 150, 75, 5.00, 2.00, 500, 20.00, '2026-01-01', TRUE);

-- Yurtiçi Kargo Rate Plans
INSERT INTO marketplace_rate_plans (provider_code, service_code, name, base_price_kurus, price_per_kg_kurus, price_per_desi_kurus, fuel_surcharge_percent, cod_fee_percent, cod_fee_min_kurus, vat_percent, effective_from, is_active)
VALUES
    ('yurtici', 'standard', 'Yurtiçi Standart', 2600, 110, 55, 5.00, 2.00, 500, 20.00, '2026-01-01', TRUE),
    ('yurtici', 'express', 'Yurtiçi Express', 4200, 160, 80, 5.00, 2.00, 500, 20.00, '2026-01-01', TRUE),
    ('yurtici', 'economy', 'Yurtiçi Ekonomik', 2000, 80, 40, 5.00, 2.00, 500, 20.00, '2026-01-01', TRUE);

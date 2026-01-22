-- Migration: 000013_seed_providers
-- Description: Seed provider data (aras, yurtici, surat, ptt, hepsijet, mng, kolaygelsin, sendeo, pakettaxi)
-- Created: 2026-01-22

-- ============================================================================
-- PROVIDERS SEED DATA
-- Insert all 9 carrier providers with their capabilities
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

-- Sürat Kargo (SOAP)
INSERT INTO providers (code, name, logo_url, website, support_email, support_phone, api_type, api_base_url, api_sandbox_url, is_active, capabilities)
VALUES (
    'surat',
    'Sürat Kargo',
    '/assets/providers/surat.png',
    'https://www.suratkargo.com.tr',
    'destek@suratkargo.com.tr',
    '444 47 87',
    'soap',
    'https://www.suratkargo.com.tr/webservice/SuratKargoWS.asmx',
    'https://test.suratkargo.com.tr/webservice/SuratKargoWS.asmx',
    TRUE,
    '{"cod": true, "insurance": true, "returns": true, "pickup": true, "tracking_webhook": false, "tracking_poll": true, "pod_image": false, "max_cod_amount_kurus": 2500000}'
);

-- PTT Kargo (SOAP)
INSERT INTO providers (code, name, logo_url, website, support_email, support_phone, api_type, api_base_url, api_sandbox_url, is_active, capabilities)
VALUES (
    'ptt',
    'PTT Kargo',
    '/assets/providers/ptt.png',
    'https://www.ptt.gov.tr',
    'destek@ptt.gov.tr',
    '444 17 88',
    'soap',
    'https://pttws.ptt.gov.tr/PttKargoWS/PttKargoWS.asmx',
    'https://pttws-test.ptt.gov.tr/PttKargoWS/PttKargoWS.asmx',
    TRUE,
    '{"cod": true, "insurance": false, "returns": true, "pickup": false, "tracking_webhook": false, "tracking_poll": true, "pod_image": false, "max_cod_amount_kurus": 1000000}'
);

-- Hepsijet (REST)
INSERT INTO providers (code, name, logo_url, website, support_email, support_phone, api_type, api_base_url, api_sandbox_url, is_active, capabilities)
VALUES (
    'hepsijet',
    'Hepsijet',
    '/assets/providers/hepsijet.png',
    'https://www.hepsijet.com',
    'destek@hepsijet.com',
    '850 222 44 88',
    'rest',
    'https://api.hepsijet.com/v1',
    'https://api-sandbox.hepsijet.com/v1',
    TRUE,
    '{"cod": true, "insurance": false, "returns": true, "pickup": true, "tracking_webhook": true, "tracking_poll": true, "pod_image": true, "same_day": true, "max_cod_amount_kurus": 2000000}'
);

-- MNG Kargo (SOAP)
INSERT INTO providers (code, name, logo_url, website, support_email, support_phone, api_type, api_base_url, api_sandbox_url, is_active, capabilities)
VALUES (
    'mng',
    'MNG Kargo',
    '/assets/providers/mng.png',
    'https://www.mngkargo.com.tr',
    'destek@mngkargo.com.tr',
    '444 06 64',
    'soap',
    'https://service.mngkargo.com.tr/maborsa/MngKargoWS.asmx',
    'https://service-test.mngkargo.com.tr/maborsa/MngKargoWS.asmx',
    TRUE,
    '{"cod": true, "insurance": true, "returns": true, "pickup": true, "tracking_webhook": true, "tracking_poll": true, "pod_image": true, "max_cod_amount_kurus": 3000000}'
);

-- Kolaygelsin (REST)
INSERT INTO providers (code, name, logo_url, website, support_email, support_phone, api_type, api_base_url, api_sandbox_url, is_active, capabilities)
VALUES (
    'kolaygelsin',
    'Kolaygelsin',
    '/assets/providers/kolaygelsin.png',
    'https://www.kolaygelsin.com',
    'destek@kolaygelsin.com',
    '850 255 55 55',
    'rest',
    'https://api.kolaygelsin.com/v1',
    'https://api-sandbox.kolaygelsin.com/v1',
    TRUE,
    '{"cod": true, "insurance": false, "returns": true, "pickup": true, "tracking_webhook": true, "tracking_poll": true, "pod_image": false, "max_cod_amount_kurus": 1500000}'
);

-- Sendeo (REST)
INSERT INTO providers (code, name, logo_url, website, support_email, support_phone, api_type, api_base_url, api_sandbox_url, is_active, capabilities)
VALUES (
    'sendeo',
    'Sendeo',
    '/assets/providers/sendeo.png',
    'https://www.sendeo.com.tr',
    'destek@sendeo.com.tr',
    '850 255 73 63',
    'rest',
    'https://api.sendeo.com.tr/v1',
    'https://api-sandbox.sendeo.com.tr/v1',
    TRUE,
    '{"cod": true, "insurance": false, "returns": true, "pickup": true, "tracking_webhook": true, "tracking_poll": true, "pod_image": true, "max_cod_amount_kurus": 2000000}'
);

-- Pakettaxi (REST)
INSERT INTO providers (code, name, logo_url, website, support_email, support_phone, api_type, api_base_url, api_sandbox_url, is_active, capabilities)
VALUES (
    'pakettaxi',
    'Pakettaxi',
    '/assets/providers/pakettaxi.png',
    'https://www.pakettaxi.com',
    'destek@pakettaxi.com',
    '850 255 72 89',
    'rest',
    'https://api.pakettaxi.com/v1',
    'https://api-sandbox.pakettaxi.com/v1',
    TRUE,
    '{"cod": true, "insurance": false, "returns": false, "pickup": true, "tracking_webhook": true, "tracking_poll": true, "pod_image": false, "same_day": true, "max_cod_amount_kurus": 1000000}'
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

-- Sürat Kargo Services
INSERT INTO provider_services (provider_code, service_code, name, description, estimated_days_min, estimated_days_max, supports_cod, supports_insurance, supports_return, max_weight_grams, max_length_cm, max_width_cm, max_height_cm, label_formats, is_active)
VALUES
    ('surat', 'standard', 'Standart', 'Standart teslimat', 2, 4, TRUE, TRUE, TRUE, 30000, 150, 100, 100, '["pdf"]', TRUE),
    ('surat', 'express', 'Hızlı', 'Hızlı teslimat', 1, 2, TRUE, TRUE, TRUE, 30000, 150, 100, 100, '["pdf"]', TRUE);

-- PTT Kargo Services
INSERT INTO provider_services (provider_code, service_code, name, description, estimated_days_min, estimated_days_max, supports_cod, supports_insurance, supports_return, max_weight_grams, max_length_cm, max_width_cm, max_height_cm, label_formats, is_active)
VALUES
    ('ptt', 'standard', 'Normal Kargo', 'Normal teslimat', 3, 5, TRUE, FALSE, TRUE, 30000, 100, 80, 80, '["pdf"]', TRUE),
    ('ptt', 'registered', 'Taahhütlü', 'Taahhütlü kargo', 2, 4, TRUE, FALSE, TRUE, 30000, 100, 80, 80, '["pdf"]', TRUE);

-- Hepsijet Services
INSERT INTO provider_services (provider_code, service_code, name, description, estimated_days_min, estimated_days_max, supports_cod, supports_insurance, supports_return, max_weight_grams, max_length_cm, max_width_cm, max_height_cm, label_formats, is_active)
VALUES
    ('hepsijet', 'standard', 'Standart', 'Standart teslimat', 2, 3, TRUE, FALSE, TRUE, 30000, 100, 60, 60, '["pdf", "png"]', TRUE),
    ('hepsijet', 'express', 'Hızlı', 'Hızlı teslimat', 1, 2, TRUE, FALSE, TRUE, 30000, 100, 60, 60, '["pdf", "png"]', TRUE),
    ('hepsijet', 'same_day', 'Aynı Gün', 'Aynı gün teslimat (sadece büyük şehirler)', 0, 1, FALSE, FALSE, TRUE, 30000, 100, 60, 60, '["pdf", "png"]', TRUE);

-- MNG Kargo Services
INSERT INTO provider_services (provider_code, service_code, name, description, estimated_days_min, estimated_days_max, supports_cod, supports_insurance, supports_return, max_weight_grams, max_length_cm, max_width_cm, max_height_cm, label_formats, is_active)
VALUES
    ('mng', 'standard', 'Standart', 'Standart teslimat', 2, 4, TRUE, TRUE, TRUE, 30000, 150, 100, 100, '["pdf", "zpl"]', TRUE),
    ('mng', 'express', 'Express', 'Hızlı teslimat', 1, 2, TRUE, TRUE, TRUE, 30000, 150, 100, 100, '["pdf", "zpl"]', TRUE);

-- Kolaygelsin Services
INSERT INTO provider_services (provider_code, service_code, name, description, estimated_days_min, estimated_days_max, supports_cod, supports_insurance, supports_return, max_weight_grams, max_length_cm, max_width_cm, max_height_cm, label_formats, is_active)
VALUES
    ('kolaygelsin', 'standard', 'Standart', 'Standart teslimat', 2, 4, TRUE, FALSE, TRUE, 30000, 100, 80, 80, '["pdf"]', TRUE),
    ('kolaygelsin', 'express', 'Hızlı', 'Hızlı teslimat', 1, 2, TRUE, FALSE, TRUE, 30000, 100, 80, 80, '["pdf"]', TRUE);

-- Sendeo Services
INSERT INTO provider_services (provider_code, service_code, name, description, estimated_days_min, estimated_days_max, supports_cod, supports_insurance, supports_return, max_weight_grams, max_length_cm, max_width_cm, max_height_cm, label_formats, is_active)
VALUES
    ('sendeo', 'standard', 'Standart', 'Standart teslimat', 2, 4, TRUE, FALSE, TRUE, 30000, 120, 80, 80, '["pdf", "png"]', TRUE),
    ('sendeo', 'express', 'Express', 'Hızlı teslimat', 1, 2, TRUE, FALSE, TRUE, 30000, 120, 80, 80, '["pdf", "png"]', TRUE);

-- Pakettaxi Services
INSERT INTO provider_services (provider_code, service_code, name, description, estimated_days_min, estimated_days_max, supports_cod, supports_insurance, supports_return, max_weight_grams, max_length_cm, max_width_cm, max_height_cm, label_formats, is_active)
VALUES
    ('pakettaxi', 'standard', 'Standart', 'Standart teslimat', 1, 3, TRUE, FALSE, FALSE, 20000, 80, 60, 60, '["pdf"]', TRUE),
    ('pakettaxi', 'same_day', 'Aynı Gün', 'Aynı gün teslimat (sadece İstanbul)', 0, 1, TRUE, FALSE, FALSE, 20000, 80, 60, 60, '["pdf"]', TRUE);

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

-- Sürat Kargo Rate Plans
INSERT INTO marketplace_rate_plans (provider_code, service_code, name, base_price_kurus, price_per_kg_kurus, price_per_desi_kurus, fuel_surcharge_percent, cod_fee_percent, cod_fee_min_kurus, vat_percent, effective_from, is_active)
VALUES
    ('surat', 'standard', 'Sürat Standart', 2400, 95, 48, 5.00, 2.00, 500, 20.00, '2026-01-01', TRUE),
    ('surat', 'express', 'Sürat Hızlı', 3800, 140, 70, 5.00, 2.00, 500, 20.00, '2026-01-01', TRUE);

-- PTT Kargo Rate Plans
INSERT INTO marketplace_rate_plans (provider_code, service_code, name, base_price_kurus, price_per_kg_kurus, price_per_desi_kurus, fuel_surcharge_percent, cod_fee_percent, cod_fee_min_kurus, vat_percent, effective_from, is_active)
VALUES
    ('ptt', 'standard', 'PTT Normal', 1800, 70, 35, 0.00, 2.50, 400, 20.00, '2026-01-01', TRUE),
    ('ptt', 'registered', 'PTT Taahhütlü', 2200, 85, 42, 0.00, 2.50, 400, 20.00, '2026-01-01', TRUE);

-- Hepsijet Rate Plans
INSERT INTO marketplace_rate_plans (provider_code, service_code, name, base_price_kurus, price_per_kg_kurus, price_per_desi_kurus, fuel_surcharge_percent, cod_fee_percent, cod_fee_min_kurus, vat_percent, effective_from, is_active)
VALUES
    ('hepsijet', 'standard', 'Hepsijet Standart', 2700, 105, 52, 5.00, 2.00, 500, 20.00, '2026-01-01', TRUE),
    ('hepsijet', 'express', 'Hepsijet Hızlı', 4500, 170, 85, 5.00, 2.00, 500, 20.00, '2026-01-01', TRUE),
    ('hepsijet', 'same_day', 'Hepsijet Aynı Gün', 8000, 250, 125, 5.00, 0.00, 0, 20.00, '2026-01-01', TRUE);

-- MNG Kargo Rate Plans
INSERT INTO marketplace_rate_plans (provider_code, service_code, name, base_price_kurus, price_per_kg_kurus, price_per_desi_kurus, fuel_surcharge_percent, cod_fee_percent, cod_fee_min_kurus, vat_percent, effective_from, is_active)
VALUES
    ('mng', 'standard', 'MNG Standart', 2550, 100, 50, 5.00, 2.00, 500, 20.00, '2026-01-01', TRUE),
    ('mng', 'express', 'MNG Express', 4100, 155, 78, 5.00, 2.00, 500, 20.00, '2026-01-01', TRUE);

-- Kolaygelsin Rate Plans
INSERT INTO marketplace_rate_plans (provider_code, service_code, name, base_price_kurus, price_per_kg_kurus, price_per_desi_kurus, fuel_surcharge_percent, cod_fee_percent, cod_fee_min_kurus, vat_percent, effective_from, is_active)
VALUES
    ('kolaygelsin', 'standard', 'Kolaygelsin Standart', 2300, 90, 45, 5.00, 2.00, 500, 20.00, '2026-01-01', TRUE),
    ('kolaygelsin', 'express', 'Kolaygelsin Hızlı', 3700, 135, 68, 5.00, 2.00, 500, 20.00, '2026-01-01', TRUE);

-- Sendeo Rate Plans
INSERT INTO marketplace_rate_plans (provider_code, service_code, name, base_price_kurus, price_per_kg_kurus, price_per_desi_kurus, fuel_surcharge_percent, cod_fee_percent, cod_fee_min_kurus, vat_percent, effective_from, is_active)
VALUES
    ('sendeo', 'standard', 'Sendeo Standart', 2350, 92, 46, 5.00, 2.00, 500, 20.00, '2026-01-01', TRUE),
    ('sendeo', 'express', 'Sendeo Express', 3750, 138, 69, 5.00, 2.00, 500, 20.00, '2026-01-01', TRUE);

-- Pakettaxi Rate Plans
INSERT INTO marketplace_rate_plans (provider_code, service_code, name, base_price_kurus, price_per_kg_kurus, price_per_desi_kurus, fuel_surcharge_percent, cod_fee_percent, cod_fee_min_kurus, vat_percent, effective_from, is_active)
VALUES
    ('pakettaxi', 'standard', 'Pakettaxi Standart', 2800, 120, 60, 5.00, 2.00, 500, 20.00, '2026-01-01', TRUE),
    ('pakettaxi', 'same_day', 'Pakettaxi Aynı Gün', 6500, 200, 100, 5.00, 2.00, 500, 20.00, '2026-01-01', TRUE);

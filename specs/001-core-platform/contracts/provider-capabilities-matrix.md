# Provider Capabilities Matrix

**Feature Branch**: `001-core-platform`  
**Created**: 2026-01-22  
**Status**: Draft  
**Source**: Derived from `vendor_docs/index.md`

---

## Overview

This document defines the capabilities, constraints, and service levels for each integrated carrier provider. Data is derived from vendor documentation and must be kept in sync with actual provider capabilities.

---

## Provider Summary

| Provider | API Type | COD | Insurance | Returns | Pickup | Tracking Webhook | Tracking Poll |
|----------|----------|-----|-----------|---------|--------|------------------|---------------|
| Aras | SOAP | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Yurtiçi | SOAP | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Sürat | SOAP | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ |
| PTT | SOAP | ✅ | ❌ | ✅ | ❌ | ❌ | ✅ |
| Hepsijet | REST | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ |
| MNG | SOAP | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Kolaygelsin | REST | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ |
| Sendeo | REST | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ |
| Pakettaxi | REST | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ |

---

## Detailed Provider Specifications

### Aras Kargo

| Property | Value |
|----------|-------|
| **Provider Code** | `aras` |
| **API Type** | SOAP |
| **WSDL URL** | `https://customerws.araskargo.com.tr/aaborsa/ArasBorsaWS.asmx?WSDL` |
| **Documentation** | `vendor_docs/aras/` |

#### Services

| Service Code | Name | Est. Days | COD | Insurance |
|--------------|------|-----------|-----|-----------|
| `standard` | Standart Kargo | 2-4 | ✅ | ✅ |
| `express` | Hızlı Kargo | 1-2 | ✅ | ✅ |

#### Constraints

| Constraint | Value |
|------------|-------|
| Max Weight | 30 kg |
| Max Length | 150 cm |
| Max Width | 100 cm |
| Max Height | 100 cm |
| Max Desi | 50 |
| Max COD Amount | 50,000 TRY |
| Label Formats | PDF, ZPL |

#### Credential Fields

| Field | Required | Description |
|-------|----------|-------------|
| `username` | Yes | API username |
| `password` | Yes | API password |
| `customerCode` | Yes | Customer code (müşteri kodu) |

#### Status Mapping

| Aras Status | Normalized Status |
|-------------|-------------------|
| `KARGO_KABUL` | `picked_up` |
| `TRANSFER_MERKEZ` | `in_transit` |
| `DAGITIM_SUBESINDE` | `at_hub` |
| `DAGITIMDA` | `out_for_delivery` |
| `TESLIM_EDILDI` | `delivered` |
| `TESLIM_EDILEMEDI` | `delivery_failed` |
| `IADE` | `returning` |
| `IADE_TESLIM` | `returned_to_sender` |
| `IPTAL` | `cancelled` |

---

### Yurtiçi Kargo

| Property | Value |
|----------|-------|
| **Provider Code** | `yurtici` |
| **API Type** | SOAP |
| **WSDL URL** | `https://webservices.yurticikargo.com/KOPSWebServices/ShippingOrderDispatcherServices?wsdl` |
| **Documentation** | `vendor_docs/yurtici/` |

#### Services

| Service Code | Name | Est. Days | COD | Insurance |
|--------------|------|-----------|-----|-----------|
| `standard` | Standart | 2-4 | ✅ | ✅ |
| `express` | Express | 1-2 | ✅ | ✅ |
| `economy` | Ekonomik | 3-5 | ✅ | ❌ |

#### Constraints

| Constraint | Value |
|------------|-------|
| Max Weight | 30 kg |
| Max Length | 120 cm |
| Max Width | 80 cm |
| Max Height | 80 cm |
| Max Desi | 30 |
| Max COD Amount | 30,000 TRY |
| Label Formats | PDF, ZPL, PNG |

#### Credential Fields

| Field | Required | Description |
|-------|----------|-------------|
| `username` | Yes | API username |
| `password` | Yes | API password |
| `userLanguage` | No | Language (default: TR) |

#### Status Mapping

| Yurtiçi Status | Normalized Status |
|----------------|-------------------|
| `Kargo Kabul` | `picked_up` |
| `Aktarma Merkezinde` | `in_transit` |
| `Dağıtım Şubesinde` | `at_hub` |
| `Dağıtımda` | `out_for_delivery` |
| `Teslim Edildi` | `delivered` |
| `Teslim Edilemedi` | `delivery_failed` |
| `İade Sürecinde` | `returning` |
| `İade Edildi` | `returned_to_sender` |
| `İptal Edildi` | `cancelled` |

---

### Sürat Kargo

| Property | Value |
|----------|-------|
| **Provider Code** | `surat` |
| **API Type** | SOAP |
| **WSDL URL** | `https://www.suratkargo.com.tr/webservice/SuratKargoWS.asmx?WSDL` |
| **Documentation** | `vendor_docs/surat/` |

#### Services

| Service Code | Name | Est. Days | COD | Insurance |
|--------------|------|-----------|-----|-----------|
| `standard` | Standart | 2-4 | ✅ | ✅ |
| `express` | Hızlı | 1-2 | ✅ | ✅ |

#### Constraints

| Constraint | Value |
|------------|-------|
| Max Weight | 30 kg |
| Max Length | 150 cm |
| Max Width | 100 cm |
| Max Height | 100 cm |
| Max Desi | 50 |
| Max COD Amount | 25,000 TRY |
| Label Formats | PDF |

#### Credential Fields

| Field | Required | Description |
|-------|----------|-------------|
| `username` | Yes | API username |
| `password` | Yes | API password |

#### Status Mapping

| Sürat Status | Normalized Status |
|--------------|-------------------|
| `KABUL` | `picked_up` |
| `TRANSFER` | `in_transit` |
| `SUBEDE` | `at_hub` |
| `DAGITIMDA` | `out_for_delivery` |
| `TESLIM` | `delivered` |
| `TESLIM_EDILEMEDI` | `delivery_failed` |
| `IADE` | `returning` |
| `IADE_TESLIM` | `returned_to_sender` |

#### Notes

- No tracking webhook support - polling required
- Pickup scheduling via separate endpoint

---

### PTT Kargo

| Property | Value |
|----------|-------|
| **Provider Code** | `ptt` |
| **API Type** | SOAP |
| **WSDL URL** | `https://pttws.ptt.gov.tr/PttKargoWS/PttKargoWS.asmx?WSDL` |
| **Documentation** | `vendor_docs/ptt/` |

#### Services

| Service Code | Name | Est. Days | COD | Insurance |
|--------------|------|-----------|-----|-----------|
| `standard` | Normal Kargo | 3-5 | ✅ | ❌ |
| `registered` | Taahhütlü | 2-4 | ✅ | ❌ |

#### Constraints

| Constraint | Value |
|------------|-------|
| Max Weight | 30 kg |
| Max Length | 100 cm |
| Max Width | 80 cm |
| Max Height | 80 cm |
| Max Desi | 30 |
| Max COD Amount | 10,000 TRY |
| Label Formats | PDF |

#### Credential Fields

| Field | Required | Description |
|-------|----------|-------------|
| `username` | Yes | API username |
| `password` | Yes | API password |
| `senderCode` | Yes | Sender code (gönderici kodu) |

#### Status Mapping

| PTT Status | Normalized Status |
|------------|-------------------|
| `KABUL_EDILDI` | `picked_up` |
| `YOLDA` | `in_transit` |
| `DAGITIM_MERKEZINDE` | `at_hub` |
| `DAGITIMA_CIKTI` | `out_for_delivery` |
| `TESLIM_EDILDI` | `delivered` |
| `TESLIM_EDILEMEDI` | `delivery_failed` |
| `IADE_EDILDI` | `returned_to_sender` |

#### Notes

- No pickup scheduling - drop-off only
- No insurance support
- No tracking webhook - polling only
- Limited to Turkey domestic only

---

### Hepsijet

| Property | Value |
|----------|-------|
| **Provider Code** | `hepsijet` |
| **API Type** | REST |
| **Base URL** | `https://api.hepsijet.com/v1` |
| **Documentation** | `vendor_docs/hepsijet/` |

#### Services

| Service Code | Name | Est. Days | COD | Insurance |
|--------------|------|-----------|-----|-----------|
| `standard` | Standart | 2-3 | ✅ | ❌ |
| `express` | Hızlı | 1-2 | ✅ | ❌ |
| `same_day` | Aynı Gün | 0-1 | ❌ | ❌ |

#### Constraints

| Constraint | Value |
|------------|-------|
| Max Weight | 30 kg |
| Max Length | 100 cm |
| Max Width | 60 cm |
| Max Height | 60 cm |
| Max Desi | 30 |
| Max COD Amount | 20,000 TRY |
| Label Formats | PDF, PNG |

#### Credential Fields

| Field | Required | Description |
|-------|----------|-------------|
| `apiKey` | Yes | API key |
| `merchantId` | Yes | Merchant ID |

#### Status Mapping

| Hepsijet Status | Normalized Status |
|-----------------|-------------------|
| `CREATED` | `created` |
| `PICKED_UP` | `picked_up` |
| `IN_TRANSIT` | `in_transit` |
| `AT_BRANCH` | `at_hub` |
| `OUT_FOR_DELIVERY` | `out_for_delivery` |
| `DELIVERED` | `delivered` |
| `DELIVERY_FAILED` | `delivery_failed` |
| `RETURNING` | `returning` |
| `RETURNED` | `returned_to_sender` |
| `CANCELLED` | `cancelled` |

#### Notes

- Same-day delivery only in major cities (Istanbul, Ankara, Izmir)
- No COD for same-day service
- REST API with JSON payloads

---

### MNG Kargo

| Property | Value |
|----------|-------|
| **Provider Code** | `mng` |
| **API Type** | SOAP |
| **WSDL URL** | `https://service.mngkargo.com.tr/maborsa/MngKargoWS.asmx?WSDL` |
| **Documentation** | `vendor_docs/mng/` |

#### Services

| Service Code | Name | Est. Days | COD | Insurance |
|--------------|------|-----------|-----|-----------|
| `standard` | Standart | 2-4 | ✅ | ✅ |
| `express` | Express | 1-2 | ✅ | ✅ |

#### Constraints

| Constraint | Value |
|------------|-------|
| Max Weight | 30 kg |
| Max Length | 150 cm |
| Max Width | 100 cm |
| Max Height | 100 cm |
| Max Desi | 50 |
| Max COD Amount | 30,000 TRY |
| Label Formats | PDF, ZPL |

#### Credential Fields

| Field | Required | Description |
|-------|----------|-------------|
| `username` | Yes | API username |
| `password` | Yes | API password |
| `customerNumber` | Yes | Customer number |

#### Status Mapping

| MNG Status | Normalized Status |
|------------|-------------------|
| `KARGO_ALINDI` | `picked_up` |
| `TRANSFER` | `in_transit` |
| `SUBEDE` | `at_hub` |
| `DAGITIMDA` | `out_for_delivery` |
| `TESLIM_EDILDI` | `delivered` |
| `TESLIM_EDILEMEDI` | `delivery_failed` |
| `IADE_SURECINDE` | `returning` |
| `IADE_EDILDI` | `returned_to_sender` |
| `IPTAL` | `cancelled` |

---

### Kolaygelsin

| Property | Value |
|----------|-------|
| **Provider Code** | `kolaygelsin` |
| **API Type** | REST |
| **Base URL** | `https://api.kolaygelsin.com/v1` |
| **Documentation** | `vendor_docs/kolaygelsin/` |

#### Services

| Service Code | Name | Est. Days | COD | Insurance |
|--------------|------|-----------|-----|-----------|
| `standard` | Standart | 2-4 | ✅ | ❌ |
| `express` | Hızlı | 1-2 | ✅ | ❌ |

#### Constraints

| Constraint | Value |
|------------|-------|
| Max Weight | 30 kg |
| Max Length | 100 cm |
| Max Width | 80 cm |
| Max Height | 80 cm |
| Max Desi | 30 |
| Max COD Amount | 15,000 TRY |
| Label Formats | PDF |

#### Credential Fields

| Field | Required | Description |
|-------|----------|-------------|
| `apiKey` | Yes | API key |

#### Status Mapping

| Kolaygelsin Status | Normalized Status |
|--------------------|-------------------|
| `created` | `created` |
| `picked_up` | `picked_up` |
| `in_transit` | `in_transit` |
| `at_branch` | `at_hub` |
| `out_for_delivery` | `out_for_delivery` |
| `delivered` | `delivered` |
| `failed` | `delivery_failed` |
| `returning` | `returning` |
| `returned` | `returned_to_sender` |

---

### Sendeo

| Property | Value |
|----------|-------|
| **Provider Code** | `sendeo` |
| **API Type** | REST |
| **Base URL** | `https://api.sendeo.com.tr/v1` |
| **Documentation** | `vendor_docs/sendeo/` |

#### Services

| Service Code | Name | Est. Days | COD | Insurance |
|--------------|------|-----------|-----|-----------|
| `standard` | Standart | 2-4 | ✅ | ❌ |
| `express` | Express | 1-2 | ✅ | ❌ |

#### Constraints

| Constraint | Value |
|------------|-------|
| Max Weight | 30 kg |
| Max Length | 120 cm |
| Max Width | 80 cm |
| Max Height | 80 cm |
| Max Desi | 30 |
| Max COD Amount | 20,000 TRY |
| Label Formats | PDF, PNG |

#### Credential Fields

| Field | Required | Description |
|-------|----------|-------------|
| `apiKey` | Yes | API key |
| `customerCode` | Yes | Customer code |

#### Status Mapping

| Sendeo Status | Normalized Status |
|---------------|-------------------|
| `CREATED` | `created` |
| `PICKED` | `picked_up` |
| `TRANSIT` | `in_transit` |
| `BRANCH` | `at_hub` |
| `DELIVERY` | `out_for_delivery` |
| `DELIVERED` | `delivered` |
| `FAILED` | `delivery_failed` |
| `RETURN` | `returning` |
| `RETURNED` | `returned_to_sender` |
| `CANCELLED` | `cancelled` |

---

### Pakettaxi

| Property | Value |
|----------|-------|
| **Provider Code** | `pakettaxi` |
| **API Type** | REST |
| **Base URL** | `https://api.pakettaxi.com/v1` |
| **Documentation** | `vendor_docs/pakettaxi/` |

#### Services

| Service Code | Name | Est. Days | COD | Insurance |
|--------------|------|-----------|-----|-----------|
| `standard` | Standart | 1-3 | ✅ | ❌ |
| `same_day` | Aynı Gün | 0-1 | ✅ | ❌ |

#### Constraints

| Constraint | Value |
|------------|-------|
| Max Weight | 20 kg |
| Max Length | 80 cm |
| Max Width | 60 cm |
| Max Height | 60 cm |
| Max Desi | 20 |
| Max COD Amount | 10,000 TRY |
| Label Formats | PDF |

#### Credential Fields

| Field | Required | Description |
|-------|----------|-------------|
| `apiKey` | Yes | API key |
| `secretKey` | Yes | Secret key for signing |

#### Status Mapping

| Pakettaxi Status | Normalized Status |
|------------------|-------------------|
| `created` | `created` |
| `picked` | `picked_up` |
| `transit` | `in_transit` |
| `delivering` | `out_for_delivery` |
| `delivered` | `delivered` |
| `failed` | `delivery_failed` |
| `cancelled` | `cancelled` |

#### Notes

- No return shipment support
- Same-day only in Istanbul
- Smaller package limits than other providers

---

## Capability Comparison Matrix

### Core Features

| Feature | Aras | Yurtiçi | Sürat | PTT | Hepsijet | MNG | Kolaygelsin | Sendeo | Pakettaxi |
|---------|------|---------|-------|-----|----------|-----|-------------|--------|-----------|
| COD | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Insurance | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ |
| Returns | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ |
| Pickup | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Same-Day | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ | ✅ |

### Tracking

| Feature | Aras | Yurtiçi | Sürat | PTT | Hepsijet | MNG | Kolaygelsin | Sendeo | Pakettaxi |
|---------|------|---------|-------|-----|----------|-----|-------------|--------|-----------|
| Webhook | ✅ | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Polling | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| POD Image | ✅ | ✅ | ❌ | ❌ | ✅ | ✅ | ❌ | ✅ | ❌ |

### Label Formats

| Format | Aras | Yurtiçi | Sürat | PTT | Hepsijet | MNG | Kolaygelsin | Sendeo | Pakettaxi |
|--------|------|---------|-------|-----|----------|-----|-------------|--------|-----------|
| PDF | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| ZPL | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ | ❌ | ❌ | ❌ |
| PNG | ❌ | ✅ | ❌ | ❌ | ✅ | ❌ | ❌ | ✅ | ❌ |

### Weight & Dimension Limits

| Provider | Max Weight | Max Length | Max Width | Max Height | Max Desi |
|----------|------------|------------|-----------|------------|----------|
| Aras | 30 kg | 150 cm | 100 cm | 100 cm | 50 |
| Yurtiçi | 30 kg | 120 cm | 80 cm | 80 cm | 30 |
| Sürat | 30 kg | 150 cm | 100 cm | 100 cm | 50 |
| PTT | 30 kg | 100 cm | 80 cm | 80 cm | 30 |
| Hepsijet | 30 kg | 100 cm | 60 cm | 60 cm | 30 |
| MNG | 30 kg | 150 cm | 100 cm | 100 cm | 50 |
| Kolaygelsin | 30 kg | 100 cm | 80 cm | 80 cm | 30 |
| Sendeo | 30 kg | 120 cm | 80 cm | 80 cm | 30 |
| Pakettaxi | 20 kg | 80 cm | 60 cm | 60 cm | 20 |

### COD Limits

| Provider | Max COD Amount |
|----------|----------------|
| Aras | 50,000 TRY |
| Yurtiçi | 30,000 TRY |
| Sürat | 25,000 TRY |
| PTT | 10,000 TRY |
| Hepsijet | 20,000 TRY |
| MNG | 30,000 TRY |
| Kolaygelsin | 15,000 TRY |
| Sendeo | 20,000 TRY |
| Pakettaxi | 10,000 TRY |

---

## Service Coverage

### Geographic Coverage

| Provider | Coverage | Notes |
|----------|----------|-------|
| Aras | All 81 provinces | Full Turkey coverage |
| Yurtiçi | All 81 provinces | Full Turkey coverage |
| Sürat | All 81 provinces | Full Turkey coverage |
| PTT | All 81 provinces | Full Turkey coverage |
| Hepsijet | 60+ provinces | Major cities, expanding |
| MNG | All 81 provinces | Full Turkey coverage |
| Kolaygelsin | 50+ provinces | Major cities |
| Sendeo | 70+ provinces | Major cities, expanding |
| Pakettaxi | Istanbul only | Same-day Istanbul |

### Same-Day Coverage

| Provider | Cities |
|----------|--------|
| Hepsijet | Istanbul, Ankara, Izmir |
| Pakettaxi | Istanbul |

---

## API Characteristics

### Rate Limits

| Provider | Rate Limit | Notes |
|----------|------------|-------|
| Aras | 100/min | Per account |
| Yurtiçi | 60/min | Per account |
| Sürat | 50/min | Per account |
| PTT | 30/min | Per account |
| Hepsijet | 100/min | Per API key |
| MNG | 100/min | Per account |
| Kolaygelsin | 60/min | Per API key |
| Sendeo | 100/min | Per API key |
| Pakettaxi | 50/min | Per API key |

### Timeout Recommendations

| Provider | Connect | Read | Notes |
|----------|---------|------|-------|
| Aras | 5s | 30s | SOAP can be slow |
| Yurtiçi | 5s | 30s | SOAP can be slow |
| Sürat | 5s | 30s | SOAP can be slow |
| PTT | 10s | 60s | Government API, slower |
| Hepsijet | 5s | 15s | REST, faster |
| MNG | 5s | 30s | SOAP can be slow |
| Kolaygelsin | 5s | 15s | REST, faster |
| Sendeo | 5s | 15s | REST, faster |
| Pakettaxi | 5s | 15s | REST, faster |

---

## Webhook Configuration

### Webhook URLs (Inbound)

| Provider | Webhook Endpoint |
|----------|------------------|
| Aras | `POST /integrations/aras/tracking-webhook` |
| Yurtiçi | `POST /integrations/yurtici/tracking-webhook` |
| MNG | `POST /integrations/mng/tracking-webhook` |
| Hepsijet | `POST /integrations/hepsijet/tracking-webhook` |
| Kolaygelsin | `POST /integrations/kolaygelsin/tracking-webhook` |
| Sendeo | `POST /integrations/sendeo/tracking-webhook` |
| Pakettaxi | `POST /integrations/pakettaxi/tracking-webhook` |

### Webhook Authentication

| Provider | Auth Method |
|----------|-------------|
| Aras | IP whitelist + signature |
| Yurtiçi | IP whitelist |
| MNG | Signature header |
| Hepsijet | HMAC signature |
| Kolaygelsin | API key header |
| Sendeo | HMAC signature |
| Pakettaxi | HMAC signature |

---

## Maintenance Windows

| Provider | Typical Maintenance | Notes |
|----------|---------------------|-------|
| Aras | Sunday 02:00-06:00 | Weekly |
| Yurtiçi | Sunday 03:00-05:00 | Weekly |
| Sürat | Sunday 02:00-04:00 | Weekly |
| PTT | Sunday 00:00-06:00 | Weekly, longer |
| Hepsijet | Rare | Announced |
| MNG | Sunday 02:00-04:00 | Weekly |
| Kolaygelsin | Rare | Announced |
| Sendeo | Rare | Announced |
| Pakettaxi | Rare | Announced |

---

## Version History

| Date | Changes |
|------|---------|
| 2026-01-22 | Initial version derived from vendor_docs |

# Iyzico Payment Integration Specification

**Feature Branch**: `001-core-platform`  
**Created**: 2026-01-22  
**Status**: Draft

---

## Overview

This document defines the integration with Iyzico payment gateway for wallet top-up functionality. Iyzico is used as the sole payment provider for processing credit card payments to fund customer wallets.

---

## Integration Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      Customer Browser                            │
└─────────────────────────────────────────────────────────────────┘
         │                                        │
         │ 1. Create Topup                        │ 4. Complete Payment
         ▼                                        ▼
┌─────────────────────┐                  ┌─────────────────────┐
│  ShipAggregator API │                  │   Iyzico Checkout   │
│  POST /billing/topups│                  │   (Hosted Page)     │
└─────────────────────┘                  └─────────────────────┘
         │                                        │
         │ 2. Initialize Payment                  │ 5. Callback
         ▼                                        ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Iyzico API                                │
└─────────────────────────────────────────────────────────────────┘
         │
         │ 3. Return Payment URL
         ▼
┌─────────────────────┐
│  ShipAggregator API │
│  Return to Customer │
└─────────────────────┘
```

---

## Iyzico API Configuration

### Environments

| Environment | Base URL | Description |
|-------------|----------|-------------|
| Sandbox | `https://sandbox-api.iyzipay.com` | Testing |
| Production | `https://api.iyzipay.com` | Live payments |

### Credentials

| Credential | Description |
|------------|-------------|
| `API_KEY` | Public API key |
| `SECRET_KEY` | Secret key for signing |
| `BASE_URL` | Environment base URL |

### Request Signing

All requests must be signed using HMAC-SHA256:

```python
import hashlib
import base64

def generate_authorization_header(api_key: str, secret_key: str, random_key: str, request_body: str) -> str:
    """Generate Iyzico authorization header."""
    # Create hash string
    hash_string = random_key + request_body
    
    # Generate HMAC-SHA256
    signature = hmac.new(
        secret_key.encode('utf-8'),
        hash_string.encode('utf-8'),
        hashlib.sha256
    ).digest()
    
    # Base64 encode
    signature_b64 = base64.b64encode(signature).decode('utf-8')
    
    # Create authorization string
    auth_string = f"{api_key}:{random_key}:{signature_b64}"
    
    # Base64 encode final header
    return base64.b64encode(auth_string.encode('utf-8')).decode('utf-8')
```

---

## Top-Up Flow

### 1. Create Top-Up Request

**Endpoint**: `POST /billing/topups`

**Request**:

```json
{
  "amount": "1000.00",
  "return_url": "https://dashboard.shipaggregator.com/billing/topup/callback"
}
```

**Response**:

```json
{
  "id": "top_550e8400-e29b-41d4-a716-446655440000",
  "amount": "1000.00",
  "currency": "TRY",
  "status": "pending",
  "payment_url": "https://sandbox-api.iyzipay.com/checkout/form/xxx",
  "expires_at": "2026-01-22T15:30:00Z",
  "created_at": "2026-01-22T14:30:00Z"
}
```

### 2. Initialize Iyzico Checkout Form

**Iyzico API**: `POST /payment/iyzipos/checkoutform/initialize/auth/ecom`

**Request to Iyzico**:

```json
{
  "locale": "tr",
  "conversationId": "top_550e8400-e29b-41d4-a716-446655440000",
  "price": "1000.00",
  "paidPrice": "1000.00",
  "currency": "TRY",
  "basketId": "top_550e8400-e29b-41d4-a716-446655440000",
  "paymentGroup": "PRODUCT",
  "callbackUrl": "https://api.shipaggregator.com/v1/integrations/iyzico/callback",
  "enabledInstallments": [1],
  "buyer": {
    "id": "org_123e4567-e89b-12d3-a456-426614174000",
    "name": "Company",
    "surname": "Name",
    "gsmNumber": "+905321234567",
    "email": "billing@company.com",
    "identityNumber": "11111111111",
    "registrationAddress": "Company Address",
    "ip": "85.34.78.112",
    "city": "Istanbul",
    "country": "Turkey"
  },
  "shippingAddress": {
    "contactName": "Company Name",
    "city": "Istanbul",
    "country": "Turkey",
    "address": "Company Address"
  },
  "billingAddress": {
    "contactName": "Company Name",
    "city": "Istanbul",
    "country": "Turkey",
    "address": "Company Address"
  },
  "basketItems": [
    {
      "id": "wallet_topup",
      "name": "Wallet Top-Up",
      "category1": "Wallet",
      "itemType": "VIRTUAL",
      "price": "1000.00"
    }
  ]
}
```

**Response from Iyzico**:

```json
{
  "status": "success",
  "locale": "tr",
  "systemTime": 1706015400000,
  "conversationId": "top_550e8400-e29b-41d4-a716-446655440000",
  "token": "xxx-checkout-token-xxx",
  "checkoutFormContent": "<script>...</script>",
  "tokenExpireTime": 1800,
  "paymentPageUrl": "https://sandbox-api.iyzipay.com/checkout/form/xxx"
}
```

### 3. Customer Completes Payment

Customer is redirected to Iyzico checkout page and completes payment.

### 4. Iyzico Callback

**Endpoint**: `POST /integrations/iyzico/callback`

**Request from Iyzico**:

```
token=xxx-checkout-token-xxx
```

### 5. Retrieve Payment Result

**Iyzico API**: `POST /payment/iyzipos/checkoutform/auth/ecom/detail`

**Request to Iyzico**:

```json
{
  "locale": "tr",
  "conversationId": "top_550e8400-e29b-41d4-a716-446655440000",
  "token": "xxx-checkout-token-xxx"
}
```

**Response from Iyzico (Success)**:

```json
{
  "status": "success",
  "locale": "tr",
  "systemTime": 1706015500000,
  "conversationId": "top_550e8400-e29b-41d4-a716-446655440000",
  "price": 1000.00,
  "paidPrice": 1000.00,
  "currency": "TRY",
  "installment": 1,
  "paymentId": "12345678",
  "paymentStatus": "SUCCESS",
  "fraudStatus": 1,
  "merchantCommissionRate": 0,
  "merchantCommissionRateAmount": 0,
  "iyziCommissionRateAmount": 25.00,
  "iyziCommissionFee": 0.35,
  "cardType": "CREDIT_CARD",
  "cardAssociation": "MASTER_CARD",
  "cardFamily": "Bonus",
  "binNumber": "545454",
  "lastFourDigits": "5454",
  "basketId": "top_550e8400-e29b-41d4-a716-446655440000",
  "itemTransactions": [
    {
      "itemId": "wallet_topup",
      "paymentTransactionId": "87654321",
      "transactionStatus": 2,
      "price": 1000.00,
      "paidPrice": 1000.00
    }
  ]
}
```

**Response from Iyzico (Failure)**:

```json
{
  "status": "failure",
  "errorCode": "10051",
  "errorMessage": "Kart limiti yetersiz",
  "locale": "tr",
  "systemTime": 1706015500000,
  "conversationId": "top_550e8400-e29b-41d4-a716-446655440000"
}
```

### 6. Process Payment Result

On successful payment:

1. Update topup status to `completed`
2. Create ledger entry (credit)
3. Update wallet balance
4. Emit `billing.topup_completed` webhook event

On failed payment:

1. Update topup status to `failed`
2. Store failure reason
3. Emit `billing.topup_failed` webhook event

---

## Ledger Postings

### Successful Top-Up

```sql
-- Create ledger entry
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
  actor_type
) VALUES (
  'org_123...',
  'credit',
  100000,  -- 1000.00 TRY in kuruş
  'TRY',
  (SELECT COALESCE(SUM(CASE WHEN entry_type = 'credit' THEN amount_kurus ELSE -amount_kurus END), 0) + 100000 
   FROM ledger_entries WHERE organization_id = 'org_123...'),
  'topup',
  'top_550e8400...',
  'Wallet top-up via Iyzico',
  '{"iyzico_payment_id": "12345678", "card_last_four": "5454", "card_type": "MASTER_CARD"}',
  'system'
);

-- Update topup record
UPDATE topups SET
  status = 'completed',
  payment_id = '12345678',
  ledger_entry_id = (SELECT id FROM ledger_entries WHERE reference_id = 'top_550e8400...'),
  completed_at = NOW(),
  updated_at = NOW()
WHERE id = 'top_550e8400...';
```

---

## Error Handling

### Iyzico Error Codes

| Code | Description | Action |
|------|-------------|--------|
| `10051` | Insufficient limit | Inform user |
| `10005` | Transaction not approved | Inform user |
| `10012` | Invalid card number | Inform user |
| `10034` | Fraud suspicion | Block and review |
| `10041` | Lost card | Block card |
| `10043` | Stolen card | Block card |
| `10054` | Expired card | Inform user |
| `10057` | Card holder cannot do this | Inform user |
| `10058` | Terminal not permitted | Contact support |
| `10084` | Invalid CVC | Inform user |

### Error Response Mapping

```typescript
function mapIyzicoError(errorCode: string, errorMessage: string): TopupError {
  const fraudCodes = ['10034', '10041', '10043'];
  const userCodes = ['10051', '10005', '10012', '10054', '10057', '10084'];
  
  if (fraudCodes.includes(errorCode)) {
    return {
      code: 'PAYMENT_FRAUD_SUSPECTED',
      message: 'Payment could not be processed. Please contact support.',
      userMessage: 'Ödeme işlemi gerçekleştirilemedi. Lütfen destek ile iletişime geçin.',
      shouldBlock: true
    };
  }
  
  if (userCodes.includes(errorCode)) {
    return {
      code: 'PAYMENT_DECLINED',
      message: errorMessage,
      userMessage: errorMessage,
      shouldBlock: false
    };
  }
  
  return {
    code: 'PAYMENT_ERROR',
    message: 'Payment processing error',
    userMessage: 'Ödeme işlemi sırasında bir hata oluştu.',
    shouldBlock: false
  };
}
```

---

## Reconciliation

### Daily Reconciliation Process

1. **Fetch Iyzico Transactions**
   - Call Iyzico reporting API for previous day
   - Get all successful transactions

2. **Match with Local Records**
   - Match by `conversationId` (our topup ID)
   - Verify amounts match

3. **Handle Discrepancies**
   - Missing in Iyzico: Mark as `reconciliation_pending`
   - Missing locally: Create alert for manual review
   - Amount mismatch: Create alert for manual review

4. **Generate Report**
   - Total transactions
   - Total amount
   - Discrepancies found
   - Actions taken

### Reconciliation Query

```sql
-- Find unreconciled topups
SELECT 
  t.id,
  t.amount_kurus,
  t.payment_id,
  t.status,
  t.created_at
FROM topups t
WHERE t.status = 'completed'
  AND t.created_at >= CURRENT_DATE - INTERVAL '1 day'
  AND t.created_at < CURRENT_DATE
  AND t.reconciled_at IS NULL;
```

---

## Refund Flow

### Refund via Iyzico

**Iyzico API**: `POST /payment/refund`

**Request**:

```json
{
  "locale": "tr",
  "conversationId": "ref_550e8400-e29b-41d4-a716-446655440000",
  "paymentTransactionId": "87654321",
  "price": "1000.00",
  "currency": "TRY",
  "ip": "85.34.78.112"
}
```

**Response**:

```json
{
  "status": "success",
  "locale": "tr",
  "systemTime": 1706015600000,
  "conversationId": "ref_550e8400-e29b-41d4-a716-446655440000",
  "paymentId": "12345678",
  "paymentTransactionId": "87654321",
  "price": 1000.00,
  "currency": "TRY"
}
```

### Refund Ledger Posting

```sql
-- Create refund ledger entry (debit - reduces balance)
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
  actor_type
) VALUES (
  'org_123...',
  'debit',
  100000,
  'TRY',
  (SELECT balance_after_kurus - 100000 FROM ledger_entries 
   WHERE organization_id = 'org_123...' ORDER BY created_at DESC LIMIT 1),
  'refund',
  'ref_550e8400...',
  'Refund for topup top_550e8400...',
  '{"original_topup_id": "top_550e8400...", "iyzico_refund_id": "..."}',
  'system'
);
```

---

## Webhook Events

### Iyzico Webhook (Inbound)

Iyzico can send webhooks for payment status changes:

**Endpoint**: `POST /integrations/iyzico/webhook`

**Payload**:

```json
{
  "iyziEventType": "CHECKOUT_FORM_AUTH",
  "iyziEventTime": 1706015500000,
  "iyziReferenceCode": "xxx-checkout-token-xxx",
  "token": "xxx-checkout-token-xxx",
  "status": "SUCCESS",
  "paymentId": 12345678,
  "paymentConversationId": "top_550e8400-e29b-41d4-a716-446655440000"
}
```

**Signature Verification**:

```python
def verify_iyzico_webhook(payload: dict, signature: str, secret_key: str) -> bool:
    """Verify Iyzico webhook signature."""
    # Iyzico sends signature in X-IYZ-SIGNATURE header
    expected = hmac.new(
        secret_key.encode('utf-8'),
        json.dumps(payload, separators=(',', ':')).encode('utf-8'),
        hashlib.sha256
    ).hexdigest()
    
    return hmac.compare_digest(expected, signature)
```

---

## Security Considerations

### PCI DSS Compliance

- **No card data storage**: All card data handled by Iyzico
- **Checkout form**: Use Iyzico hosted checkout page
- **Tokenization**: Iyzico handles card tokenization

### Fraud Prevention

1. **IP Verification**: Log and verify customer IP
2. **Velocity Checks**: Limit top-ups per day/week
3. **Amount Limits**: Maximum single top-up amount
4. **Fraud Alerts**: Monitor Iyzico fraud status

### Rate Limits

| Operation | Limit |
|-----------|-------|
| Create topup | 10/hour per organization |
| Single topup amount | 50,000 TRY max |
| Daily topup total | 100,000 TRY max |

---

## Testing

### Sandbox Test Cards

| Card Number | Result |
|-------------|--------|
| `5528790000000008` | Success |
| `5528790000000016` | Failure - Insufficient funds |
| `5528790000000024` | Failure - Invalid card |
| `4603450000000000` | 3D Secure required |

### Test Scenarios

1. **Successful Payment**
   - Create topup
   - Complete payment with success card
   - Verify ledger entry created
   - Verify balance updated
   - Verify webhook event emitted

2. **Failed Payment**
   - Create topup
   - Complete payment with failure card
   - Verify topup status is `failed`
   - Verify no ledger entry created
   - Verify failure webhook event emitted

3. **Expired Payment**
   - Create topup
   - Wait for expiration (or mock)
   - Verify topup status is `cancelled`

4. **Callback Retry**
   - Create topup
   - Simulate callback failure
   - Retry callback
   - Verify idempotent processing

5. **Refund**
   - Complete successful topup
   - Request refund
   - Verify Iyzico refund processed
   - Verify ledger debit entry
   - Verify balance reduced

---

## Monitoring

### Metrics

| Metric | Description |
|--------|-------------|
| `iyzico.topups.initiated` | Topups created |
| `iyzico.topups.completed` | Successful topups |
| `iyzico.topups.failed` | Failed topups |
| `iyzico.topups.expired` | Expired topups |
| `iyzico.topups.amount_total` | Total amount processed |
| `iyzico.api.latency` | API call latency |
| `iyzico.api.errors` | API errors |

### Alerts

| Condition | Alert |
|-----------|-------|
| Success rate < 90% | Payment success rate low |
| API latency > 5s | Iyzico API slow |
| Fraud status = 0 | Fraud detected |
| Daily volume > threshold | Unusual payment volume |

---

## Configuration

### Environment Variables

```bash
# Iyzico Configuration
IYZICO_API_KEY=sandbox-xxx
IYZICO_SECRET_KEY=sandbox-yyy
IYZICO_BASE_URL=https://sandbox-api.iyzipay.com

# Limits
IYZICO_MAX_TOPUP_AMOUNT=5000000  # 50,000 TRY in kuruş
IYZICO_MAX_DAILY_AMOUNT=10000000  # 100,000 TRY in kuruş
IYZICO_TOPUP_EXPIRY_MINUTES=60

# Callbacks
IYZICO_CALLBACK_URL=https://api.shipaggregator.com/v1/integrations/iyzico/callback
IYZICO_WEBHOOK_URL=https://api.shipaggregator.com/v1/integrations/iyzico/webhook
```

---

## API Reference

### Internal Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/billing/topups` | POST | Create topup |
| `/billing/topups` | GET | List topups |
| `/billing/topups/{id}/confirm` | POST | Confirm topup (manual) |
| `/integrations/iyzico/callback` | POST | Iyzico callback |
| `/integrations/iyzico/webhook` | POST | Iyzico webhook |

### Iyzico Endpoints Used

| Endpoint | Description |
|----------|-------------|
| `/payment/iyzipos/checkoutform/initialize/auth/ecom` | Initialize checkout |
| `/payment/iyzipos/checkoutform/auth/ecom/detail` | Get payment result |
| `/payment/refund` | Process refund |
| `/reporting/payment/detail` | Get payment details |

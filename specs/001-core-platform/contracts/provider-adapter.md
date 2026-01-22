# Provider Adapter Specification

**Feature Branch**: `001-core-platform`  
**Created**: 2026-01-22  
**Status**: Draft

---

## Overview

This document defines the Provider Adapter interface, implementation patterns, and mapping rules for integrating with Turkish cargo carriers. Each provider adapter translates between our canonical API and the provider's specific API (SOAP or REST).

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      ShipAggregator API                         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Provider Adapter Interface                    │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐            │
│  │ getQuote()   │ │createShipment│ │ getTracking()│  ...       │
│  └──────────────┘ └──────────────┘ └──────────────┘            │
└─────────────────────────────────────────────────────────────────┘
         │                    │                    │
         ▼                    ▼                    ▼
┌─────────────┐      ┌─────────────┐      ┌─────────────┐
│ ArasAdapter │      │YurticiAdapter│     │HepsijetAdapter│
│   (SOAP)    │      │   (SOAP)    │      │   (REST)    │
└─────────────┘      └─────────────┘      └─────────────┘
         │                    │                    │
         ▼                    ▼                    ▼
┌─────────────┐      ┌─────────────┐      ┌─────────────┐
│  Aras API   │      │ Yurtiçi API │      │ Hepsijet API│
└─────────────┘      └─────────────┘      └─────────────┘
```

---

## Provider Adapter Interface

### TypeScript Interface

```typescript
interface ProviderAdapter {
  // Provider identification
  readonly providerCode: string;
  readonly apiType: 'soap' | 'rest';
  
  // Credential validation
  verifyCredentials(credentials: ProviderCredentials): Promise<VerificationResult>;
  
  // Quote operations
  getQuote(request: QuoteRequest): Promise<QuoteResponse>;
  
  // Shipment operations
  createShipment(request: CreateShipmentRequest): Promise<CreateShipmentResponse>;
  cancelShipment(request: CancelShipmentRequest): Promise<CancelShipmentResponse>;
  
  // Label operations
  getLabel(request: GetLabelRequest): Promise<GetLabelResponse>;
  
  // Pickup operations
  schedulePickup(request: SchedulePickupRequest): Promise<SchedulePickupResponse>;
  cancelPickup(request: CancelPickupRequest): Promise<CancelPickupResponse>;
  
  // Tracking operations
  getTracking(request: GetTrackingRequest): Promise<GetTrackingResponse>;
  parseTrackingWebhook(payload: unknown): Promise<TrackingWebhookEvent[]>;
  
  // Return operations
  createReturn(request: CreateReturnRequest): Promise<CreateReturnResponse>;
  
  // Reference data
  getDropoffPoints(request: GetDropoffPointsRequest): Promise<DropoffPoint[]>;
}
```

### Request/Response Types

```typescript
// Common types
interface ProviderCredentials {
  [key: string]: string;  // Provider-specific credential fields
}

interface VerificationResult {
  success: boolean;
  error?: string;
  accountInfo?: {
    accountNumber?: string;
    companyName?: string;
  };
}

// Quote types
interface QuoteRequest {
  origin: Address;
  destination: Address;
  parcel: Parcel;
  codAmount?: Money;
  declaredValue?: Money;
  serviceCode?: string;  // Optional: specific service
}

interface QuoteResponse {
  offers: ProviderOffer[];
  errors?: ProviderError[];
}

interface ProviderOffer {
  serviceCode: string;
  serviceName: string;
  basePriceKurus: number;
  fuelSurchargeKurus: number;
  codFeeKurus: number;
  vatKurus: number;
  totalPriceKurus: number;
  estimatedDaysMin: number;
  estimatedDaysMax: number;
  providerQuoteId?: string;
  expiresAt: Date;
}

// Shipment types
interface CreateShipmentRequest {
  origin: Address;
  destination: Address;
  parcel: Parcel;
  serviceCode: string;
  codAmount?: Money;
  declaredValue?: Money;
  reference?: string;
  labelFormat: 'pdf' | 'zpl' | 'png';
}

interface CreateShipmentResponse {
  providerShipmentId: string;
  trackingNumber: string;
  barcode?: string;
  label?: LabelData;
}

interface LabelData {
  format: 'pdf' | 'zpl' | 'png';
  content: Buffer;
  contentType: string;
}

// Tracking types
interface GetTrackingRequest {
  trackingNumber: string;
  providerShipmentId?: string;
}

interface GetTrackingResponse {
  events: TrackingEvent[];
  currentStatus: string;
}

interface TrackingEvent {
  statusCode: string;
  statusDescription: string;
  locationCity?: string;
  locationDetail?: string;
  occurredAt: Date;
  rawStatusCode: string;
  rawData?: unknown;
}

interface TrackingWebhookEvent {
  trackingNumber: string;
  events: TrackingEvent[];
}
```

---

## Status Mapping

### Normalized Status Codes

| Code | Description |
|------|-------------|
| `created` | Shipment created in system |
| `label_generated` | Label generated |
| `picked_up` | Picked up from sender |
| `in_transit` | In transit between facilities |
| `at_hub` | At sorting/distribution hub |
| `out_for_delivery` | Out for delivery |
| `delivered` | Successfully delivered |
| `delivery_attempted` | Delivery attempted but failed |
| `delivery_failed` | Delivery failed (final) |
| `returning` | Returning to sender |
| `returned_to_sender` | Returned to sender |
| `cancelled` | Shipment cancelled |
| `exception` | Exception/hold |

### Provider Status Mapping

Each provider adapter must implement status mapping:

```typescript
abstract class BaseProviderAdapter implements ProviderAdapter {
  protected abstract statusMap: Map<string, string>;
  
  protected mapStatus(providerStatus: string): string {
    const normalized = this.statusMap.get(providerStatus.toUpperCase());
    if (!normalized) {
      console.warn(`Unknown status from ${this.providerCode}: ${providerStatus}`);
      return 'exception';  // Default to exception for unknown statuses
    }
    return normalized;
  }
}
```

---

## SOAP Client Standards

### WSDL Handling

```typescript
interface SoapClientConfig {
  wsdlUrl: string;
  endpoint: string;
  timeout: number;
  headers?: Record<string, string>;
}

class SoapProviderAdapter extends BaseProviderAdapter {
  protected client: SoapClient;
  
  async initialize(config: SoapClientConfig): Promise<void> {
    this.client = await createSoapClient(config.wsdlUrl, {
      endpoint: config.endpoint,
      wsdl_options: {
        timeout: config.timeout
      }
    });
  }
  
  protected async callSoapMethod<T>(
    method: string,
    args: unknown
  ): Promise<T> {
    const startTime = Date.now();
    
    try {
      const [result] = await this.client[method + 'Async'](args);
      
      await this.logApiCall({
        method,
        args,
        result,
        latencyMs: Date.now() - startTime,
        success: true
      });
      
      return result;
    } catch (error) {
      await this.logApiCall({
        method,
        args,
        error,
        latencyMs: Date.now() - startTime,
        success: false
      });
      
      throw this.mapSoapError(error);
    }
  }
}
```

### SOAP Error Handling

```typescript
protected mapSoapError(error: unknown): ProviderError {
  if (error instanceof SoapFault) {
    return {
      code: 'PROVIDER_REJECTED',
      message: error.faultstring,
      providerErrorCode: error.faultcode,
      providerErrorMessage: error.detail
    };
  }
  
  if (error instanceof TimeoutError) {
    return {
      code: 'PROVIDER_TIMEOUT',
      message: `Request to ${this.providerCode} timed out`
    };
  }
  
  if (error instanceof ConnectionError) {
    return {
      code: 'PROVIDER_DOWN',
      message: `Cannot connect to ${this.providerCode}`
    };
  }
  
  return {
    code: 'PROVIDER_ERROR',
    message: `Unknown error from ${this.providerCode}`,
    originalError: error
  };
}
```

---

## REST Client Standards

### HTTP Client Configuration

```typescript
interface RestClientConfig {
  baseUrl: string;
  timeout: number;
  headers?: Record<string, string>;
  auth?: {
    type: 'basic' | 'bearer' | 'api_key';
    credentials: Record<string, string>;
  };
}

class RestProviderAdapter extends BaseProviderAdapter {
  protected httpClient: HttpClient;
  
  initialize(config: RestClientConfig): void {
    this.httpClient = new HttpClient({
      baseURL: config.baseUrl,
      timeout: config.timeout,
      headers: config.headers
    });
    
    if (config.auth) {
      this.setupAuth(config.auth);
    }
  }
  
  protected async callRestMethod<T>(
    method: 'GET' | 'POST' | 'PUT' | 'DELETE',
    path: string,
    data?: unknown,
    params?: Record<string, string>
  ): Promise<T> {
    const startTime = Date.now();
    
    try {
      const response = await this.httpClient.request({
        method,
        url: path,
        data,
        params
      });
      
      await this.logApiCall({
        method: `${method} ${path}`,
        request: { data, params },
        response: response.data,
        latencyMs: Date.now() - startTime,
        success: true
      });
      
      return response.data;
    } catch (error) {
      await this.logApiCall({
        method: `${method} ${path}`,
        request: { data, params },
        error,
        latencyMs: Date.now() - startTime,
        success: false
      });
      
      throw this.mapRestError(error);
    }
  }
}
```

### REST Error Handling

```typescript
protected mapRestError(error: unknown): ProviderError {
  if (error instanceof HttpError) {
    if (error.status >= 500) {
      return {
        code: 'PROVIDER_DOWN',
        message: `${this.providerCode} returned ${error.status}`
      };
    }
    
    if (error.status === 401 || error.status === 403) {
      return {
        code: 'PROVIDER_AUTH_FAILED',
        message: 'Authentication failed with provider'
      };
    }
    
    return {
      code: 'PROVIDER_REJECTED',
      message: error.data?.message || `Request rejected with ${error.status}`,
      providerErrorCode: error.data?.code,
      providerErrorMessage: error.data?.message
    };
  }
  
  if (error instanceof TimeoutError) {
    return {
      code: 'PROVIDER_TIMEOUT',
      message: `Request to ${this.providerCode} timed out`
    };
  }
  
  return {
    code: 'PROVIDER_ERROR',
    message: `Unknown error from ${this.providerCode}`,
    originalError: error
  };
}
```

---

## Address Mapping

### Canonical Address Format

```typescript
interface Address {
  contactName: string;
  companyName?: string;
  phone: string;  // E.164 format
  email?: string;
  streetLine1: string;
  streetLine2?: string;
  neighborhoodCode: string;
  neighborhoodName: string;
  districtCode: string;
  districtName: string;
  cityCode: string;
  cityName: string;
  postalCode?: string;
}
```

### Provider Address Mapping

Each provider has different address field requirements:

```typescript
// Aras address mapping
function mapToArasAddress(address: Address): ArasAddress {
  return {
    ReceiverName: address.contactName,
    ReceiverPhone: address.phone.replace('+90', '0'),
    ReceiverAddress: [
      address.streetLine1,
      address.streetLine2,
      address.neighborhoodName
    ].filter(Boolean).join(' '),
    ReceiverCityCode: parseInt(address.cityCode),
    ReceiverTownCode: parseInt(address.districtCode),
    ReceiverPostalCode: address.postalCode || ''
  };
}

// Yurtiçi address mapping
function mapToYurticiAddress(address: Address): YurticiAddress {
  return {
    cargoReceiverName: address.contactName,
    cargoReceiverPhone: address.phone.replace('+90', ''),
    cargoReceiverAddress: address.streetLine1,
    cargoReceiverMahalle: address.neighborhoodName,
    cargoReceiverIlce: address.districtName,
    cargoReceiverIl: address.cityName
  };
}

// Hepsijet address mapping (REST)
function mapToHepsijetAddress(address: Address): HepsijetAddress {
  return {
    name: address.contactName,
    phone: address.phone,
    address: {
      line1: address.streetLine1,
      line2: address.streetLine2,
      neighborhood: address.neighborhoodName,
      district: address.districtName,
      city: address.cityName,
      postal_code: address.postalCode
    }
  };
}
```

---

## Parcel Mapping

### Canonical Parcel Format

```typescript
interface Parcel {
  weightGrams: number;
  lengthCm: number;
  widthCm: number;
  heightCm: number;
  description?: string;
}
```

### Provider Parcel Mapping

```typescript
// Aras uses desi (volumetric weight)
function mapToArasParcel(parcel: Parcel): ArasParcel {
  const desi = (parcel.lengthCm * parcel.widthCm * parcel.heightCm) / 3000;
  return {
    PieceCount: 1,
    Weight: parcel.weightGrams / 1000,  // kg
    Desi: Math.ceil(desi),
    ProductDescription: parcel.description || 'Paket'
  };
}

// Yurtiçi uses kg
function mapToYurticiParcel(parcel: Parcel): YurticiParcel {
  return {
    cargoCount: 1,
    cargoWeight: parcel.weightGrams / 1000,
    cargoDesi: calculateDesi(parcel),
    cargoDescription: parcel.description || 'Kargo'
  };
}

// Hepsijet uses grams
function mapToHepsijetParcel(parcel: Parcel): HepsijetParcel {
  return {
    weight: parcel.weightGrams,
    dimensions: {
      length: parcel.lengthCm,
      width: parcel.widthCm,
      height: parcel.heightCm
    },
    description: parcel.description
  };
}

function calculateDesi(parcel: Parcel): number {
  return Math.ceil((parcel.lengthCm * parcel.widthCm * parcel.heightCm) / 3000);
}
```

---

## Credential Schemas

### Per-Provider Credential Requirements

```typescript
const credentialSchemas: Record<string, CredentialSchema> = {
  aras: {
    fields: [
      { name: 'username', type: 'string', required: true },
      { name: 'password', type: 'string', required: true, sensitive: true },
      { name: 'customerCode', type: 'string', required: true }
    ]
  },
  yurtici: {
    fields: [
      { name: 'username', type: 'string', required: true },
      { name: 'password', type: 'string', required: true, sensitive: true },
      { name: 'userLanguage', type: 'string', required: false, default: 'TR' }
    ]
  },
  hepsijet: {
    fields: [
      { name: 'apiKey', type: 'string', required: true, sensitive: true },
      { name: 'merchantId', type: 'string', required: true }
    ]
  },
  mng: {
    fields: [
      { name: 'username', type: 'string', required: true },
      { name: 'password', type: 'string', required: true, sensitive: true },
      { name: 'customerNumber', type: 'string', required: true }
    ]
  },
  surat: {
    fields: [
      { name: 'username', type: 'string', required: true },
      { name: 'password', type: 'string', required: true, sensitive: true }
    ]
  },
  ptt: {
    fields: [
      { name: 'username', type: 'string', required: true },
      { name: 'password', type: 'string', required: true, sensitive: true },
      { name: 'senderCode', type: 'string', required: true }
    ]
  },
  sendeo: {
    fields: [
      { name: 'apiKey', type: 'string', required: true, sensitive: true },
      { name: 'customerCode', type: 'string', required: true }
    ]
  },
  kolaygelsin: {
    fields: [
      { name: 'apiKey', type: 'string', required: true, sensitive: true }
    ]
  },
  pakettaxi: {
    fields: [
      { name: 'apiKey', type: 'string', required: true, sensitive: true },
      { name: 'secretKey', type: 'string', required: true, sensitive: true }
    ]
  }
};
```

---

## API Logging

### Log Structure

```typescript
interface ProviderApiLog {
  id: string;
  organizationId?: string;
  providerCode: string;
  operation: string;
  requestMethod: string;
  requestUrl: string;
  requestHeaders?: Record<string, string>;  // Masked
  requestBodyPreview?: string;  // Masked, truncated
  responseStatus?: number;
  responseHeaders?: Record<string, string>;
  responseBodyPreview?: string;  // Masked, truncated
  latencyMs: number;
  success: boolean;
  errorCode?: string;
  errorMessage?: string;
  traceId: string;
  createdAt: Date;
}
```

### Masking Rules

```typescript
function maskSensitiveData(data: unknown): unknown {
  if (typeof data === 'string') {
    return data;
  }
  
  if (typeof data !== 'object' || data === null) {
    return data;
  }
  
  const sensitiveFields = [
    'password', 'apiKey', 'secretKey', 'token',
    'phone', 'email', 'contactName', 'receiverName',
    'address', 'streetLine1', 'streetLine2'
  ];
  
  const masked = { ...data };
  
  for (const field of sensitiveFields) {
    if (field in masked) {
      const value = masked[field];
      if (typeof value === 'string') {
        masked[field] = value.substring(0, 3) + '***';
      }
    }
  }
  
  return masked;
}
```

---

## Retry Strategy

### Retryable Errors

```typescript
const retryableErrors = [
  'PROVIDER_DOWN',
  'PROVIDER_TIMEOUT',
  'ECONNRESET',
  'ETIMEDOUT',
  'ENOTFOUND'
];

function isRetryable(error: ProviderError): boolean {
  return retryableErrors.includes(error.code);
}
```

### Retry Configuration

```typescript
interface RetryConfig {
  maxAttempts: number;
  baseDelayMs: number;
  maxDelayMs: number;
  backoffMultiplier: number;
}

const defaultRetryConfig: RetryConfig = {
  maxAttempts: 3,
  baseDelayMs: 1000,
  maxDelayMs: 10000,
  backoffMultiplier: 2
};

async function withRetry<T>(
  operation: () => Promise<T>,
  config: RetryConfig = defaultRetryConfig
): Promise<T> {
  let lastError: Error;
  
  for (let attempt = 1; attempt <= config.maxAttempts; attempt++) {
    try {
      return await operation();
    } catch (error) {
      lastError = error;
      
      if (!isRetryable(error) || attempt === config.maxAttempts) {
        throw error;
      }
      
      const delay = Math.min(
        config.baseDelayMs * Math.pow(config.backoffMultiplier, attempt - 1),
        config.maxDelayMs
      );
      
      await sleep(delay + Math.random() * 1000);  // Add jitter
    }
  }
  
  throw lastError;
}
```

---

## Circuit Breaker

### Configuration

```typescript
interface CircuitBreakerConfig {
  failureThreshold: number;  // Failures before opening
  successThreshold: number;  // Successes before closing
  timeout: number;           // Time in open state before half-open
}

const defaultCircuitConfig: CircuitBreakerConfig = {
  failureThreshold: 5,
  successThreshold: 2,
  timeout: 30000  // 30 seconds
};
```

### Implementation

```typescript
class CircuitBreaker {
  private state: 'closed' | 'open' | 'half-open' = 'closed';
  private failures = 0;
  private successes = 0;
  private lastFailureTime?: Date;
  
  async execute<T>(operation: () => Promise<T>): Promise<T> {
    if (this.state === 'open') {
      if (Date.now() - this.lastFailureTime.getTime() > this.config.timeout) {
        this.state = 'half-open';
      } else {
        throw new CircuitOpenError(this.providerCode);
      }
    }
    
    try {
      const result = await operation();
      this.onSuccess();
      return result;
    } catch (error) {
      this.onFailure();
      throw error;
    }
  }
  
  private onSuccess(): void {
    if (this.state === 'half-open') {
      this.successes++;
      if (this.successes >= this.config.successThreshold) {
        this.state = 'closed';
        this.failures = 0;
        this.successes = 0;
      }
    } else {
      this.failures = 0;
    }
  }
  
  private onFailure(): void {
    this.failures++;
    this.lastFailureTime = new Date();
    
    if (this.failures >= this.config.failureThreshold) {
      this.state = 'open';
    }
    
    if (this.state === 'half-open') {
      this.state = 'open';
      this.successes = 0;
    }
  }
}
```

---

## Testing

### Contract Tests

Each adapter must pass contract tests:

```typescript
describe('ProviderAdapter Contract', () => {
  const adapters = [
    new ArasAdapter(),
    new YurticiAdapter(),
    new HepsijetAdapter(),
    // ... all adapters
  ];
  
  adapters.forEach(adapter => {
    describe(adapter.providerCode, () => {
      it('should verify valid credentials', async () => {
        const result = await adapter.verifyCredentials(validCredentials);
        expect(result.success).toBe(true);
      });
      
      it('should reject invalid credentials', async () => {
        const result = await adapter.verifyCredentials(invalidCredentials);
        expect(result.success).toBe(false);
      });
      
      it('should return quote with required fields', async () => {
        const response = await adapter.getQuote(standardQuoteRequest);
        expect(response.offers).toBeArray();
        response.offers.forEach(offer => {
          expect(offer.serviceCode).toBeDefined();
          expect(offer.totalPriceKurus).toBeNumber();
          expect(offer.estimatedDaysMin).toBeNumber();
        });
      });
      
      it('should create shipment and return tracking number', async () => {
        const response = await adapter.createShipment(standardShipmentRequest);
        expect(response.trackingNumber).toBeDefined();
        expect(response.providerShipmentId).toBeDefined();
      });
      
      it('should map all known statuses', async () => {
        const response = await adapter.getTracking({ trackingNumber: testTrackingNumber });
        response.events.forEach(event => {
          expect(normalizedStatuses).toContain(event.statusCode);
        });
      });
    });
  });
});
```

### Mock Adapters

For testing without provider APIs:

```typescript
class MockProviderAdapter implements ProviderAdapter {
  readonly providerCode = 'mock';
  readonly apiType = 'rest';
  
  private scenarios: Map<string, unknown> = new Map();
  
  setScenario(operation: string, response: unknown): void {
    this.scenarios.set(operation, response);
  }
  
  async getQuote(request: QuoteRequest): Promise<QuoteResponse> {
    return this.scenarios.get('getQuote') as QuoteResponse || {
      offers: [{
        serviceCode: 'standard',
        serviceName: 'Standard Delivery',
        totalPriceKurus: 4500,
        estimatedDaysMin: 2,
        estimatedDaysMax: 3,
        expiresAt: new Date(Date.now() + 3600000)
      }]
    };
  }
  
  // ... other methods
}
```

---

## Provider Registration

### Adapter Factory

```typescript
class ProviderAdapterFactory {
  private adapters: Map<string, () => ProviderAdapter> = new Map();
  
  register(providerCode: string, factory: () => ProviderAdapter): void {
    this.adapters.set(providerCode, factory);
  }
  
  create(providerCode: string): ProviderAdapter {
    const factory = this.adapters.get(providerCode);
    if (!factory) {
      throw new Error(`Unknown provider: ${providerCode}`);
    }
    return factory();
  }
}

// Registration
const factory = new ProviderAdapterFactory();
factory.register('aras', () => new ArasAdapter());
factory.register('yurtici', () => new YurticiAdapter());
factory.register('hepsijet', () => new HepsijetAdapter());
factory.register('mng', () => new MngAdapter());
factory.register('surat', () => new SuratAdapter());
factory.register('ptt', () => new PttAdapter());
factory.register('sendeo', () => new SendeoAdapter());
factory.register('kolaygelsin', () => new KolaygelsinAdapter());
factory.register('pakettaxi', () => new PakettaxiAdapter());
```

---

## Health Monitoring

### Health Check Interface

```typescript
interface ProviderHealth {
  providerCode: string;
  status: 'healthy' | 'degraded' | 'down';
  lastSuccessAt?: Date;
  lastErrorAt?: Date;
  lastError?: string;
  successRate: number;  // Last hour
  avgLatencyMs: number;
  p95LatencyMs: number;
}

async function checkProviderHealth(providerCode: string): Promise<ProviderHealth> {
  const stats = await getProviderStats(providerCode, '1 hour');
  
  let status: 'healthy' | 'degraded' | 'down';
  if (stats.successRate >= 0.99) {
    status = 'healthy';
  } else if (stats.successRate >= 0.90) {
    status = 'degraded';
  } else {
    status = 'down';
  }
  
  return {
    providerCode,
    status,
    lastSuccessAt: stats.lastSuccessAt,
    lastErrorAt: stats.lastErrorAt,
    lastError: stats.lastError,
    successRate: stats.successRate,
    avgLatencyMs: stats.avgLatencyMs,
    p95LatencyMs: stats.p95LatencyMs
  };
}
```

package hepsijet

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/knid/kargonet/internal/domain"
	"github.com/knid/kargonet/internal/provider"
	"github.com/knid/kargonet/internal/provider/rest"
)

const (
	// ProviderCode is the unique identifier for Hepsijet.
	ProviderCode = domain.ProviderCodeHepsijet

	// DefaultEndpoint is the default REST endpoint for Hepsijet.
	DefaultEndpoint = "https://api.hepsijet.com/v1"

	// SandboxEndpoint is the sandbox REST endpoint for Hepsijet.
	SandboxEndpoint = "https://sandbox-api.hepsijet.com/v1"

	// DefaultTimeout is the default request timeout.
	DefaultTimeout = 30 * time.Second

	// QuoteValidityDuration is how long quotes are valid.
	QuoteValidityDuration = 1 * time.Hour

	// TokenRefreshBuffer is the time before token expiry to refresh.
	TokenRefreshBuffer = 5 * time.Minute
)

// Config contains configuration for the Hepsijet adapter.
type Config struct {
	// Endpoint is the REST API endpoint URL.
	Endpoint string
	// SandboxEndpoint is the sandbox endpoint URL.
	SandboxEndpoint string
	// Timeout is the request timeout.
	Timeout time.Duration
	// UseSandbox indicates whether to use sandbox mode.
	UseSandbox bool
	// Logger is the logger instance.
	Logger *slog.Logger
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Endpoint:        DefaultEndpoint,
		SandboxEndpoint: SandboxEndpoint,
		Timeout:         DefaultTimeout,
		UseSandbox:      false,
	}
}

// tokenInfo holds authentication token information.
type tokenInfo struct {
	accessToken string
	expiresAt   time.Time
}

// Adapter implements the provider.Adapter interface for Hepsijet.
type Adapter struct {
	provider.BaseAdapter
	config     Config
	restClient *rest.Client
	logger     *slog.Logger

	// Token management
	tokenMu sync.RWMutex
	tokens  map[string]*tokenInfo // keyed by credentials hash
}

// NewAdapter creates a new Hepsijet adapter.
func NewAdapter(config Config, logger *slog.Logger) (*Adapter, error) {
	if logger == nil {
		logger = slog.Default()
	}

	endpoint := config.Endpoint
	if config.UseSandbox && config.SandboxEndpoint != "" {
		endpoint = config.SandboxEndpoint
	}

	restConfig := rest.ClientConfig{
		BaseURL: endpoint,
		Timeout: config.Timeout,
		Headers: map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
		},
		Logger:                logger,
		MaxRetries:            3,
		RetryBaseDelay:        1 * time.Second,
		RetryMaxDelay:         10 * time.Second,
		RetryMultiplier:       2.0,
		CircuitBreakerEnabled: true,
	}

	return &Adapter{
		BaseAdapter: provider.NewBaseAdapter(ProviderCode, domain.ProviderAPITypeREST),
		config:      config,
		restClient:  rest.NewClient(restConfig),
		logger:      logger,
		tokens:      make(map[string]*tokenInfo),
	}, nil
}

// NewAdapterFromProviderConfig creates an adapter from provider.AdapterConfig.
func NewAdapterFromProviderConfig(cfg provider.AdapterConfig, logger *slog.Logger) (provider.Adapter, error) {
	config := Config{
		Endpoint:        cfg.BaseURL,
		SandboxEndpoint: cfg.SandboxURL,
		Timeout:         cfg.Timeout,
		UseSandbox:      cfg.UseSandbox,
		Logger:          logger,
	}

	if config.Endpoint == "" {
		config.Endpoint = DefaultEndpoint
	}
	if config.SandboxEndpoint == "" {
		config.SandboxEndpoint = SandboxEndpoint
	}
	if config.Timeout == 0 {
		config.Timeout = DefaultTimeout
	}

	return NewAdapter(config, logger)
}

// getCredentials extracts Hepsijet credentials from domain credentials.
func (a *Adapter) getCredentials(creds *domain.ProviderCredentials) (*Credentials, error) {
	if creds == nil {
		return nil, provider.ErrInvalidCredentials
	}

	c := &Credentials{
		APIKey:    creds.APIKey,
		APISecret: creds.APISecret,
	}

	// Check for extra fields
	if creds.Extra != nil {
		if code, ok := creds.Extra["customer_code"]; ok {
			c.CustomerCode = code
		}
	}

	if c.APIKey == "" || c.APISecret == "" {
		return nil, provider.ErrInvalidCredentials
	}

	return c, nil
}

// credentialsHash returns a hash key for caching tokens.
// Uses SHA256 to avoid leaking the raw API key in cache keys or logs.
func (a *Adapter) credentialsHash(creds *Credentials) string {
	hash := sha256.Sum256([]byte(creds.APIKey))
	return hex.EncodeToString(hash[:])
}

// getToken retrieves or refreshes an access token.
func (a *Adapter) getToken(ctx context.Context, creds *Credentials) (string, error) {
	hash := a.credentialsHash(creds)

	// Check for cached token
	a.tokenMu.RLock()
	if token, ok := a.tokens[hash]; ok {
		if time.Now().Add(TokenRefreshBuffer).Before(token.expiresAt) {
			a.tokenMu.RUnlock()
			return token.accessToken, nil
		}
	}
	a.tokenMu.RUnlock()

	// Refresh token
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()

	// Double-check after acquiring write lock
	if token, ok := a.tokens[hash]; ok {
		if time.Now().Add(TokenRefreshBuffer).Before(token.expiresAt) {
			return token.accessToken, nil
		}
	}

	// Request new token
	authReq := AuthRequest{
		APIKey:    creds.APIKey,
		APISecret: creds.APISecret,
	}

	var authResp AuthResponse
	if err := a.restClient.Post(ctx, EndpointAuth, authReq, &authResp); err != nil {
		return "", fmt.Errorf("failed to authenticate: %w", err)
	}

	if !authResp.Success || authResp.AccessToken == "" {
		errMsg := "authentication failed"
		if authResp.Error != nil {
			errMsg = authResp.Error.Message
		}
		return "", &provider.ProviderError{
			Code:    "PROVIDER_AUTH_FAILED",
			Message: errMsg,
		}
	}

	// Cache token
	a.tokens[hash] = &tokenInfo{
		accessToken: authResp.AccessToken,
		expiresAt:   time.Now().Add(time.Duration(authResp.ExpiresIn) * time.Second),
	}

	return authResp.AccessToken, nil
}

// authenticatedRequest performs an authenticated REST request.
func (a *Adapter) authenticatedRequest(ctx context.Context, creds *Credentials, method, path string, body, response interface{}) error {
	token, err := a.getToken(ctx, creds)
	if err != nil {
		return err
	}

	req := rest.Request{
		Method: method,
		Path:   path,
		Body:   body,
		Headers: map[string]string{
			"Authorization": "Bearer " + token,
		},
	}

	resp, err := a.restClient.Do(ctx, req)
	if err != nil {
		return err
	}

	// Check for HTTP errors
	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if err := json.Unmarshal(resp.Body, &errResp); err == nil && errResp.Code != "" {
			return &provider.ProviderError{
				Code:            MapErrorCode(errResp.Code),
				Message:         errResp.Message,
				ProviderCode:    errResp.Code,
				ProviderMessage: errResp.Message,
				Retryable:       IsRetryableError(errResp.Code),
			}
		}
		return &provider.ProviderError{
			Code:      "PROVIDER_ERROR",
			Message:   fmt.Sprintf("HTTP %d", resp.StatusCode),
			Retryable: resp.StatusCode >= 500,
		}
	}

	// Parse response
	if response != nil && len(resp.Body) > 0 {
		if err := json.Unmarshal(resp.Body, response); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// VerifyCredentials validates the provided credentials with Hepsijet API.
func (a *Adapter) VerifyCredentials(ctx context.Context, creds domain.ProviderCredentials) (*provider.VerificationResult, error) {
	c, err := a.getCredentials(&creds)
	if err != nil {
		return &provider.VerificationResult{
			Success: false,
			Error:   "Invalid credentials format",
		}, nil
	}

	var resp VerifyCredentialsResponse
	if err := a.authenticatedRequest(ctx, c, "GET", EndpointVerify, nil, &resp); err != nil {
		a.logger.Error("failed to verify credentials",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return &provider.VerificationResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	if !resp.Success {
		errMsg := "verification failed"
		if resp.Error != nil {
			errMsg = resp.Error.Message
		}
		return &provider.VerificationResult{
			Success: false,
			Error:   errMsg,
		}, nil
	}

	return &provider.VerificationResult{
		Success: true,
		AccountInfo: &provider.AccountInfo{
			AccountNumber: resp.AccountNumber,
			CompanyName:   resp.CompanyName,
			ContractType:  resp.ContractType,
		},
	}, nil
}

// GetQuote retrieves shipping quotes from Hepsijet.
func (a *Adapter) GetQuote(ctx context.Context, req provider.QuoteRequest) (*provider.QuoteResponse, error) {
	creds, err := a.getCredentials(req.Credentials)
	if err != nil {
		return nil, err
	}

	// Calculate desi (volumetric weight)
	desi := float64(req.Parcel.LengthCm*req.Parcel.WidthCm*req.Parcel.HeightCm) / 3000.0

	quoteReq := GetQuoteRequest{
		SenderCityCode:       req.Origin.CityCode,
		SenderDistrictCode:   req.Origin.DistrictCode,
		ReceiverCityCode:     req.Destination.CityCode,
		ReceiverDistrictCode: req.Destination.DistrictCode,
		Weight:               float64(req.Parcel.WeightGrams) / 1000.0, // Convert to kg
		Desi:                 desi,
	}

	if req.CODAmountKurus != nil {
		quoteReq.CODAmount = float64(*req.CODAmountKurus) / 100.0 // Convert to TRY
	}
	if req.DeclaredValueKurus != nil {
		quoteReq.DeclaredValue = float64(*req.DeclaredValueKurus) / 100.0
	}
	if req.ServiceCode != "" {
		quoteReq.ServiceType = MapServiceCode(req.ServiceCode)
	}

	var resp GetQuoteResponse
	if err := a.authenticatedRequest(ctx, creds, "POST", EndpointQuote, quoteReq, &resp); err != nil {
		a.logger.Error("failed to get quote",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	if !resp.Success {
		errMsg := "quote request failed"
		errCode := "PROVIDER_ERROR"
		if resp.Error != nil {
			errMsg = resp.Error.Message
			errCode = resp.Error.Code
		}
		return &provider.QuoteResponse{
			Offers: []provider.ProviderOffer{},
			Errors: []provider.ProviderError{
				{
					Code:            MapErrorCode(errCode),
					Message:         errMsg,
					ProviderCode:    errCode,
					ProviderMessage: errMsg,
					Retryable:       IsRetryableError(errCode),
				},
			},
		}, nil
	}

	offers := make([]provider.ProviderOffer, 0, len(resp.Quotes))
	for _, q := range resp.Quotes {
		expiresAt := time.Now().Add(QuoteValidityDuration)
		if q.ValidUntil != "" {
			if parsed, err := time.Parse(time.RFC3339, q.ValidUntil); err == nil {
				expiresAt = parsed
			}
		}

		offers = append(offers, provider.ProviderOffer{
			ServiceCode:        strings.ToLower(q.ServiceCode),
			ServiceName:        q.ServiceName,
			BasePriceKurus:     int64(q.BasePrice * 100),
			FuelSurchargeKurus: int64(q.FuelSurcharge * 100),
			CODFeeKurus:        int64(q.CODFee * 100),
			InsuranceFeeKurus:  int64(q.InsuranceFee * 100),
			VATKurus:           int64(q.VAT * 100),
			TotalPriceKurus:    int64(q.TotalPrice * 100),
			EstimatedDaysMin:   q.EstimatedDaysMin,
			EstimatedDaysMax:   q.EstimatedDaysMax,
			ProviderQuoteID:    q.QuoteID,
			ExpiresAt:          expiresAt,
		})
	}

	return &provider.QuoteResponse{
		Offers: offers,
	}, nil
}

// CreateShipment creates a new shipment with Hepsijet.
func (a *Adapter) CreateShipment(ctx context.Context, req provider.CreateShipmentRequest) (*provider.CreateShipmentResponse, error) {
	creds, err := a.getCredentials(req.Credentials)
	if err != nil {
		return nil, err
	}

	// Calculate desi
	desi := float64(req.Parcel.LengthCm*req.Parcel.WidthCm*req.Parcel.HeightCm) / 3000.0

	// Build address strings
	originAddress := buildAddressString(req.Origin)
	destAddress := buildAddressString(req.Destination)

	shipmentReq := CreateShipmentRequest{
		ServiceType:  MapServiceCode(req.ServiceCode),
		QuoteID:      req.ProviderQuoteID,
		Reference:    req.Reference,
		LabelFormat:  MapLabelFormat(req.LabelFormat),
		PaymentType:  PaymentTypeSender,
		DeliveryType: DeliveryTypeDoor,
		Sender: AddressDetails{
			Name:         req.Origin.ContactName,
			Phone:        formatPhone(req.Origin.Phone),
			Email:        derefString(req.Origin.Email),
			Address:      originAddress,
			CityCode:     req.Origin.CityCode,
			DistrictCode: req.Origin.DistrictCode,
			PostalCode:   derefString(req.Origin.PostalCode),
		},
		Receiver: AddressDetails{
			Name:         req.Destination.ContactName,
			Phone:        formatPhone(req.Destination.Phone),
			Email:        derefString(req.Destination.Email),
			Address:      destAddress,
			CityCode:     req.Destination.CityCode,
			DistrictCode: req.Destination.DistrictCode,
			PostalCode:   derefString(req.Destination.PostalCode),
		},
		Parcel: ParcelDetails{
			PieceCount:  1,
			Weight:      float64(req.Parcel.WeightGrams) / 1000.0,
			Desi:        desi,
			Length:      float64(req.Parcel.LengthCm),
			Width:       float64(req.Parcel.WidthCm),
			Height:      float64(req.Parcel.HeightCm),
			Description: derefString(req.Parcel.Description),
		},
	}

	if req.Origin.CompanyName != nil && *req.Origin.CompanyName != "" {
		shipmentReq.Sender.CompanyName = *req.Origin.CompanyName
	}
	if req.Destination.CompanyName != nil && *req.Destination.CompanyName != "" {
		shipmentReq.Receiver.CompanyName = *req.Destination.CompanyName
	}
	if req.CODAmountKurus != nil {
		shipmentReq.CODAmount = float64(*req.CODAmountKurus) / 100.0
	}
	if req.DeclaredValueKurus != nil {
		shipmentReq.DeclaredValue = float64(*req.DeclaredValueKurus) / 100.0
	}

	var resp CreateShipmentResponse
	if err := a.authenticatedRequest(ctx, creds, "POST", EndpointShipments, shipmentReq, &resp); err != nil {
		a.logger.Error("failed to create shipment",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	if !resp.Success {
		errMsg := "shipment creation failed"
		errCode := "PROVIDER_ERROR"
		if resp.Error != nil {
			errMsg = resp.Error.Message
			errCode = resp.Error.Code
		}
		return nil, &provider.ProviderError{
			Code:            MapErrorCode(errCode),
			Message:         errMsg,
			ProviderCode:    errCode,
			ProviderMessage: errMsg,
			Retryable:       IsRetryableError(errCode),
		}
	}

	result := &provider.CreateShipmentResponse{
		ProviderShipmentID: resp.ShipmentID,
		TrackingNumber:     resp.TrackingNumber,
		Barcode:            resp.Barcode,
	}

	// Decode label if present
	if resp.LabelData != "" {
		labelBytes, err := base64.StdEncoding.DecodeString(resp.LabelData)
		if err == nil {
			result.Label = &provider.LabelData{
				Format:      MapLabelFormatFromHepsijet(resp.LabelFormat),
				Content:     labelBytes,
				ContentType: MapContentType(resp.LabelFormat),
			}
		}
	}

	return result, nil
}

// CancelShipment cancels an existing shipment.
func (a *Adapter) CancelShipment(ctx context.Context, req provider.CancelShipmentRequest) (*provider.CancelShipmentResponse, error) {
	creds, err := a.getCredentials(req.Credentials)
	if err != nil {
		return nil, err
	}

	cancelReq := CancelShipmentRequest{
		ShipmentID:     req.ProviderShipmentID,
		TrackingNumber: req.TrackingNumber,
		Reason:         req.Reason,
	}

	path := fmt.Sprintf("%s/%s/cancel", EndpointShipments, req.TrackingNumber)
	var resp CancelShipmentResponse
	if err := a.authenticatedRequest(ctx, creds, "POST", path, cancelReq, &resp); err != nil {
		a.logger.Error("failed to cancel shipment",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	if !resp.Success {
		errMsg := "cancellation failed"
		errCode := "PROVIDER_ERROR"
		if resp.Error != nil {
			errMsg = resp.Error.Message
			errCode = resp.Error.Code
		}
		return nil, &provider.ProviderError{
			Code:            MapErrorCode(errCode),
			Message:         errMsg,
			ProviderCode:    errCode,
			ProviderMessage: errMsg,
			Retryable:       IsRetryableError(errCode),
		}
	}

	var refundAmount *int64
	if resp.RefundAmount > 0 {
		amount := int64(resp.RefundAmount * 100)
		refundAmount = &amount
	}

	return &provider.CancelShipmentResponse{
		Success:           true,
		RefundAmountKurus: refundAmount,
	}, nil
}

// GetLabel retrieves the shipping label for a shipment.
func (a *Adapter) GetLabel(ctx context.Context, req provider.GetLabelRequest) (*provider.GetLabelResponse, error) {
	creds, err := a.getCredentials(req.Credentials)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	query.Set("format", MapLabelFormat(req.Format))

	path := fmt.Sprintf("%s/%s", EndpointLabels, req.TrackingNumber)

	token, err := a.getToken(ctx, creds)
	if err != nil {
		return nil, err
	}

	restReq := rest.Request{
		Method: "GET",
		Path:   path,
		Query:  query,
		Headers: map[string]string{
			"Authorization": "Bearer " + token,
		},
	}

	resp, err := a.restClient.Do(ctx, restReq)
	if err != nil {
		a.logger.Error("failed to get label",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if err := json.Unmarshal(resp.Body, &errResp); err == nil && errResp.Code != "" {
			return nil, &provider.ProviderError{
				Code:            MapErrorCode(errResp.Code),
				Message:         errResp.Message,
				ProviderCode:    errResp.Code,
				ProviderMessage: errResp.Message,
				Retryable:       IsRetryableError(errResp.Code),
			}
		}
		return nil, &provider.ProviderError{
			Code:    "PROVIDER_ERROR",
			Message: fmt.Sprintf("HTTP %d", resp.StatusCode),
		}
	}

	var labelResp GetLabelResponse
	if err := json.Unmarshal(resp.Body, &labelResp); err != nil {
		return nil, fmt.Errorf("failed to parse label response: %w", err)
	}

	if !labelResp.Success {
		errMsg := "label retrieval failed"
		errCode := "PROVIDER_ERROR"
		if labelResp.Error != nil {
			errMsg = labelResp.Error.Message
			errCode = labelResp.Error.Code
		}
		return nil, &provider.ProviderError{
			Code:            MapErrorCode(errCode),
			Message:         errMsg,
			ProviderCode:    errCode,
			ProviderMessage: errMsg,
			Retryable:       IsRetryableError(errCode),
		}
	}

	labelBytes, err := base64.StdEncoding.DecodeString(labelResp.LabelData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode label data: %w", err)
	}

	return &provider.GetLabelResponse{
		Label: provider.LabelData{
			Format:      MapLabelFormatFromHepsijet(labelResp.LabelFormat),
			Content:     labelBytes,
			ContentType: MapContentType(labelResp.LabelFormat),
		},
	}, nil
}

// GetTracking retrieves tracking information for a shipment.
func (a *Adapter) GetTracking(ctx context.Context, req provider.GetTrackingRequest) (*provider.GetTrackingResponse, error) {
	creds, err := a.getCredentials(req.Credentials)
	if err != nil {
		return nil, err
	}

	path := fmt.Sprintf("%s/%s", EndpointTracking, req.TrackingNumber)
	var resp GetTrackingResponse
	if err := a.authenticatedRequest(ctx, creds, "GET", path, nil, &resp); err != nil {
		a.logger.Error("failed to get tracking",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	if !resp.Success {
		errMsg := "tracking retrieval failed"
		errCode := "PROVIDER_ERROR"
		if resp.Error != nil {
			errMsg = resp.Error.Message
			errCode = resp.Error.Code
		}
		return nil, &provider.ProviderError{
			Code:            MapErrorCode(errCode),
			Message:         errMsg,
			ProviderCode:    errCode,
			ProviderMessage: errMsg,
			Retryable:       IsRetryableError(errCode),
		}
	}

	events := make([]provider.TrackingEventData, 0, len(resp.Events))
	for _, e := range resp.Events {
		occurredAt, _ := time.Parse(time.RFC3339, e.OccurredAt)
		if occurredAt.IsZero() {
			occurredAt, _ = time.Parse("2006-01-02T15:04:05", e.OccurredAt)
		}

		rawData, _ := json.Marshal(e)

		events = append(events, provider.TrackingEventData{
			StatusCode:        MapStatus(e.StatusCode),
			StatusDescription: MapStatusDescription(e.StatusCode),
			LocationCity:      e.LocationCity,
			LocationDetail:    e.LocationDetail,
			OccurredAt:        occurredAt,
			RawStatusCode:     e.StatusCode,
			RawData:           rawData,
		})
	}

	result := &provider.GetTrackingResponse{
		CurrentStatus: MapStatus(resp.StatusCode),
		Events:        events,
	}

	if resp.EstimatedDelivery != "" {
		if estimated, err := time.Parse(time.RFC3339, resp.EstimatedDelivery); err == nil {
			result.EstimatedDelivery = &estimated
		}
	}

	return result, nil
}

// ParseTrackingWebhook parses an incoming tracking webhook payload.
func (a *Adapter) ParseTrackingWebhook(ctx context.Context, payload []byte) ([]provider.TrackingWebhookEvent, error) {
	var webhookPayload TrackingWebhookPayload
	if err := json.Unmarshal(payload, &webhookPayload); err != nil {
		return nil, fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	events := make([]provider.TrackingEventData, 0, len(webhookPayload.Events))
	for _, e := range webhookPayload.Events {
		rawData, _ := json.Marshal(e)
		events = append(events, provider.TrackingEventData{
			StatusCode:        MapStatus(e.StatusCode),
			StatusDescription: MapStatusDescription(e.StatusCode),
			LocationCity:      e.LocationCity,
			LocationDetail:    e.LocationDetail,
			OccurredAt:        e.OccurredAt,
			RawStatusCode:     e.StatusCode,
			RawData:           rawData,
		})
	}

	return []provider.TrackingWebhookEvent{
		{
			TrackingNumber:     webhookPayload.TrackingNumber,
			ProviderShipmentID: webhookPayload.ShipmentID,
			Events:             events,
		},
	}, nil
}

// SchedulePickup schedules a pickup for shipments.
func (a *Adapter) SchedulePickup(ctx context.Context, req provider.SchedulePickupRequest) (*provider.SchedulePickupResponse, error) {
	creds, err := a.getCredentials(req.Credentials)
	if err != nil {
		return nil, err
	}

	address := buildAddressString(req.Address)

	pickupReq := SchedulePickupRequest{
		PickupAddress: AddressDetails{
			Name:         req.Address.ContactName,
			Phone:        formatPhone(req.Address.Phone),
			Address:      address,
			CityCode:     req.Address.CityCode,
			DistrictCode: req.Address.DistrictCode,
			PostalCode:   derefString(req.Address.PostalCode),
		},
		ScheduledDate:   req.ScheduledDate.Format("2006-01-02"),
		TimeWindowStart: req.TimeWindowStart,
		TimeWindowEnd:   req.TimeWindowEnd,
		ParcelCount:     req.ParcelCount,
		TotalWeight:     float64(req.TotalWeightGrams) / 1000.0,
		TrackingNumbers: req.TrackingNumbers,
		Instructions:    req.Instructions,
	}

	var resp SchedulePickupResponse
	if err := a.authenticatedRequest(ctx, creds, "POST", EndpointPickups, pickupReq, &resp); err != nil {
		a.logger.Error("failed to schedule pickup",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	if !resp.Success {
		errMsg := "pickup scheduling failed"
		errCode := "PROVIDER_ERROR"
		if resp.Error != nil {
			errMsg = resp.Error.Message
			errCode = resp.Error.Code
		}
		return nil, &provider.ProviderError{
			Code:            MapErrorCode(errCode),
			Message:         errMsg,
			ProviderCode:    errCode,
			ProviderMessage: errMsg,
			Retryable:       IsRetryableError(errCode),
		}
	}

	confirmedDate, _ := time.Parse("2006-01-02", resp.ConfirmedDate)

	return &provider.SchedulePickupResponse{
		PickupID:        resp.PickupID,
		ConfirmedDate:   confirmedDate,
		TimeWindowStart: resp.TimeWindowStart,
		TimeWindowEnd:   resp.TimeWindowEnd,
	}, nil
}

// CancelPickup cancels a scheduled pickup.
func (a *Adapter) CancelPickup(ctx context.Context, req provider.CancelPickupRequest) (*provider.CancelPickupResponse, error) {
	creds, err := a.getCredentials(req.Credentials)
	if err != nil {
		return nil, err
	}

	cancelReq := CancelPickupRequest{
		PickupID: req.PickupID,
		Reason:   req.Reason,
	}

	path := fmt.Sprintf("%s/%s/cancel", EndpointPickups, req.PickupID)
	var resp CancelPickupResponse
	if err := a.authenticatedRequest(ctx, creds, "POST", path, cancelReq, &resp); err != nil {
		a.logger.Error("failed to cancel pickup",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	if !resp.Success {
		errMsg := "pickup cancellation failed"
		errCode := "PROVIDER_ERROR"
		if resp.Error != nil {
			errMsg = resp.Error.Message
			errCode = resp.Error.Code
		}
		return nil, &provider.ProviderError{
			Code:            MapErrorCode(errCode),
			Message:         errMsg,
			ProviderCode:    errCode,
			ProviderMessage: errMsg,
			Retryable:       IsRetryableError(errCode),
		}
	}

	return &provider.CancelPickupResponse{
		Success: true,
	}, nil
}

// CreateReturn creates a return shipment.
func (a *Adapter) CreateReturn(ctx context.Context, req provider.CreateReturnRequest) (*provider.CreateReturnResponse, error) {
	creds, err := a.getCredentials(req.Credentials)
	if err != nil {
		return nil, err
	}

	desi := float64(req.Parcel.LengthCm*req.Parcel.WidthCm*req.Parcel.HeightCm) / 3000.0
	originAddress := buildAddressString(req.Origin)
	destAddress := buildAddressString(req.Destination)

	returnReq := CreateReturnRequest{
		OriginalTrackingNumber: req.OriginalTrackingNumber,
		OriginalShipmentID:     req.OriginalProviderShipmentID,
		ServiceType:            MapServiceCode(req.ServiceCode),
		Reference:              req.Reference,
		LabelFormat:            MapLabelFormat(req.LabelFormat),
		Sender: AddressDetails{
			Name:         req.Origin.ContactName,
			Phone:        formatPhone(req.Origin.Phone),
			Address:      originAddress,
			CityCode:     req.Origin.CityCode,
			DistrictCode: req.Origin.DistrictCode,
			PostalCode:   derefString(req.Origin.PostalCode),
		},
		Receiver: AddressDetails{
			Name:         req.Destination.ContactName,
			Phone:        formatPhone(req.Destination.Phone),
			Address:      destAddress,
			CityCode:     req.Destination.CityCode,
			DistrictCode: req.Destination.DistrictCode,
			PostalCode:   derefString(req.Destination.PostalCode),
		},
		Parcel: ParcelDetails{
			PieceCount:  1,
			Weight:      float64(req.Parcel.WeightGrams) / 1000.0,
			Desi:        desi,
			Length:      float64(req.Parcel.LengthCm),
			Width:       float64(req.Parcel.WidthCm),
			Height:      float64(req.Parcel.HeightCm),
			Description: derefString(req.Parcel.Description),
		},
	}

	var resp CreateReturnResponse
	if err := a.authenticatedRequest(ctx, creds, "POST", EndpointReturns, returnReq, &resp); err != nil {
		a.logger.Error("failed to create return",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	if !resp.Success {
		errMsg := "return creation failed"
		errCode := "PROVIDER_ERROR"
		if resp.Error != nil {
			errMsg = resp.Error.Message
			errCode = resp.Error.Code
		}
		return nil, &provider.ProviderError{
			Code:            MapErrorCode(errCode),
			Message:         errMsg,
			ProviderCode:    errCode,
			ProviderMessage: errMsg,
			Retryable:       IsRetryableError(errCode),
		}
	}

	result := &provider.CreateReturnResponse{
		ProviderShipmentID: resp.ShipmentID,
		TrackingNumber:     resp.TrackingNumber,
		Barcode:            resp.Barcode,
	}

	if resp.LabelData != "" {
		labelBytes, err := base64.StdEncoding.DecodeString(resp.LabelData)
		if err == nil {
			result.Label = &provider.LabelData{
				Format:      MapLabelFormatFromHepsijet(resp.LabelFormat),
				Content:     labelBytes,
				ContentType: MapContentType(resp.LabelFormat),
			}
		}
	}

	return result, nil
}

// GetDropoffPoints retrieves available dropoff points.
func (a *Adapter) GetDropoffPoints(ctx context.Context, req provider.GetDropoffPointsRequest) (*provider.GetDropoffPointsResponse, error) {
	creds, err := a.getCredentials(req.Credentials)
	if err != nil {
		return nil, err
	}

	query := url.Values{}
	if req.CityCode != "" {
		query.Set("cityCode", req.CityCode)
	}
	if req.DistrictCode != "" {
		query.Set("districtCode", req.DistrictCode)
	}
	if req.PostalCode != "" {
		query.Set("postalCode", req.PostalCode)
	}
	if req.Limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", req.Limit))
	}

	token, err := a.getToken(ctx, creds)
	if err != nil {
		return nil, err
	}

	restReq := rest.Request{
		Method: "GET",
		Path:   EndpointDropoffPoints,
		Query:  query,
		Headers: map[string]string{
			"Authorization": "Bearer " + token,
		},
	}

	restResp, err := a.restClient.Do(ctx, restReq)
	if err != nil {
		a.logger.Error("failed to get dropoff points",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, err
	}

	if restResp.StatusCode >= 400 {
		var errResp ErrorResponse
		if err := json.Unmarshal(restResp.Body, &errResp); err == nil && errResp.Code != "" {
			return nil, &provider.ProviderError{
				Code:            MapErrorCode(errResp.Code),
				Message:         errResp.Message,
				ProviderCode:    errResp.Code,
				ProviderMessage: errResp.Message,
				Retryable:       IsRetryableError(errResp.Code),
			}
		}
		return nil, &provider.ProviderError{
			Code:    "PROVIDER_ERROR",
			Message: fmt.Sprintf("HTTP %d", restResp.StatusCode),
		}
	}

	var resp GetDropoffPointsResponse
	if err := json.Unmarshal(restResp.Body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse dropoff points response: %w", err)
	}

	if !resp.Success {
		errMsg := "dropoff points retrieval failed"
		errCode := "PROVIDER_ERROR"
		if resp.Error != nil {
			errMsg = resp.Error.Message
			errCode = resp.Error.Code
		}
		return nil, &provider.ProviderError{
			Code:            MapErrorCode(errCode),
			Message:         errMsg,
			ProviderCode:    errCode,
			ProviderMessage: errMsg,
			Retryable:       IsRetryableError(errCode),
		}
	}

	points := make([]provider.DropoffPoint, 0, len(resp.Points))
	for _, p := range resp.Points {
		var lat, lng *float64
		if p.Latitude != 0 {
			lat = &p.Latitude
		}
		if p.Longitude != 0 {
			lng = &p.Longitude
		}

		points = append(points, provider.DropoffPoint{
			ID:           p.ID,
			Name:         p.Name,
			Address:      p.Address,
			CityCode:     p.CityCode,
			CityName:     p.CityName,
			DistrictCode: p.DistrictCode,
			DistrictName: p.DistrictName,
			PostalCode:   p.PostalCode,
			Latitude:     lat,
			Longitude:    lng,
			Phone:        p.Phone,
			WorkingHours: p.WorkingHours,
			IsActive:     p.IsActive,
		})
	}

	return &provider.GetDropoffPointsResponse{
		Points: points,
	}, nil
}

// Helper functions

// buildAddressString builds a full address string from an AddressSnapshot.
func buildAddressString(addr domain.AddressSnapshot) string {
	parts := []string{addr.StreetLine1}
	if addr.StreetLine2 != nil && *addr.StreetLine2 != "" {
		parts = append(parts, *addr.StreetLine2)
	}
	if addr.Neighborhood != "" {
		parts = append(parts, addr.Neighborhood)
	}
	return strings.Join(parts, " ")
}

// formatPhone formats a phone number for Hepsijet.
func formatPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")

	// Hepsijet expects phone without country code
	if strings.HasPrefix(phone, "+90") {
		phone = "0" + phone[3:]
	} else if strings.HasPrefix(phone, "90") && len(phone) == 12 {
		phone = "0" + phone[2:]
	}

	return phone
}

// derefString safely dereferences a string pointer.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// init registers the adapter with the global factory.
func init() {
	provider.RegisterAdapter(ProviderCode, NewAdapterFromProviderConfig)
}

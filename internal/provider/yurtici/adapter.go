package yurtici

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/knid/kargonet/internal/domain"
	"github.com/knid/kargonet/internal/provider"
	"github.com/knid/kargonet/internal/provider/soap"
)

const (
	// ProviderCode is the unique identifier for Yurtici Kargo.
	ProviderCode = domain.ProviderCodeYurtici

	// DefaultEndpoint is the default SOAP endpoint for Yurtici.
	DefaultEndpoint = "https://ws.yurticikargo.com/KOPSWebServices/ShippingOrderDispatcherServices"

	// SandboxEndpoint is the sandbox SOAP endpoint for Yurtici.
	SandboxEndpoint = "https://testws.yurticikargo.com/KOPSWebServices/ShippingOrderDispatcherServices"

	// DefaultTimeout is the default request timeout.
	DefaultTimeout = 30 * time.Second
)

// Config contains configuration for the Yurtici adapter.
type Config struct {
	// Endpoint is the SOAP service endpoint URL.
	Endpoint string
	// SandboxEndpoint is the sandbox endpoint URL.
	SandboxEndpoint string
	// Timeout is the request timeout.
	Timeout time.Duration
	// UseSandbox indicates whether to use sandbox mode.
	UseSandbox bool
	// Logger is the logger instance.
	Logger *slog.Logger
	// OnAPIRequest is called after each SOAP request for logging.
	OnAPIRequest func(ctx context.Context, providerCode, operation, endpoint string, reqBody, respBody []byte, status int, latencyMs int64, success bool, err error)
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

// Adapter implements the provider.Adapter interface for Yurtici Kargo.
type Adapter struct {
	provider.BaseAdapter
	config     Config
	soapClient *soap.Client
	logger     *slog.Logger
}

// NewAdapter creates a new Yurtici adapter.
func NewAdapter(config Config, logger *slog.Logger) (*Adapter, error) {
	if logger == nil {
		logger = slog.Default()
	}

	endpoint := config.Endpoint
	if config.UseSandbox && config.SandboxEndpoint != "" {
		endpoint = config.SandboxEndpoint
	}

	soapConfig := soap.ClientConfig{
		Endpoint:    endpoint,
		Timeout:     config.Timeout,
		SOAPVersion: soap.SOAP11,
		Logger:      logger,
	}

	// Wire provider API logging callback
	if config.OnAPIRequest != nil {
		onReq := config.OnAPIRequest
		soapConfig.OnRequest = func(ctx context.Context, log soap.RequestLog) {
			onReq(ctx, string(ProviderCode), log.Action, log.Endpoint, log.RequestBody, log.ResponseBody, log.ResponseStatus, log.LatencyMs, log.Success, log.Error)
		}
	}

	return &Adapter{
		BaseAdapter: provider.NewBaseAdapter(ProviderCode, domain.ProviderAPITypeSOAP),
		config:      config,
		soapClient:  soap.NewClient(soapConfig),
		logger:      logger,
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
		OnAPIRequest:   cfg.OnAPIRequest,
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

// getCredentials extracts Yurtici credentials from domain credentials.
func (a *Adapter) getCredentials(creds *domain.ProviderCredentials) (*Credentials, error) {
	if creds == nil {
		return nil, provider.ErrInvalidCredentials
	}

	c := &Credentials{
		Username: creds.Username,
		Password: creds.Password,
		Language: LanguageTurkish,
	}

	if creds.Extra != nil {
		if lang, ok := creds.Extra["language"]; ok {
			c.Language = lang
		}
	}

	if c.Username == "" || c.Password == "" {
		return nil, provider.ErrInvalidCredentials
	}

	return c, nil
}

// VerifyCredentials validates the provided credentials with Yurtici API.
// Uses queryShipment with a dummy key: auth failure = invalid, key-not-found = valid.
func (a *Adapter) VerifyCredentials(ctx context.Context, creds domain.ProviderCredentials) (*provider.VerificationResult, error) {
	c, err := a.getCredentials(&creds)
	if err != nil {
		return &provider.VerificationResult{
			Success: false,
			Error:   "Invalid credentials format",
		}, nil
	}

	req := &QueryShipmentRequest{
		XMLNSSer:          soapNamespace,
		WsUserName:        c.Username,
		WsPassword:        c.Password,
		WsLanguage:        c.Language,
		Keys:              []string{"__VERIFY__"},
		KeyType:           0,
		AddHistoricalData: false,
		OnlyTracking:      false,
	}

	var resp QueryShipmentResponse
	if err := a.soapClient.Call(ctx, "queryShipment", req, &resp); err != nil {
		// SOAP fault or HTTP error likely means auth failure
		var provErr *provider.ProviderError
		if ok := isProviderError(err, &provErr); ok {
			if provErr.ProviderCode == "Server" || strings.Contains(provErr.Message, "auth") {
				return &provider.VerificationResult{
					Success: false,
					Error:   "Authentication failed",
				}, nil
			}
		}
		a.logger.Error("failed to verify credentials",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to verify credentials: %w", err)
	}

	// outFlag "2" with error code 936 = unexpected error (likely auth)
	if resp.Result.OutFlag == "2" {
		return &provider.VerificationResult{
			Success: false,
			Error:   resp.Result.OutResult,
		}, nil
	}

	// If we get details with errCode 82519 (key not found), credentials are valid
	if len(resp.Result.ShippingDeliveryDetailVO) > 0 {
		detail := resp.Result.ShippingDeliveryDetailVO[0]
		if detail.ErrCode == "82519" {
			return &provider.VerificationResult{
				Success: true,
			}, nil
		}
	}

	// outFlag "0" means the request was accepted, so credentials are valid
	if resp.Result.OutFlag == "0" || resp.Result.OutFlag == "1" {
		return &provider.VerificationResult{
			Success: true,
		}, nil
	}

	return &provider.VerificationResult{
		Success: false,
		Error:   resp.Result.OutResult,
	}, nil
}

// ---------------------------------------------------------------------------
// GetQuote — static contract pricing (KDV hariç / VAT excluded).
// Yurtiçi has no pricing API; prices come from the fixed contract agreement.
// Single zone: 0-9999 KM.
// ---------------------------------------------------------------------------

// yurticiContractPrice returns the Yurtiçi contract price in kuruş (KDV hariç) for a given desi.
func yurticiContractPrice(desi int) int64 {
	if desi <= 0 {
		return 11925
	}
	switch {
	case desi <= 1:
		return 12750
	case desi <= 2:
		return 13480
	case desi <= 3:
		return 14145
	case desi <= 4:
		return 15060
	case desi <= 5:
		return 16260
	case desi <= 10:
		return 19455
	case desi <= 15:
		return 26940
	case desi <= 20:
		return 35910
	case desi <= 25:
		return 44890
	case desi <= 30:
		return 53865
	default:
		extra := desi - 30
		return 53865 + int64(extra)*1796
	}
}

func (a *Adapter) GetQuote(_ context.Context, req provider.QuoteRequest) (*provider.QuoteResponse, error) {
	// Calculate desi: max(volumetric, weight_kg), rounded up
	volumetric := float64(req.Parcel.LengthCm*req.Parcel.WidthCm*req.Parcel.HeightCm) / 3000.0
	weightKg := float64(req.Parcel.WeightGrams) / 1000.0
	desi := volumetric
	if weightKg > desi {
		desi = weightKg
	}
	desiInt := int(math.Ceil(desi))

	basePriceKurus := yurticiContractPrice(desiInt)
	vatKurus := basePriceKurus * 20 / 100
	totalKurus := basePriceKurus + vatKurus

	return &provider.QuoteResponse{
		Offers: []provider.ProviderOffer{
			{
				ServiceCode:      "standard",
				ServiceName:      "Standart Kargo",
				BasePriceKurus:   basePriceKurus,
				VATKurus:         vatKurus,
				TotalPriceKurus:  totalKurus,
				EstimatedDaysMin: 2,
				EstimatedDaysMax: 4,
				ExpiresAt:        time.Now().Add(1 * time.Hour),
			},
		},
	}, nil
}

// CreateShipment creates a new shipment with Yurtici.
func (a *Adapter) CreateShipment(ctx context.Context, req provider.CreateShipmentRequest) (*provider.CreateShipmentResponse, error) {
	creds, err := a.getCredentials(req.Credentials)
	if err != nil {
		return nil, err
	}

	// Generate cargoKey from reference or use a truncated unique key
	cargoKey := req.Reference
	if cargoKey == "" {
		cargoKey = fmt.Sprintf("CK%d", time.Now().UnixNano()%1e16)
	}
	if len(cargoKey) > 20 {
		cargoKey = cargoKey[:20]
	}

	// invoiceKey = cargoKey (fatura bazli mode)
	invoiceKey := cargoKey

	// Build receiver address
	receiverAddress := buildAddressString(req.Destination)

	// Build receiver name
	receiverName := req.Destination.ContactName
	if req.Destination.CompanyName != nil && *req.Destination.CompanyName != "" {
		receiverName = *req.Destination.CompanyName + " - " + receiverName
	}

	// Calculate desi: max(volumetric, weight_kg), minimum 1
	volumetric := float64(req.Parcel.LengthCm*req.Parcel.WidthCm*req.Parcel.HeightCm) / 3000.0
	weightKg := float64(req.Parcel.WeightGrams) / 1000.0
	desi := volumetric
	if weightKg > desi {
		desi = weightKg
	}
	if desi < 1 {
		desi = 1
	}

	order := ShippingOrderVO{
		CargoKey:         cargoKey,
		InvoiceKey:       invoiceKey,
		ReceiverCustName: receiverName,
		ReceiverAddress:  receiverAddress,
		ReceiverPhone1:   formatPhone(req.Destination.Phone),
		CityName:         req.Destination.City,
		TownName:         req.Destination.District,
		Desi:             fmt.Sprintf("%.3f", desi),
		Kg:               fmt.Sprintf("%.3f", weightKg),
		CargoCount:       1,
	}

	// Set email if present
	if req.Destination.Email != nil && *req.Destination.Email != "" {
		order.EmailAddress = *req.Destination.Email
	}

	// Set description if present
	if req.Parcel.Description != nil {
		order.Description = *req.Parcel.Description
	}

	// Map COD (Kapida Odeme)
	if req.CODAmountKurus != nil && *req.CODAmountKurus > 0 {
		collType := "0" // Default: Cash
		if req.CODCollectionType != nil && *req.CODCollectionType == "credit_card" {
			collType = "1"
		}
		order.TtCollectionType = collType
		order.TtInvoiceAmount = fmt.Sprintf("%.2f", float64(*req.CODAmountKurus)/100.0)
		// ttDocumentId is Long(12) — must be purely numeric
		order.TtDocumentId = fmt.Sprintf("%d", time.Now().UnixNano()%1_000_000_000_000)
		order.TtDocumentSaveType = "0"

		// Credit card COD requires installment fields
		if collType == "1" {
			installments := 1
			if req.CODInstallments != nil && *req.CODInstallments > 0 {
				installments = int(*req.CODInstallments)
			}
			order.DcSelectedCredit = fmt.Sprintf("%d", installments)
			order.DcCreditRule = "1"
		}
	}

	soapReq := &CreateShipmentRequest{
		XMLNSSer:       soapNamespace,
		WsUserName:     creds.Username,
		WsPassword:     creds.Password,
		UserLanguage:   creds.Language,
		ShippingOrders: []ShippingOrderVO{order},
	}

	var resp CreateShipmentResponse
	if err := a.soapClient.Call(ctx, "createShipment", soapReq, &resp); err != nil {
		a.logger.Error("failed to create shipment",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to create shipment: %w", err)
	}

	// Check outFlag: 0=at least one success, 1=all failed, 2=unexpected error
	if resp.Result.OutFlag == "2" {
		return nil, &provider.ProviderError{
			Code:            MapErrorCode("936"),
			Message:         resp.Result.OutResult,
			ProviderCode:    "936",
			ProviderMessage: resp.Result.OutResult,
			Retryable:       true,
		}
	}

	if resp.Result.OutFlag == "1" {
		// All failed - get error from detail
		errMsg := resp.Result.OutResult
		errCode := "1"
		if len(resp.Result.ShippingOrderDetailVO) > 0 {
			detail := resp.Result.ShippingOrderDetailVO[0]
			errCode = detail.ErrCode
			errMsg = MapErrorMessage(detail.ErrCode, detail.ErrMessage)
		}
		return nil, &provider.ProviderError{
			Code:            MapErrorCode(errCode),
			Message:         errMsg,
			ProviderCode:    errCode,
			ProviderMessage: errMsg,
			Retryable:       IsRetryableError(errCode),
		}
	}

	// Success - check per-item detail
	if len(resp.Result.ShippingOrderDetailVO) > 0 {
		detail := resp.Result.ShippingOrderDetailVO[0]
		if detail.ErrCode != "0" && detail.ErrCode != "" {
			return nil, &provider.ProviderError{
				Code:            MapErrorCode(detail.ErrCode),
				Message:         MapErrorMessage(detail.ErrCode, detail.ErrMessage),
				ProviderCode:    detail.ErrCode,
				ProviderMessage: detail.ErrMessage,
				Retryable:       IsRetryableError(detail.ErrCode),
			}
		}
	}

	// Yurtici does NOT return labels via API - labels are generated at branch
	return &provider.CreateShipmentResponse{
		ProviderShipmentID: resp.Result.JobId,
		TrackingNumber:     cargoKey,
		// Label: nil — labels generated at YK branch when package is received
	}, nil
}

// CancelShipment cancels an existing shipment using cancelShipment SOAP method.
func (a *Adapter) CancelShipment(ctx context.Context, req provider.CancelShipmentRequest) (*provider.CancelShipmentResponse, error) {
	creds, err := a.getCredentials(req.Credentials)
	if err != nil {
		return nil, err
	}

	soapReq := &CancelShipmentRequest{
		XMLNSSer:     soapNamespace,
		WsUserName:   creds.Username,
		WsPassword:   creds.Password,
		UserLanguage: creds.Language,
		CargoKeys:    []string{req.TrackingNumber},
	}

	var resp CancelShipmentResponse
	if err := a.soapClient.Call(ctx, "cancelShipment", soapReq, &resp); err != nil {
		a.logger.Error("failed to cancel shipment",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to cancel shipment: %w", err)
	}

	// Check outFlag
	if resp.Result.OutFlag == "2" {
		return nil, &provider.ProviderError{
			Code:            "PROVIDER_ERROR",
			Message:         resp.Result.OutResult,
			ProviderCode:    "936",
			ProviderMessage: resp.Result.OutResult,
			Retryable:       true,
		}
	}

	// Check per-item cancel detail
	if len(resp.Result.ShippingCancelDetailVO) > 0 {
		detail := resp.Result.ShippingCancelDetailVO[0]

		if detail.ErrCode != "0" && detail.ErrCode != "" {
			return nil, &provider.ProviderError{
				Code:            MapErrorCode(detail.ErrCode),
				Message:         MapErrorMessage(detail.ErrCode, detail.ErrMessage),
				ProviderCode:    detail.ErrCode,
				ProviderMessage: detail.ErrMessage,
				Retryable:       IsRetryableError(detail.ErrCode),
			}
		}

		opCode := detail.OperationCode

		// DLV (5) = delivered, cannot cancel
		if opCode == CancelOpDLV {
			return nil, &provider.ProviderError{
				Code:            "PROVIDER_REJECTED",
				Message:         "Shipment has been delivered, cannot cancel",
				ProviderCode:    opCode,
				ProviderMessage: detail.OperationMessage,
				Retryable:       false,
			}
		}

		// ISC (4) = already cancelled
		if IsCancelAlreadyCancelled(opCode) {
			return &provider.CancelShipmentResponse{
				Success: true,
				Message: "Shipment was already cancelled",
			}, nil
		}

		// IND (1) = return request sent for in-transit
		if opCode == CancelOpIND {
			return &provider.CancelShipmentResponse{
				Success: true,
				Message: "Return request sent for in-transit shipment",
			}, nil
		}

		if IsCancelSuccess(opCode) {
			return &provider.CancelShipmentResponse{
				Success: true,
			}, nil
		}
	}

	return &provider.CancelShipmentResponse{
		Success: true,
	}, nil
}

// GetLabel is not supported by Yurtici API.
// Labels/barcodes are generated at the YK branch when the package is physically received.
func (a *Adapter) GetLabel(_ context.Context, _ provider.GetLabelRequest) (*provider.GetLabelResponse, error) {
	return nil, a.UnsupportedOperation("GetLabel")
}

// GetTracking retrieves tracking information using queryShipment.
func (a *Adapter) GetTracking(ctx context.Context, req provider.GetTrackingRequest) (*provider.GetTrackingResponse, error) {
	creds, err := a.getCredentials(req.Credentials)
	if err != nil {
		return nil, err
	}

	soapReq := &QueryShipmentRequest{
		XMLNSSer:          soapNamespace,
		WsUserName:        creds.Username,
		WsPassword:        creds.Password,
		WsLanguage:        creds.Language,
		Keys:              []string{req.TrackingNumber},
		KeyType:           0, // cargo key
		AddHistoricalData: true,
		OnlyTracking:      false,
	}

	var resp QueryShipmentResponse
	if err := a.soapClient.Call(ctx, "queryShipment", soapReq, &resp); err != nil {
		a.logger.Error("failed to get tracking",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to get tracking: %w", err)
	}

	// Check outFlag
	if resp.Result.OutFlag == "2" {
		return nil, &provider.ProviderError{
			Code:            "PROVIDER_ERROR",
			Message:         resp.Result.OutResult,
			ProviderCode:    "936",
			ProviderMessage: resp.Result.OutResult,
			Retryable:       true,
		}
	}

	if len(resp.Result.ShippingDeliveryDetailVO) == 0 {
		return nil, &provider.ProviderError{
			Code:    "PROVIDER_NOT_FOUND",
			Message: "Shipment not found",
		}
	}

	detail := resp.Result.ShippingDeliveryDetailVO[0]

	// Check for error
	if detail.ErrCode != "0" && detail.ErrCode != "" {
		return nil, &provider.ProviderError{
			Code:            MapErrorCode(detail.ErrCode),
			Message:         MapErrorMessage(detail.ErrCode, detail.ErrMessage),
			ProviderCode:    detail.ErrCode,
			ProviderMessage: detail.ErrMessage,
			Retryable:       IsRetryableError(detail.ErrCode),
		}
	}

	// Map current status from operationStatus
	currentStatus := MapOperationStatus(detail.OperationStatus)

	// Build events from the nested item detail (contains tracking history)
	var events []provider.TrackingEventData
	itemDetail := detail.ShippingDeliveryItemDetailVO

	if itemDetail != nil {
		events = make([]provider.TrackingEventData, 0, len(itemDetail.InvDocCargoVOArray))
		for _, evt := range itemDetail.InvDocCargoVOArray {
			occurredAt := parseYurticiDateTime(evt.EventDate, evt.EventTime)

			rawData, err := json.Marshal(evt)
			if err != nil {
				a.logger.Warn("Failed to marshal raw tracking data", "error", err)
			}

			location := evt.CityName
			locationDetail := evt.TownName
			if evt.UnitName != "" {
				if locationDetail != "" {
					locationDetail = locationDetail + " - " + evt.UnitName
				} else {
					locationDetail = evt.UnitName
				}
			}

			events = append(events, provider.TrackingEventData{
				StatusCode:        MapEventId(evt.EventId),
				StatusDescription: MapEventDescription(evt.EventId, evt.EventName),
				LocationCity:      location,
				LocationDetail:    locationDetail,
				OccurredAt:        occurredAt,
				RawStatusCode:     evt.EventId,
				RawData:           rawData,
			})
		}
	}

	// If no historical events but we have a status, add a synthetic event
	if len(events) == 0 {
		rawData, err := json.Marshal(detail)
		if err != nil {
			a.logger.Warn("Failed to marshal raw tracking data", "error", err)
		}
		events = append(events, provider.TrackingEventData{
			StatusCode:        currentStatus,
			StatusDescription: MapOperationStatusDescription(detail.OperationStatus),
			OccurredAt:        time.Now(),
			RawStatusCode:     detail.OperationStatus,
			RawData:           rawData,
		})
	}

	result := &provider.GetTrackingResponse{
		CurrentStatus: currentStatus,
		Events:        events,
	}

	// Parse delivery date from nested item detail if present
	if itemDetail != nil && itemDetail.DeliveryDate != "" {
		if delivered := parseYurticiDateTime(itemDetail.DeliveryDate, itemDetail.DeliveryTime); !delivered.IsZero() {
			result.EstimatedDelivery = &delivered
		}
	}

	return result, nil
}

// ParseTrackingWebhook parses an incoming tracking webhook payload.
func (a *Adapter) ParseTrackingWebhook(_ context.Context, payload []byte) ([]provider.TrackingWebhookEvent, error) {
	var webhookPayload TrackingWebhookPayload
	if err := json.Unmarshal(payload, &webhookPayload); err != nil {
		return nil, fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	events := make([]provider.TrackingEventData, 0, len(webhookPayload.Events))
	for _, e := range webhookPayload.Events {
		rawData, err := json.Marshal(e)
		if err != nil {
			a.logger.Warn("Failed to marshal raw tracking data", "error", err)
		}
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
			TrackingNumber:     webhookPayload.CargoKey,
			ProviderShipmentID: webhookPayload.InvoiceKey,
			Events:             events,
		},
	}, nil
}

// SchedulePickup is not supported by Yurtici API.
func (a *Adapter) SchedulePickup(_ context.Context, _ provider.SchedulePickupRequest) (*provider.SchedulePickupResponse, error) {
	return nil, a.UnsupportedOperation("SchedulePickup")
}

// CancelPickup is not supported by Yurtici API.
func (a *Adapter) CancelPickup(_ context.Context, _ provider.CancelPickupRequest) (*provider.CancelPickupResponse, error) {
	return nil, a.UnsupportedOperation("CancelPickup")
}

// CreateReturn creates a return shipment using cancelShipment.
// For in-transit shipments (operationStatus=IND), cancelShipment sends a return request.
func (a *Adapter) CreateReturn(ctx context.Context, req provider.CreateReturnRequest) (*provider.CreateReturnResponse, error) {
	creds, err := a.getCredentials(req.Credentials)
	if err != nil {
		return nil, err
	}

	soapReq := &CancelShipmentRequest{
		XMLNSSer:     soapNamespace,
		WsUserName:   creds.Username,
		WsPassword:   creds.Password,
		UserLanguage: creds.Language,
		CargoKeys:    []string{req.OriginalTrackingNumber},
	}

	var resp CancelShipmentResponse
	if err := a.soapClient.Call(ctx, "cancelShipment", soapReq, &resp); err != nil {
		a.logger.Error("failed to create return",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to create return: %w", err)
	}

	if resp.Result.OutFlag == "2" {
		return nil, &provider.ProviderError{
			Code:            "PROVIDER_ERROR",
			Message:         resp.Result.OutResult,
			ProviderCode:    "936",
			ProviderMessage: resp.Result.OutResult,
			Retryable:       true,
		}
	}

	if len(resp.Result.ShippingCancelDetailVO) > 0 {
		detail := resp.Result.ShippingCancelDetailVO[0]

		if detail.ErrCode != "0" && detail.ErrCode != "" {
			return nil, &provider.ProviderError{
				Code:            MapErrorCode(detail.ErrCode),
				Message:         MapErrorMessage(detail.ErrCode, detail.ErrMessage),
				ProviderCode:    detail.ErrCode,
				ProviderMessage: detail.ErrMessage,
				Retryable:       IsRetryableError(detail.ErrCode),
			}
		}

		return &provider.CreateReturnResponse{
			ProviderShipmentID: detail.JobId,
			TrackingNumber:     req.OriginalTrackingNumber,
		}, nil
	}

	return &provider.CreateReturnResponse{
		TrackingNumber: req.OriginalTrackingNumber,
	}, nil
}

// GetDropoffPoints is not supported by Yurtici API.
func (a *Adapter) GetDropoffPoints(_ context.Context, _ provider.GetDropoffPointsRequest) (*provider.GetDropoffPointsResponse, error) {
	return nil, a.UnsupportedOperation("GetDropoffPoints")
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

// formatPhone formats a phone number for Yurtici (10 digits with area code, no leading 0).
// Doc: "Alan kodu ile birlikte 10 adet rakamdan oluşmalıdır" — e.g. 5321234567.
func formatPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")

	if strings.HasPrefix(phone, "+90") {
		phone = phone[3:] // +905321234567 → 5321234567
	} else if strings.HasPrefix(phone, "90") && len(phone) == 12 {
		phone = phone[2:] // 905321234567 → 5321234567
	}

	// Strip leading 0 if present (05321234567 → 5321234567)
	if strings.HasPrefix(phone, "0") && len(phone) == 11 {
		phone = phone[1:]
	}

	return phone
}

// parseYurticiDateTime parses Yurtici date (YYYYMMDD) and time (HHMMSS) to time.Time.
func parseYurticiDateTime(date, timeStr string) time.Time {
	if date == "" {
		return time.Time{}
	}

	// Normalize: remove separators if present
	date = strings.ReplaceAll(date, "-", "")
	date = strings.ReplaceAll(date, "/", "")

	if len(date) != 8 {
		return time.Time{}
	}

	dateTimeStr := date
	if timeStr != "" {
		timeStr = strings.ReplaceAll(timeStr, ":", "")
		if len(timeStr) >= 6 {
			dateTimeStr = date + timeStr[:6]
		}
	}

	loc, _ := time.LoadLocation("Europe/Istanbul")
	if loc == nil {
		loc = time.UTC
	}

	if len(dateTimeStr) == 14 {
		t, err := time.ParseInLocation("20060102150405", dateTimeStr, loc)
		if err == nil {
			return t
		}
	}

	t, err := time.ParseInLocation("20060102", date, loc)
	if err != nil {
		return time.Time{}
	}
	return t
}

// isProviderError checks if an error is a ProviderError and extracts it.
func isProviderError(err error, target **provider.ProviderError) bool {
	if pe, ok := err.(*provider.ProviderError); ok {
		*target = pe
		return true
	}
	return false
}

// init registers the adapter with the global factory.
func init() {
	provider.RegisterAdapter(ProviderCode, NewAdapterFromProviderConfig)
}

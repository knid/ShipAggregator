package yurtici

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/e-port/shipping-aggregator/internal/domain"
	"github.com/e-port/shipping-aggregator/internal/provider"
	"github.com/e-port/shipping-aggregator/internal/provider/soap"
)

const (
	// ProviderCode is the unique identifier for Yurtiçi Kargo.
	ProviderCode = domain.ProviderCodeYurtici

	// DefaultEndpoint is the default SOAP endpoint for Yurtiçi.
	DefaultEndpoint = "https://webservices.yurticikargo.com/KOPSWebServices/ShippingOrderDispatcherServices"

	// SandboxEndpoint is the sandbox SOAP endpoint for Yurtiçi.
	SandboxEndpoint = "https://testwebservices.yurticikargo.com/KOPSWebServices/ShippingOrderDispatcherServices"

	// DefaultTimeout is the default request timeout.
	DefaultTimeout = 30 * time.Second

	// QuoteValidityDuration is how long quotes are valid.
	QuoteValidityDuration = 1 * time.Hour
)

// Config contains configuration for the Yurtiçi adapter.
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

// Adapter implements the provider.Adapter interface for Yurtiçi Kargo.
type Adapter struct {
	provider.BaseAdapter
	config     Config
	soapClient *soap.Client
	logger     *slog.Logger
}

// NewAdapter creates a new Yurtiçi adapter.
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

// getCredentials extracts Yurtiçi credentials from domain credentials.
func (a *Adapter) getCredentials(creds *domain.ProviderCredentials) (*Credentials, error) {
	if creds == nil {
		return nil, provider.ErrInvalidCredentials
	}

	c := &Credentials{
		Username:     creds.Username,
		Password:     creds.Password,
		CustomerCode: creds.CustomerCode,
		Language:     LanguageTurkish,
	}

	// Check for extra fields
	if c.CustomerCode == "" && creds.Extra != nil {
		if code, ok := creds.Extra["customer_code"]; ok {
			c.CustomerCode = code
		}
	}
	if creds.Extra != nil {
		if lang, ok := creds.Extra["language"]; ok {
			c.Language = lang
		}
	}

	if c.Username == "" || c.Password == "" || c.CustomerCode == "" {
		return nil, provider.ErrInvalidCredentials
	}

	return c, nil
}

// VerifyCredentials validates the provided credentials with Yurtiçi API.
func (a *Adapter) VerifyCredentials(ctx context.Context, creds domain.ProviderCredentials) (*provider.VerificationResult, error) {
	c, err := a.getCredentials(&creds)
	if err != nil {
		return &provider.VerificationResult{
			Success: false,
			Error:   "Invalid credentials format",
		}, nil
	}

	req := &VerifyCredentialsRequest{
		UserName:     c.Username,
		Password:     c.Password,
		CustomerCode: c.CustomerCode,
		Language:     c.Language,
	}

	var resp VerifyCredentialsResponse
	if err := a.soapClient.Call(ctx, "LoginCheck", req, &resp); err != nil {
		a.logger.Error("failed to verify credentials",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to verify credentials: %w", err)
	}

	if !resp.Result.Success {
		return &provider.VerificationResult{
			Success: false,
			Error:   resp.Result.ErrorMessage,
		}, nil
	}

	return &provider.VerificationResult{
		Success: true,
		AccountInfo: &provider.AccountInfo{
			AccountNumber: resp.Result.AccountNumber,
			CompanyName:   resp.Result.CompanyName,
			ContractType:  resp.Result.ContractType,
		},
	}, nil
}

// GetQuote retrieves shipping quotes from Yurtiçi.
func (a *Adapter) GetQuote(ctx context.Context, req provider.QuoteRequest) (*provider.QuoteResponse, error) {
	creds, err := a.getCredentials(req.Credentials)
	if err != nil {
		return nil, err
	}

	// Calculate desi (volumetric weight)
	desi := float64(req.Parcel.LengthCm*req.Parcel.WidthCm*req.Parcel.HeightCm) / 3000.0

	soapReq := &GetQuoteRequest{
		UserName:         creds.Username,
		Password:         creds.Password,
		CustomerCode:     creds.CustomerCode,
		Language:         creds.Language,
		SenderCityCode:   req.Origin.CityCode,
		SenderTownCode:   req.Origin.DistrictCode,
		ReceiverCityCode: req.Destination.CityCode,
		ReceiverTownCode: req.Destination.DistrictCode,
		Weight:           float64(req.Parcel.WeightGrams) / 1000.0, // Convert to kg
		Desi:             desi,
		CargoType:        CargoTypeParcel,
	}

	if req.CODAmountKurus != nil {
		soapReq.CODAmount = float64(*req.CODAmountKurus) / 100.0 // Convert to TRY
	}
	if req.DeclaredValueKurus != nil {
		soapReq.DeclaredValue = float64(*req.DeclaredValueKurus) / 100.0
	}
	if req.ServiceCode != "" {
		soapReq.ProductCode = MapProductCode(req.ServiceCode)
	}

	var resp GetQuoteResponse
	if err := a.soapClient.Call(ctx, "GetPrice", soapReq, &resp); err != nil {
		a.logger.Error("failed to get quote",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to get quote: %w", err)
	}

	if !resp.Result.Success {
		return &provider.QuoteResponse{
			Offers: []provider.ProviderOffer{},
			Errors: []provider.ProviderError{
				{
					Code:            MapErrorCode(resp.Result.ErrorCode),
					Message:         resp.Result.ErrorMessage,
					ProviderCode:    resp.Result.ErrorCode,
					ProviderMessage: resp.Result.ErrorMessage,
					Retryable:       IsRetryableError(resp.Result.ErrorCode),
				},
			},
		}, nil
	}

	offers := make([]provider.ProviderOffer, 0, len(resp.Result.Prices))
	for _, p := range resp.Result.Prices {
		expiresAt := time.Now().Add(QuoteValidityDuration)
		if p.ValidUntil != "" {
			if parsed, err := time.Parse(time.RFC3339, p.ValidUntil); err == nil {
				expiresAt = parsed
			}
		}

		offers = append(offers, provider.ProviderOffer{
			ServiceCode:        strings.ToLower(p.ProductCode),
			ServiceName:        p.ProductName,
			BasePriceKurus:     int64(p.BasePrice * 100),
			FuelSurchargeKurus: int64(p.FuelSurcharge * 100),
			CODFeeKurus:        int64(p.CODFee * 100),
			InsuranceFeeKurus:  int64(p.InsuranceFee * 100),
			VATKurus:           int64(p.VAT * 100),
			TotalPriceKurus:    int64(p.TotalPrice * 100),
			EstimatedDaysMin:   p.EstimatedDaysMin,
			EstimatedDaysMax:   p.EstimatedDaysMax,
			ProviderQuoteID:    p.QuoteID,
			ExpiresAt:          expiresAt,
		})
	}

	return &provider.QuoteResponse{
		Offers: offers,
	}, nil
}

// CreateShipment creates a new shipment with Yurtiçi.
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

	soapReq := &CreateShipmentRequest{
		UserName:     creds.Username,
		Password:     creds.Password,
		CustomerCode: creds.CustomerCode,
		Language:     creds.Language,
		Shipment: ShipmentDetails{
			ProductCode: MapProductCode(req.ServiceCode),
			QuoteID:     req.ProviderQuoteID,
			Reference:   req.Reference,
			LabelFormat: MapLabelFormat(req.LabelFormat),
			PaymentType: PaymentTypeSender,
			CargoType:   CargoTypeParcel,
			Sender: AddressDetails{
				Name:       req.Origin.ContactName,
				Phone:      formatPhone(req.Origin.Phone),
				Email:      derefString(req.Origin.Email),
				Address:    originAddress,
				CityCode:   req.Origin.CityCode,
				TownCode:   req.Origin.DistrictCode,
				PostalCode: derefString(req.Origin.PostalCode),
			},
			Receiver: AddressDetails{
				Name:       req.Destination.ContactName,
				Phone:      formatPhone(req.Destination.Phone),
				Email:      derefString(req.Destination.Email),
				Address:    destAddress,
				CityCode:   req.Destination.CityCode,
				TownCode:   req.Destination.DistrictCode,
				PostalCode: derefString(req.Destination.PostalCode),
			},
			Cargo: CargoDetails{
				PieceCount:  1,
				Weight:      float64(req.Parcel.WeightGrams) / 1000.0,
				Desi:        desi,
				Length:      float64(req.Parcel.LengthCm),
				Width:       float64(req.Parcel.WidthCm),
				Height:      float64(req.Parcel.HeightCm),
				Description: derefString(req.Parcel.Description),
			},
		},
	}

	if req.Origin.CompanyName != nil && *req.Origin.CompanyName != "" {
		soapReq.Shipment.Sender.CompanyName = *req.Origin.CompanyName
	}
	if req.Destination.CompanyName != nil && *req.Destination.CompanyName != "" {
		soapReq.Shipment.Receiver.CompanyName = *req.Destination.CompanyName
	}
	if req.CODAmountKurus != nil {
		soapReq.Shipment.CODAmount = float64(*req.CODAmountKurus) / 100.0
	}
	if req.DeclaredValueKurus != nil {
		soapReq.Shipment.DeclaredValue = float64(*req.DeclaredValueKurus) / 100.0
	}

	var resp CreateShipmentResponse
	if err := a.soapClient.Call(ctx, "CreateShipment", soapReq, &resp); err != nil {
		a.logger.Error("failed to create shipment",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to create shipment: %w", err)
	}

	if !resp.Result.Success {
		return nil, &provider.ProviderError{
			Code:            MapErrorCode(resp.Result.ErrorCode),
			Message:         resp.Result.ErrorMessage,
			ProviderCode:    resp.Result.ErrorCode,
			ProviderMessage: resp.Result.ErrorMessage,
			Retryable:       IsRetryableError(resp.Result.ErrorCode),
		}
	}

	result := &provider.CreateShipmentResponse{
		ProviderShipmentID: resp.Result.ShipmentID,
		TrackingNumber:     resp.Result.TrackingNumber,
		Barcode:            resp.Result.Barcode,
	}

	// Decode label if present
	if resp.Result.LabelData != "" {
		labelBytes, err := base64.StdEncoding.DecodeString(resp.Result.LabelData)
		if err == nil {
			result.Label = &provider.LabelData{
				Format:      MapLabelFormatFromYurtici(resp.Result.LabelFormat),
				Content:     labelBytes,
				ContentType: MapContentType(resp.Result.LabelFormat),
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

	soapReq := &CancelShipmentRequest{
		UserName:     creds.Username,
		Password:     creds.Password,
		CustomerCode: creds.CustomerCode,
		Language:     creds.Language,
		CargoKey:     req.TrackingNumber,
		InvoiceKey:   req.ProviderShipmentID,
		CancelReason: req.Reason,
	}

	var resp CancelShipmentResponse
	if err := a.soapClient.Call(ctx, "CancelShipment", soapReq, &resp); err != nil {
		a.logger.Error("failed to cancel shipment",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to cancel shipment: %w", err)
	}

	if !resp.Result.Success {
		return nil, &provider.ProviderError{
			Code:            MapErrorCode(resp.Result.ErrorCode),
			Message:         resp.Result.ErrorMessage,
			ProviderCode:    resp.Result.ErrorCode,
			ProviderMessage: resp.Result.ErrorMessage,
			Retryable:       IsRetryableError(resp.Result.ErrorCode),
		}
	}

	var refundAmount *int64
	if resp.Result.RefundAmount > 0 {
		amount := int64(resp.Result.RefundAmount * 100)
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

	soapReq := &GetLabelRequest{
		UserName:     creds.Username,
		Password:     creds.Password,
		CustomerCode: creds.CustomerCode,
		Language:     creds.Language,
		CargoKey:     req.TrackingNumber,
		InvoiceKey:   req.ProviderShipmentID,
		LabelFormat:  MapLabelFormat(req.Format),
	}

	var resp GetLabelResponse
	if err := a.soapClient.Call(ctx, "GetLabel", soapReq, &resp); err != nil {
		a.logger.Error("failed to get label",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to get label: %w", err)
	}

	if !resp.Result.Success {
		return nil, &provider.ProviderError{
			Code:            MapErrorCode(resp.Result.ErrorCode),
			Message:         resp.Result.ErrorMessage,
			ProviderCode:    resp.Result.ErrorCode,
			ProviderMessage: resp.Result.ErrorMessage,
			Retryable:       IsRetryableError(resp.Result.ErrorCode),
		}
	}

	labelBytes, err := base64.StdEncoding.DecodeString(resp.Result.LabelData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode label data: %w", err)
	}

	return &provider.GetLabelResponse{
		Label: provider.LabelData{
			Format:      MapLabelFormatFromYurtici(resp.Result.LabelFormat),
			Content:     labelBytes,
			ContentType: MapContentType(resp.Result.LabelFormat),
		},
	}, nil
}

// GetTracking retrieves tracking information for a shipment.
func (a *Adapter) GetTracking(ctx context.Context, req provider.GetTrackingRequest) (*provider.GetTrackingResponse, error) {
	creds, err := a.getCredentials(req.Credentials)
	if err != nil {
		return nil, err
	}

	soapReq := &GetTrackingRequest{
		UserName:     creds.Username,
		Password:     creds.Password,
		CustomerCode: creds.CustomerCode,
		Language:     creds.Language,
		CargoKey:     req.TrackingNumber,
		InvoiceKey:   req.ProviderShipmentID,
	}

	var resp GetTrackingResponse
	if err := a.soapClient.Call(ctx, "QueryShipment", soapReq, &resp); err != nil {
		a.logger.Error("failed to get tracking",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to get tracking: %w", err)
	}

	if !resp.Result.Success {
		return nil, &provider.ProviderError{
			Code:            MapErrorCode(resp.Result.ErrorCode),
			Message:         resp.Result.ErrorMessage,
			ProviderCode:    resp.Result.ErrorCode,
			ProviderMessage: resp.Result.ErrorMessage,
			Retryable:       IsRetryableError(resp.Result.ErrorCode),
		}
	}

	events := make([]provider.TrackingEventData, 0, len(resp.Result.Events))
	for _, e := range resp.Result.Events {
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
		CurrentStatus: MapStatus(resp.Result.StatusCode),
		Events:        events,
	}

	if resp.Result.EstimatedDelivery != "" {
		if estimated, err := time.Parse(time.RFC3339, resp.Result.EstimatedDelivery); err == nil {
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
			TrackingNumber:     webhookPayload.CargoKey,
			ProviderShipmentID: webhookPayload.InvoiceKey,
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

	soapReq := &SchedulePickupRequest{
		UserName:     creds.Username,
		Password:     creds.Password,
		CustomerCode: creds.CustomerCode,
		Language:     creds.Language,
		PickupAddress: AddressDetails{
			Name:       req.Address.ContactName,
			Phone:      formatPhone(req.Address.Phone),
			Address:    address,
			CityCode:   req.Address.CityCode,
			TownCode:   req.Address.DistrictCode,
			PostalCode: derefString(req.Address.PostalCode),
		},
		ScheduledDate:   req.ScheduledDate.Format("2006-01-02"),
		TimeWindowStart: req.TimeWindowStart,
		TimeWindowEnd:   req.TimeWindowEnd,
		ParcelCount:     req.ParcelCount,
		TotalWeight:     float64(req.TotalWeightGrams) / 1000.0,
		CargoKeys:       strings.Join(req.TrackingNumbers, ","),
		Instructions:    req.Instructions,
	}

	var resp SchedulePickupResponse
	if err := a.soapClient.Call(ctx, "SchedulePickup", soapReq, &resp); err != nil {
		a.logger.Error("failed to schedule pickup",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to schedule pickup: %w", err)
	}

	if !resp.Result.Success {
		return nil, &provider.ProviderError{
			Code:            MapErrorCode(resp.Result.ErrorCode),
			Message:         resp.Result.ErrorMessage,
			ProviderCode:    resp.Result.ErrorCode,
			ProviderMessage: resp.Result.ErrorMessage,
			Retryable:       IsRetryableError(resp.Result.ErrorCode),
		}
	}

	confirmedDate, _ := time.Parse("2006-01-02", resp.Result.ConfirmedDate)

	return &provider.SchedulePickupResponse{
		PickupID:        resp.Result.PickupID,
		ConfirmedDate:   confirmedDate,
		TimeWindowStart: resp.Result.TimeWindowStart,
		TimeWindowEnd:   resp.Result.TimeWindowEnd,
	}, nil
}

// CancelPickup cancels a scheduled pickup.
func (a *Adapter) CancelPickup(ctx context.Context, req provider.CancelPickupRequest) (*provider.CancelPickupResponse, error) {
	creds, err := a.getCredentials(req.Credentials)
	if err != nil {
		return nil, err
	}

	soapReq := &CancelPickupRequest{
		UserName:     creds.Username,
		Password:     creds.Password,
		CustomerCode: creds.CustomerCode,
		Language:     creds.Language,
		PickupID:     req.PickupID,
		Reason:       req.Reason,
	}

	var resp CancelPickupResponse
	if err := a.soapClient.Call(ctx, "CancelPickup", soapReq, &resp); err != nil {
		a.logger.Error("failed to cancel pickup",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to cancel pickup: %w", err)
	}

	if !resp.Result.Success {
		return nil, &provider.ProviderError{
			Code:            MapErrorCode(resp.Result.ErrorCode),
			Message:         resp.Result.ErrorMessage,
			ProviderCode:    resp.Result.ErrorCode,
			ProviderMessage: resp.Result.ErrorMessage,
			Retryable:       IsRetryableError(resp.Result.ErrorCode),
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

	soapReq := &CreateReturnRequest{
		UserName:         creds.Username,
		Password:         creds.Password,
		CustomerCode:     creds.CustomerCode,
		Language:         creds.Language,
		OriginalCargoKey: req.OriginalTrackingNumber,
		Shipment: ShipmentDetails{
			ProductCode: MapProductCode(req.ServiceCode),
			Reference:   req.Reference,
			LabelFormat: MapLabelFormat(req.LabelFormat),
			PaymentType: PaymentTypeSender,
			CargoType:   CargoTypeParcel,
			Sender: AddressDetails{
				Name:       req.Origin.ContactName,
				Phone:      formatPhone(req.Origin.Phone),
				Address:    originAddress,
				CityCode:   req.Origin.CityCode,
				TownCode:   req.Origin.DistrictCode,
				PostalCode: derefString(req.Origin.PostalCode),
			},
			Receiver: AddressDetails{
				Name:       req.Destination.ContactName,
				Phone:      formatPhone(req.Destination.Phone),
				Address:    destAddress,
				CityCode:   req.Destination.CityCode,
				TownCode:   req.Destination.DistrictCode,
				PostalCode: derefString(req.Destination.PostalCode),
			},
			Cargo: CargoDetails{
				PieceCount:  1,
				Weight:      float64(req.Parcel.WeightGrams) / 1000.0,
				Desi:        desi,
				Length:      float64(req.Parcel.LengthCm),
				Width:       float64(req.Parcel.WidthCm),
				Height:      float64(req.Parcel.HeightCm),
				Description: derefString(req.Parcel.Description),
			},
		},
	}

	var resp CreateReturnResponse
	if err := a.soapClient.Call(ctx, "CreateReturn", soapReq, &resp); err != nil {
		a.logger.Error("failed to create return",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to create return: %w", err)
	}

	if !resp.Result.Success {
		return nil, &provider.ProviderError{
			Code:            MapErrorCode(resp.Result.ErrorCode),
			Message:         resp.Result.ErrorMessage,
			ProviderCode:    resp.Result.ErrorCode,
			ProviderMessage: resp.Result.ErrorMessage,
			Retryable:       IsRetryableError(resp.Result.ErrorCode),
		}
	}

	result := &provider.CreateReturnResponse{
		ProviderShipmentID: resp.Result.ShipmentID,
		TrackingNumber:     resp.Result.TrackingNumber,
		Barcode:            resp.Result.Barcode,
	}

	if resp.Result.LabelData != "" {
		labelBytes, err := base64.StdEncoding.DecodeString(resp.Result.LabelData)
		if err == nil {
			result.Label = &provider.LabelData{
				Format:      MapLabelFormatFromYurtici(resp.Result.LabelFormat),
				Content:     labelBytes,
				ContentType: MapContentType(resp.Result.LabelFormat),
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

	soapReq := &GetDropoffPointsRequest{
		UserName:     creds.Username,
		Password:     creds.Password,
		CustomerCode: creds.CustomerCode,
		Language:     creds.Language,
		CityCode:     req.CityCode,
		TownCode:     req.DistrictCode,
		PostalCode:   req.PostalCode,
		Limit:        req.Limit,
	}

	var resp GetDropoffPointsResponse
	if err := a.soapClient.Call(ctx, "GetAgencies", soapReq, &resp); err != nil {
		a.logger.Error("failed to get dropoff points",
			slog.String("provider", string(ProviderCode)),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("failed to get dropoff points: %w", err)
	}

	if !resp.Result.Success {
		return nil, &provider.ProviderError{
			Code:            MapErrorCode(resp.Result.ErrorCode),
			Message:         resp.Result.ErrorMessage,
			ProviderCode:    resp.Result.ErrorCode,
			ProviderMessage: resp.Result.ErrorMessage,
			Retryable:       IsRetryableError(resp.Result.ErrorCode),
		}
	}

	points := make([]provider.DropoffPoint, 0, len(resp.Result.Agencies))
	for _, p := range resp.Result.Agencies {
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
			DistrictCode: p.TownCode,
			DistrictName: p.TownName,
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

// formatPhone formats a phone number for Yurtiçi (removes +90 prefix).
func formatPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")

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

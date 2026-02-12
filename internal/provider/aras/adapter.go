package aras

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/knid/kargonet/internal/domain"
	"github.com/knid/kargonet/internal/provider"
	"github.com/knid/kargonet/internal/provider/soap"
)

const (
	// ProviderCode is the unique identifier for Aras Kargo.
	ProviderCode = domain.ProviderCodeAras

	// DefaultEndpoint is the production .svc integration endpoint.
	DefaultEndpoint = "https://customerservices.araskargo.com.tr/ArasCargoCustomerIntegrationService/ArasCargoIntegrationService.svc"

	// SandboxEndpoint is the sandbox .svc integration endpoint.
	SandboxEndpoint = "http://customerservicestest.araskargo.com.tr/ArasCargoIntegrationService.svc"

	// DefaultTimeout is the default request timeout.
	DefaultTimeout = 30 * time.Second

	// SOAP actions for the integration service.
	soapActionGetQueryJSON = "http://tempuri.org/IArasCargoIntegrationService/GetQueryJSON"
	soapActionSetDataXML   = "http://tempuri.org/IArasCargoIntegrationService/SetDataXML"
)

// Config contains configuration for the Aras adapter.
type Config struct {
	// Endpoint is the .svc integration service URL.
	Endpoint string
	// SandboxEndpoint is the sandbox .svc URL.
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
	}
}

// Adapter implements the provider.Adapter interface for Aras Kargo.
// It uses the ArasCargoIntegrationService.svc unified SOAP 1.1 service.
type Adapter struct {
	provider.BaseAdapter
	config Config
	client *soap.Client
	logger *slog.Logger
}

// NewAdapter creates a new Aras adapter.
func NewAdapter(config Config, logger *slog.Logger) (*Adapter, error) {
	if logger == nil {
		logger = slog.Default()
	}

	endpoint := config.Endpoint
	if config.UseSandbox && config.SandboxEndpoint != "" {
		endpoint = config.SandboxEndpoint
	}

	var onRequest func(ctx context.Context, log soap.RequestLog)
	if config.OnAPIRequest != nil {
		onReq := config.OnAPIRequest
		onRequest = func(ctx context.Context, log soap.RequestLog) {
			onReq(ctx, string(ProviderCode), log.Action, log.Endpoint, log.RequestBody, log.ResponseBody, log.ResponseStatus, log.LatencyMs, log.Success, log.Error)
		}
	}

	clientConfig := soap.ClientConfig{
		Endpoint:    endpoint,
		Timeout:     config.Timeout,
		SOAPVersion: soap.SOAP11,
		Logger:      logger,
		OnRequest:   onRequest,
	}

	return &Adapter{
		BaseAdapter: provider.NewBaseAdapter(ProviderCode, domain.ProviderAPITypeSOAP),
		config:      config,
		client:      soap.NewClient(clientConfig),
		logger:      logger,
	}, nil
}

// NewAdapterFromProviderConfig creates an adapter from provider.AdapterConfig.
func NewAdapterFromProviderConfig(cfg provider.AdapterConfig, logger *slog.Logger) (provider.Adapter, error) {
	config := Config{
		Timeout:      cfg.Timeout,
		UseSandbox:   cfg.UseSandbox,
		Logger:       logger,
		OnAPIRequest: cfg.OnAPIRequest,
	}

	if cfg.BaseURL != "" {
		config.Endpoint = deriveEndpoint(cfg.BaseURL)
	}
	if cfg.SandboxURL != "" {
		config.SandboxEndpoint = deriveEndpoint(cfg.SandboxURL)
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

// deriveEndpoint constructs the full .svc endpoint from a base URL (scheme+host).
// Production and sandbox have different paths per the official Aras documentation.
func deriveEndpoint(baseURL string) string {
	schemeHost := extractSchemeHost(baseURL)
	if strings.Contains(schemeHost, "test") {
		return schemeHost + "/ArasCargoIntegrationService.svc"
	}
	return schemeHost + "/ArasCargoCustomerIntegrationService/ArasCargoIntegrationService.svc"
}

// extractSchemeHost returns the scheme://host portion of a URL.
func extractSchemeHost(u string) string {
	idx := strings.Index(u, "://")
	if idx == -1 {
		return u
	}
	afterScheme := u[idx+3:]
	slashIdx := strings.Index(afterScheme, "/")
	if slashIdx == -1 {
		return u
	}
	return u[:idx+3+slashIdx]
}

// ---------------------------------------------------------------------------
// Core helper — callQuery sends a GetQueryJSON SOAP request.
// ---------------------------------------------------------------------------

func (a *Adapter) callQuery(ctx context.Context, creds *Credentials, queryType int, params map[string]string) (json.RawMessage, error) {
	loginInfo := buildLoginInfoXML(creds)
	queryInfo := buildQueryInfoXML(queryType, params)

	req := &GetQueryJSONRequest{
		LoginInfo: loginInfo,
		QueryInfo: queryInfo,
	}

	var resp GetQueryJSONResponse
	if err := a.client.Call(ctx, soapActionGetQueryJSON, req, &resp); err != nil {
		return nil, err
	}

	if resp.Result == "" {
		return nil, nil
	}

	var qr QueryResponse
	if err := json.Unmarshal([]byte(resp.Result), &qr); err != nil {
		return nil, fmt.Errorf("failed to parse query result JSON: %w", err)
	}

	if len(qr.QueryResult) == 0 || string(qr.QueryResult) == "null" {
		return nil, nil
	}

	return qr.QueryResult, nil
}

// extractRows parses a QueryResult object and returns the first array value.
// The JSON structure is {"SomeKey": [{...}, {...}]} — the key name varies by query type.
func extractRows(raw json.RawMessage) ([]map[string]any, error) {
	if raw == nil {
		return nil, nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}

	for _, v := range obj {
		var rows []map[string]any
		if err := json.Unmarshal(v, &rows); err != nil {
			continue
		}
		return rows, nil
	}

	return nil, nil
}

// getStr extracts a string value from a row map, handling various JSON types.
func getStr(row map[string]any, key string) string {
	v, ok := row[key]
	if !ok || v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// buildLoginInfoXML builds the LoginInfo XML string for SOAP requests.
func buildLoginInfoXML(creds *Credentials) string {
	return fmt.Sprintf(
		"<LoginInfo><UserName>%s</UserName><Password>%s</Password><CustomerCode>%s</CustomerCode></LoginInfo>",
		creds.Username, creds.Password, creds.CustomerCode,
	)
}

// buildQueryInfoXML builds the QueryInfo XML string for SOAP requests.
func buildQueryInfoXML(queryType int, params map[string]string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("<QueryInfo><QueryType>%d</QueryType>", queryType))
	for k, v := range params {
		sb.WriteString(fmt.Sprintf("<%s>%s</%s>", k, v, k))
	}
	sb.WriteString("</QueryInfo>")
	return sb.String()
}

// ---------------------------------------------------------------------------
// getCredentials extracts Aras credentials from domain credentials.
// ---------------------------------------------------------------------------

func (a *Adapter) getCredentials(creds *domain.ProviderCredentials) (*Credentials, error) {
	if creds == nil {
		return nil, provider.ErrInvalidCredentials
	}

	c := &Credentials{
		Username:     creds.Username,
		Password:     creds.Password,
		CustomerCode: creds.CustomerCode,
	}

	if c.CustomerCode == "" && creds.Extra != nil {
		if code, ok := creds.Extra["customer_code"]; ok {
			c.CustomerCode = code
		}
	}

	if c.Username == "" || c.Password == "" {
		return nil, provider.ErrInvalidCredentials
	}

	return c, nil
}

// ---------------------------------------------------------------------------
// VerifyCredentials — QueryType 33 (customer representatives).
// If the call returns data without error, credentials are valid.
// ---------------------------------------------------------------------------

func (a *Adapter) VerifyCredentials(ctx context.Context, creds domain.ProviderCredentials) (*provider.VerificationResult, error) {
	c, err := a.getCredentials(&creds)
	if err != nil {
		return &provider.VerificationResult{
			Success: false,
			Error:   "Invalid credentials format",
		}, nil
	}

	result, err := a.callQuery(ctx, c, QueryTypeRepresentatives, nil)
	if err != nil {
		var provErr *provider.ProviderError
		if isProviderError(err, &provErr) {
			return &provider.VerificationResult{
				Success: false,
				Error:   provErr.Message,
			}, nil
		}
		return nil, fmt.Errorf("failed to verify credentials: %w", err)
	}

	_ = result // Success if no error
	return &provider.VerificationResult{
		Success: true,
		AccountInfo: &provider.AccountInfo{
			AccountNumber: c.CustomerCode,
		},
	}, nil
}

// ---------------------------------------------------------------------------
// GetQuote — static contract pricing (KDV hariç / VAT excluded).
// Aras has no pricing API; prices come from the fixed contract agreement.
// ---------------------------------------------------------------------------

// arasContractPrice returns the Aras contract price in kuruş (KDV hariç) for a given desi.
func arasContractPrice(desi int) int64 {
	if desi <= 0 {
		desi = 1
	}
	switch {
	case desi <= 1:
		return 9129
	case desi <= 2:
		return 10117
	case desi <= 3:
		return 11313
	case desi <= 4:
		return 12559
	case desi <= 5:
		return 13768
	case desi <= 10:
		return 17596
	case desi <= 15:
		return 21788
	case desi <= 20:
		return 27645
	case desi <= 25:
		return 33522
	case desi <= 30:
		return 39119
	default:
		extra := desi - 30
		return 39119 + int64(extra)*1099
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
	if desiInt <= 0 {
		desiInt = 1
	}

	basePriceKurus := arasContractPrice(desiInt)
	vatKurus := basePriceKurus * 20 / 100
	totalKurus := basePriceKurus + vatKurus

	return &provider.QuoteResponse{
		Offers: []provider.ProviderOffer{
			{
				ServiceCode:    "standard",
				ServiceName:    "Standart Kargo",
				BasePriceKurus: basePriceKurus,
				VATKurus:       vatKurus,
				TotalPriceKurus: totalKurus,
				EstimatedDaysMin: 2,
				EstimatedDaysMax: 4,
				ExpiresAt:        time.Now().Add(1 * time.Hour),
			},
		},
	}, nil
}

// ---------------------------------------------------------------------------
// CreateShipment — uses SetDataXML (TODO: implement when format is confirmed)
// ---------------------------------------------------------------------------

func (a *Adapter) CreateShipment(_ context.Context, _ provider.CreateShipmentRequest) (*provider.CreateShipmentResponse, error) {
	return nil, provider.ErrUnsupportedOperation
}

// ---------------------------------------------------------------------------
// CancelShipment — not available via .svc integration service
// ---------------------------------------------------------------------------

func (a *Adapter) CancelShipment(_ context.Context, _ provider.CancelShipmentRequest) (*provider.CancelShipmentResponse, error) {
	return nil, provider.ErrUnsupportedOperation
}

// ---------------------------------------------------------------------------
// GetLabel — not available via .svc integration service
// ---------------------------------------------------------------------------

func (a *Adapter) GetLabel(_ context.Context, _ provider.GetLabelRequest) (*provider.GetLabelResponse, error) {
	return nil, provider.ErrUnsupportedOperation
}

// ---------------------------------------------------------------------------
// GetTracking — QueryType 9 (cargo movement by tracking number)
// ---------------------------------------------------------------------------

func (a *Adapter) GetTracking(ctx context.Context, req provider.GetTrackingRequest) (*provider.GetTrackingResponse, error) {
	creds, err := a.getCredentials(req.Credentials)
	if err != nil {
		return nil, err
	}

	trackingNumber := req.TrackingNumber
	if trackingNumber == "" {
		trackingNumber = req.ProviderShipmentID
	}

	result, err := a.callQuery(ctx, creds, QueryTypeCargoMovement, map[string]string{
		"TrackingNumber": trackingNumber,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get tracking: %w", err)
	}

	rows, err := extractRows(result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse tracking response: %w", err)
	}

	events := make([]provider.TrackingEventData, 0, len(rows))
	var currentStatus domain.TrackingStatus

	for _, row := range rows {
		statusCode := getStr(row, "DURUM_KODU")
		transferCode := getStr(row, "DEVIR_KODU")
		status := MapStatusWithTransferCode(statusCode, transferCode)
		currentStatus = status

		occurredAt := parseArasDateTime(getStr(row, "ISLEM_TARIHI"))

		rawData, _ := json.Marshal(row)

		desc := getStr(row, "DURUM_ACIKLAMA")
		if desc == "" {
			desc = MapStatusDescription(statusCode)
		}

		events = append(events, provider.TrackingEventData{
			StatusCode:        status,
			StatusDescription: desc,
			LocationCity:      getStr(row, "SUBE_ADI"),
			LocationDetail:    getStr(row, "ISLEM_ACIKLAMA"),
			OccurredAt:        occurredAt,
			RawStatusCode:     statusCode,
			RawData:           rawData,
		})
	}

	if len(events) == 0 {
		currentStatus = domain.TrackingStatusUnknown
	}

	return &provider.GetTrackingResponse{
		CurrentStatus: currentStatus,
		Events:        events,
	}, nil
}

// ---------------------------------------------------------------------------
// ParseTrackingWebhook — JSON-based webhook
// ---------------------------------------------------------------------------

func (a *Adapter) ParseTrackingWebhook(_ context.Context, payload []byte) ([]provider.TrackingWebhookEvent, error) {
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

// ---------------------------------------------------------------------------
// SchedulePickup — not supported
// ---------------------------------------------------------------------------

func (a *Adapter) SchedulePickup(_ context.Context, _ provider.SchedulePickupRequest) (*provider.SchedulePickupResponse, error) {
	return nil, provider.ErrUnsupportedOperation
}

// ---------------------------------------------------------------------------
// CancelPickup — not supported
// ---------------------------------------------------------------------------

func (a *Adapter) CancelPickup(_ context.Context, _ provider.CancelPickupRequest) (*provider.CancelPickupResponse, error) {
	return nil, provider.ErrUnsupportedOperation
}

// ---------------------------------------------------------------------------
// CreateReturn — not supported
// ---------------------------------------------------------------------------

func (a *Adapter) CreateReturn(_ context.Context, _ provider.CreateReturnRequest) (*provider.CreateReturnResponse, error) {
	return nil, provider.ErrUnsupportedOperation
}

// ---------------------------------------------------------------------------
// GetDropoffPoints — QueryType 10 (all branches)
// ---------------------------------------------------------------------------

func (a *Adapter) GetDropoffPoints(ctx context.Context, req provider.GetDropoffPointsRequest) (*provider.GetDropoffPointsResponse, error) {
	creds, err := a.getCredentials(req.Credentials)
	if err != nil {
		return nil, err
	}

	result, err := a.callQuery(ctx, creds, QueryTypeAllBranches, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get branches: %w", err)
	}

	rows, err := extractRows(result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse branch list: %w", err)
	}

	points := make([]provider.DropoffPoint, 0, len(rows))
	for _, row := range rows {
		cityCode := getStr(row, "IL_KODU")
		districtCode := getStr(row, "ILCE_KODU")

		if req.CityCode != "" && cityCode != req.CityCode {
			continue
		}
		if req.DistrictCode != "" && districtCode != req.DistrictCode {
			continue
		}

		point := provider.DropoffPoint{
			ID:           getStr(row, "SUBE_KODU"),
			Name:         getStr(row, "SUBE_ADI"),
			Address:      getStr(row, "SUBE_ADRESI"),
			CityCode:     cityCode,
			DistrictCode: districtCode,
			DistrictName: getStr(row, "ILCE"),
			Phone:        getStr(row, "TELEFON"),
			IsActive:     true,
		}

		if coord := getStr(row, "KORDINAT"); coord != "" {
			parts := strings.SplitN(coord, ",", 2)
			if len(parts) == 2 {
				if lat := parseFloat(parts[0]); lat != 0 {
					point.Latitude = &lat
				}
				if lng := parseFloat(parts[1]); lng != 0 {
					point.Longitude = &lng
				}
			}
		}

		points = append(points, point)

		if req.Limit > 0 && len(points) >= req.Limit {
			break
		}
	}

	return &provider.GetDropoffPointsResponse{
		Points: points,
	}, nil
}

// ===========================================================================
// Helper functions
// ===========================================================================

// parseFloat parses a string to float64, returning 0 on error.
func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	s = strings.ReplaceAll(s, ",", ".")
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// parseArasDateTime parses Aras date/time strings in common formats.
func parseArasDateTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}

	formats := []string{
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"02.01.2006 15:04:05",
		"02.01.2006 15:04",
		"02-01-2006 15:04:05",
		"2006-01-02",
		time.RFC3339,
	}

	loc, _ := time.LoadLocation("Europe/Istanbul")
	if loc == nil {
		loc = time.UTC
	}

	for _, f := range formats {
		if t, err := time.ParseInLocation(f, s, loc); err == nil {
			return t
		}
	}

	return time.Time{}
}

// isProviderError checks if err is a *provider.ProviderError and sets target.
func isProviderError(err error, target **provider.ProviderError) bool {
	if err == nil {
		return false
	}
	pe, ok := err.(*provider.ProviderError)
	if ok {
		*target = pe
		return true
	}
	return false
}

// init registers the adapter with the global factory.
func init() {
	provider.RegisterAdapter(ProviderCode, NewAdapterFromProviderConfig)
}

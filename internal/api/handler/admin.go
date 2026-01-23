// Package handler provides HTTP handlers for the API.
package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/e-port/shipping-aggregator/internal/api/middleware"
	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres"
	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
	"github.com/e-port/shipping-aggregator/internal/service/tracking"
	"github.com/e-port/shipping-aggregator/internal/service/webhook"
)

// AdminHandler handles admin-related HTTP requests.
type AdminHandler struct {
	auditLogRepo    *postgres.AuditLogRepository
	providerLogRepo *postgres.ProviderAPILogRepository
	queries         *db.Queries
	trackingService *tracking.Service
	deliveryService *webhook.DeliveryService
	logger          *observability.Logger
}

// NewAdminHandler creates a new admin handler.
func NewAdminHandler(
	auditLogRepo *postgres.AuditLogRepository,
	providerLogRepo *postgres.ProviderAPILogRepository,
	queries *db.Queries,
	trackingService *tracking.Service,
	deliveryService *webhook.DeliveryService,
	logger *observability.Logger,
) *AdminHandler {
	return &AdminHandler{
		auditLogRepo:    auditLogRepo,
		providerLogRepo: providerLogRepo,
		queries:         queries,
		trackingService: trackingService,
		deliveryService: deliveryService,
		logger:          logger,
	}
}

// ============================================================================
// AUDIT LOG HANDLERS (T183)
// ============================================================================

// ListAuditLogs handles GET /admin/audit-logs
// Returns audit logs with filtering.
func (h *AdminHandler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context (optional for admin)
	orgID := middleware.GetOrganizationID(ctx)

	// Parse query parameters
	filter := &postgres.AuditLogFilter{
		Offset: int32(parseIntParam(r, "offset", 0)),
		Limit:  int32(parseIntParam(r, "limit", 20)),
	}

	// Organization filter (admin can see all, regular users see their own)
	if orgIDStr := r.URL.Query().Get("organization_id"); orgIDStr != "" {
		if id, err := uuid.Parse(orgIDStr); err == nil {
			filter.OrganizationID = &id
		}
	} else if orgID != uuid.Nil {
		filter.OrganizationID = &orgID
	}

	// Actor filter
	if actorIDStr := r.URL.Query().Get("actor_id"); actorIDStr != "" {
		if id, err := uuid.Parse(actorIDStr); err == nil {
			filter.ActorID = &id
		}
	}

	// Actor type filter
	if actorType := r.URL.Query().Get("actor_type"); actorType != "" {
		filter.ActorType = &actorType
	}

	// Action filter
	if action := r.URL.Query().Get("action"); action != "" {
		filter.Action = &action
	}

	// Resource type filter
	if resourceType := r.URL.Query().Get("resource_type"); resourceType != "" {
		filter.ResourceType = &resourceType
	}

	// Resource ID filter
	if resourceIDStr := r.URL.Query().Get("resource_id"); resourceIDStr != "" {
		if id, err := uuid.Parse(resourceIDStr); err == nil {
			filter.ResourceID = &id
		}
	}

	// Date range filters
	if createdAfter := r.URL.Query().Get("created_after"); createdAfter != "" {
		if t, err := time.Parse(time.RFC3339, createdAfter); err == nil {
			filter.CreatedAfter = &t
		}
	}
	if createdBefore := r.URL.Query().Get("created_before"); createdBefore != "" {
		if t, err := time.Parse(time.RFC3339, createdBefore); err == nil {
			filter.CreatedBefore = &t
		}
	}

	// Get audit logs
	logs, err := h.auditLogRepo.List(ctx, filter)
	if err != nil {
		h.logger.Error("Failed to list audit logs", "error", err)
		WriteError(w, r, NewInternalError("Failed to list audit logs"))
		return
	}

	// Get total count
	total, err := h.auditLogRepo.Count(ctx, filter)
	if err != nil {
		h.logger.Warn("Failed to count audit logs", "error", err)
		total = int64(len(logs))
	}

	// Build response
	response := make([]AuditLogResponse, len(logs))
	for i, log := range logs {
		response[i] = auditLogToResponse(log)
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:   response,
		Total:  total,
		Offset: int(filter.Offset),
		Limit:  int(filter.Limit),
	})
}

// GetAuditLog handles GET /admin/audit-logs/{id}
// Returns a specific audit log.
func (h *AdminHandler) GetAuditLog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get audit log ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid audit log ID"))
		return
	}

	// Get audit log
	log, err := h.auditLogRepo.GetByID(ctx, id)
	if err != nil {
		WriteError(w, r, NewNotFoundError("Audit log"))
		return
	}

	writeJSON(w, http.StatusOK, auditLogToResponse(log))
}

// GetAuditLogSummary handles GET /admin/audit-logs/summary
// Returns a summary of audit log activity.
func (h *AdminHandler) GetAuditLogSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context
	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	// Parse date range
	from := time.Now().AddDate(0, 0, -7) // Default: last 7 days
	to := time.Now()

	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = t
		}
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = t
		}
	}

	// Get summary
	summary, err := h.auditLogRepo.GetSummary(ctx, orgID, from, to)
	if err != nil {
		h.logger.Error("Failed to get audit log summary", "error", err)
		WriteError(w, r, NewInternalError("Failed to get audit log summary"))
		return
	}

	// Get actor activity
	activity, err := h.auditLogRepo.GetActorActivity(ctx, orgID, from, 10)
	if err != nil {
		h.logger.Warn("Failed to get actor activity", "error", err)
	}

	writeJSON(w, http.StatusOK, AuditLogSummaryResponse{
		Summary:       summary,
		ActorActivity: activity,
		From:          from.Format(time.RFC3339),
		To:            to.Format(time.RFC3339),
	})
}

// ============================================================================
// PROVIDER LOG HANDLERS (T184)
// ============================================================================

// ListProviderLogs handles GET /admin/provider-logs
// Returns provider API logs with filtering.
func (h *AdminHandler) ListProviderLogs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get organization ID from context (optional for admin)
	orgID := middleware.GetOrganizationID(ctx)

	// Parse query parameters
	filter := &postgres.ProviderAPILogFilter{
		Offset: int32(parseIntParam(r, "offset", 0)),
		Limit:  int32(parseIntParam(r, "limit", 20)),
	}

	// Organization filter
	if orgIDStr := r.URL.Query().Get("organization_id"); orgIDStr != "" {
		if id, err := uuid.Parse(orgIDStr); err == nil {
			filter.OrganizationID = &id
		}
	} else if orgID != uuid.Nil {
		filter.OrganizationID = &orgID
	}

	// Provider code filter
	if providerCode := r.URL.Query().Get("provider_code"); providerCode != "" {
		filter.ProviderCode = &providerCode
	}

	// Operation filter
	if operation := r.URL.Query().Get("operation"); operation != "" {
		filter.Operation = &operation
	}

	// Success filter
	if successStr := r.URL.Query().Get("success"); successStr != "" {
		success := successStr == "true"
		filter.Success = &success
	}

	// Date range filters
	if createdAfter := r.URL.Query().Get("created_after"); createdAfter != "" {
		if t, err := time.Parse(time.RFC3339, createdAfter); err == nil {
			filter.CreatedAfter = &t
		}
	}
	if createdBefore := r.URL.Query().Get("created_before"); createdBefore != "" {
		if t, err := time.Parse(time.RFC3339, createdBefore); err == nil {
			filter.CreatedBefore = &t
		}
	}

	// Get provider logs
	logs, err := h.providerLogRepo.List(ctx, filter)
	if err != nil {
		h.logger.Error("Failed to list provider logs", "error", err)
		WriteError(w, r, NewInternalError("Failed to list provider logs"))
		return
	}

	// Get total count
	total, err := h.providerLogRepo.Count(ctx, filter)
	if err != nil {
		h.logger.Warn("Failed to count provider logs", "error", err)
		total = int64(len(logs))
	}

	// Build response
	response := make([]ProviderLogResponse, len(logs))
	for i, log := range logs {
		response[i] = providerLogToResponse(log)
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:   response,
		Total:  total,
		Offset: int(filter.Offset),
		Limit:  int(filter.Limit),
	})
}

// GetProviderLog handles GET /admin/provider-logs/{id}
// Returns a specific provider API log.
func (h *AdminHandler) GetProviderLog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get provider log ID from URL
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid provider log ID"))
		return
	}

	// Get provider log
	log, err := h.providerLogRepo.GetByID(ctx, id)
	if err != nil {
		WriteError(w, r, NewNotFoundError("Provider log"))
		return
	}

	writeJSON(w, http.StatusOK, providerLogToResponse(log))
}

// ============================================================================
// PROVIDER HEALTH HANDLERS (T185)
// ============================================================================

// GetProviderHealth handles GET /admin/health/providers
// Returns health status for all providers.
func (h *AdminHandler) GetProviderHealth(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get time range for stats (default: last hour)
	from := time.Now().Add(-1 * time.Hour)
	to := time.Now()

	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = t
		}
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = t
		}
	}

	// Get provider stats
	stats, err := h.providerLogRepo.GetStats(ctx, from, to)
	if err != nil {
		h.logger.Error("Failed to get provider stats", "error", err)
		WriteError(w, r, NewInternalError("Failed to get provider health"))
		return
	}

	// Get error summary
	errorSummary, err := h.providerLogRepo.GetErrorSummary(ctx, from, 20)
	if err != nil {
		h.logger.Warn("Failed to get error summary", "error", err)
	}

	// Build response
	providerHealth := make([]ProviderHealthResponse, len(stats))
	for i, stat := range stats {
		var successRate float64
		if stat.TotalCalls > 0 {
			successRate = float64(stat.SuccessfulCalls) / float64(stat.TotalCalls) * 100
		}

		status := "healthy"
		if successRate < 90 {
			status = "degraded"
		}
		if successRate < 50 {
			status = "unhealthy"
		}

		providerHealth[i] = ProviderHealthResponse{
			ProviderCode:    stat.ProviderCode,
			Status:          status,
			TotalCalls:      stat.TotalCalls,
			SuccessfulCalls: stat.SuccessfulCalls,
			FailedCalls:     stat.FailedCalls,
			SuccessRate:     successRate,
			AvgLatencyMs:    stat.AvgLatencyMs,
			MaxLatencyMs:    stat.MaxLatencyMs,
			MinLatencyMs:    stat.MinLatencyMs,
		}
	}

	writeJSON(w, http.StatusOK, ProviderHealthSummaryResponse{
		Providers:    providerHealth,
		ErrorSummary: errorSummary,
		From:         from.Format(time.RFC3339),
		To:           to.Format(time.RFC3339),
	})
}

// GetProviderHealthDetail handles GET /admin/health/providers/{provider_code}
// Returns detailed health status for a specific provider.
func (h *AdminHandler) GetProviderHealthDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	providerCode := chi.URLParam(r, "provider_code")
	if providerCode == "" {
		WriteError(w, r, NewBadRequestError("Provider code is required"))
		return
	}

	// Get time range for stats (default: last hour)
	from := time.Now().Add(-1 * time.Hour)
	to := time.Now()

	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = t
		}
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = t
		}
	}

	// Get operation stats
	operationStats, err := h.providerLogRepo.GetStatsByOperation(ctx, providerCode, from, to)
	if err != nil {
		h.logger.Error("Failed to get provider operation stats", "error", err)
		WriteError(w, r, NewInternalError("Failed to get provider health detail"))
		return
	}

	// Get recent logs
	recentLogs, err := h.providerLogRepo.ListByProvider(ctx, providerCode, 0, 10)
	if err != nil {
		h.logger.Warn("Failed to get recent logs", "error", err)
	}

	// Build response
	recentLogResponses := make([]ProviderLogResponse, len(recentLogs))
	for i, log := range recentLogs {
		recentLogResponses[i] = providerLogToResponse(log)
	}

	writeJSON(w, http.StatusOK, ProviderHealthDetailResponse{
		ProviderCode:   providerCode,
		OperationStats: operationStats,
		RecentLogs:     recentLogResponses,
		From:           from.Format(time.RFC3339),
		To:             to.Format(time.RFC3339),
	})
}

// ============================================================================
// WEBHOOK DLQ HANDLERS (T186)
// ============================================================================

// ReprocessDLQRequest represents the request body for reprocessing DLQ items.
type ReprocessDLQRequest struct {
	DeliveryIDs []uuid.UUID `json:"delivery_ids,omitempty"`
	All         bool        `json:"all,omitempty"`
	MaxItems    int         `json:"max_items,omitempty"`
}

// ReprocessDLQ handles POST /admin/webhooks/dlq/reprocess
// Reprocesses failed webhook deliveries from the DLQ.
func (h *AdminHandler) ReprocessDLQ(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req ReprocessDLQRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Validate request
	if len(req.DeliveryIDs) == 0 && !req.All {
		WriteError(w, r, NewValidationError("Either delivery_ids or all=true must be specified", nil))
		return
	}

	var reprocessed, failed int
	var errors []string

	if req.All {
		// Reprocess all DLQ items
		maxItems := req.MaxItems
		if maxItems <= 0 {
			maxItems = 100
		}

		result, err := h.deliveryService.ReprocessAllDLQ(ctx, int32(maxItems))
		if err != nil {
			h.logger.Error("Failed to reprocess DLQ", "error", err)
			WriteError(w, r, NewInternalError("Failed to reprocess DLQ"))
			return
		}
		reprocessed = result.Reprocessed
		failed = result.Failed
		errors = result.Errors
	} else {
		// Reprocess specific deliveries
		for _, deliveryID := range req.DeliveryIDs {
			_, err := h.deliveryService.ReplayDelivery(ctx, deliveryID)
			if err != nil {
				failed++
				errors = append(errors, err.Error())
			} else {
				reprocessed++
			}
		}
	}

	writeJSON(w, http.StatusOK, ReprocessDLQResponse{
		Reprocessed: reprocessed,
		Failed:      failed,
		Errors:      errors,
	})
}

// ListDLQ handles GET /admin/webhooks/dlq
// Returns webhook deliveries in the DLQ.
func (h *AdminHandler) ListDLQ(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	offset := int32(parseIntParam(r, "offset", 0))
	limit := int32(parseIntParam(r, "limit", 20))

	// Get DLQ items
	deliveries, total, err := h.deliveryService.ListDLQ(ctx, offset, limit)
	if err != nil {
		h.logger.Error("Failed to list DLQ", "error", err)
		WriteError(w, r, NewInternalError("Failed to list DLQ"))
		return
	}

	// Build response
	response := make([]DLQItemResponse, len(deliveries))
	for i, d := range deliveries {
		response[i] = DLQItemResponse{
			ID:             d.ID,
			EventID:        d.EventID,
			SubscriptionID: d.SubscriptionID,
			EndpointURL:    d.EndpointUrl,
			Status:         d.Status,
			Attempts:       int(d.Attempts),
			ErrorMessage:   d.ErrorMessage,
			CreatedAt:      d.CreatedAt.Time.Format(time.RFC3339),
			UpdatedAt:      d.UpdatedAt.Time.Format(time.RFC3339),
		}
		if d.LastAttemptAt.Valid {
			t := d.LastAttemptAt.Time.Format(time.RFC3339)
			response[i].LastAttemptAt = &t
		}
	}

	writeJSON(w, http.StatusOK, PaginatedResponse{
		Data:   response,
		Total:  total,
		Offset: int(offset),
		Limit:  int(limit),
	})
}

// ============================================================================
// TRACKING REBUILD HANDLERS (T187)
// ============================================================================

// RebuildTrackingRequest represents the request body for rebuilding tracking.
type RebuildTrackingRequest struct {
	Force bool `json:"force,omitempty"`
}

// RebuildTracking handles POST /admin/shipments/{id}/rebuild-tracking
// Rebuilds tracking history for a shipment.
func (h *AdminHandler) RebuildTracking(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get shipment ID from URL
	shipmentIDStr := chi.URLParam(r, "id")
	shipmentID, err := uuid.Parse(shipmentIDStr)
	if err != nil {
		WriteError(w, r, NewBadRequestError("Invalid shipment ID"))
		return
	}

	var req RebuildTrackingRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			WriteError(w, r, NewBadRequestError("Invalid request body"))
			return
		}
	}

	// Rebuild tracking
	result, err := h.trackingService.RebuildTracking(ctx, shipmentID, req.Force)
	if err != nil {
		h.logger.Error("Failed to rebuild tracking", "error", err, "shipment_id", shipmentID)
		WriteError(w, r, NewInternalError("Failed to rebuild tracking"))
		return
	}

	writeJSON(w, http.StatusOK, RebuildTrackingResponse{
		ShipmentID:    shipmentID,
		EventsAdded:   result.EventsAdded,
		EventsUpdated: result.EventsUpdated,
		CurrentStatus: result.CurrentStatus,
		Message:       "Tracking rebuilt successfully",
	})
}

// ============================================================================
// POLL TRACKING HANDLERS (T188)
// ============================================================================

// PollTrackingRequest represents the request body for polling tracking.
type PollTrackingRequest struct {
	TrackingNumbers []string `json:"tracking_numbers,omitempty"`
	All             bool     `json:"all,omitempty"`
	MaxItems        int      `json:"max_items,omitempty"`
}

// PollTracking handles POST /integrations/{provider}/poll-tracking
// Polls tracking updates from a provider.
func (h *AdminHandler) PollTracking(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	providerCode := chi.URLParam(r, "provider")
	if providerCode == "" {
		WriteError(w, r, NewBadRequestError("Provider code is required"))
		return
	}

	var req PollTrackingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, r, NewBadRequestError("Invalid request body"))
		return
	}

	// Validate request
	if len(req.TrackingNumbers) == 0 && !req.All {
		WriteError(w, r, NewValidationError("Either tracking_numbers or all=true must be specified", nil))
		return
	}

	var result *tracking.PollResult
	var err error

	if req.All {
		// Poll all active shipments for this provider
		maxItems := req.MaxItems
		if maxItems <= 0 {
			maxItems = 100
		}
		result, err = h.trackingService.PollAllForProvider(ctx, providerCode, maxItems)
	} else {
		// Poll specific tracking numbers
		result, err = h.trackingService.PollTrackingNumbers(ctx, providerCode, req.TrackingNumbers)
	}

	if err != nil {
		h.logger.Error("Failed to poll tracking", "error", err, "provider", providerCode)
		WriteError(w, r, NewInternalError("Failed to poll tracking"))
		return
	}

	writeJSON(w, http.StatusOK, PollTrackingResponse{
		Provider: providerCode,
		Polled:   result.Polled,
		Updated:  result.Updated,
		Failed:   result.Failed,
		Errors:   result.Errors,
	})
}

// ============================================================================
// RESPONSE TYPES
// ============================================================================

// AuditLogResponse represents an audit log in API responses.
type AuditLogResponse struct {
	ID             uuid.UUID       `json:"id"`
	OrganizationID *uuid.UUID      `json:"organization_id,omitempty"`
	ActorID        *uuid.UUID      `json:"actor_id,omitempty"`
	ActorType      string          `json:"actor_type"`
	Action         string          `json:"action"`
	ResourceType   string          `json:"resource_type"`
	ResourceID     *uuid.UUID      `json:"resource_id,omitempty"`
	OldValue       json.RawMessage `json:"old_value,omitempty"`
	NewValue       json.RawMessage `json:"new_value,omitempty"`
	IPAddress      string          `json:"ip_address,omitempty"`
	UserAgent      *string         `json:"user_agent,omitempty"`
	TraceID        *string         `json:"trace_id,omitempty"`
	CreatedAt      string          `json:"created_at"`
}

// AuditLogSummaryResponse represents an audit log summary in API responses.
type AuditLogSummaryResponse struct {
	Summary       []*postgres.AuditLogSummary       `json:"summary"`
	ActorActivity []*postgres.AuditLogActorActivity `json:"actor_activity,omitempty"`
	From          string                            `json:"from"`
	To            string                            `json:"to"`
}

// ProviderLogResponse represents a provider API log in API responses.
type ProviderLogResponse struct {
	ID                  uuid.UUID       `json:"id"`
	OrganizationID      *uuid.UUID      `json:"organization_id,omitempty"`
	ProviderCode        string          `json:"provider_code"`
	Operation           string          `json:"operation"`
	RequestMethod       string          `json:"request_method"`
	RequestURL          string          `json:"request_url"`
	RequestHeaders      json.RawMessage `json:"request_headers,omitempty"`
	RequestBodyPreview  *string         `json:"request_body_preview,omitempty"`
	ResponseStatus      *int32          `json:"response_status,omitempty"`
	ResponseHeaders     json.RawMessage `json:"response_headers,omitempty"`
	ResponseBodyPreview *string         `json:"response_body_preview,omitempty"`
	LatencyMs           int32           `json:"latency_ms"`
	Success             bool            `json:"success"`
	ErrorCode           *string         `json:"error_code,omitempty"`
	ErrorMessage        *string         `json:"error_message,omitempty"`
	TraceID             *string         `json:"trace_id,omitempty"`
	CreatedAt           string          `json:"created_at"`
}

// ProviderHealthResponse represents provider health in API responses.
type ProviderHealthResponse struct {
	ProviderCode    string  `json:"provider_code"`
	Status          string  `json:"status"`
	TotalCalls      int64   `json:"total_calls"`
	SuccessfulCalls int64   `json:"successful_calls"`
	FailedCalls     int64   `json:"failed_calls"`
	SuccessRate     float64 `json:"success_rate"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
	MaxLatencyMs    int32   `json:"max_latency_ms"`
	MinLatencyMs    int32   `json:"min_latency_ms"`
}

// ProviderHealthSummaryResponse represents provider health summary in API responses.
type ProviderHealthSummaryResponse struct {
	Providers    []ProviderHealthResponse               `json:"providers"`
	ErrorSummary []*postgres.ProviderAPILogErrorSummary `json:"error_summary,omitempty"`
	From         string                                 `json:"from"`
	To           string                                 `json:"to"`
}

// ProviderHealthDetailResponse represents detailed provider health in API responses.
type ProviderHealthDetailResponse struct {
	ProviderCode   string                                   `json:"provider_code"`
	OperationStats []*postgres.ProviderAPILogOperationStats `json:"operation_stats"`
	RecentLogs     []ProviderLogResponse                    `json:"recent_logs"`
	From           string                                   `json:"from"`
	To             string                                   `json:"to"`
}

// ReprocessDLQResponse represents the response for reprocessing DLQ.
type ReprocessDLQResponse struct {
	Reprocessed int      `json:"reprocessed"`
	Failed      int      `json:"failed"`
	Errors      []string `json:"errors,omitempty"`
}

// DLQItemResponse represents a DLQ item in API responses.
type DLQItemResponse struct {
	ID             uuid.UUID `json:"id"`
	EventID        uuid.UUID `json:"event_id"`
	SubscriptionID uuid.UUID `json:"subscription_id"`
	EndpointURL    string    `json:"endpoint_url"`
	Status         string    `json:"status"`
	Attempts       int       `json:"attempts"`
	LastAttemptAt  *string   `json:"last_attempt_at,omitempty"`
	ErrorMessage   *string   `json:"error_message,omitempty"`
	CreatedAt      string    `json:"created_at"`
	UpdatedAt      string    `json:"updated_at"`
}

// RebuildTrackingResponse represents the response for rebuilding tracking.
type RebuildTrackingResponse struct {
	ShipmentID    uuid.UUID `json:"shipment_id"`
	EventsAdded   int       `json:"events_added"`
	EventsUpdated int       `json:"events_updated"`
	CurrentStatus string    `json:"current_status"`
	Message       string    `json:"message"`
}

// PollTrackingResponse represents the response for polling tracking.
type PollTrackingResponse struct {
	Provider string   `json:"provider"`
	Polled   int      `json:"polled"`
	Updated  int      `json:"updated"`
	Failed   int      `json:"failed"`
	Errors   []string `json:"errors,omitempty"`
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// parseIntParam parses an integer query parameter with a default value.
func parseIntParam(r *http.Request, name string, defaultValue int) int {
	if v := r.URL.Query().Get(name); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultValue
}

// auditLogToResponse converts an audit log to an API response.
func auditLogToResponse(log *postgres.AuditLog) AuditLogResponse {
	return AuditLogResponse{
		ID:             log.ID,
		OrganizationID: log.OrganizationID,
		ActorID:        log.ActorID,
		ActorType:      log.ActorType,
		Action:         log.Action,
		ResourceType:   log.ResourceType,
		ResourceID:     log.ResourceID,
		OldValue:       log.OldValue,
		NewValue:       log.NewValue,
		IPAddress:      log.IPAddress,
		UserAgent:      log.UserAgent,
		TraceID:        log.TraceID,
		CreatedAt:      log.CreatedAt.Format(time.RFC3339),
	}
}

// providerLogToResponse converts a provider API log to an API response.
func providerLogToResponse(log *postgres.ProviderAPILog) ProviderLogResponse {
	return ProviderLogResponse{
		ID:                  log.ID,
		OrganizationID:      log.OrganizationID,
		ProviderCode:        log.ProviderCode,
		Operation:           log.Operation,
		RequestMethod:       log.RequestMethod,
		RequestURL:          log.RequestURL,
		RequestHeaders:      log.RequestHeaders,
		RequestBodyPreview:  log.RequestBodyPreview,
		ResponseStatus:      log.ResponseStatus,
		ResponseHeaders:     log.ResponseHeaders,
		ResponseBodyPreview: log.ResponseBodyPreview,
		LatencyMs:           log.LatencyMs,
		Success:             log.Success,
		ErrorCode:           log.ErrorCode,
		ErrorMessage:        log.ErrorMessage,
		TraceID:             log.TraceID,
		CreatedAt:           log.CreatedAt.Format(time.RFC3339),
	}
}

// Package postgres provides PostgreSQL repository implementations.
package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/e-port/shipping-aggregator/internal/repository/postgres/db"
)

// ProviderAPILogRepository provides access to provider API log data.
type ProviderAPILogRepository struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

// NewProviderAPILogRepository creates a new provider API log repository.
func NewProviderAPILogRepository(pool *pgxpool.Pool, queries *db.Queries) *ProviderAPILogRepository {
	return &ProviderAPILogRepository{
		pool:    pool,
		queries: queries,
	}
}

// ProviderAPILog represents a provider API log entry.
type ProviderAPILog struct {
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
	CreatedAt           time.Time       `json:"created_at"`
}

// ProviderAPILogFilter represents filter options for listing provider API logs.
type ProviderAPILogFilter struct {
	OrganizationID *uuid.UUID
	ProviderCode   *string
	Operation      *string
	Success        *bool
	CreatedAfter   *time.Time
	CreatedBefore  *time.Time
	Offset         int32
	Limit          int32
}

// ProviderAPILogStats represents statistics for provider API calls.
type ProviderAPILogStats struct {
	ProviderCode    string  `json:"provider_code"`
	TotalCalls      int64   `json:"total_calls"`
	SuccessfulCalls int64   `json:"successful_calls"`
	FailedCalls     int64   `json:"failed_calls"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
	MaxLatencyMs    int32   `json:"max_latency_ms"`
	MinLatencyMs    int32   `json:"min_latency_ms"`
}

// ProviderAPILogOperationStats represents statistics for provider API operations.
type ProviderAPILogOperationStats struct {
	ProviderCode    string  `json:"provider_code"`
	Operation       string  `json:"operation"`
	TotalCalls      int64   `json:"total_calls"`
	SuccessfulCalls int64   `json:"successful_calls"`
	SuccessRate     float64 `json:"success_rate"`
	AvgLatencyMs    float64 `json:"avg_latency_ms"`
}

// ProviderAPILogErrorSummary represents a summary of provider API errors.
type ProviderAPILogErrorSummary struct {
	ProviderCode   string    `json:"provider_code"`
	ErrorCode      *string   `json:"error_code,omitempty"`
	ErrorCount     int64     `json:"error_count"`
	LastOccurrence time.Time `json:"last_occurrence"`
}

// CreateProviderAPILogParams represents parameters for creating a provider API log.
type CreateProviderAPILogParams struct {
	OrganizationID      *uuid.UUID
	ProviderCode        string
	Operation           string
	RequestMethod       string
	RequestURL          string
	RequestHeaders      json.RawMessage
	RequestBodyPreview  *string
	ResponseStatus      *int32
	ResponseHeaders     json.RawMessage
	ResponseBodyPreview *string
	LatencyMs           int32
	Success             bool
	ErrorCode           *string
	ErrorMessage        *string
	TraceID             *string
}

// Create creates a new provider API log entry.
func (r *ProviderAPILogRepository) Create(ctx context.Context, params *CreateProviderAPILogParams) (*ProviderAPILog, error) {
	var orgID pgtype.UUID
	if params.OrganizationID != nil {
		orgID = pgtype.UUID{Bytes: *params.OrganizationID, Valid: true}
	}

	dbLog, err := r.queries.CreateProviderApiLog(ctx, db.CreateProviderApiLogParams{
		OrganizationID:      orgID,
		ProviderCode:        params.ProviderCode,
		Operation:           params.Operation,
		RequestMethod:       params.RequestMethod,
		RequestUrl:          params.RequestURL,
		RequestHeaders:      params.RequestHeaders,
		RequestBodyPreview:  params.RequestBodyPreview,
		ResponseStatus:      params.ResponseStatus,
		ResponseHeaders:     params.ResponseHeaders,
		ResponseBodyPreview: params.ResponseBodyPreview,
		LatencyMs:           params.LatencyMs,
		Success:             params.Success,
		ErrorCode:           params.ErrorCode,
		ErrorMessage:        params.ErrorMessage,
		TraceID:             params.TraceID,
	})
	if err != nil {
		return nil, err
	}

	return dbProviderAPILogToModel(dbLog), nil
}

// GetByID retrieves a provider API log by ID.
func (r *ProviderAPILogRepository) GetByID(ctx context.Context, id uuid.UUID) (*ProviderAPILog, error) {
	dbLog, err := r.queries.GetProviderApiLogByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return dbProviderAPILogToModel(dbLog), nil
}

// List retrieves provider API logs with filtering.
func (r *ProviderAPILogRepository) List(ctx context.Context, filter *ProviderAPILogFilter) ([]*ProviderAPILog, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	var orgID pgtype.UUID
	if filter.OrganizationID != nil {
		orgID = pgtype.UUID{Bytes: *filter.OrganizationID, Valid: true}
	}

	var createdAfter, createdBefore pgtype.Timestamptz
	if filter.CreatedAfter != nil {
		createdAfter = pgtype.Timestamptz{Time: *filter.CreatedAfter, Valid: true}
	}
	if filter.CreatedBefore != nil {
		createdBefore = pgtype.Timestamptz{Time: *filter.CreatedBefore, Valid: true}
	}

	dbLogs, err := r.queries.ListProviderApiLogs(ctx, db.ListProviderApiLogsParams{
		OrganizationID: orgID,
		ProviderCode:   filter.ProviderCode,
		Operation:      filter.Operation,
		Success:        filter.Success,
		CreatedAfter:   createdAfter,
		CreatedBefore:  createdBefore,
		OffsetCount:    filter.Offset,
		LimitCount:     filter.Limit,
	})
	if err != nil {
		return nil, err
	}

	logs := make([]*ProviderAPILog, len(dbLogs))
	for i, dbLog := range dbLogs {
		logs[i] = dbProviderAPILogToModel(dbLog)
	}
	return logs, nil
}

// Count counts provider API logs with filtering.
func (r *ProviderAPILogRepository) Count(ctx context.Context, filter *ProviderAPILogFilter) (int64, error) {
	var orgID pgtype.UUID
	if filter.OrganizationID != nil {
		orgID = pgtype.UUID{Bytes: *filter.OrganizationID, Valid: true}
	}

	var createdAfter, createdBefore pgtype.Timestamptz
	if filter.CreatedAfter != nil {
		createdAfter = pgtype.Timestamptz{Time: *filter.CreatedAfter, Valid: true}
	}
	if filter.CreatedBefore != nil {
		createdBefore = pgtype.Timestamptz{Time: *filter.CreatedBefore, Valid: true}
	}

	return r.queries.CountProviderApiLogs(ctx, db.CountProviderApiLogsParams{
		OrganizationID: orgID,
		ProviderCode:   filter.ProviderCode,
		Operation:      filter.Operation,
		Success:        filter.Success,
		CreatedAfter:   createdAfter,
		CreatedBefore:  createdBefore,
	})
}

// ListByProvider retrieves provider API logs for a specific provider.
func (r *ProviderAPILogRepository) ListByProvider(ctx context.Context, providerCode string, offset, limit int32) ([]*ProviderAPILog, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	dbLogs, err := r.queries.ListProviderApiLogsByProvider(ctx, db.ListProviderApiLogsByProviderParams{
		ProviderCode: providerCode,
		OffsetCount:  offset,
		LimitCount:   limit,
	})
	if err != nil {
		return nil, err
	}

	logs := make([]*ProviderAPILog, len(dbLogs))
	for i, dbLog := range dbLogs {
		logs[i] = dbProviderAPILogToModel(dbLog)
	}
	return logs, nil
}

// ListByTraceID retrieves provider API logs for a specific trace ID.
func (r *ProviderAPILogRepository) ListByTraceID(ctx context.Context, traceID string) ([]*ProviderAPILog, error) {
	dbLogs, err := r.queries.ListProviderApiLogsByTraceID(ctx, &traceID)
	if err != nil {
		return nil, err
	}

	logs := make([]*ProviderAPILog, len(dbLogs))
	for i, dbLog := range dbLogs {
		logs[i] = dbProviderAPILogToModel(dbLog)
	}
	return logs, nil
}

// GetStats retrieves statistics for provider API calls.
func (r *ProviderAPILogRepository) GetStats(ctx context.Context, from, to time.Time) ([]*ProviderAPILogStats, error) {
	rows, err := r.queries.GetProviderApiLogStats(ctx, db.GetProviderApiLogStatsParams{
		CreatedAt:   pgtype.Timestamptz{Time: from, Valid: true},
		CreatedAt_2: pgtype.Timestamptz{Time: to, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	stats := make([]*ProviderAPILogStats, len(rows))
	for i, row := range rows {
		var avgLatency float64
		if row.AvgLatencyMs.Valid {
			f, _ := row.AvgLatencyMs.Float64Value()
			avgLatency = f.Float64
		}

		var maxLatency, minLatency int32
		if v, ok := row.MaxLatencyMs.(int32); ok {
			maxLatency = v
		}
		if v, ok := row.MinLatencyMs.(int32); ok {
			minLatency = v
		}

		stats[i] = &ProviderAPILogStats{
			ProviderCode:    row.ProviderCode,
			TotalCalls:      row.TotalCalls,
			SuccessfulCalls: row.SuccessfulCalls,
			FailedCalls:     row.FailedCalls,
			AvgLatencyMs:    avgLatency,
			MaxLatencyMs:    maxLatency,
			MinLatencyMs:    minLatency,
		}
	}
	return stats, nil
}

// GetStatsByOperation retrieves statistics for provider API operations.
func (r *ProviderAPILogRepository) GetStatsByOperation(ctx context.Context, providerCode string, from, to time.Time) ([]*ProviderAPILogOperationStats, error) {
	rows, err := r.queries.GetProviderApiLogStatsByOperation(ctx, db.GetProviderApiLogStatsByOperationParams{
		ProviderCode: providerCode,
		CreatedAt:    pgtype.Timestamptz{Time: from, Valid: true},
		CreatedAt_2:  pgtype.Timestamptz{Time: to, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	stats := make([]*ProviderAPILogOperationStats, len(rows))
	for i, row := range rows {
		var avgLatency, successRate float64
		if row.AvgLatencyMs.Valid {
			f, _ := row.AvgLatencyMs.Float64Value()
			avgLatency = f.Float64
		}
		if row.SuccessRate.Valid {
			f, _ := row.SuccessRate.Float64Value()
			successRate = f.Float64
		}

		stats[i] = &ProviderAPILogOperationStats{
			ProviderCode:    row.ProviderCode,
			Operation:       row.Operation,
			TotalCalls:      row.TotalCalls,
			SuccessfulCalls: row.SuccessfulCalls,
			SuccessRate:     successRate,
			AvgLatencyMs:    avgLatency,
		}
	}
	return stats, nil
}

// GetErrorSummary retrieves a summary of provider API errors.
func (r *ProviderAPILogRepository) GetErrorSummary(ctx context.Context, since time.Time, limit int32) ([]*ProviderAPILogErrorSummary, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	rows, err := r.queries.GetProviderApiLogErrorSummary(ctx, db.GetProviderApiLogErrorSummaryParams{
		CreatedAt:  pgtype.Timestamptz{Time: since, Valid: true},
		LimitCount: limit,
	})
	if err != nil {
		return nil, err
	}

	summaries := make([]*ProviderAPILogErrorSummary, len(rows))
	for i, row := range rows {
		var lastOccurrence time.Time
		if t, ok := row.LastOccurrence.(time.Time); ok {
			lastOccurrence = t
		}

		summaries[i] = &ProviderAPILogErrorSummary{
			ProviderCode:   row.ProviderCode,
			ErrorCode:      row.ErrorCode,
			ErrorCount:     row.ErrorCount,
			LastOccurrence: lastOccurrence,
		}
	}
	return summaries, nil
}

// DeleteOld deletes provider API logs older than the retention period.
func (r *ProviderAPILogRepository) DeleteOld(ctx context.Context) error {
	return r.queries.DeleteOldProviderApiLogs(ctx)
}

// dbProviderAPILogToModel converts a database provider API log to a model.
func dbProviderAPILogToModel(dbLog *db.ProviderApiLog) *ProviderAPILog {
	log := &ProviderAPILog{
		ID:                  dbLog.ID,
		ProviderCode:        dbLog.ProviderCode,
		Operation:           dbLog.Operation,
		RequestMethod:       dbLog.RequestMethod,
		RequestURL:          dbLog.RequestUrl,
		RequestHeaders:      dbLog.RequestHeaders,
		RequestBodyPreview:  dbLog.RequestBodyPreview,
		ResponseStatus:      dbLog.ResponseStatus,
		ResponseHeaders:     dbLog.ResponseHeaders,
		ResponseBodyPreview: dbLog.ResponseBodyPreview,
		LatencyMs:           dbLog.LatencyMs,
		Success:             dbLog.Success,
		ErrorCode:           dbLog.ErrorCode,
		ErrorMessage:        dbLog.ErrorMessage,
		TraceID:             dbLog.TraceID,
		CreatedAt:           dbLog.CreatedAt.Time,
	}

	if dbLog.OrganizationID.Valid {
		id := uuid.UUID(dbLog.OrganizationID.Bytes)
		log.OrganizationID = &id
	}

	return log
}

// Package postgres provides PostgreSQL repository implementations.
package postgres

import (
	"context"
	"encoding/json"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/knid/kargonet/internal/repository/postgres/db"
)

// AuditLogRepository provides access to audit log data.
type AuditLogRepository struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

// NewAuditLogRepository creates a new audit log repository.
func NewAuditLogRepository(pool *pgxpool.Pool, queries *db.Queries) *AuditLogRepository {
	return &AuditLogRepository{
		pool:    pool,
		queries: queries,
	}
}

// AuditLog represents an audit log entry.
type AuditLog struct {
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
	CreatedAt      time.Time       `json:"created_at"`
}

// AuditLogFilter represents filter options for listing audit logs.
type AuditLogFilter struct {
	OrganizationID *uuid.UUID
	ActorID        *uuid.UUID
	ActorType      *string
	Action         *string
	ResourceType   *string
	ResourceID     *uuid.UUID
	CreatedAfter   *time.Time
	CreatedBefore  *time.Time
	Offset         int32
	Limit          int32
}

// AuditLogSummary represents a summary of audit log activity.
type AuditLogSummary struct {
	Action       string `json:"action"`
	ResourceType string `json:"resource_type"`
	Count        int64  `json:"count"`
}

// AuditLogActorActivity represents activity by an actor.
type AuditLogActorActivity struct {
	ActorID      *uuid.UUID `json:"actor_id,omitempty"`
	ActorType    string     `json:"actor_type"`
	ActionCount  int64      `json:"action_count"`
	LastActivity time.Time  `json:"last_activity"`
}

// CreateAuditLogParams represents parameters for creating an audit log.
type CreateAuditLogParams struct {
	OrganizationID *uuid.UUID
	ActorID        *uuid.UUID
	ActorType      string
	Action         string
	ResourceType   string
	ResourceID     *uuid.UUID
	OldValue       json.RawMessage
	NewValue       json.RawMessage
	IPAddress      string
	UserAgent      *string
	TraceID        *string
}

// Create creates a new audit log entry.
func (r *AuditLogRepository) Create(ctx context.Context, params *CreateAuditLogParams) (*AuditLog, error) {
	// Parse IP address
	var ipAddr netip.Addr
	if params.IPAddress != "" {
		var err error
		ipAddr, err = netip.ParseAddr(params.IPAddress)
		if err != nil {
			// Use zero address if parsing fails
			ipAddr = netip.Addr{}
		}
	}

	// Convert UUIDs to pgtype
	var orgID, actorID, resourceID pgtype.UUID
	if params.OrganizationID != nil {
		orgID = pgtype.UUID{Bytes: *params.OrganizationID, Valid: true}
	}
	if params.ActorID != nil {
		actorID = pgtype.UUID{Bytes: *params.ActorID, Valid: true}
	}
	if params.ResourceID != nil {
		resourceID = pgtype.UUID{Bytes: *params.ResourceID, Valid: true}
	}

	dbLog, err := r.queries.CreateAuditLog(ctx, db.CreateAuditLogParams{
		OrganizationID: orgID,
		ActorID:        actorID,
		ActorType:      params.ActorType,
		Action:         params.Action,
		ResourceType:   params.ResourceType,
		ResourceID:     resourceID,
		OldValue:       params.OldValue,
		NewValue:       params.NewValue,
		IpAddress:      ipAddr,
		UserAgent:      params.UserAgent,
		TraceID:        params.TraceID,
	})
	if err != nil {
		return nil, err
	}

	return dbAuditLogToModel(dbLog), nil
}

// GetByID retrieves an audit log by ID.
func (r *AuditLogRepository) GetByID(ctx context.Context, id uuid.UUID) (*AuditLog, error) {
	dbLog, err := r.queries.GetAuditLogByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return dbAuditLogToModel(dbLog), nil
}

// List retrieves audit logs with filtering.
func (r *AuditLogRepository) List(ctx context.Context, filter *AuditLogFilter) ([]*AuditLog, error) {
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	// Convert filter to query params
	var orgID, actorID, resourceID pgtype.UUID
	if filter.OrganizationID != nil {
		orgID = pgtype.UUID{Bytes: *filter.OrganizationID, Valid: true}
	}
	if filter.ActorID != nil {
		actorID = pgtype.UUID{Bytes: *filter.ActorID, Valid: true}
	}
	if filter.ResourceID != nil {
		resourceID = pgtype.UUID{Bytes: *filter.ResourceID, Valid: true}
	}

	var createdAfter, createdBefore pgtype.Timestamptz
	if filter.CreatedAfter != nil {
		createdAfter = pgtype.Timestamptz{Time: *filter.CreatedAfter, Valid: true}
	}
	if filter.CreatedBefore != nil {
		createdBefore = pgtype.Timestamptz{Time: *filter.CreatedBefore, Valid: true}
	}

	dbLogs, err := r.queries.ListAuditLogsByOrganization(ctx, db.ListAuditLogsByOrganizationParams{
		OrganizationID: orgID,
		ActorID:        actorID,
		ActorType:      filter.ActorType,
		Action:         filter.Action,
		ResourceType:   filter.ResourceType,
		ResourceID:     resourceID,
		CreatedAfter:   createdAfter,
		CreatedBefore:  createdBefore,
		OffsetCount:    filter.Offset,
		LimitCount:     filter.Limit,
	})
	if err != nil {
		return nil, err
	}

	logs := make([]*AuditLog, len(dbLogs))
	for i, dbLog := range dbLogs {
		logs[i] = dbAuditLogToModel(dbLog)
	}
	return logs, nil
}

// Count counts audit logs with filtering.
func (r *AuditLogRepository) Count(ctx context.Context, filter *AuditLogFilter) (int64, error) {
	// Convert filter to query params
	var orgID, actorID, resourceID pgtype.UUID
	if filter.OrganizationID != nil {
		orgID = pgtype.UUID{Bytes: *filter.OrganizationID, Valid: true}
	}
	if filter.ActorID != nil {
		actorID = pgtype.UUID{Bytes: *filter.ActorID, Valid: true}
	}
	if filter.ResourceID != nil {
		resourceID = pgtype.UUID{Bytes: *filter.ResourceID, Valid: true}
	}

	var createdAfter, createdBefore pgtype.Timestamptz
	if filter.CreatedAfter != nil {
		createdAfter = pgtype.Timestamptz{Time: *filter.CreatedAfter, Valid: true}
	}
	if filter.CreatedBefore != nil {
		createdBefore = pgtype.Timestamptz{Time: *filter.CreatedBefore, Valid: true}
	}

	return r.queries.CountAuditLogsByOrganization(ctx, db.CountAuditLogsByOrganizationParams{
		OrganizationID: orgID,
		ActorID:        actorID,
		ActorType:      filter.ActorType,
		Action:         filter.Action,
		ResourceType:   filter.ResourceType,
		ResourceID:     resourceID,
		CreatedAfter:   createdAfter,
		CreatedBefore:  createdBefore,
	})
}

// ListByResource retrieves audit logs for a specific resource.
func (r *AuditLogRepository) ListByResource(ctx context.Context, resourceType string, resourceID uuid.UUID, offset, limit int32) ([]*AuditLog, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	dbLogs, err := r.queries.ListAuditLogsByResource(ctx, db.ListAuditLogsByResourceParams{
		ResourceType: resourceType,
		ResourceID:   pgtype.UUID{Bytes: resourceID, Valid: true},
		OffsetCount:  offset,
		LimitCount:   limit,
	})
	if err != nil {
		return nil, err
	}

	logs := make([]*AuditLog, len(dbLogs))
	for i, dbLog := range dbLogs {
		logs[i] = dbAuditLogToModel(dbLog)
	}
	return logs, nil
}

// ListByActor retrieves audit logs for a specific actor.
func (r *AuditLogRepository) ListByActor(ctx context.Context, actorID uuid.UUID, actorType string, offset, limit int32) ([]*AuditLog, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	dbLogs, err := r.queries.ListAuditLogsByActor(ctx, db.ListAuditLogsByActorParams{
		ActorID:     pgtype.UUID{Bytes: actorID, Valid: true},
		ActorType:   actorType,
		OffsetCount: offset,
		LimitCount:  limit,
	})
	if err != nil {
		return nil, err
	}

	logs := make([]*AuditLog, len(dbLogs))
	for i, dbLog := range dbLogs {
		logs[i] = dbAuditLogToModel(dbLog)
	}
	return logs, nil
}

// ListByTraceID retrieves audit logs for a specific trace ID.
func (r *AuditLogRepository) ListByTraceID(ctx context.Context, traceID string) ([]*AuditLog, error) {
	dbLogs, err := r.queries.ListAuditLogsByTraceID(ctx, &traceID)
	if err != nil {
		return nil, err
	}

	logs := make([]*AuditLog, len(dbLogs))
	for i, dbLog := range dbLogs {
		logs[i] = dbAuditLogToModel(dbLog)
	}
	return logs, nil
}

// GetSummary retrieves a summary of audit log activity for an organization.
func (r *AuditLogRepository) GetSummary(ctx context.Context, organizationID uuid.UUID, from, to time.Time) ([]*AuditLogSummary, error) {
	rows, err := r.queries.GetAuditLogSummaryByOrganization(ctx, db.GetAuditLogSummaryByOrganizationParams{
		OrganizationID: pgtype.UUID{Bytes: organizationID, Valid: true},
		CreatedAt:      pgtype.Timestamptz{Time: from, Valid: true},
		CreatedAt_2:    pgtype.Timestamptz{Time: to, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	summaries := make([]*AuditLogSummary, len(rows))
	for i, row := range rows {
		summaries[i] = &AuditLogSummary{
			Action:       row.Action,
			ResourceType: row.ResourceType,
			Count:        row.Count,
		}
	}
	return summaries, nil
}

// GetActorActivity retrieves activity by actors for an organization.
func (r *AuditLogRepository) GetActorActivity(ctx context.Context, organizationID uuid.UUID, since time.Time, limit int32) ([]*AuditLogActorActivity, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	rows, err := r.queries.GetAuditLogActivityByActor(ctx, db.GetAuditLogActivityByActorParams{
		OrganizationID: pgtype.UUID{Bytes: organizationID, Valid: true},
		CreatedAt:      pgtype.Timestamptz{Time: since, Valid: true},
		LimitCount:     limit,
	})
	if err != nil {
		return nil, err
	}

	activities := make([]*AuditLogActorActivity, len(rows))
	for i, row := range rows {
		var actorID *uuid.UUID
		if row.ActorID.Valid {
			id := uuid.UUID(row.ActorID.Bytes)
			actorID = &id
		}

		var lastActivity time.Time
		if t, ok := row.LastActivity.(time.Time); ok {
			lastActivity = t
		}

		activities[i] = &AuditLogActorActivity{
			ActorID:      actorID,
			ActorType:    row.ActorType,
			ActionCount:  row.ActionCount,
			LastActivity: lastActivity,
		}
	}
	return activities, nil
}

// DeleteOld deletes audit logs older than the retention period.
func (r *AuditLogRepository) DeleteOld(ctx context.Context) error {
	return r.queries.DeleteOldAuditLogs(ctx)
}

// dbAuditLogToModel converts a database audit log to a model.
func dbAuditLogToModel(dbLog *db.AuditLog) *AuditLog {
	log := &AuditLog{
		ID:           dbLog.ID,
		ActorType:    dbLog.ActorType,
		Action:       dbLog.Action,
		ResourceType: dbLog.ResourceType,
		OldValue:     dbLog.OldValue,
		NewValue:     dbLog.NewValue,
		UserAgent:    dbLog.UserAgent,
		TraceID:      dbLog.TraceID,
		CreatedAt:    dbLog.CreatedAt.Time,
	}

	if dbLog.OrganizationID.Valid {
		id := uuid.UUID(dbLog.OrganizationID.Bytes)
		log.OrganizationID = &id
	}
	if dbLog.ActorID.Valid {
		id := uuid.UUID(dbLog.ActorID.Bytes)
		log.ActorID = &id
	}
	if dbLog.ResourceID.Valid {
		id := uuid.UUID(dbLog.ResourceID.Bytes)
		log.ResourceID = &id
	}
	if dbLog.IpAddress.IsValid() {
		log.IPAddress = dbLog.IpAddress.String()
	}

	return log
}

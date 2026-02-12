package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/knid/kargonet/internal/api/middleware"
	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/repository/postgres/db"
)

// StatsHandler handles statistics-related HTTP requests.
type StatsHandler struct {
	pool    *pgxpool.Pool
	queries *db.Queries
	logger  *observability.Logger
}

// NewStatsHandler creates a new stats handler.
func NewStatsHandler(pool *pgxpool.Pool, queries *db.Queries, logger *observability.Logger) *StatsHandler {
	return &StatsHandler{
		pool:    pool,
		queries: queries,
		logger:  logger,
	}
}

// GetShipmentStats handles GET /stats/shipments
func (h *StatsHandler) GetShipmentStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	dateFrom, dateTo := parseDateRange(r)

	stats, err := h.queries.GetShipmentStats(ctx, db.GetShipmentStatsParams{
		OrganizationID: orgID,
		DateFrom:       dateFrom,
		DateTo:         dateTo,
	})
	if err != nil {
		h.logger.Error("Failed to get shipment stats", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to get shipment stats"))
		return
	}

	byProvider, err := h.queries.GetShipmentStatsByProvider(ctx, db.GetShipmentStatsByProviderParams{
		OrganizationID: orgID,
		DateFrom:       dateFrom,
		DateTo:         dateTo,
	})
	if err != nil {
		h.logger.Error("Failed to get shipment stats by provider", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to get shipment stats"))
		return
	}

	byStatus := map[string]int64{
		"draft":           stats.DraftCount,
		"label_purchased": stats.LabelPurchasedCount,
		"in_transit":      stats.InTransitCount,
		"delivered":       stats.DeliveredCount,
		"cancelled":       stats.CancelledCount,
		"returned":        stats.ReturnedCount,
	}

	providerMap := make(map[string]int64, len(byProvider))
	for _, p := range byProvider {
		providerMap[p.ProviderCode] = p.ShipmentCount
	}

	periodStart, periodEnd := formatPeriod(dateFrom, dateTo)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total_shipments": stats.TotalShipments,
		"by_status":       byStatus,
		"by_provider":     providerMap,
		"total_spent":     formatKurus(stats.TotalSpentKurus),
		"period_start":    periodStart,
		"period_end":      periodEnd,
	})
}

// GetBillingStats handles GET /stats/billing
func (h *StatsHandler) GetBillingStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	orgID := middleware.GetOrganizationID(ctx)
	if orgID == uuid.Nil {
		WriteError(w, r, NewUnauthorizedError("Organization context not found"))
		return
	}

	dateFrom, dateTo := parseDateRange(r)

	billingStats, err := h.queries.GetBillingStats(ctx, db.GetBillingStatsParams{
		OrganizationID: orgID,
		DateFrom:       dateFrom,
		DateTo:         dateTo,
	})
	if err != nil {
		h.logger.Error("Failed to get billing stats", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to get billing stats"))
		return
	}

	pendingCOD, err := h.queries.GetPendingCODTotal(ctx, orgID)
	if err != nil {
		h.logger.Error("Failed to get pending COD total", "error", err, "org_id", orgID)
		WriteError(w, r, NewInternalError("Failed to get billing stats"))
		return
	}

	// Current balance = total credits - total debits
	currentBalance := billingStats.TotalCreditsKurus - billingStats.TotalDebitsKurus

	periodStart, periodEnd := formatPeriod(dateFrom, dateTo)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"current_balance": formatKurus(currentBalance),
		"total_topups":    formatKurus(billingStats.TotalCreditsKurus),
		"total_spent":     formatKurus(billingStats.TotalDebitsKurus),
		"pending_cod":     formatKurus(pendingCOD),
		"period_start":    periodStart,
		"period_end":      periodEnd,
	})
}

// parseDateRange extracts date_from and date_to from query params.
func parseDateRange(r *http.Request) (pgtype.Timestamptz, pgtype.Timestamptz) {
	var dateFrom, dateTo pgtype.Timestamptz

	if v := r.URL.Query().Get("date_from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			dateFrom = pgtype.Timestamptz{Time: t, Valid: true}
		} else if t, err := time.Parse("2006-01-02", v); err == nil {
			dateFrom = pgtype.Timestamptz{Time: t, Valid: true}
		}
	}
	if v := r.URL.Query().Get("date_to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			dateTo = pgtype.Timestamptz{Time: t, Valid: true}
		} else if t, err := time.Parse("2006-01-02", v); err == nil {
			dateTo = pgtype.Timestamptz{Time: t.Add(24*time.Hour - time.Second), Valid: true}
		}
	}

	return dateFrom, dateTo
}

// formatPeriod returns ISO date strings for the period.
func formatPeriod(dateFrom, dateTo pgtype.Timestamptz) (string, string) {
	start := time.Now().AddDate(0, -1, 0).Format(time.RFC3339)
	end := time.Now().Format(time.RFC3339)
	if dateFrom.Valid {
		start = dateFrom.Time.Format(time.RFC3339)
	}
	if dateTo.Valid {
		end = dateTo.Time.Format(time.RFC3339)
	}
	return start, end
}

// formatKurus converts kurus (integer cents) to a decimal string like "123.45".
func formatKurus(kurus int64) string {
	return fmt.Sprintf("%.2f", float64(kurus)/100.0)
}

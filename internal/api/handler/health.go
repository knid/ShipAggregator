// Package handler provides HTTP handlers for the shipping aggregator API.
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/repository/redis"
)

// HealthStatus represents the health status of a component.
type HealthStatus string

const (
	// HealthStatusHealthy indicates the component is healthy.
	HealthStatusHealthy HealthStatus = "healthy"
	// HealthStatusUnhealthy indicates the component is unhealthy.
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	// HealthStatusDegraded indicates the component is degraded but functional.
	HealthStatusDegraded HealthStatus = "degraded"
)

// ComponentHealth represents the health of a single component.
type ComponentHealth struct {
	// Status is the health status of the component.
	Status HealthStatus `json:"status"`
	// Message provides additional information about the health status.
	Message string `json:"message,omitempty"`
	// Latency is the time taken to check the component health.
	Latency string `json:"latency,omitempty"`
	// Details contains additional component-specific details.
	Details map[string]interface{} `json:"details,omitempty"`
}

// LiveResponse represents the response for the liveness probe.
type LiveResponse struct {
	// Status is the overall liveness status.
	Status HealthStatus `json:"status"`
	// Timestamp is when the check was performed.
	Timestamp string `json:"timestamp"`
}

// ReadyResponse represents the response for the readiness probe.
type ReadyResponse struct {
	// Status is the overall readiness status.
	Status HealthStatus `json:"status"`
	// Timestamp is when the check was performed.
	Timestamp string `json:"timestamp"`
	// Components contains the health status of each component.
	Components map[string]ComponentHealth `json:"components"`
	// Version contains version information.
	Version VersionInfo `json:"version,omitempty"`
}

// VersionInfo contains version information about the service.
type VersionInfo struct {
	// Version is the application version.
	Version string `json:"version"`
	// Commit is the git commit hash.
	Commit string `json:"commit,omitempty"`
	// BuildTime is when the application was built.
	BuildTime string `json:"build_time,omitempty"`
	// GoVersion is the Go version used to build.
	GoVersion string `json:"go_version"`
}

// Build information - set at compile time.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// HealthHandler handles health check endpoints.
type HealthHandler struct {
	dbPool *pgxpool.Pool
	cache  *redis.Cache
	logger *observability.Logger
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler(dbPool *pgxpool.Pool, cache *redis.Cache, logger *observability.Logger) *HealthHandler {
	return &HealthHandler{
		dbPool: dbPool,
		cache:  cache,
		logger: logger,
	}
}

// Live handles the liveness probe endpoint.
// This endpoint returns 200 OK if the service is running.
// It does not check dependencies - that's what the readiness probe is for.
// GET /health/live
func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	response := LiveResponse{
		Status:    HealthStatusHealthy,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// Ready handles the readiness probe endpoint.
// This endpoint checks all dependencies and returns their health status.
// GET /health/ready
func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Check all components in parallel
	components := make(map[string]ComponentHealth)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Check database
	wg.Add(1)
	go func() {
		defer wg.Done()
		health := h.checkDatabase(ctx)
		mu.Lock()
		components["database"] = health
		mu.Unlock()
	}()

	// Check Redis cache
	wg.Add(1)
	go func() {
		defer wg.Done()
		health := h.checkRedis(ctx)
		mu.Lock()
		components["redis"] = health
		mu.Unlock()
	}()

	// Wait for all checks to complete
	wg.Wait()

	// Determine overall status
	overallStatus := HealthStatusHealthy
	for _, health := range components {
		if health.Status == HealthStatusUnhealthy {
			overallStatus = HealthStatusUnhealthy
			break
		}
		if health.Status == HealthStatusDegraded {
			overallStatus = HealthStatusDegraded
		}
	}

	response := ReadyResponse{
		Status:     overallStatus,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		Components: components,
		Version: VersionInfo{
			Version:   Version,
			Commit:    Commit,
			BuildTime: BuildTime,
			GoVersion: runtime.Version(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	if overallStatus == HealthStatusHealthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	_ = json.NewEncoder(w).Encode(response)
}

// checkDatabase checks the database connection health.
func (h *HealthHandler) checkDatabase(ctx context.Context) ComponentHealth {
	if h.dbPool == nil {
		return ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Message: "database pool not configured",
		}
	}

	start := time.Now()

	// Try to ping the database
	err := h.dbPool.Ping(ctx)
	latency := time.Since(start)

	if err != nil {
		if h.logger != nil {
			h.logger.Error("database health check failed",
				"error", err.Error(),
				"latency_ms", latency.Milliseconds(),
			)
		}
		return ComponentHealth{
			Status:  HealthStatusUnhealthy,
			Message: "database connection failed: " + err.Error(),
			Latency: latency.String(),
		}
	}

	// Get pool stats
	stats := h.dbPool.Stat()

	// Check if pool is degraded (too many connections in use)
	status := HealthStatusHealthy
	message := "database connection healthy"

	usagePercent := float64(stats.AcquiredConns()) / float64(stats.MaxConns()) * 100
	if usagePercent > 90 {
		status = HealthStatusDegraded
		message = "database connection pool near capacity"
	}

	return ComponentHealth{
		Status:  status,
		Message: message,
		Latency: latency.String(),
		Details: map[string]interface{}{
			"total_conns":    stats.TotalConns(),
			"acquired_conns": stats.AcquiredConns(),
			"idle_conns":     stats.IdleConns(),
			"max_conns":      stats.MaxConns(),
			"usage_percent":  usagePercent,
		},
	}
}

// checkRedis checks the Redis connection health.
func (h *HealthHandler) checkRedis(ctx context.Context) ComponentHealth {
	if h.cache == nil {
		return ComponentHealth{
			Status:  HealthStatusHealthy,
			Message: "redis cache not configured (optional)",
		}
	}

	start := time.Now()

	// Try to ping Redis
	err := h.cache.Ping(ctx)
	latency := time.Since(start)

	if err != nil {
		if h.logger != nil {
			h.logger.Error("redis health check failed",
				"error", err.Error(),
				"latency_ms", latency.Milliseconds(),
			)
		}
		// Redis is optional, so we mark as degraded instead of unhealthy
		return ComponentHealth{
			Status:  HealthStatusDegraded,
			Message: "redis connection failed: " + err.Error(),
			Latency: latency.String(),
		}
	}

	return ComponentHealth{
		Status:  HealthStatusHealthy,
		Message: "redis connection healthy",
		Latency: latency.String(),
	}
}

// NotFoundHandler handles 404 Not Found responses.
func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	WriteError(w, r, &ProblemDetails{
		Type:   "https://api.shipaggregator.com/errors/not-found",
		Title:  "Not Found",
		Status: http.StatusNotFound,
		Detail: "The requested resource was not found",
		Code:   "NOT_FOUND",
	})
}

// MethodNotAllowedHandler handles 405 Method Not Allowed responses.
func MethodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	WriteError(w, r, &ProblemDetails{
		Type:   "https://api.shipaggregator.com/errors/method-not-allowed",
		Title:  "Method Not Allowed",
		Status: http.StatusMethodNotAllowed,
		Detail: "The HTTP method is not allowed for this resource",
		Code:   "METHOD_NOT_ALLOWED",
	})
}

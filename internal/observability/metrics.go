package observability

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsConfig holds configuration for metrics.
type MetricsConfig struct {
	Enabled     bool
	Namespace   string
	Subsystem   string
	ServiceName string
	Environment string
}

// DefaultMetricsConfig returns a default metrics configuration.
func DefaultMetricsConfig() MetricsConfig {
	return MetricsConfig{
		Enabled:     true,
		Namespace:   "shipping",
		Subsystem:   "aggregator",
		ServiceName: "shipping-aggregator",
		Environment: "development",
	}
}

// Metrics holds all Prometheus metrics for the application.
type Metrics struct {
	config   MetricsConfig
	registry *prometheus.Registry

	// HTTP metrics
	httpRequestsTotal    *prometheus.CounterVec
	httpRequestDuration  *prometheus.HistogramVec
	httpRequestsInFlight prometheus.Gauge
	httpResponseSize     *prometheus.HistogramVec

	// Provider API metrics
	providerAPILatency   *prometheus.HistogramVec
	providerAPITotal     *prometheus.CounterVec
	providerAPIErrors    *prometheus.CounterVec
	providerCircuitState *prometheus.GaugeVec
	providerHealthStatus *prometheus.GaugeVec

	// Business metrics
	shipmentsCreated    *prometheus.CounterVec
	shipmentsStatus     *prometheus.GaugeVec
	quotesRequested     *prometheus.CounterVec
	labelsGenerated     *prometheus.CounterVec
	webhooksDelivered   *prometheus.CounterVec
	webhookDeliveryTime *prometheus.HistogramVec

	// Billing metrics
	walletBalance      *prometheus.GaugeVec
	topupsTotal        *prometheus.CounterVec
	topupsAmount       *prometheus.CounterVec
	ledgerEntriesTotal *prometheus.CounterVec

	// Worker metrics
	tasksProcessed      *prometheus.CounterVec
	tasksProcessingTime *prometheus.HistogramVec
	tasksInQueue        *prometheus.GaugeVec
	tasksFailed         *prometheus.CounterVec

	// Database metrics
	dbConnectionsOpen  prometheus.Gauge
	dbConnectionsInUse prometheus.Gauge
	dbQueryDuration    *prometheus.HistogramVec

	// Redis metrics
	redisOperations  *prometheus.CounterVec
	redisLatency     *prometheus.HistogramVec
	redisCacheHits   *prometheus.CounterVec
	redisCacheMisses *prometheus.CounterVec

	// Error metrics
	panicsTotal *prometheus.CounterVec
}

var (
	globalMetrics     *Metrics
	globalMetricsOnce sync.Once
)

// NewMetrics creates a new Metrics instance.
func NewMetrics(cfg MetricsConfig) *Metrics {
	registry := prometheus.NewRegistry()

	// Register default collectors
	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	m := &Metrics{
		config:   cfg,
		registry: registry,
	}

	m.initHTTPMetrics()
	m.initProviderMetrics()
	m.initBusinessMetrics()
	m.initBillingMetrics()
	m.initWorkerMetrics()
	m.initDatabaseMetrics()
	m.initRedisMetrics()
	m.initErrorMetrics()

	return m
}

// initHTTPMetrics initializes HTTP-related metrics.
func (m *Metrics) initHTTPMetrics() {
	m.httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests processed",
		},
		[]string{"method", "path", "status"},
	)
	m.registry.MustRegister(m.httpRequestsTotal)

	m.httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request duration in seconds",
			Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path", "status"},
	)
	m.registry.MustRegister(m.httpRequestDuration)

	m.httpRequestsInFlight = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "http_requests_in_flight",
			Help:      "Number of HTTP requests currently being processed",
		},
	)
	m.registry.MustRegister(m.httpRequestsInFlight)

	m.httpResponseSize = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "http_response_size_bytes",
			Help:      "HTTP response size in bytes",
			Buckets:   prometheus.ExponentialBuckets(100, 10, 8),
		},
		[]string{"method", "path"},
	)
	m.registry.MustRegister(m.httpResponseSize)
}

// initProviderMetrics initializes provider API metrics.
func (m *Metrics) initProviderMetrics() {
	m.providerAPILatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "provider_api_latency_seconds",
			Help:      "Provider API call latency in seconds",
			Buckets:   []float64{.1, .25, .5, 1, 2.5, 5, 10, 30},
		},
		[]string{"provider", "operation"},
	)
	m.registry.MustRegister(m.providerAPILatency)

	m.providerAPITotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "provider_api_requests_total",
			Help:      "Total number of provider API requests",
		},
		[]string{"provider", "operation", "status"},
	)
	m.registry.MustRegister(m.providerAPITotal)

	m.providerAPIErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "provider_api_errors_total",
			Help:      "Total number of provider API errors",
		},
		[]string{"provider", "operation", "error_type"},
	)
	m.registry.MustRegister(m.providerAPIErrors)

	m.providerCircuitState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "provider_circuit_state",
			Help:      "Provider circuit breaker state (0=closed, 1=half-open, 2=open)",
		},
		[]string{"provider"},
	)
	m.registry.MustRegister(m.providerCircuitState)

	m.providerHealthStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "provider_health_status",
			Help:      "Provider health status (0=unhealthy, 1=healthy)",
		},
		[]string{"provider"},
	)
	m.registry.MustRegister(m.providerHealthStatus)
}

// initBusinessMetrics initializes business-related metrics.
func (m *Metrics) initBusinessMetrics() {
	m.shipmentsCreated = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "shipments_created_total",
			Help:      "Total number of shipments created",
		},
		[]string{"provider", "service_type", "business_model"},
	)
	m.registry.MustRegister(m.shipmentsCreated)

	m.shipmentsStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "shipments_by_status",
			Help:      "Number of shipments by status",
		},
		[]string{"status"},
	)
	m.registry.MustRegister(m.shipmentsStatus)

	m.quotesRequested = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "quotes_requested_total",
			Help:      "Total number of quotes requested",
		},
		[]string{"business_model"},
	)
	m.registry.MustRegister(m.quotesRequested)

	m.labelsGenerated = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "labels_generated_total",
			Help:      "Total number of shipping labels generated",
		},
		[]string{"provider", "format"},
	)
	m.registry.MustRegister(m.labelsGenerated)

	m.webhooksDelivered = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "webhooks_delivered_total",
			Help:      "Total number of webhooks delivered",
		},
		[]string{"event_type", "status"},
	)
	m.registry.MustRegister(m.webhooksDelivered)

	m.webhookDeliveryTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "webhook_delivery_duration_seconds",
			Help:      "Webhook delivery duration in seconds",
			Buckets:   []float64{.1, .25, .5, 1, 2.5, 5, 10, 30},
		},
		[]string{"event_type"},
	)
	m.registry.MustRegister(m.webhookDeliveryTime)
}

// initBillingMetrics initializes billing-related metrics.
func (m *Metrics) initBillingMetrics() {
	m.walletBalance = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "wallet_balance_kurus",
			Help:      "Current wallet balance in kuruş",
		},
		[]string{"organization_id"},
	)
	m.registry.MustRegister(m.walletBalance)

	m.topupsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "topups_total",
			Help:      "Total number of wallet top-ups",
		},
		[]string{"status", "payment_method"},
	)
	m.registry.MustRegister(m.topupsTotal)

	m.topupsAmount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "topups_amount_kurus_total",
			Help:      "Total amount of wallet top-ups in kuruş",
		},
		[]string{"payment_method"},
	)
	m.registry.MustRegister(m.topupsAmount)

	m.ledgerEntriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "ledger_entries_total",
			Help:      "Total number of ledger entries",
		},
		[]string{"entry_type"},
	)
	m.registry.MustRegister(m.ledgerEntriesTotal)
}

// initWorkerMetrics initializes worker-related metrics.
func (m *Metrics) initWorkerMetrics() {
	m.tasksProcessed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "tasks_processed_total",
			Help:      "Total number of tasks processed",
		},
		[]string{"task_type", "status"},
	)
	m.registry.MustRegister(m.tasksProcessed)

	m.tasksProcessingTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "tasks_processing_duration_seconds",
			Help:      "Task processing duration in seconds",
			Buckets:   []float64{.1, .25, .5, 1, 2.5, 5, 10, 30, 60},
		},
		[]string{"task_type"},
	)
	m.registry.MustRegister(m.tasksProcessingTime)

	m.tasksInQueue = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "tasks_in_queue",
			Help:      "Number of tasks currently in queue",
		},
		[]string{"queue"},
	)
	m.registry.MustRegister(m.tasksInQueue)

	m.tasksFailed = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "tasks_failed_total",
			Help:      "Total number of failed tasks",
		},
		[]string{"task_type", "error_type"},
	)
	m.registry.MustRegister(m.tasksFailed)
}

// initDatabaseMetrics initializes database-related metrics.
func (m *Metrics) initDatabaseMetrics() {
	m.dbConnectionsOpen = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "db_connections_open",
			Help:      "Number of open database connections",
		},
	)
	m.registry.MustRegister(m.dbConnectionsOpen)

	m.dbConnectionsInUse = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "db_connections_in_use",
			Help:      "Number of database connections currently in use",
		},
	)
	m.registry.MustRegister(m.dbConnectionsInUse)

	m.dbQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "db_query_duration_seconds",
			Help:      "Database query duration in seconds",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"query_type"},
	)
	m.registry.MustRegister(m.dbQueryDuration)
}

// initErrorMetrics initializes error-related metrics.
func (m *Metrics) initErrorMetrics() {
	m.panicsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "panics_total",
			Help:      "Total number of panics recovered",
		},
		[]string{"method", "path"},
	)
	m.registry.MustRegister(m.panicsTotal)
}

// initRedisMetrics initializes Redis-related metrics.
func (m *Metrics) initRedisMetrics() {
	m.redisOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "redis_operations_total",
			Help:      "Total number of Redis operations",
		},
		[]string{"operation", "status"},
	)
	m.registry.MustRegister(m.redisOperations)

	m.redisLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "redis_latency_seconds",
			Help:      "Redis operation latency in seconds",
			Buckets:   []float64{.0005, .001, .005, .01, .025, .05, .1},
		},
		[]string{"operation"},
	)
	m.registry.MustRegister(m.redisLatency)

	m.redisCacheHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "redis_cache_hits_total",
			Help:      "Total number of Redis cache hits",
		},
		[]string{"cache_type"},
	)
	m.registry.MustRegister(m.redisCacheHits)

	m.redisCacheMisses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: m.config.Namespace,
			Subsystem: m.config.Subsystem,
			Name:      "redis_cache_misses_total",
			Help:      "Total number of Redis cache misses",
		},
		[]string{"cache_type"},
	)
	m.registry.MustRegister(m.redisCacheMisses)
}

// Handler returns an HTTP handler for the metrics endpoint.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// Registry returns the Prometheus registry.
func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}

// HTTP metrics methods

// RecordHTTPRequest records an HTTP request.
func (m *Metrics) RecordHTTPRequest(method, path string, status int, duration time.Duration, responseSize int) {
	statusStr := strconv.Itoa(status)
	m.httpRequestsTotal.WithLabelValues(method, path, statusStr).Inc()
	m.httpRequestDuration.WithLabelValues(method, path, statusStr).Observe(duration.Seconds())
	m.httpResponseSize.WithLabelValues(method, path).Observe(float64(responseSize))
}

// IncHTTPRequestsInFlight increments the in-flight requests counter.
func (m *Metrics) IncHTTPRequestsInFlight() {
	m.httpRequestsInFlight.Inc()
}

// DecHTTPRequestsInFlight decrements the in-flight requests counter.
func (m *Metrics) DecHTTPRequestsInFlight() {
	m.httpRequestsInFlight.Dec()
}

// Provider metrics methods

// RecordProviderAPICall records a provider API call.
func (m *Metrics) RecordProviderAPICall(provider, operation, status string, duration time.Duration) {
	m.providerAPITotal.WithLabelValues(provider, operation, status).Inc()
	m.providerAPILatency.WithLabelValues(provider, operation).Observe(duration.Seconds())
}

// RecordProviderAPIError records a provider API error.
func (m *Metrics) RecordProviderAPIError(provider, operation, errorType string) {
	m.providerAPIErrors.WithLabelValues(provider, operation, errorType).Inc()
}

// SetProviderCircuitState sets the circuit breaker state for a provider.
// state: 0=closed, 1=half-open, 2=open
func (m *Metrics) SetProviderCircuitState(provider string, state int) {
	m.providerCircuitState.WithLabelValues(provider).Set(float64(state))
}

// SetProviderHealthStatus sets the health status for a provider.
// healthy: true=1, false=0
func (m *Metrics) SetProviderHealthStatus(provider string, healthy bool) {
	value := 0.0
	if healthy {
		value = 1.0
	}
	m.providerHealthStatus.WithLabelValues(provider).Set(value)
}

// Business metrics methods

// RecordShipmentCreated records a shipment creation.
func (m *Metrics) RecordShipmentCreated(provider, serviceType, businessModel string) {
	m.shipmentsCreated.WithLabelValues(provider, serviceType, businessModel).Inc()
}

// SetShipmentsByStatus sets the number of shipments by status.
func (m *Metrics) SetShipmentsByStatus(status string, count int) {
	m.shipmentsStatus.WithLabelValues(status).Set(float64(count))
}

// RecordQuoteRequested records a quote request.
func (m *Metrics) RecordQuoteRequested(businessModel string) {
	m.quotesRequested.WithLabelValues(businessModel).Inc()
}

// RecordLabelGenerated records a label generation.
func (m *Metrics) RecordLabelGenerated(provider, format string) {
	m.labelsGenerated.WithLabelValues(provider, format).Inc()
}

// RecordWebhookDelivered records a webhook delivery.
func (m *Metrics) RecordWebhookDelivered(eventType, status string, duration time.Duration) {
	m.webhooksDelivered.WithLabelValues(eventType, status).Inc()
	m.webhookDeliveryTime.WithLabelValues(eventType).Observe(duration.Seconds())
}

// Billing metrics methods

// SetWalletBalance sets the wallet balance for an organization.
func (m *Metrics) SetWalletBalance(organizationID string, balanceKurus int64) {
	m.walletBalance.WithLabelValues(organizationID).Set(float64(balanceKurus))
}

// RecordTopup records a wallet top-up.
func (m *Metrics) RecordTopup(status, paymentMethod string, amountKurus int64) {
	m.topupsTotal.WithLabelValues(status, paymentMethod).Inc()
	if status == "completed" {
		m.topupsAmount.WithLabelValues(paymentMethod).Add(float64(amountKurus))
	}
}

// RecordLedgerEntry records a ledger entry.
func (m *Metrics) RecordLedgerEntry(entryType string) {
	m.ledgerEntriesTotal.WithLabelValues(entryType).Inc()
}

// Worker metrics methods

// RecordTaskProcessed records a processed task.
func (m *Metrics) RecordTaskProcessed(taskType, status string, duration time.Duration) {
	m.tasksProcessed.WithLabelValues(taskType, status).Inc()
	m.tasksProcessingTime.WithLabelValues(taskType).Observe(duration.Seconds())
}

// RecordTaskFailed records a failed task.
func (m *Metrics) RecordTaskFailed(taskType, errorType string) {
	m.tasksFailed.WithLabelValues(taskType, errorType).Inc()
}

// SetTasksInQueue sets the number of tasks in a queue.
func (m *Metrics) SetTasksInQueue(queue string, count int) {
	m.tasksInQueue.WithLabelValues(queue).Set(float64(count))
}

// Database metrics methods

// SetDBConnections sets the database connection metrics.
func (m *Metrics) SetDBConnections(open, inUse int) {
	m.dbConnectionsOpen.Set(float64(open))
	m.dbConnectionsInUse.Set(float64(inUse))
}

// RecordDBQuery records a database query.
func (m *Metrics) RecordDBQuery(queryType string, duration time.Duration) {
	m.dbQueryDuration.WithLabelValues(queryType).Observe(duration.Seconds())
}

// Redis metrics methods

// RecordRedisOperation records a Redis operation.
func (m *Metrics) RecordRedisOperation(operation, status string, duration time.Duration) {
	m.redisOperations.WithLabelValues(operation, status).Inc()
	m.redisLatency.WithLabelValues(operation).Observe(duration.Seconds())
}

// RecordRedisCacheHit records a Redis cache hit.
func (m *Metrics) RecordRedisCacheHit(cacheType string) {
	m.redisCacheHits.WithLabelValues(cacheType).Inc()
}

// RecordRedisCacheMiss records a Redis cache miss.
func (m *Metrics) RecordRedisCacheMiss(cacheType string) {
	m.redisCacheMisses.WithLabelValues(cacheType).Inc()
}

// Error metrics methods

// RecordPanic records a panic recovery.
func (m *Metrics) RecordPanic(method, path string) {
	m.panicsTotal.WithLabelValues(method, path).Inc()
}

// HTTPMetricsMiddleware returns an HTTP middleware that records metrics.
func (m *Metrics) HTTPMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.IncHTTPRequestsInFlight()
		defer m.DecHTTPRequestsInFlight()

		start := time.Now()
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		m.RecordHTTPRequest(r.Method, r.URL.Path, wrapped.statusCode, duration, wrapped.bytesWritten)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code and response size.
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

// WriteHeader captures the status code.
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write captures the response size.
func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += n
	return n, err
}

// Global metrics functions

// SetGlobalMetrics sets the global metrics instance.
func SetGlobalMetrics(m *Metrics) {
	globalMetrics = m
}

// GetGlobalMetrics returns the global metrics instance.
// If not set, it creates a default metrics instance.
func GetGlobalMetrics() *Metrics {
	globalMetricsOnce.Do(func() {
		if globalMetrics == nil {
			globalMetrics = NewMetrics(DefaultMetricsConfig())
		}
	})
	return globalMetrics
}

// M returns the global metrics for convenience.
func M() *Metrics {
	return GetGlobalMetrics()
}

// ProviderAPITimer is a helper for timing provider API calls.
type ProviderAPITimer struct {
	metrics   *Metrics
	provider  string
	operation string
	start     time.Time
}

// NewProviderAPITimer creates a new provider API timer.
func NewProviderAPITimer(m *Metrics, provider, operation string) *ProviderAPITimer {
	return &ProviderAPITimer{
		metrics:   m,
		provider:  provider,
		operation: operation,
		start:     time.Now(),
	}
}

// ObserveSuccess records a successful API call.
func (t *ProviderAPITimer) ObserveSuccess() {
	t.metrics.RecordProviderAPICall(t.provider, t.operation, "success", time.Since(t.start))
}

// ObserveError records a failed API call.
func (t *ProviderAPITimer) ObserveError(errorType string) {
	t.metrics.RecordProviderAPICall(t.provider, t.operation, "error", time.Since(t.start))
	t.metrics.RecordProviderAPIError(t.provider, t.operation, errorType)
}

// DBQueryTimer is a helper for timing database queries.
type DBQueryTimer struct {
	metrics   *Metrics
	queryType string
	start     time.Time
}

// NewDBQueryTimer creates a new database query timer.
func NewDBQueryTimer(m *Metrics, queryType string) *DBQueryTimer {
	return &DBQueryTimer{
		metrics:   m,
		queryType: queryType,
		start:     time.Now(),
	}
}

// Observe records the query duration.
func (t *DBQueryTimer) Observe() {
	t.metrics.RecordDBQuery(t.queryType, time.Since(t.start))
}

// RedisOperationTimer is a helper for timing Redis operations.
type RedisOperationTimer struct {
	metrics   *Metrics
	operation string
	start     time.Time
}

// NewRedisOperationTimer creates a new Redis operation timer.
func NewRedisOperationTimer(m *Metrics, operation string) *RedisOperationTimer {
	return &RedisOperationTimer{
		metrics:   m,
		operation: operation,
		start:     time.Now(),
	}
}

// ObserveSuccess records a successful operation.
func (t *RedisOperationTimer) ObserveSuccess() {
	t.metrics.RecordRedisOperation(t.operation, "success", time.Since(t.start))
}

// ObserveError records a failed operation.
func (t *RedisOperationTimer) ObserveError() {
	t.metrics.RecordRedisOperation(t.operation, "error", time.Since(t.start))
}

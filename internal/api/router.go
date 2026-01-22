// Package api provides the HTTP API for the shipping aggregator.
package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/e-port/shipping-aggregator/internal/api/handler"
	"github.com/e-port/shipping-aggregator/internal/api/middleware"
	"github.com/e-port/shipping-aggregator/internal/observability"
	"github.com/e-port/shipping-aggregator/internal/repository/redis"
)

// RouterConfig holds configuration for the API router.
type RouterConfig struct {
	// Logger is the application logger.
	Logger *observability.Logger
	// Metrics is the metrics collector.
	Metrics *observability.Metrics
	// Tracer is the distributed tracer.
	Tracer *observability.Tracer
	// DBPool is the database connection pool.
	DBPool *pgxpool.Pool
	// RedisCache is the Redis cache.
	RedisCache *redis.Cache
	// RateLimiter is the rate limiter.
	RateLimiter *redis.RateLimiter
	// APIKeyValidator validates API keys.
	APIKeyValidator middleware.APIKeyValidator
	// SessionValidator validates sessions.
	SessionValidator middleware.SessionValidator
	// OrganizationLoader loads organization information.
	OrganizationLoader middleware.OrganizationLoader
	// CORSAllowedOrigins are the allowed CORS origins.
	CORSAllowedOrigins []string
	// RateLimitPerMinute is the default rate limit per minute.
	RateLimitPerMinute int
	// IdempotencyTTL is the TTL for idempotency keys.
	IdempotencyTTL time.Duration
	// Environment is the deployment environment (development, staging, production).
	Environment string
}

// NewRouter creates a new API router with all middleware and routes configured.
func NewRouter(cfg RouterConfig) *chi.Mux {
	r := chi.NewRouter()

	// Set defaults
	if cfg.RateLimitPerMinute == 0 {
		cfg.RateLimitPerMinute = 1000
	}
	if cfg.IdempotencyTTL == 0 {
		cfg.IdempotencyTTL = 24 * time.Hour
	}

	// Configure middleware chain
	configureMiddleware(r, cfg)

	// Configure routes
	configureRoutes(r, cfg)

	return r
}

// configureMiddleware sets up the middleware chain.
func configureMiddleware(r *chi.Mux, cfg RouterConfig) {
	// Recovery middleware - must be first to catch panics
	r.Use(chimiddleware.Recoverer)

	// Real IP middleware for proper client IP detection behind proxies
	r.Use(chimiddleware.RealIP)

	// Request ID and trace ID middleware
	r.Use(middleware.Trace(middleware.TraceConfig{
		Logger:                cfg.Logger,
		TrustForwardedHeaders: cfg.Environment != "production",
	}))

	// Request logging middleware
	if cfg.Logger != nil {
		r.Use(requestLoggingMiddleware(cfg.Logger))
	}

	// Metrics middleware
	if cfg.Metrics != nil {
		r.Use(metricsMiddleware(cfg.Metrics))
	}

	// CORS middleware
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID", "Idempotency-Key"},
		ExposedHeaders:   []string{"X-Request-ID", "X-Trace-ID", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset", "Retry-After"},
		AllowCredentials: true,
		MaxAge:           300, // 5 minutes
	}))

	// Timeout middleware
	r.Use(chimiddleware.Timeout(60 * time.Second))

	// Compression middleware
	r.Use(chimiddleware.Compress(5))
}

// configureRoutes sets up all API routes.
func configureRoutes(r *chi.Mux, cfg RouterConfig) {
	// Health check endpoints (no auth required)
	healthHandler := handler.NewHealthHandler(cfg.DBPool, cfg.RedisCache, cfg.Logger)
	r.Get("/health/live", healthHandler.Live)
	r.Get("/health/ready", healthHandler.Ready)

	// API v1 routes
	r.Route("/v1", func(r chi.Router) {
		// Authentication middleware for all v1 routes
		r.Use(middleware.Auth(middleware.AuthConfig{
			APIKeyValidator:  cfg.APIKeyValidator,
			SessionValidator: cfg.SessionValidator,
			Logger:           cfg.Logger,
			RequireAuth:      true,
			SkipPaths: []string{
				"/v1/health",
				"/v1/docs",
			},
		}))

		// Tenant middleware for organization context
		r.Use(middleware.Tenant(middleware.TenantConfig{
			DBPool:             cfg.DBPool,
			OrganizationLoader: cfg.OrganizationLoader,
			Logger:             cfg.Logger,
			RequireTenant:      true,
			SkipPaths: []string{
				"/v1/health",
				"/v1/docs",
			},
		}))

		// Rate limiting middleware
		r.Use(middleware.RateLimitByAPIKey(middleware.RateLimitConfig{
			RateLimiter:  cfg.RateLimiter,
			Logger:       cfg.Logger,
			DefaultLimit: cfg.RateLimitPerMinute,
			DefaultBurst: cfg.RateLimitPerMinute / 10,
			SkipPaths: []string{
				"/v1/health",
			},
		}))

		// Idempotency middleware for mutating operations
		r.Use(middleware.Idempotency(middleware.IdempotencyConfig{
			Cache:           cfg.RedisCache,
			Logger:          cfg.Logger,
			TTL:             cfg.IdempotencyTTL,
			HeaderName:      "Idempotency-Key",
			RequiredMethods: []string{"POST", "PUT", "PATCH"},
			SkipPaths: []string{
				"/v1/health",
				"/v1/docs",
			},
		}))

		// Health endpoint within v1 (for versioned health checks)
		r.Get("/health", healthHandler.Ready)

		// Quotes routes
		r.Route("/quotes", func(r chi.Router) {
			r.Use(middleware.RequireScope("quotes:read"))
			// r.Get("/", quotesHandler.List)
			// r.Post("/", quotesHandler.Create)
			// r.Get("/{quoteID}", quotesHandler.Get)
		})

		// Shipments routes
		r.Route("/shipments", func(r chi.Router) {
			r.Use(middleware.RequireScope("shipments:read"))
			// r.Get("/", shipmentsHandler.List)
			// r.Post("/", shipmentsHandler.Create)
			// r.Get("/{shipmentID}", shipmentsHandler.Get)
			// r.Post("/{shipmentID}/cancel", shipmentsHandler.Cancel)
			// r.Get("/{shipmentID}/label", shipmentsHandler.GetLabel)
			// r.Get("/{shipmentID}/tracking", shipmentsHandler.GetTracking)
		})

		// Tracking routes
		r.Route("/tracking", func(r chi.Router) {
			r.Use(middleware.RequireScope("tracking:read"))
			// r.Get("/{trackingNumber}", trackingHandler.Get)
		})

		// Addresses routes
		r.Route("/addresses", func(r chi.Router) {
			r.Use(middleware.RequireScope("addresses:read"))
			// r.Get("/", addressesHandler.List)
			// r.Post("/", addressesHandler.Create)
			// r.Get("/{addressID}", addressesHandler.Get)
			// r.Put("/{addressID}", addressesHandler.Update)
			// r.Delete("/{addressID}", addressesHandler.Delete)
			// r.Post("/validate", addressesHandler.Validate)
		})

		// Webhooks routes
		r.Route("/webhooks", func(r chi.Router) {
			r.Use(middleware.RequireScope("webhooks:read"))
			// r.Get("/", webhooksHandler.List)
			// r.Post("/", webhooksHandler.Create)
			// r.Get("/{webhookID}", webhooksHandler.Get)
			// r.Put("/{webhookID}", webhooksHandler.Update)
			// r.Delete("/{webhookID}", webhooksHandler.Delete)
		})

		// Organization routes (admin only)
		r.Route("/organization", func(r chi.Router) {
			r.Use(middleware.RequireScope("organization:read"))
			// r.Get("/", organizationHandler.Get)
			// r.Put("/", organizationHandler.Update)
			// r.Get("/members", organizationHandler.ListMembers)
			// r.Post("/members", organizationHandler.InviteMember)
			// r.Get("/api-keys", organizationHandler.ListAPIKeys)
			// r.Post("/api-keys", organizationHandler.CreateAPIKey)
		})

		// Billing routes
		r.Route("/billing", func(r chi.Router) {
			r.Use(middleware.RequireScope("billing:read"))
			// r.Get("/balance", billingHandler.GetBalance)
			// r.Get("/transactions", billingHandler.ListTransactions)
			// r.Post("/topup", billingHandler.CreateTopup)
		})
	})

	// Webhook receiver endpoints (for provider callbacks)
	r.Route("/webhooks", func(r chi.Router) {
		// These endpoints receive webhooks from providers
		// They have their own authentication (signature verification)
		// r.Post("/tracking/{provider}", webhookReceiverHandler.ReceiveTracking)
	})

	// Not found handler
	r.NotFound(handler.NotFoundHandler)

	// Method not allowed handler
	r.MethodNotAllowed(handler.MethodNotAllowedHandler)
}

// requestLoggingMiddleware creates a request logging middleware.
func requestLoggingMiddleware(logger *observability.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Process request
			next.ServeHTTP(ww, r)

			// Log request
			duration := time.Since(start)
			logger.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration_ms", duration.Milliseconds(),
				"bytes", ww.BytesWritten(),
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
				"trace_id", middleware.GetTraceID(r.Context()),
				"request_id", middleware.GetRequestID(r.Context()),
			)
		})
	}
}

// metricsMiddleware creates a metrics collection middleware.
func metricsMiddleware(metrics *observability.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap response writer to capture status code
			ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Process request
			next.ServeHTTP(ww, r)

			// Record metrics
			duration := time.Since(start)
			metrics.RecordHTTPRequest(r.Method, r.URL.Path, ww.Status(), duration, ww.BytesWritten())
		})
	}
}

// Server represents the HTTP server.
type Server struct {
	router *chi.Mux
	config RouterConfig
}

// NewServer creates a new HTTP server.
func NewServer(cfg RouterConfig) *Server {
	return &Server{
		router: NewRouter(cfg),
		config: cfg,
	}
}

// Router returns the chi router.
func (s *Server) Router() *chi.Mux {
	return s.router
}

// ServeHTTP implements the http.Handler interface.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

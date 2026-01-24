// Package api provides the HTTP API for the shipping aggregator.
package api

import (
	_ "embed"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/knid/kargonet/internal/api/handler"
	"github.com/knid/kargonet/internal/api/integration"
	"github.com/knid/kargonet/internal/api/middleware"
	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/payment/iyzico"
	"github.com/knid/kargonet/internal/repository/postgres"
	"github.com/knid/kargonet/internal/repository/postgres/db"
	"github.com/knid/kargonet/internal/repository/redis"
	"github.com/knid/kargonet/internal/service/auth"
	"github.com/knid/kargonet/internal/service/billing"
	"github.com/knid/kargonet/internal/service/organization"
	providerservice "github.com/knid/kargonet/internal/service/provider"
	"github.com/knid/kargonet/internal/service/quote"
	"github.com/knid/kargonet/internal/service/shipment"
	"github.com/knid/kargonet/internal/service/tracking"
	"github.com/knid/kargonet/internal/service/webhook"
)

//go:embed openapi.json
var openAPISpec []byte

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
	// Queries is the database queries interface.
	Queries *db.Queries
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
	// QuoteService is the quote service.
	QuoteService *quote.Service
	// ShipmentService is the shipment service.
	ShipmentService *shipment.Service
	// ProviderAccountService is the provider account service (US2: BYOA).
	ProviderAccountService *providerservice.AccountService
	// ProviderVerificationService is the provider verification service (US2: BYOA).
	ProviderVerificationService *providerservice.VerificationService
	// WalletService is the wallet service (US3: COD).
	WalletService *billing.WalletService
	// LedgerService is the ledger service (US3: COD).
	LedgerService *billing.LedgerService
	// CODService is the COD settlement service (US3: COD).
	CODService *billing.CODService
	// TopupService is the topup service (US4: Wallet Top-Up).
	TopupService *billing.TopupService
	// IyzicoClient is the Iyzico payment client (US4: Wallet Top-Up).
	IyzicoClient *iyzico.Client
	// IyzicoReturnURL is the return URL for Iyzico callbacks (US4: Wallet Top-Up).
	IyzicoReturnURL string
	// TrackingService is the tracking service (US5: Tracking Updates).
	TrackingService *tracking.Service
	// WebhookPublisher is the webhook publisher (US5: Webhooks).
	WebhookPublisher *webhook.Publisher
	// WebhookDeliveryService is the webhook delivery service (US5: Webhooks).
	WebhookDeliveryService *webhook.DeliveryService
	// APIKeyService is the API key service (US6: API Key Management).
	APIKeyService *auth.APIKeyService
	// SessionService is the session service for authentication.
	SessionService *auth.SessionService
	// AuditLogRepository is the audit log repository (US10: Admin Tools).
	AuditLogRepository *postgres.AuditLogRepository
	// ProviderAPILogRepository is the provider API log repository (US10: Admin Tools).
	ProviderAPILogRepository *postgres.ProviderAPILogRepository
	// OrganizationService is the organization service for managing organizations and members.
	OrganizationService *organization.Service
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
	// r.Get("/health/ready", healthHandler.Ready)

	// Initialize handlers
	var quoteHandler *handler.QuoteHandler
	var shipmentHandler *handler.ShipmentHandler
	var providerAccountHandler *handler.ProviderAccountHandler
	var billingHandler *handler.BillingHandler
	var addressHandler *handler.AddressHandler
	var organizationHandler *handler.OrganizationHandler

	if cfg.Queries != nil {
		addressHandler = handler.NewAddressHandler(cfg.Queries, cfg.Logger)
	}
	if cfg.OrganizationService != nil {
		organizationHandler = handler.NewOrganizationHandler(cfg.OrganizationService, cfg.Logger)
	}
	if cfg.QuoteService != nil && cfg.Queries != nil {
		quoteHandler = handler.NewQuoteHandler(cfg.QuoteService, cfg.Queries, cfg.Logger)
	}
	if cfg.ShipmentService != nil && cfg.Queries != nil {
		shipmentHandler = handler.NewShipmentHandler(cfg.ShipmentService, cfg.TrackingService, cfg.Queries, cfg.Logger)
	}
	if cfg.ProviderAccountService != nil && cfg.ProviderVerificationService != nil {
		providerAccountHandler = handler.NewProviderAccountHandler(
			cfg.ProviderAccountService,
			cfg.ProviderVerificationService,
			cfg.Logger,
		)
	}
	if cfg.WalletService != nil && cfg.LedgerService != nil {
		if cfg.TopupService != nil {
			billingHandler = handler.NewBillingHandlerFull(
				cfg.WalletService,
				cfg.LedgerService,
				cfg.CODService,
				cfg.TopupService,
				cfg.Logger,
			)
		} else if cfg.CODService != nil {
			billingHandler = handler.NewBillingHandlerWithCOD(
				cfg.WalletService,
				cfg.LedgerService,
				cfg.CODService,
				cfg.Logger,
			)
		} else {
			billingHandler = handler.NewBillingHandler(
				cfg.WalletService,
				cfg.LedgerService,
				cfg.Logger,
			)
		}
	}

	// Initialize Iyzico integration handler (US4: Wallet Top-Up)
	var iyzicoHandler *integration.IyzicoHandler
	if cfg.TopupService != nil && cfg.IyzicoClient != nil {
		iyzicoHandler = integration.NewIyzicoHandler(
			cfg.TopupService,
			cfg.IyzicoClient,
			cfg.Logger,
			cfg.IyzicoReturnURL,
		)
	}

	// Initialize carrier tracking webhook handlers (US5: Tracking Updates)
	var arasWebhookHandler *integration.ArasWebhookHandler
	var yurticiWebhookHandler *integration.YurticiWebhookHandler
	var hepsijetWebhookHandler *integration.HepsijetWebhookHandler
	if cfg.TrackingService != nil && cfg.WebhookPublisher != nil {
		arasWebhookHandler = integration.NewArasWebhookHandler(
			cfg.DBPool,
			cfg.Queries,
			cfg.TrackingService,
			cfg.WebhookPublisher,
			cfg.Logger,
		)
		yurticiWebhookHandler = integration.NewYurticiWebhookHandler(
			cfg.DBPool,
			cfg.Queries,
			cfg.TrackingService,
			cfg.WebhookPublisher,
			cfg.Logger,
		)
		hepsijetWebhookHandler = integration.NewHepsijetWebhookHandler(
			cfg.DBPool,
			cfg.Queries,
			cfg.TrackingService,
			cfg.WebhookPublisher,
			cfg.Logger,
		)
	}

	// Initialize API key handler (US6: API Key Management)
	var apiKeyHandler *handler.APIKeyHandler
	if cfg.APIKeyService != nil {
		apiKeyHandler = handler.NewAPIKeyHandler(cfg.APIKeyService, cfg.Logger)
	}

	// Initialize admin handler (US10: Admin Tools)
	var adminHandler *handler.AdminHandler
	if cfg.AuditLogRepository != nil && cfg.ProviderAPILogRepository != nil {
		adminHandler = handler.NewAdminHandler(
			cfg.AuditLogRepository,
			cfg.ProviderAPILogRepository,
			cfg.Queries,
			cfg.TrackingService,
			cfg.WebhookDeliveryService,
			cfg.Logger,
		)
	}

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
				"/v1/openapi.json",
				"/v1/auth/login",
				"/v1/auth/register",
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
				"/v1/openapi.json",
				"/v1/auth/login",
				"/v1/auth/register",
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
				"/v1/auth/login",
				"/v1/auth/register",
			},
		}))

		// Health endpoint within v1 (for versioned health checks)
		r.Get("/health", healthHandler.Live)

		// Auth routes (public endpoints for login/register, authenticated for others)
		if cfg.SessionService != nil {
			authHandler := handler.NewAuthHandler(cfg.SessionService, cfg.Logger)
			r.Route("/auth", func(r chi.Router) {
				r.Post("/register", authHandler.Register)
				r.Post("/login", authHandler.Login)

				r.Group(func(r chi.Router) {
					/* r.Use(middleware.Auth(middleware.AuthConfig{
						APIKeyValidator:  cfg.APIKeyValidator,
						SessionValidator: cfg.SessionValidator,
						Logger:           cfg.Logger,
						RequireAuth:      true,
					})) */
					r.Post("/logout", authHandler.Logout)
					r.Post("/logout-all", authHandler.LogoutAll)
					r.Get("/me", authHandler.Me)
					r.Get("/sessions", authHandler.ListSessions)
					r.Delete("/sessions/{id}", authHandler.DeleteSession)
				})
			})
		}

		// OpenAPI spec endpoint (T217)
		r.Get("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "public, max-age=3600")
			w.WriteHeader(http.StatusOK)
			w.Write(openAPISpec)
		})

		// Quotes routes (US1: Multi-carrier rate shopping)
		r.Route("/quotes", func(r chi.Router) {
			r.Use(middleware.RequireScope("quotes:read"))
			if quoteHandler != nil {
				r.Get("/", quoteHandler.ListQuotes)
				r.With(middleware.RequireScope("quotes:create")).Post("/", quoteHandler.CreateQuote)
				r.Get("/{id}", quoteHandler.GetQuote)
				r.Get("/{id}/offers", quoteHandler.GetQuoteOffers)
				r.With(middleware.RequireScope("quotes:create")).Post("/{id}/refresh", quoteHandler.RefreshQuote)
			}
		})

		// Offers routes (US1: Multi-carrier rate shopping)
		r.Route("/offers", func(r chi.Router) {
			r.Use(middleware.RequireScope("quotes:read"))
			if quoteHandler != nil {
				r.Get("/{id}", quoteHandler.GetOffer)
			}
		})

		// Shipments routes (US1: Multi-carrier rate shopping, US5: Tracking, US8: Returns, US9: Pickup Scheduling)
		r.Route("/shipments", func(r chi.Router) {
			r.Use(middleware.RequireScope("shipments:read"))
			if shipmentHandler != nil {
				r.Get("/", shipmentHandler.ListShipments)
				r.With(middleware.RequireScope("shipments:create")).Post("/", shipmentHandler.CreateShipment)
				r.Get("/{id}", shipmentHandler.GetShipment)
				r.With(middleware.RequireScope("shipments:label")).Post("/{id}/purchase-label", shipmentHandler.PurchaseLabel)
				r.With(middleware.RequireScope("shipments:cancel")).Post("/{id}/cancel", shipmentHandler.CancelShipment)
				// US5: Tracking endpoint
				r.Get("/{id}/tracking", shipmentHandler.GetTracking)
				// US8: Return shipment endpoint
				r.With(middleware.RequireScope("shipments:return")).Post("/{id}/return", shipmentHandler.CreateReturn)
				// US9: Pickup scheduling endpoint
				r.With(middleware.RequireScope("shipments:pickup")).Post("/{id}/pickup", shipmentHandler.SchedulePickup)
			}
		})

		// Tracking routes
		r.Route("/tracking", func(r chi.Router) {
			r.Use(middleware.RequireScope("tracking:read"))
			// r.Get("/{trackingNumber}", trackingHandler.Get)
		})

		// Addresses routes
		r.Route("/addresses", func(r chi.Router) {
			r.Use(middleware.RequireScope("addresses:read"))
			if addressHandler != nil {
				r.Get("/", addressHandler.ListAddresses)
				r.With(middleware.RequireScope("addresses:create")).Post("/", addressHandler.CreateAddress)
				r.Get("/default-sender", addressHandler.GetDefaultSender)
				r.Get("/default-recipient", addressHandler.GetDefaultRecipient)
				r.Get("/{id}", addressHandler.GetAddress)
				r.With(middleware.RequireScope("addresses:update")).Patch("/{id}", addressHandler.UpdateAddress)
				r.With(middleware.RequireScope("addresses:delete")).Delete("/{id}", addressHandler.DeleteAddress)
				r.With(middleware.RequireScope("addresses:update")).Post("/{id}/default-sender", addressHandler.SetDefaultSender)
				r.With(middleware.RequireScope("addresses:update")).Post("/{id}/default-recipient", addressHandler.SetDefaultRecipient)
			}
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

		// Organization routes - /orgs/current for current organization context
		r.Route("/orgs/current", func(r chi.Router) {
			r.Use(middleware.RequireScope("organization:read"))
			if organizationHandler != nil {
				r.Get("/", organizationHandler.GetCurrentOrganization)
				r.With(middleware.RequireScope("organization:update")).Patch("/", organizationHandler.UpdateCurrentOrganization)
				r.With(middleware.RequireScope("members:read")).Get("/members", organizationHandler.ListMembers)
				r.With(middleware.RequireScope("members:create")).Post("/members", organizationHandler.CreateMember)
				r.With(middleware.RequireScope("members:read")).Get("/members/{id}", organizationHandler.GetMember)
				r.With(middleware.RequireScope("members:update")).Patch("/members/{id}", organizationHandler.UpdateMember)
				r.With(middleware.RequireScope("members:delete")).Delete("/members/{id}", organizationHandler.RemoveMember)
				r.With(middleware.RequireScope("members:update")).Post("/members/{id}/suspend", organizationHandler.SuspendMember)
				r.With(middleware.RequireScope("members:update")).Post("/members/{id}/reactivate", organizationHandler.ReactivateMember)
			}
		})

		// API Keys routes (US6: API Key Management)
		r.Route("/api-keys", func(r chi.Router) {
			r.Use(middleware.RequireScope("api_keys:read"))
			if apiKeyHandler != nil {
				r.Get("/", apiKeyHandler.ListAPIKeys)
				r.With(middleware.RequireScope("api_keys:create")).Post("/", apiKeyHandler.CreateAPIKey)
				r.Get("/{id}", apiKeyHandler.GetAPIKey)
				r.With(middleware.RequireScope("api_keys:update")).Patch("/{id}", apiKeyHandler.UpdateAPIKey)
				r.With(middleware.RequireScope("api_keys:delete")).Delete("/{id}", apiKeyHandler.RevokeAPIKey)
				r.With(middleware.RequireScope("api_keys:rotate")).Post("/{id}/rotate", apiKeyHandler.RotateAPIKey)
			}
		})

		// Billing routes (US3: COD Settlements, US4: Wallet Top-Up, US7: Refunds)
		r.Route("/billing", func(r chi.Router) {
			r.Use(middleware.RequireScope("billing:read"))
			if billingHandler != nil {
				r.Get("/balance", billingHandler.GetBalance)
				r.Get("/transactions", billingHandler.GetTransactions) // T218: Combined transactions view
				r.Get("/ledger", billingHandler.GetLedger)
				r.Get("/ledger/summary", billingHandler.GetLedgerSummary)
				r.Get("/ledger/{id}", billingHandler.GetLedgerEntry)

				// COD Settlement routes (US3)
				r.Route("/cod/settlements", func(r chi.Router) {
					r.Get("/", billingHandler.ListCODSettlements)
					r.Get("/summary", billingHandler.GetCODSettlementSummary)
					r.Get("/{id}", billingHandler.GetCODSettlement)
					r.With(middleware.RequireScope("billing:refund")).Post("/{id}/dispute", billingHandler.DisputeCODSettlement)

					// Admin-only reconciliation endpoints
					r.With(middleware.RequireScope("billing:cod_reconcile")).Post("/reconcile", billingHandler.ReconcileCODSettlements)
					r.With(middleware.RequireScope("billing:cod_reconcile")).Post("/{id}/collected", billingHandler.MarkCODSettlementCollected)
				})

				// Topup routes (US4: Wallet Top-Up via Iyzico)
				r.Route("/topups", func(r chi.Router) {
					r.Get("/", billingHandler.ListTopups)
					r.With(middleware.RequireScope("billing:topup")).Post("/", billingHandler.CreateTopup)
					r.Get("/{id}", billingHandler.GetTopup)
					r.With(middleware.RequireScope("billing:topup")).Post("/{id}/confirm", billingHandler.ConfirmTopup)
					r.With(middleware.RequireScope("billing:topup")).Post("/{id}/cancel", billingHandler.CancelTopup)
				})

				// Refund routes (US7: Shipment Cancellation & Refund)
				r.Route("/refunds", func(r chi.Router) {
					r.Get("/", billingHandler.ListRefunds)
					r.Get("/summary", billingHandler.GetRefundSummary)
					r.Get("/shipment/{shipment_id}", billingHandler.GetRefundByShipment)
					r.Get("/{id}", billingHandler.GetRefund)
				})
			}
		})

		// Providers routes (reference data)
		r.Route("/providers", func(r chi.Router) {
			r.Use(middleware.RequireScope("providers:read"))

			// Initialize provider handler
			if cfg.Queries != nil {
				providerHandler := handler.NewProviderHandler(cfg.Queries, cfg.Logger)
				r.Get("/", providerHandler.ListProviders)
				r.Get("/{code}", providerHandler.GetProvider)
				r.Get("/{code}/details", providerHandler.GetProviderWithServices)
				r.Get("/{code}/services", providerHandler.ListProviderServices)
				r.Get("/{code}/services/{serviceCode}", providerHandler.GetProviderService)
				r.Get("/{code}/services/cod", providerHandler.ListProviderServicesWithCOD)
				r.Get("/{code}/services/return", providerHandler.ListProviderServicesWithReturn)
				r.Get("/{code}/capabilities", providerHandler.GetProviderCapabilities)
				r.Get("/{code}/rate-plans", providerHandler.ListMarketplaceRatePlans)
				r.Get("/{code}/dropoff-points", providerHandler.GetDropoffPoints)
			}

			// Credential schema endpoint (US2: BYOA)
			if providerAccountHandler != nil {
				r.Get("/{provider_code}/credential-schema", providerAccountHandler.GetCredentialSchema)
			}
		})

		// Rate plans route (marketplace pricing)
		r.Route("/rate-plans", func(r chi.Router) {
			r.Use(middleware.RequireScope("providers:read"))
			if cfg.Queries != nil {
				providerHandler := handler.NewProviderHandler(cfg.Queries, cfg.Logger)
				r.Get("/", providerHandler.ListCurrentMarketplaceRatePlans)
			}
		})

		// Provider Accounts routes (US2: Bring Your Own Account)
		r.Route("/provider-accounts", func(r chi.Router) {
			r.Use(middleware.RequireScope("provider_accounts:read"))
			if providerAccountHandler != nil {
				r.Get("/", providerAccountHandler.ListProviderAccounts)
				r.With(middleware.RequireScope("provider_accounts:create")).Post("/", providerAccountHandler.CreateProviderAccount)
				r.Get("/{id}", providerAccountHandler.GetProviderAccount)
				r.With(middleware.RequireScope("provider_accounts:update")).Put("/{id}", providerAccountHandler.UpdateProviderAccount)
				r.With(middleware.RequireScope("provider_accounts:delete")).Delete("/{id}", providerAccountHandler.DeleteProviderAccount)
				r.With(middleware.RequireScope("provider_accounts:verify")).Post("/{id}/verify", providerAccountHandler.VerifyProviderAccount)
				r.With(middleware.RequireScope("provider_accounts:update")).Post("/{id}/set-default", providerAccountHandler.SetDefaultProviderAccount)
				r.With(middleware.RequireScope("provider_accounts:update")).Post("/{id}/suspend", providerAccountHandler.SuspendProviderAccount)
			}
		})

		// Reference data routes
		r.Route("/ref", func(r chi.Router) {
			// Reference data is public within authenticated context
			if cfg.Queries != nil {
				referenceHandler := handler.NewReferenceHandler(cfg.Queries, cfg.Logger)
				r.Get("/cities", referenceHandler.ListCities)
				r.Get("/cities/{code}", referenceHandler.GetCity)
				r.Get("/districts", referenceHandler.ListDistricts)
				r.Get("/districts/{code}", referenceHandler.GetDistrict)
				r.Get("/neighborhoods", referenceHandler.ListNeighborhoods)
				r.Get("/neighborhoods/{code}", referenceHandler.GetNeighborhood)
				r.Get("/postal-codes", referenceHandler.ListPostalCodes)
				r.Get("/postal-codes/{code}", referenceHandler.GetPostalCode)
				r.Get("/lookup/postal-code/{code}", referenceHandler.LookupByPostalCode)
				r.Get("/search", referenceHandler.SearchLocations)
				r.Get("/stats", referenceHandler.GetReferenceDataStats)
			}
		})

		// Roles routes (T220)
		r.Route("/roles", func(r chi.Router) {
			r.Use(middleware.RequireScope("roles:read"))
			if cfg.Queries != nil {
				roleHandler := handler.NewRoleHandler(cfg.Queries, cfg.Logger)
				r.Get("/", roleHandler.ListRoles)
				r.With(middleware.RequireScope("roles:create")).Post("/", roleHandler.CreateRole)
				r.Get("/{id}", roleHandler.GetRole)
				r.With(middleware.RequireScope("roles:update")).Put("/{id}", roleHandler.UpdateRole)
				r.With(middleware.RequireScope("roles:delete")).Delete("/{id}", roleHandler.DeleteRole)
			}
		})

		// Admin routes (US10: Admin Tools)
		r.Route("/admin", func(r chi.Router) {
			if adminHandler != nil {
				// Audit logs (T183)
				r.With(middleware.RequireScope("admin:audit_logs")).Get("/audit-logs", adminHandler.ListAuditLogs)
				r.With(middleware.RequireScope("admin:audit_logs")).Get("/audit-logs/summary", adminHandler.GetAuditLogSummary)
				r.With(middleware.RequireScope("admin:audit_logs")).Get("/audit-logs/{id}", adminHandler.GetAuditLog)

				// Provider logs (T184)
				r.With(middleware.RequireScope("admin:provider_logs")).Get("/provider-logs", adminHandler.ListProviderLogs)
				r.With(middleware.RequireScope("admin:provider_logs")).Get("/provider-logs/{id}", adminHandler.GetProviderLog)

				// Provider health (T185)
				r.With(middleware.RequireScope("admin:health")).Get("/health/providers", adminHandler.GetProviderHealth)
				r.With(middleware.RequireScope("admin:health")).Get("/health/providers/{provider_code}", adminHandler.GetProviderHealthDetail)

				// Webhook DLQ (T186)
				r.With(middleware.RequireScope("admin:audit_logs")).Get("/webhooks/dlq", adminHandler.ListDLQ)
				r.With(middleware.RequireScope("admin:audit_logs")).Post("/webhooks/dlq/reprocess", adminHandler.ReprocessDLQ)

				// Shipment tracking rebuild (T187)
				r.With(middleware.RequireScope("admin:audit_logs")).Post("/shipments/{id}/rebuild-tracking", adminHandler.RebuildTracking)
			}
		})
	})

	// Webhook receiver endpoints (for provider callbacks)
	r.Route("/webhooks", func(r chi.Router) {
		// These endpoints receive webhooks from providers
		// They have their own authentication (signature verification)

		// US5: Carrier tracking webhook endpoints
		r.Route("/tracking", func(r chi.Router) {
			if arasWebhookHandler != nil {
				r.Post("/aras", arasWebhookHandler.HandleWebhook)
			}
			if yurticiWebhookHandler != nil {
				r.Post("/yurtici", yurticiWebhookHandler.HandleWebhook)
			}
			if hepsijetWebhookHandler != nil {
				r.Post("/hepsijet", hepsijetWebhookHandler.HandleWebhook)
			}
		})
	})

	// Integration endpoints (for external service callbacks)
	r.Route("/integrations", func(r chi.Router) {
		// Iyzico payment integration (US4: Wallet Top-Up)
		r.Route("/iyzico", func(r chi.Router) {
			if iyzicoHandler != nil {
				// Callback endpoint - called when user completes payment on Iyzico
				r.Post("/callback", iyzicoHandler.HandleCallback)
				// Webhook endpoint - called by Iyzico for async notifications
				r.Post("/webhook", iyzicoHandler.HandleWebhook)
			}
		})

		// US5: Carrier tracking webhook endpoints (alternative path)
		r.Route("/tracking", func(r chi.Router) {
			if arasWebhookHandler != nil {
				r.Post("/aras", arasWebhookHandler.HandleWebhook)
			}
			if yurticiWebhookHandler != nil {
				r.Post("/yurtici", yurticiWebhookHandler.HandleWebhook)
			}
			if hepsijetWebhookHandler != nil {
				r.Post("/hepsijet", hepsijetWebhookHandler.HandleWebhook)
			}
		})

		// US10: Admin poll tracking endpoint (T188)
		// This requires admin authentication
		r.Route("/{provider}", func(r chi.Router) {
			// Add auth middleware for admin operations
			r.Use(middleware.Auth(middleware.AuthConfig{
				APIKeyValidator:  cfg.APIKeyValidator,
				SessionValidator: cfg.SessionValidator,
				Logger:           cfg.Logger,
				RequireAuth:      true,
			}))
			r.Use(middleware.RequireScope("admin:provider_logs"))
			if adminHandler != nil {
				r.Post("/poll-tracking", adminHandler.PollTracking)
			}
		})
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

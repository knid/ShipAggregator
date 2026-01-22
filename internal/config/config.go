// Package config provides configuration loading and validation for the shipping aggregator.
// It loads configuration from environment variables with support for .env files.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Environment represents the application environment.
type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvStaging     Environment = "staging"
	EnvProduction  Environment = "production"
)

// Config holds all application configuration.
type Config struct {
	App           AppConfig
	API           APIConfig
	Database      DatabaseConfig
	Redis         RedisConfig
	Storage       StorageConfig
	Security      SecurityConfig
	RateLimit     RateLimitConfig
	Idempotency   IdempotencyConfig
	Webhook       WebhookConfig
	Worker        WorkerConfig
	Observability ObservabilityConfig
	Iyzico        IyzicoConfig
	Providers     ProvidersConfig
	Features      FeaturesConfig
	CORS          CORSConfig
	Retention     RetentionConfig
}

// AppConfig holds application-level configuration.
type AppConfig struct {
	Env       Environment
	Name      string
	Version   string
	Debug     bool
	LogLevel  string
	LogFormat string
}

// APIConfig holds HTTP server configuration.
type APIConfig struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

// DatabaseConfig holds PostgreSQL configuration.
type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	URL             string // Alternative to individual settings
}

// DSN returns the PostgreSQL connection string.
func (c *DatabaseConfig) DSN() string {
	if c.URL != "" {
		return c.URL
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode,
	)
}

// RedisConfig holds Redis configuration.
type RedisConfig struct {
	Host       string
	Port       int
	Password   string
	DB         int
	MaxRetries int
	PoolSize   int
	URL        string // Alternative to individual settings
}

// Addr returns the Redis address.
func (c *RedisConfig) Addr() string {
	if c.URL != "" {
		return c.URL
	}
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// StorageConfig holds MinIO/S3 configuration.
type StorageConfig struct {
	Endpoint        string
	AccessKey       string
	SecretKey       string
	UseSSL          bool
	BucketDocuments string
	BucketLabels    string
	Region          string
}

// SecurityConfig holds security-related configuration.
type SecurityConfig struct {
	EncryptionKey []byte // 32-byte key for AES-256-GCM
	JWTSecret     string
	APIKeyPrefix  string
}

// RateLimitConfig holds rate limiting configuration.
type RateLimitConfig struct {
	Enabled           bool
	RequestsPerSecond int
	Burst             int
}

// IdempotencyConfig holds idempotency configuration.
type IdempotencyConfig struct {
	TTL time.Duration
}

// WebhookConfig holds outbound webhook configuration.
type WebhookConfig struct {
	Timeout       time.Duration
	MaxRetries    int
	RetryBackoff  string
	SigningSecret string
}

// WorkerConfig holds Asynq worker configuration.
type WorkerConfig struct {
	Concurrency   int
	QueueCritical int
	QueueDefault  int
	QueueLow      int
}

// ObservabilityConfig holds observability configuration.
type ObservabilityConfig struct {
	OTELEnabled     bool
	OTELEndpoint    string
	OTELServiceName string
	MetricsEnabled  bool
	MetricsPort     int
	MetricsPath     string
}

// IyzicoConfig holds Iyzico payment gateway configuration.
type IyzicoConfig struct {
	APIKey      string
	SecretKey   string
	BaseURL     string
	CallbackURL string
}

// ProviderConfig holds configuration for a single provider.
type ProviderConfig struct {
	WSDLURL        string
	BaseURL        string
	Username       string
	Password       string
	APIKey         string
	SecretKey      string
	CustomerCode   string
	CustomerID     string
	CustomerNumber string
	UserLanguage   string
}

// ProvidersConfig holds all provider configurations.
type ProvidersConfig struct {
	Aras        ProviderConfig
	Yurtici     ProviderConfig
	Surat       ProviderConfig
	PTT         ProviderConfig
	Hepsijet    ProviderConfig
	MNG         ProviderConfig
	Kolaygelsin ProviderConfig
	Sendeo      ProviderConfig
	Pakettaxi   ProviderConfig
}

// FeaturesConfig holds feature flag configuration.
type FeaturesConfig struct {
	ArasEnabled        bool
	YurticiEnabled     bool
	SuratEnabled       bool
	PTTEnabled         bool
	HepsijetEnabled    bool
	MNGEnabled         bool
	KolaygelsinEnabled bool
	SendeoEnabled      bool
	PakettaxiEnabled   bool
	CODEnabled         bool
	BYOAEnabled        bool
	WebhooksEnabled    bool
}

// CORSConfig holds CORS configuration.
type CORSConfig struct {
	Enabled        bool
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	ExposedHeaders []string
	MaxAge         int
}

// RetentionConfig holds data retention configuration.
type RetentionConfig struct {
	AuditLogsDays      int
	ProviderLogsDays   int
	TrackingEventsDays int
	DeletedDataDays    int
}

// Load loads configuration from environment variables.
// It optionally loads from .env file if present.
func Load() (*Config, error) {
	// Load .env file if it exists (ignore error if not found)
	_ = godotenv.Load()

	cfg := &Config{}

	var errs []error

	// App config
	cfg.App.Env = Environment(getEnvOrDefault("APP_ENV", "development"))
	cfg.App.Name = getEnvOrDefault("APP_NAME", "shipping-aggregator")
	cfg.App.Version = getEnvOrDefault("APP_VERSION", "1.0.0")
	cfg.App.Debug = getEnvBool("APP_DEBUG", true)
	cfg.App.LogLevel = getEnvOrDefault("LOG_LEVEL", "debug")
	cfg.App.LogFormat = getEnvOrDefault("LOG_FORMAT", "json")

	// API config
	cfg.API.Host = getEnvOrDefault("API_HOST", "0.0.0.0")
	cfg.API.Port = getEnvInt("API_PORT", 8080)
	cfg.API.ReadTimeout = getEnvDuration("API_READ_TIMEOUT", 30*time.Second)
	cfg.API.WriteTimeout = getEnvDuration("API_WRITE_TIMEOUT", 30*time.Second)
	cfg.API.IdleTimeout = getEnvDuration("API_IDLE_TIMEOUT", 120*time.Second)
	cfg.API.ShutdownTimeout = getEnvDuration("API_SHUTDOWN_TIMEOUT", 30*time.Second)

	// Database config
	cfg.Database.Host = getEnvOrDefault("DB_HOST", "localhost")
	cfg.Database.Port = getEnvInt("DB_PORT", 5432)
	cfg.Database.User = getEnvOrDefault("DB_USER", "shipping")
	cfg.Database.Password = getEnvOrDefault("DB_PASSWORD", "shipping")
	cfg.Database.Name = getEnvOrDefault("DB_NAME", "shipping")
	cfg.Database.SSLMode = getEnvOrDefault("DB_SSLMODE", "disable")
	cfg.Database.MaxOpenConns = getEnvInt("DB_MAX_OPEN_CONNS", 25)
	cfg.Database.MaxIdleConns = getEnvInt("DB_MAX_IDLE_CONNS", 5)
	cfg.Database.ConnMaxLifetime = getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute)
	cfg.Database.ConnMaxIdleTime = getEnvDuration("DB_CONN_MAX_IDLE_TIME", 5*time.Minute)
	cfg.Database.URL = os.Getenv("DATABASE_URL")

	// Redis config
	cfg.Redis.Host = getEnvOrDefault("REDIS_HOST", "localhost")
	cfg.Redis.Port = getEnvInt("REDIS_PORT", 6379)
	cfg.Redis.Password = os.Getenv("REDIS_PASSWORD")
	cfg.Redis.DB = getEnvInt("REDIS_DB", 0)
	cfg.Redis.MaxRetries = getEnvInt("REDIS_MAX_RETRIES", 3)
	cfg.Redis.PoolSize = getEnvInt("REDIS_POOL_SIZE", 10)
	cfg.Redis.URL = os.Getenv("REDIS_URL")

	// Storage config
	cfg.Storage.Endpoint = getEnvOrDefault("MINIO_ENDPOINT", "localhost:9000")
	cfg.Storage.AccessKey = getEnvOrDefault("MINIO_ACCESS_KEY", "minioadmin")
	cfg.Storage.SecretKey = getEnvOrDefault("MINIO_SECRET_KEY", "minioadmin")
	cfg.Storage.UseSSL = getEnvBool("MINIO_USE_SSL", false)
	cfg.Storage.BucketDocuments = getEnvOrDefault("MINIO_BUCKET_DOCUMENTS", "shipping-documents")
	cfg.Storage.BucketLabels = getEnvOrDefault("MINIO_BUCKET_LABELS", "shipping-labels")
	cfg.Storage.Region = getEnvOrDefault("MINIO_REGION", "tr-istanbul")

	// Security config
	encKeyHex := os.Getenv("ENCRYPTION_KEY")
	if encKeyHex != "" {
		key, err := parseHexKey(encKeyHex, 32)
		if err != nil {
			errs = append(errs, fmt.Errorf("ENCRYPTION_KEY: %w", err))
		} else {
			cfg.Security.EncryptionKey = key
		}
	}
	cfg.Security.JWTSecret = os.Getenv("JWT_SECRET")
	cfg.Security.APIKeyPrefix = getEnvOrDefault("API_KEY_PREFIX", "sk_live_")

	// Rate limit config
	cfg.RateLimit.Enabled = getEnvBool("RATE_LIMIT_ENABLED", true)
	cfg.RateLimit.RequestsPerSecond = getEnvInt("RATE_LIMIT_REQUESTS_PER_SECOND", 100)
	cfg.RateLimit.Burst = getEnvInt("RATE_LIMIT_BURST", 200)

	// Idempotency config
	cfg.Idempotency.TTL = getEnvDuration("IDEMPOTENCY_TTL", 24*time.Hour)

	// Webhook config
	cfg.Webhook.Timeout = getEnvDuration("WEBHOOK_TIMEOUT", 30*time.Second)
	cfg.Webhook.MaxRetries = getEnvInt("WEBHOOK_MAX_RETRIES", 7)
	cfg.Webhook.RetryBackoff = getEnvOrDefault("WEBHOOK_RETRY_BACKOFF", "exponential")
	cfg.Webhook.SigningSecret = os.Getenv("WEBHOOK_SIGNING_SECRET")

	// Worker config
	cfg.Worker.Concurrency = getEnvInt("ASYNQ_CONCURRENCY", 10)
	cfg.Worker.QueueCritical = getEnvInt("ASYNQ_QUEUE_CRITICAL", 6)
	cfg.Worker.QueueDefault = getEnvInt("ASYNQ_QUEUE_DEFAULT", 3)
	cfg.Worker.QueueLow = getEnvInt("ASYNQ_QUEUE_LOW", 1)

	// Observability config
	cfg.Observability.OTELEnabled = getEnvBool("OTEL_ENABLED", false)
	cfg.Observability.OTELEndpoint = getEnvOrDefault("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317")
	cfg.Observability.OTELServiceName = getEnvOrDefault("OTEL_SERVICE_NAME", "shipping-aggregator")
	cfg.Observability.MetricsEnabled = getEnvBool("METRICS_ENABLED", true)
	cfg.Observability.MetricsPort = getEnvInt("METRICS_PORT", 9090)
	cfg.Observability.MetricsPath = getEnvOrDefault("METRICS_PATH", "/metrics")

	// Iyzico config
	cfg.Iyzico.APIKey = os.Getenv("IYZICO_API_KEY")
	cfg.Iyzico.SecretKey = os.Getenv("IYZICO_SECRET_KEY")
	cfg.Iyzico.BaseURL = getEnvOrDefault("IYZICO_BASE_URL", "https://sandbox-api.iyzipay.com")
	cfg.Iyzico.CallbackURL = os.Getenv("IYZICO_CALLBACK_URL")

	// Provider configs
	cfg.Providers.Aras = ProviderConfig{
		WSDLURL:      os.Getenv("ARAS_WSDL_URL"),
		Username:     os.Getenv("ARAS_USERNAME"),
		Password:     os.Getenv("ARAS_PASSWORD"),
		CustomerCode: os.Getenv("ARAS_CUSTOMER_CODE"),
	}
	cfg.Providers.Yurtici = ProviderConfig{
		WSDLURL:      os.Getenv("YURTICI_WSDL_URL"),
		Username:     os.Getenv("YURTICI_USERNAME"),
		Password:     os.Getenv("YURTICI_PASSWORD"),
		UserLanguage: getEnvOrDefault("YURTICI_USER_LANGUAGE", "TR"),
	}
	cfg.Providers.Surat = ProviderConfig{
		WSDLURL:  os.Getenv("SURAT_WSDL_URL"),
		Username: os.Getenv("SURAT_USERNAME"),
		Password: os.Getenv("SURAT_PASSWORD"),
	}
	cfg.Providers.PTT = ProviderConfig{
		WSDLURL:    os.Getenv("PTT_WSDL_URL"),
		Username:   os.Getenv("PTT_USERNAME"),
		Password:   os.Getenv("PTT_PASSWORD"),
		CustomerID: os.Getenv("PTT_CUSTOMER_ID"),
	}
	cfg.Providers.Hepsijet = ProviderConfig{
		BaseURL:   os.Getenv("HEPSIJET_BASE_URL"),
		APIKey:    os.Getenv("HEPSIJET_API_KEY"),
		SecretKey: os.Getenv("HEPSIJET_SECRET_KEY"),
	}
	cfg.Providers.MNG = ProviderConfig{
		WSDLURL:        os.Getenv("MNG_WSDL_URL"),
		Username:       os.Getenv("MNG_USERNAME"),
		Password:       os.Getenv("MNG_PASSWORD"),
		CustomerNumber: os.Getenv("MNG_CUSTOMER_NUMBER"),
	}
	cfg.Providers.Kolaygelsin = ProviderConfig{
		BaseURL:   os.Getenv("KOLAYGELSIN_BASE_URL"),
		APIKey:    os.Getenv("KOLAYGELSIN_API_KEY"),
		SecretKey: os.Getenv("KOLAYGELSIN_SECRET_KEY"),
	}
	cfg.Providers.Sendeo = ProviderConfig{
		BaseURL:    os.Getenv("SENDEO_BASE_URL"),
		APIKey:     os.Getenv("SENDEO_API_KEY"),
		CustomerID: os.Getenv("SENDEO_CUSTOMER_ID"),
	}
	cfg.Providers.Pakettaxi = ProviderConfig{
		BaseURL:   os.Getenv("PAKETTAXI_BASE_URL"),
		APIKey:    os.Getenv("PAKETTAXI_API_KEY"),
		SecretKey: os.Getenv("PAKETTAXI_SECRET_KEY"),
	}

	// Feature flags
	cfg.Features.ArasEnabled = getEnvBool("FEATURE_ARAS_ENABLED", true)
	cfg.Features.YurticiEnabled = getEnvBool("FEATURE_YURTICI_ENABLED", true)
	cfg.Features.SuratEnabled = getEnvBool("FEATURE_SURAT_ENABLED", true)
	cfg.Features.PTTEnabled = getEnvBool("FEATURE_PTT_ENABLED", true)
	cfg.Features.HepsijetEnabled = getEnvBool("FEATURE_HEPSIJET_ENABLED", true)
	cfg.Features.MNGEnabled = getEnvBool("FEATURE_MNG_ENABLED", true)
	cfg.Features.KolaygelsinEnabled = getEnvBool("FEATURE_KOLAYGELSIN_ENABLED", true)
	cfg.Features.SendeoEnabled = getEnvBool("FEATURE_SENDEO_ENABLED", true)
	cfg.Features.PakettaxiEnabled = getEnvBool("FEATURE_PAKETTAXI_ENABLED", true)
	cfg.Features.CODEnabled = getEnvBool("FEATURE_COD_ENABLED", true)
	cfg.Features.BYOAEnabled = getEnvBool("FEATURE_BYOA_ENABLED", true)
	cfg.Features.WebhooksEnabled = getEnvBool("FEATURE_WEBHOOKS_ENABLED", true)

	// CORS config
	cfg.CORS.Enabled = getEnvBool("CORS_ENABLED", true)
	cfg.CORS.AllowedOrigins = getEnvStringSlice("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000", "http://localhost:8080"})
	cfg.CORS.AllowedMethods = getEnvStringSlice("CORS_ALLOWED_METHODS", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"})
	cfg.CORS.AllowedHeaders = getEnvStringSlice("CORS_ALLOWED_HEADERS", []string{"Authorization", "Content-Type", "X-Idempotency-Key", "X-Trace-ID"})
	cfg.CORS.ExposedHeaders = getEnvStringSlice("CORS_EXPOSED_HEADERS", []string{"X-Trace-ID", "X-Request-ID", "X-Idempotency-Replayed"})
	cfg.CORS.MaxAge = getEnvInt("CORS_MAX_AGE", 86400)

	// Retention config
	cfg.Retention.AuditLogsDays = getEnvInt("RETENTION_AUDIT_LOGS_DAYS", 365)
	cfg.Retention.ProviderLogsDays = getEnvInt("RETENTION_PROVIDER_LOGS_DAYS", 30)
	cfg.Retention.TrackingEventsDays = getEnvInt("RETENTION_TRACKING_EVENTS_DAYS", 90)
	cfg.Retention.DeletedDataDays = getEnvInt("RETENTION_DELETED_DATA_DAYS", 30)

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	return cfg, nil
}

// Validate validates the configuration for production readiness.
func (c *Config) Validate() error {
	var errs []error

	// Validate environment
	switch c.App.Env {
	case EnvDevelopment, EnvStaging, EnvProduction:
		// Valid
	default:
		errs = append(errs, fmt.Errorf("invalid APP_ENV: %s (must be development, staging, or production)", c.App.Env))
	}

	// Production-specific validations
	if c.App.Env == EnvProduction {
		if c.App.Debug {
			errs = append(errs, errors.New("APP_DEBUG must be false in production"))
		}

		if len(c.Security.EncryptionKey) != 32 {
			errs = append(errs, errors.New("ENCRYPTION_KEY must be a 32-byte hex-encoded key in production"))
		}

		if c.Security.JWTSecret == "" || c.Security.JWTSecret == "your-jwt-secret-key-change-in-production" {
			errs = append(errs, errors.New("JWT_SECRET must be set to a secure value in production"))
		}

		if c.Webhook.SigningSecret == "" || c.Webhook.SigningSecret == "your-webhook-signing-secret" {
			errs = append(errs, errors.New("WEBHOOK_SIGNING_SECRET must be set to a secure value in production"))
		}

		if c.Database.SSLMode == "disable" {
			errs = append(errs, errors.New("DB_SSLMODE should not be 'disable' in production"))
		}
	}

	// Validate API port
	if c.API.Port < 1 || c.API.Port > 65535 {
		errs = append(errs, fmt.Errorf("invalid API_PORT: %d (must be 1-65535)", c.API.Port))
	}

	// Validate database connection pool
	if c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		errs = append(errs, errors.New("DB_MAX_IDLE_CONNS cannot be greater than DB_MAX_OPEN_CONNS"))
	}

	// Validate rate limit
	if c.RateLimit.Enabled && c.RateLimit.RequestsPerSecond <= 0 {
		errs = append(errs, errors.New("RATE_LIMIT_REQUESTS_PER_SECOND must be positive when rate limiting is enabled"))
	}

	// Validate webhook retry backoff
	switch c.Webhook.RetryBackoff {
	case "exponential", "linear":
		// Valid
	default:
		errs = append(errs, fmt.Errorf("invalid WEBHOOK_RETRY_BACKOFF: %s (must be exponential or linear)", c.Webhook.RetryBackoff))
	}

	// Validate log level
	switch c.App.LogLevel {
	case "debug", "info", "warn", "error":
		// Valid
	default:
		errs = append(errs, fmt.Errorf("invalid LOG_LEVEL: %s (must be debug, info, warn, or error)", c.App.LogLevel))
	}

	// Validate log format
	switch c.App.LogFormat {
	case "json", "text":
		// Valid
	default:
		errs = append(errs, fmt.Errorf("invalid LOG_FORMAT: %s (must be json or text)", c.App.LogFormat))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

// IsProduction returns true if running in production environment.
func (c *Config) IsProduction() bool {
	return c.App.Env == EnvProduction
}

// IsDevelopment returns true if running in development environment.
func (c *Config) IsDevelopment() bool {
	return c.App.Env == EnvDevelopment
}

// APIAddr returns the API server address.
func (c *Config) APIAddr() string {
	return fmt.Sprintf("%s:%d", c.API.Host, c.API.Port)
}

// Helper functions

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}

func getEnvStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return defaultValue
}

func parseHexKey(hexStr string, expectedLen int) ([]byte, error) {
	// Remove any whitespace
	hexStr = strings.TrimSpace(hexStr)

	// Check length (2 hex chars per byte)
	if len(hexStr) != expectedLen*2 {
		return nil, fmt.Errorf("expected %d hex characters, got %d", expectedLen*2, len(hexStr))
	}

	// Parse hex string
	key := make([]byte, expectedLen)
	for i := 0; i < expectedLen; i++ {
		b, err := strconv.ParseUint(hexStr[i*2:i*2+2], 16, 8)
		if err != nil {
			return nil, fmt.Errorf("invalid hex at position %d: %w", i*2, err)
		}
		key[i] = byte(b)
	}

	return key, nil
}

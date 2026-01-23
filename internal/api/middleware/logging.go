// Package middleware provides HTTP middleware for the shipping aggregator API.
package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/e-port/shipping-aggregator/internal/observability"
)

// LoggingConfig holds configuration for the logging middleware.
type LoggingConfig struct {
	// Logger is the logger instance.
	Logger *observability.Logger
	// LogRequestBody enables logging of request bodies.
	LogRequestBody bool
	// LogResponseBody enables logging of response bodies.
	LogResponseBody bool
	// MaxBodySize is the maximum body size to log (default: 4KB).
	MaxBodySize int
	// SkipPaths are paths that should not be logged.
	SkipPaths []string
	// SkipHealthChecks skips logging for health check endpoints.
	SkipHealthChecks bool
	// SlowRequestThreshold is the threshold for logging slow requests (default: 1s).
	SlowRequestThreshold time.Duration
	// SensitiveHeaders are headers that should be masked in logs.
	SensitiveHeaders []string
	// SensitivePaths are paths where request/response bodies should not be logged.
	SensitivePaths []string
}

// DefaultLoggingConfig returns a default logging configuration.
func DefaultLoggingConfig() LoggingConfig {
	return LoggingConfig{
		LogRequestBody:       false,
		LogResponseBody:      false,
		MaxBodySize:          4096, // 4KB
		SkipHealthChecks:     true,
		SlowRequestThreshold: time.Second,
		SensitiveHeaders: []string{
			"Authorization",
			"X-API-Key",
			"Cookie",
			"Set-Cookie",
		},
		SensitivePaths: []string{
			"/auth/login",
			"/auth/register",
			"/billing/topups",
		},
	}
}

// Logging returns a middleware that logs HTTP requests and responses.
// It captures request/response details, timing, and optionally bodies.
func Logging(cfg LoggingConfig) func(http.Handler) http.Handler {
	// Set defaults
	if cfg.MaxBodySize == 0 {
		cfg.MaxBodySize = 4096
	}
	if cfg.SlowRequestThreshold == 0 {
		cfg.SlowRequestThreshold = time.Second
	}
	if len(cfg.SensitiveHeaders) == 0 {
		cfg.SensitiveHeaders = DefaultLoggingConfig().SensitiveHeaders
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip logging for certain paths
			if shouldSkipLogging(r.URL.Path, cfg) {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			ctx := r.Context()

			// Capture request body if enabled
			var requestBody string
			if cfg.LogRequestBody && !isSensitivePath(r.URL.Path, cfg.SensitivePaths) {
				requestBody = captureRequestBody(r, cfg.MaxBodySize)
			}

			// Wrap response writer to capture status code and body
			ww := newLoggingResponseWriter(w, r.ProtoMajor, cfg.LogResponseBody, cfg.MaxBodySize)

			// Process request
			next.ServeHTTP(ww, r)

			// Calculate duration
			duration := time.Since(start)

			// Build log attributes
			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration_ms", duration.Milliseconds(),
				"bytes_written", ww.BytesWritten(),
				"remote_addr", GetClientIP(r),
				"user_agent", r.UserAgent(),
				"trace_id", GetTraceID(ctx),
				"request_id", GetRequestID(ctx),
			}

			// Add query parameters if present
			if r.URL.RawQuery != "" {
				attrs = append(attrs, "query", r.URL.RawQuery)
			}

			// Add content type
			if ct := r.Header.Get("Content-Type"); ct != "" {
				attrs = append(attrs, "content_type", ct)
			}

			// Add request body if captured
			if requestBody != "" {
				attrs = append(attrs, "request_body", requestBody)
			}

			// Add response body if captured and not sensitive
			if cfg.LogResponseBody && !isSensitivePath(r.URL.Path, cfg.SensitivePaths) {
				if responseBody := ww.Body(); responseBody != "" {
					attrs = append(attrs, "response_body", responseBody)
				}
			}

			// Add masked headers
			maskedHeaders := maskHeaders(r.Header, cfg.SensitiveHeaders)
			if len(maskedHeaders) > 0 {
				attrs = append(attrs, "headers", maskedHeaders)
			}

			// Log at appropriate level based on status and duration
			if cfg.Logger != nil {
				logRequest(cfg.Logger, ww.Status(), duration, cfg.SlowRequestThreshold, attrs)
			}
		})
	}
}

// LoggingMiddleware returns a simple logging middleware with default configuration.
func LoggingMiddleware(logger *observability.Logger) func(http.Handler) http.Handler {
	cfg := DefaultLoggingConfig()
	cfg.Logger = logger
	return Logging(cfg)
}

// loggingResponseWriter wraps http.ResponseWriter to capture response details.
type loggingResponseWriter struct {
	chimiddleware.WrapResponseWriter
	body        *bytes.Buffer
	captureBody bool
	maxBodySize int
}

// newLoggingResponseWriter creates a new logging response writer.
func newLoggingResponseWriter(w http.ResponseWriter, protoMajor int, captureBody bool, maxBodySize int) *loggingResponseWriter {
	ww := chimiddleware.NewWrapResponseWriter(w, protoMajor)
	lrw := &loggingResponseWriter{
		WrapResponseWriter: ww,
		captureBody:        captureBody,
		maxBodySize:        maxBodySize,
	}
	if captureBody {
		lrw.body = &bytes.Buffer{}
	}
	return lrw
}

// Write captures the response body if enabled.
func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	if lrw.captureBody && lrw.body != nil && lrw.body.Len() < lrw.maxBodySize {
		remaining := lrw.maxBodySize - lrw.body.Len()
		if len(b) > remaining {
			lrw.body.Write(b[:remaining])
		} else {
			lrw.body.Write(b)
		}
	}
	return lrw.WrapResponseWriter.Write(b)
}

// Body returns the captured response body.
func (lrw *loggingResponseWriter) Body() string {
	if lrw.body == nil {
		return ""
	}
	return lrw.body.String()
}

// captureRequestBody reads and returns the request body, then restores it.
func captureRequestBody(r *http.Request, maxSize int) string {
	if r.Body == nil {
		return ""
	}

	// Read up to maxSize bytes
	body, err := io.ReadAll(io.LimitReader(r.Body, int64(maxSize)))
	if err != nil {
		return ""
	}

	// Restore the body for the handler
	r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), r.Body))

	return string(body)
}

// shouldSkipLogging checks if logging should be skipped for a path.
func shouldSkipLogging(path string, cfg LoggingConfig) bool {
	// Skip health checks if configured
	if cfg.SkipHealthChecks {
		if strings.HasPrefix(path, "/health") {
			return true
		}
	}

	// Check skip paths
	for _, skip := range cfg.SkipPaths {
		if path == skip || strings.HasPrefix(path, skip) {
			return true
		}
	}

	return false
}

// isSensitivePath checks if a path is sensitive.
func isSensitivePath(path string, sensitivePaths []string) bool {
	for _, sensitive := range sensitivePaths {
		if strings.Contains(path, sensitive) {
			return true
		}
	}
	return false
}

// maskHeaders masks sensitive headers.
func maskHeaders(headers http.Header, sensitiveHeaders []string) map[string]string {
	result := make(map[string]string)
	sensitiveSet := make(map[string]struct{})
	for _, h := range sensitiveHeaders {
		sensitiveSet[strings.ToLower(h)] = struct{}{}
	}

	for key, values := range headers {
		if _, sensitive := sensitiveSet[strings.ToLower(key)]; sensitive {
			result[key] = "[REDACTED]"
		} else if len(values) > 0 {
			result[key] = values[0]
		}
	}

	return result
}

// logRequest logs the request at the appropriate level.
func logRequest(logger *observability.Logger, status int, duration, slowThreshold time.Duration, attrs []any) {
	// Determine log level based on status code and duration
	switch {
	case status >= 500:
		logger.Error("http request", attrs...)
	case status >= 400:
		logger.Warn("http request", attrs...)
	case duration > slowThreshold:
		// Log slow requests as warnings
		attrs = append(attrs, "slow_request", true)
		logger.Warn("http request (slow)", attrs...)
	default:
		logger.Info("http request", attrs...)
	}
}

// RequestLogger is a structured request logger that can be used for detailed logging.
type RequestLogger struct {
	logger *observability.Logger
	config LoggingConfig
}

// NewRequestLogger creates a new request logger.
func NewRequestLogger(logger *observability.Logger, config LoggingConfig) *RequestLogger {
	return &RequestLogger{
		logger: logger,
		config: config,
	}
}

// LogRequest logs a request with the given details.
func (rl *RequestLogger) LogRequest(r *http.Request, status int, duration time.Duration, bytesWritten int) {
	ctx := r.Context()

	attrs := []any{
		"method", r.Method,
		"path", r.URL.Path,
		"status", status,
		"duration_ms", duration.Milliseconds(),
		"bytes_written", bytesWritten,
		"remote_addr", GetClientIP(r),
		"user_agent", r.UserAgent(),
		"trace_id", GetTraceID(ctx),
		"request_id", GetRequestID(ctx),
	}

	logRequest(rl.logger, status, duration, rl.config.SlowRequestThreshold, attrs)
}

// LogError logs an error that occurred during request processing.
func (rl *RequestLogger) LogError(r *http.Request, err error, attrs ...any) {
	ctx := r.Context()

	baseAttrs := []any{
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", GetClientIP(r),
		"trace_id", GetTraceID(ctx),
		"request_id", GetRequestID(ctx),
		"error", err.Error(),
	}

	baseAttrs = append(baseAttrs, attrs...)
	rl.logger.Error("request error", baseAttrs...)
}

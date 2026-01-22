// Package middleware provides HTTP middleware for the shipping aggregator API.
// It includes authentication, rate limiting, tracing, and other cross-cutting concerns.
package middleware

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/e-port/shipping-aggregator/internal/observability"
)

// TraceIDHeader is the header name for trace ID.
const TraceIDHeader = "X-Trace-ID"

// RequestIDHeader is the header name for request ID.
const RequestIDHeader = "X-Request-ID"

// TraceConfig holds configuration for the trace middleware.
type TraceConfig struct {
	// Logger is the logger for trace events.
	Logger *observability.Logger
	// TrustIncomingTraceID determines whether to trust incoming X-Trace-ID headers.
	// Set to false in production if you don't trust upstream proxies.
	TrustIncomingTraceID bool
	// TrustForwardedHeaders determines whether to trust forwarded headers.
	// This is an alias for TrustIncomingTraceID for convenience.
	TrustForwardedHeaders bool
	// GenerateRequestID determines whether to generate a request ID if not provided.
	GenerateRequestID bool
}

// DefaultTraceConfig returns a default trace configuration.
func DefaultTraceConfig() TraceConfig {
	return TraceConfig{
		TrustIncomingTraceID: true,
		GenerateRequestID:    true,
	}
}

// Trace returns a middleware that propagates trace_id and request_id.
// It extracts trace_id from the X-Trace-ID header (if trusted) or generates a new one.
// It always generates a new request_id for each request.
// Both IDs are added to the response headers and request context.
func Trace(cfg TraceConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract or generate trace ID
			traceID := ""
			trustHeaders := cfg.TrustIncomingTraceID || cfg.TrustForwardedHeaders
			if trustHeaders {
				traceID = r.Header.Get(TraceIDHeader)
			}
			if traceID == "" {
				traceID = generateID()
			}

			// Always generate a new request ID
			requestID := ""
			if cfg.GenerateRequestID {
				requestID = generateID()
			}

			// Add IDs to response headers
			w.Header().Set(TraceIDHeader, traceID)
			if requestID != "" {
				w.Header().Set(RequestIDHeader, requestID)
			}

			// Add IDs to context using observability package context helpers
			ctx := r.Context()
			ctx = observability.ContextWithTraceID(ctx, traceID)
			if requestID != "" {
				ctx = observability.ContextWithRequestID(ctx, requestID)
			}

			// Continue with the request
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// TraceMiddleware is a convenience function that returns the trace middleware with default config.
func TraceMiddleware(next http.Handler) http.Handler {
	return Trace(DefaultTraceConfig())(next)
}

// generateID generates a new UUID v4 string.
func generateID() string {
	return uuid.NewString()
}

// GetTraceID extracts the trace ID from the context.
// Returns an empty string if not found.
func GetTraceID(ctx context.Context) string {
	return observability.TraceIDFromContext(ctx)
}

// GetRequestID extracts the request ID from the context.
// Returns an empty string if not found.
func GetRequestID(ctx context.Context) string {
	return observability.RequestIDFromContext(ctx)
}

// WithTraceID adds a trace ID to the context.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return observability.ContextWithTraceID(ctx, traceID)
}

// WithRequestID adds a request ID to the context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return observability.ContextWithRequestID(ctx, requestID)
}

// TraceFromRequest extracts trace and request IDs from an HTTP request.
// This is useful for propagating trace context to downstream services.
func TraceFromRequest(r *http.Request) (traceID, requestID string) {
	traceID = GetTraceID(r.Context())
	requestID = GetRequestID(r.Context())
	return
}

// InjectTraceHeaders injects trace and request IDs into HTTP request headers.
// This is useful for propagating trace context to downstream services.
func InjectTraceHeaders(r *http.Request, traceID, requestID string) {
	if traceID != "" {
		r.Header.Set(TraceIDHeader, traceID)
	}
	if requestID != "" {
		r.Header.Set(RequestIDHeader, requestID)
	}
}

// PropagateTrace creates a new HTTP request with trace context propagated from the given context.
func PropagateTrace(ctx context.Context, r *http.Request) *http.Request {
	traceID := GetTraceID(ctx)
	requestID := GetRequestID(ctx)
	InjectTraceHeaders(r, traceID, requestID)
	return r
}

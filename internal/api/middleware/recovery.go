// Package middleware provides HTTP middleware for the shipping aggregator API.
package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"runtime/debug"

	"github.com/knid/kargonet/internal/observability"
)

// RecoveryConfig holds configuration for the recovery middleware.
type RecoveryConfig struct {
	// Logger is the logger instance.
	Logger *observability.Logger
	// StackTrace enables stack trace logging.
	StackTrace bool
	// StackSize is the maximum stack trace size (default: 4KB).
	StackSize int
	// PrintStack prints the stack trace to stderr.
	PrintStack bool
	// PanicHandler is a custom panic handler.
	// If nil, the default handler is used.
	PanicHandler func(w http.ResponseWriter, r *http.Request, err interface{}, stack []byte)
	// EnableMetrics enables panic metrics.
	EnableMetrics bool
	// Metrics is the metrics collector.
	Metrics *observability.Metrics
}

// DefaultRecoveryConfig returns a default recovery configuration.
func DefaultRecoveryConfig() RecoveryConfig {
	return RecoveryConfig{
		StackTrace: true,
		StackSize:  4096, // 4KB
		PrintStack: false,
	}
}

// Recovery returns a middleware that recovers from panics.
// It logs the panic, optionally captures the stack trace, and returns a 500 error.
func Recovery(cfg RecoveryConfig) func(http.Handler) http.Handler {
	// Set defaults
	if cfg.StackSize == 0 {
		cfg.StackSize = 4096
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// Capture stack trace
					var stack []byte
					if cfg.StackTrace {
						stack = make([]byte, cfg.StackSize)
						n := runtime.Stack(stack, false)
						stack = stack[:n]
					}

					// Log the panic
					logPanic(cfg, r, err, stack)

					// Record metrics
					if cfg.EnableMetrics && cfg.Metrics != nil {
						cfg.Metrics.RecordPanic(r.Method, r.URL.Path)
					}

					// Use custom handler if provided
					if cfg.PanicHandler != nil {
						cfg.PanicHandler(w, r, err, stack)
						return
					}

					// Default: write error response
					writePanicError(w, r, err)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// RecoveryMiddleware returns a simple recovery middleware with default configuration.
func RecoveryMiddleware(logger *observability.Logger) func(http.Handler) http.Handler {
	cfg := DefaultRecoveryConfig()
	cfg.Logger = logger
	return Recovery(cfg)
}

// logPanic logs the panic with context.
func logPanic(cfg RecoveryConfig, r *http.Request, err interface{}, stack []byte) {
	ctx := r.Context()

	attrs := []any{
		"panic", fmt.Sprintf("%v", err),
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", GetClientIP(r),
		"trace_id", GetTraceID(ctx),
		"request_id", GetRequestID(ctx),
	}

	if cfg.StackTrace && len(stack) > 0 {
		attrs = append(attrs, "stack", string(stack))
	}

	if cfg.Logger != nil {
		cfg.Logger.Error("panic recovered", attrs...)
	}

	// Optionally print to stderr
	if cfg.PrintStack && len(stack) > 0 {
		fmt.Printf("PANIC: %v\n%s\n", err, stack)
	}
}

// writePanicError writes a panic error response.
func writePanicError(w http.ResponseWriter, r *http.Request, err interface{}) {
	// Check if headers have already been written
	if w.Header().Get("Content-Type") != "" {
		// Headers already written, can't send error response
		return
	}

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusInternalServerError)

	traceID := GetTraceID(r.Context())
	requestID := GetRequestID(r.Context())

	problem := map[string]interface{}{
		"type":       "https://api.shipaggregator.com/errors/internal-error",
		"title":      "Internal Server Error",
		"status":     http.StatusInternalServerError,
		"detail":     "An unexpected error occurred. Please try again later.",
		"code":       "INTERNAL_ERROR",
		"trace_id":   traceID,
		"request_id": requestID,
		"instance":   r.URL.Path,
	}

	_ = json.NewEncoder(w).Encode(problem)
}

// PanicInfo contains information about a recovered panic.
type PanicInfo struct {
	// Error is the panic value.
	Error interface{}
	// Stack is the stack trace.
	Stack []byte
	// Method is the HTTP method.
	Method string
	// Path is the request path.
	Path string
	// TraceID is the trace ID.
	TraceID string
	// RequestID is the request ID.
	RequestID string
}

// NewPanicInfo creates a new PanicInfo from a request and panic.
func NewPanicInfo(r *http.Request, err interface{}, stack []byte) *PanicInfo {
	return &PanicInfo{
		Error:     err,
		Stack:     stack,
		Method:    r.Method,
		Path:      r.URL.Path,
		TraceID:   GetTraceID(r.Context()),
		RequestID: GetRequestID(r.Context()),
	}
}

// String returns a string representation of the panic info.
func (p *PanicInfo) String() string {
	return fmt.Sprintf("panic: %v\nmethod: %s\npath: %s\ntrace_id: %s\nrequest_id: %s\nstack:\n%s",
		p.Error, p.Method, p.Path, p.TraceID, p.RequestID, string(p.Stack))
}

// RecoveryWithNotifier returns a recovery middleware that notifies on panics.
// This is useful for sending alerts to external services.
func RecoveryWithNotifier(cfg RecoveryConfig, notifier func(*PanicInfo)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					// Capture stack trace
					stack := debug.Stack()

					// Create panic info
					info := NewPanicInfo(r, err, stack)

					// Log the panic
					logPanic(cfg, r, err, stack)

					// Notify
					if notifier != nil {
						go notifier(info)
					}

					// Record metrics
					if cfg.EnableMetrics && cfg.Metrics != nil {
						cfg.Metrics.RecordPanic(r.Method, r.URL.Path)
					}

					// Write error response
					writePanicError(w, r, err)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}

// SafeHandler wraps a handler function to recover from panics.
// This is useful for wrapping individual handlers.
func SafeHandler(logger *observability.Logger, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()

				if logger != nil {
					logger.Error("handler panic recovered",
						"panic", fmt.Sprintf("%v", err),
						"method", r.Method,
						"path", r.URL.Path,
						"stack", string(stack),
						"trace_id", GetTraceID(r.Context()),
						"request_id", GetRequestID(r.Context()),
					)
				}

				writePanicError(w, r, err)
			}
		}()

		handler(w, r)
	}
}

// SafeHandlerFunc is a type alias for handlers that can return an error.
type SafeHandlerFunc func(w http.ResponseWriter, r *http.Request) error

// WrapSafeHandler wraps a SafeHandlerFunc to handle errors and panics.
func WrapSafeHandler(logger *observability.Logger, handler SafeHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				stack := debug.Stack()

				if logger != nil {
					logger.Error("handler panic recovered",
						"panic", fmt.Sprintf("%v", err),
						"method", r.Method,
						"path", r.URL.Path,
						"stack", string(stack),
						"trace_id", GetTraceID(r.Context()),
						"request_id", GetRequestID(r.Context()),
					)
				}

				writePanicError(w, r, err)
			}
		}()

		if err := handler(w, r); err != nil {
			if logger != nil {
				logger.Error("handler error",
					"error", err.Error(),
					"method", r.Method,
					"path", r.URL.Path,
					"trace_id", GetTraceID(r.Context()),
					"request_id", GetRequestID(r.Context()),
				)
			}

			// Write error response if headers haven't been written
			if w.Header().Get("Content-Type") == "" {
				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusInternalServerError)

				problem := map[string]interface{}{
					"type":       "https://api.shipaggregator.com/errors/internal-error",
					"title":      "Internal Server Error",
					"status":     http.StatusInternalServerError,
					"detail":     "An error occurred while processing your request.",
					"code":       "INTERNAL_ERROR",
					"trace_id":   GetTraceID(r.Context()),
					"request_id": GetRequestID(r.Context()),
				}

				_ = json.NewEncoder(w).Encode(problem)
			}
		}
	}
}

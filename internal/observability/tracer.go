package observability

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// TracerConfig holds configuration for the tracer.
type TracerConfig struct {
	Enabled      bool
	Endpoint     string
	ServiceName  string
	Version      string
	Environment  string
	SampleRate   float64
	Insecure     bool
	BatchTimeout time.Duration
}

// DefaultTracerConfig returns a default tracer configuration.
func DefaultTracerConfig() TracerConfig {
	return TracerConfig{
		Enabled:      false,
		Endpoint:     "localhost:4317",
		ServiceName:  "shipping-aggregator",
		Version:      "1.0.0",
		Environment:  "development",
		SampleRate:   1.0, // Sample all traces in development
		Insecure:     true,
		BatchTimeout: 5 * time.Second,
	}
}

// Tracer wraps OpenTelemetry tracer with additional functionality.
type Tracer struct {
	config         TracerConfig
	tracerProvider *sdktrace.TracerProvider
	tracer         trace.Tracer
	propagator     propagation.TextMapPropagator
}

var (
	globalTracer     *Tracer
	globalTracerOnce sync.Once
)

// NewTracer creates a new Tracer instance.
func NewTracer(ctx context.Context, cfg TracerConfig) (*Tracer, error) {
	if !cfg.Enabled {
		// Return a no-op tracer
		return &Tracer{
			config:     cfg,
			tracer:     otel.Tracer(cfg.ServiceName),
			propagator: propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
		}, nil
	}

	// Create OTLP exporter
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptrace.New(ctx, otlptracegrpc.NewClient(opts...))
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource with service information
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.Version),
			attribute.String("deployment.environment", cfg.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create sampler based on sample rate
	var sampler sdktrace.Sampler
	if cfg.SampleRate >= 1.0 {
		sampler = sdktrace.AlwaysSample()
	} else if cfg.SampleRate <= 0.0 {
		sampler = sdktrace.NeverSample()
	} else {
		sampler = sdktrace.TraceIDRatioBased(cfg.SampleRate)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(cfg.BatchTimeout)),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	// Set global propagator
	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
	otel.SetTextMapPropagator(propagator)

	return &Tracer{
		config:         cfg,
		tracerProvider: tp,
		tracer:         tp.Tracer(cfg.ServiceName),
		propagator:     propagator,
	}, nil
}

// Shutdown gracefully shuts down the tracer.
func (t *Tracer) Shutdown(ctx context.Context) error {
	if t.tracerProvider != nil {
		return t.tracerProvider.Shutdown(ctx)
	}
	return nil
}

// Tracer returns the underlying OpenTelemetry tracer.
func (t *Tracer) Tracer() trace.Tracer {
	return t.tracer
}

// Propagator returns the text map propagator.
func (t *Tracer) Propagator() propagation.TextMapPropagator {
	return t.propagator
}

// StartSpan starts a new span with the given name.
func (t *Tracer) StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, opts...)
}

// StartSpanWithAttributes starts a new span with attributes.
func (t *Tracer) StartSpanWithAttributes(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

// SpanFromContext returns the current span from context.
func (t *Tracer) SpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// AddEvent adds an event to the current span.
func (t *Tracer) AddEvent(ctx context.Context, name string, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// SetAttributes sets attributes on the current span.
func (t *Tracer) SetAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attrs...)
}

// RecordError records an error on the current span.
func (t *Tracer) RecordError(ctx context.Context, err error, opts ...trace.EventOption) {
	span := trace.SpanFromContext(ctx)
	span.RecordError(err, opts...)
	span.SetStatus(codes.Error, err.Error())
}

// SetStatus sets the status on the current span.
func (t *Tracer) SetStatus(ctx context.Context, code codes.Code, description string) {
	span := trace.SpanFromContext(ctx)
	span.SetStatus(code, description)
}

// InjectHTTP injects trace context into HTTP headers.
func (t *Tracer) InjectHTTP(ctx context.Context, headers http.Header) {
	t.propagator.Inject(ctx, propagation.HeaderCarrier(headers))
}

// ExtractHTTP extracts trace context from HTTP headers.
func (t *Tracer) ExtractHTTP(ctx context.Context, headers http.Header) context.Context {
	return t.propagator.Extract(ctx, propagation.HeaderCarrier(headers))
}

// HTTPTracingMiddleware returns an HTTP middleware that adds tracing.
func (t *Tracer) HTTPTracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract trace context from incoming request
		ctx := t.ExtractHTTP(r.Context(), r.Header)

		// Start a new span for this request
		spanName := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
		ctx, span := t.tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				semconv.HTTPRequestMethodKey.String(r.Method),
				semconv.URLPath(r.URL.Path),
				semconv.URLScheme(r.URL.Scheme),
				semconv.ServerAddress(r.Host),
				semconv.UserAgentOriginal(r.UserAgent()),
				semconv.ClientAddress(r.RemoteAddr),
			),
		)
		defer span.End()

		// Add trace ID to response headers
		traceID := span.SpanContext().TraceID().String()
		w.Header().Set("X-Trace-ID", traceID)

		// Add trace ID to context for logging
		ctx = ContextWithTraceID(ctx, traceID)

		// Wrap response writer to capture status code
		wrapped := &tracingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Serve the request
		next.ServeHTTP(wrapped, r.WithContext(ctx))

		// Record response attributes
		span.SetAttributes(
			semconv.HTTPResponseStatusCode(wrapped.statusCode),
		)

		// Set span status based on HTTP status code
		if wrapped.statusCode >= 400 {
			span.SetStatus(codes.Error, http.StatusText(wrapped.statusCode))
		} else {
			span.SetStatus(codes.Ok, "")
		}
	})
}

// tracingResponseWriter wraps http.ResponseWriter to capture status code.
type tracingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code.
func (rw *tracingResponseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Common span attributes

// HTTPClientAttributes returns common attributes for HTTP client spans.
func HTTPClientAttributes(method, url string, statusCode int) []attribute.KeyValue {
	return []attribute.KeyValue{
		semconv.HTTPRequestMethodKey.String(method),
		semconv.URLFull(url),
		semconv.HTTPResponseStatusCode(statusCode),
	}
}

// DatabaseAttributes returns common attributes for database spans.
func DatabaseAttributes(system, operation, table string) []attribute.KeyValue {
	return []attribute.KeyValue{
		semconv.DBSystemKey.String(system),
		semconv.DBOperationName(operation),
		semconv.DBCollectionName(table),
	}
}

// ProviderAttributes returns common attributes for provider API spans.
func ProviderAttributes(provider, operation string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("provider.code", provider),
		attribute.String("provider.operation", operation),
	}
}

// ShipmentAttributes returns common attributes for shipment-related spans.
func ShipmentAttributes(shipmentID, status string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("shipment.id", shipmentID),
		attribute.String("shipment.status", status),
	}
}

// OrganizationAttributes returns common attributes for organization context.
func OrganizationAttributes(orgID string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("organization.id", orgID),
	}
}

// SpanHelper provides convenience methods for working with spans.
type SpanHelper struct {
	ctx  context.Context
	span trace.Span
}

// NewSpanHelper creates a new span helper.
func NewSpanHelper(ctx context.Context, span trace.Span) *SpanHelper {
	return &SpanHelper{ctx: ctx, span: span}
}

// Context returns the context with the span.
func (h *SpanHelper) Context() context.Context {
	return h.ctx
}

// Span returns the span.
func (h *SpanHelper) Span() trace.Span {
	return h.span
}

// End ends the span.
func (h *SpanHelper) End() {
	h.span.End()
}

// SetAttributes sets attributes on the span.
func (h *SpanHelper) SetAttributes(attrs ...attribute.KeyValue) {
	h.span.SetAttributes(attrs...)
}

// AddEvent adds an event to the span.
func (h *SpanHelper) AddEvent(name string, attrs ...attribute.KeyValue) {
	h.span.AddEvent(name, trace.WithAttributes(attrs...))
}

// RecordError records an error on the span.
func (h *SpanHelper) RecordError(err error) {
	if err != nil {
		h.span.RecordError(err)
		h.span.SetStatus(codes.Error, err.Error())
	}
}

// SetOK sets the span status to OK.
func (h *SpanHelper) SetOK() {
	h.span.SetStatus(codes.Ok, "")
}

// SetError sets the span status to Error.
func (h *SpanHelper) SetError(msg string) {
	h.span.SetStatus(codes.Error, msg)
}

// Global tracer functions

// SetGlobalTracer sets the global tracer instance.
func SetGlobalTracer(t *Tracer) {
	globalTracer = t
}

// GetGlobalTracer returns the global tracer instance.
// If not set, it creates a default (no-op) tracer.
func GetGlobalTracer() *Tracer {
	globalTracerOnce.Do(func() {
		if globalTracer == nil {
			var err error
			globalTracer, err = NewTracer(context.Background(), DefaultTracerConfig())
			if err != nil {
				// Fall back to no-op tracer
				globalTracer = &Tracer{
					config:     DefaultTracerConfig(),
					tracer:     otel.Tracer("shipping-aggregator"),
					propagator: propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
				}
			}
		}
	})
	return globalTracer
}

// T returns the global tracer for convenience.
func T() *Tracer {
	return GetGlobalTracer()
}

// StartSpan is a convenience function to start a span using the global tracer.
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return GetGlobalTracer().StartSpan(ctx, name, opts...)
}

// StartSpanWithHelper starts a span and returns a helper for convenience.
func StartSpanWithHelper(ctx context.Context, name string, opts ...trace.SpanStartOption) *SpanHelper {
	ctx, span := GetGlobalTracer().StartSpan(ctx, name, opts...)
	return NewSpanHelper(ctx, span)
}

// TraceIDFromSpan extracts the trace ID from a span.
func TraceIDFromSpan(span trace.Span) string {
	if span == nil {
		return ""
	}
	sc := span.SpanContext()
	if !sc.HasTraceID() {
		return ""
	}
	return sc.TraceID().String()
}

// SpanIDFromSpan extracts the span ID from a span.
func SpanIDFromSpan(span trace.Span) string {
	if span == nil {
		return ""
	}
	sc := span.SpanContext()
	if !sc.HasSpanID() {
		return ""
	}
	return sc.SpanID().String()
}

// WithSpan executes a function within a span.
func WithSpan(ctx context.Context, name string, fn func(ctx context.Context) error, opts ...trace.SpanStartOption) error {
	ctx, span := GetGlobalTracer().StartSpan(ctx, name, opts...)
	defer span.End()

	err := fn(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	return err
}

// WithSpanResult executes a function within a span and returns a result.
func WithSpanResult[T any](ctx context.Context, name string, fn func(ctx context.Context) (T, error), opts ...trace.SpanStartOption) (T, error) {
	ctx, span := GetGlobalTracer().StartSpan(ctx, name, opts...)
	defer span.End()

	result, err := fn(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	return result, err
}

// ProviderSpan starts a span for a provider API call.
func ProviderSpan(ctx context.Context, provider, operation string) (context.Context, trace.Span) {
	return GetGlobalTracer().StartSpan(ctx, fmt.Sprintf("provider.%s.%s", provider, operation),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(ProviderAttributes(provider, operation)...),
	)
}

// DatabaseSpan starts a span for a database operation.
func DatabaseSpan(ctx context.Context, operation, table string) (context.Context, trace.Span) {
	return GetGlobalTracer().StartSpan(ctx, fmt.Sprintf("db.%s", operation),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(DatabaseAttributes("postgresql", operation, table)...),
	)
}

// RedisSpan starts a span for a Redis operation.
func RedisSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	return GetGlobalTracer().StartSpan(ctx, fmt.Sprintf("redis.%s", operation),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemRedis,
			semconv.DBOperationName(operation),
		),
	)
}

// HTTPClientSpan starts a span for an HTTP client request.
func HTTPClientSpan(ctx context.Context, method, url string) (context.Context, trace.Span) {
	return GetGlobalTracer().StartSpan(ctx, fmt.Sprintf("HTTP %s", method),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.HTTPRequestMethodKey.String(method),
			semconv.URLFull(url),
		),
	)
}

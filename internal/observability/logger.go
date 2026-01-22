// Package observability provides logging, metrics, and tracing utilities for the shipping aggregator.
// It implements structured logging with PII masking, Prometheus metrics, and OpenTelemetry tracing.
package observability

import (
	"context"
	"io"
	"log/slog"
	"os"
	"regexp"
	"strings"
	"sync"
)

// LogLevel represents the logging level.
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// LogFormat represents the log output format.
type LogFormat string

const (
	LogFormatJSON LogFormat = "json"
	LogFormatText LogFormat = "text"
)

// Context keys for logging.
type contextKey string

const (
	// TraceIDKey is the context key for trace ID.
	TraceIDKey contextKey = "trace_id"
	// RequestIDKey is the context key for request ID.
	RequestIDKey contextKey = "request_id"
	// OrganizationIDKey is the context key for organization ID.
	OrganizationIDKey contextKey = "organization_id"
	// ActorIDKey is the context key for actor ID.
	ActorIDKey contextKey = "actor_id"
	// ActorTypeKey is the context key for actor type.
	ActorTypeKey contextKey = "actor_type"
)

// LoggerConfig holds configuration for the logger.
type LoggerConfig struct {
	Level       LogLevel
	Format      LogFormat
	Output      io.Writer
	ServiceName string
	Version     string
	Environment string
	AddSource   bool
}

// DefaultLoggerConfig returns a default logger configuration.
func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Level:       LogLevelInfo,
		Format:      LogFormatJSON,
		Output:      os.Stdout,
		ServiceName: "shipping-aggregator",
		Version:     "1.0.0",
		Environment: "development",
		AddSource:   false,
	}
}

// PIIMasker handles masking of PII fields in log output.
type PIIMasker struct {
	// piiFields contains field names that should be masked.
	piiFields map[string]struct{}
	// patterns contains regex patterns for detecting PII in values.
	patterns map[string]*regexp.Regexp
	mu       sync.RWMutex
}

// NewPIIMasker creates a new PII masker with default fields.
func NewPIIMasker() *PIIMasker {
	m := &PIIMasker{
		piiFields: make(map[string]struct{}),
		patterns:  make(map[string]*regexp.Regexp),
	}

	// Default PII fields to mask
	defaultFields := []string{
		"phone", "email", "contact_name", "name",
		"street_line_1", "street_line_2", "address",
		"password", "api_key", "secret", "token",
		"credentials", "credit_card", "card_number",
		"cvv", "ssn", "tc_kimlik", "tckn",
	}

	for _, field := range defaultFields {
		m.piiFields[field] = struct{}{}
		m.piiFields[strings.ToLower(field)] = struct{}{}
		m.piiFields[toSnakeCase(field)] = struct{}{}
		m.piiFields[toCamelCase(field)] = struct{}{}
	}

	// Compile regex patterns for detecting PII in values
	m.patterns["email"] = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	m.patterns["phone_tr"] = regexp.MustCompile(`\+90[0-9]{10}`)
	m.patterns["phone_generic"] = regexp.MustCompile(`\+[0-9]{10,15}`)
	m.patterns["api_key"] = regexp.MustCompile(`sk_(live|test)_[a-zA-Z0-9]{32}`)
	m.patterns["session"] = regexp.MustCompile(`sess_[a-zA-Z0-9]{40}`)

	return m
}

// AddField adds a field name to the PII masking list.
func (m *PIIMasker) AddField(field string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.piiFields[field] = struct{}{}
	m.piiFields[strings.ToLower(field)] = struct{}{}
}

// IsPIIField checks if a field name is a PII field.
func (m *PIIMasker) IsPIIField(field string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.piiFields[strings.ToLower(field)]
	return ok
}

// MaskValue masks a PII value based on its type.
func (m *PIIMasker) MaskValue(field string, value any) any {
	if value == nil {
		return nil
	}

	str, ok := value.(string)
	if !ok {
		// For non-string values, return [REDACTED] if it's a PII field
		if m.IsPIIField(field) {
			return "[REDACTED]"
		}
		return value
	}

	if str == "" {
		return str
	}

	fieldLower := strings.ToLower(field)

	// Check if field is a known PII field
	if m.IsPIIField(field) {
		return m.maskByFieldType(fieldLower, str)
	}

	// Check if value matches any PII pattern
	return m.maskByPattern(str)
}

// maskByFieldType masks a value based on the field type.
func (m *PIIMasker) maskByFieldType(field, value string) string {
	switch {
	case strings.Contains(field, "phone"):
		return maskPhone(value)
	case strings.Contains(field, "email"):
		return maskEmail(value)
	case strings.Contains(field, "name"):
		return maskName(value)
	case strings.Contains(field, "street") || strings.Contains(field, "address"):
		return maskAddress(value)
	case strings.Contains(field, "password") || strings.Contains(field, "secret"):
		return "[REDACTED]"
	case strings.Contains(field, "api_key") || strings.Contains(field, "apikey"):
		return maskAPIKey(value)
	case strings.Contains(field, "token"):
		return maskToken(value)
	case strings.Contains(field, "credential"):
		return "[ENCRYPTED]"
	default:
		return maskGeneric(value)
	}
}

// maskByPattern masks a value if it matches any PII pattern.
func (m *PIIMasker) maskByPattern(value string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check each pattern
	for name, pattern := range m.patterns {
		if pattern.MatchString(value) {
			switch name {
			case "email":
				return pattern.ReplaceAllStringFunc(value, maskEmail)
			case "phone_tr", "phone_generic":
				return pattern.ReplaceAllStringFunc(value, maskPhone)
			case "api_key":
				return pattern.ReplaceAllStringFunc(value, maskAPIKey)
			case "session":
				return pattern.ReplaceAllStringFunc(value, maskToken)
			}
		}
	}

	return value
}

// Masking functions for different PII types.

// maskPhone masks a phone number: +9053****1234
func maskPhone(phone string) string {
	if len(phone) < 6 {
		return "****"
	}
	// Keep first 4 chars and last 4 chars
	if len(phone) <= 8 {
		return phone[:2] + "****" + phone[len(phone)-2:]
	}
	return phone[:4] + "****" + phone[len(phone)-4:]
}

// maskEmail masks an email: ab***@example.com
func maskEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return maskGeneric(email)
	}
	local := parts[0]
	domain := parts[1]
	if len(local) <= 2 {
		return local + "***@" + domain
	}
	return local[:2] + "***@" + domain
}

// maskName masks a name: Meh***
func maskName(name string) string {
	if len(name) <= 3 {
		return name[:1] + "***"
	}
	return name[:3] + "***"
}

// maskAddress masks an address: Ata***
func maskAddress(address string) string {
	if len(address) <= 3 {
		return address[:1] + "***"
	}
	return address[:3] + "***"
}

// maskAPIKey masks an API key: sk_live_****xxxx
func maskAPIKey(key string) string {
	if len(key) < 12 {
		return "****"
	}
	// Keep prefix and last 4 chars
	prefixEnd := strings.Index(key, "_")
	if prefixEnd == -1 {
		prefixEnd = 0
	}
	// Find second underscore for sk_live_ pattern
	secondUnderscore := strings.Index(key[prefixEnd+1:], "_")
	if secondUnderscore != -1 {
		prefixEnd = prefixEnd + 1 + secondUnderscore
	}
	if prefixEnd > 0 && prefixEnd < len(key)-4 {
		return key[:prefixEnd+1] + "****" + key[len(key)-4:]
	}
	return "****" + key[len(key)-4:]
}

// maskToken masks a token: sess_****xxxx
func maskToken(token string) string {
	if len(token) < 8 {
		return "****"
	}
	prefixEnd := strings.Index(token, "_")
	if prefixEnd != -1 && prefixEnd < len(token)-4 {
		return token[:prefixEnd+1] + "****" + token[len(token)-4:]
	}
	return "****" + token[len(token)-4:]
}

// maskGeneric masks a generic value: xxx***
func maskGeneric(value string) string {
	if len(value) <= 3 {
		return "***"
	}
	return value[:3] + "***"
}

// PIIMaskingHandler is a slog handler that masks PII fields.
type PIIMaskingHandler struct {
	handler slog.Handler
	masker  *PIIMasker
}

// NewPIIMaskingHandler creates a new PII masking handler.
func NewPIIMaskingHandler(handler slog.Handler, masker *PIIMasker) *PIIMaskingHandler {
	return &PIIMaskingHandler{
		handler: handler,
		masker:  masker,
	}
}

// Enabled reports whether the handler handles records at the given level.
func (h *PIIMaskingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// Handle handles the Record by masking PII fields.
func (h *PIIMaskingHandler) Handle(ctx context.Context, r slog.Record) error {
	// Create a new record with masked attributes
	maskedAttrs := make([]slog.Attr, 0, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		maskedAttrs = append(maskedAttrs, h.maskAttr(a))
		return true
	})

	// Create new record with masked attributes
	newRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	newRecord.AddAttrs(maskedAttrs...)

	return h.handler.Handle(ctx, newRecord)
}

// WithAttrs returns a new handler with the given attributes.
func (h *PIIMaskingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	maskedAttrs := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		maskedAttrs[i] = h.maskAttr(a)
	}
	return &PIIMaskingHandler{
		handler: h.handler.WithAttrs(maskedAttrs),
		masker:  h.masker,
	}
}

// WithGroup returns a new handler with the given group name.
func (h *PIIMaskingHandler) WithGroup(name string) slog.Handler {
	return &PIIMaskingHandler{
		handler: h.handler.WithGroup(name),
		masker:  h.masker,
	}
}

// maskAttr masks a single attribute.
func (h *PIIMaskingHandler) maskAttr(a slog.Attr) slog.Attr {
	// Handle groups recursively
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		maskedAttrs := make([]slog.Attr, len(attrs))
		for i, attr := range attrs {
			maskedAttrs[i] = h.maskAttr(attr)
		}
		return slog.Group(a.Key, attrsToAny(maskedAttrs)...)
	}

	// Mask the value if it's a PII field
	maskedValue := h.masker.MaskValue(a.Key, a.Value.Any())
	return slog.Any(a.Key, maskedValue)
}

// attrsToAny converts a slice of slog.Attr to a slice of any.
func attrsToAny(attrs []slog.Attr) []any {
	result := make([]any, len(attrs))
	for i, a := range attrs {
		result[i] = a
	}
	return result
}

// ContextHandler is a slog handler that extracts context values and adds them to log records.
type ContextHandler struct {
	handler slog.Handler
}

// NewContextHandler creates a new context handler.
func NewContextHandler(handler slog.Handler) *ContextHandler {
	return &ContextHandler{handler: handler}
}

// Enabled reports whether the handler handles records at the given level.
func (h *ContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// Handle handles the Record by adding context values.
func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	// Extract context values and add them to the record
	attrs := extractContextAttrs(ctx)
	if len(attrs) > 0 {
		r.AddAttrs(attrs...)
	}
	return h.handler.Handle(ctx, r)
}

// WithAttrs returns a new handler with the given attributes.
func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ContextHandler{handler: h.handler.WithAttrs(attrs)}
}

// WithGroup returns a new handler with the given group name.
func (h *ContextHandler) WithGroup(name string) slog.Handler {
	return &ContextHandler{handler: h.handler.WithGroup(name)}
}

// extractContextAttrs extracts logging attributes from context.
func extractContextAttrs(ctx context.Context) []slog.Attr {
	var attrs []slog.Attr

	if traceID, ok := ctx.Value(TraceIDKey).(string); ok && traceID != "" {
		attrs = append(attrs, slog.String("trace_id", traceID))
	}
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok && requestID != "" {
		attrs = append(attrs, slog.String("request_id", requestID))
	}
	if orgID, ok := ctx.Value(OrganizationIDKey).(string); ok && orgID != "" {
		attrs = append(attrs, slog.String("organization_id", orgID))
	}
	if actorID, ok := ctx.Value(ActorIDKey).(string); ok && actorID != "" {
		attrs = append(attrs, slog.String("actor_id", actorID))
	}
	if actorType, ok := ctx.Value(ActorTypeKey).(string); ok && actorType != "" {
		attrs = append(attrs, slog.String("actor_type", actorType))
	}

	return attrs
}

// Logger wraps slog.Logger with additional functionality.
type Logger struct {
	*slog.Logger
	masker *PIIMasker
}

// NewLogger creates a new logger with the given configuration.
func NewLogger(cfg LoggerConfig) *Logger {
	// Parse log level
	var level slog.Level
	switch cfg.Level {
	case LogLevelDebug:
		level = slog.LevelDebug
	case LogLevelWarn:
		level = slog.LevelWarn
	case LogLevelError:
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Set output
	output := cfg.Output
	if output == nil {
		output = os.Stdout
	}

	// Create handler options
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: cfg.AddSource,
	}

	// Create base handler based on format
	var baseHandler slog.Handler
	switch cfg.Format {
	case LogFormatText:
		baseHandler = slog.NewTextHandler(output, opts)
	default:
		baseHandler = slog.NewJSONHandler(output, opts)
	}

	// Create PII masker
	masker := NewPIIMasker()

	// Wrap with PII masking handler
	piiHandler := NewPIIMaskingHandler(baseHandler, masker)

	// Wrap with context handler
	contextHandler := NewContextHandler(piiHandler)

	// Create logger with service attributes
	logger := slog.New(contextHandler).With(
		slog.String("service", cfg.ServiceName),
		slog.String("version", cfg.Version),
		slog.String("environment", cfg.Environment),
	)

	return &Logger{
		Logger: logger,
		masker: masker,
	}
}

// WithContext returns a new logger with context values.
func (l *Logger) WithContext(ctx context.Context) *Logger {
	return &Logger{
		Logger: l.Logger,
		masker: l.masker,
	}
}

// WithTraceID returns a new logger with the trace ID.
func (l *Logger) WithTraceID(traceID string) *Logger {
	return &Logger{
		Logger: l.Logger.With(slog.String("trace_id", traceID)),
		masker: l.masker,
	}
}

// WithRequestID returns a new logger with the request ID.
func (l *Logger) WithRequestID(requestID string) *Logger {
	return &Logger{
		Logger: l.Logger.With(slog.String("request_id", requestID)),
		masker: l.masker,
	}
}

// WithOrganization returns a new logger with the organization ID.
func (l *Logger) WithOrganization(orgID string) *Logger {
	return &Logger{
		Logger: l.Logger.With(slog.String("organization_id", orgID)),
		masker: l.masker,
	}
}

// WithActor returns a new logger with the actor information.
func (l *Logger) WithActor(actorID, actorType string) *Logger {
	return &Logger{
		Logger: l.Logger.With(
			slog.String("actor_id", actorID),
			slog.String("actor_type", actorType),
		),
		masker: l.masker,
	}
}

// WithError returns a new logger with the error.
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}
	return &Logger{
		Logger: l.Logger.With(slog.String("error", err.Error())),
		masker: l.masker,
	}
}

// Masker returns the PII masker for this logger.
func (l *Logger) Masker() *PIIMasker {
	return l.masker
}

// Helper functions for case conversion.

// toSnakeCase converts a string to snake_case.
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// toCamelCase converts a string to camelCase.
func toCamelCase(s string) string {
	parts := strings.Split(s, "_")
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// Context helper functions.

// ContextWithTraceID adds a trace ID to the context.
func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// ContextWithRequestID adds a request ID to the context.
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// ContextWithOrganizationID adds an organization ID to the context.
func ContextWithOrganizationID(ctx context.Context, orgID string) context.Context {
	return context.WithValue(ctx, OrganizationIDKey, orgID)
}

// ContextWithActor adds actor information to the context.
func ContextWithActor(ctx context.Context, actorID, actorType string) context.Context {
	ctx = context.WithValue(ctx, ActorIDKey, actorID)
	ctx = context.WithValue(ctx, ActorTypeKey, actorType)
	return ctx
}

// TraceIDFromContext extracts the trace ID from context.
func TraceIDFromContext(ctx context.Context) string {
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok {
		return traceID
	}
	return ""
}

// RequestIDFromContext extracts the request ID from context.
func RequestIDFromContext(ctx context.Context) string {
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// OrganizationIDFromContext extracts the organization ID from context.
func OrganizationIDFromContext(ctx context.Context) string {
	if orgID, ok := ctx.Value(OrganizationIDKey).(string); ok {
		return orgID
	}
	return ""
}

// Global logger instance.
var (
	globalLogger     *Logger
	globalLoggerOnce sync.Once
)

// SetGlobalLogger sets the global logger instance.
func SetGlobalLogger(l *Logger) {
	globalLogger = l
}

// GetGlobalLogger returns the global logger instance.
// If not set, it creates a default logger.
func GetGlobalLogger() *Logger {
	globalLoggerOnce.Do(func() {
		if globalLogger == nil {
			globalLogger = NewLogger(DefaultLoggerConfig())
		}
	})
	return globalLogger
}

// L returns the global logger for convenience.
func L() *Logger {
	return GetGlobalLogger()
}

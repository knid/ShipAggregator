// Package middleware provides HTTP middleware for the shipping aggregator API.
package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/repository/redis"
)

// IdempotencyConfig holds configuration for the idempotency middleware.
type IdempotencyConfig struct {
	// Cache is the Redis cache for storing idempotency keys.
	Cache *redis.Cache
	// Logger is the logger for idempotency events.
	Logger *observability.Logger
	// TTL is the time-to-live for idempotency keys.
	TTL time.Duration
	// HeaderName is the name of the idempotency key header.
	HeaderName string
	// RequiredMethods are HTTP methods that require idempotency keys.
	RequiredMethods []string
	// SkipPaths are paths that don't require idempotency.
	SkipPaths []string
}

// IdempotencyResult represents a cached response for an idempotent request.
type IdempotencyResult struct {
	// StatusCode is the HTTP status code.
	StatusCode int `json:"status_code"`
	// Headers are the response headers.
	Headers map[string][]string `json:"headers"`
	// Body is the response body.
	Body []byte `json:"body"`
	// CreatedAt is when the result was created.
	CreatedAt time.Time `json:"created_at"`
	// RequestHash is the hash of the original request.
	RequestHash string `json:"request_hash"`
}

// IdempotencyState represents the state of an idempotency key.
type IdempotencyState string

const (
	// IdempotencyStatePending indicates the request is being processed.
	IdempotencyStatePending IdempotencyState = "pending"
	// IdempotencyStateCompleted indicates the request has completed.
	IdempotencyStateCompleted IdempotencyState = "completed"
)

// idempotencyContextKey is the context key for idempotency info.
type idempotencyContextKey string

const idempotencyInfoKey idempotencyContextKey = "idempotency_info"

// IdempotencyInfo contains idempotency information for the current request.
type IdempotencyInfo struct {
	// Key is the idempotency key.
	Key string
	// IsReplay indicates if this is a replayed response.
	IsReplay bool
	// RequestHash is the hash of the request.
	RequestHash string
}

// Idempotency returns a middleware that handles idempotent requests.
// It uses the Idempotency-Key header to detect duplicate requests and return cached responses.
func Idempotency(cfg IdempotencyConfig) func(http.Handler) http.Handler {
	// Set defaults
	if cfg.TTL == 0 {
		cfg.TTL = 24 * time.Hour // Default 24 hour TTL
	}
	if cfg.HeaderName == "" {
		cfg.HeaderName = "Idempotency-Key"
	}
	if len(cfg.RequiredMethods) == 0 {
		cfg.RequiredMethods = []string{"POST", "PUT", "PATCH"}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if method requires idempotency
			if !requiresIdempotency(r.Method, cfg.RequiredMethods) {
				next.ServeHTTP(w, r)
				return
			}

			// Check if path should skip idempotency
			if shouldSkipIdempotency(r.URL.Path, cfg.SkipPaths) {
				next.ServeHTTP(w, r)
				return
			}

			// Get idempotency key from header
			idempotencyKey := r.Header.Get(cfg.HeaderName)
			if idempotencyKey == "" {
				// No idempotency key provided, continue without idempotency
				next.ServeHTTP(w, r)
				return
			}

			// Validate idempotency key format (should be a valid UUID or similar)
			if !isValidIdempotencyKey(idempotencyKey) {
				writeIdempotencyError(w, r, "INVALID_IDEMPOTENCY_KEY", "Idempotency key must be a valid UUID", http.StatusBadRequest)
				return
			}

			// Build the cache key including organization ID for isolation
			cacheKey := buildIdempotencyCacheKey(r, idempotencyKey)

			// Calculate request hash for conflict detection
			requestHash, err := calculateRequestHash(r)
			if err != nil {
				if cfg.Logger != nil {
					cfg.Logger.Error("failed to calculate request hash",
						"error", err.Error(),
						"idempotency_key", idempotencyKey,
					)
				}
				next.ServeHTTP(w, r)
				return
			}

			// Check if we have a cached result
			if cfg.Cache != nil {
				cachedResult, err := getCachedResult(r.Context(), cfg.Cache, cacheKey)
				if err != nil {
					if cfg.Logger != nil {
						cfg.Logger.Error("failed to get cached result",
							"error", err.Error(),
							"idempotency_key", idempotencyKey,
						)
					}
				} else if cachedResult != nil {
					// Check for request hash mismatch (different request with same key)
					if cachedResult.RequestHash != "" && cachedResult.RequestHash != requestHash {
						writeIdempotencyError(w, r, "IDEMPOTENCY_KEY_REUSED",
							"Idempotency key has already been used with a different request",
							http.StatusUnprocessableEntity)
						return
					}

					// Return cached response
					replayResponse(w, cachedResult)

					if cfg.Logger != nil {
						cfg.Logger.Debug("replaying cached response",
							"idempotency_key", idempotencyKey,
							"status_code", cachedResult.StatusCode,
						)
					}
					return
				}

				// Try to acquire lock for this idempotency key
				acquired, err := acquireIdempotencyLock(r.Context(), cfg.Cache, cacheKey, cfg.TTL)
				if err != nil {
					if cfg.Logger != nil {
						cfg.Logger.Error("failed to acquire idempotency lock",
							"error", err.Error(),
							"idempotency_key", idempotencyKey,
						)
					}
				} else if !acquired {
					// Another request is processing with this key
					writeIdempotencyError(w, r, "IDEMPOTENCY_KEY_IN_USE",
						"A request with this idempotency key is currently being processed",
						http.StatusConflict)
					return
				}
			}

			// Add idempotency info to context
			ctx := r.Context()
			ctx = context.WithValue(ctx, idempotencyInfoKey, &IdempotencyInfo{
				Key:         idempotencyKey,
				IsReplay:    false,
				RequestHash: requestHash,
			})

			// Create a response recorder to capture the response
			recorder := &responseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				body:           &bytes.Buffer{},
			}

			// Process the request
			next.ServeHTTP(recorder, r.WithContext(ctx))

			// Cache the response if successful (2xx status codes)
			if cfg.Cache != nil && recorder.statusCode >= 200 && recorder.statusCode < 300 {
				result := &IdempotencyResult{
					StatusCode:  recorder.statusCode,
					Headers:     recorder.Header().Clone(),
					Body:        recorder.body.Bytes(),
					CreatedAt:   time.Now(),
					RequestHash: requestHash,
				}

				if err := cacheResult(r.Context(), cfg.Cache, cacheKey, result, cfg.TTL); err != nil {
					if cfg.Logger != nil {
						cfg.Logger.Error("failed to cache idempotency result",
							"error", err.Error(),
							"idempotency_key", idempotencyKey,
						)
					}
				}
			}
		})
	}
}

// responseRecorder captures the response for caching.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// requiresIdempotency checks if the HTTP method requires idempotency.
func requiresIdempotency(method string, requiredMethods []string) bool {
	for _, m := range requiredMethods {
		if strings.EqualFold(method, m) {
			return true
		}
	}
	return false
}

// shouldSkipIdempotency checks if the path should skip idempotency.
func shouldSkipIdempotency(path string, skipPaths []string) bool {
	for _, skip := range skipPaths {
		if path == skip || strings.HasPrefix(path, skip) {
			return true
		}
	}
	return false
}

// isValidIdempotencyKey validates the idempotency key format.
func isValidIdempotencyKey(key string) bool {
	// Accept UUIDs
	if _, err := uuid.Parse(key); err == nil {
		return true
	}
	// Accept any string between 16 and 64 characters
	return len(key) >= 16 && len(key) <= 64
}

// buildIdempotencyCacheKey builds the cache key for an idempotency key.
func buildIdempotencyCacheKey(r *http.Request, idempotencyKey string) string {
	// Include organization ID for tenant isolation
	authInfo := GetAuthInfo(r.Context())
	if authInfo != nil && authInfo.OrganizationID != uuid.Nil {
		return "idempotency:" + authInfo.OrganizationID.String() + ":" + idempotencyKey
	}
	return "idempotency:" + idempotencyKey
}

// calculateRequestHash calculates a hash of the request for conflict detection.
func calculateRequestHash(r *http.Request) (string, error) {
	// Read the body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	// Restore the body for the handler
	r.Body = io.NopCloser(bytes.NewReader(body))

	// Create hash from method, path, and body
	h := sha256.New()
	h.Write([]byte(r.Method))
	h.Write([]byte(r.URL.Path))
	h.Write(body)

	return hex.EncodeToString(h.Sum(nil)), nil
}

// getCachedResult retrieves a cached idempotency result.
func getCachedResult(ctx context.Context, cache *redis.Cache, key string) (*IdempotencyResult, error) {
	var result IdempotencyResult
	err := cache.Get(ctx, key, &result)
	if err != nil {
		// Check if it's a not found error
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "nil") {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

// acquireIdempotencyLock tries to acquire a lock for an idempotency key.
func acquireIdempotencyLock(ctx context.Context, cache *redis.Cache, key string, ttl time.Duration) (bool, error) {
	lockKey := key + ":lock"
	// Use SetNX-like behavior through the cache
	// This is a simplified implementation - in production, use proper distributed locking
	var existing string
	err := cache.Get(ctx, lockKey, &existing)
	if err == nil && existing != "" {
		// Lock exists
		return false, nil
	}

	// Try to set the lock
	err = cache.SetWithTTL(ctx, lockKey, "locked", ttl)
	if err != nil {
		return false, err
	}

	return true, nil
}

// cacheResult stores an idempotency result in the cache.
func cacheResult(ctx context.Context, cache *redis.Cache, key string, result *IdempotencyResult, ttl time.Duration) error {
	// Store the result
	if err := cache.SetWithTTL(ctx, key, result, ttl); err != nil {
		return err
	}

	// Remove the lock
	lockKey := key + ":lock"
	_ = cache.Delete(ctx, lockKey)

	return nil
}

// replayResponse replays a cached response.
func replayResponse(w http.ResponseWriter, result *IdempotencyResult) {
	// Copy headers
	for key, values := range result.Headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Add replay indicator header
	w.Header().Set("Idempotent-Replayed", "true")

	// Write status code
	w.WriteHeader(result.StatusCode)

	// Write body
	_, _ = w.Write(result.Body)
}

// writeIdempotencyError writes an idempotency error response.
func writeIdempotencyError(w http.ResponseWriter, r *http.Request, code, detail string, status int) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	traceID := GetTraceID(r.Context())

	problem := map[string]interface{}{
		"type":     "https://api.shipaggregator.com/errors/" + strings.ToLower(strings.ReplaceAll(code, "_", "-")),
		"title":    getIdempotencyErrorTitle(code),
		"status":   status,
		"detail":   detail,
		"code":     code,
		"trace_id": traceID,
	}

	_ = json.NewEncoder(w).Encode(problem)
}

// getIdempotencyErrorTitle returns the title for an idempotency error code.
func getIdempotencyErrorTitle(code string) string {
	switch code {
	case "INVALID_IDEMPOTENCY_KEY":
		return "Invalid Idempotency Key"
	case "IDEMPOTENCY_KEY_REUSED":
		return "Idempotency Key Reused"
	case "IDEMPOTENCY_KEY_IN_USE":
		return "Idempotency Key In Use"
	default:
		return "Idempotency Error"
	}
}

// GetIdempotencyInfo extracts the idempotency info from the context.
func GetIdempotencyInfo(ctx context.Context) *IdempotencyInfo {
	if info, ok := ctx.Value(idempotencyInfoKey).(*IdempotencyInfo); ok {
		return info
	}
	return nil
}

// IsIdempotentReplay checks if the current request is a replayed idempotent response.
func IsIdempotentReplay(ctx context.Context) bool {
	info := GetIdempotencyInfo(ctx)
	return info != nil && info.IsReplay
}

// Package middleware provides HTTP middleware for the shipping aggregator API.
package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/knid/kargonet/internal/observability"
	"github.com/knid/kargonet/internal/repository/redis"
)

// RateLimitConfig holds configuration for the rate limit middleware.
type RateLimitConfig struct {
	// RateLimiter is the rate limiter implementation.
	RateLimiter *redis.RateLimiter
	// Logger is the logger for rate limit events.
	Logger *observability.Logger
	// DefaultLimit is the default rate limit per minute.
	DefaultLimit int
	// DefaultBurst is the default burst size.
	DefaultBurst int
	// SkipPaths are paths that don't have rate limiting.
	SkipPaths []string
	// KeyFunc extracts the rate limit key from the request.
	// If nil, uses the API key ID or client IP.
	KeyFunc func(r *http.Request) string
	// LimitFunc returns the rate limit for a request.
	// If nil, uses DefaultLimit.
	LimitFunc func(r *http.Request) int
	// BurstFunc returns the burst size for a request.
	// If nil, uses DefaultBurst.
	BurstFunc func(r *http.Request) int
}

// RateLimitResult contains the result of a rate limit check.
type RateLimitResult struct {
	// Allowed indicates if the request is allowed.
	Allowed bool
	// Limit is the rate limit.
	Limit int
	// Remaining is the number of remaining requests.
	Remaining int
	// ResetAt is when the rate limit resets.
	ResetAt time.Time
	// RetryAfter is the duration to wait before retrying.
	RetryAfter time.Duration
}

// RateLimit returns a middleware that enforces rate limiting.
// It uses a sliding window algorithm with Redis for distributed rate limiting.
func RateLimit(cfg RateLimitConfig) func(http.Handler) http.Handler {
	// Set defaults
	if cfg.DefaultLimit == 0 {
		cfg.DefaultLimit = 1000 // 1000 requests per minute
	}
	if cfg.DefaultBurst == 0 {
		cfg.DefaultBurst = 100 // Allow burst of 100 requests
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if path should skip rate limiting
			if shouldSkipRateLimit(r.URL.Path, cfg.SkipPaths) {
				next.ServeHTTP(w, r)
				return
			}

			// Get rate limit key
			key := getRateLimitKey(r, cfg.KeyFunc)
			if key == "" {
				// No key means no rate limiting (e.g., unauthenticated request to public endpoint)
				next.ServeHTTP(w, r)
				return
			}

			// Get rate limit for this request
			limit := cfg.DefaultLimit
			if cfg.LimitFunc != nil {
				limit = cfg.LimitFunc(r)
			}

			// Get burst size for this request
			burst := cfg.DefaultBurst
			if cfg.BurstFunc != nil {
				burst = cfg.BurstFunc(r)
			}

			// Check rate limit
			result, err := checkRateLimit(r.Context(), cfg.RateLimiter, key, limit, burst)
			if err != nil {
				if cfg.Logger != nil {
					cfg.Logger.Error("rate limit check failed",
						"error", err.Error(),
						"key", key,
					)
				}
				// On error, allow the request but don't set headers
				next.ServeHTTP(w, r)
				return
			}

			// Set rate limit headers
			setRateLimitHeaders(w, result)

			// Check if request is allowed
			if !result.Allowed {
				if cfg.Logger != nil {
					cfg.Logger.Warn("rate limit exceeded",
						"key", key,
						"limit", result.Limit,
						"retry_after", result.RetryAfter.Seconds(),
					)
				}
				writeRateLimitError(w, r, result)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitByAPIKey returns a middleware that enforces rate limiting per API key.
// It uses the API key's configured rate limit if available.
func RateLimitByAPIKey(cfg RateLimitConfig) func(http.Handler) http.Handler {
	// Override key function to use API key ID
	cfg.KeyFunc = func(r *http.Request) string {
		authInfo := GetAuthInfo(r.Context())
		if authInfo == nil {
			return ""
		}
		if authInfo.APIKeyID != uuid.Nil {
			return fmt.Sprintf("api_key:%s", authInfo.APIKeyID.String())
		}
		if authInfo.MemberID != uuid.Nil {
			return fmt.Sprintf("member:%s", authInfo.MemberID.String())
		}
		return ""
	}

	return RateLimit(cfg)
}

// RateLimitByOrganization returns a middleware that enforces rate limiting per organization.
func RateLimitByOrganization(cfg RateLimitConfig) func(http.Handler) http.Handler {
	// Override key function to use organization ID
	cfg.KeyFunc = func(r *http.Request) string {
		authInfo := GetAuthInfo(r.Context())
		if authInfo == nil {
			return ""
		}
		if authInfo.OrganizationID != uuid.Nil {
			return fmt.Sprintf("org:%s", authInfo.OrganizationID.String())
		}
		return ""
	}

	return RateLimit(cfg)
}

// RateLimitByIP returns a middleware that enforces rate limiting per IP address.
func RateLimitByIP(cfg RateLimitConfig) func(http.Handler) http.Handler {
	// Override key function to use client IP
	cfg.KeyFunc = func(r *http.Request) string {
		return fmt.Sprintf("ip:%s", GetClientIP(r))
	}

	return RateLimit(cfg)
}

// RateLimitByEndpoint returns a middleware that enforces rate limiting per endpoint.
func RateLimitByEndpoint(cfg RateLimitConfig) func(http.Handler) http.Handler {
	// Override key function to use endpoint + API key
	cfg.KeyFunc = func(r *http.Request) string {
		authInfo := GetAuthInfo(r.Context())
		if authInfo == nil {
			return fmt.Sprintf("endpoint:%s:%s:ip:%s", r.Method, r.URL.Path, GetClientIP(r))
		}
		if authInfo.APIKeyID != uuid.Nil {
			return fmt.Sprintf("endpoint:%s:%s:api_key:%s", r.Method, r.URL.Path, authInfo.APIKeyID.String())
		}
		return fmt.Sprintf("endpoint:%s:%s:org:%s", r.Method, r.URL.Path, authInfo.OrganizationID.String())
	}

	return RateLimit(cfg)
}

// checkRateLimit checks the rate limit for a key.
func checkRateLimit(ctx context.Context, limiter *redis.RateLimiter, key string, limit, burst int) (*RateLimitResult, error) {
	if limiter == nil {
		// No limiter configured, allow all requests
		return &RateLimitResult{
			Allowed:   true,
			Limit:     limit,
			Remaining: limit,
			ResetAt:   time.Now().Add(time.Minute),
		}, nil
	}

	// Use the rate limiter with custom limit
	redisResult, err := limiter.AllowWithLimit(ctx, key, limit, time.Minute)
	if err != nil {
		return nil, err
	}

	result := &RateLimitResult{
		Allowed:    redisResult.Allowed,
		Limit:      redisResult.Limit,
		Remaining:  redisResult.Remaining,
		ResetAt:    redisResult.ResetAt,
		RetryAfter: redisResult.RetryAfter,
	}

	return result, nil
}

// getRateLimitKey extracts the rate limit key from the request.
func getRateLimitKey(r *http.Request, keyFunc func(r *http.Request) string) string {
	if keyFunc != nil {
		return keyFunc(r)
	}

	// Default: use API key ID if available, otherwise use client IP
	authInfo := GetAuthInfo(r.Context())
	if authInfo != nil {
		if authInfo.APIKeyID != uuid.Nil {
			return fmt.Sprintf("api_key:%s", authInfo.APIKeyID.String())
		}
		if authInfo.OrganizationID != uuid.Nil {
			return fmt.Sprintf("org:%s", authInfo.OrganizationID.String())
		}
	}

	// Fall back to IP address
	return fmt.Sprintf("ip:%s", GetClientIP(r))
}

// shouldSkipRateLimit checks if the path should skip rate limiting.
func shouldSkipRateLimit(path string, skipPaths []string) bool {
	return matchSkipPath(path, skipPaths)
}

// setRateLimitHeaders sets the rate limit headers on the response.
func setRateLimitHeaders(w http.ResponseWriter, result *RateLimitResult) {
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(result.Limit))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))

	if !result.Allowed {
		w.Header().Set("Retry-After", strconv.Itoa(int(result.RetryAfter.Seconds())))
	}
}

// writeRateLimitError writes a rate limit exceeded error response.
func writeRateLimitError(w http.ResponseWriter, r *http.Request, result *RateLimitResult) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusTooManyRequests)

	traceID := GetTraceID(r.Context())

	problem := map[string]interface{}{
		"type":        "https://api.kargonet.com/errors/rate-limit-exceeded",
		"title":       "Rate Limit Exceeded",
		"status":      http.StatusTooManyRequests,
		"detail":      fmt.Sprintf("Rate limit of %d requests per minute exceeded. Please retry after %d seconds.", result.Limit, int(result.RetryAfter.Seconds())),
		"code":        "RATE_LIMIT_EXCEEDED",
		"trace_id":    traceID,
		"retry_after": int(result.RetryAfter.Seconds()),
		"limit":       result.Limit,
		"reset_at":    result.ResetAt.Unix(),
	}

	_ = json.NewEncoder(w).Encode(problem)
}

// RateLimitInfo contains rate limit information for the current request.
type RateLimitInfo struct {
	// Key is the rate limit key.
	Key string
	// Limit is the rate limit.
	Limit int
	// Remaining is the number of remaining requests.
	Remaining int
	// ResetAt is when the rate limit resets.
	ResetAt time.Time
}

// rateLimitContextKey is the context key for rate limit info.
type rateLimitContextKey string

const rateLimitInfoKey rateLimitContextKey = "rate_limit_info"

// GetRateLimitInfo extracts the rate limit info from the context.
func GetRateLimitInfo(ctx context.Context) *RateLimitInfo {
	if info, ok := ctx.Value(rateLimitInfoKey).(*RateLimitInfo); ok {
		return info
	}
	return nil
}

// WithRateLimitInfo adds rate limit info to the context.
func WithRateLimitInfo(ctx context.Context, info *RateLimitInfo) context.Context {
	return context.WithValue(ctx, rateLimitInfoKey, info)
}

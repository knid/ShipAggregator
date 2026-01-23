// Package redis provides Redis-based caching and rate limiting for the shipping aggregator.
// This file implements a sliding window rate limiter using Redis sorted sets.
package redis

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Common errors for rate limiting.
var (
	// ErrRateLimitExceeded indicates the rate limit has been exceeded.
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

// RateLimitConfig holds configuration for the rate limiter.
type RateLimitConfig struct {
	// KeyPrefix is the prefix for rate limit keys.
	KeyPrefix string
	// DefaultLimit is the default number of requests allowed per window.
	DefaultLimit int
	// DefaultWindow is the default time window for rate limiting.
	DefaultWindow time.Duration
	// BurstMultiplier allows temporary bursts above the limit (e.g., 1.5 = 50% burst).
	BurstMultiplier float64
}

// DefaultRateLimitConfig returns a default rate limit configuration.
func DefaultRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		KeyPrefix:       "ratelimit:",
		DefaultLimit:    100,
		DefaultWindow:   time.Second,
		BurstMultiplier: 1.0,
	}
}

// RateLimiter implements a sliding window rate limiter using Redis sorted sets.
// The sliding window algorithm provides more accurate rate limiting than fixed windows
// by considering the timestamp of each request.
type RateLimiter struct {
	client *redis.Client
	config RateLimitConfig
	logger *slog.Logger
}

// NewRateLimiter creates a new rate limiter with the given configuration.
func NewRateLimiter(client *redis.Client, cfg RateLimitConfig, logger *slog.Logger) *RateLimiter {
	if logger == nil {
		logger = slog.Default()
	}

	return &RateLimiter{
		client: client,
		config: cfg,
		logger: logger,
	}
}

// RateLimitResult contains the result of a rate limit check.
type RateLimitResult struct {
	// Allowed indicates whether the request is allowed.
	Allowed bool
	// Remaining is the number of requests remaining in the current window.
	Remaining int
	// Limit is the maximum number of requests allowed in the window.
	Limit int
	// ResetAt is the time when the rate limit window resets.
	ResetAt time.Time
	// RetryAfter is the duration to wait before retrying (if rate limited).
	RetryAfter time.Duration
}

// Allow checks if a request is allowed under the rate limit.
// It uses the sliding window log algorithm with Redis sorted sets.
func (r *RateLimiter) Allow(ctx context.Context, key string) (*RateLimitResult, error) {
	return r.AllowN(ctx, key, 1, r.config.DefaultLimit, r.config.DefaultWindow)
}

// AllowWithLimit checks if a request is allowed with a custom limit.
func (r *RateLimiter) AllowWithLimit(ctx context.Context, key string, limit int, window time.Duration) (*RateLimitResult, error) {
	return r.AllowN(ctx, key, 1, limit, window)
}

// AllowN checks if n requests are allowed under the rate limit.
// This is the core rate limiting function using the sliding window log algorithm.
func (r *RateLimiter) AllowN(ctx context.Context, key string, n int, limit int, window time.Duration) (*RateLimitResult, error) {
	now := time.Now()
	windowStart := now.Add(-window)
	prefixedKey := r.config.KeyPrefix + key

	// Calculate effective limit with burst multiplier
	effectiveLimit := int(float64(limit) * r.config.BurstMultiplier)

	// Use a Lua script for atomic operations
	// This script:
	// 1. Removes expired entries (older than window start)
	// 2. Counts current entries in the window
	// 3. If under limit, adds new entries
	// 4. Sets expiration on the key
	script := redis.NewScript(`
		local key = KEYS[1]
		local window_start = tonumber(ARGV[1])
		local now = tonumber(ARGV[2])
		local limit = tonumber(ARGV[3])
		local count = tonumber(ARGV[4])
		local window_ms = tonumber(ARGV[5])

		-- Remove expired entries
		redis.call('ZREMRANGEBYSCORE', key, '-inf', window_start)

		-- Count current entries
		local current = redis.call('ZCARD', key)

		-- Check if we can add more
		if current + count <= limit then
			-- Add new entries with current timestamp as score
			for i = 1, count do
				redis.call('ZADD', key, now, now .. ':' .. i .. ':' .. math.random())
			end
			-- Set expiration
			redis.call('PEXPIRE', key, window_ms)
			return {1, limit - current - count, current + count}
		else
			-- Get the oldest entry to calculate retry time
			-- We need to find when enough entries will expire to allow this request
			local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
			local retry_after = 1000  -- Default to 1 second minimum
			if #oldest >= 2 then
				local oldest_timestamp = tonumber(oldest[2])
				local expire_time = oldest_timestamp + window_ms
				retry_after = expire_time - now
				-- Ensure retry_after is at least 1 second (1000ms) to prevent tight loops
				if retry_after < 1000 then
					retry_after = 1000
				end
			end
			return {0, 0, current, retry_after}
		end
	`)

	windowStartMs := windowStart.UnixMilli()
	nowMs := now.UnixMilli()
	windowMs := window.Milliseconds()

	result, err := script.Run(ctx, r.client, []string{prefixedKey},
		windowStartMs, nowMs, effectiveLimit, n, windowMs).Slice()
	if err != nil {
		r.logger.Error("rate limit check failed",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("rate limit check failed: %w", err)
	}

	allowed := result[0].(int64) == 1
	remaining := int(result[1].(int64))
	_ = result[2].(int64) // current count (used for logging if needed)

	var retryAfter time.Duration
	if !allowed && len(result) > 3 {
		retryAfterMs := result[3].(int64)
		if retryAfterMs > 0 {
			retryAfter = time.Duration(retryAfterMs) * time.Millisecond
		}
	}

	resetAt := now.Add(window)

	return &RateLimitResult{
		Allowed:    allowed,
		Remaining:  remaining,
		Limit:      limit,
		ResetAt:    resetAt,
		RetryAfter: retryAfter,
	}, nil
}

// Reset resets the rate limit for a key.
func (r *RateLimiter) Reset(ctx context.Context, key string) error {
	prefixedKey := r.config.KeyPrefix + key

	if err := r.client.Del(ctx, prefixedKey).Err(); err != nil {
		return fmt.Errorf("rate limit reset failed: %w", err)
	}

	return nil
}

// GetCount returns the current count for a key within the window.
func (r *RateLimiter) GetCount(ctx context.Context, key string, window time.Duration) (int, error) {
	now := time.Now()
	windowStart := now.Add(-window)
	prefixedKey := r.config.KeyPrefix + key

	// Remove expired entries and count
	pipe := r.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, prefixedKey, "-inf", strconv.FormatInt(windowStart.UnixMilli(), 10))
	countCmd := pipe.ZCard(ctx, prefixedKey)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("rate limit count failed: %w", err)
	}

	return int(countCmd.Val()), nil
}

// SlidingWindowCounter implements a simpler sliding window counter algorithm.
// This is more efficient than the log algorithm but slightly less accurate.
type SlidingWindowCounter struct {
	client *redis.Client
	config RateLimitConfig
	logger *slog.Logger
}

// NewSlidingWindowCounter creates a new sliding window counter.
func NewSlidingWindowCounter(client *redis.Client, cfg RateLimitConfig, logger *slog.Logger) *SlidingWindowCounter {
	if logger == nil {
		logger = slog.Default()
	}

	return &SlidingWindowCounter{
		client: client,
		config: cfg,
		logger: logger,
	}
}

// Allow checks if a request is allowed using the sliding window counter algorithm.
// This algorithm uses two fixed windows and interpolates between them.
func (s *SlidingWindowCounter) Allow(ctx context.Context, key string) (*RateLimitResult, error) {
	return s.AllowN(ctx, key, 1, s.config.DefaultLimit, s.config.DefaultWindow)
}

// AllowN checks if n requests are allowed using the sliding window counter algorithm.
func (s *SlidingWindowCounter) AllowN(ctx context.Context, key string, n int, limit int, window time.Duration) (*RateLimitResult, error) {
	now := time.Now()
	windowSecs := int64(window.Seconds())
	if windowSecs == 0 {
		windowSecs = 1
	}

	// Calculate current and previous window keys
	currentWindow := now.Unix() / windowSecs
	previousWindow := currentWindow - 1

	currentKey := fmt.Sprintf("%s%s:%d", s.config.KeyPrefix, key, currentWindow)
	previousKey := fmt.Sprintf("%s%s:%d", s.config.KeyPrefix, key, previousWindow)

	// Calculate the weight of the previous window
	// (how much of the current window has elapsed)
	windowStart := time.Unix(currentWindow*windowSecs, 0)
	elapsed := now.Sub(windowStart)
	previousWeight := 1.0 - (float64(elapsed) / float64(window))
	if previousWeight < 0 {
		previousWeight = 0
	}

	// Use a Lua script for atomic operations
	script := redis.NewScript(`
		local current_key = KEYS[1]
		local previous_key = KEYS[2]
		local limit = tonumber(ARGV[1])
		local count = tonumber(ARGV[2])
		local previous_weight = tonumber(ARGV[3])
		local window_secs = tonumber(ARGV[4])

		-- Get counts from both windows
		local current_count = tonumber(redis.call('GET', current_key) or '0')
		local previous_count = tonumber(redis.call('GET', previous_key) or '0')

		-- Calculate weighted count
		local weighted_count = current_count + (previous_count * previous_weight)

		-- Check if we can add more
		if weighted_count + count <= limit then
			-- Increment current window
			local new_count = redis.call('INCRBY', current_key, count)
			-- Set expiration (2 windows to ensure previous window data is available)
			redis.call('EXPIRE', current_key, window_secs * 2)
			return {1, limit - new_count, new_count}
		else
			return {0, 0, current_count}
		end
	`)

	result, err := script.Run(ctx, s.client, []string{currentKey, previousKey},
		limit, n, previousWeight, windowSecs).Slice()
	if err != nil {
		s.logger.Error("rate limit check failed",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("rate limit check failed: %w", err)
	}

	allowed := result[0].(int64) == 1
	remaining := int(result[1].(int64))

	// Calculate reset time (end of current window)
	resetAt := time.Unix((currentWindow+1)*windowSecs, 0)

	// Calculate retry after
	var retryAfter time.Duration
	if !allowed {
		retryAfter = resetAt.Sub(now)
	}

	return &RateLimitResult{
		Allowed:    allowed,
		Remaining:  remaining,
		Limit:      limit,
		ResetAt:    resetAt,
		RetryAfter: retryAfter,
	}, nil
}

// TokenBucket implements a token bucket rate limiter.
// This algorithm allows for controlled bursting while maintaining a steady rate.
type TokenBucket struct {
	client *redis.Client
	config RateLimitConfig
	logger *slog.Logger
}

// NewTokenBucket creates a new token bucket rate limiter.
func NewTokenBucket(client *redis.Client, cfg RateLimitConfig, logger *slog.Logger) *TokenBucket {
	if logger == nil {
		logger = slog.Default()
	}

	return &TokenBucket{
		client: client,
		config: cfg,
		logger: logger,
	}
}

// TokenBucketConfig holds configuration for a token bucket.
type TokenBucketConfig struct {
	// Capacity is the maximum number of tokens in the bucket.
	Capacity int
	// RefillRate is the number of tokens added per second.
	RefillRate float64
}

// Allow checks if a request is allowed using the token bucket algorithm.
func (t *TokenBucket) Allow(ctx context.Context, key string, cfg TokenBucketConfig) (*RateLimitResult, error) {
	return t.AllowN(ctx, key, 1, cfg)
}

// AllowN checks if n tokens can be consumed from the bucket.
func (t *TokenBucket) AllowN(ctx context.Context, key string, n int, cfg TokenBucketConfig) (*RateLimitResult, error) {
	now := time.Now()
	prefixedKey := t.config.KeyPrefix + key

	// Use a Lua script for atomic operations
	// The bucket state is stored as a hash with:
	// - tokens: current number of tokens
	// - last_refill: timestamp of last refill
	script := redis.NewScript(`
		local key = KEYS[1]
		local capacity = tonumber(ARGV[1])
		local refill_rate = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		local requested = tonumber(ARGV[4])

		-- Get current state
		local tokens = tonumber(redis.call('HGET', key, 'tokens') or capacity)
		local last_refill = tonumber(redis.call('HGET', key, 'last_refill') or now)

		-- Calculate tokens to add based on time elapsed
		local elapsed = now - last_refill
		local tokens_to_add = elapsed * refill_rate
		tokens = math.min(capacity, tokens + tokens_to_add)

		-- Check if we have enough tokens
		if tokens >= requested then
			tokens = tokens - requested
			redis.call('HSET', key, 'tokens', tokens, 'last_refill', now)
			redis.call('EXPIRE', key, math.ceil(capacity / refill_rate) + 1)
			return {1, math.floor(tokens), 0}
		else
			-- Calculate time until enough tokens are available
			local tokens_needed = requested - tokens
			local wait_time = tokens_needed / refill_rate
			redis.call('HSET', key, 'tokens', tokens, 'last_refill', now)
			redis.call('EXPIRE', key, math.ceil(capacity / refill_rate) + 1)
			return {0, math.floor(tokens), wait_time}
		end
	`)

	result, err := script.Run(ctx, t.client, []string{prefixedKey},
		cfg.Capacity, cfg.RefillRate, float64(now.Unix()), n).Slice()
	if err != nil {
		t.logger.Error("token bucket check failed",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("token bucket check failed: %w", err)
	}

	allowed := result[0].(int64) == 1
	remaining := int(result[1].(int64))
	waitTime := result[2].(int64)

	var retryAfter time.Duration
	if !allowed && waitTime > 0 {
		retryAfter = time.Duration(waitTime) * time.Second
	}

	// Calculate reset time (when bucket will be full)
	tokensNeeded := cfg.Capacity - remaining
	refillTime := time.Duration(float64(tokensNeeded)/cfg.RefillRate) * time.Second
	resetAt := now.Add(refillTime)

	return &RateLimitResult{
		Allowed:    allowed,
		Remaining:  remaining,
		Limit:      cfg.Capacity,
		ResetAt:    resetAt,
		RetryAfter: retryAfter,
	}, nil
}

// FixedWindowCounter implements a simple fixed window counter.
// This is the simplest rate limiting algorithm but can allow bursts at window boundaries.
type FixedWindowCounter struct {
	client *redis.Client
	config RateLimitConfig
	logger *slog.Logger
}

// NewFixedWindowCounter creates a new fixed window counter.
func NewFixedWindowCounter(client *redis.Client, cfg RateLimitConfig, logger *slog.Logger) *FixedWindowCounter {
	if logger == nil {
		logger = slog.Default()
	}

	return &FixedWindowCounter{
		client: client,
		config: cfg,
		logger: logger,
	}
}

// Allow checks if a request is allowed using the fixed window algorithm.
func (f *FixedWindowCounter) Allow(ctx context.Context, key string) (*RateLimitResult, error) {
	return f.AllowN(ctx, key, 1, f.config.DefaultLimit, f.config.DefaultWindow)
}

// AllowN checks if n requests are allowed using the fixed window algorithm.
func (f *FixedWindowCounter) AllowN(ctx context.Context, key string, n int, limit int, window time.Duration) (*RateLimitResult, error) {
	now := time.Now()
	windowSecs := int64(window.Seconds())
	if windowSecs == 0 {
		windowSecs = 1
	}

	// Calculate current window
	currentWindow := now.Unix() / windowSecs
	prefixedKey := fmt.Sprintf("%s%s:%d", f.config.KeyPrefix, key, currentWindow)

	// Use a Lua script for atomic increment and check
	script := redis.NewScript(`
		local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local count = tonumber(ARGV[2])
		local window_secs = tonumber(ARGV[3])

		local current = tonumber(redis.call('GET', key) or '0')

		if current + count <= limit then
			local new_count = redis.call('INCRBY', key, count)
			redis.call('EXPIRE', key, window_secs)
			return {1, limit - new_count, new_count}
		else
			return {0, 0, current}
		end
	`)

	result, err := script.Run(ctx, f.client, []string{prefixedKey},
		limit, n, windowSecs).Slice()
	if err != nil {
		f.logger.Error("rate limit check failed",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
		return nil, fmt.Errorf("rate limit check failed: %w", err)
	}

	allowed := result[0].(int64) == 1
	remaining := int(result[1].(int64))

	// Calculate reset time (end of current window)
	resetAt := time.Unix((currentWindow+1)*windowSecs, 0)

	// Calculate retry after
	var retryAfter time.Duration
	if !allowed {
		retryAfter = resetAt.Sub(now)
	}

	return &RateLimitResult{
		Allowed:    allowed,
		Remaining:  remaining,
		Limit:      limit,
		ResetAt:    resetAt,
		RetryAfter: retryAfter,
	}, nil
}

// RateLimitKey helpers for building rate limit keys.

// APIKeyRateLimitKey returns a rate limit key for an API key.
func APIKeyRateLimitKey(keyID string) string {
	return fmt.Sprintf("apikey:%s", keyID)
}

// IPRateLimitKey returns a rate limit key for an IP address.
func IPRateLimitKey(ip string) string {
	return fmt.Sprintf("ip:%s", ip)
}

// OrgRateLimitKey returns a rate limit key for an organization.
func OrgRateLimitKey(orgID string) string {
	return fmt.Sprintf("org:%s", orgID)
}

// EndpointRateLimitKey returns a rate limit key for a specific endpoint.
func EndpointRateLimitKey(keyID, method, path string) string {
	return fmt.Sprintf("endpoint:%s:%s:%s", keyID, method, path)
}

// ProviderRateLimitKey returns a rate limit key for provider API calls.
func ProviderRateLimitKey(providerCode, operation string) string {
	return fmt.Sprintf("provider:%s:%s", providerCode, operation)
}

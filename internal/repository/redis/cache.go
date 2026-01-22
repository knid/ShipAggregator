// Package redis provides Redis-based caching and rate limiting for the shipping aggregator.
// It implements generic cache operations with TTL support and type-safe serialization.
package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// Common errors for cache operations.
var (
	// ErrCacheMiss indicates the key was not found in cache.
	ErrCacheMiss = errors.New("cache miss")
	// ErrCacheUnavailable indicates the cache is unavailable.
	ErrCacheUnavailable = errors.New("cache unavailable")
	// ErrInvalidValue indicates the cached value could not be deserialized.
	ErrInvalidValue = errors.New("invalid cached value")
)

// CacheConfig holds configuration for the cache client.
type CacheConfig struct {
	// Addr is the Redis server address (host:port).
	Addr string
	// Password is the Redis password (optional).
	Password string
	// DB is the Redis database number.
	DB int
	// MaxRetries is the maximum number of retries for failed operations.
	MaxRetries int
	// PoolSize is the maximum number of connections in the pool.
	PoolSize int
	// MinIdleConns is the minimum number of idle connections.
	MinIdleConns int
	// DialTimeout is the timeout for establishing new connections.
	DialTimeout time.Duration
	// ReadTimeout is the timeout for read operations.
	ReadTimeout time.Duration
	// WriteTimeout is the timeout for write operations.
	WriteTimeout time.Duration
	// DefaultTTL is the default TTL for cache entries.
	DefaultTTL time.Duration
	// KeyPrefix is the prefix for all cache keys.
	KeyPrefix string
}

// DefaultCacheConfig returns a default cache configuration.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Addr:         "localhost:6379",
		Password:     "",
		DB:           0,
		MaxRetries:   3,
		PoolSize:     10,
		MinIdleConns: 2,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		DefaultTTL:   5 * time.Minute,
		KeyPrefix:    "shipping:",
	}
}

// Cache provides generic cache operations backed by Redis.
type Cache struct {
	client     *redis.Client
	config     CacheConfig
	logger     *slog.Logger
	serializer Serializer
}

// Serializer defines the interface for cache value serialization.
type Serializer interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(data []byte, v any) error
}

// JSONSerializer implements Serializer using JSON encoding.
type JSONSerializer struct{}

// Marshal serializes a value to JSON.
func (s *JSONSerializer) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// Unmarshal deserializes JSON data into a value.
func (s *JSONSerializer) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// NewCache creates a new cache client with the given configuration.
func NewCache(cfg CacheConfig, logger *slog.Logger) (*Cache, error) {
	if logger == nil {
		logger = slog.Default()
	}

	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		MaxRetries:   cfg.MaxRetries,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Info("connected to Redis cache",
		slog.String("addr", cfg.Addr),
		slog.Int("db", cfg.DB),
		slog.String("prefix", cfg.KeyPrefix),
	)

	return &Cache{
		client:     client,
		config:     cfg,
		logger:     logger,
		serializer: &JSONSerializer{},
	}, nil
}

// NewCacheWithClient creates a new cache using an existing Redis client.
// This is useful for sharing a connection pool across multiple components.
func NewCacheWithClient(client *redis.Client, cfg CacheConfig, logger *slog.Logger) *Cache {
	if logger == nil {
		logger = slog.Default()
	}

	return &Cache{
		client:     client,
		config:     cfg,
		logger:     logger,
		serializer: &JSONSerializer{},
	}
}

// Client returns the underlying Redis client.
func (c *Cache) Client() *redis.Client {
	return c.client
}

// Close closes the cache connection.
func (c *Cache) Close() error {
	return c.client.Close()
}

// Ping checks if the cache is available.
func (c *Cache) Ping(ctx context.Context) error {
	if err := c.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("%w: %v", ErrCacheUnavailable, err)
	}
	return nil
}

// prefixKey adds the configured prefix to a key.
func (c *Cache) prefixKey(key string) string {
	return c.config.KeyPrefix + key
}

// Get retrieves a value from the cache.
// Returns ErrCacheMiss if the key does not exist.
func (c *Cache) Get(ctx context.Context, key string, dest any) error {
	prefixedKey := c.prefixKey(key)

	data, err := c.client.Get(ctx, prefixedKey).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrCacheMiss
		}
		c.logger.Error("cache get failed",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("cache get failed: %w", err)
	}

	if err := c.serializer.Unmarshal(data, dest); err != nil {
		c.logger.Error("cache unmarshal failed",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("%w: %v", ErrInvalidValue, err)
	}

	return nil
}

// GetString retrieves a string value from the cache.
// Returns ErrCacheMiss if the key does not exist.
func (c *Cache) GetString(ctx context.Context, key string) (string, error) {
	prefixedKey := c.prefixKey(key)

	val, err := c.client.Get(ctx, prefixedKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", ErrCacheMiss
		}
		return "", fmt.Errorf("cache get failed: %w", err)
	}

	return val, nil
}

// Set stores a value in the cache with the default TTL.
func (c *Cache) Set(ctx context.Context, key string, value any) error {
	return c.SetWithTTL(ctx, key, value, c.config.DefaultTTL)
}

// SetWithTTL stores a value in the cache with a custom TTL.
func (c *Cache) SetWithTTL(ctx context.Context, key string, value any, ttl time.Duration) error {
	prefixedKey := c.prefixKey(key)

	data, err := c.serializer.Marshal(value)
	if err != nil {
		return fmt.Errorf("cache marshal failed: %w", err)
	}

	if err := c.client.Set(ctx, prefixedKey, data, ttl).Err(); err != nil {
		c.logger.Error("cache set failed",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("cache set failed: %w", err)
	}

	return nil
}

// SetString stores a string value in the cache with the default TTL.
func (c *Cache) SetString(ctx context.Context, key, value string) error {
	return c.SetStringWithTTL(ctx, key, value, c.config.DefaultTTL)
}

// SetStringWithTTL stores a string value in the cache with a custom TTL.
func (c *Cache) SetStringWithTTL(ctx context.Context, key, value string, ttl time.Duration) error {
	prefixedKey := c.prefixKey(key)

	if err := c.client.Set(ctx, prefixedKey, value, ttl).Err(); err != nil {
		return fmt.Errorf("cache set failed: %w", err)
	}

	return nil
}

// SetNX stores a value only if the key does not exist (SET NX).
// Returns true if the value was set, false if the key already exists.
func (c *Cache) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	prefixedKey := c.prefixKey(key)

	data, err := c.serializer.Marshal(value)
	if err != nil {
		return false, fmt.Errorf("cache marshal failed: %w", err)
	}

	ok, err := c.client.SetNX(ctx, prefixedKey, data, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("cache setnx failed: %w", err)
	}

	return ok, nil
}

// Delete removes a key from the cache.
func (c *Cache) Delete(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}

	prefixedKeys := make([]string, len(keys))
	for i, key := range keys {
		prefixedKeys[i] = c.prefixKey(key)
	}

	if err := c.client.Del(ctx, prefixedKeys...).Err(); err != nil {
		c.logger.Error("cache delete failed",
			slog.Any("keys", keys),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("cache delete failed: %w", err)
	}

	return nil
}

// Exists checks if a key exists in the cache.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	prefixedKey := c.prefixKey(key)

	count, err := c.client.Exists(ctx, prefixedKey).Result()
	if err != nil {
		return false, fmt.Errorf("cache exists failed: %w", err)
	}

	return count > 0, nil
}

// Expire sets a TTL on an existing key.
func (c *Cache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	prefixedKey := c.prefixKey(key)

	if err := c.client.Expire(ctx, prefixedKey, ttl).Err(); err != nil {
		return fmt.Errorf("cache expire failed: %w", err)
	}

	return nil
}

// TTL returns the remaining TTL for a key.
// Returns -1 if the key exists but has no TTL, -2 if the key does not exist.
func (c *Cache) TTL(ctx context.Context, key string) (time.Duration, error) {
	prefixedKey := c.prefixKey(key)

	ttl, err := c.client.TTL(ctx, prefixedKey).Result()
	if err != nil {
		return 0, fmt.Errorf("cache ttl failed: %w", err)
	}

	return ttl, nil
}

// Incr increments an integer value by 1.
// If the key does not exist, it is set to 0 before incrementing.
func (c *Cache) Incr(ctx context.Context, key string) (int64, error) {
	prefixedKey := c.prefixKey(key)

	val, err := c.client.Incr(ctx, prefixedKey).Result()
	if err != nil {
		return 0, fmt.Errorf("cache incr failed: %w", err)
	}

	return val, nil
}

// IncrBy increments an integer value by the given amount.
func (c *Cache) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	prefixedKey := c.prefixKey(key)

	val, err := c.client.IncrBy(ctx, prefixedKey, value).Result()
	if err != nil {
		return 0, fmt.Errorf("cache incrby failed: %w", err)
	}

	return val, nil
}

// Decr decrements an integer value by 1.
func (c *Cache) Decr(ctx context.Context, key string) (int64, error) {
	prefixedKey := c.prefixKey(key)

	val, err := c.client.Decr(ctx, prefixedKey).Result()
	if err != nil {
		return 0, fmt.Errorf("cache decr failed: %w", err)
	}

	return val, nil
}

// DecrBy decrements an integer value by the given amount.
func (c *Cache) DecrBy(ctx context.Context, key string, value int64) (int64, error) {
	prefixedKey := c.prefixKey(key)

	val, err := c.client.DecrBy(ctx, prefixedKey, value).Result()
	if err != nil {
		return 0, fmt.Errorf("cache decrby failed: %w", err)
	}

	return val, nil
}

// HGet retrieves a field from a hash.
func (c *Cache) HGet(ctx context.Context, key, field string) (string, error) {
	prefixedKey := c.prefixKey(key)

	val, err := c.client.HGet(ctx, prefixedKey, field).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", ErrCacheMiss
		}
		return "", fmt.Errorf("cache hget failed: %w", err)
	}

	return val, nil
}

// HSet sets a field in a hash.
func (c *Cache) HSet(ctx context.Context, key string, values ...any) error {
	prefixedKey := c.prefixKey(key)

	if err := c.client.HSet(ctx, prefixedKey, values...).Err(); err != nil {
		return fmt.Errorf("cache hset failed: %w", err)
	}

	return nil
}

// HGetAll retrieves all fields from a hash.
func (c *Cache) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	prefixedKey := c.prefixKey(key)

	val, err := c.client.HGetAll(ctx, prefixedKey).Result()
	if err != nil {
		return nil, fmt.Errorf("cache hgetall failed: %w", err)
	}

	return val, nil
}

// HDel deletes fields from a hash.
func (c *Cache) HDel(ctx context.Context, key string, fields ...string) error {
	prefixedKey := c.prefixKey(key)

	if err := c.client.HDel(ctx, prefixedKey, fields...).Err(); err != nil {
		return fmt.Errorf("cache hdel failed: %w", err)
	}

	return nil
}

// HIncrBy increments a hash field by the given amount.
func (c *Cache) HIncrBy(ctx context.Context, key, field string, value int64) (int64, error) {
	prefixedKey := c.prefixKey(key)

	val, err := c.client.HIncrBy(ctx, prefixedKey, field, value).Result()
	if err != nil {
		return 0, fmt.Errorf("cache hincrby failed: %w", err)
	}

	return val, nil
}

// SAdd adds members to a set.
func (c *Cache) SAdd(ctx context.Context, key string, members ...any) error {
	prefixedKey := c.prefixKey(key)

	if err := c.client.SAdd(ctx, prefixedKey, members...).Err(); err != nil {
		return fmt.Errorf("cache sadd failed: %w", err)
	}

	return nil
}

// SMembers returns all members of a set.
func (c *Cache) SMembers(ctx context.Context, key string) ([]string, error) {
	prefixedKey := c.prefixKey(key)

	val, err := c.client.SMembers(ctx, prefixedKey).Result()
	if err != nil {
		return nil, fmt.Errorf("cache smembers failed: %w", err)
	}

	return val, nil
}

// SIsMember checks if a value is a member of a set.
func (c *Cache) SIsMember(ctx context.Context, key string, member any) (bool, error) {
	prefixedKey := c.prefixKey(key)

	val, err := c.client.SIsMember(ctx, prefixedKey, member).Result()
	if err != nil {
		return false, fmt.Errorf("cache sismember failed: %w", err)
	}

	return val, nil
}

// SRem removes members from a set.
func (c *Cache) SRem(ctx context.Context, key string, members ...any) error {
	prefixedKey := c.prefixKey(key)

	if err := c.client.SRem(ctx, prefixedKey, members...).Err(); err != nil {
		return fmt.Errorf("cache srem failed: %w", err)
	}

	return nil
}

// SCard returns the number of members in a set.
func (c *Cache) SCard(ctx context.Context, key string) (int64, error) {
	prefixedKey := c.prefixKey(key)

	val, err := c.client.SCard(ctx, prefixedKey).Result()
	if err != nil {
		return 0, fmt.Errorf("cache scard failed: %w", err)
	}

	return val, nil
}

// GetOrSet retrieves a value from cache, or calls the loader function and caches the result.
// This is useful for implementing cache-aside pattern.
func (c *Cache) GetOrSet(ctx context.Context, key string, dest any, ttl time.Duration, loader func() (any, error)) error {
	// Try to get from cache first
	err := c.Get(ctx, key, dest)
	if err == nil {
		return nil
	}

	if !errors.Is(err, ErrCacheMiss) {
		// Log the error but continue to load from source
		c.logger.Warn("cache get failed, loading from source",
			slog.String("key", key),
			slog.String("error", err.Error()),
		)
	}

	// Load from source
	value, err := loader()
	if err != nil {
		return err
	}

	// Cache the result (ignore cache errors)
	if cacheErr := c.SetWithTTL(ctx, key, value, ttl); cacheErr != nil {
		c.logger.Warn("failed to cache loaded value",
			slog.String("key", key),
			slog.String("error", cacheErr.Error()),
		)
	}

	// Copy the loaded value to dest
	data, err := c.serializer.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal loaded value: %w", err)
	}

	if err := c.serializer.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("failed to unmarshal loaded value: %w", err)
	}

	return nil
}

// DeletePattern deletes all keys matching a pattern.
// Use with caution as this can be slow for large datasets.
func (c *Cache) DeletePattern(ctx context.Context, pattern string) (int64, error) {
	prefixedPattern := c.prefixKey(pattern)

	var cursor uint64
	var deleted int64

	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, prefixedPattern, 100).Result()
		if err != nil {
			return deleted, fmt.Errorf("cache scan failed: %w", err)
		}

		if len(keys) > 0 {
			count, err := c.client.Del(ctx, keys...).Result()
			if err != nil {
				return deleted, fmt.Errorf("cache delete failed: %w", err)
			}
			deleted += count
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return deleted, nil
}

// Pipeline returns a new pipeline for batching commands.
func (c *Cache) Pipeline() redis.Pipeliner {
	return c.client.Pipeline()
}

// TxPipeline returns a new transactional pipeline.
func (c *Cache) TxPipeline() redis.Pipeliner {
	return c.client.TxPipeline()
}

// Watch watches keys for changes and executes a transaction.
func (c *Cache) Watch(ctx context.Context, fn func(*redis.Tx) error, keys ...string) error {
	prefixedKeys := make([]string, len(keys))
	for i, key := range keys {
		prefixedKeys[i] = c.prefixKey(key)
	}

	return c.client.Watch(ctx, fn, prefixedKeys...)
}

// Stats returns cache statistics.
type CacheStats struct {
	Hits       int64
	Misses     int64
	Timeouts   int64
	TotalConns int64
	IdleConns  int64
	StaleConns int64
}

// Stats returns cache connection pool statistics.
func (c *Cache) Stats() CacheStats {
	stats := c.client.PoolStats()
	return CacheStats{
		Hits:       int64(stats.Hits),
		Misses:     int64(stats.Misses),
		Timeouts:   int64(stats.Timeouts),
		TotalConns: int64(stats.TotalConns),
		IdleConns:  int64(stats.IdleConns),
		StaleConns: int64(stats.StaleConns),
	}
}

// CacheKey is a helper for building cache keys.
type CacheKey struct {
	parts []string
}

// NewCacheKey creates a new cache key builder.
func NewCacheKey(parts ...string) *CacheKey {
	return &CacheKey{parts: parts}
}

// Add adds a part to the cache key.
func (k *CacheKey) Add(part string) *CacheKey {
	k.parts = append(k.parts, part)
	return k
}

// String returns the cache key as a string.
func (k *CacheKey) String() string {
	return fmt.Sprintf("%s", k.parts[0])
}

// Build returns the cache key as a colon-separated string.
func (k *CacheKey) Build() string {
	result := ""
	for i, part := range k.parts {
		if i > 0 {
			result += ":"
		}
		result += part
	}
	return result
}

// Common cache key prefixes for the shipping aggregator.
const (
	// KeyPrefixQuote is the prefix for quote cache keys.
	KeyPrefixQuote = "quote"
	// KeyPrefixOffer is the prefix for offer cache keys.
	KeyPrefixOffer = "offer"
	// KeyPrefixProvider is the prefix for provider cache keys.
	KeyPrefixProvider = "provider"
	// KeyPrefixRateLimit is the prefix for rate limit keys.
	KeyPrefixRateLimit = "ratelimit"
	// KeyPrefixSession is the prefix for session cache keys.
	KeyPrefixSession = "session"
	// KeyPrefixAPIKey is the prefix for API key cache keys.
	KeyPrefixAPIKey = "apikey"
	// KeyPrefixIdempotency is the prefix for idempotency keys.
	KeyPrefixIdempotency = "idempotency"
	// KeyPrefixLock is the prefix for distributed lock keys.
	KeyPrefixLock = "lock"
)

// QuoteKey returns a cache key for a quote.
func QuoteKey(quoteID string) string {
	return NewCacheKey(KeyPrefixQuote, quoteID).Build()
}

// OfferKey returns a cache key for an offer.
func OfferKey(offerID string) string {
	return NewCacheKey(KeyPrefixOffer, offerID).Build()
}

// ProviderKey returns a cache key for a provider.
func ProviderKey(providerCode string) string {
	return NewCacheKey(KeyPrefixProvider, providerCode).Build()
}

// SessionKey returns a cache key for a session.
func SessionKey(sessionID string) string {
	return NewCacheKey(KeyPrefixSession, sessionID).Build()
}

// APIKeyKey returns a cache key for an API key.
func APIKeyKey(keyHash string) string {
	return NewCacheKey(KeyPrefixAPIKey, keyHash).Build()
}

// IdempotencyKey returns a cache key for an idempotency record.
func IdempotencyKey(orgID, key, endpoint string) string {
	return NewCacheKey(KeyPrefixIdempotency, orgID, endpoint, key).Build()
}

// LockKey returns a cache key for a distributed lock.
func LockKey(resource string) string {
	return NewCacheKey(KeyPrefixLock, resource).Build()
}

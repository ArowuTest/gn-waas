// Package cache provides a Redis-backed cache for the GN-WAAS API gateway.
// It is used to cache frequently-read, rarely-changing data such as district
// lists, tariff rates, and system configuration to reduce DB load.
package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// TTL constants for different data types
const (
	TTLDistricts   = 10 * time.Minute  // Districts change rarely
	TTLTariffRates = 30 * time.Minute  // Tariff rates change rarely
	TTLSysConfig   = 5 * time.Minute   // System config changes occasionally
	TTLDashboard   = 2 * time.Minute   // Dashboard stats refresh frequently
	TTLUserProfile = 5 * time.Minute   // User profiles
	TTLReports     = 15 * time.Minute  // Report data
)

// Client wraps a Redis client with helper methods.
type Client struct {
	rdb    *redis.Client
	logger *zap.Logger
	prefix string
}

// NewClient creates a new Redis cache client.
// Returns a no-op client if addr is empty (graceful degradation).
func NewClient(addr, password string, db int, logger *zap.Logger) *Client {
	if addr == "" || addr == ":" {
		logger.Warn("Redis address not configured — caching disabled")
		return &Client{logger: logger, prefix: "gnwaas:"}
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolSize:     10,
		MinIdleConns: 2,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Warn("Redis ping failed — caching disabled", zap.Error(err))
		return &Client{logger: logger, prefix: "gnwaas:"}
	}

	logger.Info("Redis cache connected", zap.String("addr", addr))
	return &Client{rdb: rdb, logger: logger, prefix: "gnwaas:"}
}

// key builds a namespaced cache key.
func (c *Client) key(parts ...string) string {
	k := c.prefix
	for i, p := range parts {
		if i > 0 {
			k += ":"
		}
		k += p
	}
	return k
}

// Set stores a value in the cache with the given TTL.
// Silently no-ops if Redis is not connected.
func (c *Client) Set(ctx context.Context, ttl time.Duration, keyParts ...string) func(v interface{}) {
	return func(v interface{}) {
		if c.rdb == nil {
			return
		}
		data, err := json.Marshal(v)
		if err != nil {
			c.logger.Warn("Cache marshal failed", zap.Error(err))
			return
		}
		k := c.key(keyParts...)
		if err := c.rdb.Set(ctx, k, data, ttl).Err(); err != nil {
			c.logger.Warn("Cache set failed", zap.String("key", k), zap.Error(err))
		}
	}
}

// Get retrieves a value from the cache. Returns false if not found or Redis unavailable.
func (c *Client) Get(ctx context.Context, dest interface{}, keyParts ...string) bool {
	if c.rdb == nil {
		return false
	}
	k := c.key(keyParts...)
	data, err := c.rdb.Get(ctx, k).Bytes()
	if err != nil {
		if err != redis.Nil {
			c.logger.Warn("Cache get failed", zap.String("key", k), zap.Error(err))
		}
		return false
	}
	if err := json.Unmarshal(data, dest); err != nil {
		c.logger.Warn("Cache unmarshal failed", zap.String("key", k), zap.Error(err))
		return false
	}
	return true
}

// Invalidate deletes one or more cache keys by prefix pattern.
func (c *Client) Invalidate(ctx context.Context, pattern string) {
	if c.rdb == nil {
		return
	}
	fullPattern := c.prefix + pattern
	keys, err := c.rdb.Keys(ctx, fullPattern).Result()
	if err != nil {
		c.logger.Warn("Cache invalidate scan failed", zap.Error(err))
		return
	}
	if len(keys) > 0 {
		if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
			c.logger.Warn("Cache invalidate delete failed", zap.Error(err))
			return
		}
		c.logger.Debug("Cache invalidated", zap.String("pattern", fullPattern), zap.Int("keys", len(keys)))
	}
}

// IsConnected returns true if Redis is available.
func (c *Client) IsConnected() bool {
	return c.rdb != nil
}

// Stats returns cache statistics for the health endpoint.
func (c *Client) Stats(ctx context.Context) map[string]interface{} {
	if c.rdb == nil {
		return map[string]interface{}{"status": "disabled"}
	}
	info, err := c.rdb.Info(ctx, "stats", "keyspace").Result()
	if err != nil {
		return map[string]interface{}{"status": "error", "error": err.Error()}
	}
	keyCount, _ := c.rdb.DBSize(ctx).Result()
	return map[string]interface{}{
		"status":    "connected",
		"key_count": keyCount,
		"info":      fmt.Sprintf("%.200s", info),
	}
}

// Package services — Redis-backed memory cache.
// Mirrors Python services/cache.py — low-latency caching for memories and search results.
package services

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	cacheTTL       = 30 * time.Minute // Default TTL for memory cache
	searchCacheTTL = 5 * time.Minute  // Shorter TTL for search results
	maxLocalCache  = 500              // Max entries in local fallback
)

// MemoryCache provides Redis-backed caching with in-memory fallback.
type MemoryCache struct {
	client     *redis.Client
	mu         sync.RWMutex
	localCache map[string][]byte // JSON-encoded fallback
}

// NewMemoryCache creates a new cache instance. client may be nil for local-only mode.
func NewMemoryCache(client *redis.Client) *MemoryCache {
	return &MemoryCache{
		client:     client,
		localCache: make(map[string][]byte, 64),
	}
}

// IsAvailable returns true if Redis is connected.
func (c *MemoryCache) IsAvailable() bool { return c.client != nil }

// --- Key Helpers ---

func cacheKey(prefix string, parts ...string) string {
	raw := prefix + ":"
	for i, p := range parts {
		if i > 0 {
			raw += "|"
		}
		raw += p
	}
	h := md5.Sum([]byte(raw))
	return fmt.Sprintf("%x", h)
}

// --- Memory CRUD ---

// GetMemory retrieves a cached memory by ID.
func (c *MemoryCache) GetMemory(ctx context.Context, memoryID string) (map[string]any, error) {
	key := "mem:" + memoryID

	// Try Redis first
	if c.client != nil {
		data, err := c.client.Get(ctx, key).Bytes()
		if err == nil {
			var result map[string]any
			if json.Unmarshal(data, &result) == nil {
				return result, nil
			}
		}
		if err != redis.Nil {
			slog.Debug("Cache get failed", "error", err)
		}
	}

	// Fallback to local
	c.mu.RLock()
	data, ok := c.localCache[key]
	c.mu.RUnlock()
	if ok {
		var result map[string]any
		if json.Unmarshal(data, &result) == nil {
			return result, nil
		}
	}
	return nil, nil
}

// SetMemory caches a memory.
func (c *MemoryCache) SetMemory(ctx context.Context, memoryID string, data map[string]any) {
	key := "mem:" + memoryID
	encoded, err := json.Marshal(data)
	if err != nil {
		return
	}

	if c.client != nil {
		if err := c.client.Set(ctx, key, encoded, cacheTTL).Err(); err == nil {
			return // Stored in Redis successfully
		}
		slog.Debug("Cache set failed, using local fallback")
	}

	c.setLocal(key, encoded)
}

// InvalidateMemory removes a cached memory.
func (c *MemoryCache) InvalidateMemory(ctx context.Context, memoryID string) {
	key := "mem:" + memoryID
	if c.client != nil {
		c.client.Del(ctx, key)
	}
	c.mu.Lock()
	delete(c.localCache, key)
	c.mu.Unlock()
}

// GetBytes retrieves raw bytes for a given key (Redis + local fallback).
// Returns (data, true) on hit, (nil, false) on miss.
func (c *MemoryCache) GetBytes(ctx context.Context, key string) ([]byte, bool) {
	if c.client != nil {
		data, err := c.client.Get(ctx, key).Bytes()
		if err == nil {
			return data, true
		}
	}
	c.mu.RLock()
	data, ok := c.localCache[key]
	c.mu.RUnlock()
	if ok {
		// Return a copy to prevent accidental mutation of localCache data.
		cp := make([]byte, len(data))
		copy(cp, data)
		return cp, true
	}
	return nil, false
}

// SetBytes stores raw bytes for a given key with an explicit TTL.
func (c *MemoryCache) SetBytes(ctx context.Context, key string, data []byte, ttl time.Duration) {
	if c.client != nil {
		if err := c.client.Set(ctx, key, data, ttl).Err(); err == nil {
			return
		}
		slog.Debug("Cache SetBytes failed, using local fallback", "key", key)
	}
	c.setLocal(key, data)
}

// --- Search Cache ---

// GetSearchResults retrieves cached search results.
func (c *MemoryCache) GetSearchResults(ctx context.Context, query, userID string, limit int) ([]map[string]any, error) {
	key := cacheKey("search", query, userID, fmt.Sprintf("%d", limit))

	if c.client != nil {
		data, err := c.client.Get(ctx, key).Bytes()
		if err == nil {
			var results []map[string]any
			if json.Unmarshal(data, &results) == nil {
				return results, nil
			}
		}
	}

	c.mu.RLock()
	data, ok := c.localCache[key]
	c.mu.RUnlock()
	if ok {
		var results []map[string]any
		if json.Unmarshal(data, &results) == nil {
			return results, nil
		}
	}
	return nil, nil
}

// SetSearchResults caches search results with a shorter TTL.
func (c *MemoryCache) SetSearchResults(ctx context.Context, query, userID string, limit int, results []map[string]any) {
	key := cacheKey("search", query, userID, fmt.Sprintf("%d", limit))
	encoded, err := json.Marshal(results)
	if err != nil {
		return
	}

	if c.client != nil {
		if err := c.client.Set(ctx, key, encoded, searchCacheTTL).Err(); err == nil {
			return
		}
	}
	c.setLocal(key, encoded)
}

// --- User-scoped Clear ---

// ClearUserCache removes all cache entries related to a user.
func (c *MemoryCache) ClearUserCache(ctx context.Context, userID string) {
	if c.client != nil {
		var cursor uint64
		for {
			keys, nextCursor, err := c.client.Scan(ctx, cursor, "*"+userID+"*", 100).Result()
			if err != nil {
				slog.Debug("Cache scan failed", "error", err)
				break
			}
			if len(keys) > 0 {
				c.client.Del(ctx, keys...)
			}
			cursor = nextCursor
			if cursor == 0 {
				break
			}
		}
	}

	// Clear local
	c.mu.Lock()
	for k := range c.localCache {
		if len(k) > 0 && contains(k, userID) {
			delete(c.localCache, k)
		}
	}
	c.mu.Unlock()
}

// --- Internal ---

func (c *MemoryCache) setLocal(key string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict oldest 25% if at capacity
	if len(c.localCache) >= maxLocalCache {
		count := 0
		for k := range c.localCache {
			delete(c.localCache, k)
			count++
			if count >= maxLocalCache/4 {
				break
			}
		}
	}
	c.localCache[key] = data
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// --- Singleton ---

var (
	memoryCacheOnce sync.Once
	memoryCacheSvc  *MemoryCache
)

// GetMemoryCache returns the singleton MemoryCache.
// Must be initialized after Redis client is ready.
func GetMemoryCache(client *redis.Client) *MemoryCache {
	memoryCacheOnce.Do(func() {
		memoryCacheSvc = NewMemoryCache(client)
	})
	return memoryCacheSvc
}

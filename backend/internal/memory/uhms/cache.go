package uhms

import (
	"sync"
	"time"
)

// LRUCache is a simple in-memory LRU cache with TTL eviction.
// Replaces Redis for the embedded local deployment.
type LRUCache struct {
	mu       sync.RWMutex
	items    map[string]*cacheEntry
	order    []string // LRU order: most recent at end
	maxItems int
	ttl      time.Duration
}

type cacheEntry struct {
	value     interface{}
	expiresAt time.Time
}

// NewLRUCache creates a new LRU cache with the given capacity and TTL.
func NewLRUCache(maxItems int, ttl time.Duration) *LRUCache {
	if maxItems <= 0 {
		maxItems = 1000
	}
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return &LRUCache{
		items:    make(map[string]*cacheEntry, maxItems),
		order:    make([]string, 0, maxItems),
		maxItems: maxItems,
		ttl:      ttl,
	}
}

// Get retrieves a value from the cache. Returns nil if not found or expired.
func (c *LRUCache) Get(key string) (interface{}, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.items[key]
	if !ok {
		return nil, false
	}

	// Check TTL
	if time.Now().After(entry.expiresAt) {
		c.removeLocked(key)
		return nil, false
	}

	// Move to end (most recently used)
	c.touchLocked(key)
	return entry.value, true
}

// Set stores a value in the cache, evicting the oldest entry if at capacity.
func (c *LRUCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update existing
	if _, ok := c.items[key]; ok {
		c.items[key] = &cacheEntry{value: value, expiresAt: time.Now().Add(c.ttl)}
		c.touchLocked(key)
		return
	}

	// Evict if at capacity
	for len(c.items) >= c.maxItems && len(c.order) > 0 {
		oldest := c.order[0]
		c.removeLocked(oldest)
	}

	// Insert
	c.items[key] = &cacheEntry{value: value, expiresAt: time.Now().Add(c.ttl)}
	c.order = append(c.order, key)
}

// Delete removes a key from the cache.
func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeLocked(key)
}

// Len returns the number of items in the cache.
func (c *LRUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Clear removes all entries.
func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*cacheEntry, c.maxItems)
	c.order = c.order[:0]
}

// EvictExpired removes all expired entries. Call periodically if needed.
func (c *LRUCache) EvictExpired() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	evicted := 0
	for key, entry := range c.items {
		if now.After(entry.expiresAt) {
			c.removeLocked(key)
			evicted++
		}
	}
	return evicted
}

// ---------- internal ----------

func (c *LRUCache) removeLocked(key string) {
	delete(c.items, key)
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

func (c *LRUCache) touchLocked(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, key)
			break
		}
	}
}

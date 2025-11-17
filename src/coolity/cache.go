package coolify

import (
	"strings"
	"sync"
	"time"
)

type cacheEntry struct {
	value     any
	expiresAt time.Time
}

// MemoryCache is a minimal in-memory cache with TTL-based invalidation.
type MemoryCache struct {
	ttl  time.Duration
	mu   sync.RWMutex
	data map[string]cacheEntry
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		ttl: 30 * time.Second,
		data: make(map[string]cacheEntry),
	}
}

func (c *MemoryCache) WithTTL(ttl time.Duration) *MemoryCache {
	c.ttl = ttl
	return c
}

func (c *MemoryCache) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.data[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.value, true
}

func (c *MemoryCache) Set(key string, value any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ttl <= 0 {
		ttl = c.ttl
	}
	c.data[key] = cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(ttl),
	}
}

func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, key)
}

func (c *MemoryCache) DeletePrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for k := range c.data {
		if strings.HasPrefix(k, prefix) {
			delete(c.data, k)
		}
	}
}

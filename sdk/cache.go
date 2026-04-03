package configclient

import (
	"encoding/json"
	"sync"
	"time"
)

type cacheEntry struct {
	value     json.RawMessage
	expiresAt time.Time
}

type cache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
	ttl     time.Duration
}

func newCache(ttl time.Duration) *cache {
	return &cache{
		entries: make(map[string]cacheEntry),
		ttl:     ttl,
	}
}

// Get returns the cached value and whether it was found (even if expired).
func (c *cache) Get(key string) (json.RawMessage, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	return entry.value, true
}

// IsExpired returns true if the key is not cached or has expired.
func (c *cache) IsExpired(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[key]
	if !ok {
		return true
	}
	return time.Now().After(entry.expiresAt)
}

// Set stores a value with the configured TTL.
func (c *cache) Set(key string, value json.RawMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = cacheEntry{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// SetBatch stores multiple values with the configured TTL.
func (c *cache) SetBatch(entries map[string]json.RawMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()

	expires := time.Now().Add(c.ttl)
	for key, value := range entries {
		c.entries[key] = cacheEntry{
			value:     value,
			expiresAt: expires,
		}
	}
}

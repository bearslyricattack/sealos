package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// Cache manages authentication result caching
type Cache struct {
	store sync.Map
	ttl   time.Duration
}

// CacheEntry represents a cached authentication result
type CacheEntry struct {
	allowed   bool
	expiresAt time.Time
	mu        sync.RWMutex
}

// NewCache creates a new cache instance
func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		ttl: ttl,
	}
}

// Get retrieves a cached entry
func (c *Cache) Get(key string) (*CacheEntry, bool) {
	if entry, ok := c.store.Load(key); ok {
		cached := entry.(*CacheEntry)
		cached.mu.RLock()
		defer cached.mu.RUnlock()

		if time.Now().Before(cached.expiresAt) {
			return cached, true
		}
	}
	return nil, false
}

// Set stores a cache entry
func (c *Cache) Set(key string, allowed bool) {
	entry := &CacheEntry{
		allowed:   allowed,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.store.Store(key, entry)
}

// Clear removes all cached entries
func (c *Cache) Clear() {
	c.store = sync.Map{}
}

// GenerateKey creates a cache key from namespace and kubeconfig
func GenerateKey(namespace, kubeconfig string) string {
	hash := sha256.Sum256([]byte(namespace + "::" + kubeconfig))
	return hex.EncodeToString(hash[:])
}

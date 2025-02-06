package auth

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

type AuthCache struct {
	cache         map[string]time.Time
	order         *list.List
	mutex         sync.RWMutex
	ttl           time.Duration
	capacity      int
	cleanupTicker *time.Ticker
}

func NewAuthCache(ttl time.Duration, capacity int) *AuthCache {
	ac := &AuthCache{
		cache:         make(map[string]time.Time),
		order:         list.New(),
		ttl:           ttl,
		capacity:      capacity,
		cleanupTicker: time.NewTicker(5 * time.Minute),
	}

	// Periodic cleanup of expired items
	go func() {
		for range ac.cleanupTicker.C {
			ac.cleanup()
		}
	}()

	return ac
}

func (ac *AuthCache) Set(ns, kc string) {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()

	key := fmt.Sprintf("%s-%s", kc, ns)
	if len(ac.cache) >= ac.capacity {
		// Evict least recently used entry
		ac.evict()
	}

	ac.cache[key] = time.Now()
	ac.order.PushFront(key)
}

func (ac *AuthCache) Get(ns, kc string) bool {
	ac.mutex.RLock()
	defer ac.mutex.RUnlock()

	key := fmt.Sprintf("%s-%s", kc, ns)
	_, exists := ac.cache[key]
	if !exists {
		return false
	}

	if time.Now().Sub(ac.cache[key]) > ac.ttl {
		// Expired entry
		return false
	}

	return true
}

func (ac *AuthCache) evict() {
	// Remove least recently used item (from the back of the list)
	if ac.order.Len() == 0 {
		return
	}
	oldestKey := ac.order.Back()
	ac.order.Remove(oldestKey)
	delete(ac.cache, oldestKey.Value.(string))
}

func (ac *AuthCache) cleanup() {
	ac.mutex.Lock()
	defer ac.mutex.Unlock()

	// Clean up expired entries
	for key, timestamp := range ac.cache {
		if time.Now().Sub(timestamp) > ac.ttl {
			delete(ac.cache, key)
		}
	}
}

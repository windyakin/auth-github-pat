package cache

import (
	"context"
	"sync"
	"time"
)

type memoryEntry struct {
	entry     Entry
	expiresAt time.Time
}

type MemoryCache struct {
	mu      sync.RWMutex
	items   map[string]memoryEntry
	stopCh  chan struct{}
}

func NewMemoryCache() *MemoryCache {
	mc := &MemoryCache{
		items:  make(map[string]memoryEntry),
		stopCh: make(chan struct{}),
	}
	go mc.cleanup()
	return mc
}

func (mc *MemoryCache) Get(_ context.Context, key string) (Entry, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	item, ok := mc.items[key]
	if !ok {
		return Entry{}, false
	}
	if time.Now().After(item.expiresAt) {
		return Entry{}, false
	}
	return item.entry, true
}

func (mc *MemoryCache) Set(_ context.Context, key string, entry Entry, ttl time.Duration) {
	if ttl <= 0 {
		return
	}
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.items[key] = memoryEntry{
		entry:     entry,
		expiresAt: time.Now().Add(ttl),
	}
}

func (mc *MemoryCache) Stop() {
	close(mc.stopCh)
}

func (mc *MemoryCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-mc.stopCh:
			return
		case <-ticker.C:
			mc.mu.Lock()
			now := time.Now()
			for k, v := range mc.items {
				if now.After(v.expiresAt) {
					delete(mc.items, k)
				}
			}
			mc.mu.Unlock()
		}
	}
}

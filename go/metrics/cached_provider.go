// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package metrics

import (
	"context"
	"sync"
	"time"
)

// CachedProvider wraps a Provider with TTL-based caching to optimize query latency
type CachedProvider struct {
	provider Provider
	cache    map[string]*cacheEntry
	mu       sync.RWMutex
	ttl      time.Duration
}

type cacheEntry struct {
	metrics   Metrics
	timestamp time.Time
}

// NewCachedProvider creates a new cached metrics provider
// ttl: time-to-live for cache entries (e.g., 30 seconds)
func NewCachedProvider(provider Provider, ttl time.Duration) Provider {
	c := &CachedProvider{
		provider: provider,
		cache:    make(map[string]*cacheEntry),
		ttl:      ttl,
	}

	// Start background cleanup goroutine
	go c.cleanup()

	return c
}

// FetchPodMetrics fetches metrics with caching
func (c *CachedProvider) FetchPodMetrics(ctx context.Context, namespace, podName string) (Metrics, error) {
	key := namespace + "/" + podName

	// Try cache first (fast path)
	c.mu.RLock()
	if entry, ok := c.cache[key]; ok {
		if time.Since(entry.timestamp) < c.ttl {
			c.mu.RUnlock()
			return entry.metrics, nil
		}
	}
	c.mu.RUnlock()

	// Cache miss or stale - fetch from upstream
	metrics, err := c.provider.FetchPodMetrics(ctx, namespace, podName)
	if err != nil {
		return metrics, err
	}

	// Update cache
	c.mu.Lock()
	c.cache[key] = &cacheEntry{
		metrics:   metrics,
		timestamp: time.Now(),
	}
	c.mu.Unlock()

	return metrics, nil
}

// cleanup removes stale cache entries periodically
func (c *CachedProvider) cleanup() {
	ticker := time.NewTicker(c.ttl)
	defer ticker.Stop()

	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.cache {
			if now.Sub(entry.timestamp) > c.ttl*2 {
				delete(c.cache, key)
			}
		}
		c.mu.Unlock()
	}
}

// Invalidate removes a specific pod from the cache
func (c *CachedProvider) Invalidate(namespace, podName string) {
	key := namespace + "/" + podName
	c.mu.Lock()
	delete(c.cache, key)
	c.mu.Unlock()
}

// Clear removes all entries from the cache
func (c *CachedProvider) Clear() {
	c.mu.Lock()
	c.cache = make(map[string]*cacheEntry)
	c.mu.Unlock()
}

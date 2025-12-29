package controllers

import (
	"fmt"
	"time"
)

// shouldLogResizeDecision checks if we should log this resize decision based on cache
func (r *InPlaceRightSizer) shouldLogResizeDecision(namespace, podName, containerName, oldCPU, newCPU, oldMemory, newMemory string) bool {
	containerKey := fmt.Sprintf("%s/%s/%s", namespace, podName, containerName)

	r.cacheMutex.Lock()
	defer r.cacheMutex.Unlock()

	cached, exists := r.resizeCache[containerKey]

	if !exists {
		// First time seeing this decision, cache it and allow logging
		r.resizeCache[containerKey] = &ResizeDecisionCache{
			ContainerKey: containerKey,
			OldCPU:       oldCPU,
			NewCPU:       newCPU,
			OldMemory:    oldMemory,
			NewMemory:    newMemory,
			LastSeen:     time.Now(),
		}
		return true
	}

	// Check if decision has changed or cache has expired
	now := time.Now()
	if now.Sub(cached.LastSeen) > r.cacheExpiry ||
		cached.OldCPU != oldCPU || cached.NewCPU != newCPU ||
		cached.OldMemory != oldMemory || cached.NewMemory != newMemory {
		// Decision changed or expired, update cache and allow logging
		r.resizeCache[containerKey] = &ResizeDecisionCache{
			ContainerKey: containerKey,
			OldCPU:       oldCPU,
			NewCPU:       newCPU,
			OldMemory:    oldMemory,
			NewMemory:    newMemory,
			LastSeen:     now,
		}
		return true
	}

	// Same decision within cache period, suppress logging
	return false
}

// cleanExpiredCacheEntries removes expired cache entries
func (r *InPlaceRightSizer) cleanExpiredCacheEntries() {
	r.cacheMutex.Lock()
	defer r.cacheMutex.Unlock()

	now := time.Now()
	for key, cached := range r.resizeCache {
		if now.Sub(cached.LastSeen) > r.cacheExpiry {
			delete(r.resizeCache, key)
		}
	}
}

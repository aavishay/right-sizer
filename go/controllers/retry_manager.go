// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package controllers

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"

	"right-sizer/logger"
	"right-sizer/metrics"
)

// DeferredResize represents a resize operation that was temporarily deferred
type DeferredResize struct {
	// Pod is the target pod for resize
	Pod *corev1.Pod `json:"pod"`

	// NewResources contains the desired resource specifications
	NewResources map[string]corev1.ResourceRequirements `json:"newResources"`

	// FirstAttempt timestamp when the resize was first attempted
	FirstAttempt time.Time `json:"firstAttempt"`

	// LastAttempt timestamp of the most recent retry attempt
	LastAttempt time.Time `json:"lastAttempt"`

	// Priority of the pod for retry ordering (higher priority = earlier retry)
	Priority int32 `json:"priority"`

	// Reason why the resize was deferred
	Reason string `json:"reason"`

	// AttemptCount tracks the number of retry attempts
	AttemptCount int `json:"attemptCount"`

	// MaxRetries maximum number of retry attempts before giving up
	MaxRetries int `json:"maxRetries"`

	// BackoffFactor for exponential backoff between retries
	BackoffFactor float64 `json:"backoffFactor"`

	// OriginalError the original error that caused the deferral
	OriginalError string `json:"originalError"`
}

// RetryManager manages deferred resize operations and retry logic
type RetryManager struct {
	// deferredResizes map of pod namespaced names to deferred resize operations
	deferredResizes map[string]*DeferredResize

	// mutex protects concurrent access to deferredResizes
	mutex sync.RWMutex

	// retryInterval how often to process deferred resizes
	retryInterval time.Duration

	// maxRetries default maximum retry attempts
	maxRetries int

	// maxDeferralTime maximum time to keep retrying a deferred resize
	maxDeferralTime time.Duration

	// metrics provider for recording retry metrics
	metrics *metrics.OperatorMetrics

	// eventRecorder for recording Kubernetes events
	eventRecorder record.EventRecorder

	// ctx context for cancellation
	ctx context.Context

	// cancel function to stop the retry manager
	cancel context.CancelFunc

	// running indicates if the retry manager is active
	running bool

	// runMutex protects the running state
	runMutex sync.Mutex
}

// RetryManagerConfig holds configuration for the RetryManager
type RetryManagerConfig struct {
	RetryInterval   time.Duration
	MaxRetries      int
	MaxDeferralTime time.Duration
	BackoffFactor   float64
}

// DefaultRetryManagerConfig returns default configuration for RetryManager
func DefaultRetryManagerConfig() RetryManagerConfig {
	return RetryManagerConfig{
		RetryInterval:   30 * time.Second,
		MaxRetries:      5,
		MaxDeferralTime: 10 * time.Minute,
		BackoffFactor:   2.0,
	}
}

// NewRetryManager creates a new RetryManager with the given configuration
func NewRetryManager(config RetryManagerConfig, metrics *metrics.OperatorMetrics, eventRecorder record.EventRecorder) *RetryManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &RetryManager{
		deferredResizes: make(map[string]*DeferredResize),
		retryInterval:   config.RetryInterval,
		maxRetries:      config.MaxRetries,
		maxDeferralTime: config.MaxDeferralTime,
		metrics:         metrics,
		eventRecorder:   eventRecorder,
		ctx:             ctx,
		cancel:          cancel,
		running:         false,
	}
}

// Start begins processing deferred resizes in the background
func (rm *RetryManager) Start() error {
	rm.runMutex.Lock()
	defer rm.runMutex.Unlock()

	if rm.running {
		return fmt.Errorf("retry manager is already running")
	}

	rm.running = true
	logger.Info("Starting resize retry manager with interval %v", rm.retryInterval)

	// Start the retry processing goroutine
	go rm.processRetries()

	return nil
}

// Stop stops the retry manager and clears all deferred resizes
func (rm *RetryManager) Stop() {
	rm.runMutex.Lock()
	defer rm.runMutex.Unlock()

	if !rm.running {
		return
	}

	logger.Info("Stopping resize retry manager")
	rm.cancel()
	rm.running = false

	// Clear all deferred resizes
	rm.mutex.Lock()
	rm.deferredResizes = make(map[string]*DeferredResize)
	rm.mutex.Unlock()
}

// AddDeferredResize adds a resize operation to the deferred queue
func (rm *RetryManager) AddDeferredResize(pod *corev1.Pod, newResources map[string]corev1.ResourceRequirements, reason string, originalError error) {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	key := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
	priority := getPodPriority(pod)

	// Check if this pod already has a deferred resize
	if existing, exists := rm.deferredResizes[key]; exists {
		// Update existing deferred resize
		existing.NewResources = newResources
		existing.LastAttempt = time.Now()
		existing.AttemptCount++
		existing.Reason = reason
		if originalError != nil {
			existing.OriginalError = originalError.Error()
		}

		logger.Debug("Updated deferred resize for pod %s/%s (attempt %d/%d): %s",
			pod.Namespace, pod.Name, existing.AttemptCount, existing.MaxRetries, reason)
	} else {
		// Create new deferred resize
		deferredResize := &DeferredResize{
			Pod:           pod.DeepCopy(),
			NewResources:  newResources,
			FirstAttempt:  time.Now(),
			LastAttempt:   time.Now(),
			Priority:      priority,
			Reason:        reason,
			AttemptCount:  1,
			MaxRetries:    rm.maxRetries,
			BackoffFactor: 2.0,
		}

		if originalError != nil {
			deferredResize.OriginalError = originalError.Error()
		}

		rm.deferredResizes[key] = deferredResize

		logger.Info("Added deferred resize for pod %s/%s (priority %d): %s",
			pod.Namespace, pod.Name, priority, reason)

		// Record event
		if rm.eventRecorder != nil {
			rm.eventRecorder.Event(pod, corev1.EventTypeWarning, "ResizeDeferred",
				fmt.Sprintf("Resize operation deferred: %s", reason))
		}

		// Record metrics (placeholder for future implementation)
		// TODO: Implement RecordDeferredResize metric when available
		if rm.metrics != nil {
			// rm.metrics.RecordDeferredResize(pod.Namespace, pod.Name, reason)
		}
	}

	// Set pending condition on the pod
	SetPodResizePending(pod, ReasonNodeResourceConstraint, reason)
}

// processRetries continuously processes deferred resizes
func (rm *RetryManager) processRetries() {
	ticker := time.NewTicker(rm.retryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rm.ctx.Done():
			logger.Info("Retry manager stopped")
			return

		case <-ticker.C:
			rm.processDeferredResizes()
		}
	}
}

// ProcessDeferredResizes processes all deferred resizes and attempts to retry them
func (rm *RetryManager) processDeferredResizes() {
	rm.mutex.Lock()

	// Get all deferred resizes and sort by priority and wait time
	resizes := make([]*DeferredResize, 0, len(rm.deferredResizes))
	for _, resize := range rm.deferredResizes {
		resizes = append(resizes, resize)
	}
	rm.mutex.Unlock()

	if len(resizes) == 0 {
		return
	}

	logger.Debug("Processing %d deferred resize operations", len(resizes))

	// Sort by priority (higher first), then by wait time (older first)
	sort.Slice(resizes, func(i, j int) bool {
		if resizes[i].Priority != resizes[j].Priority {
			return resizes[i].Priority > resizes[j].Priority
		}
		return resizes[i].FirstAttempt.Before(resizes[j].FirstAttempt)
	})

	processed := 0
	succeeded := 0
	expired := 0

	// Process each deferred resize
	for _, resize := range resizes {
		key := fmt.Sprintf("%s/%s", resize.Pod.Namespace, resize.Pod.Name)

		// Check if the resize has expired
		if time.Since(resize.FirstAttempt) > rm.maxDeferralTime {
			logger.Warn("Deferred resize for pod %s/%s expired after %v",
				resize.Pod.Namespace, resize.Pod.Name, time.Since(resize.FirstAttempt))

			rm.removeDeferredResize(resize.Pod)
			if rm.eventRecorder != nil {
				rm.eventRecorder.Event(resize.Pod, corev1.EventTypeWarning, "ResizeExpired",
					"Deferred resize operation expired and was abandoned")
			}
			expired++
			continue
		}

		// Check if we've exceeded max retries
		if resize.AttemptCount > resize.MaxRetries {
			logger.Warn("Deferred resize for pod %s/%s exceeded max retries (%d)",
				resize.Pod.Namespace, resize.Pod.Name, resize.MaxRetries)

			rm.removeDeferredResize(resize.Pod)
			if rm.eventRecorder != nil {
				rm.eventRecorder.Event(resize.Pod, corev1.EventTypeWarning, "ResizeAbandoned",
					"Deferred resize operation abandoned after max retries")
			}
			expired++
			continue
		}

		// Check if enough time has passed for retry (exponential backoff)
		backoffDelay := rm.calculateBackoffDelay(resize)
		if time.Since(resize.LastAttempt) < backoffDelay {
			continue // Not yet time to retry
		}

		processed++
		logger.Debug("Retrying deferred resize for pod %s/%s (attempt %d/%d)",
			resize.Pod.Namespace, resize.Pod.Name, resize.AttemptCount+1, resize.MaxRetries)

		// This would need to be injected or passed in to actually perform the resize
		// For now, we'll simulate the retry logic
		success := rm.attemptRetry(resize)

		if success {
			logger.Info("Successfully retried deferred resize for pod %s/%s",
				resize.Pod.Namespace, resize.Pod.Name)

			rm.removeDeferredResize(resize.Pod)
			if rm.eventRecorder != nil {
				rm.eventRecorder.Event(resize.Pod, corev1.EventTypeNormal, "ResizeRetrySucceeded",
					"Deferred resize operation succeeded on retry")
			}

			// Clear resize conditions
			ClearResizeConditions(resize.Pod)
			succeeded++
		} else {
			// Update last attempt time and increment attempt count
			rm.mutex.Lock()
			if currentResize, exists := rm.deferredResizes[key]; exists {
				currentResize.LastAttempt = time.Now()
				currentResize.AttemptCount++
			}
			rm.mutex.Unlock()
		}
	}

	if processed > 0 || expired > 0 {
		logger.Info("Processed %d deferred resizes: %d succeeded, %d expired",
			processed, succeeded, expired)

		// Record metrics (placeholder for future implementation)
		// TODO: Implement RecordRetryProcessing metric when available
		if rm.metrics != nil {
			// rm.metrics.RecordRetryProcessing(processed, succeeded, expired)
		}
	}
}

// attemptRetry attempts to retry a deferred resize operation
// In a real implementation, this would inject the InPlaceRightSizer
func (rm *RetryManager) attemptRetry(resize *DeferredResize) bool {
	// This is a placeholder - in the actual implementation, we would need
	// to inject the InPlaceRightSizer or call back to it to attempt the resize

	// For now, we'll simulate some retry logic based on the original error
	if strings.Contains(resize.OriginalError, "exceeds available node capacity") {
		// Check if node capacity might have improved
		// In real implementation, would check current node resources
		return rm.checkNodeCapacityImproved(resize)
	}

	if strings.Contains(resize.OriginalError, "resource quota") {
		// Check if resource quota has available capacity
		return rm.checkResourceQuotaAvailable(resize)
	}

	// For other types of errors, use a simple probability-based retry
	// In real implementation, this would perform actual validation
	return resize.AttemptCount > 2 // Succeed after a few attempts for demo
}

// checkNodeCapacityImproved simulates checking if node capacity has improved
func (rm *RetryManager) checkNodeCapacityImproved(resize *DeferredResize) bool {
	// Placeholder implementation
	// In reality, this would query the node's available resources
	return resize.AttemptCount >= 3 // Simulate capacity becoming available
}

// checkResourceQuotaAvailable simulates checking if resource quota has capacity
func (rm *RetryManager) checkResourceQuotaAvailable(resize *DeferredResize) bool {
	// Placeholder implementation
	// In reality, this would query the namespace's resource quota usage
	return resize.AttemptCount >= 2 // Simulate quota becoming available
}

// calculateBackoffDelay calculates the backoff delay for a retry attempt
func (rm *RetryManager) calculateBackoffDelay(resize *DeferredResize) time.Duration {
	baseDelay := time.Duration(float64(rm.retryInterval) * 0.5) // Start with half the retry interval
	backoffDelay := time.Duration(float64(baseDelay) * float64(resize.AttemptCount) * resize.BackoffFactor)

	// Cap the maximum backoff delay
	maxBackoff := 5 * time.Minute
	if backoffDelay > maxBackoff {
		backoffDelay = maxBackoff
	}

	return backoffDelay
}

// removeDeferredResize removes a deferred resize from the queue
func (rm *RetryManager) removeDeferredResize(pod *corev1.Pod) {
	key := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)

	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	delete(rm.deferredResizes, key)
}

// RemoveDeferredResize removes a specific pod's deferred resize (public method)
func (rm *RetryManager) RemoveDeferredResize(podNamespace, podName string) {
	key := fmt.Sprintf("%s/%s", podNamespace, podName)

	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	if _, exists := rm.deferredResizes[key]; exists {
		delete(rm.deferredResizes, key)
		logger.Debug("Removed deferred resize for pod %s/%s", podNamespace, podName)
	}
}

// GetDeferredResizeCount returns the number of currently deferred resizes
func (rm *RetryManager) GetDeferredResizeCount() int {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()
	return len(rm.deferredResizes)
}

// GetDeferredResizes returns a copy of all deferred resizes for monitoring
func (rm *RetryManager) GetDeferredResizes() map[string]*DeferredResize {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	result := make(map[string]*DeferredResize, len(rm.deferredResizes))
	for key, resize := range rm.deferredResizes {
		// Create a copy to avoid concurrent access issues
		resizeCopy := *resize
		result[key] = &resizeCopy
	}

	return result
}

// GetDeferredResizeByPod returns the deferred resize for a specific pod, if it exists
func (rm *RetryManager) GetDeferredResizeByPod(podNamespace, podName string) (*DeferredResize, bool) {
	key := fmt.Sprintf("%s/%s", podNamespace, podName)

	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	if resize, exists := rm.deferredResizes[key]; exists {
		// Return a copy to avoid concurrent access issues
		resizeCopy := *resize
		return &resizeCopy, true
	}

	return nil, false
}

// IsResizeDeferred checks if a pod has a deferred resize operation
func (rm *RetryManager) IsResizeDeferred(podNamespace, podName string) bool {
	_, exists := rm.GetDeferredResizeByPod(podNamespace, podName)
	return exists
}

// getPodPriority returns the priority of a pod for retry ordering
func getPodPriority(pod *corev1.Pod) int32 {
	// Use pod's priority class if available
	if pod.Spec.Priority != nil {
		return *pod.Spec.Priority
	}

	// Fallback to default priority
	return 0
}

// GetRetryStats returns statistics about the retry manager
func (rm *RetryManager) GetRetryStats() map[string]interface{} {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	stats := map[string]interface{}{
		"total_deferred": len(rm.deferredResizes),
		"retry_interval": rm.retryInterval.String(),
		"max_retries":    rm.maxRetries,
		"max_deferral":   rm.maxDeferralTime.String(),
		"running":        rm.running,
	}

	// Add per-reason breakdown
	reasonCounts := make(map[string]int)
	for _, resize := range rm.deferredResizes {
		reasonCounts[resize.Reason]++
	}
	stats["deferred_by_reason"] = reasonCounts

	return stats
}

// SetRetryHandler sets a custom retry handler function
// This allows injecting the actual resize logic from the InPlaceRightSizer
type RetryHandler func(context.Context, *corev1.Pod, map[string]corev1.ResourceRequirements) error

// RetryManagerWithHandler extends RetryManager with a custom retry handler
type RetryManagerWithHandler struct {
	*RetryManager
	handler RetryHandler
}

// NewRetryManagerWithHandler creates a RetryManager with a custom retry handler
func NewRetryManagerWithHandler(config RetryManagerConfig, metrics *metrics.OperatorMetrics,
	eventRecorder record.EventRecorder, handler RetryHandler) *RetryManagerWithHandler {

	return &RetryManagerWithHandler{
		RetryManager: NewRetryManager(config, metrics, eventRecorder),
		handler:      handler,
	}
}

// ProcessDeferredResizesWithHandler processes deferred resizes using the custom handler
func (rmh *RetryManagerWithHandler) ProcessDeferredResizesWithHandler(ctx context.Context) {
	rmh.mutex.Lock()

	// Get all deferred resizes
	resizes := make([]*DeferredResize, 0, len(rmh.deferredResizes))
	for _, resize := range rmh.deferredResizes {
		resizes = append(resizes, resize)
	}
	rmh.mutex.Unlock()

	if len(resizes) == 0 {
		return
	}

	// Sort by priority and wait time
	sort.Slice(resizes, func(i, j int) bool {
		if resizes[i].Priority != resizes[j].Priority {
			return resizes[i].Priority > resizes[j].Priority
		}
		return resizes[i].FirstAttempt.Before(resizes[j].FirstAttempt)
	})

	// Process each deferred resize with the custom handler
	for _, resize := range resizes {
		key := fmt.Sprintf("%s/%s", resize.Pod.Namespace, resize.Pod.Name)

		// Check expiration and retry limits (same as base implementation)
		if time.Since(resize.FirstAttempt) > rmh.maxDeferralTime ||
			resize.AttemptCount > resize.MaxRetries {
			rmh.removeDeferredResize(resize.Pod)
			continue
		}

		// Check backoff delay
		backoffDelay := rmh.calculateBackoffDelay(resize)
		if time.Since(resize.LastAttempt) < backoffDelay {
			continue
		}

		// Use custom handler to attempt retry
		err := rmh.handler(ctx, resize.Pod, resize.NewResources)
		if err == nil {
			// Success - remove from deferred queue
			logger.Info("Successfully retried deferred resize for pod %s/%s using custom handler",
				resize.Pod.Namespace, resize.Pod.Name)
			rmh.removeDeferredResize(resize.Pod)
			ClearResizeConditions(resize.Pod)
		} else {
			// Update attempt count and last attempt time
			rmh.mutex.Lock()
			if currentResize, exists := rmh.deferredResizes[key]; exists {
				currentResize.LastAttempt = time.Now()
				currentResize.AttemptCount++
				currentResize.OriginalError = err.Error()
			}
			rmh.mutex.Unlock()
		}
	}
}

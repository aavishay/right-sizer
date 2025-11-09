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

package events

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"right-sizer/metrics"
)

// RecommendationManager manages remediation recommendations and their lifecycle
type RecommendationManager struct {
	k8sClient kubernetes.Interface
	eventBus  *EventBus
	logger    logr.Logger
	metrics   *metrics.OperatorMetrics

	// Storage
	recommendations map[string]*Recommendation
	mutex           sync.RWMutex

	// Configuration
	maxRecommendations int
	expirationTime     time.Duration
	cleanupInterval    time.Duration

	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewRecommendationManager creates a new recommendation manager
func NewRecommendationManager(k8sClient kubernetes.Interface, eventBus *EventBus, logger logr.Logger, operatorMetrics *metrics.OperatorMetrics) *RecommendationManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &RecommendationManager{
		k8sClient:          k8sClient,
		eventBus:           eventBus,
		logger:             logger,
		metrics:            operatorMetrics,
		recommendations:    make(map[string]*Recommendation),
		maxRecommendations: 1000,
		expirationTime:     1 * time.Hour,
		cleanupInterval:    5 * time.Minute,
		ctx:                ctx,
		cancel:             cancel,
	}
}

// Start begins the recommendation manager
func (rm *RecommendationManager) Start() error {
	rm.wg.Add(1)
	go rm.cleanupLoop()
	rm.logger.Info("Recommendation manager started")
	return nil
}

// Stop stops the recommendation manager
func (rm *RecommendationManager) Stop() {
	rm.cancel()
	rm.wg.Wait()
	rm.logger.Info("Recommendation manager stopped")
}

// CreateRecommendation creates a new recommendation
func (rm *RecommendationManager) CreateRecommendation(
	eventID string,
	resourceType, resourceName, namespace string,
	title, description, action string,
	parameters map[string]interface{},
	urgency Urgency,
	severity Severity,
	confidence float64,
	timeToAction time.Duration,
) *Recommendation {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	// Generate unique ID
	id := fmt.Sprintf("rec-%d-%s", time.Now().Unix(), randomString(8))

	recommendation := &Recommendation{
		ID:           id,
		EventID:      eventID,
		ResourceType: resourceType,
		ResourceName: resourceName,
		Namespace:    namespace,
		Title:        title,
		Description:  description,
		Action:       action,
		Parameters:   parameters,
		Urgency:      urgency,
		Severity:     severity,
		Confidence:   confidence,
		TimeToAction: timeToAction,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(rm.expirationTime),
		Status:       RecommendationStatusPending,
		Tags:         []string{"auto-generated"},
	}

	// Enforce max recommendations limit
	if len(rm.recommendations) >= rm.maxRecommendations {
		rm.evictOldest()
	}

	rm.recommendations[id] = recommendation

	// Record metrics
	if rm.metrics != nil {
		rm.metrics.RecordRecommendationCreated(string(urgency), string(severity), action)
		rm.updatePendingRecommendationsMetric()
	}

	// Publish event
	event := NewEvent(EventRemediationProposed, "cluster-id", namespace, resourceName, SeverityInfo,
		fmt.Sprintf("New recommendation: %s", title))
	event.WithDetails(map[string]interface{}{
		"recommendationId": id,
		"urgency":          urgency,
		"severity":         severity,
		"confidence":       confidence,
		"timeToAction":     timeToAction.String(),
		"action":           action,
	})
	event.WithTags("recommendation", "created")

	rm.eventBus.Publish(event)

	rm.logger.Info("Created recommendation", "id", id, "action", action, "urgency", urgency)
	return recommendation
}

// GetRecommendations returns all recommendations sorted by urgency and creation time
func (rm *RecommendationManager) GetRecommendations() []*Recommendation {
	rm.mutex.RLock()
	defer rm.mutex.RUnlock()

	recommendations := make([]*Recommendation, 0, len(rm.recommendations))
	for _, rec := range rm.recommendations {
		recommendations = append(recommendations, rec)
	}

	// Sort by urgency (critical first), then by creation time (newest first)
	sort.Slice(recommendations, func(i, j int) bool {
		if recommendations[i].Urgency != recommendations[j].Urgency {
			return rm.urgencyPriority(recommendations[i].Urgency) > rm.urgencyPriority(recommendations[j].Urgency)
		}
		return recommendations[i].CreatedAt.After(recommendations[j].CreatedAt)
	})

	return recommendations
}

// GetRecommendationsByStatus returns recommendations filtered by status
func (rm *RecommendationManager) GetRecommendationsByStatus(status RecommendationStatus) []*Recommendation {
	all := rm.GetRecommendations()
	filtered := make([]*Recommendation, 0)

	for _, rec := range all {
		if rec.Status == status {
			filtered = append(filtered, rec)
		}
	}

	return filtered
}

// ApproveRecommendation approves a recommendation for execution
func (rm *RecommendationManager) ApproveRecommendation(id, approvedBy string) error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	rec, exists := rm.recommendations[id]
	if !exists {
		return fmt.Errorf("recommendation %s not found", id)
	}

	if rec.Status != RecommendationStatusPending {
		return fmt.Errorf("recommendation %s is not in pending status", id)
	}

	now := time.Now()
	rec.Status = RecommendationStatusApproved
	rec.ApprovedBy = approvedBy
	rec.ApprovedAt = &now

	// Publish approval event
	event := NewEvent(EventRemediationProposed, "cluster-id", rec.Namespace, rec.ResourceName, SeverityInfo,
		fmt.Sprintf("Recommendation approved: %s", rec.Title))
	event.WithDetails(map[string]interface{}{
		"recommendationId": id,
		"approvedBy":       approvedBy,
		"action":           rec.Action,
	})
	event.WithTags("recommendation", "approved")

	rm.eventBus.Publish(event)

	rm.logger.Info("Recommendation approved", "id", id, "approvedBy", approvedBy)
	return nil
}

// RejectRecommendation rejects a recommendation
func (rm *RecommendationManager) RejectRecommendation(id, rejectedBy, reason string) error {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	rec, exists := rm.recommendations[id]
	if !exists {
		return fmt.Errorf("recommendation %s not found", id)
	}

	if rec.Status != RecommendationStatusPending {
		return fmt.Errorf("recommendation %s is not in pending status", id)
	}

	rec.Status = RecommendationStatusRejected
	now := time.Now()
	rec.RejectedBy = rejectedBy
	rec.RejectedAt = &now
	rec.RejectedReason = reason

	// Publish rejection event
	event := NewEvent(EventRemediationProposed, "cluster-id", rec.Namespace, rec.ResourceName, SeverityInfo,
		fmt.Sprintf("Recommendation rejected: %s", rec.Title))
	event.WithDetails(map[string]interface{}{
		"recommendationId": id,
		"rejectedBy":       rejectedBy,
		"reason":           reason,
	})
	event.WithTags("recommendation", "rejected")

	rm.eventBus.Publish(event)

	rm.logger.Info("Recommendation rejected", "id", id, "rejectedBy", rejectedBy)
	return nil
}

// ExecuteRecommendation executes an approved recommendation
func (rm *RecommendationManager) ExecuteRecommendation(id string) error {
	rm.mutex.Lock()
	rec, exists := rm.recommendations[id]
	rm.mutex.Unlock()

	if !exists {
		return fmt.Errorf("recommendation %s not found", id)
	}

	if rec.Status != RecommendationStatusApproved {
		return fmt.Errorf("recommendation %s is not approved", id)
	}

	// Mark as executing
	rm.mutex.Lock()
	rec.Status = RecommendationStatusExecuting
	now := time.Now()
	rec.ExecutedAt = &now
	rm.mutex.Unlock()

	// Execute the action
	err := rm.executeAction(rec)

	rm.mutex.Lock()
	if err != nil {
		rec.Status = RecommendationStatusFailed
		rec.Error = err.Error()
		rm.logger.Error(err, "Recommendation execution failed", "id", id)
	} else {
		rec.Status = RecommendationStatusCompleted
		rec.Result = "success"
		rm.logger.Info("Recommendation executed successfully", "id", id)
	}
	rm.mutex.Unlock()

	// Publish execution result event
	eventType := EventRemediationApplied
	severity := SeverityInfo
	message := fmt.Sprintf("Recommendation executed: %s", rec.Title)

	if err != nil {
		eventType = EventRemediationFailed
		severity = SeverityWarning
		message = fmt.Sprintf("Recommendation failed: %s - %v", rec.Title, err)
	}

	event := NewEvent(eventType, "cluster-id", rec.Namespace, rec.ResourceName, severity, message)
	event.WithDetails(map[string]interface{}{
		"recommendationId": id,
		"action":           rec.Action,
		"result":           rec.Result,
		"error":            rec.Error,
	})
	event.WithTags("recommendation", "executed")

	rm.eventBus.Publish(event)

	return err
}

// executeAction performs the actual remediation action
func (rm *RecommendationManager) executeAction(rec *Recommendation) error {
	switch rec.Action {
	case "increase_memory_limit":
		return rm.executeIncreaseMemoryLimit(rec)
	case "increase_cpu_limit":
		return rm.executeIncreaseCPULimit(rec)
	case "drain_node":
		return rm.executeDrainNode(rec)
	default:
		return fmt.Errorf("unsupported action: %s", rec.Action)
	}
}

// executeIncreaseMemoryLimit increases memory limits for a pod
func (rm *RecommendationManager) executeIncreaseMemoryLimit(rec *Recommendation) error {
	// Get the pod
	pod, err := rm.k8sClient.CoreV1().Pods(rec.Namespace).Get(rm.ctx, rec.ResourceName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod: %v", err)
	}

	// Calculate new limits
	multiplier, ok := rec.Parameters["multiplier"].(float64)
	if !ok {
		multiplier = 1.5 // default
	}

	// Update each container
	for i, container := range pod.Spec.Containers {
		if container.Resources.Limits == nil {
			container.Resources.Limits = corev1.ResourceList{}
		}

		if memLimit := container.Resources.Limits.Memory(); memLimit != nil {
			newLimit := int64(float64(memLimit.Value()) * multiplier)
			pod.Spec.Containers[i].Resources.Limits[corev1.ResourceMemory] = *resource.NewQuantity(newLimit, resource.BinarySI)
		}
	}

	// Update the pod
	_, err = rm.k8sClient.CoreV1().Pods(rec.Namespace).Update(rm.ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update pod: %v", err)
	}

	rm.logger.Info("Increased memory limits", "pod", rec.ResourceName, "namespace", rec.Namespace, "multiplier", multiplier)
	return nil
}

// executeIncreaseCPULimit increases CPU limits for a pod
func (rm *RecommendationManager) executeIncreaseCPULimit(rec *Recommendation) error {
	// Similar to memory limit increase but for CPU
	pod, err := rm.k8sClient.CoreV1().Pods(rec.Namespace).Get(rm.ctx, rec.ResourceName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod: %v", err)
	}

	multiplier, ok := rec.Parameters["multiplier"].(float64)
	if !ok {
		multiplier = 1.3 // default
	}

	for i, container := range pod.Spec.Containers {
		if container.Resources.Limits == nil {
			container.Resources.Limits = corev1.ResourceList{}
		}

		if cpuLimit := container.Resources.Limits.Cpu(); cpuLimit != nil {
			newLimit := int64(float64(cpuLimit.MilliValue()) * multiplier)
			pod.Spec.Containers[i].Resources.Limits[corev1.ResourceCPU] = *resource.NewMilliQuantity(newLimit, resource.DecimalSI)
		}
	}

	_, err = rm.k8sClient.CoreV1().Pods(rec.Namespace).Update(rm.ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update pod: %v", err)
	}

	rm.logger.Info("Increased CPU limits", "pod", rec.ResourceName, "namespace", rec.Namespace, "multiplier", multiplier)
	return nil
}

// executeDrainNode drains a node (placeholder - would need more complex logic)
func (rm *RecommendationManager) executeDrainNode(rec *Recommendation) error {
	// This is a placeholder - actual node draining would require:
	// 1. Cordon the node
	// 2. Evict all pods
	// 3. Wait for pods to be rescheduled
	// 4. Uncordon or remove the node

	rm.logger.Info("Node drain requested (not implemented)", "node", rec.ResourceName)
	return fmt.Errorf("node draining not yet implemented")
}

// cleanupLoop periodically cleans up expired recommendations
func (rm *RecommendationManager) cleanupLoop() {
	defer rm.wg.Done()

	ticker := time.NewTicker(rm.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rm.ctx.Done():
			return
		case <-ticker.C:
			rm.cleanupExpired()
		}
	}
}

// cleanupExpired removes expired recommendations
func (rm *RecommendationManager) cleanupExpired() {
	rm.mutex.Lock()
	defer rm.mutex.Unlock()

	now := time.Now()
	expired := make([]string, 0)

	for id, rec := range rm.recommendations {
		if now.After(rec.ExpiresAt) && rec.Status == RecommendationStatusPending {
			rec.Status = RecommendationStatusExpired
			expired = append(expired, id)
		}
	}

	for _, id := range expired {
		delete(rm.recommendations, id)
		rm.logger.Info("Expired recommendation cleaned up", "id", id)
	}
}

// evictOldest removes the oldest recommendations when at capacity
func (rm *RecommendationManager) evictOldest() {
	oldestID := ""
	oldestTime := time.Now()

	for id, rec := range rm.recommendations {
		if rec.CreatedAt.Before(oldestTime) {
			oldestID = id
			oldestTime = rec.CreatedAt
		}
	}

	if oldestID != "" {
		delete(rm.recommendations, oldestID)
		rm.logger.Info("Evicted oldest recommendation", "id", oldestID)
	}
}

// updatePendingRecommendationsMetric updates the pending recommendations metric
func (rm *RecommendationManager) updatePendingRecommendationsMetric() {
	if rm.metrics == nil {
		return
	}

	pendingCount := 0
	for _, rec := range rm.recommendations {
		if rec.Status == RecommendationStatusPending {
			pendingCount++
		}
	}

	rm.metrics.UpdatePendingRecommendations(float64(pendingCount))
}

// urgencyPriority returns a priority number for sorting (higher = more urgent)
func (rm *RecommendationManager) urgencyPriority(urgency Urgency) int {
	switch urgency {
	case UrgencyCritical:
		return 4
	case UrgencyHigh:
		return 3
	case UrgencyMedium:
		return 2
	case UrgencyLow:
		return 1
	default:
		return 0
	}
}

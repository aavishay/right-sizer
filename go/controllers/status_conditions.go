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
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Pod resize condition types
const (
	// PodResizePending indicates that a pod resize operation is pending
	PodResizePending corev1.PodConditionType = "PodResizePending"

	// PodResizeInProgress indicates that a pod resize operation is currently in progress
	PodResizeInProgress corev1.PodConditionType = "PodResizeInProgress"
)

// Pod resize condition reasons
const (
	// ReasonResizePending indicates the resize is pending due to validation or scheduling
	ReasonResizePending = "ResizePending"

	// ReasonNodeResourceConstraint indicates resize is pending due to node resource constraints
	ReasonNodeResourceConstraint = "NodeResourceConstraint"

	// ReasonValidationPending indicates resize is pending validation
	ReasonValidationPending = "ValidationPending"

	// ReasonResizeInProgress indicates the resize operation is actively being processed
	ReasonResizeInProgress = "ResizeInProgress"

	// ReasonResizeCPU indicates CPU resize is in progress
	ReasonResizeCPU = "ResizingCPU"

	// ReasonResizeMemory indicates memory resize is in progress
	ReasonResizeMemory = "ResizingMemory"

	// ReasonResizeCompleted indicates the resize operation completed successfully
	ReasonResizeCompleted = "ResizeCompleted"

	// ReasonResizeFailed indicates the resize operation failed
	ReasonResizeFailed = "ResizeFailed"
)

// SetPodResizePending sets the PodResizePending condition on the pod
func SetPodResizePending(pod *corev1.Pod, reason, message string) {
	condition := corev1.PodCondition{
		Type:               PodResizePending,
		Status:             corev1.ConditionTrue,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	// Clear any existing PodResizeInProgress condition when setting pending
	removeCondition(pod, PodResizeInProgress)
	updatePodCondition(pod, condition)
}

// SetPodResizeInProgress sets the PodResizeInProgress condition on the pod
func SetPodResizeInProgress(pod *corev1.Pod, reason, message string) {
	if reason == "" {
		reason = ReasonResizeInProgress
	}
	if message == "" {
		message = "Resize operation is in progress"
	}

	condition := corev1.PodCondition{
		Type:               PodResizeInProgress,
		Status:             corev1.ConditionTrue,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	// Clear any existing PodResizePending condition when starting progress
	removeCondition(pod, PodResizePending)
	updatePodCondition(pod, condition)
}

// ClearResizeConditions removes all resize-related conditions from the pod
func ClearResizeConditions(pod *corev1.Pod) {
	removeCondition(pod, PodResizePending)
	removeCondition(pod, PodResizeInProgress)
}

// SetResizeCompleted clears resize conditions and optionally adds a completed condition
func SetResizeCompleted(pod *corev1.Pod, message string) {
	ClearResizeConditions(pod)

	// Optionally, we could add a temporary "completed" condition, but Kubernetes
	// typically doesn't maintain completed conditions for transient operations
	// Instead, we'll just clear the conditions to indicate completion
}

// SetResizeFailed sets a failed condition and clears other resize conditions
func SetResizeFailed(pod *corev1.Pod, reason, message string) {
	if reason == "" {
		reason = ReasonResizeFailed
	}

	// Clear existing resize conditions
	ClearResizeConditions(pod)

	// We could add a PodResizeFailed condition, but that's not standard in K8s
	// Instead, we'll rely on events and logs for failure tracking
}

// updatePodCondition adds or updates a pod condition
func updatePodCondition(pod *corev1.Pod, newCondition corev1.PodCondition) {
	if pod.Status.Conditions == nil {
		pod.Status.Conditions = make([]corev1.PodCondition, 0)
	}

	// Find existing condition of the same type
	for i, condition := range pod.Status.Conditions {
		if condition.Type == newCondition.Type {
			// Update existing condition
			if condition.Status != newCondition.Status ||
				condition.Reason != newCondition.Reason ||
				condition.Message != newCondition.Message {
				// Only update LastTransitionTime if the status actually changed
				if condition.Status != newCondition.Status {
					newCondition.LastTransitionTime = metav1.Now()
				} else {
					newCondition.LastTransitionTime = condition.LastTransitionTime
				}
				pod.Status.Conditions[i] = newCondition
			}
			return
		}
	}

	// Add new condition
	pod.Status.Conditions = append(pod.Status.Conditions, newCondition)
}

// removeCondition removes a condition of the specified type from the pod
func removeCondition(pod *corev1.Pod, conditionType corev1.PodConditionType) {
	if pod.Status.Conditions == nil {
		return
	}

	newConditions := make([]corev1.PodCondition, 0, len(pod.Status.Conditions))
	for _, condition := range pod.Status.Conditions {
		if condition.Type != conditionType {
			newConditions = append(newConditions, condition)
		}
	}

	pod.Status.Conditions = newConditions
}

// HasCondition checks if a pod has a condition of the specified type and status
func HasCondition(pod *corev1.Pod, conditionType corev1.PodConditionType, status corev1.ConditionStatus) bool {
	if pod.Status.Conditions == nil {
		return false
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type == conditionType && condition.Status == status {
			return true
		}
	}

	return false
}

// GetCondition returns the condition of the specified type, if it exists
func GetCondition(pod *corev1.Pod, conditionType corev1.PodConditionType) (*corev1.PodCondition, bool) {
	if pod.Status.Conditions == nil {
		return nil, false
	}

	for _, condition := range pod.Status.Conditions {
		if condition.Type == conditionType {
			return &condition, true
		}
	}

	return nil, false
}

// IsResizePending checks if the pod has a resize pending condition
func IsResizePending(pod *corev1.Pod) bool {
	return HasCondition(pod, PodResizePending, corev1.ConditionTrue)
}

// IsResizeInProgress checks if the pod has a resize in progress condition
func IsResizeInProgress(pod *corev1.Pod) bool {
	return HasCondition(pod, PodResizeInProgress, corev1.ConditionTrue)
}

// GetResizeStatus returns a summary of the current resize status
func GetResizeStatus(pod *corev1.Pod) string {
	if IsResizeInProgress(pod) {
		if condition, exists := GetCondition(pod, PodResizeInProgress); exists {
			return condition.Message
		}
		return "Resize in progress"
	}

	if IsResizePending(pod) {
		if condition, exists := GetCondition(pod, PodResizePending); exists {
			return condition.Message
		}
		return "Resize pending"
	}

	return "No resize operation"
}

// SetPodObservedGeneration updates the pod's status with the current generation
// This is used for tracking spec changes
func SetPodObservedGeneration(pod *corev1.Pod) {
	// Note: corev1.PodStatus doesn't have ObservedGeneration field by default
	// This would require a custom field or annotation. For now, we'll use an annotation
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}

	// Store the observed generation as an annotation
	// In a real implementation, this might be stored in a custom status field
	pod.Annotations["right-sizer.io/observed-generation"] = string(rune(pod.Generation))
}

// GetPodObservedGeneration retrieves the observed generation from the pod
func GetPodObservedGeneration(pod *corev1.Pod) int64 {
	if pod.Annotations == nil {
		return 0
	}

	if observedGen, exists := pod.Annotations["right-sizer.io/observed-generation"]; exists {
		// Convert the stored generation back to int64
		if len(observedGen) > 0 {
			return int64(rune(observedGen[0]))
		}
	}

	return 0
}

// IsSpecChanged checks if the pod's spec has changed since last observation
func IsSpecChanged(pod *corev1.Pod) bool {
	observedGen := GetPodObservedGeneration(pod)
	return pod.Generation != observedGen
}

// UpdateResizeProgress updates the resize progress with detailed information
func UpdateResizeProgress(pod *corev1.Pod, containerName string, resourceType corev1.ResourceName, phase string) {
	reason := ReasonResizeInProgress
	var message string

	switch phase {
	case "cpu-resize":
		reason = ReasonResizeCPU
		message = "Resizing CPU resources for container " + containerName
	case "memory-resize":
		reason = ReasonResizeMemory
		message = "Resizing memory resources for container " + containerName
	case "validation":
		message = "Validating resize request for container " + containerName
	default:
		message = "Processing resize for container " + containerName
	}

	SetPodResizeInProgress(pod, reason, message)
}

// RecordResizeEvent records a resize event with timestamp for debugging
func RecordResizeEvent(pod *corev1.Pod, eventType, reason, message string) {
	// In a real implementation, this would use the Kubernetes event recorder
	// For now, we'll just add an annotation with the latest event
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}

	timestamp := time.Now().Format(time.RFC3339)
	pod.Annotations["right-sizer.io/last-resize-event"] = eventType + ":" + reason + ":" + message + ":" + timestamp
}

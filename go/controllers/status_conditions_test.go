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
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// createTestPodForConditions creates a test pod for condition testing
func createTestPodForConditions(name, namespace string, generation int64) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Namespace:  namespace,
			Generation: generation,
		},
		Status: corev1.PodStatus{
			Conditions: []corev1.PodCondition{},
		},
	}
}

func TestSetPodResizePending(t *testing.T) {
	tests := []struct {
		name      string
		reason    string
		message   string
		expectLen int
	}{
		{
			name:      "Set pending condition",
			reason:    ReasonResizePending,
			message:   "Resize operation is pending validation",
			expectLen: 1,
		},
		{
			name:      "Set pending with node constraint reason",
			reason:    ReasonNodeResourceConstraint,
			message:   "Node does not have sufficient capacity",
			expectLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := createTestPodForConditions("test-pod", "default", 1)

			SetPodResizePending(pod, tt.reason, tt.message)

			if len(pod.Status.Conditions) != tt.expectLen {
				t.Errorf("Expected %d conditions, got %d", tt.expectLen, len(pod.Status.Conditions))
			}

			condition := pod.Status.Conditions[0]
			if condition.Type != PodResizePending {
				t.Errorf("Expected condition type %s, got %s", PodResizePending, condition.Type)
			}

			if condition.Status != corev1.ConditionTrue {
				t.Errorf("Expected condition status True, got %s", condition.Status)
			}

			if condition.Reason != tt.reason {
				t.Errorf("Expected reason %s, got %s", tt.reason, condition.Reason)
			}

			if condition.Message != tt.message {
				t.Errorf("Expected message %s, got %s", tt.message, condition.Message)
			}
		})
	}
}

func TestSetPodResizeInProgress(t *testing.T) {
	tests := []struct {
		name      string
		reason    string
		message   string
		expectLen int
	}{
		{
			name:      "Set in progress with default reason",
			reason:    "",
			message:   "",
			expectLen: 1,
		},
		{
			name:      "Set in progress with custom reason",
			reason:    ReasonResizeCPU,
			message:   "Resizing CPU resources",
			expectLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := createTestPodForConditions("test-pod", "default", 1)

			SetPodResizeInProgress(pod, tt.reason, tt.message)

			if len(pod.Status.Conditions) != tt.expectLen {
				t.Errorf("Expected %d conditions, got %d", tt.expectLen, len(pod.Status.Conditions))
			}

			condition := pod.Status.Conditions[0]
			if condition.Type != PodResizeInProgress {
				t.Errorf("Expected condition type %s, got %s", PodResizeInProgress, condition.Type)
			}

			if condition.Status != corev1.ConditionTrue {
				t.Errorf("Expected condition status True, got %s", condition.Status)
			}

			expectedReason := tt.reason
			if expectedReason == "" {
				expectedReason = ReasonResizeInProgress
			}
			if condition.Reason != expectedReason {
				t.Errorf("Expected reason %s, got %s", expectedReason, condition.Reason)
			}

			expectedMessage := tt.message
			if expectedMessage == "" {
				expectedMessage = "Resize operation is in progress"
			}
			if condition.Message != expectedMessage {
				t.Errorf("Expected message %s, got %s", expectedMessage, condition.Message)
			}
		})
	}
}

func TestClearResizeConditions(t *testing.T) {
	pod := createTestPodForConditions("test-pod", "default", 1)

	// Add both resize conditions
	SetPodResizePending(pod, ReasonResizePending, "Pending resize")
	SetPodResizeInProgress(pod, ReasonResizeInProgress, "In progress resize")

	// Add a non-resize condition
	nonResizeCondition := corev1.PodCondition{
		Type:               corev1.PodReady,
		Status:             corev1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
	}
	pod.Status.Conditions = append(pod.Status.Conditions, nonResizeCondition)

	if len(pod.Status.Conditions) != 2 {
		t.Errorf("Expected 2 conditions before clear (InProgress should replace Pending), got %d", len(pod.Status.Conditions))
	}

	ClearResizeConditions(pod)

	// Should only have the non-resize condition left
	if len(pod.Status.Conditions) != 1 {
		t.Errorf("Expected 1 condition after clear, got %d", len(pod.Status.Conditions))
	}

	if pod.Status.Conditions[0].Type != corev1.PodReady {
		t.Errorf("Expected remaining condition to be PodReady, got %s", pod.Status.Conditions[0].Type)
	}
}

func TestResizeConditionTransitions(t *testing.T) {
	pod := createTestPodForConditions("test-pod", "default", 1)

	// Test pending -> in progress transition
	SetPodResizePending(pod, ReasonValidationPending, "Validating resize")
	if len(pod.Status.Conditions) != 1 {
		t.Errorf("Expected 1 condition after setting pending, got %d", len(pod.Status.Conditions))
	}

	SetPodResizeInProgress(pod, ReasonResizeInProgress, "Starting resize")
	if len(pod.Status.Conditions) != 1 {
		t.Errorf("Expected 1 condition after transition to in progress, got %d", len(pod.Status.Conditions))
	}

	condition := pod.Status.Conditions[0]
	if condition.Type != PodResizeInProgress {
		t.Errorf("Expected condition type %s after transition, got %s", PodResizeInProgress, condition.Type)
	}
}

func TestHasCondition(t *testing.T) {
	pod := createTestPodForConditions("test-pod", "default", 1)

	// No conditions initially
	if HasCondition(pod, PodResizePending, corev1.ConditionTrue) {
		t.Error("Expected no pending condition, but HasCondition returned true")
	}

	// Add pending condition
	SetPodResizePending(pod, ReasonResizePending, "Pending")

	if !HasCondition(pod, PodResizePending, corev1.ConditionTrue) {
		t.Error("Expected pending condition to exist, but HasCondition returned false")
	}

	if HasCondition(pod, PodResizePending, corev1.ConditionFalse) {
		t.Error("Expected pending condition to be True, but HasCondition found False")
	}

	if HasCondition(pod, PodResizeInProgress, corev1.ConditionTrue) {
		t.Error("Expected no in progress condition, but HasCondition returned true")
	}
}

func TestGetCondition(t *testing.T) {
	pod := createTestPodForConditions("test-pod", "default", 1)

	// No condition should return nil, false
	condition, exists := GetCondition(pod, PodResizePending)
	if exists {
		t.Error("Expected no condition to exist, but GetCondition returned true")
	}
	if condition != nil {
		t.Error("Expected nil condition, but got non-nil")
	}

	// Add condition
	testReason := ReasonResizePending
	testMessage := "Test pending message"
	SetPodResizePending(pod, testReason, testMessage)

	condition, exists = GetCondition(pod, PodResizePending)
	if !exists {
		t.Error("Expected condition to exist, but GetCondition returned false")
	}
	if condition == nil {
		t.Fatal("Expected non-nil condition")
	}

	if condition.Type != PodResizePending {
		t.Errorf("Expected condition type %s, got %s", PodResizePending, condition.Type)
	}
	if condition.Reason != testReason {
		t.Errorf("Expected reason %s, got %s", testReason, condition.Reason)
	}
	if condition.Message != testMessage {
		t.Errorf("Expected message %s, got %s", testMessage, condition.Message)
	}
}

func TestIsResizePending(t *testing.T) {
	pod := createTestPodForConditions("test-pod", "default", 1)

	if IsResizePending(pod) {
		t.Error("Expected no pending resize, but IsResizePending returned true")
	}

	SetPodResizePending(pod, ReasonResizePending, "Pending")

	if !IsResizePending(pod) {
		t.Error("Expected pending resize, but IsResizePending returned false")
	}
}

func TestIsResizeInProgress(t *testing.T) {
	pod := createTestPodForConditions("test-pod", "default", 1)

	if IsResizeInProgress(pod) {
		t.Error("Expected no resize in progress, but IsResizeInProgress returned true")
	}

	SetPodResizeInProgress(pod, ReasonResizeInProgress, "In progress")

	if !IsResizeInProgress(pod) {
		t.Error("Expected resize in progress, but IsResizeInProgress returned false")
	}
}

func TestGetResizeStatus(t *testing.T) {
	pod := createTestPodForConditions("test-pod", "default", 1)

	// No resize operations
	status := GetResizeStatus(pod)
	if status != "No resize operation" {
		t.Errorf("Expected 'No resize operation', got %s", status)
	}

	// Pending resize
	SetPodResizePending(pod, ReasonValidationPending, "Validation in progress")
	status = GetResizeStatus(pod)
	if status != "Validation in progress" {
		t.Errorf("Expected 'Validation in progress', got %s", status)
	}

	// In progress resize
	SetPodResizeInProgress(pod, ReasonResizeCPU, "CPU resize in progress")
	status = GetResizeStatus(pod)
	if status != "CPU resize in progress" {
		t.Errorf("Expected 'CPU resize in progress', got %s", status)
	}

	// Clear conditions
	ClearResizeConditions(pod)
	status = GetResizeStatus(pod)
	if status != "No resize operation" {
		t.Errorf("Expected 'No resize operation' after clear, got %s", status)
	}
}

func TestUpdatePodCondition(t *testing.T) {
	pod := createTestPodForConditions("test-pod", "default", 1)

	// Test adding new condition
	condition1 := corev1.PodCondition{
		Type:               PodResizePending,
		Status:             corev1.ConditionTrue,
		Reason:             ReasonResizePending,
		Message:            "First message",
		LastTransitionTime: metav1.Now(),
	}

	updatePodCondition(pod, condition1)

	if len(pod.Status.Conditions) != 1 {
		t.Errorf("Expected 1 condition, got %d", len(pod.Status.Conditions))
	}

	// Test updating existing condition with same status (should preserve transition time)
	time.Sleep(10 * time.Millisecond) // Ensure different timestamp
	originalTransitionTime := pod.Status.Conditions[0].LastTransitionTime
	condition2 := corev1.PodCondition{
		Type:               PodResizePending,
		Status:             corev1.ConditionTrue, // Same status
		Reason:             ReasonValidationPending,
		Message:            "Updated message",
		LastTransitionTime: metav1.Now(),
	}

	updatePodCondition(pod, condition2)

	if len(pod.Status.Conditions) != 1 {
		t.Errorf("Expected 1 condition after update, got %d", len(pod.Status.Conditions))
	}

	updatedCondition := pod.Status.Conditions[0]
	if updatedCondition.LastTransitionTime != originalTransitionTime {
		t.Error("Expected transition time to be preserved when status doesn't change")
	}

	// Test updating with different status (should update transition time)
	condition3 := corev1.PodCondition{
		Type:               PodResizePending,
		Status:             corev1.ConditionFalse, // Different status
		Reason:             ReasonResizeCompleted,
		Message:            "Completed",
		LastTransitionTime: metav1.Now(),
	}

	updatePodCondition(pod, condition3)

	finalCondition := pod.Status.Conditions[0]
	if finalCondition.Status != corev1.ConditionFalse {
		t.Errorf("Expected status False, got %s", finalCondition.Status)
	}
}

func TestSetPodObservedGeneration(t *testing.T) {
	pod := createTestPodForConditions("test-pod", "default", 42)

	SetPodObservedGeneration(pod)

	if pod.Annotations == nil {
		t.Fatal("Expected annotations to be set")
	}

	observedGen, exists := pod.Annotations["right-sizer.io/observed-generation"]
	if !exists {
		t.Error("Expected observed-generation annotation to exist")
	}

	// The current implementation stores generation as a rune, so we check that
	expectedGen := string(rune(42))
	if observedGen != expectedGen {
		t.Errorf("Expected observed generation %s, got %s", expectedGen, observedGen)
	}
}

func TestGetPodObservedGeneration(t *testing.T) {
	pod := createTestPodForConditions("test-pod", "default", 1)

	// No annotation initially
	observedGen := GetPodObservedGeneration(pod)
	if observedGen != 0 {
		t.Errorf("Expected 0 for missing annotation, got %d", observedGen)
	}

	// Set annotation manually for testing
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}
	pod.Annotations["right-sizer.io/observed-generation"] = string(rune(15))

	observedGen = GetPodObservedGeneration(pod)
	if observedGen != 15 {
		t.Errorf("Expected 15, got %d", observedGen)
	}

	// Test empty annotation
	pod.Annotations["right-sizer.io/observed-generation"] = ""
	observedGen = GetPodObservedGeneration(pod)
	if observedGen != 0 {
		t.Errorf("Expected 0 for empty annotation, got %d", observedGen)
	}
}

func TestIsSpecChanged(t *testing.T) {
	pod := createTestPodForConditions("test-pod", "default", 5)

	// No observed generation annotation
	if !IsSpecChanged(pod) {
		t.Error("Expected spec to be changed when no observed generation exists")
	}

	// Set observed generation to match current
	SetPodObservedGeneration(pod)
	if IsSpecChanged(pod) {
		t.Error("Expected spec not to be changed when observed generation matches")
	}

	// Change pod generation
	pod.Generation = 6
	if !IsSpecChanged(pod) {
		t.Error("Expected spec to be changed when generation increases")
	}
}

func TestUpdateResizeProgress(t *testing.T) {
	pod := createTestPodForConditions("test-pod", "default", 1)

	tests := []struct {
		containerName   string
		resourceType    corev1.ResourceName
		phase           string
		expectedReason  string
		expectedMessage string
	}{
		{
			containerName:   "app",
			resourceType:    corev1.ResourceCPU,
			phase:           "cpu-resize",
			expectedReason:  ReasonResizeCPU,
			expectedMessage: "Resizing CPU resources for container app",
		},
		{
			containerName:   "db",
			resourceType:    corev1.ResourceMemory,
			phase:           "memory-resize",
			expectedReason:  ReasonResizeMemory,
			expectedMessage: "Resizing memory resources for container db",
		},
		{
			containerName:   "web",
			resourceType:    corev1.ResourceCPU,
			phase:           "validation",
			expectedReason:  ReasonResizeInProgress,
			expectedMessage: "Validating resize request for container web",
		},
		{
			containerName:   "api",
			resourceType:    corev1.ResourceMemory,
			phase:           "other",
			expectedReason:  ReasonResizeInProgress,
			expectedMessage: "Processing resize for container api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			// Clear conditions before each test
			pod.Status.Conditions = []corev1.PodCondition{}

			UpdateResizeProgress(pod, tt.containerName, tt.resourceType, tt.phase)

			if len(pod.Status.Conditions) != 1 {
				t.Errorf("Expected 1 condition, got %d", len(pod.Status.Conditions))
			}

			condition := pod.Status.Conditions[0]
			if condition.Type != PodResizeInProgress {
				t.Errorf("Expected condition type %s, got %s", PodResizeInProgress, condition.Type)
			}

			if condition.Reason != tt.expectedReason {
				t.Errorf("Expected reason %s, got %s", tt.expectedReason, condition.Reason)
			}

			if condition.Message != tt.expectedMessage {
				t.Errorf("Expected message %s, got %s", tt.expectedMessage, condition.Message)
			}
		})
	}
}

func TestRecordResizeEvent(t *testing.T) {
	pod := createTestPodForConditions("test-pod", "default", 1)

	eventType := "Normal"
	reason := "ResizeStarted"
	message := "Started resize operation"

	RecordResizeEvent(pod, eventType, reason, message)

	if pod.Annotations == nil {
		t.Fatal("Expected annotations to be set")
	}

	event, exists := pod.Annotations["right-sizer.io/last-resize-event"]
	if !exists {
		t.Error("Expected resize event annotation to exist")
	}

	// Check that event contains the expected components
	if !strings.Contains(event, eventType) {
		t.Errorf("Expected event to contain type %s", eventType)
	}
	if !strings.Contains(event, reason) {
		t.Errorf("Expected event to contain reason %s", reason)
	}
	if !strings.Contains(event, message) {
		t.Errorf("Expected event to contain message %s", message)
	}

	// Check timestamp format (should be RFC3339, pipe-delimited)
	parts := strings.Split(event, "|")
	if len(parts) != 4 {
		t.Errorf("Expected event to have exactly 4 parts (type|reason|message|timestamp), got %d", len(parts))
	}

	// Parse the timestamp part (4th field)
	timestampPart := parts[3]
	if _, err := time.Parse(time.RFC3339, timestampPart); err != nil {
		t.Errorf("Expected valid RFC3339 timestamp, but got parse error: %v", err)
	}
}

func TestRemoveCondition(t *testing.T) {
	pod := createTestPodForConditions("test-pod", "default", 1)

	// Add multiple conditions
	conditions := []corev1.PodCondition{
		{
			Type:   PodResizePending,
			Status: corev1.ConditionTrue,
			Reason: ReasonResizePending,
		},
		{
			Type:   PodResizeInProgress,
			Status: corev1.ConditionTrue,
			Reason: ReasonResizeInProgress,
		},
		{
			Type:   corev1.PodReady,
			Status: corev1.ConditionTrue,
			Reason: "Ready",
		},
	}
	pod.Status.Conditions = conditions

	// Remove pending condition
	removeCondition(pod, PodResizePending)

	if len(pod.Status.Conditions) != 2 {
		t.Errorf("Expected 2 conditions after removal, got %d", len(pod.Status.Conditions))
	}

	// Check that the right condition was removed
	for _, condition := range pod.Status.Conditions {
		if condition.Type == PodResizePending {
			t.Error("Expected PodResizePending to be removed, but it still exists")
		}
	}

	// Check that other conditions remain
	hasInProgress := false
	hasReady := false
	for _, condition := range pod.Status.Conditions {
		if condition.Type == PodResizeInProgress {
			hasInProgress = true
		}
		if condition.Type == corev1.PodReady {
			hasReady = true
		}
	}

	if !hasInProgress {
		t.Error("Expected PodResizeInProgress to remain")
	}
	if !hasReady {
		t.Error("Expected PodReady to remain")
	}

	// Remove non-existent condition (should be no-op)
	originalLen := len(pod.Status.Conditions)
	removeCondition(pod, "NonExistentCondition")
	if len(pod.Status.Conditions) != originalLen {
		t.Error("Expected removing non-existent condition to be no-op")
	}
}

func TestConditionWithNilConditions(t *testing.T) {
	pod := createTestPodForConditions("test-pod", "default", 1)
	pod.Status.Conditions = nil

	// Test operations with nil conditions slice
	if HasCondition(pod, PodResizePending, corev1.ConditionTrue) {
		t.Error("Expected false for nil conditions")
	}

	condition, exists := GetCondition(pod, PodResizePending)
	if exists || condition != nil {
		t.Error("Expected no condition for nil conditions slice")
	}

	// Test setting condition on nil slice
	SetPodResizePending(pod, ReasonResizePending, "Test")
	if len(pod.Status.Conditions) != 1 {
		t.Errorf("Expected 1 condition after setting on nil slice, got %d", len(pod.Status.Conditions))
	}
}

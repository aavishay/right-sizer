// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package events

import (
	"encoding/json"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// EventType represents the type of cluster event
type EventType string

const (
	// Resource Events
	EventResourceOptimized      EventType = "resource.optimized"
	EventResourceExhaustion     EventType = "resource.exhaustion"
	EventResourceUnderUtilized  EventType = "resource.underutilized"
	EventResourcePredictedOOM   EventType = "resource.predicted_oom"
	EventResourcePredictedCrash EventType = "resource.predicted_crash"

	// Pod Events
	EventPodOOMKilled        EventType = "pod.oom_killed"
	EventPodCrashLoop        EventType = "pod.crash_loop"
	EventPodPending          EventType = "pod.pending"
	EventPodFailedScheduling EventType = "pod.failed_scheduling"
	EventPodCPUThrottled     EventType = "pod.cpu_throttled"
	EventPodLivenessFailed   EventType = "pod.liveness_failed"
	EventPodReadinessFailed  EventType = "pod.readiness_failed"
	EventPodNetworkIssue     EventType = "pod.network_issue"
	EventPodDNSIssue         EventType = "pod.dns_issue"
	EventPodAffinityIssue    EventType = "pod.affinity_issue"
	EventPodImagePullIssue   EventType = "pod.image_pull_issue"
	EventPodVolumeIssue      EventType = "pod.volume_issue"
	EventPodEvicted          EventType = "pod.evicted"
	EventPodStarted          EventType = "pod.started"
	EventPodTerminated       EventType = "pod.terminated"
	EventPodFailed           EventType = "pod.failed"
	EventPodPredictedFailure EventType = "pod.predicted_failure"
	EventDiskPressure        EventType = "pod.disk_pressure"
	EventPodRestarts         EventType = "pod.restarts"

	// Node Events
	EventNodeReady            EventType = "node.ready"
	EventNodeNotReady         EventType = "node.not_ready"
	EventNodePressure         EventType = "node.pressure"
	EventNodeResourcesFull    EventType = "node.resources_full"
	EventNodePredictedFailure EventType = "node.predicted_failure"

	// Controller Events
	EventDeploymentScaled  EventType = "deployment.scaled"
	EventStatefulSetScaled EventType = "statefulset.scaled"
	EventReplicaSetUpdated EventType = "replicaset.updated"

	// System Events
	EventHealthCheckFailed    EventType = "system.health_check_failed"
	EventConfigurationChanged EventType = "system.config_changed"
	EventRemediationApplied   EventType = "system.remediation_applied"
	EventRemediationFailed    EventType = "system.remediation_failed"
	EventRemediationProposed  EventType = "system.remediation_proposed"

	// Dashboard Events
	EventDashboardConnected    EventType = "dashboard.connected"
	EventDashboardDisconnected EventType = "dashboard.disconnected"
	EventDashboardCommand      EventType = "dashboard.command"
)

// Event represents a cluster event that can be streamed to dashboard
type Event struct {
	ID            string                 `json:"id"`
	Type          EventType              `json:"type"`
	Timestamp     time.Time              `json:"timestamp"`
	ClusterID     string                 `json:"clusterId"`
	Namespace     string                 `json:"namespace,omitempty"`
	Resource      string                 `json:"resource,omitempty"` // pod/deployment/node name
	Severity      Severity               `json:"severity"`
	Message       string                 `json:"message"`
	Details       map[string]interface{} `json:"details,omitempty"`
	Tags          []string               `json:"tags,omitempty"`
	Source        string                 `json:"source"` // right-sizer-operator
	CorrelationID string                 `json:"correlationId,omitempty"`
}

// Severity represents event severity
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityError    Severity = "error"
	SeverityCritical Severity = "critical"
)

// Urgency represents recommendation urgency
type Urgency string

const (
	UrgencyLow      Urgency = "low"
	UrgencyMedium   Urgency = "medium"
	UrgencyHigh     Urgency = "high"
	UrgencyCritical Urgency = "critical"
)

// ResourceEvent contains resource-specific event data
type ResourceEvent struct {
	PodName         string                      `json:"podName"`
	ContainerName   string                      `json:"containerName"`
	PreviousRequest corev1.ResourceRequirements `json:"previousRequest"`
	NewRequest      corev1.ResourceRequirements `json:"newRequest"`
	Reason          string                      `json:"reason"`
	Recommendation  *ResourceRecommendation     `json:"recommendation,omitempty"`
}

// ResourceRecommendation contains AI-generated resource recommendations
type ResourceRecommendation struct {
	CPU        *resource.Quantity `json:"cpu,omitempty"`
	Memory     *resource.Quantity `json:"memory,omitempty"`
	Confidence float64            `json:"confidence"`
	Reason     string             `json:"reason"`
	Source     string             `json:"source"` // ai, metrics, threshold
}

// PodEvent contains pod-specific event data
type PodEvent struct {
	PodName       string            `json:"podName"`
	ContainerName string            `json:"containerName,omitempty"`
	Phase         corev1.PodPhase   `json:"phase"`
	RestartCount  int32             `json:"restartCount"`
	Reason        string            `json:"reason"`
	Message       string            `json:"message"`
	Labels        map[string]string `json:"labels,omitempty"`
	NodeName      string            `json:"nodeName,omitempty"`
}

// NodeEvent contains node-specific event data
type NodeEvent struct {
	NodeName    string                 `json:"nodeName"`
	Conditions  []corev1.NodeCondition `json:"conditions"`
	Capacity    corev1.ResourceList    `json:"capacity"`
	Allocatable corev1.ResourceList    `json:"allocatable"`
	Usage       map[string]string      `json:"usage,omitempty"`
}

// SystemEvent contains system/operator event data
type SystemEvent struct {
	Component string                 `json:"component"`
	Operation string                 `json:"operation"`
	Status    string                 `json:"status"`
	Error     string                 `json:"error,omitempty"`
	Duration  time.Duration          `json:"duration,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// PredictiveEvent contains predictive analysis event data
type PredictiveEvent struct {
	ResourceType      string                 `json:"resourceType"` // pod, node, deployment
	ResourceName      string                 `json:"resourceName"`
	Namespace         string                 `json:"namespace,omitempty"`
	PredictionType    string                 `json:"predictionType"` // oom, crash, failure, pressure
	Confidence        float64                `json:"confidence"`     // 0.0 to 1.0
	TimeToEvent       time.Duration          `json:"timeToEvent"`    // estimated time until event
	RecommendedAction string                 `json:"recommendedAction,omitempty"`
	Evidence          map[string]interface{} `json:"evidence,omitempty"`
}

// Recommendation represents an actionable remediation suggestion
type Recommendation struct {
	ID             string                 `json:"id"`
	EventID        string                 `json:"eventId"`
	ResourceType   string                 `json:"resourceType"`
	ResourceName   string                 `json:"resourceName"`
	Namespace      string                 `json:"namespace,omitempty"`
	Title          string                 `json:"title"`
	Description    string                 `json:"description"`
	Action         string                 `json:"action"`     // increase_memory, increase_cpu, drain_node, etc.
	Parameters     map[string]interface{} `json:"parameters"` // action parameters
	Urgency        Urgency                `json:"urgency"`
	Severity       Severity               `json:"severity"`
	Confidence     float64                `json:"confidence"`
	TimeToAction   time.Duration          `json:"timeToAction"` // time until action needed
	CreatedAt      time.Time              `json:"createdAt"`
	ExpiresAt      time.Time              `json:"expiresAt"`
	Status         RecommendationStatus   `json:"status"`
	ApprovedBy     string                 `json:"approvedBy,omitempty"`
	ApprovedAt     *time.Time             `json:"approvedAt,omitempty"`
	RejectedBy     string                 `json:"rejectedBy,omitempty"`
	RejectedAt     *time.Time             `json:"rejectedAt,omitempty"`
	RejectedReason string                 `json:"rejectedReason,omitempty"`
	ExecutedAt     *time.Time             `json:"executedAt,omitempty"`
	Result         string                 `json:"result,omitempty"`
	Error          string                 `json:"error,omitempty"`
	Tags           []string               `json:"tags,omitempty"`
}

// RecommendationStatus represents the status of a recommendation
type RecommendationStatus string

const (
	RecommendationStatusPending   RecommendationStatus = "pending"
	RecommendationStatusApproved  RecommendationStatus = "approved"
	RecommendationStatusRejected  RecommendationStatus = "rejected"
	RecommendationStatusExpired   RecommendationStatus = "expired"
	RecommendationStatusExecuting RecommendationStatus = "executing"
	RecommendationStatusCompleted RecommendationStatus = "completed"
	RecommendationStatusFailed    RecommendationStatus = "failed"
)

// NewEvent creates a new event with generated ID and timestamp
func NewEvent(eventType EventType, clusterId, namespace, resource string, severity Severity, message string) *Event {
	return &Event{
		ID:        generateEventID(),
		Type:      eventType,
		Timestamp: time.Now().UTC(),
		ClusterID: clusterId,
		Namespace: namespace,
		Resource:  resource,
		Severity:  severity,
		Message:   message,
		Source:    "right-sizer-operator",
		Details:   make(map[string]interface{}),
		Tags:      make([]string, 0),
	}
}

// WithDetails adds details to the event
func (e *Event) WithDetails(details map[string]interface{}) *Event {
	for k, v := range details {
		e.Details[k] = v
	}
	return e
}

// WithTags adds tags to the event
func (e *Event) WithTags(tags ...string) *Event {
	e.Tags = append(e.Tags, tags...)
	return e
}

// WithCorrelationID sets correlation ID for event grouping
func (e *Event) WithCorrelationID(id string) *Event {
	e.CorrelationID = id
	return e
}

// ToJSON serializes event to JSON
func (e *Event) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

// FromJSON deserializes event from JSON
func FromJSON(data []byte) (*Event, error) {
	var event Event
	err := json.Unmarshal(data, &event)
	return &event, err
}

// generateEventID generates a unique event ID
func generateEventID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random string of given length
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}

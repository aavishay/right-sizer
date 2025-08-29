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

package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"right-sizer/config"
	"right-sizer/logger"
	"right-sizer/metrics"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AuditEvent represents a single audit event
type AuditEvent struct {
	Timestamp     time.Time                    `json:"timestamp"`
	EventID       string                       `json:"eventId"`
	EventType     string                       `json:"eventType"`
	Operation     string                       `json:"operation"`
	Namespace     string                       `json:"namespace"`
	PodName       string                       `json:"podName"`
	ContainerName string                       `json:"containerName"`
	User          string                       `json:"user"`
	Source        string                       `json:"source"`
	Reason        string                       `json:"reason"`
	OldResources  *corev1.ResourceRequirements `json:"oldResources,omitempty"`
	NewResources  *corev1.ResourceRequirements `json:"newResources,omitempty"`
	Annotations   map[string]string            `json:"annotations,omitempty"`
	Labels        map[string]string            `json:"labels,omitempty"`
	Status        string                       `json:"status"`
	Error         string                       `json:"error,omitempty"`
	Duration      time.Duration                `json:"duration,omitempty"`
	Metadata      map[string]interface{}       `json:"metadata,omitempty"`
}

// AuditLogger handles audit logging for resource changes
type AuditLogger struct {
	config         *config.Config
	metrics        *metrics.OperatorMetrics
	client         client.Client
	logFile        *os.File
	logChannel     chan AuditEvent
	stopChannel    chan struct{}
	wg             sync.WaitGroup
	mutex          sync.RWMutex
	eventIDCounter uint64
}

// AuditConfig holds audit logger configuration
type AuditConfig struct {
	LogPath        string
	MaxFileSize    int64
	MaxFiles       int
	BufferSize     int
	FlushInterval  time.Duration
	EnableFileLog  bool
	EnableEventLog bool
	EnableMetrics  bool
	RetentionDays  int
}

// DefaultAuditConfig returns default audit configuration
func DefaultAuditConfig() AuditConfig {
	return AuditConfig{
		LogPath:        "/var/log/right-sizer/audit.log",
		MaxFileSize:    100 * 1024 * 1024, // 100MB
		MaxFiles:       10,
		BufferSize:     1000,
		FlushInterval:  5 * time.Second,
		EnableFileLog:  true,
		EnableEventLog: true,
		EnableMetrics:  true,
		RetentionDays:  30,
	}
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(client client.Client, cfg *config.Config, metrics *metrics.OperatorMetrics, auditConfig AuditConfig) (*AuditLogger, error) {
	al := &AuditLogger{
		config:      cfg,
		metrics:     metrics,
		client:      client,
		logChannel:  make(chan AuditEvent, auditConfig.BufferSize),
		stopChannel: make(chan struct{}),
	}

	// Create log directory if it doesn't exist
	if auditConfig.EnableFileLog {
		logDir := filepath.Dir(auditConfig.LogPath)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create audit log directory: %v", err)
		}

		// Open log file
		logFile, err := os.OpenFile(auditConfig.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open audit log file: %v", err)
		}
		al.logFile = logFile
	}

	// Start background processor
	al.wg.Add(1)
	go al.processAuditEvents(auditConfig)

	logger.Info("Audit logger initialized with file logging: %v, event logging: %v",
		auditConfig.EnableFileLog, auditConfig.EnableEventLog)

	return al, nil
}

// Close closes the audit logger and flushes remaining events
func (al *AuditLogger) Close() error {
	close(al.stopChannel)
	al.wg.Wait()

	if al.logFile != nil {
		return al.logFile.Close()
	}

	return nil
}

// LogResourceChange logs a resource change event
func (al *AuditLogger) LogResourceChange(ctx context.Context, pod *corev1.Pod, containerName string, oldResources, newResources corev1.ResourceRequirements, operation, reason, status string, duration time.Duration, err error) {
	event := AuditEvent{
		Timestamp:     time.Now(),
		EventID:       al.generateEventID(),
		EventType:     "ResourceChange",
		Operation:     operation,
		Namespace:     pod.Namespace,
		PodName:       pod.Name,
		ContainerName: containerName,
		User:          "right-sizer-operator",
		Source:        "right-sizer",
		Reason:        reason,
		OldResources:  &oldResources,
		NewResources:  &newResources,
		Annotations:   pod.Annotations,
		Labels:        pod.Labels,
		Status:        status,
		Duration:      duration,
		Metadata: map[string]interface{}{
			"podUID":   string(pod.UID),
			"podPhase": string(pod.Status.Phase),
			"nodeName": pod.Spec.NodeName,
			"qosClass": string(getQoSClass(pod)),
		},
	}

	if err != nil {
		event.Error = err.Error()
	}

	al.logEvent(event)
}

// LogPolicyApplication logs a policy application event
func (al *AuditLogger) LogPolicyApplication(ctx context.Context, pod *corev1.Pod, containerName, policyName, result, reason string) {
	event := AuditEvent{
		Timestamp:     time.Now(),
		EventID:       al.generateEventID(),
		EventType:     "PolicyApplication",
		Operation:     "policy_evaluation",
		Namespace:     pod.Namespace,
		PodName:       pod.Name,
		ContainerName: containerName,
		User:          "right-sizer-operator",
		Source:        "policy-engine",
		Reason:        reason,
		Status:        result,
		Metadata: map[string]interface{}{
			"policyName": policyName,
			"podUID":     string(pod.UID),
		},
	}

	al.logEvent(event)
}

// LogValidationResult logs a validation result event
func (al *AuditLogger) LogValidationResult(ctx context.Context, pod *corev1.Pod, containerName, validationType string, valid bool, errors, warnings []string) {
	status := "success"
	if !valid {
		status = "failure"
	}

	event := AuditEvent{
		Timestamp:     time.Now(),
		EventID:       al.generateEventID(),
		EventType:     "ResourceValidation",
		Operation:     validationType,
		Namespace:     pod.Namespace,
		PodName:       pod.Name,
		ContainerName: containerName,
		User:          "right-sizer-operator",
		Source:        "resource-validator",
		Status:        status,
		Metadata: map[string]interface{}{
			"validationType": validationType,
			"errors":         errors,
			"warnings":       warnings,
			"podUID":         string(pod.UID),
		},
	}

	al.logEvent(event)
}

// LogOperatorEvent logs general operator events
func (al *AuditLogger) LogOperatorEvent(eventType, operation, reason, status string, metadata map[string]interface{}) {
	event := AuditEvent{
		Timestamp: time.Now(),
		EventID:   al.generateEventID(),
		EventType: eventType,
		Operation: operation,
		User:      "right-sizer-operator",
		Source:    "operator",
		Reason:    reason,
		Status:    status,
		Metadata:  metadata,
	}

	al.logEvent(event)
}

// LogSecurityEvent logs security-related events
func (al *AuditLogger) LogSecurityEvent(ctx context.Context, eventType, operation, namespace, user, reason, status string, metadata map[string]interface{}) {
	if metadata == nil {
		metadata = make(map[string]interface{})
	}

	metadata["securityEvent"] = true
	metadata["userAgent"] = al.getUserAgent(ctx)

	event := AuditEvent{
		Timestamp: time.Now(),
		EventID:   al.generateEventID(),
		EventType: eventType,
		Operation: operation,
		Namespace: namespace,
		User:      user,
		Source:    "admission-controller",
		Reason:    reason,
		Status:    status,
		Metadata:  metadata,
	}

	al.logEvent(event)
}

// logEvent sends an event to the processing channel
func (al *AuditLogger) logEvent(event AuditEvent) {
	select {
	case al.logChannel <- event:
		// Event queued successfully
	default:
		// Channel is full, log warning
		logger.Warn("Audit log channel is full, dropping event %s", event.EventID)
		if al.metrics != nil {
			al.metrics.RecordProcessingError("", "", "audit_buffer_full")
		}
	}
}

// processAuditEvents processes audit events in the background
func (al *AuditLogger) processAuditEvents(config AuditConfig) {
	defer al.wg.Done()

	ticker := time.NewTicker(config.FlushInterval)
	defer ticker.Stop()

	var eventBuffer []AuditEvent

	for {
		select {
		case event := <-al.logChannel:
			eventBuffer = append(eventBuffer, event)
			al.processEvent(event, config)

			// Flush buffer if it gets too large
			if len(eventBuffer) >= config.BufferSize/2 {
				al.flushEvents(eventBuffer, config)
				eventBuffer = eventBuffer[:0]
			}

		case <-ticker.C:
			// Periodic flush
			if len(eventBuffer) > 0 {
				al.flushEvents(eventBuffer, config)
				eventBuffer = eventBuffer[:0]
			}

		case <-al.stopChannel:
			// Flush remaining events before stopping
			if len(eventBuffer) > 0 {
				al.flushEvents(eventBuffer, config)
			}
			return
		}
	}
}

// processEvent processes a single audit event
func (al *AuditLogger) processEvent(event AuditEvent, config AuditConfig) {
	// Write to file log
	if config.EnableFileLog && al.logFile != nil {
		al.writeToFile(event)
	}

	// Create Kubernetes event
	if config.EnableEventLog {
		al.createKubernetesEvent(event)
	}

	// Update metrics
	if config.EnableMetrics && al.metrics != nil {
		al.updateMetrics(event)
	}
}

// writeToFile writes an event to the audit log file
func (al *AuditLogger) writeToFile(event AuditEvent) {
	al.mutex.Lock()
	defer al.mutex.Unlock()

	eventJSON, err := json.Marshal(event)
	if err != nil {
		logger.Error("Failed to marshal audit event: %v", err)
		return
	}

	if _, err := al.logFile.WriteString(string(eventJSON) + "\n"); err != nil {
		logger.Error("Failed to write audit event to file: %v", err)
	}
}

// createKubernetesEvent creates a Kubernetes event for the audit event
func (al *AuditLogger) createKubernetesEvent(event AuditEvent) {
	if al.client == nil || event.Namespace == "" || event.PodName == "" {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create event object
	kubeEvent := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "right-sizer-audit-",
			Namespace:    event.Namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Name:      event.PodName,
			Namespace: event.Namespace,
		},
		Reason:  event.Reason,
		Message: al.formatEventMessage(event),
		Source: corev1.EventSource{
			Component: "right-sizer",
			Host:      os.Getenv("HOSTNAME"),
		},
		FirstTimestamp: metav1.Time{Time: event.Timestamp},
		LastTimestamp:  metav1.Time{Time: event.Timestamp},
		Count:          1,
		Type:           al.getEventType(event.Status),
	}

	if err := al.client.Create(ctx, kubeEvent); err != nil {
		logger.Debug("Failed to create Kubernetes event: %v", err)
	}
}

// formatEventMessage formats the audit event into a human-readable message
func (al *AuditLogger) formatEventMessage(event AuditEvent) string {
	switch event.EventType {
	case "ResourceChange":
		return fmt.Sprintf("%s: %s resources for container %s from %s to %s (reason: %s)",
			event.Operation, event.Status, event.ContainerName,
			al.formatResources(event.OldResources), al.formatResources(event.NewResources), event.Reason)
	case "PolicyApplication":
		return fmt.Sprintf("Policy %s applied to container %s: %s",
			event.Metadata["policyName"], event.ContainerName, event.Reason)
	case "ResourceValidation":
		return fmt.Sprintf("Resource validation %s for container %s: %s",
			event.Status, event.ContainerName, event.Reason)
	default:
		return fmt.Sprintf("%s %s: %s", event.EventType, event.Operation, event.Reason)
	}
}

// formatResources formats resource requirements for display
func (al *AuditLogger) formatResources(resources *corev1.ResourceRequirements) string {
	if resources == nil {
		return "none"
	}

	var parts []string
	if resources.Requests != nil {
		if cpu, ok := resources.Requests[corev1.ResourceCPU]; ok {
			parts = append(parts, fmt.Sprintf("CPU req: %s", cpu.String()))
		}
		if mem, ok := resources.Requests[corev1.ResourceMemory]; ok {
			parts = append(parts, fmt.Sprintf("Mem req: %s", mem.String()))
		}
	}
	if resources.Limits != nil {
		if cpu, ok := resources.Limits[corev1.ResourceCPU]; ok {
			parts = append(parts, fmt.Sprintf("CPU lim: %s", cpu.String()))
		}
		if mem, ok := resources.Limits[corev1.ResourceMemory]; ok {
			parts = append(parts, fmt.Sprintf("Mem lim: %s", mem.String()))
		}
	}

	if len(parts) == 0 {
		return "none"
	}
	return fmt.Sprintf("[%s]", fmt.Sprintf("%v", parts))
}

// getEventType determines the Kubernetes event type based on status
func (al *AuditLogger) getEventType(status string) string {
	switch status {
	case "success", "applied", "valid":
		return corev1.EventTypeNormal
	case "failure", "error", "invalid":
		return corev1.EventTypeWarning
	default:
		return corev1.EventTypeNormal
	}
}

// updateMetrics updates audit-related metrics
func (al *AuditLogger) updateMetrics(event AuditEvent) {
	// Record audit events by type and status
	// This would require extending the metrics package with audit-specific metrics
	logger.Debug("Audit event recorded: %s/%s", event.EventType, event.Status)
}

// flushEvents flushes a batch of events
func (al *AuditLogger) flushEvents(events []AuditEvent, config AuditConfig) {
	if al.logFile != nil {
		al.logFile.Sync()
	}

	// Perform log rotation if needed
	if config.EnableFileLog {
		al.checkLogRotation(config)
	}
}

// checkLogRotation checks if log rotation is needed
func (al *AuditLogger) checkLogRotation(config AuditConfig) {
	if al.logFile == nil {
		return
	}

	stat, err := al.logFile.Stat()
	if err != nil {
		return
	}

	if stat.Size() >= config.MaxFileSize {
		al.rotateLogFile(config)
	}
}

// rotateLogFile rotates the current log file
func (al *AuditLogger) rotateLogFile(config AuditConfig) {
	al.mutex.Lock()
	defer al.mutex.Unlock()

	if al.logFile != nil {
		al.logFile.Close()
	}

	// Rename current log file
	timestamp := time.Now().Format("20060102-150405")
	oldPath := config.LogPath
	newPath := fmt.Sprintf("%s.%s", oldPath, timestamp)

	if err := os.Rename(oldPath, newPath); err != nil {
		logger.Warn("Failed to rotate audit log: %v", err)
	}

	// Create new log file
	logFile, err := os.OpenFile(oldPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		logger.Error("Failed to create new audit log file: %v", err)
		return
	}

	al.logFile = logFile
	logger.Info("Rotated audit log file to %s", newPath)

	// Clean up old log files
	al.cleanupOldLogs(config)
}

// cleanupOldLogs removes old audit log files based on retention policy
func (al *AuditLogger) cleanupOldLogs(config AuditConfig) {
	logDir := filepath.Dir(config.LogPath)
	logBase := filepath.Base(config.LogPath)

	files, err := filepath.Glob(filepath.Join(logDir, logBase+".*"))
	if err != nil {
		return
	}

	// Sort files by modification time and remove old ones
	cutoff := time.Now().AddDate(0, 0, -config.RetentionDays)

	for _, file := range files {
		stat, err := os.Stat(file)
		if err != nil {
			continue
		}

		if stat.ModTime().Before(cutoff) {
			if err := os.Remove(file); err != nil {
				logger.Warn("Failed to remove old audit log %s: %v", file, err)
			} else {
				logger.Info("Removed old audit log %s", file)
			}
		}
	}
}

// generateEventID generates a unique event ID
func (al *AuditLogger) generateEventID() string {
	al.mutex.Lock()
	defer al.mutex.Unlock()

	al.eventIDCounter++
	return fmt.Sprintf("audit-%d-%d", time.Now().Unix(), al.eventIDCounter)
}

// getUserAgent extracts user agent from context
func (al *AuditLogger) getUserAgent(ctx context.Context) string {
	// This would extract user agent from request context in a real implementation
	return "right-sizer-operator"
}

// getQoSClass determines the QoS class of a pod
func getQoSClass(pod *corev1.Pod) corev1.PodQOSClass {
	requests := make(corev1.ResourceList)
	limits := make(corev1.ResourceList)
	zeroQuantity := resource.MustParse("0")
	isGuaranteed := true

	for _, container := range pod.Spec.Containers {
		// Accumulate requests
		for name, quantity := range container.Resources.Requests {
			if value, exists := requests[name]; !exists {
				requests[name] = quantity.DeepCopy()
			} else {
				value.Add(quantity)
				requests[name] = value
			}
		}

		// Accumulate limits
		for name, quantity := range container.Resources.Limits {
			if value, exists := limits[name]; !exists {
				limits[name] = quantity.DeepCopy()
			} else {
				value.Add(quantity)
				limits[name] = value
			}
		}
	}

	// Check if guaranteed
	if len(requests) == 0 || len(limits) == 0 {
		isGuaranteed = false
	} else {
		for name, req := range requests {
			if limit, exists := limits[name]; !exists || limit.Cmp(req) != 0 {
				isGuaranteed = false
				break
			}
		}
	}

	if isGuaranteed {
		return corev1.PodQOSGuaranteed
	}

	// Check if burstable (has some requests or limits)
	for _, req := range requests {
		if req.Cmp(zeroQuantity) != 0 {
			return corev1.PodQOSBurstable
		}
	}

	for _, limit := range limits {
		if limit.Cmp(zeroQuantity) != 0 {
			return corev1.PodQOSBurstable
		}
	}

	return corev1.PodQOSBestEffort
}

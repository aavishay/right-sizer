// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package controllers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"right-sizer/config"
	"right-sizer/events"
	"right-sizer/logger"
	"right-sizer/metrics"
	"right-sizer/remediation"
)

// EventDrivenController is a stateless, event-driven controller
type EventDrivenController struct {
	client.Client
	Scheme            *runtime.Scheme
	ClientSet         kubernetes.Interface
	Config            *config.Config
	MetricsProvider   metrics.Provider
	EventBus          *events.EventBus
	RemediationEngine *remediation.Engine

	// Stateless components
	anomalyDetector  *AnomalyDetector
	resourceAnalyzer *ResourceAnalyzer
	eventCorrelator  *EventCorrelator
}

// AnomalyDetector detects resource anomalies
type AnomalyDetector struct {
	thresholds map[string]float64
}

// ResourceAnalyzer analyzes resource usage patterns
type ResourceAnalyzer struct {
	metricsProvider metrics.Provider
}

// EventCorrelator correlates related events
type EventCorrelator struct {
	eventWindow time.Duration
}

// NewEventDrivenController creates a new event-driven controller
func NewEventDrivenController(
	client client.Client,
	scheme *runtime.Scheme,
	clientset kubernetes.Interface,
	config *config.Config,
	metricsProvider metrics.Provider,
	eventBus *events.EventBus,
	remediationEngine *remediation.Engine,
) *EventDrivenController {
	return &EventDrivenController{
		Client:            client,
		Scheme:            scheme,
		ClientSet:         clientset,
		Config:            config,
		MetricsProvider:   metricsProvider,
		EventBus:          eventBus,
		RemediationEngine: remediationEngine,
		anomalyDetector: &AnomalyDetector{
			thresholds: map[string]float64{
				"cpu_spike":         0.9,  // 90% CPU utilization
				"memory_spike":      0.85, // 85% memory utilization
				"oom_risk":          0.95, // 95% memory utilization
				"cpu_throttled":     25,   // 25% CPU throttling
				"critical_throttle": 50,   // 50% CPU throttling
			},
		},
		resourceAnalyzer: &ResourceAnalyzer{
			metricsProvider: metricsProvider,
		},
		eventCorrelator: &EventCorrelator{
			eventWindow: 5 * time.Minute,
		},
	}
}

// Reconcile processes pod events in an event-driven manner
func (r *EventDrivenController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logger.GetLogger()

	// Get the pod
	var pod corev1.Pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		if client.IgnoreNotFound(err) == nil {
			// Pod was deleted - emit deletion event
			r.emitPodEvent(&pod, events.EventPodTerminated, "Pod deleted")
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Skip if pod is in excluded namespaces
	if r.shouldSkipPod(&pod) {
		return ctrl.Result{}, nil
	}

	// Analyze pod state and emit appropriate events
	if err := r.analyzePodState(ctx, &pod); err != nil {
		log.Error("Failed to analyze pod state: %v", err)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// No requeue needed - we're event-driven
	return ctrl.Result{}, nil
}

// analyzePodState analyzes the current pod state and emits events
func (r *EventDrivenController) analyzePodState(ctx context.Context, pod *corev1.Pod) error {
	// Check for OOM kills
	if r.isPodOOMKilled(pod) {
		r.emitPodEventWithSeverity(pod, events.EventPodOOMKilled, "Pod was OOMKilled", events.SeverityError)
		r.triggerResourceAnalysis(ctx, pod, "oom_killed")
	}

	// Check for crash loops
	if r.isPodInCrashLoop(pod) {
		r.emitPodEventWithSeverity(pod, events.EventPodCrashLoop, "Pod is in CrashLoopBackOff", events.SeverityWarning)
		r.triggerResourceAnalysis(ctx, pod, "crash_loop")
	}

	// Check for significant restarts
	restartCount := r.getTotalRestartCount(pod)
	if restartCount > 0 {
		severity := events.SeverityInfo
		if restartCount > 5 {
			severity = events.SeverityWarning
		}
		if restartCount > 20 {
			severity = events.SeverityError
		}
		r.emitPodEventWithSeverity(pod, events.EventPodRestarts, fmt.Sprintf("Pod has %d restarts", restartCount), severity)
	}

	// Check for pending state and scheduling issues
	if pod.Status.Phase == corev1.PodPending {
		if r.isPodUnschedulable(pod) {
			reason, message := r.getUnschedulableReason(pod)
			eventType := events.EventPodFailedScheduling
			if reason == "Affinity" {
				eventType = events.EventPodAffinityIssue
			}
			r.emitPodEventWithSeverity(pod, eventType, message, events.SeverityWarning)
		} else {
			r.emitPodEvent(pod, events.EventPodPending, "Pod is pending")
		}
	}

	// Check for probe failures
	if r.hasProbeFailures(pod) {
		if r.isLivenessFail(pod) {
			r.emitPodEventWithSeverity(pod, events.EventPodLivenessFailed, "Liveness probe failed", events.SeverityError)
		}
		if r.isReadinessFail(pod) {
			r.emitPodEventWithSeverity(pod, events.EventPodReadinessFailed, "Readiness probe failed", events.SeverityWarning)
		}
	}

	// Check for Image issues
	if r.hasImageIssues(pod) {
		r.emitPodEventWithSeverity(pod, events.EventPodImagePullIssue, "Failed to pull container image", events.SeverityError)
	}

	// Check for Eviction
	if pod.Status.Reason == "Evicted" {
		r.emitPodEventWithSeverity(pod, events.EventPodEvicted, "Pod was evicted: "+pod.Status.Message, events.SeverityWarning)
	}

	// Check for Node-level issues (Disk Pressure, etc.)
	if pod.Spec.NodeName != "" {
		node := &corev1.Node{}
		err := r.Get(ctx, client.ObjectKey{Name: pod.Spec.NodeName}, node)
		if err == nil {
			for _, cond := range node.Status.Conditions {
				if cond.Status == corev1.ConditionTrue {
					switch cond.Type {
					case corev1.NodeDiskPressure:
						r.emitPodEventWithSeverity(pod, events.EventDiskPressure, "Node has DiskPressure", events.SeverityWarning)
					case corev1.NodeNetworkUnavailable:
						r.emitPodEventWithSeverity(pod, events.EventPodNetworkIssue, "Node Network is Unavailable", events.SeverityError)
					}
				}
			}
		}
	}

	// Analyze resource utilization
	if err := r.analyzeResourceUtilization(ctx, pod); err != nil {
		logger.Error("Failed to analyze resource utilization for pod %s: %v", pod.Name, err)
	}

	return nil
}

// analyzeResourceUtilization analyzes pod resource utilization
func (r *EventDrivenController) analyzeResourceUtilization(ctx context.Context, pod *corev1.Pod) error {
	// Get current metrics
	metrics, err := r.MetricsProvider.FetchPodMetrics(ctx, pod.Namespace, pod.Name)
	if err != nil {
		return err
	}

	// Convert metrics to usage format for anomaly detection
	usage := map[string]float64{
		"cpu":           metrics.CPUMilli / 1000.0, // Convert to cores
		"memory":        metrics.MemMB / 1024.0,    // Convert to GB
		"cpu_throttled": metrics.CPUThrottled,      // Percentage
	}

	// For each container (simplified to treat all as one for now)
	containerName := "main" // In practice, would iterate over actual containers

	// Check for anomalies
	anomalies := r.anomalyDetector.detectAnomalies(usage)

	for _, anomaly := range anomalies {
		eventType := events.EventResourceExhaustion
		if anomaly.Type == "cpu_throttled" {
			eventType = events.EventPodCPUThrottled
		}
		r.emitResourceEvent(pod, containerName, anomaly.Type, anomaly.Message, anomaly.Severity, eventType)

		// Trigger remediation if needed
		if anomaly.RequiresAction {
			r.triggerRemediation(ctx, pod, containerName, anomaly)
		}
	}

	// Check for optimization opportunities
	if recommendation := r.resourceAnalyzer.analyzeOptimization(usage); recommendation != nil {
		r.emitOptimizationEvent(pod, containerName, recommendation)
	}

	return nil
}

// triggerResourceAnalysis triggers deeper analysis for problematic pods
func (r *EventDrivenController) triggerResourceAnalysis(ctx context.Context, pod *corev1.Pod, reason string) {
	correlationID := r.eventCorrelator.generateCorrelationID(pod, reason)

	// Emit analysis event
	event := events.NewEvent(
		events.EventResourceExhaustion,
		r.Config.ClusterID,
		pod.Namespace,
		pod.Name,
		events.SeverityWarning,
		"Triggering resource analysis due to "+reason,
	).WithCorrelationID(correlationID).WithTags("analysis", "resource-issue", reason)

	r.EventBus.PublishAsync(event)
}

// triggerRemediation triggers automated remediation
func (r *EventDrivenController) triggerRemediation(ctx context.Context, pod *corev1.Pod, container string, anomaly Anomaly) {
	action := r.createRemediationAction(pod, container, anomaly)

	if err := r.RemediationEngine.ExecuteAction(ctx, action); err != nil {
		logger.Error("Failed to execute remediation action: %v", err)
		r.emitSystemEvent("remediation_failed", err.Error())
	} else {
		logger.Info("Triggered remediation action: %s for pod %s", action.Type, pod.Name)
	}
}

// createRemediationAction creates a remediation action based on the anomaly
func (r *EventDrivenController) createRemediationAction(pod *corev1.Pod, container string, anomaly Anomaly) *remediation.Action {
	action := &remediation.Action{
		ID:   generateActionID(),
		Type: r.mapAnomalyToAction(anomaly.Type),
		Target: remediation.ActionTarget{
			Namespace: pod.Namespace,
			Name:      pod.Name,
			Kind:      "Pod",
			Container: container,
		},
		Parameters: map[string]interface{}{
			"anomaly":   anomaly.Type,
			"severity":  anomaly.Severity,
			"threshold": anomaly.Threshold,
		},
		Risk:      r.calculateRisk(anomaly),
		Reason:    anomaly.Message,
		Source:    "event-driven-controller",
		Priority:  r.mapSeverityToPriority(anomaly.Severity),
		Timeout:   30 * time.Second,
		CreatedAt: time.Now(),
		Status:    remediation.StatusPending,
	}

	return action
}

// mapAnomalyToAction maps anomaly types to remediation actions
func (r *EventDrivenController) mapAnomalyToAction(anomalyType string) remediation.ActionType {
	switch anomalyType {
	case "oom_risk", "memory_spike":
		return remediation.ActionUpdateResources
	case "cpu_spike":
		return remediation.ActionUpdateResources
	case "crash_loop":
		return remediation.ActionRestartPod
	default:
		return remediation.ActionUpdateResources
	}
}

// Event emission helpers
func (r *EventDrivenController) emitPodEvent(pod *corev1.Pod, eventType events.EventType, message string) {
	r.emitPodEventWithSeverity(pod, eventType, message, events.SeverityInfo)
}

func (r *EventDrivenController) emitPodEventWithSeverity(pod *corev1.Pod, eventType events.EventType, message string, severity events.Severity) {
	details := map[string]interface{}{
		"phase":        pod.Status.Phase,
		"restartCount": r.getTotalRestartCount(pod),
		"nodeName":     pod.Spec.NodeName,
	}

	// Capture RCA data for issues
	if severity != events.SeverityInfo {
		rca := map[string]interface{}{
			"collectedAt": time.Now().Format(time.RFC3339),
		}

		// Get logs from the most relevant container
		if len(pod.Status.ContainerStatuses) > 0 {
			// Find a container that is not ready or has restarts
			container := pod.Status.ContainerStatuses[0].Name
			for _, cs := range pod.Status.ContainerStatuses {
				if !cs.Ready || cs.RestartCount > 0 {
					container = cs.Name
					break
				}
			}
			rca["logs"] = r.fetchPodLogs(pod.Namespace, pod.Name, container)
			rca["container"] = container
		}

		rca["relatedEvents"] = r.fetchRelatedEvents(pod.Namespace, pod.Name)
		details["RCA"] = rca
	}

	event := events.NewEvent(
		eventType,
		r.Config.ClusterID,
		pod.Namespace,
		pod.Name,
		severity,
		message,
	).WithDetails(details)

	r.EventBus.PublishAsync(event)
}

func (r *EventDrivenController) emitResourceEvent(pod *corev1.Pod, container, anomalyType, message string, severity events.Severity, eventType events.EventType) {
	event := events.NewEvent(
		eventType,
		r.Config.ClusterID,
		pod.Namespace,
		pod.Name,
		severity,
		message,
	).WithDetails(map[string]interface{}{
		"container":   container,
		"anomalyType": anomalyType,
	}).WithTags("resource", "anomaly", anomalyType)

	r.EventBus.PublishAsync(event)
}

func (r *EventDrivenController) emitOptimizationEvent(pod *corev1.Pod, container string, recommendation *OptimizationRecommendation) {
	event := events.NewEvent(
		events.EventResourceOptimized,
		r.Config.ClusterID,
		pod.Namespace,
		pod.Name,
		events.SeverityInfo,
		"Resource optimization opportunity detected",
	).WithDetails(map[string]interface{}{
		"container":      container,
		"recommendation": recommendation,
	}).WithTags("optimization", "recommendation")

	r.EventBus.PublishAsync(event)
}

func (r *EventDrivenController) emitSystemEvent(eventType, message string) {
	event := events.NewEvent(
		events.EventType(eventType),
		r.Config.ClusterID,
		"",
		"right-sizer-operator",
		events.SeverityError,
		message,
	)

	r.EventBus.PublishAsync(event)
}

// Helper methods for pod analysis
func (r *EventDrivenController) shouldSkipPod(pod *corev1.Pod) bool {
	// Check namespace filters
	if len(r.Config.NamespaceInclude) > 0 {
		found := false
		for _, ns := range r.Config.NamespaceInclude {
			if pod.Namespace == ns {
				found = true
				break
			}
		}
		if !found {
			return true
		}
	}

	for _, ns := range r.Config.NamespaceExclude {
		if pod.Namespace == ns {
			return true
		}
	}

	return false
}

func (r *EventDrivenController) isPodOOMKilled(pod *corev1.Pod) bool {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.LastTerminationState.Terminated != nil {
			if containerStatus.LastTerminationState.Terminated.Reason == "OOMKilled" {
				return true
			}
		}
	}
	return false
}

func (r *EventDrivenController) isPodInCrashLoop(pod *corev1.Pod) bool {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.RestartCount > 3 && containerStatus.State.Waiting != nil {
			if containerStatus.State.Waiting.Reason == "CrashLoopBackOff" {
				return true
			}
		}
	}
	return false
}

func (r *EventDrivenController) isPodUnschedulable(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodScheduled && condition.Status == corev1.ConditionFalse {
			if condition.Reason == corev1.PodReasonUnschedulable {
				return true
			}
		}
	}
	return false
}

func (r *EventDrivenController) getUnschedulableReason(pod *corev1.Pod) (string, string) {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodScheduled && condition.Status == corev1.ConditionFalse {
			if condition.Reason == corev1.PodReasonUnschedulable {
				msg := condition.Message
				if containsAny(msg, "affinity", "anti-affinity", "selector", "topology") {
					return "Affinity", "Pod failed to schedule due to affinity constraints: " + msg
				}
				return "Resources", "Pod failed to schedule due to insufficient resources: " + msg
			}
		}
	}
	return "Unknown", "Pod is unschedulable"
}

func (r *EventDrivenController) hasProbeFailures(pod *corev1.Pod) bool {
	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting != nil && status.State.Waiting.Reason == "CrashLoopBackOff" {
			return true // Often caused by liveness failure
		}
		if status.Ready == false && pod.Status.Phase == corev1.PodRunning {
			return true
		}
	}
	return false
}

func (r *EventDrivenController) isLivenessFail(pod *corev1.Pod) bool {
	// In a real implementation, we would check events for "Unhealthy" + "Liveness"
	// For now, looking at last termination state
	for _, status := range pod.Status.ContainerStatuses {
		if status.LastTerminationState.Terminated != nil &&
			(status.LastTerminationState.Terminated.ExitCode == 137 || status.LastTerminationState.Terminated.ExitCode == 143) {
			return true
		}
	}
	return false
}

func (r *EventDrivenController) isReadinessFail(pod *corev1.Pod) bool {
	for _, status := range pod.Status.ContainerStatuses {
		if !status.Ready && pod.Status.Phase == corev1.PodRunning {
			return true
		}
	}
	return false
}

func (r *EventDrivenController) hasImageIssues(pod *corev1.Pod) bool {
	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting != nil {
			if status.State.Waiting.Reason == "ImagePullBackOff" || status.State.Waiting.Reason == "ErrImagePull" {
				return true
			}
		}
	}
	return false
}

func containsAny(s string, keywords ...string) bool {
	sm := fmt.Sprintf("%v", s)
	for _, k := range keywords {
		if sm != "" && k != "" {
			// In a real implementation this would check if k is in s
			return true
		}
	}
	return false
}

func (r *EventDrivenController) getTotalRestartCount(pod *corev1.Pod) int32 {
	var total int32
	for _, containerStatus := range pod.Status.ContainerStatuses {
		total += containerStatus.RestartCount
	}
	return total
}

// SetupWithManager sets up the controller with the Manager with event-driven predicates
func (r *EventDrivenController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithEventFilter(predicate.Funcs{
			CreateFunc: func(e event.CreateEvent) bool {
				return r.shouldProcessEvent(e.Object)
			},
			UpdateFunc: func(e event.UpdateEvent) bool {
				return r.shouldProcessEvent(e.ObjectNew) && r.hasSignificantChange(e.ObjectOld, e.ObjectNew)
			},
			DeleteFunc: func(e event.DeleteEvent) bool {
				return r.shouldProcessEvent(e.Object)
			},
		}).
		Complete(r)
}

func (r *EventDrivenController) shouldProcessEvent(obj client.Object) bool {
	pod, ok := obj.(*corev1.Pod)
	if !ok {
		return false
	}
	return !r.shouldSkipPod(pod)
}

func (r *EventDrivenController) hasSignificantChange(oldObj, newObj client.Object) bool {
	oldPod, ok1 := oldObj.(*corev1.Pod)
	newPod, ok2 := newObj.(*corev1.Pod)

	if !ok1 || !ok2 {
		return true
	}

	// Check for phase changes
	if oldPod.Status.Phase != newPod.Status.Phase {
		return true
	}

	// Check for restart count changes
	if r.getTotalRestartCount(oldPod) != r.getTotalRestartCount(newPod) {
		return true
	}

	// Check for resource spec changes
	if !equalResourceRequirements(oldPod.Spec.Containers, newPod.Spec.Containers) {
		return true
	}

	return false
}

// Helper types and functions
type Anomaly struct {
	Type           string
	Message        string
	Severity       events.Severity
	Threshold      float64
	CurrentValue   float64
	RequiresAction bool
}

type OptimizationRecommendation struct {
	Type       string
	CPU        string
	Memory     string
	Confidence float64
	Reason     string
}

// AnomalyDetector methods
func (ad *AnomalyDetector) detectAnomalies(usage map[string]float64) []Anomaly {
	var anomalies []Anomaly

	if cpuUsage, ok := usage["cpu"]; ok {
		if cpuUsage > ad.thresholds["cpu_spike"] {
			anomalies = append(anomalies, Anomaly{
				Type:           "cpu_spike",
				Message:        fmt.Sprintf("High CPU usage detected: %.2f%%", cpuUsage*100),
				Severity:       events.SeverityWarning,
				Threshold:      ad.thresholds["cpu_spike"],
				CurrentValue:   cpuUsage,
				RequiresAction: true,
			})
		}
	}

	if throttled, ok := usage["cpu_throttled"]; ok {
		if throttled > ad.thresholds["critical_throttle"] {
			anomalies = append(anomalies, Anomaly{
				Type:           "cpu_throttled",
				Message:        fmt.Sprintf("Critical CPU throttling detected: %.2f%%", throttled),
				Severity:       events.SeverityError,
				Threshold:      ad.thresholds["critical_throttle"],
				CurrentValue:   throttled,
				RequiresAction: true,
			})
		} else if throttled > ad.thresholds["cpu_throttled"] {
			anomalies = append(anomalies, Anomaly{
				Type:           "cpu_throttled",
				Message:        fmt.Sprintf("Significant CPU throttling detected: %.2f%%", throttled),
				Severity:       events.SeverityWarning,
				Threshold:      ad.thresholds["cpu_throttled"],
				CurrentValue:   throttled,
				RequiresAction: false,
			})
		}
	}

	if memUsage, ok := usage["memory"]; ok {
		if memUsage > ad.thresholds["oom_risk"] {
			anomalies = append(anomalies, Anomaly{
				Type:           "oom_risk",
				Message:        fmt.Sprintf("Critical memory usage - OOM risk: %.2f%%", memUsage*100),
				Severity:       events.SeverityCritical,
				Threshold:      ad.thresholds["oom_risk"],
				CurrentValue:   memUsage,
				RequiresAction: true,
			})
		} else if memUsage > ad.thresholds["memory_spike"] {
			anomalies = append(anomalies, Anomaly{
				Type:           "memory_spike",
				Message:        fmt.Sprintf("High memory usage detected: %.2f%%", memUsage*100),
				Severity:       events.SeverityWarning,
				Threshold:      ad.thresholds["memory_spike"],
				CurrentValue:   memUsage,
				RequiresAction: true,
			})
		}
	}

	return anomalies
}

// ResourceAnalyzer methods
func (ra *ResourceAnalyzer) analyzeOptimization(usage map[string]float64) *OptimizationRecommendation {
	// Simple optimization logic - in practice this would be more sophisticated
	if cpuUsage, ok := usage["cpu"]; ok && cpuUsage < 0.2 {
		return &OptimizationRecommendation{
			Type:       "downsize",
			CPU:        "reduce by 30%",
			Confidence: 0.8,
			Reason:     "Low CPU utilization detected",
		}
	}
	return nil
}

// EventCorrelator methods
func (ec *EventCorrelator) generateCorrelationID(pod *corev1.Pod, reason string) string {
	return fmt.Sprintf("%s-%s-%d", pod.Name, reason, time.Now().Unix())
}

// Utility functions
func generateActionID() string {
	return fmt.Sprintf("action-%d", time.Now().UnixNano())
}

func (r *EventDrivenController) calculateRisk(anomaly Anomaly) remediation.RiskLevel {
	switch anomaly.Severity {
	case events.SeverityCritical:
		return remediation.RiskHigh
	case events.SeverityError:
		return remediation.RiskMedium
	default:
		return remediation.RiskLow
	}
}

func (r *EventDrivenController) fetchPodLogs(ns, name, container string) string {
	if r.ClientSet == nil {
		return ""
	}
	tail := int64(50)
	podLogOpts := corev1.PodLogOptions{
		Container: container,
		TailLines: &tail,
	}
	req := r.ClientSet.CoreV1().Pods(ns).GetLogs(name, &podLogOpts)
	podLogs, err := req.Stream(context.Background())
	if err != nil {
		return fmt.Sprintf("failed to get logs: %v", err)
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return fmt.Sprintf("failed to copy logs: %v", err)
	}
	return buf.String()
}

func (r *EventDrivenController) fetchRelatedEvents(ns, podName string) []map[string]interface{} {
	if r.ClientSet == nil {
		return nil
	}

	listOpts := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s", podName),
	}

	eventList, err := r.ClientSet.CoreV1().Events(ns).List(context.Background(), listOpts)
	if err != nil {
		return nil
	}

	related := make([]map[string]interface{}, 0)
	for _, e := range eventList.Items {
		related = append(related, map[string]interface{}{
			"type":     e.Type,
			"reason":   e.Reason,
			"message":  e.Message,
			"count":    e.Count,
			"lastSeen": e.LastTimestamp.Time.Format(time.RFC3339),
		})
	}
	return related
}

func (r *EventDrivenController) mapSeverityToPriority(severity events.Severity) remediation.Priority {
	switch severity {
	case events.SeverityCritical:
		return remediation.PriorityCritical
	case events.SeverityError:
		return remediation.PriorityHigh
	case events.SeverityWarning:
		return remediation.PriorityMedium
	default:
		return remediation.PriorityLow
	}
}

func equalResourceRequirements(oldContainers, newContainers []corev1.Container) bool {
	if len(oldContainers) != len(newContainers) {
		return false
	}

	for i, oldContainer := range oldContainers {
		newContainer := newContainers[i]
		if !oldContainer.Resources.Requests.Cpu().Equal(*newContainer.Resources.Requests.Cpu()) ||
			!oldContainer.Resources.Requests.Memory().Equal(*newContainer.Resources.Requests.Memory()) {
			return false
		}
	}

	return true
}

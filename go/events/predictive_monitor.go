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
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	"right-sizer/predictor"
)

// PredictiveMonitor monitors pod and node signals to predict and prevent issues
type PredictiveMonitor struct {
	k8sClient             kubernetes.Interface
	metricsClient         metricsv.Interface
	predictor             *predictor.Engine
	eventBus              *EventBus
	recommendationManager *RecommendationManager
	logger                logr.Logger

	// Configuration
	checkInterval    time.Duration
	alertThreshold   float64
	remediationDelay time.Duration

	// State
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
	mutex   sync.RWMutex

	// Alert tracking to prevent spam
	alertHistory map[string]time.Time
}

// NewPredictiveMonitor creates a new predictive monitor
func NewPredictiveMonitor(
	k8sClient kubernetes.Interface,
	metricsClient metricsv.Interface,
	predictorEngine *predictor.Engine,
	eventBus *EventBus,
	recommendationManager *RecommendationManager,
	logger logr.Logger,
) *PredictiveMonitor {
	return &PredictiveMonitor{
		k8sClient:             k8sClient,
		metricsClient:         metricsClient,
		predictor:             predictorEngine,
		eventBus:              eventBus,
		recommendationManager: recommendationManager,
		logger:                logger,
		checkInterval:         30 * time.Second,
		alertThreshold:        0.8, // 80% confidence threshold
		remediationDelay:      5 * time.Minute,
		alertHistory:          make(map[string]time.Time),
		stopCh:                make(chan struct{}),
	}
}

// Start begins the predictive monitoring
func (pm *PredictiveMonitor) Start(ctx context.Context) error {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if pm.running {
		return fmt.Errorf("predictive monitor already running")
	}

	pm.running = true
	pm.wg.Add(1)

	go pm.monitorLoop(ctx)

	pm.logger.Info("Predictive monitor started")
	return nil
}

// Stop stops the predictive monitoring
func (pm *PredictiveMonitor) Stop() {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()

	if !pm.running {
		return
	}

	pm.running = false
	close(pm.stopCh)
	pm.wg.Wait()

	pm.logger.Info("Predictive monitor stopped")
}

// monitorLoop runs the main monitoring loop
func (pm *PredictiveMonitor) monitorLoop(ctx context.Context) {
	defer pm.wg.Done()

	ticker := time.NewTicker(pm.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pm.stopCh:
			return
		case <-ticker.C:
			pm.checkPredictions(ctx)
		}
	}
}

// checkPredictions analyzes current metrics and makes predictions
func (pm *PredictiveMonitor) checkPredictions(ctx context.Context) {
	// Get all pods with metrics
	podMetrics, err := pm.metricsClient.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{})
	if err != nil {
		pm.logger.Error(err, "Failed to get pod metrics")
		return
	}

	// Check each pod for potential issues
	for _, podMetric := range podMetrics.Items {
		pm.analyzePodMetrics(ctx, &podMetric)
	}

	// Check node metrics
	nodeMetrics, err := pm.metricsClient.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{})
	if err != nil {
		pm.logger.Error(err, "Failed to get node metrics")
		return
	}

	for _, nodeMetric := range nodeMetrics.Items {
		pm.analyzeNodeMetrics(ctx, &nodeMetric)
	}
}

// analyzePodMetrics analyzes metrics for a single pod
func (pm *PredictiveMonitor) analyzePodMetrics(ctx context.Context, podMetric *v1beta1.PodMetrics) {
	podName := podMetric.Name
	namespace := podMetric.Namespace

	// Get pod spec for limits
	pod, err := pm.k8sClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		pm.logger.Error(err, "Failed to get pod spec", "pod", podName, "namespace", namespace)
		return
	}

	// Analyze each container
	for _, container := range pod.Spec.Containers {
		containerName := container.Name

		// Find corresponding metrics
		var containerMetric *v1beta1.ContainerMetrics
		for _, cm := range podMetric.Containers {
			if cm.Name == containerName {
				containerMetric = &cm
				break
			}
		}

		if containerMetric == nil {
			continue
		}

		pm.analyzeContainerMetrics(ctx, pod, &container, containerMetric)
	}
}

// analyzeContainerMetrics analyzes metrics for a single container
func (pm *PredictiveMonitor) analyzeContainerMetrics(
	ctx context.Context,
	pod *corev1.Pod,
	container *corev1.Container,
	containerMetric *v1beta1.ContainerMetrics,
) {
	// Calculate current utilization
	currentCPU := containerMetric.Usage.Cpu().MilliValue()
	currentMemory := containerMetric.Usage.Memory().Value()

	// Store data points for prediction
	if err := pm.predictor.StoreDataPoint(pod.Namespace, pod.Name, container.Name, "cpu", float64(currentCPU), time.Now()); err != nil {
		pm.logger.Error(err, "Failed to store CPU data point", "pod", pod.Name, "container", container.Name)
	}
	if err := pm.predictor.StoreDataPoint(pod.Namespace, pod.Name, container.Name, "memory", float64(currentMemory), time.Now()); err != nil {
		pm.logger.Error(err, "Failed to store memory data point", "pod", pod.Name, "container", container.Name)
	}

	// Get limits
	var cpuLimit, memoryLimit int64
	if container.Resources.Limits != nil {
		if cpu := container.Resources.Limits.Cpu(); cpu != nil {
			cpuLimit = cpu.MilliValue()
		}
		if mem := container.Resources.Limits.Memory(); mem != nil {
			memoryLimit = mem.Value()
		}
	}

	// Calculate utilization percentages
	var cpuUtilization, memoryUtilization float64
	if cpuLimit > 0 {
		cpuUtilization = float64(currentCPU) / float64(cpuLimit)
	}
	if memoryLimit > 0 {
		memoryUtilization = float64(currentMemory) / float64(memoryLimit)
	}

	// Check for immediate alerts
	if memoryUtilization > 0.8 {
		pm.sendAlert(EventResourcePredictedOOM, pod.Namespace, pod.Name,
			fmt.Sprintf("Container %s is at %.1f%% memory utilization", container.Name, memoryUtilization*100),
			map[string]interface{}{
				"container":         container.Name,
				"currentMemoryMB":   currentMemory / 1024 / 1024,
				"memoryLimitMB":     memoryLimit / 1024 / 1024,
				"utilization":       memoryUtilization,
				"recommendedAction": "Increase memory limit or optimize application",
			})
	}

	if cpuUtilization > 0.8 {
		pm.sendAlert(EventPodPredictedFailure, pod.Namespace, pod.Name,
			fmt.Sprintf("Container %s is at %.1f%% CPU utilization", container.Name, cpuUtilization*100),
			map[string]interface{}{
				"container":         container.Name,
				"currentCPUm":       currentCPU,
				"cpuLimitm":         cpuLimit,
				"utilization":       cpuUtilization,
				"recommendedAction": "Increase CPU limit or optimize application",
			})
	}

	// Use predictor for trend analysis
	pm.predictContainerIssues(ctx, pod, container, containerMetric)
}

// predictContainerIssues uses the predictor to forecast potential issues
func (pm *PredictiveMonitor) predictContainerIssues(
	ctx context.Context,
	pod *corev1.Pod,
	container *corev1.Container,
	containerMetric *v1beta1.ContainerMetrics,
) {
	// Request predictions for memory (most critical)
	request := predictor.PredictionRequest{
		Namespace:    pod.Namespace,
		PodName:      pod.Name,
		Container:    container.Name,
		ResourceType: "memory",
		Horizons:     []time.Duration{5 * time.Minute, 15 * time.Minute, 1 * time.Hour},
		Methods:      []predictor.PredictionMethod{predictor.PredictionMethodLinearRegression},
	}

	response, err := pm.predictor.Predict(ctx, request)
	if err != nil {
		pm.logger.Error(err, "Failed to get memory predictions", "pod", pod.Name, "container", container.Name)
		return
	}

	// Get current memory limit
	var memoryLimit int64
	if container.Resources.Limits != nil {
		if mem := container.Resources.Limits.Memory(); mem != nil {
			memoryLimit = mem.Value()
		}
	}

	// Check predictions for potential OOM
	for _, prediction := range response.Predictions {
		if prediction.Confidence < pm.alertThreshold {
			continue
		}

		predictedUtilization := prediction.Value / float64(memoryLimit)
		if predictedUtilization > 0.95 {
			pm.sendPredictiveAlert(
				EventResourcePredictedOOM,
				pod.Namespace,
				pod.Name,
				fmt.Sprintf("Container %s predicted OOM in %v (%.1f%% confidence)",
					container.Name, prediction.Horizon, prediction.Confidence*100),
				prediction.Horizon,
				prediction.Confidence,
				map[string]interface{}{
					"container":            container.Name,
					"predictedMemoryMB":    prediction.Value / 1024 / 1024,
					"memoryLimitMB":        memoryLimit / 1024 / 1024,
					"predictedUtilization": predictedUtilization,
					"recommendedAction":    "Increase memory limit preemptively",
					"evidence": map[string]interface{}{
						"method":  prediction.Method,
						"horizon": prediction.Horizon.String(),
					},
				},
			)
		}
	}
}

// analyzeNodeMetrics analyzes metrics for a single node
func (pm *PredictiveMonitor) analyzeNodeMetrics(ctx context.Context, nodeMetric *v1beta1.NodeMetrics) {
	nodeName := nodeMetric.Name

	// Get node spec
	node, err := pm.k8sClient.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		pm.logger.Error(err, "Failed to get node spec", "node", nodeName)
		return
	}

	currentCPU := nodeMetric.Usage.Cpu().MilliValue()
	currentMemory := nodeMetric.Usage.Memory().Value()

	// Get node capacity
	var cpuCapacity, memoryCapacity int64
	if cpu := node.Status.Capacity.Cpu(); cpu != nil {
		cpuCapacity = cpu.MilliValue()
	}
	if mem := node.Status.Capacity.Memory(); mem != nil {
		memoryCapacity = mem.Value()
	}

	// Calculate utilization
	var cpuUtilization, memoryUtilization float64
	if cpuCapacity > 0 {
		cpuUtilization = float64(currentCPU) / float64(cpuCapacity)
	}
	if memoryCapacity > 0 {
		memoryUtilization = float64(currentMemory) / float64(memoryCapacity)
	}

	// Check for node pressure
	if memoryUtilization > 0.85 {
		pm.sendAlert(EventNodePredictedFailure, "", nodeName,
			fmt.Sprintf("Node %s approaching memory pressure (%.1f%%)", nodeName, memoryUtilization*100),
			map[string]interface{}{
				"node":              nodeName,
				"currentMemoryGB":   currentMemory / 1024 / 1024 / 1024,
				"memoryCapacityGB":  memoryCapacity / 1024 / 1024 / 1024,
				"utilization":       memoryUtilization,
				"recommendedAction": "Consider draining node or adding more nodes",
			})
	}

	if cpuUtilization > 0.85 {
		pm.sendAlert(EventNodePredictedFailure, "", nodeName,
			fmt.Sprintf("Node %s approaching CPU pressure (%.1f%%)", nodeName, cpuUtilization*100),
			map[string]interface{}{
				"node":              nodeName,
				"currentCPUm":       currentCPU,
				"cpuCapacitym":      cpuCapacity,
				"utilization":       cpuUtilization,
				"recommendedAction": "Consider draining node or adding more nodes",
			})
	}
}

// sendAlert sends an immediate alert event
func (pm *PredictiveMonitor) sendAlert(eventType EventType, namespace, resource, message string, details map[string]interface{}) {
	alertKey := fmt.Sprintf("%s-%s-%s-%s", eventType, namespace, resource, message)

	// Check if we recently sent this alert
	if lastSent, exists := pm.alertHistory[alertKey]; exists {
		if time.Since(lastSent) < 10*time.Minute {
			return // Don't spam
		}
	}

	pm.alertHistory[alertKey] = time.Now()

	event := NewEvent(eventType, "cluster-id", namespace, resource, SeverityWarning, message)
	event.WithDetails(details)
	event.WithTags("predictive", "alert")

	pm.eventBus.Publish(event)

	// Also propose remediation for immediate alerts
	pm.proposeRemediation(eventType, namespace, resource, details)
}

// sendPredictiveAlert sends a predictive alert with time-to-event
func (pm *PredictiveMonitor) sendPredictiveAlert(
	eventType EventType,
	namespace, resource, message string,
	timeToEvent time.Duration,
	confidence float64,
	details map[string]interface{},
) {
	alertKey := fmt.Sprintf("%s-%s-%s-%s", eventType, namespace, resource, message)

	// Check if we recently sent this alert
	if lastSent, exists := pm.alertHistory[alertKey]; exists {
		if time.Since(lastSent) < 30*time.Minute {
			return // Don't spam predictive alerts as often
		}
	}

	pm.alertHistory[alertKey] = time.Now()

	event := NewEvent(eventType, "cluster-id", namespace, resource, SeverityInfo, message)
	event.WithDetails(details)
	event.WithTags("predictive", "forecast")

	// Add predictive event data
	predictiveData := &PredictiveEvent{
		ResourceType:      "pod",
		ResourceName:      resource,
		Namespace:         namespace,
		PredictionType:    string(eventType),
		Confidence:        confidence,
		TimeToEvent:       timeToEvent,
		RecommendedAction: details["recommendedAction"].(string),
		Evidence:          details["evidence"].(map[string]interface{}),
	}

	event.Details["predictive"] = predictiveData

	pm.eventBus.Publish(event)

	// Also send a remediation proposal
	pm.proposeRemediation(eventType, namespace, resource, details)
}

// proposeRemediation proposes automatic remediation actions
func (pm *PredictiveMonitor) proposeRemediation(eventType EventType, namespace, resource string, details map[string]interface{}) {
	var remediationAction, title, description string
	var parameters map[string]interface{}
	var urgency Urgency
	var severity Severity
	var timeToAction time.Duration

	switch eventType {
	case EventResourcePredictedOOM:
		remediationAction = "increase_memory_limit"
		title = "Increase Memory Limits"
		description = "Predicted out-of-memory condition detected. Increase memory limits to prevent pod crashes."
		parameters = map[string]interface{}{
			"targetResource": "memory",
			"multiplier":     1.5,
			"reason":         "Predicted OOM based on usage trends",
		}
		urgency = UrgencyHigh
		severity = SeverityWarning
		timeToAction = 15 * time.Minute

	case EventPodPredictedFailure:
		remediationAction = "increase_cpu_limit"
		title = "Increase CPU Limits"
		description = "Predicted CPU exhaustion detected. Increase CPU limits to prevent pod throttling."
		parameters = map[string]interface{}{
			"targetResource": "cpu",
			"multiplier":     1.3,
			"reason":         "Predicted CPU exhaustion based on usage trends",
		}
		urgency = UrgencyHigh
		severity = SeverityWarning
		timeToAction = 15 * time.Minute

	case EventNodePredictedFailure:
		remediationAction = "drain_node"
		title = "Drain Overloaded Node"
		description = "Node approaching resource capacity limits. Consider draining to redistribute workload."
		parameters = map[string]interface{}{
			"reason": "Node approaching resource limits",
		}
		urgency = UrgencyMedium
		severity = SeverityInfo
		timeToAction = 30 * time.Minute
	}

	if remediationAction != "" {
		// Create recommendation
		recommendation := pm.recommendationManager.CreateRecommendation(
			fmt.Sprintf("event-%d", time.Now().Unix()),
			"pod", // resource type
			resource,
			namespace,
			title,
			description,
			remediationAction,
			parameters,
			urgency,
			severity,
			0.85, // confidence
			timeToAction,
		)

		pm.logger.Info("Created remediation recommendation",
			"id", recommendation.ID,
			"action", remediationAction,
			"urgency", urgency,
			"resource", resource)
	}
}

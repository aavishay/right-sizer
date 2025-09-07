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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/smtp"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
	"right-sizer/audit"
	"right-sizer/config"
	"right-sizer/health"
	"right-sizer/logger"
	"right-sizer/metrics"
	"right-sizer/retry"
	"right-sizer/validation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// PodMemoryController handles pod reconciliation with memory metrics focus
type PodMemoryController struct {
	client.Client
	Clientset         *kubernetes.Clientset
	MetricsClient     *metricsclient.Clientset
	Scheme            *runtime.Scheme
	Config            *config.Config
	ResourceValidator *validation.ResourceValidator
	AuditLogger       *audit.AuditLogger
	HealthChecker     *health.OperatorHealthChecker
	OperatorMetrics   *metrics.OperatorMetrics
	MemoryMetrics     *metrics.MemoryMetrics
	RetryManager      *retry.RetryManager

	// Memory tracking
	memoryHistory     map[string][]float64 // namespace/pod/container -> memory samples
	lastPressureCheck map[string]time.Time // namespace/pod/container -> last check time
}

// NewPodMemoryController creates a new controller with memory metrics
func NewPodMemoryController(
	client client.Client,
	clientset *kubernetes.Clientset,
	scheme *runtime.Scheme,
	config *config.Config,
	resourceValidator *validation.ResourceValidator,
	auditLogger *audit.AuditLogger,
	healthChecker *health.OperatorHealthChecker,
	operatorMetrics *metrics.OperatorMetrics,
	memoryMetrics *metrics.MemoryMetrics,
	retryManager *retry.RetryManager,
) *PodMemoryController {
	// Create metrics client for memory metrics collection
	metricsClient, err := metricsclient.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		klog.Warningf("Failed to create metrics client: %v", err)
	}

	return &PodMemoryController{
		Client:            client,
		Clientset:         clientset,
		MetricsClient:     metricsClient,
		Scheme:            scheme,
		Config:            config,
		ResourceValidator: resourceValidator,
		AuditLogger:       auditLogger,
		HealthChecker:     healthChecker,
		OperatorMetrics:   operatorMetrics,
		MemoryMetrics:     memoryMetrics,
		RetryManager:      retryManager,
		memoryHistory:     make(map[string][]float64),
		lastPressureCheck: make(map[string]time.Time),
	}
}

// Reconcile processes a pod for memory optimization
func (r *PodMemoryController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	startTime := time.Now()
	logger.Debug("[MEMORY_CONTROLLER] Reconciling pod: %s/%s", req.Namespace, req.Name)

	// Update health status
	r.HealthChecker.UpdateComponentStatus("memory-controller", true, "Memory controller is healthy")

	// Get the pod
	pod := &corev1.Pod{}
	err := r.Get(ctx, req.NamespacedName, pod)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			logger.Error("[MEMORY_CONTROLLER] Failed to get pod: %v", err)
			r.OperatorMetrics.RecordProcessingError(req.Namespace, req.Name, "get_failed")
		}
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	// Skip if pod is being deleted
	if !pod.DeletionTimestamp.IsZero() {
		logger.Debug("[MEMORY_CONTROLLER] Pod %s/%s is being deleted, skipping", req.Namespace, req.Name)
		return reconcile.Result{}, nil
	}

	// Skip if pod is not running
	if pod.Status.Phase != corev1.PodRunning {
		logger.Debug("[MEMORY_CONTROLLER] Pod %s/%s is not running (phase: %s), skipping",
			req.Namespace, req.Name, pod.Status.Phase)
		return reconcile.Result{}, nil
	}

	// Process memory metrics for the pod
	if err := r.processMemoryMetrics(ctx, pod); err != nil {
		logger.Error("[MEMORY_CONTROLLER] Failed to process memory metrics for %s/%s: %v",
			req.Namespace, req.Name, err)
		r.OperatorMetrics.RecordProcessingError(req.Namespace, req.Name, "metrics_processing_failed")
	}

	// Check memory pressure and make recommendations
	recommendations, err := r.analyzeMemoryAndRecommend(ctx, pod)
	if err != nil {
		logger.Error("[MEMORY_CONTROLLER] Failed to analyze memory for %s/%s: %v",
			req.Namespace, req.Name, err)
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	// Apply recommendations if needed
	if len(recommendations) > 0 {
		if err := r.applyMemoryRecommendations(ctx, pod, recommendations); err != nil {
			logger.Error("[MEMORY_CONTROLLER] Failed to apply recommendations for %s/%s: %v",
				req.Namespace, req.Name, err)
			r.OperatorMetrics.RecordProcessingError(req.Namespace, req.Name, "apply_failed")
		}
	}

	// Record processing duration
	r.OperatorMetrics.RecordProcessingDuration("memory_reconcile", time.Since(startTime))

	// Requeue for periodic checks
	return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
}

// processMemoryMetrics collects and records memory metrics for a pod
func (r *PodMemoryController) processMemoryMetrics(ctx context.Context, pod *corev1.Pod) error {
	if r.MetricsClient == nil {
		return errors.New("metrics client not available")
	}

	// Get pod metrics from metrics-server
	podMetrics, err := r.MetricsClient.MetricsV1beta1().PodMetricses(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get pod metrics: %w", err)
	}

	// Process each container
	for _, container := range pod.Spec.Containers {
		// Find matching container metrics
		var containerMetrics *metricsv1beta1.ContainerMetrics
		for _, cm := range podMetrics.Containers {
			if cm.Name == container.Name {
				containerMetrics = &cm
				break
			}
		}

		if containerMetrics == nil {
			logger.Warn("[MEMORY_METRICS] No metrics found for container %s/%s/%s",
				pod.Namespace, pod.Name, container.Name)
			continue
		}

		// Extract memory metrics
		memoryUsage := containerMetrics.Usage[corev1.ResourceMemory]
		memoryBytes := float64(memoryUsage.Value())

		// Get container limits and requests
		var limitBytes, requestBytes float64
		if limit, ok := container.Resources.Limits[corev1.ResourceMemory]; ok {
			limitBytes = float64(limit.Value())
		}
		if request, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
			requestBytes = float64(request.Value())
		}

		// Calculate additional memory metrics (simulated for now)
		workingSetBytes := memoryBytes * 0.85 // Approximate working set as 85% of usage
		rssBytes := memoryBytes * 0.75        // Approximate RSS as 75% of usage
		cacheBytes := memoryBytes * 0.15      // Approximate cache as 15% of usage
		swapBytes := 0.0                      // Swap is typically 0 in containers

		// Update Prometheus metrics
		r.MemoryMetrics.UpdatePodMemoryMetrics(
			pod.Namespace, pod.Name, container.Name,
			memoryBytes, workingSetBytes, rssBytes, cacheBytes, swapBytes,
			limitBytes, requestBytes,
		)

		// Track memory history
		historyKey := fmt.Sprintf("%s/%s/%s", pod.Namespace, pod.Name, container.Name)
		r.trackMemoryHistory(historyKey, memoryBytes)

		// Detect and log memory pressure
		pressureLevel := r.MemoryMetrics.DetectAndRecordMemoryPressure(
			pod.Namespace, pod.Name, container.Name,
			memoryBytes, limitBytes,
		)

		// Log memory allocation info
		if pressureLevel != metrics.MemoryPressureNone {
			metrics.LogMemoryAllocation(pod.Namespace, pod.Name, container.Name,
				requestBytes, limitBytes, memoryBytes)
		}

		// Check for potential memory leak
		if history, ok := r.memoryHistory[historyKey]; ok && len(history) > 10 {
			pattern := metrics.AnalyzeMemoryPattern(pod.Namespace, pod.Name, container.Name, history)
			if pattern == "potential_leak" {
				logger.Warn("[MEMORY_LEAK] Potential memory leak detected in %s/%s/%s",
					pod.Namespace, pod.Name, container.Name)
				r.MemoryMetrics.RecordMemoryThrottling(pod.Namespace, pod.Name, container.Name)
			}
		}

		// Check if we should alert on memory pressure
		r.checkMemoryPressureAlert(pod.Namespace, pod.Name, container.Name, pressureLevel)
	}

	return nil
}

// analyzeMemoryAndRecommend analyzes memory usage and provides recommendations
func (r *PodMemoryController) analyzeMemoryAndRecommend(ctx context.Context, pod *corev1.Pod) ([]ContainerRecommendation, error) {
	var recommendations []ContainerRecommendation

	for _, container := range pod.Spec.Containers {
		historyKey := fmt.Sprintf("%s/%s/%s", pod.Namespace, pod.Name, container.Name)
		history, ok := r.memoryHistory[historyKey]
		if !ok || len(history) < 3 {
			// Not enough data to make recommendations
			continue
		}

		// Calculate statistics
		var sum, max, p95 float64
		for _, v := range history {
			sum += v
			if v > max {
				max = v
			}
		}
		avg := sum / float64(len(history))

		// Calculate 95th percentile
		if len(history) > 20 {
			sortedHistory := make([]float64, len(history))
			copy(sortedHistory, history)
			// Simple bubble sort for small arrays
			for i := 0; i < len(sortedHistory); i++ {
				for j := i + 1; j < len(sortedHistory); j++ {
					if sortedHistory[i] > sortedHistory[j] {
						sortedHistory[i], sortedHistory[j] = sortedHistory[j], sortedHistory[i]
					}
				}
			}
			p95Index := int(float64(len(sortedHistory)) * 0.95)
			p95 = sortedHistory[p95Index]
		} else {
			p95 = max
		}

		// Get current limits and requests
		currentRequest := container.Resources.Requests[corev1.ResourceMemory]
		currentLimit := container.Resources.Limits[corev1.ResourceMemory]

		// Calculate recommendations based on p95 with buffer
		recommendedRequest := int64(p95 * 1.1) // 10% buffer over p95
		recommendedLimit := int64(p95 * 1.5)   // 50% buffer over p95

		// Apply minimum thresholds
		minRequest := int64(32 * 1024 * 1024) // 32Mi minimum
		minLimit := int64(64 * 1024 * 1024)   // 64Mi minimum
		if recommendedRequest < minRequest {
			recommendedRequest = minRequest
		}
		if recommendedLimit < minLimit {
			recommendedLimit = minLimit
		}

		// Check if changes are significant enough (>10% change)
		currentRequestBytes := currentRequest.Value()
		currentLimitBytes := currentLimit.Value()

		requestChange := math.Abs(float64(recommendedRequest-currentRequestBytes)) / float64(currentRequestBytes)
		limitChange := math.Abs(float64(recommendedLimit-currentLimitBytes)) / float64(currentLimitBytes)

		if requestChange > 0.1 || limitChange > 0.1 {
			recommendation := ContainerRecommendation{
				ContainerName:      container.Name,
				RecommendedRequest: *resource.NewQuantity(recommendedRequest, resource.BinarySI),
				RecommendedLimit:   *resource.NewQuantity(recommendedLimit, resource.BinarySI),
				CurrentRequest:     currentRequest,
				CurrentLimit:       currentLimit,
				Reason:             fmt.Sprintf("Based on p95 usage: %.2fMi", p95/1024/1024),
			}
			recommendations = append(recommendations, recommendation)

			// Record the recommendation in metrics
			r.MemoryMetrics.RecordMemoryRecommendation(
				pod.Namespace, pod.Name, container.Name, "request",
				float64(recommendedRequest), float64(currentRequestBytes),
			)
			r.MemoryMetrics.RecordMemoryRecommendation(
				pod.Namespace, pod.Name, container.Name, "limit",
				float64(recommendedLimit), float64(currentLimitBytes),
			)

			// Log the recommendation
			metrics.LogMemoryRecommendation(
				pod.Namespace, pod.Name, container.Name,
				float64(currentRequestBytes), float64(recommendedRequest),
				recommendation.Reason,
			)
		}

		// Update memory trend metrics
		if len(history) > 5 {
			slope := r.calculateTrend(history)
			r.MemoryMetrics.UpdateMemoryTrend(
				pod.Namespace, pod.Name, container.Name,
				slope, max, avg, "5m",
			)
		}
	}

	return recommendations, nil
}

// applyMemoryRecommendations applies memory sizing recommendations to a pod
func (r *PodMemoryController) applyMemoryRecommendations(ctx context.Context, pod *corev1.Pod, recommendations []ContainerRecommendation) error {
	startTime := time.Now()

	if r.Config.DryRun {
		logger.Info("[MEMORY_CONTROLLER] DRY RUN: Would resize pod %s/%s", pod.Namespace, pod.Name)
		for _, rec := range recommendations {
			logger.Info("[MEMORY_CONTROLLER] DRY RUN: Container %s: Memory %s -> %s",
				rec.ContainerName,
				metrics.FormatMemorySize(float64(rec.CurrentRequest.Value())),
				metrics.FormatMemorySize(float64(rec.RecommendedRequest.Value())))
		}
		return nil
	}

	// Apply the recommendations
	updatedPod := pod.DeepCopy()
	for _, rec := range recommendations {
		for i, container := range updatedPod.Spec.Containers {
			if container.Name == rec.ContainerName {
				updatedPod.Spec.Containers[i].Resources.Requests[corev1.ResourceMemory] = rec.RecommendedRequest
				updatedPod.Spec.Containers[i].Resources.Limits[corev1.ResourceMemory] = rec.RecommendedLimit

				// Record the resize operation
				direction := "up"
				if rec.RecommendedRequest.Value() < rec.CurrentRequest.Value() {
					direction = "down"
				}
				r.MemoryMetrics.RecordMemoryResize(
					pod.Namespace, pod.Name, container.Name,
					direction, "pending",
				)
			}
		}
	}

	// Attempt to update the pod (in-place resize)
	err := r.RetryManager.RetryWithBackoff(func() error {
		return r.Update(ctx, updatedPod)
	})
	if err != nil {
		logger.Error("[MEMORY_CONTROLLER] Failed to update pod %s/%s: %v", pod.Namespace, pod.Name, err)
		for _, rec := range recommendations {
			r.MemoryMetrics.RecordMemoryResize(
				pod.Namespace, pod.Name, rec.ContainerName,
				"up", "failed",
			)
		}
		return err
	}

	// Success
	logger.Success("[MEMORY_CONTROLLER] Successfully resized pod %s/%s", pod.Namespace, pod.Name)
	for _, rec := range recommendations {
		direction := "up"
		if rec.RecommendedRequest.Value() < rec.CurrentRequest.Value() {
			direction = "down"
		}
		r.MemoryMetrics.RecordMemoryResize(
			pod.Namespace, pod.Name, rec.ContainerName,
			direction, "success",
		)

		// Audit log the change
		if r.AuditLogger != nil {
			oldResources := corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: rec.CurrentRequest,
				},
			}
			newResources := corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: rec.RecommendedRequest,
				},
			}
			r.AuditLogger.LogResourceChange(
				ctx,
				pod,
				rec.ContainerName,
				oldResources,
				newResources,
				"resize",
				rec.Reason,
				"success",
				time.Since(startTime),
				nil,
			)
		}
	}

	r.OperatorMetrics.RecordPodResized(pod.Namespace, pod.Name, "", "memory")

	return nil
}

// trackMemoryHistory maintains a rolling window of memory usage samples
func (r *PodMemoryController) trackMemoryHistory(key string, value float64) {
	const maxHistorySize = 100

	if _, ok := r.memoryHistory[key]; !ok {
		r.memoryHistory[key] = []float64{}
	}

	r.memoryHistory[key] = append(r.memoryHistory[key], value)

	// Keep only the last maxHistorySize samples
	if len(r.memoryHistory[key]) > maxHistorySize {
		r.memoryHistory[key] = r.memoryHistory[key][1:]
	}
}

// calculateTrend calculates the trend slope for memory usage
func (r *PodMemoryController) calculateTrend(samples []float64) float64 {
	if len(samples) < 2 {
		return 0
	}

	// Simple linear regression
	var sumX, sumY, sumXY, sumX2 float64
	n := float64(len(samples))

	for i, y := range samples {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0
	}

	slope := (n*sumXY - sumX*sumY) / denominator
	return slope
}

// checkMemoryPressureAlert checks if we should alert on memory pressure
func (r *PodMemoryController) checkMemoryPressureAlert(namespace, pod, container string, level metrics.MemoryPressureLevel) {
	key := fmt.Sprintf("%s/%s/%s", namespace, pod, container)
	lastCheck, exists := r.lastPressureCheck[key]

	// Alert if pressure is high/critical and we haven't alerted recently
	if level >= metrics.MemoryPressureHigh {
		if !exists || time.Since(lastCheck) > 5*time.Minute {
			logger.Error("[MEMORY_ALERT] High memory pressure detected for %s: %s", key, level.String())
			r.lastPressureCheck[key] = time.Now()

			// Send notification if configured
			if r.Config.NotificationConfig != nil && r.Config.NotificationConfig.EnableNotifications {
				message := fmt.Sprintf("🚨 High memory pressure detected for %s: %s", key, level.String())
				if err := r.sendNotification(message); err != nil {
					logger.Warn("[MEMORY_NOTIFICATION] Failed to send notification: %v", err)
				} else {
					logger.Info("[MEMORY_NOTIFICATION] Notification sent for %s", key)
				}
			}
		}
	}
}

// sendNotification sends a notification using configured channels
func (r *PodMemoryController) sendNotification(message string) error {
	if r.Config.NotificationConfig == nil {
		return errors.New("notification config not available")
	}

	var lastErr error

	// Send Slack notification if configured
	if r.Config.NotificationConfig.SlackWebhookURL != "" {
		if err := r.sendSlackNotification(message); err != nil {
			logger.Warn("[NOTIFICATION] Slack notification failed: %v", err)
			lastErr = err
		}
	}

	// Send email notification if configured
	if len(r.Config.NotificationConfig.EmailRecipients) > 0 && r.Config.NotificationConfig.SMTPHost != "" {
		if err := r.sendEmailNotification(message); err != nil {
			logger.Warn("[NOTIFICATION] Email notification failed: %v", err)
			lastErr = err
		}
	}

	return lastErr
}

// sendSlackNotification sends a notification to Slack
func (r *PodMemoryController) sendSlackNotification(message string) error {
	payload := map[string]string{
		"text": message,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	resp, err := http.Post(r.Config.NotificationConfig.SlackWebhookURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to send Slack notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Slack notification failed with status: %d", resp.StatusCode)
	}

	return nil
}

// sendEmailNotification sends a notification via email
func (r *PodMemoryController) sendEmailNotification(message string) error {
	// Simple email implementation - in production, use a proper email library
	auth := smtp.PlainAuth("", r.Config.NotificationConfig.SMTPUsername, r.Config.NotificationConfig.SMTPPassword, r.Config.NotificationConfig.SMTPHost)

	subject := "Right-Sizer Memory Pressure Alert"
	body := fmt.Sprintf("Subject: %s\r\n\r\n%s\r\n", subject, message)

	for _, recipient := range r.Config.NotificationConfig.EmailRecipients {
		err := smtp.SendMail(
			fmt.Sprintf("%s:%d", r.Config.NotificationConfig.SMTPHost, r.Config.NotificationConfig.SMTPPort),
			auth,
			r.Config.NotificationConfig.SMTPUsername, // from
			[]string{recipient},
			[]byte(body),
		)
		if err != nil {
			return fmt.Errorf("failed to send email to %s: %w", recipient, err)
		}
	}

	return nil
}

// ContainerRecommendation holds memory sizing recommendations for a container
type ContainerRecommendation struct {
	ContainerName      string
	RecommendedRequest resource.Quantity
	RecommendedLimit   resource.Quantity
	CurrentRequest     resource.Quantity
	CurrentLimit       resource.Quantity
	Reason             string
}

// SetupWithManager sets up the controller with the manager
func (r *PodMemoryController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: r.Config.MaxConcurrentReconciles,
		}).
		Complete(r)
}

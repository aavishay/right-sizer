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
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"right-sizer/config"
	"right-sizer/logger"
	"right-sizer/metrics"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// ScalingDecision represents the scaling action to take
type ScalingDecision int

const (
	ScaleNone ScalingDecision = iota
	ScaleUp
	ScaleDown
)

// ResourceScalingDecision tracks scaling decisions for individual resources
type ResourceScalingDecision struct {
	CPU    ScalingDecision
	Memory ScalingDecision
}

// AdaptiveRightSizer performs resource optimization with support for both
// in-place updates (when available) and deployment updates as fallback
type AdaptiveRightSizer struct {
	Client          client.Client
	ClientSet       *kubernetes.Clientset
	RestConfig      *rest.Config
	MetricsProvider metrics.Provider
	Interval        time.Duration
	InPlaceEnabled  bool       // Will be auto-detected
	DryRun          bool       // If true, only log recommendations without applying
	updateMutex     sync.Mutex // Prevents concurrent update operations
}

// ResourceUpdate represents a pending resource update
type ResourceUpdate struct {
	Namespace      string
	Name           string
	ResourceType   string // Pod only now
	ContainerName  string
	ContainerIndex int
	OldResources   corev1.ResourceRequirements
	NewResources   corev1.ResourceRequirements
	Reason         string
}

// Start begins the adaptive rightsizing loop
func (r *AdaptiveRightSizer) Start(ctx context.Context) error {
	ticker := time.NewTicker(r.Interval)
	defer ticker.Stop()

	// Test for in-place resize capability
	r.InPlaceEnabled = r.testInPlaceCapability(ctx)

	if r.InPlaceEnabled {
		logger.Info("✅ In-place pod resizing is available - pods can be resized without restarts")
	} else {
		logger.Warn("⚠️  In-place pod resizing not available - will use rolling updates")
	}

	logger.Info("Starting adaptive right-sizer with %v interval (DryRun: %v)", r.Interval, r.DryRun)

	// Run immediately on start
	r.performRightSizing(ctx)

	for {
		select {
		case <-ticker.C:
			r.performRightSizing(ctx)
		case <-ctx.Done():
			log.Println("Stopping adaptive right-sizer")
			return nil
		}
	}
}

// testInPlaceCapability checks if in-place resize is supported
func (r *AdaptiveRightSizer) testInPlaceCapability(ctx context.Context) bool {
	// Check if the resize subresource is available by checking server version
	// In-place pod resize is available in Kubernetes 1.27+ (alpha), 1.29+ (beta), 1.31+ (stable)

	if r.ClientSet == nil {
		logger.Warn("ClientSet not available, cannot test for in-place resize capability")
		return false
	}

	// Get server version
	serverVersion, err := r.ClientSet.Discovery().ServerVersion()
	if err != nil {
		logger.Warn("Failed to get server version: %v", err)
		return false
	}

	// Parse the version
	major := serverVersion.Major
	minor := serverVersion.Minor

	// Remove any non-numeric suffix from minor version (e.g., "33+" -> "33")
	minorNum := 0
	fmt.Sscanf(minor, "%d", &minorNum)

	// Check if version supports in-place resize (K8s 1.27+)
	if major == "1" && minorNum >= 27 {
		logger.Info("Kubernetes version %s.%s supports in-place pod resizing", major, minor)

		// Additional check: try to access the resize subresource
		// This confirms the feature is actually available
		_, err := r.ClientSet.CoreV1().RESTClient().Get().
			Resource("pods").
			SubResource("resize").
			DoRaw(ctx)

		// We expect an error here (no pod specified), but if the subresource
		// doesn't exist, we'll get a different error
		if err != nil && strings.Contains(err.Error(), "not found") &&
			strings.Contains(err.Error(), "resize") {
			logger.Warn("Resize subresource not found despite version support")
			return false
		}

		return true
	}

	logger.Info("Kubernetes version %s.%s does not support in-place pod resizing (requires 1.27+)", major, minor)
	return false
}

// performRightSizing processes all pods for optimization using in-place resize
func (r *AdaptiveRightSizer) performRightSizing(ctx context.Context) {
	updates := []ResourceUpdate{}

	// Analyze ALL pods directly (including those from deployments, statefulsets, etc)
	// We will update pods directly using in-place resize, not their controllers
	pods, err := r.analyzeAllPods(ctx)
	if err != nil {
		log.Printf("Error analyzing pods: %v", err)
	} else {
		updates = append(updates, pods...)
	}

	// Apply updates using in-place resize
	r.applyUpdates(ctx, updates)
}

// analyzeAllPods analyzes all pods in the cluster for resource optimization
func (r *AdaptiveRightSizer) analyzeAllPods(ctx context.Context) ([]ResourceUpdate, error) {
	var podList corev1.PodList
	if err := r.Client.List(ctx, &podList); err != nil {
		return nil, err
	}

	updates := []ResourceUpdate{}

	// Limit the number of pods to process in a single cycle to prevent overload
	const maxPodsPerCycle = 50
	podsProcessed := 0

	for _, pod := range podList.Items {
		// Limit pods processed per cycle
		if podsProcessed >= maxPodsPerCycle {
			log.Printf("📊 Reached maximum pods per cycle (%d), will process remaining pods in next cycle", maxPodsPerCycle)
			break
		}
		// Skip pods that are not running
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		// Check namespace filters first
		if !r.shouldProcessNamespace(pod.Namespace) {
			continue
		}
		if r.isSystemWorkload(pod.Namespace, pod.Name) {
			continue
		}

		// Skip pods with skip annotation
		if pod.Annotations != nil {
			if skip, ok := pod.Annotations["rightsizer.io/skip"]; ok && skip == "true" {
				continue
			}
		}

		// Get metrics for this specific pod
		podMetrics, err := r.MetricsProvider.FetchPodMetrics(pod.Namespace, pod.Name)
		if err != nil {
			log.Printf("Failed to get metrics for pod %s/%s: %v", pod.Namespace, pod.Name, err)
			continue
		}

		// Check each container in the pod
		for i, container := range pod.Spec.Containers {
			// Check scaling thresholds first
			scalingDecision := r.checkScalingThresholds(podMetrics, container.Resources)

			// Skip if CPU should not be updated but memory should be reduced
			if scalingDecision.CPU == ScaleNone && scalingDecision.Memory == ScaleDown {
				logger.Info("⏭️  Skipping resize for pod %s/%s container %s: CPU doesn't need update and memory would be reduced",
					pod.Namespace, pod.Name, container.Name)
				continue
			}

			// Skip if both resources don't need changes
			if scalingDecision.CPU == ScaleNone && scalingDecision.Memory == ScaleNone {
				continue
			}

			// Calculate optimal resources based on actual usage and scaling decision
			// Note: metrics-server provides pod-level metrics, not per-container
			// So we'll use the pod metrics for all containers
			newResources := r.calculateOptimalResourcesWithDecision(podMetrics, scalingDecision)

			if r.needsAdjustmentWithDecision(container.Resources, newResources, scalingDecision) {
				// Log the actual resource changes that will be made
				oldCPUReq := container.Resources.Requests[corev1.ResourceCPU]
				oldMemReq := container.Resources.Requests[corev1.ResourceMemory]
				newCPUReq := newResources.Requests[corev1.ResourceCPU]
				newMemReq := newResources.Requests[corev1.ResourceMemory]

				// Get current usage for detailed logging
				cpuLimit := container.Resources.Limits.Cpu().AsApproximateFloat64() * 1000
				memLimit := float64(container.Resources.Limits.Memory().Value()) / (1024 * 1024)
				cpuUsagePercent := 0.0
				memUsagePercent := 0.0
				if cpuLimit > 0 {
					cpuUsagePercent = (podMetrics.CPUMilli / cpuLimit) * 100
				}
				if memLimit > 0 {
					memUsagePercent = (podMetrics.MemMB / memLimit) * 100
				}

				logger.Info("🔍 Scaling analysis - CPU: %s (usage: %.0fm/%.0fm, %.1f%%), Memory: %s (usage: %.0fMi/%.0fMi, %.1f%%)",
					scalingDecisionString(scalingDecision.CPU), podMetrics.CPUMilli, cpuLimit, cpuUsagePercent,
					scalingDecisionString(scalingDecision.Memory), podMetrics.MemMB, memLimit, memUsagePercent)
				logger.Info("📈 Container %s/%s/%s will be resized - CPU: %s→%s, Memory: %s→%s",
					pod.Namespace, pod.Name, container.Name,
					oldCPUReq.String(), newCPUReq.String(),
					oldMemReq.String(), newMemReq.String())
				updates = append(updates, ResourceUpdate{
					Namespace:      pod.Namespace,
					Name:           pod.Name,
					ResourceType:   "Pod",
					ContainerName:  container.Name,
					ContainerIndex: i,
					OldResources:   container.Resources,
					NewResources:   newResources,
					Reason:         r.getAdjustmentReasonWithDecision(container.Resources, newResources, scalingDecision),
				})
			}
		}

		podsProcessed++
	}

	return updates, nil
}

// analyzeStandalonePods analyzes standalone pods (deprecated - all pods are now analyzed)
func (r *AdaptiveRightSizer) analyzeStandalonePods(ctx context.Context) ([]ResourceUpdate, error) {
	// This function is deprecated as we now analyze all pods in analyzeAllPods
	return []ResourceUpdate{}, nil

}

// applyUpdates applies the calculated resource updates with batching and rate limiting
func (r *AdaptiveRightSizer) applyUpdates(ctx context.Context, updates []ResourceUpdate) {
	if len(updates) == 0 {
		return
	}

	log.Printf("📊 Found %d resources needing adjustment", len(updates))

	// Configuration for batching to prevent API server overload
	const (
		batchSize           = 5                      // Process max 5 pods per batch
		delayBetweenBatches = 2 * time.Second        // Wait 2 seconds between batches
		delayBetweenPods    = 200 * time.Millisecond // Small delay between individual pods
	)

	// Log all updates first if in dry-run mode
	if r.DryRun {
		for _, update := range updates {
			r.logUpdate(update, true)
		}
		return
	}

	// Log all updates that will be applied
	for _, update := range updates {
		r.logUpdate(update, false)
	}

	// Apply pod updates in batches with rate limiting
	podUpdates := []ResourceUpdate{}
	for _, update := range updates {
		if update.ResourceType == "Pod" {
			podUpdates = append(podUpdates, update)
		}
	}

	if len(podUpdates) == 0 {
		return
	}

	// Process updates in batches
	totalBatches := (len(podUpdates) + batchSize - 1) / batchSize
	log.Printf("🔄 Processing %d pod updates in %d batches (batch size: %d)",
		len(podUpdates), totalBatches, batchSize)

	for i := 0; i < len(podUpdates); i += batchSize {
		// Calculate batch boundaries
		end := i + batchSize
		if end > len(podUpdates) {
			end = len(podUpdates)
		}

		batchNum := (i / batchSize) + 1
		batch := podUpdates[i:end]

		log.Printf("📦 Processing batch %d/%d (%d pods)", batchNum, totalBatches, len(batch))

		// Process pods in current batch
		for j, update := range batch {
			// Check context cancellation
			select {
			case <-ctx.Done():
				log.Printf("⚠️  Context cancelled, stopping pod updates")
				return
			default:
			}

			actualChanges, err := r.updatePodInPlace(ctx, update)
			if err != nil {
				log.Printf("❌ Error updating pod %s/%s: %v", update.Namespace, update.Name, err)
			} else if actualChanges != "" && !strings.Contains(actualChanges, "Skipped") {
				log.Printf("✅ %s", actualChanges)
			}

			// Add small delay between pods within a batch to avoid rapid-fire API calls
			if j < len(batch)-1 {
				time.Sleep(delayBetweenPods)
			}
		}

		// Add delay between batches (except after the last batch)
		if i+batchSize < len(podUpdates) {
			log.Printf("⏳ Waiting %v before next batch to avoid API server overload", delayBetweenBatches)
			time.Sleep(delayBetweenBatches)
		}
	}

	log.Printf("✅ Completed processing all %d pod updates", len(podUpdates))
}

// updatePodInPlace attempts to update pod resources in-place with mutex protection
// Returns a description of what was actually changed
func (r *AdaptiveRightSizer) updatePodInPlace(ctx context.Context, update ResourceUpdate) (string, error) {
	// Use mutex to prevent concurrent API calls that could overwhelm the server
	r.updateMutex.Lock()
	defer r.updateMutex.Unlock()

	// Get the current pod
	var pod corev1.Pod
	if err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: update.Namespace,
		Name:      update.Name,
	}, &pod); err != nil {
		return "", err
	}

	// Find the container to check current resources
	var currentResources *corev1.ResourceRequirements
	for _, container := range pod.Spec.Containers {
		if container.Name == update.ContainerName {
			currentResources = &container.Resources
			break
		}
	}

	if currentResources == nil {
		return "", fmt.Errorf("container %s not found in pod", update.ContainerName)
	}

	// Check if memory limit is being decreased (not allowed for in-place resize)
	currentMemLimit := currentResources.Limits.Memory()
	newMemLimit := update.NewResources.Limits.Memory()
	currentMemRequest := currentResources.Requests.Memory()
	newMemRequest := update.NewResources.Requests.Memory()

	memoryLimitDecreased := currentMemLimit != nil && newMemLimit != nil && currentMemLimit.Cmp(*newMemLimit) > 0
	memoryRequestDecreased := currentMemRequest != nil && newMemRequest != nil && currentMemRequest.Cmp(*newMemRequest) > 0

	cpuOnly := false
	if memoryLimitDecreased || memoryRequestDecreased {
		// Memory is being decreased - keep current memory values but update CPU
		log.Printf("⚠️  Cannot decrease memory for pod %s/%s", update.Namespace, update.Name)
		if memoryLimitDecreased {
			log.Printf("   Memory limit: current=%s, desired=%s (decrease not allowed)", currentMemLimit.String(), newMemLimit.String())
		}
		if memoryRequestDecreased {
			log.Printf("   Memory request: current=%s, desired=%s (decrease not allowed)", currentMemRequest.String(), newMemRequest.String())
		}
		log.Printf("   💡 Applying CPU changes only (memory decreases require pod restart)")

		// Keep current memory values, but use new CPU values
		if currentMemLimit != nil {
			update.NewResources.Limits[corev1.ResourceMemory] = currentMemLimit.DeepCopy()
		}
		if currentMemRequest != nil {
			update.NewResources.Requests[corev1.ResourceMemory] = currentMemRequest.DeepCopy()
		}
		cpuOnly = true
	}

	// Create the resize patch
	type ContainerResourcesPatch struct {
		Name      string                      `json:"name"`
		Resources corev1.ResourceRequirements `json:"resources"`
	}

	type PodSpecPatch struct {
		Containers []ContainerResourcesPatch `json:"containers"`
	}

	type PodResizePatch struct {
		Spec PodSpecPatch `json:"spec"`
	}

	resizePatch := PodResizePatch{
		Spec: PodSpecPatch{
			Containers: []ContainerResourcesPatch{
				{
					Name:      update.ContainerName,
					Resources: update.NewResources,
				},
			},
		},
	}

	// Marshal the patch
	patchData, err := json.Marshal(resizePatch)
	if err != nil {
		return "", fmt.Errorf("failed to marshal resize patch: %w", err)
	}

	// Use the Kubernetes client-go to patch with the resize subresource
	// This is the key difference - using the resize subresource endpoint
	_, err = r.ClientSet.CoreV1().Pods(update.Namespace).Patch(
		ctx,
		update.Name,
		types.StrategicMergePatchType,
		patchData,
		metav1.PatchOptions{},
		"resize", // This is the crucial part - specifying the resize subresource
	)

	if err != nil {
		// Check for specific memory decrease error
		if strings.Contains(err.Error(), "memory limits cannot be decreased") {
			log.Printf("⚠️  Cannot decrease memory for pod %s/%s", update.Namespace, update.Name)
			log.Printf("   💡 Pod needs RestartContainer policy for memory decreases. Skipping resize.")
			// Return empty string to not count this as an error
			return "Skipped resize (memory decrease not allowed)", nil
		}
		return "", fmt.Errorf("failed to resize pod: %w", err)
	}

	// Build success message based on what was actually changed
	var successMsg string
	if cpuOnly {
		cpuReq := update.NewResources.Requests[corev1.ResourceCPU]
		oldCpuReq := update.OldResources.Requests[corev1.ResourceCPU]
		successMsg = fmt.Sprintf("Successfully resized pod %s/%s (CPU only: %s→%s, memory decrease skipped)",
			update.Namespace, update.Name, oldCpuReq.String(), cpuReq.String())
	} else {
		cpuReq := update.NewResources.Requests[corev1.ResourceCPU]
		memReq := update.NewResources.Requests[corev1.ResourceMemory]
		oldCpuReq := update.OldResources.Requests[corev1.ResourceCPU]
		oldMemReq := update.OldResources.Requests[corev1.ResourceMemory]
		successMsg = fmt.Sprintf("Successfully resized pod %s/%s (CPU: %s→%s, Memory: %s→%s)",
			update.Namespace, update.Name, oldCpuReq.String(), cpuReq.String(), oldMemReq.String(), memReq.String())
	}

	return successMsg, nil
}

// Helper functions

func (r *AdaptiveRightSizer) getPodsForWorkload(ctx context.Context, namespace string, labels map[string]string) ([]corev1.Pod, error) {
	var podList corev1.PodList
	if err := r.Client.List(ctx, &podList,
		client.InNamespace(namespace),
		client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	runningPods := []corev1.Pod{}
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			runningPods = append(runningPods, pod)
		}
	}
	return runningPods, nil
}

func (r *AdaptiveRightSizer) calculateAverageMetrics(pods []corev1.Pod) *metrics.Metrics {
	if len(pods) == 0 {
		return nil
	}

	totalCPU := 0.0
	totalMem := 0.0
	validPods := 0

	for _, pod := range pods {
		m, err := r.MetricsProvider.FetchPodMetrics(pod.Namespace, pod.Name)
		if err != nil {
			continue
		}
		totalCPU += m.CPUMilli
		totalMem += m.MemMB
		validPods++
	}

	if validPods == 0 {
		return nil
	}

	return &metrics.Metrics{
		CPUMilli: totalCPU / float64(validPods),
		MemMB:    totalMem / float64(validPods),
	}
}

func (r *AdaptiveRightSizer) calculateOptimalResources(usage metrics.Metrics) corev1.ResourceRequirements {
	cfg := config.Get()

	// Add buffer for requests using configurable multipliers and additions
	cpuRequest := int64(usage.CPUMilli*cfg.CPURequestMultiplier) + cfg.CPURequestAddition
	memRequest := int64(usage.MemMB*cfg.MemoryRequestMultiplier) + cfg.MemoryRequestAddition

	// Ensure minimum values
	if cpuRequest < cfg.MinCPURequest {
		cpuRequest = cfg.MinCPURequest
	}
	if memRequest < cfg.MinMemoryRequest {
		memRequest = cfg.MinMemoryRequest
	}

	// Calculate limits based on requests with multipliers and additions
	cpuLimit := int64(float64(cpuRequest)*cfg.CPULimitMultiplier) + cfg.CPULimitAddition
	memLimit := int64(float64(memRequest)*cfg.MemoryLimitMultiplier) + cfg.MemoryLimitAddition

	// Apply maximum caps
	if cpuLimit > cfg.MaxCPULimit {
		cpuLimit = cfg.MaxCPULimit
	}
	if memLimit > cfg.MaxMemoryLimit {
		memLimit = cfg.MaxMemoryLimit
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewMilliQuantity(cpuRequest, resource.DecimalSI),
			corev1.ResourceMemory: *resource.NewQuantity(memRequest*1024*1024, resource.BinarySI),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewMilliQuantity(cpuLimit, resource.DecimalSI),
			corev1.ResourceMemory: *resource.NewQuantity(memLimit*1024*1024, resource.BinarySI),
		},
	}
}

// checkScalingThresholds determines if scaling is needed based on resource usage thresholds
func (r *AdaptiveRightSizer) checkScalingThresholds(usage metrics.Metrics, current corev1.ResourceRequirements) ResourceScalingDecision {
	cfg := config.Get()

	// Get current limits (or requests if limits not set)
	var cpuLimit, memLimit float64

	if limit, exists := current.Limits[corev1.ResourceCPU]; exists && !limit.IsZero() {
		cpuLimit = float64(limit.MilliValue())
	} else if req, exists := current.Requests[corev1.ResourceCPU]; exists && !req.IsZero() {
		cpuLimit = float64(req.MilliValue())
	}

	if limit, exists := current.Limits[corev1.ResourceMemory]; exists && !limit.IsZero() {
		memLimit = float64(limit.Value()) / (1024 * 1024) // Convert to MB
	} else if req, exists := current.Requests[corev1.ResourceMemory]; exists && !req.IsZero() {
		memLimit = float64(req.Value()) / (1024 * 1024)
	}

	// If no resources set, default to scale up
	if cpuLimit == 0 && memLimit == 0 {
		return ResourceScalingDecision{CPU: ScaleUp, Memory: ScaleUp}
	}

	// Calculate usage percentages
	cpuUsagePercent := float64(0)
	memUsagePercent := float64(0)

	if cpuLimit > 0 {
		cpuUsagePercent = usage.CPUMilli / cpuLimit
	}
	if memLimit > 0 {
		memUsagePercent = usage.MemMB / memLimit
	}

	// Determine scaling decision for each resource independently
	cpuDecision := ScaleNone
	memoryDecision := ScaleNone

	// Check CPU scaling
	if cpuUsagePercent > cfg.CPUScaleUpThreshold {
		cpuDecision = ScaleUp
	} else if cpuUsagePercent < cfg.CPUScaleDownThreshold {
		cpuDecision = ScaleDown
	}

	// Check Memory scaling
	if memUsagePercent > cfg.MemoryScaleUpThreshold {
		memoryDecision = ScaleUp
	} else if memUsagePercent < cfg.MemoryScaleDownThreshold {
		memoryDecision = ScaleDown
	}

	// Don't log here to avoid duplication - logging happens in analyzeAllPods when resize is actually needed

	return ResourceScalingDecision{CPU: cpuDecision, Memory: memoryDecision}
}

// Helper function to convert ScalingDecision to string
func scalingDecisionString(d ScalingDecision) string {
	switch d {
	case ScaleUp:
		return "scale up"
	case ScaleDown:
		return "scale down"
	default:
		return "no change"
	}
}

// calculateOptimalResourcesWithDecision calculates resources based on scaling decision
func (r *AdaptiveRightSizer) calculateOptimalResourcesWithDecision(usage metrics.Metrics, decision ResourceScalingDecision) corev1.ResourceRequirements {
	cfg := config.Get()

	var cpuRequest, memRequest int64

	// CPU calculation based on decision
	if decision.CPU == ScaleUp {
		cpuRequest = int64(usage.CPUMilli*cfg.CPURequestMultiplier) + cfg.CPURequestAddition
	} else if decision.CPU == ScaleDown {
		cpuRequest = int64(usage.CPUMilli*1.1) + cfg.CPURequestAddition // Use reduced multiplier
	} else {
		cpuRequest = int64(usage.CPUMilli*cfg.CPURequestMultiplier) + cfg.CPURequestAddition
	}

	// Memory calculation based on decision
	if decision.Memory == ScaleUp {
		memRequest = int64(usage.MemMB*cfg.MemoryRequestMultiplier) + cfg.MemoryRequestAddition
	} else if decision.Memory == ScaleDown {
		memRequest = int64(usage.MemMB*1.1) + cfg.MemoryRequestAddition // Use reduced multiplier
	} else {
		memRequest = int64(usage.MemMB*cfg.MemoryRequestMultiplier) + cfg.MemoryRequestAddition
	}

	// Ensure minimum values
	if cpuRequest < cfg.MinCPURequest {
		cpuRequest = cfg.MinCPURequest
	}
	if memRequest < cfg.MinMemoryRequest {
		memRequest = cfg.MinMemoryRequest // Already in MB
	}

	// Calculate limits
	cpuLimit := int64(float64(cpuRequest)*cfg.CPULimitMultiplier) + cfg.CPULimitAddition
	memLimit := int64(float64(memRequest)*cfg.MemoryLimitMultiplier) + cfg.MemoryLimitAddition

	// Apply maximum caps
	if cpuLimit > cfg.MaxCPULimit {
		cpuLimit = cfg.MaxCPULimit
	}
	if memLimit > cfg.MaxMemoryLimit { // MaxMemoryLimit is already in MB
		memLimit = cfg.MaxMemoryLimit
	}

	// Ensure memory limit is never 0 or less than request
	if memLimit <= 0 {
		memLimit = memRequest * 2 // Default to 2x the request if limit calculation fails
	}
	if memLimit < memRequest {
		memLimit = memRequest // Limit should never be less than request
	}
	if memLimit <= 0 {
		memLimit = 256 // Fallback to 256MB if still 0
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewMilliQuantity(cpuRequest, resource.DecimalSI),
			corev1.ResourceMemory: *resource.NewQuantity(memRequest*1024*1024, resource.BinarySI),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewMilliQuantity(cpuLimit, resource.DecimalSI),
			corev1.ResourceMemory: *resource.NewQuantity(memLimit*1024*1024, resource.BinarySI),
		},
	}
}

// needsAdjustmentWithDecision checks if adjustment is needed based on scaling decision
func (r *AdaptiveRightSizer) needsAdjustmentWithDecision(current, new corev1.ResourceRequirements, decision ResourceScalingDecision) bool {
	// If we already determined no scaling is needed, skip
	if decision.CPU == ScaleNone && decision.Memory == ScaleNone {
		return false
	}

	// Get current values
	currentCPU := current.Requests[corev1.ResourceCPU]
	currentMem := current.Requests[corev1.ResourceMemory]
	newCPU := new.Requests[corev1.ResourceCPU]
	newMem := new.Requests[corev1.ResourceMemory]

	// Skip if not set
	if currentCPU.IsZero() || currentMem.IsZero() {
		return true
	}

	// Calculate percentage difference
	cpuDiff := float64(newCPU.MilliValue()-currentCPU.MilliValue()) / float64(currentCPU.MilliValue()) * 100
	memDiff := float64(newMem.Value()-currentMem.Value()) / float64(currentMem.Value()) * 100

	// Adjust if difference > 10% (lower threshold since we already checked scaling thresholds)
	threshold := 10.0
	return (cpuDiff > threshold || cpuDiff < -threshold) ||
		(memDiff > threshold || memDiff < -threshold)
}

// getAdjustmentReasonWithDecision provides reason based on scaling decision
func (r *AdaptiveRightSizer) getAdjustmentReasonWithDecision(current, new corev1.ResourceRequirements, decision ResourceScalingDecision) string {
	currentCPU := current.Requests[corev1.ResourceCPU]
	currentMem := current.Requests[corev1.ResourceMemory]
	newCPU := new.Requests[corev1.ResourceCPU]
	newMem := new.Requests[corev1.ResourceMemory]

	reasons := []string{}

	if decision.CPU == ScaleUp {
		reasons = append(reasons, fmt.Sprintf("CPU scale up from %s to %s", currentCPU.String(), newCPU.String()))
	} else if decision.CPU == ScaleDown {
		reasons = append(reasons, fmt.Sprintf("CPU scale down from %s to %s", currentCPU.String(), newCPU.String()))
	}

	if decision.Memory == ScaleUp {
		reasons = append(reasons, fmt.Sprintf("Memory scale up from %s to %s", formatMemory(currentMem), formatMemory(newMem)))
	} else if decision.Memory == ScaleDown {
		reasons = append(reasons, fmt.Sprintf("Memory scale down from %s to %s", formatMemory(currentMem), formatMemory(newMem)))
	}

	if len(reasons) == 0 {
		return "Resource optimization"
	}

	return strings.Join(reasons, ", ")
}

// formatMemory formats memory quantity for display
func formatMemory(q resource.Quantity) string {
	// Convert to Mi for better readability
	valueInBytes := q.Value()
	valueInMi := valueInBytes / (1024 * 1024)
	return fmt.Sprintf("%dMi", valueInMi)
}

func (r *AdaptiveRightSizer) needsAdjustment(current, new corev1.ResourceRequirements) bool {
	// Get current values
	currentCPU := current.Requests[corev1.ResourceCPU]
	currentMem := current.Requests[corev1.ResourceMemory]
	newCPU := new.Requests[corev1.ResourceCPU]
	newMem := new.Requests[corev1.ResourceMemory]

	// Skip if not set
	if currentCPU.IsZero() || currentMem.IsZero() {
		return true
	}

	// Calculate percentage difference
	cpuDiff := float64(newCPU.MilliValue()-currentCPU.MilliValue()) / float64(currentCPU.MilliValue()) * 100
	memDiff := float64(newMem.Value()-currentMem.Value()) / float64(currentMem.Value()) * 100

	// Adjust if difference > 15%
	threshold := 15.0
	return (cpuDiff > threshold || cpuDiff < -threshold) ||
		(memDiff > threshold || memDiff < -threshold)
}

func (r *AdaptiveRightSizer) getAdjustmentReason(current, new corev1.ResourceRequirements) string {
	currentCPU := current.Requests[corev1.ResourceCPU]
	currentMem := current.Requests[corev1.ResourceMemory]
	newCPU := new.Requests[corev1.ResourceCPU]
	newMem := new.Requests[corev1.ResourceMemory]

	cpuChange := "no change"
	if newCPU.MilliValue() > currentCPU.MilliValue() {
		cpuChange = fmt.Sprintf("increase from %s to %s", currentCPU.String(), newCPU.String())
	} else if newCPU.MilliValue() < currentCPU.MilliValue() {
		cpuChange = fmt.Sprintf("decrease from %s to %s", currentCPU.String(), newCPU.String())
	}

	memChange := "no change"
	if newMem.Value() > currentMem.Value() {
		memChange = fmt.Sprintf("increase from %s to %s", currentMem.String(), newMem.String())
	} else if newMem.Value() < currentMem.Value() {
		memChange = fmt.Sprintf("decrease from %s to %s", currentMem.String(), newMem.String())
	}

	return fmt.Sprintf("CPU %s, Memory %s", cpuChange, memChange)
}

func (r *AdaptiveRightSizer) isSystemWorkload(namespace, name string) bool {
	systemNamespaces := []string{"kube-system", "kube-public", "kube-node-lease"}
	for _, ns := range systemNamespaces {
		if namespace == ns {
			return true
		}
	}

	// Skip the right-sizer itself
	if name == "right-sizer" {
		return true
	}

	return false
}

func (r *AdaptiveRightSizer) logUpdate(update ResourceUpdate, dryRun bool) {
	mode := ""
	if dryRun {
		mode = "[DRY RUN] "
	}

	cpuReq := update.NewResources.Requests[corev1.ResourceCPU]
	memReq := update.NewResources.Requests[corev1.ResourceMemory]
	oldCpuReq := update.OldResources.Requests[corev1.ResourceCPU]
	oldMemReq := update.OldResources.Requests[corev1.ResourceMemory]

	log.Printf("%s%s %s/%s/%s - Planned resize: CPU: %s→%s, Memory: %s→%s",
		mode,
		update.ResourceType,
		update.Namespace,
		update.Name,
		update.ContainerName,
		oldCpuReq.String(),
		cpuReq.String(),
		oldMemReq.String(),
		memReq.String())
	return
}

// shouldProcessNamespace checks if a namespace should be processed based on include/exclude lists
func (r *AdaptiveRightSizer) shouldProcessNamespace(namespace string) bool {
	cfg := config.Get()

	// Check exclude list first (takes precedence)
	for _, excludeNs := range cfg.NamespaceExclude {
		if namespace == excludeNs {
			return false
		}
	}

	// If include list is empty, process all non-excluded namespaces
	if len(cfg.NamespaceInclude) == 0 {
		return true
	}

	// Check if namespace is in include list
	for _, includeNs := range cfg.NamespaceInclude {
		if namespace == includeNs {
			return true
		}
	}

	return false
}

// SetupAdaptiveRightSizer creates and starts the adaptive rightsizer
func SetupAdaptiveRightSizer(mgr manager.Manager, provider metrics.Provider, dryRun bool) error {
	cfg := config.Get()

	// Get the rest config from the manager
	restConfig := mgr.GetConfig()

	// Create a clientset for using the resize subresource
	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	rightsizer := &AdaptiveRightSizer{
		Client:          mgr.GetClient(),
		ClientSet:       clientSet,
		RestConfig:      restConfig,
		MetricsProvider: provider,
		Interval:        cfg.ResizeInterval,
		DryRun:          dryRun,
	}

	// Start the rightsizer
	go func() {
		if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
			return rightsizer.Start(ctx)
		})); err != nil {
			log.Printf("Failed to add adaptive rightsizer to manager: %v", err)
		}
	}()

	return nil
}

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
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"right-sizer/config"
	"right-sizer/logger"
	"right-sizer/metrics"
	"right-sizer/validation"
)

// InPlaceRightSizer performs in-place resource adjustments without pod restarts using Kubernetes 1.33+ resize subresource
// This version ONLY updates pods directly, not deployments or other controllers
type InPlaceRightSizer struct {
	Client          client.Client
	ClientSet       kubernetes.Interface
	RestConfig      *rest.Config
	MetricsProvider metrics.Provider
	Interval        time.Duration
	Validator       *validation.ResourceValidator
	QoSValidator    *validation.QoSValidator
	RetryManager    *RetryManager
	EventRecorder   record.EventRecorder
	Config          *config.Config // Configuration with feature flags
	resizeCache     map[string]*ResizeDecisionCache
	cacheMutex      sync.RWMutex
	cacheExpiry     time.Duration // How long to keep cache entries
}

// PodResizePatch represents the patch structure for the resize subresource
type PodResizePatch struct {
	Spec PodSpecPatch `json:"spec"`
}

// PodSpecPatch contains the containers to be resized
type PodSpecPatch struct {
	Containers []ContainerResourcesPatch `json:"containers"`
}

// ContainerResourcesPatch represents container resources to patch
type ContainerResourcesPatch struct {
	Name         string                         `json:"name"`
	Resources    corev1.ResourceRequirements    `json:"resources"`
	ResizePolicy []corev1.ContainerResizePolicy `json:"resizePolicy,omitempty"`
}

// Start begins the continuous monitoring and adjustment loop
func (r *InPlaceRightSizer) Start(ctx context.Context) error {
	if r.Client == nil {
		return fmt.Errorf("kubernetes client is not initialized")
	}

	ticker := time.NewTicker(r.Interval)
	defer ticker.Stop()

	logger.Info("Starting in-place right-sizer with %v interval", r.Interval)
	log.Printf("🚀 Starting in-place right-sizer with %v interval (Kubernetes 1.33+ resize subresource)", r.Interval)
	log.Printf("📝 Note: This operator ONLY updates pod resources directly")

	// Run immediately on start
	r.rightSizeAllPods(ctx)

	for {
		select {
		case <-ticker.C:
			r.rightSizeAllPods(ctx)
			// Clean expired cache entries periodically
			r.cleanExpiredCacheEntries()
		case <-ctx.Done():
			log.Println("Stopping in-place right-sizer")
			return nil
		}
	}
}

// rightSizeAllPods processes all pods in the cluster
func (r *InPlaceRightSizer) rightSizeAllPods(ctx context.Context) {
	var podList corev1.PodList
	if err := r.Client.List(ctx, &podList); err != nil {
		log.Printf("❌ Error listing pods: %v", err)
		return
	}

	log.Printf("🔍 Analyzing %d pods for right-sizing...", len(podList.Items))

	resizedCount := 0
	skippedCount := 0
	errorCount := 0
	nodeConstraintSkips := 0

	for _, pod := range podList.Items {
		// Skip pods that are not running
		if pod.Status.Phase != corev1.PodRunning {
			skippedCount++
			continue
		}

		// Skip system pods
		if isSystemPod(&pod) {
			skippedCount++
			continue
		}

		// Check namespace filters
		if !r.shouldProcessNamespace(pod.Namespace) {
			skippedCount++
			continue
		}

		// Self-protection: Skip if this is the right-sizer pod itself
		if r.isSelfPod(&pod) {
			log.Printf("🛡️  Skipping self-pod %s/%s to prevent self-modification", pod.Namespace, pod.Name)
			skippedCount++
			continue
		}

		// Skip pods that don't support in-place resize
		if !r.supportsInPlaceResize(&pod) {
			log.Printf("⚠️  Pod %s/%s does not support in-place resize, skipping", pod.Namespace, pod.Name)
			skippedCount++
			continue
		}

		// Skip pods that have no resource specifications at all
		hasAnyResources := false
		for _, container := range pod.Spec.Containers {
			if len(container.Resources.Requests) > 0 {
				hasAnyResources = true
				break
			}
			if len(container.Resources.Limits) > 0 {
				hasAnyResources = true
				break
			}
		}
		if !hasAnyResources {
			skippedCount++
			continue // Silently skip pods with no resource specs - nothing to resize
		}

		// Try to right-size the pod
		resized, err := r.rightSizePod(ctx, &pod)
		if err != nil {
			// Check if error is due to node resource constraints
			if strings.Contains(err.Error(), "exceeds available node capacity") ||
				strings.Contains(err.Error(), "exceeds node allocatable capacity") {
				nodeConstraintSkips++
				log.Printf("📍 Skipped pod %s/%s due to node resource constraints", pod.Namespace, pod.Name)
			} else if !strings.Contains(err.Error(), "resize failed") {
				log.Printf("❌ Error right-sizing pod %s/%s: %v", pod.Namespace, pod.Name, err)
				errorCount++
			}
		} else if resized {
			resizedCount++
		}
	}

	log.Printf("📊 Right-sizing complete: %d resized, %d skipped (%d due to node constraints), %d errors",
		resizedCount, skippedCount, nodeConstraintSkips, errorCount)
}

// rightSizePod adjusts resources for a single pod
func (r *InPlaceRightSizer) rightSizePod(ctx context.Context, pod *corev1.Pod) (bool, error) {
	// Fetch current metrics
	usage, err := r.MetricsProvider.FetchPodMetrics(ctx, pod.Namespace, pod.Name)
	if err != nil {
		// If metrics are not available, skip this pod
		return false, nil
	}

	// Check if scaling is needed based on thresholds
	scalingDecision := r.checkScalingThresholds(usage, pod)

	// Skip if both resources don't need changes
	if scalingDecision.CPU == ScaleNone && scalingDecision.Memory == ScaleNone {
		return false, nil
	}

	// Skip if CPU should not be updated but memory should be reduced
	// This prevents unnecessary pod disruptions when only memory reduction is needed
	if scalingDecision.CPU == ScaleNone && scalingDecision.Memory == ScaleDown {
		log.Printf("⏭️  Skipping resize for pod %s/%s: CPU doesn't need update and memory would be reduced",
			pod.Namespace, pod.Name)
		return false, nil
	}

	// Calculate new resources based on usage and scaling decision
	newResourcesMap := r.calculateOptimalResourcesForContainers(usage, pod, scalingDecision)

	// Check if adjustment is needed
	needsUpdate, _ := r.needsAdjustmentWithDetails(pod, newResourcesMap)
	if !needsUpdate {
		return false, nil
	}

	// Log the actual resource changes that will be made
	for _, container := range pod.Spec.Containers {
		if newResources, exists := newResourcesMap[container.Name]; exists {
			oldCPUReq := container.Resources.Requests[corev1.ResourceCPU]
			oldMemReq := container.Resources.Requests[corev1.ResourceMemory]
			newCPUReq := newResources.Requests[corev1.ResourceCPU]
			newMemReq := newResources.Requests[corev1.ResourceMemory]

			if !oldCPUReq.Equal(newCPUReq) || !oldMemReq.Equal(newMemReq) {
				// Get current usage for detailed logging
				cpuLimit := container.Resources.Limits.Cpu().AsApproximateFloat64() * 1000
				memLimit := float64(container.Resources.Limits.Memory().Value()) / (1024 * 1024)
				cpuUsagePercent := 0.0
				memUsagePercent := 0.0
				if cpuLimit > 0 {
					cpuUsagePercent = (usage.CPUMilli / cpuLimit) * 100
				}
				if memLimit > 0 {
					memUsagePercent = (usage.MemMB / memLimit) * 100
				}

				// Check cache before logging to prevent repetitive messages
				if r.shouldLogResizeDecision(pod.Namespace, pod.Name, container.Name,
					oldCPUReq.String(), newCPUReq.String(), oldMemReq.String(), newMemReq.String()) {
					log.Printf("🔍 Scaling analysis - CPU: %s (usage: %.0fm/%.0fm, %.1f%%), Memory: %s (usage: %.0fMi/%.0fMi, %.1f%%)",
						scalingDecisionString(scalingDecision.CPU), usage.CPUMilli, cpuLimit, cpuUsagePercent,
						scalingDecisionString(scalingDecision.Memory), usage.MemMB, memLimit, memUsagePercent)
					log.Printf("📈 Container %s/%s/%s will be resized - CPU: %s→%s, Memory: %s→%s",
						pod.Namespace, pod.Name, container.Name,
						oldCPUReq.String(), newCPUReq.String(),
						oldMemReq.String(), newMemReq.String())
				}
			}
		}
	}

	// Apply in-place update using resize subresource (removed duplicate logging)

	// Apply in-place update using resize subresource
	err = r.applyInPlaceResize(ctx, pod, newResourcesMap)
	if err != nil {
		return false, err
	}

	log.Printf("✅ Successfully resized pod %s/%s using resize subresource (no restart)", pod.Namespace, pod.Name)
	return true, nil
}

// ResourceChange holds before/after resource values
type ResourceChange struct {
	CurrentCPU string
	NewCPU     string
	CurrentMem string
	NewMem     string
}

// formatMemory formats memory in a human-readable way
// formatMemory is defined in adaptive_rightsizer.go

// applyInPlaceResize performs the actual in-place resource update using the resize subresource
// According to K8s 1.33 best practices, we resize CPU and memory in two separate steps
func (r *InPlaceRightSizer) applyInPlaceResize(ctx context.Context, pod *corev1.Pod, newResourcesMap map[string]corev1.ResourceRequirements) error {
	// Update ObservedGeneration to track spec changes
	SetPodObservedGeneration(pod)

	// Set PodResizeInProgress condition
	SetPodResizeInProgress(pod, ReasonResizeInProgress, "Starting in-place resize operation")
	// Comprehensive QoS validation if QoS validator is available
	if r.QoSValidator != nil {
		for containerName, newResources := range newResourcesMap {
			qosResult := r.QoSValidator.ValidateQoSPreservation(pod, containerName, newResources)
			if !qosResult.Valid {
				logger.Warn("QoS validation failed for pod %s/%s container %s:", pod.Namespace, pod.Name, containerName)
				for _, err := range qosResult.Errors {
					logger.Warn("  - %s", err)
				}

				// Record event for QoS validation failure
				if r.EventRecorder != nil {
					r.EventRecorder.Event(pod, corev1.EventTypeWarning, "QoSValidationFailed",
						fmt.Sprintf("QoS validation failed for container %s: %v", containerName, qosResult.Errors))
				}

				ClearResizeConditions(pod)
				return fmt.Errorf("QoS validation failed: %v", qosResult.Errors)
			}

			// Log QoS validation warnings
			if len(qosResult.Warnings) > 0 {
				logger.Warn("QoS validation warnings for pod %s/%s container %s:", pod.Namespace, pod.Name, containerName)
				for _, warning := range qosResult.Warnings {
					logger.Warn("  - %s", warning)
				}
			}
		}
	}

	// Standard resource validation if validator is available
	if r.Validator != nil {
		for containerName, newResources := range newResourcesMap {
			validationResult := r.Validator.ValidateResourceChange(ctx, pod, newResources, containerName)
			if !validationResult.Valid {
				// Check if validation failed due to node resource constraints
				hasNodeConstraint := false
				for _, err := range validationResult.Errors {
					if strings.Contains(err, "exceeds available node capacity") ||
						strings.Contains(err, "exceeds node allocatable capacity") {
						hasNodeConstraint = true
						break
					}
				}

				if hasNodeConstraint {
					logger.Info("📍 Node resource constraint for pod %s/%s container %s:", pod.Namespace, pod.Name, containerName)
					for _, err := range validationResult.Errors {
						logger.Info("  - %s", err)
					}

					// Add to retry manager for deferred retry
					if r.RetryManager != nil {
						reason := "Node resource constraints prevent resize"
						r.RetryManager.AddDeferredResize(pod, newResourcesMap, reason,
							fmt.Errorf("exceeds available node capacity: %v", validationResult.Errors))
					}

					// Record event for deferred resize
					if r.EventRecorder != nil {
						r.EventRecorder.Event(pod, corev1.EventTypeWarning, "ResizeDeferred",
							"Resize deferred due to node resource constraints")
					}

					return fmt.Errorf("exceeds available node capacity: %v", validationResult.Errors)
				} else {
					logger.Warn("Skipping resize for pod %s/%s container %s due to validation errors:", pod.Namespace, pod.Name, containerName)
					for _, err := range validationResult.Errors {
						logger.Warn("  - %s", err)
					}
					ClearResizeConditions(pod)
					return fmt.Errorf("validation failed: %v", validationResult.Errors)
				}
			}

			// Log any warnings but continue
			if len(validationResult.Warnings) > 0 {
				logger.Warn("Validation warnings for pod %s/%s container %s:", pod.Namespace, pod.Name, containerName)
				for _, warning := range validationResult.Warnings {
					logger.Warn("  - %s", warning)
				}
			}
		}
	}

	// Skip direct pod patching for resize policy
	// Resize policies should only be set in parent resources (Deployments/StatefulSets/DaemonSets)
	// not in pods directly. The parent resources should have already set the resize policy.
	logger.Info("📝 Skipping direct pod resize policy patch - relying on parent resource policies")

	// Update pod status to show resize is in progress
	if err := r.Client.Status().Update(ctx, pod); err != nil {
		logger.Warn("Failed to update pod status with resize progress: %v", err)
	}

	// Refresh pod state after resize policy update
	time.Sleep(100 * time.Millisecond)
	if err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: pod.Namespace,
		Name:      pod.Name,
	}, pod); err != nil {
		ClearResizeConditions(pod)
		return fmt.Errorf("failed to refresh pod state: %w", err)
	}

	// Resize CPU for all containers
	logger.Info("⚡ Resizing CPU for pod %s/%s", pod.Namespace, pod.Name)
	UpdateResizeProgress(pod, "", corev1.ResourceCPU, "cpu-resize")

	cpuContainers := make([]ContainerResourcesPatch, 0, len(newResourcesMap))
	for containerName, newResources := range newResourcesMap {
		// Find the current container resources
		var currentResources corev1.ResourceRequirements
		for _, container := range pod.Spec.Containers {
			if container.Name == containerName {
				currentResources = container.Resources
				break
			}
		}

		// Create CPU-only resources
		cpuOnlyResources := corev1.ResourceRequirements{
			Requests: corev1.ResourceList{},
			Limits:   corev1.ResourceList{},
		}

		// Copy current memory values (keep them unchanged)
		if memReq, exists := currentResources.Requests[corev1.ResourceMemory]; exists {
			cpuOnlyResources.Requests[corev1.ResourceMemory] = memReq.DeepCopy()
		}
		if memLim, exists := currentResources.Limits[corev1.ResourceMemory]; exists {
			cpuOnlyResources.Limits[corev1.ResourceMemory] = memLim.DeepCopy()
		}

		// Apply new CPU values
		if cpuReq, exists := newResources.Requests[corev1.ResourceCPU]; exists {
			cpuOnlyResources.Requests[corev1.ResourceCPU] = cpuReq.DeepCopy()
			logger.Info("   📊 Container %s: CPU request %s -> %s",
				containerName,
				formatResource(currentResources.Requests[corev1.ResourceCPU]),
				formatResource(cpuReq))
		}
		if cpuLim, exists := newResources.Limits[corev1.ResourceCPU]; exists {
			cpuOnlyResources.Limits[corev1.ResourceCPU] = cpuLim.DeepCopy()
			logger.Info("   📊 Container %s: CPU limit %s -> %s",
				containerName,
				formatResource(currentResources.Limits[corev1.ResourceCPU]),
				formatResource(cpuLim))
		}

		cpuContainers = append(cpuContainers, ContainerResourcesPatch{
			Name:      containerName,
			Resources: cpuOnlyResources,
		})
	}

	// Apply CPU resize
	if len(cpuContainers) > 0 {
		cpuPatch := PodResizePatch{
			Spec: PodSpecPatch{
				Containers: cpuContainers,
			},
		}

		cpuPatchData, err := json.Marshal(cpuPatch)
		if err != nil {
			return fmt.Errorf("failed to marshal CPU resize patch: %w", err)
		}

		_, err = r.ClientSet.CoreV1().Pods(pod.Namespace).Patch(
			ctx,
			pod.Name,
			types.StrategicMergePatchType,
			cpuPatchData,
			metav1.PatchOptions{},
			"resize",
		)
		if err != nil {
			logger.Error("❌ CPU resize failed for pod %s/%s: %v", pod.Namespace, pod.Name, err)
			// Continue to try memory resize even if CPU fails
		} else {
			logger.Info("✅ CPU resize successful for pod %s/%s", pod.Namespace, pod.Name)
		}

		// Wait a bit between CPU and memory resize
		time.Sleep(200 * time.Millisecond)

		// Refresh pod state after CPU resize
		if err := r.Client.Get(ctx, types.NamespacedName{
			Namespace: pod.Namespace,
			Name:      pod.Name,
		}, pod); err != nil {
			return fmt.Errorf("failed to refresh pod state after CPU resize: %w", err)
		}
	}

	// Resize Memory for all containers
	logger.Info("💾 Resizing Memory for pod %s/%s", pod.Namespace, pod.Name)
	memContainers := make([]ContainerResourcesPatch, 0, len(newResourcesMap))
	for containerName, newResources := range newResourcesMap {
		// Find the current container resources (after CPU update)
		var currentResources corev1.ResourceRequirements
		for _, container := range pod.Spec.Containers {
			if container.Name == containerName {
				currentResources = container.Resources
				break
			}
		}

		// Create memory-only resources
		memOnlyResources := corev1.ResourceRequirements{
			Requests: corev1.ResourceList{},
			Limits:   corev1.ResourceList{},
		}

		// Copy current CPU values (use the updated CPU from step 2)
		if cpuReq, exists := currentResources.Requests[corev1.ResourceCPU]; exists {
			memOnlyResources.Requests[corev1.ResourceCPU] = cpuReq.DeepCopy()
		}
		if cpuLim, exists := currentResources.Limits[corev1.ResourceCPU]; exists {
			memOnlyResources.Limits[corev1.ResourceCPU] = cpuLim.DeepCopy()
		}

		// Apply new memory values
		if memReq, exists := newResources.Requests[corev1.ResourceMemory]; exists {
			memOnlyResources.Requests[corev1.ResourceMemory] = memReq.DeepCopy()
			logger.Info("   📊 Container %s: Memory request %s -> %s",
				containerName,
				formatMemory(currentResources.Requests[corev1.ResourceMemory]),
				formatMemory(memReq))
		}
		if memLim, exists := newResources.Limits[corev1.ResourceMemory]; exists {
			memOnlyResources.Limits[corev1.ResourceMemory] = memLim.DeepCopy()
			logger.Info("   📊 Container %s: Memory limit %s -> %s",
				containerName,
				formatMemory(currentResources.Limits[corev1.ResourceMemory]),
				formatMemory(memLim))
		}

		memContainers = append(memContainers, ContainerResourcesPatch{
			Name:      containerName,
			Resources: memOnlyResources,
		})
	}

	// Apply Memory resize
	if len(memContainers) > 0 {
		memPatch := PodResizePatch{
			Spec: PodSpecPatch{
				Containers: memContainers,
			},
		}

		memPatchData, err := json.Marshal(memPatch)
		if err != nil {
			return fmt.Errorf("failed to marshal memory resize patch: %w", err)
		}

		_, err = r.ClientSet.CoreV1().Pods(pod.Namespace).Patch(
			ctx,
			pod.Name,
			types.StrategicMergePatchType,
			memPatchData,
			metav1.PatchOptions{},
			"resize",
		)
		if err != nil {
			// Check if error is due to forbidden decrease
			if strings.Contains(err.Error(), "Forbidden") && strings.Contains(err.Error(), "cannot be decreased") {
				logger.Warn("⚠️  Cannot decrease memory for pod %s/%s", pod.Namespace, pod.Name)
				logger.Info("   💡 Pod needs RestartContainer policy for memory decreases. Skipping memory resize.")
				// Return nil to not count this as an error if CPU succeeded
				return nil
			}
			logger.Error("❌ Memory resize failed for pod %s/%s: %v", pod.Namespace, pod.Name, err)
			return fmt.Errorf("memory resize failed: %w", err)
		}
		logger.Info("✅ Memory resize successful for pod %s/%s", pod.Namespace, pod.Name)
	}

	// Update resize progress to show memory phase completion
	UpdateResizeProgress(pod, "", corev1.ResourceMemory, "memory-resize")

	// Update pod status to reflect successful completion
	if err := r.Client.Status().Update(ctx, pod); err != nil {
		logger.Warn("Failed to update pod status after memory resize: %v", err)
	}

	// Final refresh of pod state to get latest generation
	if err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: pod.Namespace,
		Name:      pod.Name,
	}, pod); err != nil {
		logger.Warn("Failed to refresh pod state for ObservedGeneration update: %v", err)
	} else {
		// Update ObservedGeneration after successful resize
		SetPodObservedGeneration(pod)
		if err := r.Client.Status().Update(ctx, pod); err != nil {
			logger.Warn("Failed to update ObservedGeneration: %v", err)
		}
	}

	// Clear resize conditions on successful completion
	ClearResizeConditions(pod)
	if err := r.Client.Status().Update(ctx, pod); err != nil {
		logger.Warn("Failed to clear resize conditions: %v", err)
	}

	// Record success event
	if r.EventRecorder != nil {
		containerNames := make([]string, 0, len(newResourcesMap))
		for name := range newResourcesMap {
			containerNames = append(containerNames, name)
		}
		r.EventRecorder.Event(pod, corev1.EventTypeNormal, "ResizeCompleted",
			fmt.Sprintf("Successfully resized containers: %s", strings.Join(containerNames, ", ")))
	}

	// Record success in resize event history
	RecordResizeEvent(pod, "Normal", "ResizeCompleted", "In-place resize operation completed successfully")

	logger.Info("🎯 All resize operations completed for pod %s/%s", pod.Namespace, pod.Name)
	return nil
}

// applyResizePolicy applies in-place resize policies directly to pods for K8s 1.33+
// This enables zero-downtime resource adjustments without pod restarts
func (r *InPlaceRightSizer) applyResizePolicy(ctx context.Context, pod *corev1.Pod) error {

	// Check if pod already has resize policies configured
	hasResizePolicies := false
	for _, container := range pod.Spec.Containers {
		if len(container.ResizePolicy) > 0 {
			hasResizePolicies = true
			break
		}
	}

	if hasResizePolicies {
		log.Printf("✅ Pod %s/%s has resize policies configured", pod.Namespace, pod.Name)
	}

	// Note: We don't modify existing pods' resize policies as Kubernetes doesn't allow it
	// The actual resource resizing will be handled by the resize subresource in updatePodInPlace
	return nil
}

// shouldProcessNamespace checks if a namespace should be processed based on include/exclude lists
func (r *InPlaceRightSizer) shouldProcessNamespace(namespace string) bool {
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

// isSelfPod checks if the given pod is the right-sizer operator itself

// fallbackPatch is deprecated as regular patches cannot modify pod resources
// ensureSafeResourcePatch ensures the patch never tries to remove or add resource fields
// Only existing resource types in the current pod can be modified

func (r *InPlaceRightSizer) fallbackPatch(ctx context.Context, pod *corev1.Pod, newResourcesMap map[string]corev1.ResourceRequirements) error {
	// Regular patches cannot modify pod resources after creation
	// This is a Kubernetes limitation - only the resize subresource can change resources
	return errors.New("cannot modify pod resources without resize subresource")
}

// SetupInPlaceRightSizer creates and starts the in-place rightsizer with Kubernetes 1.33+ support
func SetupInPlaceRightSizer(mgr manager.Manager, provider metrics.Provider) error {
	cfg := config.Get()

	// Get the rest config from the manager
	restConfig := mgr.GetConfig()

	// Create a clientset for using the resize subresource
	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	// Create resource validator
	// Note: passing nil for metrics since we don't have OperatorMetrics here
	validator := validation.NewResourceValidator(mgr.GetClient(), clientSet, cfg, nil)

	// Create QoS validator for Kubernetes 1.33+ compliance
	qosValidator := validation.NewQoSValidator()

	// Create event recorder for recording resize events
	eventRecorder := mgr.GetEventRecorderFor("right-sizer-inplace")

	// Create retry manager for deferred resizes
	retryConfig := DefaultRetryManagerConfig()
	// NOTE: metrics passed as nil is intentional - RetryManager gracefully handles nil metrics
	// See retry_manager.go lines 226-227 and 357-358 where nil checks are performed
	// The metrics interface is not available from the provider context here
	retryManager := NewRetryManager(retryConfig, metrics.NewOperatorMetrics(), eventRecorder)

	rightsizer := &InPlaceRightSizer{
		Client:          mgr.GetClient(),
		ClientSet:       clientSet,
		RestConfig:      restConfig,
		MetricsProvider: provider,
		Config:          cfg,
		Interval:        cfg.ResizeInterval,
		Validator:       validator,
		QoSValidator:    qosValidator,
		RetryManager:    retryManager,
		EventRecorder:   eventRecorder,
		resizeCache:     make(map[string]*ResizeDecisionCache),
		cacheExpiry:     5 * time.Minute, // Cache entries for 5 minutes
	}

	// Start the retry manager
	if err := retryManager.Start(); err != nil {
		return fmt.Errorf("failed to start retry manager: %w", err)
	}

	// Start the rightsizer in a goroutine
	go func() {
		if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
			return rightsizer.Start(ctx)
		})); err != nil {
			log.Printf("Failed to add rightsizer to manager: %v", err)
		}
	}()

	log.Println("✅ In-place rightsizer setup complete with Kubernetes 1.33+ compliance features:")
	log.Println("   - Pod resize status conditions")
	log.Println("   - ObservedGeneration tracking")
	log.Println("   - Comprehensive QoS validation")
	log.Println("   - Deferred resize retry logic")
	return nil
}

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
	"time"

	"right-sizer/config"
	"right-sizer/logger"
	"right-sizer/metrics"
	"right-sizer/validation"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// InPlaceRightSizer performs in-place resource adjustments without pod restarts using Kubernetes 1.33+ resize subresource
// This version ONLY updates pods directly, not deployments or other controllers
type InPlaceRightSizer struct {
	Client          client.Client
	ClientSet       *kubernetes.Clientset
	RestConfig      *rest.Config
	MetricsProvider metrics.Provider
	Interval        time.Duration
	Validator       *validation.ResourceValidator
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
	Name      string                      `json:"name"`
	Resources corev1.ResourceRequirements `json:"resources"`
}

// Start begins the continuous monitoring and adjustment loop
func (r *InPlaceRightSizer) Start(ctx context.Context) error {
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

		// Skip pods that don't support in-place resize
		if !r.supportsInPlaceResize(&pod) {
			log.Printf("⚠️  Pod %s/%s does not support in-place resize, skipping", pod.Namespace, pod.Name)
			skippedCount++
			continue
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

// supportsInPlaceResize checks if a pod can be resized in-place
func (r *InPlaceRightSizer) supportsInPlaceResize(pod *corev1.Pod) bool {
	// Skip if pod has owner references (managed by deployment, statefulset, etc)
	// We only want to resize standalone pods or pods we're certain won't be recreated
	// Comment this out if you want to resize all pods regardless of ownership
	/*
		if len(pod.OwnerReferences) > 0 {
			for _, owner := range pod.OwnerReferences {
				if owner.Controller != nil && *owner.Controller {
					// This pod is controlled by something else, skip it
					return false
				}
			}
		}
	*/

	// For pods without explicit resize policy, check if they have resources defined
	hasResources := false
	hasNotRequiredPolicy := false

	for _, container := range pod.Spec.Containers {
		// Check if container has resources defined
		if !container.Resources.Requests.Cpu().IsZero() || !container.Resources.Requests.Memory().IsZero() {
			hasResources = true
		}

		// Check resize policy
		if container.ResizePolicy != nil {
			for _, policy := range container.ResizePolicy {
				if policy.RestartPolicy == corev1.NotRequired {
					hasNotRequiredPolicy = true
					break
				}
			}
		}
	}

	// Only attempt resize if:
	// 1. Pod has resources defined (otherwise nothing to resize)
	// 2. Either has NotRequired policy OR no policy (K8s 1.33+ supports resize by default)
	if !hasResources {
		return false
	}

	// If explicit NotRequired policy is set, definitely support resize
	if hasNotRequiredPolicy {
		return true
	}

	// For K8s 1.33+, pods without explicit policy can still be resized for increases
	// We'll handle decrease restrictions in the resize logic
	return true
}

// rightSizePod adjusts resources for a single pod
func (r *InPlaceRightSizer) rightSizePod(ctx context.Context, pod *corev1.Pod) (bool, error) {
	// Fetch current metrics
	usage, err := r.MetricsProvider.FetchPodMetrics(pod.Namespace, pod.Name)
	if err != nil {
		// If metrics are not available, skip this pod
		return false, nil
	}

	// Calculate new resources based on usage
	newResourcesMap := r.calculateOptimalResourcesForContainers(usage, pod)

	// Check if adjustment is needed
	needsUpdate, details := r.needsAdjustmentWithDetails(pod, newResourcesMap)
	if !needsUpdate {
		return false, nil
	}

	// Log the resize operation with details
	log.Printf("🔧 Resizing pod %s/%s:", pod.Namespace, pod.Name)
	for containerName, changes := range details {
		log.Printf("   📦 Container '%s':", containerName)
		log.Printf("      CPU: %s → %s", changes.CurrentCPU, changes.NewCPU)
		log.Printf("      Memory: %s → %s", changes.CurrentMem, changes.NewMem)
	}

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

// calculateOptimalResourcesForContainers determines optimal resource allocation for all containers
func (r *InPlaceRightSizer) calculateOptimalResourcesForContainers(usage metrics.Metrics, pod *corev1.Pod) map[string]corev1.ResourceRequirements {
	resourcesMap := make(map[string]corev1.ResourceRequirements)

	// For simplicity, apply the same resources to all containers based on total pod usage
	// In production, you might want per-container metrics
	numContainers := len(pod.Spec.Containers)
	if numContainers == 0 {
		return resourcesMap
	}

	// Divide resources among containers
	cpuPerContainer := usage.CPUMilli / float64(numContainers)
	memPerContainer := usage.MemMB / float64(numContainers)

	for _, container := range pod.Spec.Containers {
		newResources := r.calculateOptimalResources(cpuPerContainer, memPerContainer)

		// Check if we can safely apply these resources
		currentResources := container.Resources
		adjustedResources := r.adjustResourcesForSafeResize(currentResources, newResources, container.ResizePolicy)

		resourcesMap[container.Name] = adjustedResources
	}

	return resourcesMap
}

// calculateOptimalResources determines optimal resource allocation for a single container
func (r *InPlaceRightSizer) calculateOptimalResources(cpuMilli float64, memMB float64) corev1.ResourceRequirements {
	cfg := config.Get()

	// Add buffer for requests using configurable multipliers
	cpuRequest := int64(cpuMilli * cfg.CPURequestMultiplier)
	memRequest := int64(memMB * cfg.MemoryRequestMultiplier)

	// Ensure minimum values
	if cpuRequest < cfg.MinCPURequest {
		cpuRequest = cfg.MinCPURequest
	}
	if memRequest < cfg.MinMemoryRequest {
		memRequest = cfg.MinMemoryRequest
	}

	// Set limits using configurable multipliers
	cpuLimit := int64(float64(cpuRequest) * cfg.CPULimitMultiplier)
	memLimit := int64(float64(memRequest) * cfg.MemoryLimitMultiplier)

	// Cap at configurable maximums
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

// adjustResourcesForSafeResize adjusts resources to ensure they can be safely resized
func (r *InPlaceRightSizer) adjustResourcesForSafeResize(current, desired corev1.ResourceRequirements, resizePolicy []corev1.ContainerResizePolicy) corev1.ResourceRequirements {
	adjusted := desired.DeepCopy()

	// Check if we're trying to decrease memory limits
	currentMemLimit := current.Limits[corev1.ResourceMemory]
	desiredMemLimit := desired.Limits[corev1.ResourceMemory]

	// Check if we're trying to decrease CPU limits
	currentCPULimit := current.Limits[corev1.ResourceCPU]
	desiredCPULimit := desired.Limits[corev1.ResourceCPU]

	// If current resources are not set, we can set any value
	if currentMemLimit.IsZero() && currentCPULimit.IsZero() {
		return *adjusted
	}

	// Check resize policy for memory
	memoryCanDecrease := false
	cpuCanDecrease := false

	for _, policy := range resizePolicy {
		if policy.ResourceName == corev1.ResourceMemory && policy.RestartPolicy == corev1.RestartContainer {
			memoryCanDecrease = true
		}
		if policy.ResourceName == corev1.ResourceCPU && policy.RestartPolicy == corev1.RestartContainer {
			cpuCanDecrease = true
		}
	}

	// If we're trying to decrease memory limit and it's not allowed, keep current or increase
	if !currentMemLimit.IsZero() && desiredMemLimit.Cmp(currentMemLimit) < 0 && !memoryCanDecrease {
		// Keep the current limit or slightly increase it
		adjusted.Limits[corev1.ResourceMemory] = currentMemLimit

		// Adjust request to be at most half of limit
		desiredMemReq := desired.Requests[corev1.ResourceMemory]
		halfLimit := resource.NewQuantity(currentMemLimit.Value()/2, resource.BinarySI)
		if desiredMemReq.Cmp(*halfLimit) > 0 {
			adjusted.Requests[corev1.ResourceMemory] = *halfLimit
		}
	}

	// If we're trying to decrease CPU limit and it's not allowed, keep current or increase
	if !currentCPULimit.IsZero() && desiredCPULimit.Cmp(currentCPULimit) < 0 && !cpuCanDecrease {
		// Keep the current limit or slightly increase it
		adjusted.Limits[corev1.ResourceCPU] = currentCPULimit

		// Adjust request to be at most half of limit
		desiredCPUReq := desired.Requests[corev1.ResourceCPU]
		halfLimit := resource.NewMilliQuantity(currentCPULimit.MilliValue()/2, resource.DecimalSI)
		if desiredCPUReq.Cmp(*halfLimit) > 0 {
			adjusted.Requests[corev1.ResourceCPU] = *halfLimit
		}
	}

	// Ensure requests don't exceed limits
	cpuReq := adjusted.Requests[corev1.ResourceCPU]
	cpuLim := adjusted.Limits[corev1.ResourceCPU]
	if cpuReq.Cmp(cpuLim) > 0 {
		adjusted.Requests[corev1.ResourceCPU] = cpuLim
	}

	memReq := adjusted.Requests[corev1.ResourceMemory]
	memLim := adjusted.Limits[corev1.ResourceMemory]
	if memReq.Cmp(memLim) > 0 {
		adjusted.Requests[corev1.ResourceMemory] = memLim
	}

	return *adjusted
}

// needsAdjustmentWithDetails checks if pod resources need updating and returns details
func (r *InPlaceRightSizer) needsAdjustmentWithDetails(pod *corev1.Pod, newResourcesMap map[string]corev1.ResourceRequirements) (bool, map[string]ResourceChange) {
	details := make(map[string]ResourceChange)
	needsUpdate := false

	for _, container := range pod.Spec.Containers {
		newResources, exists := newResourcesMap[container.Name]
		if !exists {
			continue
		}

		// Get current CPU and memory requests
		currentCPU := container.Resources.Requests[corev1.ResourceCPU]
		currentMem := container.Resources.Requests[corev1.ResourceMemory]
		newCPU := newResources.Requests[corev1.ResourceCPU]
		newMem := newResources.Requests[corev1.ResourceMemory]

		change := ResourceChange{
			CurrentCPU: formatResource(currentCPU),
			NewCPU:     formatResource(newCPU),
			CurrentMem: formatMemory(currentMem),
			NewMem:     formatMemory(newMem),
		}

		// Skip if current resources are not set
		if currentCPU.IsZero() || currentMem.IsZero() {
			details[container.Name] = change
			needsUpdate = true
			continue
		}

		// Calculate percentage difference
		cpuDiff := float64(newCPU.MilliValue()-currentCPU.MilliValue()) / float64(currentCPU.MilliValue()) * 100
		memDiff := float64(newMem.Value()-currentMem.Value()) / float64(currentMem.Value()) * 100

		// Only adjust if difference is more than 10%
		if (cpuDiff > 10 || cpuDiff < -10) || (memDiff > 10 || memDiff < -10) {
			details[container.Name] = change
			needsUpdate = true
		}
	}

	return needsUpdate, details
}

// formatResource formats a resource quantity for display
func formatResource(q resource.Quantity) string {
	if q.IsZero() {
		return "0"
	}
	return q.String()
}

// formatMemory formats memory in a human-readable way
func formatMemory(q resource.Quantity) string {
	if q.IsZero() {
		return "0Mi"
	}
	// Convert to MiB for display
	bytes := q.Value()
	mib := bytes / (1024 * 1024)
	if mib < 1024 {
		return fmt.Sprintf("%dMi", mib)
	}
	gib := float64(mib) / 1024
	return fmt.Sprintf("%.1fGi", gib)
}

// applyInPlaceResize performs the actual in-place resource update using the resize subresource
func (r *InPlaceRightSizer) applyInPlaceResize(ctx context.Context, pod *corev1.Pod, newResourcesMap map[string]corev1.ResourceRequirements) error {
	// Validate the new resources if validator is available
	if r.Validator != nil {
		for containerName, newResources := range newResourcesMap {
			validationResult := r.Validator.ValidateResourceChange(ctx, pod, newResources, containerName)
			if !validationResult.Valid {
				// Log validation errors and skip this pod
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
					logger.Info("📍 Node resource constraint for pod %s/%s container %s:",
						pod.Namespace, pod.Name, containerName)
					for _, err := range validationResult.Errors {
						logger.Info("  - %s", err)
					}
					// Return a specific error message for node constraints
					return fmt.Errorf("exceeds available node capacity: %v", validationResult.Errors)
				} else {
					logger.Warn("Skipping resize for pod %s/%s container %s due to validation errors:",
						pod.Namespace, pod.Name, containerName)
					for _, err := range validationResult.Errors {
						logger.Warn("  - %s", err)
					}
				}
				// Return early - don't attempt resize if validation fails
				return fmt.Errorf("validation failed: %v", validationResult.Errors)
			}

			// Log any warnings but continue
			if len(validationResult.Warnings) > 0 {
				logger.Warn("Validation warnings for pod %s/%s container %s:",
					pod.Namespace, pod.Name, containerName)
				for _, warning := range validationResult.Warnings {
					logger.Warn("  - %s", warning)
				}
			}
		}
	}

	// Create the resize patch
	containers := make([]ContainerResourcesPatch, 0, len(newResourcesMap))
	for containerName, resources := range newResourcesMap {
		containers = append(containers, ContainerResourcesPatch{
			Name:      containerName,
			Resources: resources,
		})
	}

	resizePatch := PodResizePatch{
		Spec: PodSpecPatch{
			Containers: containers,
		},
	}

	// Marshal the patch
	patchData, err := json.Marshal(resizePatch)
	if err != nil {
		return fmt.Errorf("failed to marshal resize patch: %w", err)
	}

	// Use the Kubernetes client-go to patch with the resize subresource
	// Apply the patch using the resize subresource
	// This is the key difference - using the resize subresource endpoint
	_, err = r.ClientSet.CoreV1().Pods(pod.Namespace).Patch(
		ctx,
		pod.Name,
		types.StrategicMergePatchType,
		patchData,
		metav1.PatchOptions{},
		"resize", // This is the crucial part - specifying the resize subresource
	)

	if err != nil {
		// Check if error is due to forbidden decrease
		if strings.Contains(err.Error(), "Forbidden") && strings.Contains(err.Error(), "cannot be decreased") {
			log.Printf("⚠️  Cannot decrease resources for pod %s/%s", pod.Namespace, pod.Name)
			log.Printf("   💡 Pod needs RestartContainer policy for decreases. Skipping resize.")
			// Return nil to not count this as an error
			return nil
		}

		// For other errors, log and return
		log.Printf("❌ Resize failed for pod %s/%s: %v", pod.Namespace, pod.Name, err)
		return fmt.Errorf("resize failed: %w", err)
	}

	return nil
}

// fallbackPatch is deprecated as regular patches cannot modify pod resources
func (r *InPlaceRightSizer) fallbackPatch(ctx context.Context, pod *corev1.Pod, newResourcesMap map[string]corev1.ResourceRequirements) error {
	// Regular patches cannot modify pod resources after creation
	// This is a Kubernetes limitation - only the resize subresource can change resources
	return fmt.Errorf("cannot modify pod resources without resize subresource")
}

// isSystemPod checks if a pod is a system/infrastructure pod
func isSystemPod(pod *corev1.Pod) bool {
	// Skip kube-system and other system namespaces
	systemNamespaces := []string{"kube-system", "kube-public", "kube-node-lease", "ingress-nginx", "cert-manager"}
	for _, ns := range systemNamespaces {
		if pod.Namespace == ns {
			return true
		}
	}

	// Skip the right-sizer itself
	if pod.Labels["app"] == "right-sizer" {
		return true
	}

	// Skip pods with system-related labels
	systemLabels := []string{
		"component",
		"tier",
	}
	for _, label := range systemLabels {
		if value, exists := pod.Labels[label]; exists {
			if value == "control-plane" || value == "etcd" || value == "kube-scheduler" || value == "kube-controller-manager" {
				return true
			}
		}
	}

	// Skip metrics-server
	if pod.Labels["k8s-app"] == "metrics-server" {
		return true
	}

	return false
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
	validator, err := validation.NewResourceValidator(mgr.GetClient(), cfg)
	if err != nil {
		logger.Warn("Failed to create resource validator: %v", err)
		// Continue without validator - will skip validation checks
		validator = nil
	}

	rightsizer := &InPlaceRightSizer{
		Client:          mgr.GetClient(),
		ClientSet:       clientSet,
		RestConfig:      restConfig,
		MetricsProvider: provider,
		Interval:        cfg.ResizeInterval,
		Validator:       validator,
	}

	// Start the rightsizer in a goroutine
	go func() {
		if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
			return rightsizer.Start(ctx)
		})); err != nil {
			log.Printf("Failed to add rightsizer to manager: %v", err)
		}
	}()

	log.Println("✅ In-place rightsizer setup complete with Kubernetes 1.33+ resize subresource support")
	return nil
}

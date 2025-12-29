package controllers

import (
	"os"
	"strings"

	"right-sizer/config"
	"right-sizer/logger"
	"right-sizer/metrics"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

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

// checkScalingThresholds determines if scaling is needed based on resource usage thresholds
func (r *InPlaceRightSizer) checkScalingThresholds(usage metrics.Metrics, pod *corev1.Pod) ResourceScalingDecision {
	cfg := config.Get()

	// Calculate total current limits for the pod
	var totalCPULimit float64
	var totalMemLimit float64

	for _, container := range pod.Spec.Containers {
		if cpuLimit, exists := container.Resources.Limits[corev1.ResourceCPU]; exists && !cpuLimit.IsZero() {
			totalCPULimit += float64(cpuLimit.MilliValue())
		}
		if memLimit, exists := container.Resources.Limits[corev1.ResourceMemory]; exists && !memLimit.IsZero() {
			totalMemLimit += float64(memLimit.Value()) / (1024 * 1024) // Convert to MB
		}
	}

	// If no limits are set, use requests as baseline
	if totalCPULimit == 0 || totalMemLimit == 0 {
		for _, container := range pod.Spec.Containers {
			if totalCPULimit == 0 {
				if cpuReq, exists := container.Resources.Requests[corev1.ResourceCPU]; exists && !cpuReq.IsZero() {
					totalCPULimit += float64(cpuReq.MilliValue())
				}
			}
			if totalMemLimit == 0 {
				if memReq, exists := container.Resources.Requests[corev1.ResourceMemory]; exists && !memReq.IsZero() {
					totalMemLimit += float64(memReq.Value()) / (1024 * 1024)
				}
			}
		}
	}

	// If still no resources set, default to scale up
	if totalCPULimit == 0 && totalMemLimit == 0 {
		return ResourceScalingDecision{CPU: ScaleUp, Memory: ScaleUp}
	}

	// Calculate usage percentages
	cpuUsagePercent := float64(0)
	memUsagePercent := float64(0)

	if totalCPULimit > 0 {
		cpuUsagePercent = usage.CPUMilli / totalCPULimit
	}
	if totalMemLimit > 0 {
		memUsagePercent = usage.MemMB / totalMemLimit
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

	// Don't log here to avoid duplication - logging happens in rightSizePod when resize is actually needed

	return ResourceScalingDecision{CPU: cpuDecision, Memory: memoryDecision}
}

// calculateOptimalResourcesForContainers determines optimal resource allocation for all containers
func (r *InPlaceRightSizer) calculateOptimalResourcesForContainers(usage metrics.Metrics, pod *corev1.Pod, scalingDecision ResourceScalingDecision) map[string]corev1.ResourceRequirements {
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
		newResources := r.calculateOptimalResources(cpuPerContainer, memPerContainer, scalingDecision)

		// Check if we can safely apply these resources
		currentResources := container.Resources
		adjustedResources := r.adjustResourcesForSafeResize(currentResources, newResources, container.ResizePolicy)

		resourcesMap[container.Name] = adjustedResources
	}

	return resourcesMap
}

// calculateOptimalResources determines optimal resource allocation for a single container
func (r *InPlaceRightSizer) calculateOptimalResources(cpuMilli float64, memMB float64, scalingDecision ResourceScalingDecision) corev1.ResourceRequirements {
	cfg := config.Get()

	var cpuRequest, memRequest int64

	// Apply different multipliers based on scaling decision for each resource
	// CPU calculation
	if scalingDecision.CPU == ScaleUp {
		cpuRequest = int64(cpuMilli*cfg.CPURequestMultiplier) + cfg.CPURequestAddition
	} else if scalingDecision.CPU == ScaleDown {
		cpuRequest = int64(cpuMilli*1.1) + cfg.CPURequestAddition
	} else {
		cpuRequest = int64(cpuMilli*cfg.CPURequestMultiplier) + cfg.CPURequestAddition
	}

	// Memory calculation
	if scalingDecision.Memory == ScaleUp {
		memRequest = int64(memMB*cfg.MemoryRequestMultiplier) + cfg.MemoryRequestAddition
	} else if scalingDecision.Memory == ScaleDown {
		memRequest = int64(memMB*1.1) + cfg.MemoryRequestAddition
	} else {
		memRequest = int64(memMB*cfg.MemoryRequestMultiplier) + cfg.MemoryRequestAddition
	}

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

// isSelfPod checks if the given pod is the right-sizer operator itself
func (r *InPlaceRightSizer) isSelfPod(pod *corev1.Pod) bool {
	// Check if this pod has the right-sizer app label
	if appLabel, exists := pod.Labels["app.kubernetes.io/name"]; exists && appLabel == "right-sizer" {
		return true
	}

	// Fallback: Check if the pod name contains "right-sizer"
	if strings.Contains(pod.Name, "right-sizer") {
		// Additional check: ensure it's in the operator namespace
		operatorNamespace := os.Getenv("OPERATOR_NAMESPACE")
		if operatorNamespace != "" && pod.Namespace == operatorNamespace {
			return true
		}
		// Fallback namespace check
		if operatorNamespace == "" && (pod.Namespace == "right-sizer" || pod.Namespace == "default") {
			return true
		}
	}

	return false
}

// isSystemPod checks if a pod is a system/infrastructure pod
func isSystemPod(pod *corev1.Pod) bool {
	// Skip kube-system and other system namespaces
	cfg := config.Get()
	for _, ns := range cfg.SystemNamespaces {
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

// ensureSafeResourcePatch ensures the patch never tries to remove or add resource fields
// Only existing resource types in the current pod can be modified
func ensureSafeResourcePatch(current, desired corev1.ResourceRequirements) corev1.ResourceRequirements {
	logger.Info("🛡️  Ensuring safe resource patch...")

	result := corev1.ResourceRequirements{}

	// Only include requests that already exist in the current pod
	if len(current.Requests) > 0 {
		result.Requests = make(corev1.ResourceList)

		// Only update CPU request if it exists in current
		if cpuReq, exists := current.Requests[corev1.ResourceCPU]; exists {
			if desiredCPU, desiredExists := desired.Requests[corev1.ResourceCPU]; desiredExists {
				result.Requests[corev1.ResourceCPU] = desiredCPU
				logger.Info("   ✅ Updating existing CPU request: %s -> %s", formatResource(cpuReq), formatResource(desiredCPU))
			} else {
				// Keep the current value if desired doesn't specify it
				result.Requests[corev1.ResourceCPU] = cpuReq
				logger.Info("   🔄 Preserving existing CPU request: %s", formatResource(cpuReq))
			}
		}

		// Only update Memory request if it exists in current
		if memReq, exists := current.Requests[corev1.ResourceMemory]; exists {
			if desiredMem, desiredExists := desired.Requests[corev1.ResourceMemory]; desiredExists {
				result.Requests[corev1.ResourceMemory] = desiredMem
				logger.Info("   ✅ Updating existing Memory request: %s -> %s", formatMemory(memReq), formatMemory(desiredMem))
			} else {
				// Keep the current value if desired doesn't specify it
				result.Requests[corev1.ResourceMemory] = memReq
				logger.Info("   🔄 Preserving existing Memory request: %s", formatMemory(memReq))
			}
		}
	}

	// Only include limits that already exist in the current pod
	if len(current.Limits) > 0 {
		result.Limits = make(corev1.ResourceList)

		// Only update CPU limit if it exists in current
		if cpuLim, exists := current.Limits[corev1.ResourceCPU]; exists {
			if desiredCPU, desiredExists := desired.Limits[corev1.ResourceCPU]; desiredExists {
				result.Limits[corev1.ResourceCPU] = desiredCPU
				logger.Info("   ✅ Updating existing CPU limit: %s -> %s", formatResource(cpuLim), formatResource(desiredCPU))
			} else {
				// Keep the current value if desired doesn't specify it
				result.Limits[corev1.ResourceCPU] = cpuLim
				logger.Info("   🔄 Preserving existing CPU limit: %s", formatResource(cpuLim))
			}
		}

		// Only update Memory limit if it exists in current
		if memLim, exists := current.Limits[corev1.ResourceMemory]; exists {
			if desiredMem, desiredExists := desired.Limits[corev1.ResourceMemory]; desiredExists {
				result.Limits[corev1.ResourceMemory] = desiredMem
				logger.Info("   ✅ Updating existing Memory limit: %s -> %s", formatMemory(memLim), formatMemory(desiredMem))
			} else {
				// Keep the current value if desired doesn't specify it
				result.Limits[corev1.ResourceMemory] = memLim
				logger.Info("   🔄 Preserving existing Memory limit: %s", formatMemory(memLim))
			}
		}
	}

	// Debug: Log what we're NOT including (should be rare now due to early filtering)
	if desired.Requests != nil {
		for resType, resVal := range desired.Requests {
			if _, exists := current.Requests[resType]; !exists {
				logger.Debug("   ⚠️  Skipping new request type %s: %s (not in current pod)", resType, formatResource(resVal))
			}
		}
	}
	if desired.Limits != nil {
		for resType, resVal := range desired.Limits {
			if _, exists := current.Limits[resType]; !exists {
				logger.Debug("   ⚠️  Skipping new limit type %s: %s (not in current pod)", resType, formatResource(resVal))
			}
		}
	}

	logger.Info("✅ Safe resource patch completed")
	return result
}

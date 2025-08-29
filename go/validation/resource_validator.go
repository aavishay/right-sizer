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

package validation

import (
	"context"
	"fmt"
	"strings"

	"right-sizer/config"
	"right-sizer/logger"
	"right-sizer/metrics"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ValidationResult represents the result of a resource validation
type ValidationResult struct {
	Valid    bool
	Errors   []string
	Warnings []string
	Info     []string
}

// IsValid returns true if the validation passed
func (vr *ValidationResult) IsValid() bool {
	return vr.Valid
}

// HasWarnings returns true if there are warnings
func (vr *ValidationResult) HasWarnings() bool {
	return len(vr.Warnings) > 0
}

// AddError adds an error to the validation result
func (vr *ValidationResult) AddError(msg string) {
	vr.Errors = append(vr.Errors, msg)
	vr.Valid = false
}

// AddWarning adds a warning to the validation result
func (vr *ValidationResult) AddWarning(msg string) {
	vr.Warnings = append(vr.Warnings, msg)
}

// AddInfo adds an info message to the validation result
func (vr *ValidationResult) AddInfo(msg string) {
	vr.Info = append(vr.Info, msg)
}

// String returns a string representation of the validation result
func (vr *ValidationResult) String() string {
	var parts []string

	if len(vr.Errors) > 0 {
		parts = append(parts, fmt.Sprintf("Errors: %s", strings.Join(vr.Errors, "; ")))
	}
	if len(vr.Warnings) > 0 {
		parts = append(parts, fmt.Sprintf("Warnings: %s", strings.Join(vr.Warnings, "; ")))
	}
	if len(vr.Info) > 0 {
		parts = append(parts, fmt.Sprintf("Info: %s", strings.Join(vr.Info, "; ")))
	}

	return strings.Join(parts, " | ")
}

// ResourceValidator validates resource requests and limits
type ResourceValidator struct {
	client      client.Client
	clientset   *kubernetes.Clientset
	config      *config.Config
	metrics     *metrics.OperatorMetrics
	nodeCache   map[string]*corev1.Node
	quotaCache  map[string]*corev1.ResourceQuota
	limitRanges map[string][]*corev1.LimitRange
}

// NewResourceValidator creates a new resource validator
func NewResourceValidator(client client.Client, clientset *kubernetes.Clientset, cfg *config.Config, metrics *metrics.OperatorMetrics) *ResourceValidator {
	return &ResourceValidator{
		client:      client,
		clientset:   clientset,
		config:      cfg,
		metrics:     metrics,
		nodeCache:   make(map[string]*corev1.Node),
		quotaCache:  make(map[string]*corev1.ResourceQuota),
		limitRanges: make(map[string][]*corev1.LimitRange),
	}
}

// ValidateResourceChange validates a proposed resource change for a pod
func (rv *ResourceValidator) ValidateResourceChange(ctx context.Context, pod *corev1.Pod, newResources corev1.ResourceRequirements, containerName string) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// Basic resource validation
	rv.validateBasicResources(newResources, result)

	// Configuration limits validation
	rv.validateConfigurationLimits(newResources, result)

	// Safety threshold validation
	if pod.Spec.Containers != nil {
		for _, container := range pod.Spec.Containers {
			if container.Name == containerName {
				rv.validateSafetyThreshold(container.Resources, newResources, result)
				break
			}
		}
	}

	// Node capacity validation
	rv.validateNodeCapacity(ctx, pod, newResources, result)

	// Resource quota validation
	rv.validateResourceQuota(ctx, pod, newResources, result)

	// Limit range validation
	rv.validateLimitRanges(ctx, pod, newResources, result)

	// QoS class validation
	rv.validateQoSImpact(pod, newResources, containerName, result)

	// Record metrics
	if rv.metrics != nil {
		if !result.IsValid() {
			for _, err := range result.Errors {
				rv.metrics.RecordResourceValidationError("resource_change", err)
			}
		}
	}

	return result
}

// validateBasicResources performs basic validation on resource requests and limits
func (rv *ResourceValidator) validateBasicResources(resources corev1.ResourceRequirements, result *ValidationResult) {
	// Check that requests don't exceed limits
	if resources.Requests != nil && resources.Limits != nil {
		if cpuRequest, ok := resources.Requests[corev1.ResourceCPU]; ok {
			if cpuLimit, ok := resources.Limits[corev1.ResourceCPU]; ok {
				if cpuRequest.Cmp(cpuLimit) > 0 {
					result.AddError("CPU request cannot exceed CPU limit")
				}
			}
		}

		if memRequest, ok := resources.Requests[corev1.ResourceMemory]; ok {
			if memLimit, ok := resources.Limits[corev1.ResourceMemory]; ok {
				if memRequest.Cmp(memLimit) > 0 {
					result.AddError("Memory request cannot exceed memory limit")
				}
			}
		}
	}

	// Check for negative values
	if resources.Requests != nil {
		for resourceName, quantity := range resources.Requests {
			if quantity.Sign() < 0 {
				result.AddError(fmt.Sprintf("Resource request for %s cannot be negative", resourceName))
			}
		}
	}

	if resources.Limits != nil {
		for resourceName, quantity := range resources.Limits {
			if quantity.Sign() < 0 {
				result.AddError(fmt.Sprintf("Resource limit for %s cannot be negative", resourceName))
			}
		}
	}
}

// validateConfigurationLimits validates against configured min/max limits
func (rv *ResourceValidator) validateConfigurationLimits(resources corev1.ResourceRequirements, result *ValidationResult) {
	if resources.Requests != nil {
		if cpuRequest, ok := resources.Requests[corev1.ResourceCPU]; ok {
			cpuMillis := cpuRequest.MilliValue()
			if cpuMillis < rv.config.MinCPURequest {
				result.AddError(fmt.Sprintf("CPU request %dm is below minimum %dm", cpuMillis, rv.config.MinCPURequest))
			}
		}

		if memRequest, ok := resources.Requests[corev1.ResourceMemory]; ok {
			memMB := memRequest.Value() / (1024 * 1024)
			if memMB < rv.config.MinMemoryRequest {
				result.AddError(fmt.Sprintf("Memory request %dMB is below minimum %dMB", memMB, rv.config.MinMemoryRequest))
			}
		}
	}

	if resources.Limits != nil {
		if cpuLimit, ok := resources.Limits[corev1.ResourceCPU]; ok {
			cpuMillis := cpuLimit.MilliValue()
			if cpuMillis > rv.config.MaxCPULimit {
				result.AddError(fmt.Sprintf("CPU limit %dm exceeds maximum %dm", cpuMillis, rv.config.MaxCPULimit))
			}
		}

		if memLimit, ok := resources.Limits[corev1.ResourceMemory]; ok {
			memMB := memLimit.Value() / (1024 * 1024)
			if memMB > rv.config.MaxMemoryLimit {
				result.AddError(fmt.Sprintf("Memory limit %dMB exceeds maximum %dMB", memMB, rv.config.MaxMemoryLimit))
			}
		}
	}
}

// validateSafetyThreshold checks if the change is within safety limits
func (rv *ResourceValidator) validateSafetyThreshold(current, new corev1.ResourceRequirements, result *ValidationResult) {
	// Check CPU request change
	if current.Requests != nil && new.Requests != nil {
		if currentCPU, ok := current.Requests[corev1.ResourceCPU]; ok {
			if newCPU, ok := new.Requests[corev1.ResourceCPU]; ok {
				if !rv.config.IsChangeWithinSafetyThreshold(currentCPU.MilliValue(), newCPU.MilliValue()) {
					if rv.metrics != nil {
						rv.metrics.RecordSafetyThresholdViolation("", "", "cpu")
					}
					result.AddWarning(fmt.Sprintf("CPU request change exceeds safety threshold of %.0f%%", rv.config.SafetyThreshold*100))
				}
			}
		}
	}

	// Check memory request change
	if current.Requests != nil && new.Requests != nil {
		if currentMem, ok := current.Requests[corev1.ResourceMemory]; ok {
			if newMem, ok := new.Requests[corev1.ResourceMemory]; ok {
				currentMB := currentMem.Value() / (1024 * 1024)
				newMB := newMem.Value() / (1024 * 1024)
				if !rv.config.IsChangeWithinSafetyThreshold(currentMB, newMB) {
					if rv.metrics != nil {
						rv.metrics.RecordSafetyThresholdViolation("", "", "memory")
					}
					result.AddWarning(fmt.Sprintf("Memory request change exceeds safety threshold of %.0f%%", rv.config.SafetyThreshold*100))
				}
			}
		}
	}
}

// calculateNodeAvailableResources calculates the available resources on a node
func (rv *ResourceValidator) calculateNodeAvailableResources(ctx context.Context, node *corev1.Node, excludePod *corev1.Pod) (corev1.ResourceList, error) {
	// Start with allocatable resources (total minus system reserved)
	availableCPU := node.Status.Allocatable[corev1.ResourceCPU].DeepCopy()
	availableMemory := node.Status.Allocatable[corev1.ResourceMemory].DeepCopy()

	// List all pods on the node
	podList := &corev1.PodList{}
	if err := rv.client.List(ctx, podList, client.MatchingFields{"spec.nodeName": node.Name}); err != nil {
		return nil, fmt.Errorf("failed to list pods on node %s: %v", node.Name, err)
	}

	// Subtract resources used by all running pods (except the pod being resized)
	for _, p := range podList.Items {
		// Skip the pod being resized (we'll add its new resources later)
		if excludePod != nil && p.Namespace == excludePod.Namespace && p.Name == excludePod.Name {
			continue
		}

		// Only count pods that are running or pending
		if p.Status.Phase != corev1.PodRunning && p.Status.Phase != corev1.PodPending {
			continue
		}

		// Sum up all container requests
		for _, container := range p.Spec.Containers {
			if cpuReq, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
				availableCPU.Sub(cpuReq)
			}
			if memReq, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
				availableMemory.Sub(memReq)
			}
		}
	}

	return corev1.ResourceList{
		corev1.ResourceCPU:    availableCPU,
		corev1.ResourceMemory: availableMemory,
	}, nil
}

// validateNodeCapacity checks if the new resources fit within available node capacity
func (rv *ResourceValidator) validateNodeCapacity(ctx context.Context, pod *corev1.Pod, resources corev1.ResourceRequirements, result *ValidationResult) {
	if pod.Spec.NodeName == "" {
		result.AddInfo("Pod not yet scheduled, skipping node capacity validation")
		return
	}

	node, err := rv.getNode(ctx, pod.Spec.NodeName)
	if err != nil {
		result.AddWarning(fmt.Sprintf("Could not retrieve node %s for capacity validation: %v", pod.Spec.NodeName, err))
		return
	}

	// Calculate available resources on the node (excluding current pod)
	availableResources, err := rv.calculateNodeAvailableResources(ctx, node, pod)
	if err != nil {
		result.AddWarning(fmt.Sprintf("Could not calculate available resources on node %s: %v", pod.Spec.NodeName, err))
		// Fall back to simple capacity check
		rv.validateAgainstTotalCapacity(node, resources, result)
		return
	}

	// Check if new resources fit within available capacity
	if resources.Requests != nil {
		if cpuRequest, ok := resources.Requests[corev1.ResourceCPU]; ok {
			availableCPU := availableResources[corev1.ResourceCPU]
			if cpuRequest.Cmp(availableCPU) > 0 {
				allocatableCPU := node.Status.Allocatable[corev1.ResourceCPU]
				result.AddError(fmt.Sprintf("CPU request %s exceeds available node capacity %s (allocatable: %s)",
					cpuRequest.String(),
					(&availableCPU).String(),
					(&allocatableCPU).String()))
			}
		}

		if memRequest, ok := resources.Requests[corev1.ResourceMemory]; ok {
			availableMemory := availableResources[corev1.ResourceMemory]
			if memRequest.Cmp(availableMemory) > 0 {
				allocatableMemory := node.Status.Allocatable[corev1.ResourceMemory]
				result.AddError(fmt.Sprintf("Memory request %s exceeds available node capacity %s (allocatable: %s)",
					memRequest.String(),
					(&availableMemory).String(),
					(&allocatableMemory).String()))
			}
		}
	}

	// Also validate limits against allocatable (limits can't exceed allocatable)
	if resources.Limits != nil {
		if cpuLimit, ok := resources.Limits[corev1.ResourceCPU]; ok {
			allocatableCPU := node.Status.Allocatable[corev1.ResourceCPU]
			if cpuLimit.Cmp(allocatableCPU) > 0 {
				result.AddError(fmt.Sprintf("CPU limit %s exceeds node allocatable capacity %s",
					cpuLimit.String(),
					(&allocatableCPU).String()))
			}
		}

		if memLimit, ok := resources.Limits[corev1.ResourceMemory]; ok {
			allocatableMemory := node.Status.Allocatable[corev1.ResourceMemory]
			if memLimit.Cmp(allocatableMemory) > 0 {
				result.AddError(fmt.Sprintf("Memory limit %s exceeds node allocatable capacity %s",
					memLimit.String(),
					(&allocatableMemory).String()))
			}
		}
	}
}

// validateAgainstTotalCapacity fallback validation against total capacity
func (rv *ResourceValidator) validateAgainstTotalCapacity(node *corev1.Node, resources corev1.ResourceRequirements, result *ValidationResult) {
	if resources.Requests != nil {
		if cpuRequest, ok := resources.Requests[corev1.ResourceCPU]; ok {
			nodeCPUCapacity := node.Status.Allocatable[corev1.ResourceCPU]
			if cpuRequest.Cmp(nodeCPUCapacity) > 0 {
				result.AddError(fmt.Sprintf("CPU request %s exceeds node allocatable capacity %s", cpuRequest.String(), (&nodeCPUCapacity).String()))
			}
		}

		if memRequest, ok := resources.Requests[corev1.ResourceMemory]; ok {
			nodeMemCapacity := node.Status.Allocatable[corev1.ResourceMemory]
			if memRequest.Cmp(nodeMemCapacity) > 0 {
				result.AddError(fmt.Sprintf("Memory request %s exceeds node allocatable capacity %s", memRequest.String(), (&nodeMemCapacity).String()))
			}
		}
	}
}

// validateResourceQuota checks if the resource change violates resource quotas
func (rv *ResourceValidator) validateResourceQuota(ctx context.Context, pod *corev1.Pod, resources corev1.ResourceRequirements, result *ValidationResult) {
	quota, err := rv.getResourceQuota(ctx, pod.Namespace)
	if err != nil {
		logger.Debug("No resource quota found for namespace %s: %v", pod.Namespace, err)
		return
	}

	if quota.Status.Hard == nil {
		return
	}

	// Calculate current usage and new usage
	rv.checkQuotaLimits(quota, resources, result)
}

// validateLimitRanges checks if resources comply with limit ranges
func (rv *ResourceValidator) validateLimitRanges(ctx context.Context, pod *corev1.Pod, resources corev1.ResourceRequirements, result *ValidationResult) {
	limitRanges, err := rv.getLimitRanges(ctx, pod.Namespace)
	if err != nil {
		logger.Debug("Could not retrieve limit ranges for namespace %s: %v", pod.Namespace, err)
		return
	}

	for _, limitRange := range limitRanges {
		rv.checkLimitRangeCompliance(limitRange, resources, result)
	}
}

// validateQoSImpact checks if the resource change affects QoS class
func (rv *ResourceValidator) validateQoSImpact(pod *corev1.Pod, newResources corev1.ResourceRequirements, containerName string, result *ValidationResult) {
	currentQoS := getQoSClass(pod)

	// Create a copy of the pod with new resources to determine new QoS
	podCopy := pod.DeepCopy()
	for i, container := range podCopy.Spec.Containers {
		if container.Name == containerName {
			podCopy.Spec.Containers[i].Resources = newResources
			break
		}
	}

	newQoS := getQoSClass(podCopy)

	if currentQoS != newQoS {
		result.AddWarning(fmt.Sprintf("Resource change will modify QoS class from %s to %s", currentQoS, newQoS))
	}
}

// getNode retrieves a node from cache or API
func (rv *ResourceValidator) getNode(ctx context.Context, nodeName string) (*corev1.Node, error) {
	if node, ok := rv.nodeCache[nodeName]; ok {
		return node, nil
	}

	node := &corev1.Node{}
	err := rv.client.Get(ctx, client.ObjectKey{Name: nodeName}, node)
	if err != nil {
		return nil, err
	}

	rv.nodeCache[nodeName] = node
	return node, nil
}

// getResourceQuota retrieves resource quota for a namespace
func (rv *ResourceValidator) getResourceQuota(ctx context.Context, namespace string) (*corev1.ResourceQuota, error) {
	if quota, ok := rv.quotaCache[namespace]; ok {
		return quota, nil
	}

	quotaList := &corev1.ResourceQuotaList{}
	err := rv.client.List(ctx, quotaList, client.InNamespace(namespace))
	if err != nil {
		return nil, err
	}

	if len(quotaList.Items) == 0 {
		return nil, fmt.Errorf("no resource quota found")
	}

	// Use the first quota found
	quota := &quotaList.Items[0]
	rv.quotaCache[namespace] = quota
	return quota, nil
}

// getLimitRanges retrieves limit ranges for a namespace
func (rv *ResourceValidator) getLimitRanges(ctx context.Context, namespace string) ([]*corev1.LimitRange, error) {
	if ranges, ok := rv.limitRanges[namespace]; ok {
		return ranges, nil
	}

	limitRangeList := &corev1.LimitRangeList{}
	err := rv.client.List(ctx, limitRangeList, client.InNamespace(namespace))
	if err != nil {
		return nil, err
	}

	var ranges []*corev1.LimitRange
	for i := range limitRangeList.Items {
		ranges = append(ranges, &limitRangeList.Items[i])
	}

	rv.limitRanges[namespace] = ranges
	return ranges, nil
}

// checkQuotaLimits checks if resources comply with quota limits
func (rv *ResourceValidator) checkQuotaLimits(quota *corev1.ResourceQuota, resources corev1.ResourceRequirements, result *ValidationResult) {
	hard := quota.Status.Hard
	used := quota.Status.Used

	if resources.Requests != nil {
		if cpuRequest, ok := resources.Requests[corev1.ResourceCPU]; ok {
			if hardCPU, ok := hard["requests.cpu"]; ok {
				if usedCPU, ok := used["requests.cpu"]; ok {
					available := hardCPU.DeepCopy()
					available.Sub(usedCPU)
					if cpuRequest.Cmp(available) > 0 {
						result.AddError(fmt.Sprintf("CPU request %s exceeds available quota %s", cpuRequest.String(), (&available).String()))
					}
				}
			}
		}

		if memRequest, ok := resources.Requests[corev1.ResourceMemory]; ok {
			if hardMem, ok := hard["requests.memory"]; ok {
				if usedMem, ok := used["requests.memory"]; ok {
					available := hardMem.DeepCopy()
					available.Sub(usedMem)
					if memRequest.Cmp(available) > 0 {
						result.AddError(fmt.Sprintf("Memory request %s exceeds available quota %s", memRequest.String(), (&available).String()))
					}
				}
			}
		}
	}
}

// checkLimitRangeCompliance checks if resources comply with limit ranges
func (rv *ResourceValidator) checkLimitRangeCompliance(limitRange *corev1.LimitRange, resources corev1.ResourceRequirements, result *ValidationResult) {
	for _, limit := range limitRange.Spec.Limits {
		if limit.Type != corev1.LimitTypeContainer {
			continue
		}

		// Check minimum constraints
		if limit.Min != nil {
			rv.checkMinimumConstraints(limit.Min, resources, result)
		}

		// Check maximum constraints
		if limit.Max != nil {
			rv.checkMaximumConstraints(limit.Max, resources, result)
		}

		// Check default ratio constraints
		if limit.MaxLimitRequestRatio != nil {
			rv.checkRatioConstraints(limit.MaxLimitRequestRatio, resources, result)
		}
	}
}

// checkMinimumConstraints validates minimum resource constraints
func (rv *ResourceValidator) checkMinimumConstraints(min corev1.ResourceList, resources corev1.ResourceRequirements, result *ValidationResult) {
	if resources.Requests != nil {
		for resourceName, minQuantity := range min {
			if requestQuantity, ok := resources.Requests[resourceName]; ok {
				if requestQuantity.Cmp(minQuantity) < 0 {
					result.AddError(fmt.Sprintf("%s request %s is below minimum %s", resourceName, requestQuantity.String(), (&minQuantity).String()))
				}
			}
		}
	}
}

// checkMaximumConstraints validates maximum resource constraints
func (rv *ResourceValidator) checkMaximumConstraints(max corev1.ResourceList, resources corev1.ResourceRequirements, result *ValidationResult) {
	if resources.Limits != nil {
		for resourceName, maxQuantity := range max {
			if limitQuantity, ok := resources.Limits[resourceName]; ok {
				if limitQuantity.Cmp(maxQuantity) > 0 {
					result.AddError(fmt.Sprintf("%s limit %s exceeds maximum %s", resourceName, limitQuantity.String(), (&maxQuantity).String()))
				}
			}
		}
	}
}

// checkRatioConstraints validates limit-to-request ratio constraints
func (rv *ResourceValidator) checkRatioConstraints(maxRatio corev1.ResourceList, resources corev1.ResourceRequirements, result *ValidationResult) {
	if resources.Requests == nil || resources.Limits == nil {
		return
	}

	for resourceName, maxRatioQuantity := range maxRatio {
		request, hasRequest := resources.Requests[resourceName]
		limit, hasLimit := resources.Limits[resourceName]

		if hasRequest && hasLimit && !request.IsZero() {
			ratio := float64(limit.MilliValue()) / float64(request.MilliValue())
			maxAllowedRatio := float64(maxRatioQuantity.MilliValue()) / 1000.0 // Convert from milli

			if ratio > maxAllowedRatio {
				result.AddError(fmt.Sprintf("%s limit-to-request ratio %.2f exceeds maximum %.2f", resourceName, ratio, maxAllowedRatio))
			}
		}
	}
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

// ClearCaches clears all internal caches
func (rv *ResourceValidator) ClearCaches() {
	rv.nodeCache = make(map[string]*corev1.Node)
	rv.quotaCache = make(map[string]*corev1.ResourceQuota)
	rv.limitRanges = make(map[string][]*corev1.LimitRange)
}

// RefreshCaches refreshes all caches
func (rv *ResourceValidator) RefreshCaches(ctx context.Context) error {
	rv.ClearCaches()

	// Pre-populate node cache
	nodeList := &corev1.NodeList{}
	if err := rv.client.List(ctx, nodeList); err != nil {
		return fmt.Errorf("failed to list nodes: %v", err)
	}

	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		rv.nodeCache[node.Name] = node
	}

	logger.Info("Refreshed resource validator caches with %d nodes", len(rv.nodeCache))
	return nil
}

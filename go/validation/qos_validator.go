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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// QoSValidationResult represents the result of QoS validation
type QoSValidationResult struct {
	Valid              bool
	CurrentQoS         corev1.PodQOSClass
	ProposedQoS        corev1.PodQOSClass
	Errors             []string
	Warnings           []string
	PreservationPolicy string
}

// QoSValidator provides comprehensive QoS validation capabilities
type QoSValidator struct {
	allowQoSUpgrade   bool // Allow upgrading from BestEffort/Burstable to higher QoS
	allowQoSDowngrade bool // Allow downgrading from higher to lower QoS
	strictValidation  bool // Enable strict validation mode
}

// NewQoSValidator creates a new QoS validator with default settings
func NewQoSValidator() *QoSValidator {
	return &QoSValidator{
		allowQoSUpgrade:   false, // Kubernetes default - no QoS upgrades
		allowQoSDowngrade: false, // Kubernetes default - no QoS downgrades
		strictValidation:  true,
	}
}

// NewQoSValidatorWithConfig creates a new QoS validator with custom configuration
func NewQoSValidatorWithConfig(allowUpgrade, allowDowngrade, strict bool) *QoSValidator {
	return &QoSValidator{
		allowQoSUpgrade:   allowUpgrade,
		allowQoSDowngrade: allowDowngrade,
		strictValidation:  strict,
	}
}

// ValidateQoSPreservation validates that a resource change preserves the pod's QoS class
// This is a critical requirement for Kubernetes 1.33+ in-place resize compliance
func (qv *QoSValidator) ValidateQoSPreservation(pod *corev1.Pod, containerName string, newResources corev1.ResourceRequirements) QoSValidationResult {
	result := QoSValidationResult{
		Valid:    true,
		Errors:   make([]string, 0),
		Warnings: make([]string, 0),
	}

	// Calculate current QoS class
	result.CurrentQoS = qv.CalculateQoSClass(pod)

	// Create a copy of the pod with the new resources to calculate proposed QoS
	podCopy := pod.DeepCopy()
	updated := false
	for i, container := range podCopy.Spec.Containers {
		if container.Name == containerName {
			podCopy.Spec.Containers[i].Resources = newResources
			updated = true
			break
		}
	}

	if !updated {
		result.Valid = false
		result.Errors = append(result.Errors, fmt.Sprintf("container %s not found in pod", containerName))
		return result
	}

	// Calculate proposed QoS class
	result.ProposedQoS = qv.CalculateQoSClass(podCopy)

	// Validate QoS preservation according to Kubernetes rules
	if err := qv.validateQoSTransition(result.CurrentQoS, result.ProposedQoS, &result); err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err.Error())
	}

	// Perform additional validations based on QoS class
	switch result.ProposedQoS {
	case corev1.PodQOSGuaranteed:
		if err := qv.validateGuaranteedQoS(newResources); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, err.Error())
		}
	case corev1.PodQOSBurstable:
		if warnings := qv.validateBurstableQoS(newResources); len(warnings) > 0 {
			result.Warnings = append(result.Warnings, warnings...)
		}
	case corev1.PodQOSBestEffort:
		if err := qv.validateBestEffortQoS(newResources); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, err.Error())
		}
	}

	// Set preservation policy description
	result.PreservationPolicy = qv.getPreservationPolicyDescription(result.CurrentQoS, result.ProposedQoS)

	return result
}

// validateQoSTransition validates transitions between QoS classes
func (qv *QoSValidator) validateQoSTransition(currentQoS, proposedQoS corev1.PodQOSClass, result *QoSValidationResult) error {
	// If QoS class remains the same, transition is always valid
	if currentQoS == proposedQoS {
		return nil
	}

	// Check for forbidden transitions based on Kubernetes 1.33+ requirements
	switch currentQoS {
	case corev1.PodQOSGuaranteed:
		if !qv.allowQoSDowngrade {
			return fmt.Errorf("cannot change QoS class from Guaranteed (%s) to %s - QoS preservation required",
				currentQoS, proposedQoS)
		}
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Downgrading QoS from Guaranteed to %s may impact performance", proposedQoS))

	case corev1.PodQOSBurstable:
		if proposedQoS == corev1.PodQOSGuaranteed && !qv.allowQoSUpgrade {
			return fmt.Errorf("cannot change QoS class from Burstable to Guaranteed - upgrade not permitted")
		}
		if proposedQoS == corev1.PodQOSBestEffort && !qv.allowQoSDowngrade {
			return fmt.Errorf("cannot change QoS class from Burstable to BestEffort - downgrade not permitted")
		}

	case corev1.PodQOSBestEffort:
		if !qv.allowQoSUpgrade {
			return fmt.Errorf("cannot add resource requirements to BestEffort pod - QoS upgrade not permitted")
		}
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("Upgrading QoS from BestEffort to %s will add resource constraints", proposedQoS))
	}

	return nil
}

// validateGuaranteedQoS validates that resources meet Guaranteed QoS requirements
func (qv *QoSValidator) validateGuaranteedQoS(resources corev1.ResourceRequirements) error {
	// For Guaranteed QoS, requests must equal limits for all resource types

	// Check CPU
	cpuRequest := resources.Requests[corev1.ResourceCPU]
	cpuLimit := resources.Limits[corev1.ResourceCPU]

	if !cpuRequest.IsZero() || !cpuLimit.IsZero() {
		if cpuRequest.IsZero() || cpuLimit.IsZero() {
			return fmt.Errorf("Guaranteed QoS requires both CPU request and limit to be specified")
		}
		if !cpuRequest.Equal(cpuLimit) {
			return fmt.Errorf("Guaranteed QoS requires CPU request (%s) to equal CPU limit (%s)",
				cpuRequest.String(), cpuLimit.String())
		}
	}

	// Check Memory
	memRequest := resources.Requests[corev1.ResourceMemory]
	memLimit := resources.Limits[corev1.ResourceMemory]

	if !memRequest.IsZero() || !memLimit.IsZero() {
		if memRequest.IsZero() || memLimit.IsZero() {
			return fmt.Errorf("Guaranteed QoS requires both memory request and limit to be specified")
		}
		if !memRequest.Equal(memLimit) {
			return fmt.Errorf("Guaranteed QoS requires memory request (%s) to equal memory limit (%s)",
				memRequest.String(), memLimit.String())
		}
	}

	// Check other resources
	for resourceName, request := range resources.Requests {
		if resourceName == corev1.ResourceCPU || resourceName == corev1.ResourceMemory {
			continue // Already checked above
		}

		limit, hasLimit := resources.Limits[resourceName]
		if !hasLimit {
			return fmt.Errorf("Guaranteed QoS requires limit for resource %s to match request %s",
				resourceName, request.String())
		}
		if !request.Equal(limit) {
			return fmt.Errorf("Guaranteed QoS requires request (%s) to equal limit (%s) for resource %s",
				request.String(), limit.String(), resourceName)
		}
	}

	return nil
}

// validateBurstableQoS validates Burstable QoS configuration and returns warnings
func (qv *QoSValidator) validateBurstableQoS(resources corev1.ResourceRequirements) []string {
	warnings := make([]string, 0)

	// Check for potential issues in Burstable configuration
	for resourceName, request := range resources.Requests {
		if limit, hasLimit := resources.Limits[resourceName]; hasLimit {
			if limit.Cmp(request) < 0 {
				warnings = append(warnings, fmt.Sprintf("Resource limit (%s) is less than request (%s) for %s",
					limit.String(), request.String(), resourceName))
			}
		} else {
			warnings = append(warnings, fmt.Sprintf("No limit specified for resource %s with request %s - may impact scheduling",
				resourceName, request.String()))
		}
	}

	return warnings
}

// validateBestEffortQoS validates that resources are appropriate for BestEffort QoS
func (qv *QoSValidator) validateBestEffortQoS(resources corev1.ResourceRequirements) error {
	// BestEffort pods should not have any resource requests or limits

	if len(resources.Requests) > 0 {
		return fmt.Errorf("BestEffort QoS requires no resource requests, but found: %v", resources.Requests)
	}

	if len(resources.Limits) > 0 {
		return fmt.Errorf("BestEffort QoS requires no resource limits, but found: %v", resources.Limits)
	}

	return nil
}

// CalculateQoSClass determines the QoS class for a pod based on its resource specifications
func (qv *QoSValidator) CalculateQoSClass(pod *corev1.Pod) corev1.PodQOSClass {
	requests := make(corev1.ResourceList)
	limits := make(corev1.ResourceList)

	// Aggregate resources from all containers (including init containers)
	allContainers := make([]corev1.Container, 0, len(pod.Spec.Containers)+len(pod.Spec.InitContainers))
	allContainers = append(allContainers, pod.Spec.Containers...)
	allContainers = append(allContainers, pod.Spec.InitContainers...)

	for _, container := range allContainers {
		// Accumulate requests
		for name, quantity := range container.Resources.Requests {
			if existing, exists := requests[name]; exists {
				existing.Add(quantity)
				requests[name] = existing
			} else {
				requests[name] = quantity.DeepCopy()
			}
		}

		// Accumulate limits
		for name, quantity := range container.Resources.Limits {
			if existing, exists := limits[name]; exists {
				existing.Add(quantity)
				limits[name] = existing
			} else {
				limits[name] = quantity.DeepCopy()
			}
		}
	}

	// Determine QoS class based on aggregated resources
	return qv.calculateQoSFromResources(requests, limits)
}

// calculateQoSFromResources determines QoS class from resource lists
func (qv *QoSValidator) calculateQoSFromResources(requests, limits corev1.ResourceList) corev1.PodQOSClass {
	zeroQuantity := resource.MustParse("0")

	// Check if pod has any resource specifications
	hasRequests := false
	hasLimits := false

	for _, quantity := range requests {
		if quantity.Cmp(zeroQuantity) > 0 {
			hasRequests = true
			break
		}
	}

	for _, quantity := range limits {
		if quantity.Cmp(zeroQuantity) > 0 {
			hasLimits = true
			break
		}
	}

	// BestEffort: no requests and no limits
	if !hasRequests && !hasLimits {
		return corev1.PodQOSBestEffort
	}

	// Check for Guaranteed: all resources have requests == limits
	isGuaranteed := true

	// All resources with requests must have equal limits
	for name, request := range requests {
		if request.Cmp(zeroQuantity) > 0 {
			limit, hasLimit := limits[name]
			if !hasLimit || !request.Equal(limit) {
				isGuaranteed = false
				break
			}
		}
	}

	// All resources with limits must have equal requests
	if isGuaranteed {
		for name, limit := range limits {
			if limit.Cmp(zeroQuantity) > 0 {
				request, hasRequest := requests[name]
				if !hasRequest || !limit.Equal(request) {
					isGuaranteed = false
					break
				}
			}
		}
	}

	if isGuaranteed {
		return corev1.PodQOSGuaranteed
	}

	// If not BestEffort and not Guaranteed, must be Burstable
	return corev1.PodQOSBurstable
}

// getPreservationPolicyDescription returns a human-readable description of the QoS preservation policy
func (qv *QoSValidator) getPreservationPolicyDescription(currentQoS, proposedQoS corev1.PodQOSClass) string {
	if currentQoS == proposedQoS {
		return fmt.Sprintf("QoS class preserved (%s)", currentQoS)
	}

	return fmt.Sprintf("QoS class change: %s -> %s", currentQoS, proposedQoS)
}

// ValidateContainerQoSImpact validates the QoS impact of changing a specific container's resources
func (qv *QoSValidator) ValidateContainerQoSImpact(pod *corev1.Pod, containerName string, newResources corev1.ResourceRequirements) error {
	result := qv.ValidateQoSPreservation(pod, containerName, newResources)

	if !result.Valid {
		return fmt.Errorf("QoS validation failed: %v", result.Errors)
	}

	return nil
}

// GetQoSTransitionRules returns the current QoS transition rules
func (qv *QoSValidator) GetQoSTransitionRules() map[string]interface{} {
	return map[string]interface{}{
		"allowQoSUpgrade":   qv.allowQoSUpgrade,
		"allowQoSDowngrade": qv.allowQoSDowngrade,
		"strictValidation":  qv.strictValidation,
	}
}

// SetQoSTransitionRules updates the QoS transition rules
func (qv *QoSValidator) SetQoSTransitionRules(allowUpgrade, allowDowngrade, strict bool) {
	qv.allowQoSUpgrade = allowUpgrade
	qv.allowQoSDowngrade = allowDowngrade
	qv.strictValidation = strict
}

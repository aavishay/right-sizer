package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"right-sizer/validation"
)

// TestResizePolicyValidation tests resize policy validation according to K8s 1.33+ spec
func TestResizePolicyValidation(t *testing.T) {
	testCases := []struct {
		name         string
		pod          *corev1.Pod
		isValid      bool
		errorMessage string
	}{
		{
			name: "Valid NotRequired policy for both CPU and Memory",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "test:latest",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
							ResizePolicy: []corev1.ContainerResizePolicy{
								{
									ResourceName:  corev1.ResourceCPU,
									RestartPolicy: corev1.NotRequired,
								},
								{
									ResourceName:  corev1.ResourceMemory,
									RestartPolicy: corev1.NotRequired,
								},
							},
						},
					},
				},
			},
			isValid: true,
		},
		{
			name: "Valid RestartContainer policy for Memory",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "test:latest",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
							ResizePolicy: []corev1.ContainerResizePolicy{
								{
									ResourceName:  corev1.ResourceCPU,
									RestartPolicy: corev1.NotRequired,
								},
								{
									ResourceName:  corev1.ResourceMemory,
									RestartPolicy: corev1.RestartContainer,
								},
							},
						},
					},
				},
			},
			isValid: true,
		},
		{
			name: "Invalid - RestartContainer policy with Never restart policy",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "test:latest",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
							ResizePolicy: []corev1.ContainerResizePolicy{
								{
									ResourceName:  corev1.ResourceMemory,
									RestartPolicy: corev1.RestartContainer,
								},
							},
						},
					},
				},
			},
			isValid:      false,
			errorMessage: "RestartContainer resize policy cannot be used with Never restart policy",
		},
		{
			name: "Valid - Default behavior (no resize policy specified)",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "test:latest",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
					},
				},
			},
			isValid: true,
		},
		{
			name: "Invalid - Unknown resource name in resize policy",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "test:latest",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
							ResizePolicy: []corev1.ContainerResizePolicy{
								{
									ResourceName:  "unknown-resource",
									RestartPolicy: corev1.NotRequired,
								},
							},
						},
					},
				},
			},
			isValid:      false,
			errorMessage: "unsupported resource name in resize policy",
		},
	}

	validator := validation.NewResizePolicyValidator()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := validator.ValidateResizePolicy(tc.pod)

			assert.Equal(t, tc.isValid, result.IsValid(),
				"Expected validation result to be %v", tc.isValid)

			if !tc.isValid && tc.errorMessage != "" {
				assert.Contains(t, result.Error(), tc.errorMessage,
					"Expected error message to contain: %s", tc.errorMessage)
			}
		})
	}
}

// TestQoSClassPreservationValidation tests QoS class preservation during resize
func TestQoSClassPreservationValidation(t *testing.T) {
	testCases := []struct {
		name           string
		currentQoS     corev1.PodQOSClass
		currentRes     corev1.ResourceRequirements
		newRes         corev1.ResourceRequirements
		shouldPreserve bool
		expectedQoS    corev1.PodQOSClass
		errorMessage   string
	}{
		{
			name:       "Guaranteed QoS preservation - valid resize",
			currentQoS: corev1.PodQOSGuaranteed,
			currentRes: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			},
			newRes: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
			shouldPreserve: true,
			expectedQoS:    corev1.PodQOSGuaranteed,
		},
		{
			name:       "Guaranteed QoS violation - requests != limits",
			currentQoS: corev1.PodQOSGuaranteed,
			currentRes: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			},
			newRes: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"), // Different from request
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
			shouldPreserve: false,
			errorMessage:   "would violate Guaranteed QoS class",
		},
		{
			name:       "Burstable QoS preservation - valid resize",
			currentQoS: corev1.PodQOSBurstable,
			currentRes: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
			newRes: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("150m"),
					corev1.ResourceMemory: resource.MustParse("192Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("300m"),
					corev1.ResourceMemory: resource.MustParse("384Mi"),
				},
			},
			shouldPreserve: true,
			expectedQoS:    corev1.PodQOSBurstable,
		},
		{
			name:       "Burstable to Guaranteed violation",
			currentQoS: corev1.PodQOSBurstable,
			currentRes: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
			newRes: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
			shouldPreserve: false,
			errorMessage:   "would change QoS class from Burstable to Guaranteed",
		},
		{
			name:       "BestEffort QoS - cannot add resources",
			currentQoS: corev1.PodQOSBestEffort,
			currentRes: corev1.ResourceRequirements{},
			newRes: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				},
			},
			shouldPreserve: false,
			errorMessage:   "would change QoS class from BestEffort",
		},
	}

	validator := validation.NewQoSValidator()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := validator.ValidateQoSPreservation(tc.currentQoS, tc.currentRes, tc.newRes)

			assert.Equal(t, tc.shouldPreserve, result.IsValid(),
				"Expected QoS preservation validation to be %v", tc.shouldPreserve)

			if !tc.shouldPreserve && tc.errorMessage != "" {
				assert.Contains(t, result.Error(), tc.errorMessage,
					"Expected error message to contain: %s", tc.errorMessage)
			}

			if tc.shouldPreserve {
				newQoS := validation.CalculateQoSClass(tc.newRes)
				assert.Equal(t, tc.expectedQoS, newQoS,
					"Expected QoS class to be %s", tc.expectedQoS)
			}
		})
	}
}

// TestResourceValidationRules tests resource validation rules for resizing
func TestResourceValidationRules(t *testing.T) {
	testCases := []struct {
		name         string
		current      corev1.ResourceRequirements
		desired      corev1.ResourceRequirements
		resizePolicy []corev1.ContainerResizePolicy
		isValid      bool
		errorMessage string
	}{
		{
			name: "Valid CPU increase",
			current: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("200m"),
				},
			},
			desired: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("150m"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("300m"),
				},
			},
			resizePolicy: []corev1.ContainerResizePolicy{
				{ResourceName: corev1.ResourceCPU, RestartPolicy: corev1.NotRequired},
			},
			isValid: true,
		},
		{
			name: "Valid memory increase",
			current: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
			desired: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("192Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("384Mi"),
				},
			},
			resizePolicy: []corev1.ContainerResizePolicy{
				{ResourceName: corev1.ResourceMemory, RestartPolicy: corev1.NotRequired},
			},
			isValid: true,
		},
		{
			name: "Memory decrease with NotRequired policy (should warn)",
			current: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
			desired: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
			resizePolicy: []corev1.ContainerResizePolicy{
				{ResourceName: corev1.ResourceMemory, RestartPolicy: corev1.NotRequired},
			},
			isValid: true, // Should be valid but with warnings
		},
		{
			name: "Memory decrease with RestartContainer policy",
			current: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
			desired: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
			resizePolicy: []corev1.ContainerResizePolicy{
				{ResourceName: corev1.ResourceMemory, RestartPolicy: corev1.RestartContainer},
			},
			isValid: true,
		},
		{
			name: "Invalid - limits less than requests",
			current: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("200m"),
				},
			},
			desired: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("300m"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("200m"), // Less than request
				},
			},
			isValid:      false,
			errorMessage: "limits cannot be less than requests",
		},
		{
			name: "Invalid - negative resources",
			current: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				},
			},
			desired: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: *resource.NewMilliQuantity(-100, resource.DecimalSI),
				},
			},
			isValid:      false,
			errorMessage: "resource values cannot be negative",
		},
	}

	validator := validation.NewResourceValidator(nil, nil, nil, nil)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := validator.ValidateResourceChange(nil, &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:         "test",
							Resources:    tc.current,
							ResizePolicy: tc.resizePolicy,
						},
					},
				},
			}, tc.desired, "test")

			assert.Equal(t, tc.isValid, result.Valid,
				"Expected resource validation to be %v", tc.isValid)

			if !tc.isValid && tc.errorMessage != "" {
				found := false
				for _, err := range result.Errors {
					if assert.Contains(t, err, tc.errorMessage) {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected to find error message containing: %s", tc.errorMessage)
			}

			// Check for warnings on memory decrease with NotRequired policy
			if tc.name == "Memory decrease with NotRequired policy (should warn)" {
				assert.NotEmpty(t, result.Warnings, "Should have warnings for memory decrease")
			}
		})
	}
}

// TestContainerRestartPolicyLogic tests container restart policy logic
func TestContainerRestartPolicyLogic(t *testing.T) {
	testCases := []struct {
		name           string
		resizePolicy   []corev1.ContainerResizePolicy
		resourceChange map[corev1.ResourceName]bool // true = changed
		shouldRestart  bool
		reason         string
	}{
		{
			name: "CPU change with NotRequired policy - no restart",
			resizePolicy: []corev1.ContainerResizePolicy{
				{ResourceName: corev1.ResourceCPU, RestartPolicy: corev1.NotRequired},
				{ResourceName: corev1.ResourceMemory, RestartPolicy: corev1.NotRequired},
			},
			resourceChange: map[corev1.ResourceName]bool{
				corev1.ResourceCPU: true,
			},
			shouldRestart: false,
			reason:        "CPU policy is NotRequired",
		},
		{
			name: "Memory change with RestartContainer policy - restart",
			resizePolicy: []corev1.ContainerResizePolicy{
				{ResourceName: corev1.ResourceCPU, RestartPolicy: corev1.NotRequired},
				{ResourceName: corev1.ResourceMemory, RestartPolicy: corev1.RestartContainer},
			},
			resourceChange: map[corev1.ResourceName]bool{
				corev1.ResourceMemory: true,
			},
			shouldRestart: true,
			reason:        "Memory policy requires restart",
		},
		{
			name: "Both CPU and Memory change - restart if any requires restart",
			resizePolicy: []corev1.ContainerResizePolicy{
				{ResourceName: corev1.ResourceCPU, RestartPolicy: corev1.NotRequired},
				{ResourceName: corev1.ResourceMemory, RestartPolicy: corev1.RestartContainer},
			},
			resourceChange: map[corev1.ResourceName]bool{
				corev1.ResourceCPU:    true,
				corev1.ResourceMemory: true,
			},
			shouldRestart: true,
			reason:        "Memory policy requires restart",
		},
		{
			name:         "No policy specified - default to NotRequired",
			resizePolicy: nil,
			resourceChange: map[corev1.ResourceName]bool{
				corev1.ResourceCPU:    true,
				corev1.ResourceMemory: true,
			},
			shouldRestart: false,
			reason:        "Default policy is NotRequired",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			restartDecision := validation.ShouldRestartContainer(tc.resizePolicy, tc.resourceChange)

			assert.Equal(t, tc.shouldRestart, restartDecision.ShouldRestart,
				"Expected restart decision to be %v for reason: %s", tc.shouldRestart, tc.reason)

			if tc.shouldRestart {
				assert.NotEmpty(t, restartDecision.Reason,
					"Should have a reason for restart")
			}
		})
	}
}

// TestEdgeCasesAndErrorScenarios tests edge cases and error scenarios
func TestEdgeCasesAndErrorScenarios(t *testing.T) {
	validator := validation.NewResourceValidator(nil, nil, nil, nil)

	t.Run("Empty container list", func(t *testing.T) {
		pod := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{},
			},
		}

		result := validator.ValidateResourceChange(nil, pod, corev1.ResourceRequirements{}, "non-existent")
		assert.False(t, result.Valid, "Should fail validation for non-existent container")
		assert.Contains(t, result.Errors[0], "container not found")
	})

	t.Run("Container without resources", func(t *testing.T) {
		pod := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "test",
						// No resources specified
					},
				},
			},
		}

		newResources := corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("100m"),
			},
		}

		result := validator.ValidateResourceChange(nil, pod, newResources, "test")
		// Should be valid - adding resources to a container that had none
		assert.True(t, result.Valid, "Should allow adding resources to container without resources")
	})

	t.Run("Very large resource values", func(t *testing.T) {
		pod := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: "test",
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU: resource.MustParse("100m"),
							},
						},
					},
				},
			},
		}

		// Extremely large resource request
		newResources := corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("999999"),
			},
		}

		result := validator.ValidateResourceChange(nil, pod, newResources, "test")
		// Should warn about very large values but may not invalidate
		if !result.Valid {
			assert.Contains(t, result.Errors[0], "resource value too large")
		}
	})

	t.Run("Duplicate resize policy entries", func(t *testing.T) {
		resizePolicy := []corev1.ContainerResizePolicy{
			{ResourceName: corev1.ResourceCPU, RestartPolicy: corev1.NotRequired},
			{ResourceName: corev1.ResourceCPU, RestartPolicy: corev1.RestartContainer}, // Duplicate
		}

		policyValidator := validation.NewResizePolicyValidator()
		pod := &corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:         "test",
						ResizePolicy: resizePolicy,
					},
				},
			},
		}

		result := policyValidator.ValidateResizePolicy(pod)
		assert.False(t, result.IsValid(), "Should fail validation for duplicate resize policy entries")
		assert.Contains(t, result.Error(), "duplicate")
	})
}

// TestResizePolicyDefaults tests default resize policy behavior
func TestResizePolicyDefaults(t *testing.T) {
	t.Run("Default policy when none specified", func(t *testing.T) {
		// When no resize policy is specified, default should be NotRequired
		defaultPolicy := validation.GetDefaultResizePolicy(corev1.ResourceCPU)
		assert.Equal(t, corev1.NotRequired, defaultPolicy,
			"Default CPU resize policy should be NotRequired")

		defaultPolicy = validation.GetDefaultResizePolicy(corev1.ResourceMemory)
		assert.Equal(t, corev1.NotRequired, defaultPolicy,
			"Default Memory resize policy should be NotRequired")
	})

	t.Run("Apply default policies to container", func(t *testing.T) {
		container := &corev1.Container{
			Name: "test",
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			},
		}

		validation.ApplyDefaultResizePolicies(container)

		require.NotNil(t, container.ResizePolicy)
		assert.Len(t, container.ResizePolicy, 2, "Should have policies for CPU and Memory")

		// Check CPU policy
		var cpuPolicy *corev1.ContainerResizePolicy
		var memoryPolicy *corev1.ContainerResizePolicy

		for i := range container.ResizePolicy {
			if container.ResizePolicy[i].ResourceName == corev1.ResourceCPU {
				cpuPolicy = &container.ResizePolicy[i]
			} else if container.ResizePolicy[i].ResourceName == corev1.ResourceMemory {
				memoryPolicy = &container.ResizePolicy[i]
			}
		}

		require.NotNil(t, cpuPolicy, "Should have CPU resize policy")
		assert.Equal(t, corev1.NotRequired, cpuPolicy.RestartPolicy,
			"Default CPU policy should be NotRequired")

		require.NotNil(t, memoryPolicy, "Should have Memory resize policy")
		assert.Equal(t, corev1.NotRequired, memoryPolicy.RestartPolicy,
			"Default Memory policy should be NotRequired")
	})
}

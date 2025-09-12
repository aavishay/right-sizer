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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Helper function to create resource requirements
func createResourceRequirements(cpuReq, memReq, cpuLim, memLim string) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits:   corev1.ResourceList{},
	}

	if cpuReq != "" {
		resources.Requests[corev1.ResourceCPU] = resource.MustParse(cpuReq)
	}
	if memReq != "" {
		resources.Requests[corev1.ResourceMemory] = resource.MustParse(memReq)
	}
	if cpuLim != "" {
		resources.Limits[corev1.ResourceCPU] = resource.MustParse(cpuLim)
	}
	if memLim != "" {
		resources.Limits[corev1.ResourceMemory] = resource.MustParse(memLim)
	}

	return resources
}

// Helper function to create test pod
func createTestPodWithResources(name, namespace string, containers []corev1.Container) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: containers,
		},
	}
}

func TestNewQoSValidator(t *testing.T) {
	validator := NewQoSValidator()

	if validator == nil {
		t.Fatal("Expected non-nil validator")
	}

	rules := validator.GetQoSTransitionRules()
	if rules["allowQoSUpgrade"].(bool) {
		t.Error("Expected default allowQoSUpgrade to be false")
	}
	if rules["allowQoSDowngrade"].(bool) {
		t.Error("Expected default allowQoSDowngrade to be false")
	}
	if !rules["strictValidation"].(bool) {
		t.Error("Expected default strictValidation to be true")
	}
}

func TestNewQoSValidatorWithConfig(t *testing.T) {
	validator := NewQoSValidatorWithConfig(true, true, false)

	rules := validator.GetQoSTransitionRules()
	if !rules["allowQoSUpgrade"].(bool) {
		t.Error("Expected allowQoSUpgrade to be true")
	}
	if !rules["allowQoSDowngrade"].(bool) {
		t.Error("Expected allowQoSDowngrade to be true")
	}
	if rules["strictValidation"].(bool) {
		t.Error("Expected strictValidation to be false")
	}
}

func TestCalculateQoSClass(t *testing.T) {
	validator := NewQoSValidator()

	tests := []struct {
		name        string
		containers  []corev1.Container
		expectedQoS corev1.PodQOSClass
		description string
	}{
		{
			name: "Guaranteed QoS - equal requests and limits",
			containers: []corev1.Container{
				{
					Name:      "app",
					Resources: createResourceRequirements("100m", "128Mi", "100m", "128Mi"),
				},
			},
			expectedQoS: corev1.PodQOSGuaranteed,
			description: "Pod with equal CPU and memory requests and limits",
		},
		{
			name: "Burstable QoS - requests only",
			containers: []corev1.Container{
				{
					Name:      "app",
					Resources: createResourceRequirements("100m", "128Mi", "", ""),
				},
			},
			expectedQoS: corev1.PodQOSBurstable,
			description: "Pod with only requests, no limits",
		},
		{
			name: "Burstable QoS - limits only",
			containers: []corev1.Container{
				{
					Name:      "app",
					Resources: createResourceRequirements("", "", "200m", "256Mi"),
				},
			},
			expectedQoS: corev1.PodQOSBurstable,
			description: "Pod with only limits, no requests",
		},
		{
			name: "Burstable QoS - unequal requests and limits",
			containers: []corev1.Container{
				{
					Name:      "app",
					Resources: createResourceRequirements("100m", "128Mi", "200m", "256Mi"),
				},
			},
			expectedQoS: corev1.PodQOSBurstable,
			description: "Pod with requests less than limits",
		},
		{
			name: "BestEffort QoS - no resources",
			containers: []corev1.Container{
				{
					Name:      "app",
					Resources: corev1.ResourceRequirements{},
				},
			},
			expectedQoS: corev1.PodQOSBestEffort,
			description: "Pod with no resource requirements",
		},
		{
			name: "Multiple containers - Guaranteed",
			containers: []corev1.Container{
				{
					Name:      "app",
					Resources: createResourceRequirements("100m", "128Mi", "100m", "128Mi"),
				},
				{
					Name:      "sidecar",
					Resources: createResourceRequirements("50m", "64Mi", "50m", "64Mi"),
				},
			},
			expectedQoS: corev1.PodQOSGuaranteed,
			description: "Multiple containers all with equal requests and limits",
		},
		{
			name: "Multiple containers - Burstable",
			containers: []corev1.Container{
				{
					Name:      "app",
					Resources: createResourceRequirements("100m", "128Mi", "100m", "128Mi"),
				},
				{
					Name:      "sidecar",
					Resources: createResourceRequirements("50m", "64Mi", "100m", "128Mi"),
				},
			},
			expectedQoS: corev1.PodQOSBurstable,
			description: "Multiple containers with mixed resource configurations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := createTestPodWithResources("test-pod", "default", tt.containers)

			qos := validator.CalculateQoSClass(pod)

			if qos != tt.expectedQoS {
				t.Errorf("Expected QoS class %s, got %s. %s", tt.expectedQoS, qos, tt.description)
			}
		})
	}
}

func TestValidateQoSPreservation(t *testing.T) {
	validator := NewQoSValidator()

	tests := []struct {
		name              string
		pod               *corev1.Pod
		containerName     string
		newResources      corev1.ResourceRequirements
		expectValid       bool
		expectCurrentQoS  corev1.PodQOSClass
		expectProposedQoS corev1.PodQOSClass
		expectErrors      int
	}{
		{
			name: "Valid preservation - Guaranteed to Guaranteed",
			pod: createTestPodWithResources("test", "default", []corev1.Container{
				{
					Name:      "app",
					Resources: createResourceRequirements("100m", "128Mi", "100m", "128Mi"),
				},
			}),
			containerName:     "app",
			newResources:      createResourceRequirements("200m", "256Mi", "200m", "256Mi"),
			expectValid:       true,
			expectCurrentQoS:  corev1.PodQOSGuaranteed,
			expectProposedQoS: corev1.PodQOSGuaranteed,
			expectErrors:      0,
		},
		{
			name: "Invalid transition - Guaranteed to Burstable",
			pod: createTestPodWithResources("test", "default", []corev1.Container{
				{
					Name:      "app",
					Resources: createResourceRequirements("100m", "128Mi", "100m", "128Mi"),
				},
			}),
			containerName:     "app",
			newResources:      createResourceRequirements("100m", "128Mi", "200m", "256Mi"),
			expectValid:       false,
			expectCurrentQoS:  corev1.PodQOSGuaranteed,
			expectProposedQoS: corev1.PodQOSBurstable,
			expectErrors:      1,
		},
		{
			name: "Valid preservation - Burstable to Burstable",
			pod: createTestPodWithResources("test", "default", []corev1.Container{
				{
					Name:      "app",
					Resources: createResourceRequirements("100m", "128Mi", "200m", "256Mi"),
				},
			}),
			containerName:     "app",
			newResources:      createResourceRequirements("150m", "192Mi", "300m", "384Mi"),
			expectValid:       true,
			expectCurrentQoS:  corev1.PodQOSBurstable,
			expectProposedQoS: corev1.PodQOSBurstable,
			expectErrors:      0,
		},
		{
			name: "Container not found",
			pod: createTestPodWithResources("test", "default", []corev1.Container{
				{
					Name:      "app",
					Resources: createResourceRequirements("100m", "128Mi", "100m", "128Mi"),
				},
			}),
			containerName:     "nonexistent",
			newResources:      createResourceRequirements("200m", "256Mi", "200m", "256Mi"),
			expectValid:       false,
			expectCurrentQoS:  corev1.PodQOSGuaranteed,
			expectProposedQoS: corev1.PodQOSGuaranteed, // Won't be calculated due to error
			expectErrors:      1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateQoSPreservation(tt.pod, tt.containerName, tt.newResources)

			if result.Valid != tt.expectValid {
				t.Errorf("Expected valid=%v, got %v", tt.expectValid, result.Valid)
			}

			if result.CurrentQoS != tt.expectCurrentQoS {
				t.Errorf("Expected current QoS %s, got %s", tt.expectCurrentQoS, result.CurrentQoS)
			}

			if tt.expectValid && result.ProposedQoS != tt.expectProposedQoS {
				t.Errorf("Expected proposed QoS %s, got %s", tt.expectProposedQoS, result.ProposedQoS)
			}

			if len(result.Errors) != tt.expectErrors {
				t.Errorf("Expected %d errors, got %d: %v", tt.expectErrors, len(result.Errors), result.Errors)
			}
		})
	}
}

func TestValidateGuaranteedQoS(t *testing.T) {
	validator := NewQoSValidator()

	tests := []struct {
		name        string
		resources   corev1.ResourceRequirements
		expectError bool
		description string
	}{
		{
			name:        "Valid Guaranteed - equal CPU and memory",
			resources:   createResourceRequirements("100m", "128Mi", "100m", "128Mi"),
			expectError: false,
			description: "CPU and memory requests equal limits",
		},
		{
			name:        "Valid Guaranteed - CPU only",
			resources:   createResourceRequirements("100m", "", "100m", ""),
			expectError: false,
			description: "Only CPU with equal request and limit",
		},
		{
			name:        "Valid Guaranteed - memory only",
			resources:   createResourceRequirements("", "128Mi", "", "128Mi"),
			expectError: false,
			description: "Only memory with equal request and limit",
		},
		{
			name:        "Invalid - CPU request without limit",
			resources:   createResourceRequirements("100m", "", "", ""),
			expectError: true,
			description: "CPU request without corresponding limit",
		},
		{
			name:        "Invalid - CPU limit without request",
			resources:   createResourceRequirements("", "", "100m", ""),
			expectError: true,
			description: "CPU limit without corresponding request",
		},
		{
			name:        "Invalid - unequal CPU request and limit",
			resources:   createResourceRequirements("100m", "", "200m", ""),
			expectError: true,
			description: "CPU request not equal to CPU limit",
		},
		{
			name:        "Invalid - memory request without limit",
			resources:   createResourceRequirements("", "128Mi", "", ""),
			expectError: true,
			description: "Memory request without corresponding limit",
		},
		{
			name:        "Invalid - unequal memory request and limit",
			resources:   createResourceRequirements("", "128Mi", "", "256Mi"),
			expectError: true,
			description: "Memory request not equal to memory limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateGuaranteedQoS(tt.resources)

			if tt.expectError && err == nil {
				t.Errorf("Expected error for %s, but got nil", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error for %s, but got: %v", tt.description, err)
			}
		})
	}
}

func TestValidateBurstableQoS(t *testing.T) {
	validator := NewQoSValidator()

	tests := []struct {
		name           string
		resources      corev1.ResourceRequirements
		expectWarnings int
		description    string
	}{
		{
			name:           "No warnings - requests less than limits",
			resources:      createResourceRequirements("100m", "128Mi", "200m", "256Mi"),
			expectWarnings: 0,
			description:    "Normal burstable configuration",
		},
		{
			name:           "Warning - limit less than request",
			resources:      createResourceRequirements("200m", "256Mi", "100m", "128Mi"),
			expectWarnings: 2, // One for CPU, one for memory
			description:    "Limits less than requests",
		},
		{
			name:           "Warning - requests without limits",
			resources:      createResourceRequirements("100m", "128Mi", "", ""),
			expectWarnings: 2, // One for CPU, one for memory
			description:    "Requests specified without limits",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			warnings := validator.validateBurstableQoS(tt.resources)

			if len(warnings) != tt.expectWarnings {
				t.Errorf("Expected %d warnings for %s, got %d: %v",
					tt.expectWarnings, tt.description, len(warnings), warnings)
			}
		})
	}
}

func TestValidateBestEffortQoS(t *testing.T) {
	validator := NewQoSValidator()

	tests := []struct {
		name        string
		resources   corev1.ResourceRequirements
		expectError bool
		description string
	}{
		{
			name:        "Valid BestEffort - no resources",
			resources:   corev1.ResourceRequirements{},
			expectError: false,
			description: "No resource requirements",
		},
		{
			name:        "Valid BestEffort - empty requests and limits",
			resources:   corev1.ResourceRequirements{Requests: corev1.ResourceList{}, Limits: corev1.ResourceList{}},
			expectError: false,
			description: "Empty resource lists",
		},
		{
			name:        "Invalid - has requests",
			resources:   createResourceRequirements("100m", "", "", ""),
			expectError: true,
			description: "BestEffort with CPU request",
		},
		{
			name:        "Invalid - has limits",
			resources:   createResourceRequirements("", "", "100m", ""),
			expectError: true,
			description: "BestEffort with CPU limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateBestEffortQoS(tt.resources)

			if tt.expectError && err == nil {
				t.Errorf("Expected error for %s, but got nil", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error for %s, but got: %v", tt.description, err)
			}
		})
	}
}

func TestQoSTransitionWithConfig(t *testing.T) {
	tests := []struct {
		name           string
		allowUpgrade   bool
		allowDowngrade bool
		currentQoS     corev1.PodQOSClass
		proposedQoS    corev1.PodQOSClass
		expectValid    bool
	}{
		{
			name:           "Upgrade allowed - BestEffort to Burstable",
			allowUpgrade:   true,
			allowDowngrade: false,
			currentQoS:     corev1.PodQOSBestEffort,
			proposedQoS:    corev1.PodQOSBurstable,
			expectValid:    true,
		},
		{
			name:           "Upgrade forbidden - BestEffort to Burstable",
			allowUpgrade:   false,
			allowDowngrade: false,
			currentQoS:     corev1.PodQOSBestEffort,
			proposedQoS:    corev1.PodQOSBurstable,
			expectValid:    false,
		},
		{
			name:           "Downgrade allowed - Guaranteed to Burstable",
			allowUpgrade:   false,
			allowDowngrade: true,
			currentQoS:     corev1.PodQOSGuaranteed,
			proposedQoS:    corev1.PodQOSBurstable,
			expectValid:    true,
		},
		{
			name:           "Downgrade forbidden - Guaranteed to Burstable",
			allowUpgrade:   false,
			allowDowngrade: false,
			currentQoS:     corev1.PodQOSGuaranteed,
			proposedQoS:    corev1.PodQOSBurstable,
			expectValid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewQoSValidatorWithConfig(tt.allowUpgrade, tt.allowDowngrade, true)

			// Create pods with appropriate QoS classes
			var currentPod *corev1.Pod

			switch tt.currentQoS {
			case corev1.PodQOSGuaranteed:
				currentPod = createTestPodWithResources("test", "default", []corev1.Container{
					{Name: "app", Resources: createResourceRequirements("100m", "128Mi", "100m", "128Mi")},
				})
			case corev1.PodQOSBurstable:
				currentPod = createTestPodWithResources("test", "default", []corev1.Container{
					{Name: "app", Resources: createResourceRequirements("100m", "128Mi", "200m", "256Mi")},
				})
			case corev1.PodQOSBestEffort:
				currentPod = createTestPodWithResources("test", "default", []corev1.Container{
					{Name: "app", Resources: corev1.ResourceRequirements{}},
				})
			}

			// Create new resources that would result in the proposed QoS
			var newResources corev1.ResourceRequirements
			switch tt.proposedQoS {
			case corev1.PodQOSGuaranteed:
				newResources = createResourceRequirements("200m", "256Mi", "200m", "256Mi")
			case corev1.PodQOSBurstable:
				newResources = createResourceRequirements("100m", "128Mi", "200m", "256Mi")
			case corev1.PodQOSBestEffort:
				newResources = corev1.ResourceRequirements{}
			}

			result := validator.ValidateQoSPreservation(currentPod, "app", newResources)

			if result.Valid != tt.expectValid {
				t.Errorf("Expected valid=%v, got %v. Errors: %v", tt.expectValid, result.Valid, result.Errors)
			}

			// Verify the QoS classes are as expected
			if result.CurrentQoS != tt.currentQoS {
				t.Errorf("Expected current QoS %s, got %s", tt.currentQoS, result.CurrentQoS)
			}
			if result.ProposedQoS != tt.proposedQoS {
				t.Errorf("Expected proposed QoS %s, got %s", tt.proposedQoS, result.ProposedQoS)
			}
		})
	}
}

func TestValidateContainerQoSImpact(t *testing.T) {
	validator := NewQoSValidator()

	pod := createTestPodWithResources("test", "default", []corev1.Container{
		{
			Name:      "app",
			Resources: createResourceRequirements("100m", "128Mi", "100m", "128Mi"),
		},
	})

	// Valid change that preserves QoS
	newResources := createResourceRequirements("200m", "256Mi", "200m", "256Mi")
	err := validator.ValidateContainerQoSImpact(pod, "app", newResources)
	if err != nil {
		t.Errorf("Expected no error for valid QoS preservation, got: %v", err)
	}

	// Invalid change that violates QoS
	invalidResources := createResourceRequirements("100m", "128Mi", "200m", "256Mi")
	err = validator.ValidateContainerQoSImpact(pod, "app", invalidResources)
	if err == nil {
		t.Error("Expected error for QoS violation, but got nil")
	}
}

func TestSetQoSTransitionRules(t *testing.T) {
	validator := NewQoSValidator()

	// Initial state
	rules := validator.GetQoSTransitionRules()
	if rules["allowQoSUpgrade"].(bool) || rules["allowQoSDowngrade"].(bool) {
		t.Error("Expected initial rules to forbid transitions")
	}

	// Update rules
	validator.SetQoSTransitionRules(true, true, false)

	// Verify update
	rules = validator.GetQoSTransitionRules()
	if !rules["allowQoSUpgrade"].(bool) {
		t.Error("Expected allowQoSUpgrade to be true after update")
	}
	if !rules["allowQoSDowngrade"].(bool) {
		t.Error("Expected allowQoSDowngrade to be true after update")
	}
	if rules["strictValidation"].(bool) {
		t.Error("Expected strictValidation to be false after update")
	}
}

func TestPreservationPolicyDescription(t *testing.T) {
	validator := NewQoSValidator()

	tests := []struct {
		currentQoS  corev1.PodQOSClass
		proposedQoS corev1.PodQOSClass
		expected    string
	}{
		{
			currentQoS:  corev1.PodQOSGuaranteed,
			proposedQoS: corev1.PodQOSGuaranteed,
			expected:    "QoS class preserved (Guaranteed)",
		},
		{
			currentQoS:  corev1.PodQOSBurstable,
			proposedQoS: corev1.PodQOSGuaranteed,
			expected:    "QoS class change: Burstable -> Guaranteed",
		},
		{
			currentQoS:  corev1.PodQOSGuaranteed,
			proposedQoS: corev1.PodQOSBestEffort,
			expected:    "QoS class change: Guaranteed -> BestEffort",
		},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := validator.getPreservationPolicyDescription(tt.currentQoS, tt.proposedQoS)
			if result != tt.expected {
				t.Errorf("Expected description '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestCalculateQoSFromResources(t *testing.T) {
	validator := NewQoSValidator()

	tests := []struct {
		name        string
		requests    corev1.ResourceList
		limits      corev1.ResourceList
		expectedQoS corev1.PodQOSClass
	}{
		{
			name:        "Empty resources - BestEffort",
			requests:    corev1.ResourceList{},
			limits:      corev1.ResourceList{},
			expectedQoS: corev1.PodQOSBestEffort,
		},
		{
			name: "Equal requests and limits - Guaranteed",
			requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			expectedQoS: corev1.PodQOSGuaranteed,
		},
		{
			name: "Different requests and limits - Burstable",
			requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
			limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("200m"),
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
			expectedQoS: corev1.PodQOSBurstable,
		},
		{
			name: "Only requests - Burstable",
			requests: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("100m"),
			},
			limits:      corev1.ResourceList{},
			expectedQoS: corev1.PodQOSBurstable,
		},
		{
			name:     "Only limits - Burstable",
			requests: corev1.ResourceList{},
			limits: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("100m"),
			},
			expectedQoS: corev1.PodQOSBurstable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			qos := validator.calculateQoSFromResources(tt.requests, tt.limits)
			if qos != tt.expectedQoS {
				t.Errorf("Expected QoS %s, got %s", tt.expectedQoS, qos)
			}
		})
	}
}

func TestCalculateQoSClassWithInitContainers(t *testing.T) {
	validator := NewQoSValidator()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name:      "init",
					Resources: createResourceRequirements("50m", "64Mi", "50m", "64Mi"),
				},
			},
			Containers: []corev1.Container{
				{
					Name:      "app",
					Resources: createResourceRequirements("100m", "128Mi", "100m", "128Mi"),
				},
			},
		},
	}

	qos := validator.CalculateQoSClass(pod)
	if qos != corev1.PodQOSGuaranteed {
		t.Errorf("Expected Guaranteed QoS with init containers, got %s", qos)
	}
}

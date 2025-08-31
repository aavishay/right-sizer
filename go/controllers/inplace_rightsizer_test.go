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
	"testing"

	"right-sizer/config"
	"right-sizer/metrics"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCheckScalingThresholds(t *testing.T) {
	tests := []struct {
		name             string
		usage            metrics.Metrics
		pod              *corev1.Pod
		memoryUpThresh   float64
		memoryDownThresh float64
		cpuUpThresh      float64
		cpuDownThresh    float64
		expectedDecision ScalingDecision
		description      string
	}{
		{
			name: "scale_up_memory_exceeds_threshold",
			usage: metrics.Metrics{
				CPUMilli: 500,  // 50% of 1000m limit
				MemMB:    1700, // 85% of 2000MB limit
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-container",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1000m"),
									corev1.ResourceMemory: resource.MustParse("2000Mi"),
								},
							},
						},
					},
				},
			},
			memoryUpThresh:   0.8, // 80%
			memoryDownThresh: 0.3,
			cpuUpThresh:      0.8,
			cpuDownThresh:    0.3,
			expectedDecision: ScaleUp,
			description:      "Should scale up when memory usage (85%) exceeds threshold (80%)",
		},
		{
			name: "scale_up_cpu_exceeds_threshold",
			usage: metrics.Metrics{
				CPUMilli: 900,  // 90% of 1000m limit
				MemMB:    1400, // 70% of 2000MB limit
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-container",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1000m"),
									corev1.ResourceMemory: resource.MustParse("2000Mi"),
								},
							},
						},
					},
				},
			},
			memoryUpThresh:   0.8,
			memoryDownThresh: 0.3,
			cpuUpThresh:      0.8, // 80%
			cpuDownThresh:    0.3,
			expectedDecision: ScaleUp,
			description:      "Should scale up when CPU usage (90%) exceeds threshold (80%)",
		},
		{
			name: "scale_up_both_exceed_threshold",
			usage: metrics.Metrics{
				CPUMilli: 850,  // 85% of 1000m limit
				MemMB:    1700, // 85% of 2000MB limit
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-container",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1000m"),
									corev1.ResourceMemory: resource.MustParse("2000Mi"),
								},
							},
						},
					},
				},
			},
			memoryUpThresh:   0.8, // 80%
			memoryDownThresh: 0.3,
			cpuUpThresh:      0.8, // 80%
			cpuDownThresh:    0.3,
			expectedDecision: ScaleUp,
			description:      "Should scale up when both CPU (85%) and memory (85%) exceed thresholds",
		},
		{
			name: "scale_down_both_below_threshold",
			usage: metrics.Metrics{
				CPUMilli: 200, // 20% of 1000m limit
				MemMB:    400, // 20% of 2000MB limit
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-container",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1000m"),
									corev1.ResourceMemory: resource.MustParse("2000Mi"),
								},
							},
						},
					},
				},
			},
			memoryUpThresh:   0.8,
			memoryDownThresh: 0.3, // 30%
			cpuUpThresh:      0.8,
			cpuDownThresh:    0.3, // 30%
			expectedDecision: ScaleDown,
			description:      "Should scale down when both CPU (20%) and memory (20%) are below thresholds",
		},
		{
			name: "no_scale_between_thresholds",
			usage: metrics.Metrics{
				CPUMilli: 500,  // 50% of 1000m limit
				MemMB:    1000, // 50% of 2000MB limit
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-container",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1000m"),
									corev1.ResourceMemory: resource.MustParse("2000Mi"),
								},
							},
						},
					},
				},
			},
			memoryUpThresh:   0.8, // 80%
			memoryDownThresh: 0.3, // 30%
			cpuUpThresh:      0.8, // 80%
			cpuDownThresh:    0.3, // 30%
			expectedDecision: ScaleNone,
			description:      "Should not scale when usage (50%) is between thresholds (30%-80%)",
		},
		{
			name: "no_scale_down_when_only_cpu_below",
			usage: metrics.Metrics{
				CPUMilli: 200, // 20% of 1000m limit
				MemMB:    800, // 40% of 2000MB limit
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-container",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1000m"),
									corev1.ResourceMemory: resource.MustParse("2000Mi"),
								},
							},
						},
					},
				},
			},
			memoryUpThresh:   0.8,
			memoryDownThresh: 0.3, // 30%
			cpuUpThresh:      0.8,
			cpuDownThresh:    0.3, // 30%
			expectedDecision: ScaleNone,
			description:      "Should not scale down when only CPU (20%) is below threshold but memory (40%) is not",
		},
		{
			name: "scale_up_no_limits_uses_requests",
			usage: metrics.Metrics{
				CPUMilli: 450, // 90% of 500m request
				MemMB:    900, // 90% of 1000MB request
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-container",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("1000Mi"),
								},
								// No limits set
							},
						},
					},
				},
			},
			memoryUpThresh:   0.8, // 80%
			memoryDownThresh: 0.3,
			cpuUpThresh:      0.8, // 80%
			cpuDownThresh:    0.3,
			expectedDecision: ScaleUp,
			description:      "Should use requests as baseline when limits are not set",
		},
		{
			name: "scale_up_no_resources_set",
			usage: metrics.Metrics{
				CPUMilli: 100,
				MemMB:    256,
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-container",
							// No resources set at all
						},
					},
				},
			},
			memoryUpThresh:   0.8,
			memoryDownThresh: 0.3,
			cpuUpThresh:      0.8,
			cpuDownThresh:    0.3,
			expectedDecision: ScaleUp,
			description:      "Should scale up when no resources are set",
		},
		{
			name: "multi_container_pod_aggregate",
			usage: metrics.Metrics{
				CPUMilli: 1800, // 90% of total 2000m limit
				MemMB:    3600, // 90% of total 4000MB limit
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "container-1",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1000m"),
									corev1.ResourceMemory: resource.MustParse("2000Mi"),
								},
							},
						},
						{
							Name: "container-2",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1000m"),
									corev1.ResourceMemory: resource.MustParse("2000Mi"),
								},
							},
						},
					},
				},
			},
			memoryUpThresh:   0.8, // 80%
			memoryDownThresh: 0.3,
			cpuUpThresh:      0.8, // 80%
			cpuDownThresh:    0.3,
			expectedDecision: ScaleUp,
			description:      "Should correctly aggregate limits from multiple containers",
		},
		{
			name: "custom_aggressive_thresholds",
			usage: metrics.Metrics{
				CPUMilli: 920,  // 92% of 1000m limit
				MemMB:    1800, // 90% of 2000MB limit
			},
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-container",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("1000m"),
									corev1.ResourceMemory: resource.MustParse("2000Mi"),
								},
							},
						},
					},
				},
			},
			memoryUpThresh:   0.95, // 95% - very conservative
			memoryDownThresh: 0.5,  // 50% - aggressive scale down
			cpuUpThresh:      0.95, // 95% - very conservative
			cpuDownThresh:    0.5,  // 50% - aggressive scale down
			expectedDecision: ScaleNone,
			description:      "Should not scale up with conservative thresholds (95%) even at high usage (90-92%)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up config with test thresholds
			cfg := config.GetDefaults()
			cfg.MemoryScaleUpThreshold = tt.memoryUpThresh
			cfg.MemoryScaleDownThreshold = tt.memoryDownThresh
			cfg.CPUScaleUpThreshold = tt.cpuUpThresh
			cfg.CPUScaleDownThreshold = tt.cpuDownThresh
			config.Global = cfg

			// Create InPlaceRightSizer
			r := &InPlaceRightSizer{}

			// Test the scaling decision
			decision := r.checkScalingThresholds(tt.usage, tt.pod)

			if decision != tt.expectedDecision {
				t.Errorf("%s: expected %v, got %v", tt.description, tt.expectedDecision, decision)
			}
		})
	}
}

func TestCalculateOptimalResourcesWithScalingDecision(t *testing.T) {
	// Set up default config
	cfg := config.GetDefaults()
	cfg.CPURequestMultiplier = 1.2
	cfg.MemoryRequestMultiplier = 1.2
	cfg.CPURequestAddition = 50
	cfg.MemoryRequestAddition = 100
	cfg.MinCPURequest = 10
	cfg.MinMemoryRequest = 64
	cfg.MaxCPULimit = 4000
	cfg.MaxMemoryLimit = 8192
	config.Global = cfg

	r := &InPlaceRightSizer{}

	tests := []struct {
		name            string
		cpuMilli        float64
		memMB           float64
		scalingDecision ScalingDecision
		expectedCPUReq  int64
		expectedMemReq  int64
		description     string
	}{
		{
			name:            "scale_up_with_multipliers",
			cpuMilli:        100,
			memMB:           500,
			scalingDecision: ScaleUp,
			expectedCPUReq:  170, // (100 * 1.2) + 50 = 170
			expectedMemReq:  700, // (500 * 1.2) + 100 = 700
			description:     "Should apply standard multipliers when scaling up",
		},
		{
			name:            "scale_down_with_reduced_multipliers",
			cpuMilli:        100,
			memMB:           500,
			scalingDecision: ScaleDown,
			expectedCPUReq:  160, // (100 * 1.1) + 50 = 160
			expectedMemReq:  650, // (500 * 1.1) + 100 = 650
			description:     "Should use reduced multipliers when scaling down",
		},
		{
			name:            "respect_minimum_values",
			cpuMilli:        1,
			memMB:           10,
			scalingDecision: ScaleUp,
			expectedCPUReq:  51,  // Would be (1 * 1.2) + 50 = 51
			expectedMemReq:  112, // Would be (10 * 1.2) + 100 = 112
			description:     "Should respect minimum values even with low usage",
		},
		{
			name:            "respect_maximum_limits",
			cpuMilli:        5000,
			memMB:           10000,
			scalingDecision: ScaleUp,
			expectedCPUReq:  6050,  // (5000 * 1.2) + 50 = 6050, but not capped at request level
			expectedMemReq:  12100, // (10000 * 1.2) + 100 = 12100, but not capped at request level
			description:     "Should calculate requests without capping, limits will be capped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources := r.calculateOptimalResources(tt.cpuMilli, tt.memMB, tt.scalingDecision)

			cpuReq := resources.Requests[corev1.ResourceCPU]
			actualCPUReq := cpuReq.MilliValue()
			memReq := resources.Requests[corev1.ResourceMemory]
			actualMemReq := memReq.Value() / (1024 * 1024) // Convert to MB

			if actualCPUReq != tt.expectedCPUReq {
				t.Errorf("%s: CPU request expected %d, got %d", tt.description, tt.expectedCPUReq, actualCPUReq)
			}

			if actualMemReq != tt.expectedMemReq {
				t.Errorf("%s: Memory request expected %d, got %d", tt.description, tt.expectedMemReq, actualMemReq)
			}

			// Verify limits are properly calculated
			cpuLimitQ := resources.Limits[corev1.ResourceCPU]
			cpuLimit := cpuLimitQ.MilliValue()
			memLimitQ := resources.Limits[corev1.ResourceMemory]
			memLimit := memLimitQ.Value() / (1024 * 1024)

			// Limits should be capped at max values
			if cpuLimit > cfg.MaxCPULimit {
				t.Errorf("%s: CPU limit %d exceeds max %d", tt.description, cpuLimit, cfg.MaxCPULimit)
			}

			if memLimit > cfg.MaxMemoryLimit {
				t.Errorf("%s: Memory limit %d exceeds max %d", tt.description, memLimit, cfg.MaxMemoryLimit)
			}
		})
	}
}

func TestScalingDecisionIntegration(t *testing.T) {
	// Test the full flow from metrics to scaling decision

	// Create a mock metrics provider
	mockProvider := &mockMetricsProvider{
		metrics: metrics.Metrics{
			CPUMilli: 850,  // 85% of 1000m
			MemMB:    1700, // 85% of 2000MB
		},
	}

	// Set up config
	cfg := config.GetDefaults()
	cfg.MemoryScaleUpThreshold = 0.8
	cfg.CPUScaleUpThreshold = 0.8
	config.Global = cfg

	// Create InPlaceRightSizer
	r := &InPlaceRightSizer{
		MetricsProvider: mockProvider,
	}

	// Create test pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "test-container",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("1000Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1000m"),
							corev1.ResourceMemory: resource.MustParse("2000Mi"),
						},
					},
				},
			},
		},
	}

	// Check scaling thresholds
	decision := r.checkScalingThresholds(mockProvider.metrics, pod)

	if decision != ScaleUp {
		t.Errorf("Expected ScaleUp decision for 85%% usage with 80%% threshold, got %v", decision)
	}

	// Calculate new resources
	newResourcesMap := r.calculateOptimalResourcesForContainers(mockProvider.metrics, pod, decision)

	if len(newResourcesMap) != 1 {
		t.Errorf("Expected resources for 1 container, got %d", len(newResourcesMap))
	}

	newResources, exists := newResourcesMap["test-container"]
	if !exists {
		t.Fatal("Expected resources for test-container")
	}

	// Verify resources were increased
	newCPUReqQ := newResources.Requests[corev1.ResourceCPU]
	newCPUReq := newCPUReqQ.MilliValue()
	oldCPUReqQ := pod.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU]
	oldCPUReq := oldCPUReqQ.MilliValue()

	if newCPUReq <= oldCPUReq {
		t.Errorf("Expected CPU request to increase from %d to %d", oldCPUReq, newCPUReq)
	}

	newMemReqQ := newResources.Requests[corev1.ResourceMemory]
	newMemReq := newMemReqQ.Value()
	oldMemReqQ := pod.Spec.Containers[0].Resources.Requests[corev1.ResourceMemory]
	oldMemReq := oldMemReqQ.Value()

	if newMemReq <= oldMemReq {
		t.Errorf("Expected memory request to increase from %d to %d", oldMemReq, newMemReq)
	}
}

// Mock metrics provider for testing
type mockMetricsProvider struct {
	metrics metrics.Metrics
	err     error
}

func (m *mockMetricsProvider) FetchPodMetrics(namespace, name string) (metrics.Metrics, error) {
	return m.metrics, m.err
}

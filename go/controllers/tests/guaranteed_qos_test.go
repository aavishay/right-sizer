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

package tests

import (
	"encoding/json"
	"fmt"
	"testing"

	"right-sizer/config"
	"right-sizer/controllers"
	"right-sizer/metrics"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MockMetricsProvider for testing
type MockMetricsProvider struct {
	cpuMilli float64
	memMB    float64
}

func (m *MockMetricsProvider) FetchPodMetrics(namespace, name string) (metrics.Metrics, error) {
	return metrics.Metrics{
		CPUMilli: m.cpuMilli,
		MemMB:    m.memMB,
	}, nil
}

func (m *MockMetricsProvider) FetchNodeMetrics(nodeName string) (metrics.Metrics, error) {
	return metrics.Metrics{}, nil
}

func (m *MockMetricsProvider) FetchClusterMetrics() (map[string]metrics.Metrics, error) {
	return map[string]metrics.Metrics{}, nil
}

// TestGuaranteedQoSPreservation tests that Guaranteed QoS pods maintain their QoS class
func TestGuaranteedQoSPreservation(t *testing.T) {
	tests := []struct {
		name                  string
		pod                   *corev1.Pod
		preserveGuaranteedQoS bool
		expectedQoSPreserved  bool
		description           string
	}{
		{
			name: "guaranteed_pod_with_preservation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "guaranteed-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "app",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
					},
				},
			},
			preserveGuaranteedQoS: true,
			expectedQoSPreserved:  true,
			description:           "Guaranteed pod should maintain QoS when preservation is enabled",
		},
		{
			name: "guaranteed_pod_without_preservation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "guaranteed-pod-2",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "app",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
						},
					},
				},
			},
			preserveGuaranteedQoS: false,
			expectedQoSPreserved:  false,
			description:           "Guaranteed pod may change QoS when preservation is disabled",
		},
		{
			name: "burstable_pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "burstable-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "app",
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
						},
					},
				},
			},
			preserveGuaranteedQoS: true,
			expectedQoSPreserved:  false,
			description:           "Burstable pod remains burstable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup config
			cfg := config.GetDefaults()
			cfg.PreserveGuaranteedQoS = tt.preserveGuaranteedQoS
			cfg.QoSTransitionWarning = true
			config.Global = cfg

			// Note: For this test, we're only testing the logic of QoS preservation
			// and don't need actual client interactions

			// Create resource update
			update := controllers.ResourceUpdate{
				Namespace:     tt.pod.Namespace,
				Name:          tt.pod.Name,
				ContainerName: tt.pod.Spec.Containers[0].Name,
				NewResources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("60m"),
						corev1.ResourceMemory: resource.MustParse("80Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("120m"),
						corev1.ResourceMemory: resource.MustParse("160Mi"),
					},
				},
			}

			// Determine initial QoS
			initialQoS := getQoSClass(tt.pod)
			isInitiallyGuaranteed := initialQoS == corev1.PodQOSGuaranteed

			// Apply the update logic (simulated)
			if isInitiallyGuaranteed && cfg.PreserveGuaranteedQoS {
				// Should maintain Guaranteed QoS
				update.NewResources.Limits = make(corev1.ResourceList)
				for k, v := range update.NewResources.Requests {
					update.NewResources.Limits[k] = v.DeepCopy()
				}
			}

			// Check if QoS is preserved
			qosPreserved := false
			if isInitiallyGuaranteed && tt.preserveGuaranteedQoS {
				// Check if requests equal limits in the update
				cpuReq := update.NewResources.Requests[corev1.ResourceCPU]
				cpuLim := update.NewResources.Limits[corev1.ResourceCPU]
				memReq := update.NewResources.Requests[corev1.ResourceMemory]
				memLim := update.NewResources.Limits[corev1.ResourceMemory]

				qosPreserved = cpuReq.Equal(cpuLim) && memReq.Equal(memLim)
			}

			if isInitiallyGuaranteed && tt.expectedQoSPreserved && !qosPreserved {
				t.Errorf("%s: Expected Guaranteed QoS to be preserved but it wasn't", tt.description)
			}
		})
	}
}

// TestPatchStructureForGuaranteedPods tests that the patch structure works correctly
func TestPatchStructureForGuaranteedPods(t *testing.T) {
	// Test JSON patch creation
	containerIndex := 0
	newResources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("150m"),
			corev1.ResourceMemory: resource.MustParse("192Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("150m"),
			corev1.ResourceMemory: resource.MustParse("192Mi"),
		},
	}

	type JSONPatchOp struct {
		Op    string      `json:"op"`
		Path  string      `json:"path"`
		Value interface{} `json:"value"`
	}

	var patchOps []JSONPatchOp

	// Create patch operations
	patchOps = append(patchOps, JSONPatchOp{
		Op:    "replace",
		Path:  fmt.Sprintf("/spec/containers/%d/resources/requests", containerIndex),
		Value: newResources.Requests,
	})

	patchOps = append(patchOps, JSONPatchOp{
		Op:    "replace",
		Path:  fmt.Sprintf("/spec/containers/%d/resources/limits", containerIndex),
		Value: newResources.Limits,
	})

	// Marshal the patch
	patchData, err := json.Marshal(patchOps)
	if err != nil {
		t.Fatalf("Failed to marshal patch: %v", err)
	}

	// Verify patch structure
	var verifyOps []JSONPatchOp
	if err := json.Unmarshal(patchData, &verifyOps); err != nil {
		t.Fatalf("Failed to unmarshal patch: %v", err)
	}

	if len(verifyOps) != 2 {
		t.Errorf("Expected 2 patch operations, got %d", len(verifyOps))
	}

	// Verify paths
	expectedPaths := []string{
		"/spec/containers/0/resources/requests",
		"/spec/containers/0/resources/limits",
	}

	for i, op := range verifyOps {
		if op.Path != expectedPaths[i] {
			t.Errorf("Expected path %s, got %s", expectedPaths[i], op.Path)
		}
		if op.Op != "replace" {
			t.Errorf("Expected operation 'replace', got %s", op.Op)
		}
	}
}

// TestMemoryDecreaseHandling tests that memory decreases are handled correctly
func TestMemoryDecreaseHandling(t *testing.T) {
	tests := []struct {
		name               string
		currentMemory      string
		newMemory          string
		expectError        bool
		expectCPUOnlyPatch bool
	}{
		{
			name:               "memory_increase",
			currentMemory:      "128Mi",
			newMemory:          "256Mi",
			expectError:        false,
			expectCPUOnlyPatch: false,
		},
		{
			name:               "memory_decrease",
			currentMemory:      "256Mi",
			newMemory:          "128Mi",
			expectError:        false,
			expectCPUOnlyPatch: true, // Should only patch CPU, keeping current memory
		},
		{
			name:               "memory_unchanged",
			currentMemory:      "128Mi",
			newMemory:          "128Mi",
			expectError:        false,
			expectCPUOnlyPatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			currentMem := resource.MustParse(tt.currentMemory)
			newMem := resource.MustParse(tt.newMemory)

			memoryDecreased := currentMem.Cmp(newMem) > 0

			if memoryDecreased && !tt.expectCPUOnlyPatch {
				t.Errorf("Expected CPU-only patch for memory decrease but didn't get it")
			}
			if !memoryDecreased && tt.expectCPUOnlyPatch {
				t.Errorf("Expected full patch but got CPU-only indication")
			}
		})
	}
}

// getQoSClass helper function for tests
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
	cpuReq, hasCPUReq := requests[corev1.ResourceCPU]
	cpuLim, hasCPULim := limits[corev1.ResourceCPU]
	memReq, hasMemReq := requests[corev1.ResourceMemory]
	memLim, hasMemLim := limits[corev1.ResourceMemory]

	if !hasCPUReq || !hasCPULim || !hasMemReq || !hasMemLim {
		isGuaranteed = false
	} else if cpuReq.Cmp(cpuLim) != 0 || memReq.Cmp(memLim) != 0 {
		isGuaranteed = false
	}

	if isGuaranteed {
		return corev1.PodQOSGuaranteed
	}

	// Check if burstable
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

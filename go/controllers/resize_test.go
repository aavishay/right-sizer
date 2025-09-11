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
	"right-sizer/config"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	ctrlclientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// TestTwoStepResize tests that resize operations are performed in two steps
func TestTwoStepResize(t *testing.T) {
	tests := []struct {
		name              string
		pod               *corev1.Pod
		newResources      map[string]corev1.ResourceRequirements
		expectCPUPatch    bool
		expectMemoryPatch bool
		expectError       bool
	}{
		{
			name: "resize both CPU and memory",
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
			newResources: map[string]corev1.ResourceRequirements{
				"test-container": {
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("150m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("300m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
			},
			expectCPUPatch:    true,
			expectMemoryPatch: true,
			expectError:       false,
		},
		{
			name: "resize only CPU",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-cpu",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-container",
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
			newResources: map[string]corev1.ResourceRequirements{
				"test-container": {
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("150m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("300m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
				},
			},
			expectCPUPatch:    true,
			expectMemoryPatch: false,
			expectError:       false,
		},
		{
			name: "resize only memory",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod-mem",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-container",
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
			newResources: map[string]corev1.ResourceRequirements{
				"test-container": {
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
			},
			expectCPUPatch:    false,
			expectMemoryPatch: true,
			expectError:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Track patch operations
			var patchOperations []string
			var patchCount int

			// Create fake clientset with reactor to track patches
			fakeClient := fake.NewSimpleClientset(tt.pod)
			fakeClient.PrependReactor("patch", "pods", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
				patchAction := action.(clienttesting.PatchAction)
				patchCount++

				// Check if it's a resize subresource patch
				if patchAction.GetSubresource() == "resize" {
					// Decode the patch to check what's being updated
					var patch interface{}
					if err := json.Unmarshal(patchAction.GetPatch(), &patch); err == nil {
						// Determine if it's CPU or memory based on patch content
						patchStr := string(patchAction.GetPatch())
						if contains(patchStr, "cpu") && !contains(patchStr, "memory") {
							patchOperations = append(patchOperations, "cpu")
						} else if contains(patchStr, "memory") && !contains(patchStr, "cpu") {
							patchOperations = append(patchOperations, "memory")
						} else if contains(patchStr, "cpu") && contains(patchStr, "memory") {
							// This would be wrong - we should never patch both at once
							patchOperations = append(patchOperations, "both")
						}
					}
				} else if patchAction.GetSubresource() == "" {
					// Regular patch (for resize policy)
					patchOperations = append(patchOperations, "policy")
				}

				return true, tt.pod, nil
			})

			// Create fake controller-runtime client
			scheme := runtime.NewScheme()
			_ = corev1.AddToScheme(scheme)
			ctrlClient := ctrlclientfake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(tt.pod).
				Build()

			// Create InPlaceRightSizer
			cfg := config.GetDefaults()
			cfg.UpdateResizePolicy = true
			r := &InPlaceRightSizer{
				Client:    ctrlClient,
				ClientSet: fakeClient,
				Config:    cfg,
			}

			// Apply resize
			ctx := context.Background()
			err := r.applyInPlaceResize(ctx, tt.pod, tt.newResources)

			// Check error
			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Verify that resize policy was set first
			if len(patchOperations) > 0 && patchOperations[0] != "policy" {
				t.Errorf("expected first patch to be resize policy, got: %s", patchOperations[0])
			}

			// Verify CPU and memory were patched separately
			cpuPatched := false
			memoryPatched := false
			for _, op := range patchOperations {
				if op == "cpu" {
					cpuPatched = true
				}
				if op == "memory" {
					memoryPatched = true
				}
				if op == "both" {
					t.Errorf("CPU and memory should not be patched together")
				}
			}

			if tt.expectCPUPatch && !cpuPatched {
				t.Errorf("expected CPU patch but it didn't happen")
			}
			if tt.expectMemoryPatch && !memoryPatched {
				t.Errorf("expected memory patch but it didn't happen")
			}

			// Verify order: policy -> CPU -> memory
			if len(patchOperations) >= 3 {
				expectedOrder := []string{"policy", "cpu", "memory"}
				for i := 0; i < len(expectedOrder) && i < len(patchOperations); i++ {
					if patchOperations[i] != expectedOrder[i] {
						t.Errorf("patch order incorrect at position %d: expected %s, got %s",
							i, expectedOrder[i], patchOperations[i])
					}
				}
			}
		})
	}
}

// TestResizePolicy tests that resize policy is correctly applied
func TestResizePolicy(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "container1",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				},
				{
					Name: "container2",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
					// This container already has resize policy
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
	}

	// Create fake clientset
	fakeClient := fake.NewSimpleClientset(pod)

	var patchCalled bool
	fakeClient.PrependReactor("patch", "pods", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		patchCalled = true
		return true, pod, nil
	})

	// Create fake controller-runtime client
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	ctrlClient := ctrlclientfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(pod).
		Build()

	// Create InPlaceRightSizer
	cfg := config.GetDefaults()
	cfg.UpdateResizePolicy = true
	r := &InPlaceRightSizer{
		Client:    ctrlClient,
		ClientSet: fakeClient,
		Config:    cfg,
	}

	// Apply resize policy
	ctx := context.Background()
	err := r.applyResizePolicy(ctx, pod)
	if err != nil {
		t.Errorf("unexpected error applying resize policy: %v", err)
	}

	// InPlaceRightSizer.applyResizePolicy should skip direct pod patching
	// (resize policies should be set in parent resources only)
	if patchCalled {
		t.Error("expected no patch to be called for direct pod resize policy update")
	}
}

// TestAdaptiveRightSizerTwoStepResize tests the adaptive rightsizer's two-step resize
func TestAdaptiveRightSizerTwoStepResize(t *testing.T) {
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
	}

	update := ResourceUpdate{
		Namespace:     "default",
		Name:          "test-pod",
		ContainerName: "test-container",
		NewResources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("150m"),
				corev1.ResourceMemory: resource.MustParse("256Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("300m"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
		},
	}

	// Track patch operations
	var patchSequence []string

	// Create fake clientset
	fakeClient := fake.NewSimpleClientset(pod)
	fakeClient.PrependReactor("patch", "pods", func(action clienttesting.Action) (handled bool, ret runtime.Object, err error) {
		patchAction := action.(clienttesting.PatchAction)

		// Check if it's a resize subresource patch
		if patchAction.GetSubresource() == "resize" {
			// Decode the patch to determine CPU vs memory
			var patchOps []map[string]interface{}
			if err := json.Unmarshal(patchAction.GetPatch(), &patchOps); err == nil {
				for _, op := range patchOps {
					if path, ok := op["path"].(string); ok {
						if contains(path, "resources") {
							if value, ok := op["value"].(map[string]interface{}); ok {
								_, hasCPU := value["cpu"]
								_, hasMemory := value["memory"]

								// Determine patch type based on which resources are being updated
								if hasCPU && !hasMemory {
									patchSequence = append(patchSequence, "cpu-resize")
								} else if hasMemory && !hasCPU {
									patchSequence = append(patchSequence, "memory-resize")
								} else if hasCPU && hasMemory {
									// Check if this is maintaining current values
									// In two-step resize, we keep one resource at current value
									patchSequence = append(patchSequence, "mixed-resize")
								}
							}
						}
					}
				}
			}
		} else {
			// Regular patch (for resize policy)
			patchSequence = append(patchSequence, "policy")
		}

		return true, pod, nil
	})

	// Create fake controller-runtime client
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	ctrlClient := ctrlclientfake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(pod).
		Build()

	// Create AdaptiveRightSizer
	cfg := config.GetDefaults()
	cfg.UpdateResizePolicy = true
	r := &AdaptiveRightSizer{
		Client:    ctrlClient,
		ClientSet: fakeClient,
		Config:    cfg,
	}

	// Apply resize
	ctx := context.Background()
	result, err := r.updatePodInPlace(ctx, update)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result == "" {
		t.Errorf("expected result message but got empty string")
	}

	// Verify the sequence of operations
	if len(patchSequence) < 2 {
		t.Errorf("expected at least 2 patch operations, got %d", len(patchSequence))
	}

	// First should be policy
	if len(patchSequence) > 0 && patchSequence[0] != "policy" {
		t.Logf("Warning: expected first operation to be policy, got %s", patchSequence[0])
	}

	// Should have separate CPU and memory operations
	hasCPUResize := false
	hasMemoryResize := false
	for _, op := range patchSequence {
		if op == "cpu-resize" || contains(op, "cpu") {
			hasCPUResize = true
		}
		if op == "memory-resize" || contains(op, "memory") {
			hasMemoryResize = true
		}
	}

	if !hasCPUResize {
		t.Errorf("expected CPU resize operation")
	}
	if !hasMemoryResize {
		t.Errorf("expected memory resize operation")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// Helper function to create a fake pod with specific resources
func createTestPod(name, namespace string, cpuReq, memReq, cpuLim, memLim string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "test-container",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse(cpuReq),
							corev1.ResourceMemory: resource.MustParse(memReq),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse(cpuLim),
							corev1.ResourceMemory: resource.MustParse(memLim),
						},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
}

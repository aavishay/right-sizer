package unit

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MockWebhookServer is a mock implementation for testing
type MockWebhookServer struct {
	UpdateResizePolicy bool
}

// JSONPatch represents a JSON patch operation
type JSONPatch struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// GenerateResourcePatches generates patches for pod resources
func (m *MockWebhookServer) GenerateResourcePatches(pod *corev1.Pod) []JSONPatch {
	patches := make([]JSONPatch, 0)

	// Add default resource requests if missing
	for i := range pod.Spec.Containers {
		container := &pod.Spec.Containers[i]
		if container.Resources.Requests == nil {
			patches = append(patches, JSONPatch{
				Op:   "add",
				Path: "/spec/containers/" + string(rune(i)) + "/resources/requests",
				Value: map[string]string{
					"cpu":    "10m",
					"memory": "64Mi",
				},
			})
		}

		// Add resize policy only if the UpdateResizePolicy feature flag is enabled
		if m.UpdateResizePolicy {
			resizePolicy := []corev1.ContainerResizePolicy{
				{
					ResourceName:  corev1.ResourceCPU,
					RestartPolicy: corev1.NotRequired,
				},
				{
					ResourceName:  corev1.ResourceMemory,
					RestartPolicy: corev1.NotRequired,
				},
			}

			hasResizePolicy := container.ResizePolicy != nil && len(container.ResizePolicy) > 0
			resizePolicyOp := "add"
			if hasResizePolicy {
				resizePolicyOp = "replace"
			}

			patches = append(patches, JSONPatch{
				Op:    resizePolicyOp,
				Path:  "/spec/containers/" + string(rune(i)) + "/resizePolicy",
				Value: resizePolicy,
			})
		}
	}

	return patches
}

// TestResizePolicyFeatureFlag tests that resize policy patches
// are only added when the UpdateResizePolicy feature flag is enabled
func TestResizePolicyFeatureFlag(t *testing.T) {
	tests := []struct {
		name                    string
		updateResizePolicy      bool
		pod                     *corev1.Pod
		expectResizePolicyPatch bool
	}{
		{
			name:               "Should add resize policy when feature flag is enabled",
			updateResizePolicy: true,
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
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
			expectResizePolicyPatch: true,
		},
		{
			name:               "Should NOT add resize policy when feature flag is disabled",
			updateResizePolicy: false,
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
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
			expectResizePolicyPatch: false,
		},
		{
			name:               "Should handle multiple containers when feature flag is enabled",
			updateResizePolicy: true,
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multi-container-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "container-1",
							Image: "test:latest",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
						{
							Name:  "container-2",
							Image: "test:latest",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
						},
					},
				},
			},
			expectResizePolicyPatch: true,
		},
		{
			name:               "Should handle container with existing resize policy when feature flag is enabled",
			updateResizePolicy: true,
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-policy-pod",
					Namespace: "default",
				},
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
									ResourceName:  corev1.ResourceCPU,
									RestartPolicy: corev1.RestartContainer,
								},
							},
						},
					},
				},
			},
			expectResizePolicyPatch: true, // Should replace existing policy
		},
		{
			name:               "Should NOT add resize policy to pod without containers",
			updateResizePolicy: true,
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "empty-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{},
				},
			},
			expectResizePolicyPatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock webhook server with specific UpdateResizePolicy setting
			server := &MockWebhookServer{
				UpdateResizePolicy: tt.updateResizePolicy,
			}

			// Generate patches
			patches := server.GenerateResourcePatches(tt.pod)

			// Check if resize policy patch is present
			hasResizePolicyPatch := false
			resizePolicyPatchCount := 0
			for _, patch := range patches {
				if strings.Contains(patch.Path, "resizePolicy") {
					hasResizePolicyPatch = true
					resizePolicyPatchCount++
				}
			}

			// Verify expectations
			if tt.expectResizePolicyPatch {
				assert.True(t, hasResizePolicyPatch,
					"Expected resize policy patch when feature flag is enabled")

				// For multi-container pods, we expect one patch per container
				if tt.name == "Should handle multiple containers when feature flag is enabled" {
					assert.Equal(t, 2, resizePolicyPatchCount,
						"Expected one resize policy patch per container")
				}
			} else {
				assert.False(t, hasResizePolicyPatch,
					"Expected no resize policy patch when feature flag is disabled or no containers")
			}
		})
	}
}

// TestResizePolicyContent tests the content of resize policy patches
func TestResizePolicyContent(t *testing.T) {
	server := &MockWebhookServer{
		UpdateResizePolicy: true,
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "test:latest",
				},
			},
		},
	}

	patches := server.GenerateResourcePatches(pod)
	require.NotEmpty(t, patches, "Expected patches to be generated")

	// Find the resize policy patch
	var resizePolicyPatch *JSONPatch
	for i := range patches {
		if strings.Contains(patches[i].Path, "resizePolicy") {
			resizePolicyPatch = &patches[i]
			break
		}
	}

	require.NotNil(t, resizePolicyPatch, "Expected to find resize policy patch")

	// Verify the patch content
	resizePolicies, ok := resizePolicyPatch.Value.([]corev1.ContainerResizePolicy)
	require.True(t, ok, "Expected resize policy value to be []ContainerResizePolicy")
	require.Len(t, resizePolicies, 2, "Expected two resize policies (CPU and Memory)")

	// Check CPU resize policy
	cpuPolicy := resizePolicies[0]
	assert.Equal(t, corev1.ResourceCPU, cpuPolicy.ResourceName)
	assert.Equal(t, corev1.NotRequired, cpuPolicy.RestartPolicy)

	// Check Memory resize policy
	memPolicy := resizePolicies[1]
	assert.Equal(t, corev1.ResourceMemory, memPolicy.ResourceName)
	assert.Equal(t, corev1.NotRequired, memPolicy.RestartPolicy)
}

// TestResizePolicyDefaultBehavior tests the default behavior without feature flag
func TestResizePolicyDefaultBehavior(t *testing.T) {
	// Create server with default settings (feature flag disabled)
	server := &MockWebhookServer{
		UpdateResizePolicy: false, // Default should be false
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
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
	}

	patches := server.GenerateResourcePatches(pod)

	// Verify no resize policy patches are present
	for _, patch := range patches {
		assert.False(t, strings.Contains(patch.Path, "resizePolicy"),
			"Expected no resize policy patches with default settings")
	}
}

package admission

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"right-sizer/config"
)

// TestGenerateResourcePatches_ResizePolicyFeatureFlag tests that resize policy patches
// are only added when the UpdateResizePolicy feature flag is enabled
func TestGenerateResourcePatches_ResizePolicyFeatureFlag(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config with specific UpdateResizePolicy setting
			cfg := config.GetDefaults()
			cfg.UpdateResizePolicy = tt.updateResizePolicy

			// Create a minimal webhook server with just the config
			ws := &WebhookServer{
				config: cfg,
			}

			// Generate patches
			patches := ws.generateResourcePatches(tt.pod)

			// Check if resize policy patch is present
			hasResizePolicyPatch := false
			resizePolicyPatchCount := 0
			for _, patch := range patches {
				if containsString(patch.Path, "resizePolicy") {
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
					"Expected no resize policy patch when feature flag is disabled")
			}
		})
	}
}

// TestDefaultFeatureFlagValue tests that the default value of UpdateResizePolicy is false
func TestDefaultFeatureFlagValue(t *testing.T) {
	cfg := config.GetDefaults()

	assert.False(t, cfg.UpdateResizePolicy,
		"UpdateResizePolicy feature flag should default to false for safety")
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr ||
		len(substr) > 0 && len(s) > len(substr) && containsSubstring(s, substr)
}

// Simple substring check
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

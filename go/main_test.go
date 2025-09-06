package main

import (
	"right-sizer/config"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsSystemNamespace(t *testing.T) {
	systemNamespaces := []string{
		"kube-system",
		"kube-public",
		"kube-node-lease",
		"cert-manager",
		"ingress-nginx",
		"istio-system",
	}

	userNamespaces := []string{
		"default",
		"production",
		"staging",
		"my-app",
	}

	for _, ns := range systemNamespaces {
		t.Run("system namespace "+ns, func(t *testing.T) {
			assert.True(t, isSystemNamespace(ns), "Namespace %s should be considered system", ns)
		})
	}

	for _, ns := range userNamespaces {
		t.Run("user namespace "+ns, func(t *testing.T) {
			assert.False(t, isSystemNamespace(ns), "Namespace %s should not be considered system", ns)
		})
	}
}

func TestShouldSkipPod(t *testing.T) {
	tests := []struct {
		name       string
		pod        *corev1.Pod
		shouldSkip bool
		skipReason string
	}{
		{
			name: "pod with disable annotation should be skipped",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
					Annotations: map[string]string{
						"rightsizer.io/disable": "true",
					},
				},
			},
			shouldSkip: true,
			skipReason: "disabled by annotation",
		},
		{
			name: "pod with disable label should be skipped",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
					Labels: map[string]string{
						"rightsizer": "disabled",
					},
				},
			},
			shouldSkip: true,
			skipReason: "disabled by label",
		},
		{
			name: "system pod should be skipped",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "system-pod",
					Namespace: "kube-system",
				},
			},
			shouldSkip: true,
			skipReason: "system namespace",
		},
		{
			name: "normal pod should not be skipped",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "normal-pod",
					Namespace: "default",
				},
			},
			shouldSkip: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skip, reason := shouldSkipPod(tt.pod)
			assert.Equal(t, tt.shouldSkip, skip)
			if tt.shouldSkip {
				assert.Contains(t, reason, tt.skipReason)
			}
		})
	}
}

func TestCreateTestPod(t *testing.T) {
	pod := createTestPod("test-pod", "default")

	assert.Equal(t, "test-pod", pod.Name)
	assert.Equal(t, "default", pod.Namespace)
	assert.Len(t, pod.Spec.Containers, 1)
	assert.Equal(t, "app", pod.Spec.Containers[0].Name)

	// Check resource requests
	requests := pod.Spec.Containers[0].Resources.Requests
	expectedCPU := resource.MustParse("200m")
	expectedMem := resource.MustParse("256Mi")
	assert.True(t, expectedCPU.Equal(requests[corev1.ResourceCPU]))
	assert.True(t, expectedMem.Equal(requests[corev1.ResourceMemory]))

	// Check resource limits
	limits := pod.Spec.Containers[0].Resources.Limits
	expectedCPULimit := resource.MustParse("500m")
	expectedMemLimit := resource.MustParse("512Mi")
	assert.True(t, expectedCPULimit.Equal(limits[corev1.ResourceCPU]))
	assert.True(t, expectedMemLimit.Equal(limits[corev1.ResourceMemory]))
}

func TestResourceCalculations(t *testing.T) {
	tests := []struct {
		name           string
		usageCPU       string
		usageMemory    string
		cpuMultiplier  float64
		memMultiplier  float64
		minExpectedCPU string
		minExpectedMem string
	}{
		{
			name:           "normal resource calculation",
			usageCPU:       "100m",
			usageMemory:    "128Mi",
			cpuMultiplier:  1.2,
			memMultiplier:  1.2,
			minExpectedCPU: "120m",
			minExpectedMem: "153Mi",
		},
		{
			name:           "high usage requires more resources",
			usageCPU:       "150m",
			usageMemory:    "200Mi",
			cpuMultiplier:  1.5,
			memMultiplier:  1.5,
			minExpectedCPU: "225m",
			minExpectedMem: "300Mi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usageCPUQuantity := resource.MustParse(tt.usageCPU)
			usageMemQuantity := resource.MustParse(tt.usageMemory)

			// Calculate recommended CPU
			usageCPUMillis := usageCPUQuantity.MilliValue()
			recommendedCPUMillis := int64(float64(usageCPUMillis) * tt.cpuMultiplier)
			recommendedCPU := resource.NewMilliQuantity(recommendedCPUMillis, resource.DecimalSI)

			// Calculate recommended Memory
			usageMemBytes := usageMemQuantity.Value()
			recommendedMemBytes := int64(float64(usageMemBytes) * tt.memMultiplier)
			recommendedMem := resource.NewQuantity(recommendedMemBytes, resource.BinarySI)

			minExpectedCPU := resource.MustParse(tt.minExpectedCPU)
			minExpectedMem := resource.MustParse(tt.minExpectedMem)

			assert.True(t, recommendedCPU.Cmp(minExpectedCPU) >= 0,
				"Recommended CPU %s should be >= expected %s", recommendedCPU.String(), minExpectedCPU.String())
			assert.True(t, recommendedMem.Cmp(minExpectedMem) >= 0,
				"Recommended Memory %s should be >= expected %s", recommendedMem.String(), minExpectedMem.String())
		})
	}
}

// Helper functions for tests

func shouldSkipPod(pod *corev1.Pod) (bool, string) {
	// Check annotations
	if pod.Annotations != nil {
		if disable, exists := pod.Annotations["rightsizer.io/disable"]; exists && disable == "true" {
			return true, "disabled by annotation"
		}
	}

	// Check labels
	if pod.Labels != nil {
		if disable, exists := pod.Labels["rightsizer"]; exists && disable == "disabled" {
			return true, "disabled by label"
		}
	}

	// Check system namespaces
	if isSystemNamespace(pod.Namespace) {
		return true, "system namespace"
	}

	return false, ""
}

func isSystemNamespace(namespace string) bool {
	cfg := config.GetDefaults()
	for _, ns := range cfg.SystemNamespaces {
		if namespace == ns {
			return true
		}
	}
	return false
}

func createTestPod(name, namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "nginx:alpine",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
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

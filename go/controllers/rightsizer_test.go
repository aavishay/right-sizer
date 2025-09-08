package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"right-sizer/config"
)

func TestShouldSkipPod(t *testing.T) {
	tests := []struct {
		name       string
		pod        *corev1.Pod
		shouldSkip bool
		reason     string
	}{
		{
			name: "pod in system namespace",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "system-pod",
					Namespace: "kube-system",
				},
			},
			shouldSkip: true,
			reason:     "system namespace",
		},
		{
			name: "pod with disable annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "disabled-pod",
					Namespace: "default",
					Annotations: map[string]string{
						"rightsizer.io/disable": "true",
					},
				},
			},
			shouldSkip: true,
			reason:     "disabled",
		},
		{
			name: "pod with disable label",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "disabled-pod",
					Namespace: "default",
					Labels: map[string]string{
						"rightsizer": "disabled",
					},
				},
			},
			shouldSkip: true,
			reason:     "disabled",
		},
		{
			name: "pod not running",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pending-pod",
					Namespace: "default",
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			},
			shouldSkip: true,
			reason:     "not running",
		},
		{
			name: "normal running pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "normal-pod",
					Namespace: "default",
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
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
							},
						},
					},
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
				assert.Contains(t, reason, tt.reason)
			}
		})
	}
}

func TestCalculateNewResources(t *testing.T) {
	tests := []struct {
		name           string
		cpuUsage       string
		memoryUsage    string
		cpuMultiplier  float64
		memMultiplier  float64
		expectedMinCPU string
		expectedMinMem string
	}{
		{
			name:           "normal usage calculation",
			cpuUsage:       "80m",
			memoryUsage:    "100Mi",
			cpuMultiplier:  1.2,
			memMultiplier:  1.2,
			expectedMinCPU: "96m",
			expectedMinMem: "120Mi",
		},
		{
			name:           "high usage calculation",
			cpuUsage:       "200m",
			memoryUsage:    "300Mi",
			cpuMultiplier:  1.5,
			memMultiplier:  1.5,
			expectedMinCPU: "300m",
			expectedMinMem: "450Mi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpuUsage := resource.MustParse(tt.cpuUsage)
			memoryUsage := resource.MustParse(tt.memoryUsage)

			// Calculate new CPU request
			cpuMillis := cpuUsage.MilliValue()
			newCPUMillis := int64(float64(cpuMillis) * tt.cpuMultiplier)
			newCPURequest := resource.NewMilliQuantity(newCPUMillis, resource.DecimalSI)

			// Calculate new memory request
			memBytes := memoryUsage.Value()
			newMemBytes := int64(float64(memBytes) * tt.memMultiplier)
			newMemRequest := resource.NewQuantity(newMemBytes, resource.BinarySI)

			expectedMinCPU := resource.MustParse(tt.expectedMinCPU)
			expectedMinMem := resource.MustParse(tt.expectedMinMem)

			assert.GreaterOrEqual(t, newCPURequest.Cmp(expectedMinCPU), 0,
				"CPU calculation: got %s, expected >= %s", newCPURequest.String(), expectedMinCPU.String())
			assert.GreaterOrEqual(t, newMemRequest.Cmp(expectedMinMem), 0,
				"Memory calculation: got %s, expected >= %s", newMemRequest.String(), expectedMinMem.String())
		})
	}
}

func TestResourceMultipliers(t *testing.T) {
	multipliers := ResourceMultipliers{
		CPURequest:    1.2,
		MemoryRequest: 1.3,
		CPULimit:      2.0,
		MemoryLimit:   2.5,
	}

	assert.Equal(t, 1.2, multipliers.CPURequest)
	assert.Equal(t, 1.3, multipliers.MemoryRequest)
	assert.Equal(t, 2.0, multipliers.CPULimit)
	assert.Equal(t, 2.5, multipliers.MemoryLimit)

	// Test validation
	assert.Greater(t, multipliers.CPURequest, 1.0)
	assert.Greater(t, multipliers.MemoryRequest, 1.0)
	assert.GreaterOrEqual(t, multipliers.CPULimit, 1.0)
	assert.GreaterOrEqual(t, multipliers.MemoryLimit, 1.0)
}

// Helper functions for testing

func shouldSkipPod(pod *corev1.Pod) (bool, string) {
	// System namespace check
	cfg := config.GetDefaults()
	for _, ns := range cfg.SystemNamespaces {
		if pod.Namespace == ns {
			return true, "system namespace"
		}
	}

	// Disabled annotation check
	if pod.Annotations != nil {
		if disable, exists := pod.Annotations["rightsizer.io/disable"]; exists && disable == "true" {
			return true, "disabled by annotation"
		}
	}

	// Disabled label check
	if pod.Labels != nil {
		if disable, exists := pod.Labels["rightsizer"]; exists && disable == "disabled" {
			return true, "disabled by label"
		}
	}

	// Running state check
	if pod.Status.Phase != corev1.PodRunning {
		return true, "not running"
	}

	// Resource requirements check
	if len(pod.Spec.Containers) == 0 {
		return true, "no containers"
	}

	container := pod.Spec.Containers[0]
	if container.Resources.Requests == nil {
		return true, "no resource requests"
	}

	return false, ""
}

// ResourceMultipliers represents multipliers for calculating new resource values
type ResourceMultipliers struct {
	CPURequest    float64
	MemoryRequest float64
	CPULimit      float64
	MemoryLimit   float64
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

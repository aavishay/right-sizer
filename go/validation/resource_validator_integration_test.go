package validation

import (
	"context"
	"testing"

	"right-sizer/config"
	"right-sizer/metrics"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clientfake "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func createTestValidator(objects []runtime.Object) *ResourceValidator {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	client := clientfake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()
	clientset := k8sfake.NewSimpleClientset(objects...)
	cfg := &config.Config{
		MinCPURequest:    10,
		MinMemoryRequest: 10,
		MaxCPULimit:      10000,
		MaxMemoryLimit:   10000,
		SafetyThreshold:  0.1,
	}
	// metrics can be nil for these tests as we are testing logic, or we can mock it if needed.
	// For now, nil is fine as the validator code handles nil metrics.
	var m *metrics.OperatorMetrics = nil

	return NewResourceValidator(client, clientset, cfg, m)
}

func TestValidateNodeCapacity(t *testing.T) {
	nodeName := "worker-1"
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("2000m"),
				corev1.ResourceMemory: resource.MustParse("4Gi"),
			},
		},
	}

	validator := createTestValidator([]runtime.Object{node})

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: nodeName,
			Containers: []corev1.Container{
				{
					Name: "app",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("200Mi"),
						},
					},
				},
			},
		},
	}

	ctx := context.TODO()

	tests := []struct {
		name         string
		newResources corev1.ResourceRequirements
		expectError  bool
	}{
		{
			name: "Fits within capacity",
			newResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1000m"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
			},
			expectError: false,
		},
		{
			name: "Exceeds CPU capacity",
			newResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("3000m"),
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
			expectError: true,
		},
		{
			name: "Exceeds Memory capacity",
			newResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("5Gi"),
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ValidationResult{Valid: true}
			validator.validateNodeCapacity(ctx, pod, tt.newResources, result)
			if tt.expectError {
				assert.False(t, result.IsValid(), "Expected validation error but got valid")
				assert.NotEmpty(t, result.Errors, "Expected error messages")
			} else {
				assert.True(t, result.IsValid(), "Expected valid result but got errors: %v", result.Errors)
			}
		})
	}
}

func TestValidateResourceQuota(t *testing.T) {
	namespace := "test-ns"
	quota := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-quota",
			Namespace: namespace,
		},
		Status: corev1.ResourceQuotaStatus{
			Hard: corev1.ResourceList{
				"requests.cpu":    resource.MustParse("2000m"),
				"requests.memory": resource.MustParse("4Gi"),
			},
			Used: corev1.ResourceList{
				"requests.cpu":    resource.MustParse("500m"),
				"requests.memory": resource.MustParse("1Gi"),
			},
		},
	}

	validator := createTestValidator([]runtime.Object{quota})

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: namespace,
		},
	}

	ctx := context.TODO()

	tests := []struct {
		name         string
		newResources corev1.ResourceRequirements
		expectError  bool
	}{
		{
			name: "Fits within quota",
			newResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("1000m"), // 500 used + 1000 < 2000
					corev1.ResourceMemory: resource.MustParse("2Gi"),   // 1Gi used + 2Gi < 4Gi
				},
			},
			expectError: false,
		},
		{
			name: "Exceeds CPU quota",
			newResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("2000m"), // 500 + 2000 > 2000
					corev1.ResourceMemory: resource.MustParse("1Gi"),
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ValidationResult{Valid: true}
			validator.validateResourceQuota(ctx, pod, tt.newResources, result)
			if tt.expectError {
				assert.False(t, result.IsValid())
				assert.NotEmpty(t, result.Errors)
			} else {
				assert.True(t, result.IsValid())
			}
		})
	}
}

func TestValidateLimitRanges(t *testing.T) {
	namespace := "limit-ns"
	limitRange := &corev1.LimitRange{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-limit",
			Namespace: namespace,
		},
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{
				{
					Type: corev1.LimitTypeContainer,
					Min: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
					Max: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1000m"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				},
			},
		},
	}

	validator := createTestValidator([]runtime.Object{limitRange})

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: namespace,
		},
	}

	ctx := context.TODO()

	tests := []struct {
		name         string
		newResources corev1.ResourceRequirements
		expectError  bool
	}{
		{
			name: "Within ranges",
			newResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("500m"),
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
			expectError: false,
		},
		{
			name: "Below minimum CPU",
			newResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("50m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
			expectError: true,
		},
		{
			name: "Above maximum Memory",
			newResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("2Gi"),
				},
			},
			expectError: true, // While validateLimitRanges checks Limits generally, the implementation checks requests against min, and limits against max. Wait, checkMinimumConstraints checks requests. checkMaximumConstraints checks limits.
			// Let's verify the implementation logic to be sure.
		},
	}
	// Note on "Above maximum Memory" case: in `resource_validator.go`, `checkMaximumConstraints` checks `resources.Limits`.
	// Since my test case above only supplies Requests, let's update it to supply Limits for the Max check.

	tests[2].newResources = corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("200m"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ValidationResult{Valid: true}
			validator.validateLimitRanges(ctx, pod, tt.newResources, result)
			if tt.expectError {
				assert.False(t, result.IsValid())
				assert.NotEmpty(t, result.Errors)
			} else {
				assert.True(t, result.IsValid())
			}
		})
	}
}

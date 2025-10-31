package metrics

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestMetrics(t *testing.T) {
	tests := []struct {
		name    string
		cpu     *resource.Quantity
		memory  *resource.Quantity
		wantErr bool
	}{
		{
			name:    "valid metrics",
			cpu:     resource.NewMilliQuantity(100, resource.DecimalSI),
			memory:  resource.NewQuantity(128*1024*1024, resource.BinarySI),
			wantErr: false,
		},
		{
			name:    "zero metrics",
			cpu:     resource.NewMilliQuantity(0, resource.DecimalSI),
			memory:  resource.NewQuantity(0, resource.BinarySI),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cpu == nil && !tt.wantErr {
				t.Errorf("CPU should not be nil for valid test")
			}
			if tt.memory == nil && !tt.wantErr {
				t.Errorf("Memory should not be nil for valid test")
			}
		})
	}
}

func TestPodMetrics(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "test-container",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(128*1024*1024, resource.BinarySI),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(500, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(512*1024*1024, resource.BinarySI),
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name string
		pod  *corev1.Pod
	}{
		{
			name: "valid pod with containers",
			pod:  pod,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.pod == nil {
				t.Errorf("Pod should not be nil")
			}
			if len(tt.pod.Spec.Containers) == 0 {
				t.Errorf("Pod should have containers")
			}
		})
	}
}

func TestMetricsAverage(t *testing.T) {
	metrics := []int64{100, 200, 300}

	tests := []struct {
		name     string
		values   []int64
		expected int64
	}{
		{
			name:     "average of metrics",
			values:   metrics,
			expected: 200,
		},
		{
			name:     "single metric",
			values:   []int64{100},
			expected: 100,
		},
		{
			name:     "empty metrics",
			values:   []int64{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sum int64
			for _, v := range tt.values {
				sum += v
			}

			var avg int64
			if len(tt.values) > 0 {
				avg = sum / int64(len(tt.values))
			}

			if avg != tt.expected {
				t.Errorf("got %d, want %d", avg, tt.expected)
			}
		})
	}
}

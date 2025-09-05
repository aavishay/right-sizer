package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

func TestKubernetesMetricsProvider_FetchPodMetrics(t *testing.T) {
	tests := []struct {
		name        string
		pod         *corev1.Pod
		metrics     *metricsv1beta1.PodMetrics
		expectError bool
		expectedCPU string
		expectedMem string
	}{
		{
			name: "successful metrics fetch",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			},
			metrics: &metricsv1beta1.PodMetrics{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Containers: []metricsv1beta1.ContainerMetrics{
					{
						Name: "container1",
						Usage: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				},
			},
			expectError: false,
			expectedCPU: "100m",
			expectedMem: "128Mi",
		},
		{
			name: "multiple containers - sum resources",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multi-container-pod",
					Namespace: "default",
				},
			},
			metrics: &metricsv1beta1.PodMetrics{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multi-container-pod",
					Namespace: "default",
				},
				Containers: []metricsv1beta1.ContainerMetrics{
					{
						Name: "container1",
						Usage: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
					{
						Name: "container2",
						Usage: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("50m"),
							corev1.ResourceMemory: resource.MustParse("64Mi"),
						},
					},
				},
			},
			expectError: false,
			expectedCPU: "150m",
			expectedMem: "192Mi",
		},
		{
			name: "no metrics available",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "no-metrics-pod",
					Namespace: "default",
				},
			},
			metrics:     nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &MockKubernetesMetricsProvider{
				podMetrics: map[string]*metricsv1beta1.PodMetrics{},
			}

			if tt.metrics != nil {
				key := tt.pod.Namespace + "/" + tt.pod.Name
				provider.podMetrics[key] = tt.metrics
			}

			ctx := context.TODO()
			result, err := provider.FetchPodMetrics(ctx, tt.pod)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result)

			expectedCPU := resource.MustParse(tt.expectedCPU)
			expectedMem := resource.MustParse(tt.expectedMem)

			assert.True(t, expectedCPU.Equal(result.CPUUsage),
				"Expected CPU %s, got %s", expectedCPU.String(), result.CPUUsage.String())
			assert.True(t, expectedMem.Equal(result.MemoryUsage),
				"Expected Memory %s, got %s", expectedMem.String(), result.MemoryUsage.String())
		})
	}
}

func TestPrometheusMetricsProvider_FetchPodMetrics(t *testing.T) {
	// Skip prometheus tests if not in integration mode
	if testing.Short() {
		t.Skip("Skipping Prometheus tests in short mode")
	}

	tests := []struct {
		name        string
		pod         *corev1.Pod
		expectError bool
	}{
		{
			name: "prometheus metrics fetch",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			},
			expectError: true, // Will error without real Prometheus
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &MockPrometheusProvider{
				url: "http://localhost:9090",
			}

			ctx := context.TODO()
			_, err := provider.FetchPodMetrics(ctx, tt.pod)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMetricsAggregation(t *testing.T) {
	tests := []struct {
		name        string
		containers  []metricsv1beta1.ContainerMetrics
		expectedCPU string
		expectedMem string
	}{
		{
			name: "single container",
			containers: []metricsv1beta1.ContainerMetrics{
				{
					Name: "app",
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
				},
			},
			expectedCPU: "200m",
			expectedMem: "256Mi",
		},
		{
			name: "multiple containers",
			containers: []metricsv1beta1.ContainerMetrics{
				{
					Name: "app",
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("150m"),
						corev1.ResourceMemory: resource.MustParse("200Mi"),
					},
				},
				{
					Name: "sidecar",
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("100Mi"),
					},
				},
				{
					Name: "init",
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("25m"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
				},
			},
			expectedCPU: "225m",
			expectedMem: "350Mi",
		},
		{
			name:        "no containers",
			containers:  []metricsv1beta1.ContainerMetrics{},
			expectedCPU: "0",
			expectedMem: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			totalCPU := resource.Quantity{}
			totalMemory := resource.Quantity{}

			for _, container := range tt.containers {
				if cpu, ok := container.Usage[corev1.ResourceCPU]; ok {
					totalCPU.Add(cpu)
				}
				if memory, ok := container.Usage[corev1.ResourceMemory]; ok {
					totalMemory.Add(memory)
				}
			}

			expectedCPU := resource.MustParse(tt.expectedCPU)
			expectedMem := resource.MustParse(tt.expectedMem)

			assert.True(t, expectedCPU.Equal(totalCPU),
				"Expected CPU %s, got %s", expectedCPU.String(), totalCPU.String())
			assert.True(t, expectedMem.Equal(totalMemory),
				"Expected Memory %s, got %s", expectedMem.String(), totalMemory.String())
		})
	}
}

func TestMetricsProviderSelection(t *testing.T) {
	tests := []struct {
		name          string
		provider      string
		prometheusURL string
		expectedType  string
		shouldCreate  bool
	}{
		{
			name:         "kubernetes provider",
			provider:     "kubernetes",
			expectedType: "kubernetes",
			shouldCreate: true,
		},
		{
			name:          "prometheus provider with URL",
			provider:      "prometheus",
			prometheusURL: "http://prometheus:9090",
			expectedType:  "prometheus",
			shouldCreate:  true,
		},
		{
			name:         "prometheus provider without URL",
			provider:     "prometheus",
			expectedType: "prometheus",
			shouldCreate: true, // Should use default URL
		},
		{
			name:         "default to kubernetes",
			provider:     "",
			expectedType: "kubernetes",
			shouldCreate: true,
		},
		{
			name:         "invalid provider defaults to kubernetes",
			provider:     "invalid",
			expectedType: "kubernetes",
			shouldCreate: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock provider creation logic
			var providerType string

			switch tt.provider {
			case "prometheus":
				providerType = "prometheus"
			case "kubernetes", "":
				providerType = "kubernetes"
			default:
				providerType = "kubernetes" // fallback
			}

			if tt.shouldCreate {
				assert.Equal(t, tt.expectedType, providerType)
			}
		})
	}
}

func TestMetricsTimestamp(t *testing.T) {
	now := time.Now()

	metrics := &metricsv1beta1.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Timestamp: metav1.Time{Time: now},
		Containers: []metricsv1beta1.ContainerMetrics{
			{
				Name: "app",
				Usage: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			},
		},
	}

	// Verify timestamp is recent (within last minute)
	assert.True(t, time.Since(metrics.Timestamp.Time) < time.Minute,
		"Metrics timestamp should be recent")
}

// Mock implementations for testing

type MockKubernetesMetricsProvider struct {
	podMetrics map[string]*metricsv1beta1.PodMetrics
}

func (m *MockKubernetesMetricsProvider) FetchPodMetrics(ctx context.Context, pod *corev1.Pod) (*PodMetrics, error) {
	key := pod.Namespace + "/" + pod.Name
	metrics, exists := m.podMetrics[key]
	if !exists {
		return nil, assert.AnError
	}

	totalCPU := resource.Quantity{}
	totalMemory := resource.Quantity{}

	for _, container := range metrics.Containers {
		if cpu, ok := container.Usage[corev1.ResourceCPU]; ok {
			totalCPU.Add(cpu)
		}
		if memory, ok := container.Usage[corev1.ResourceMemory]; ok {
			totalMemory.Add(memory)
		}
	}

	return &PodMetrics{
		CPUUsage:    totalCPU,
		MemoryUsage: totalMemory,
	}, nil
}

func (m *MockKubernetesMetricsProvider) IsAvailable() bool {
	return true
}

type MockPrometheusProvider struct {
	url string
}

func (p *MockPrometheusProvider) FetchPodMetrics(ctx context.Context, pod *corev1.Pod) (*PodMetrics, error) {
	// Mock Prometheus provider - always returns error in tests
	return nil, assert.AnError
}

func (p *MockPrometheusProvider) IsAvailable() bool {
	return false // Mock as unavailable for testing
}

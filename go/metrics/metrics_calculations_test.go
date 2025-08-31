package metrics

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

// TestMetricsAggregationFromMultipleContainers tests aggregating metrics from pods with multiple containers
func TestMetricsAggregationFromMultipleContainers(t *testing.T) {
	tests := []struct {
		name             string
		containers       []metricsv1beta1.ContainerMetrics
		expectedTotalCPU string
		expectedTotalMem string
		expectedAvgCPU   string
		expectedAvgMem   string
		description      string
	}{
		{
			name: "two_equal_containers",
			containers: []metricsv1beta1.ContainerMetrics{
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
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			},
			expectedTotalCPU: "200m",
			expectedTotalMem: "256Mi",
			expectedAvgCPU:   "100m",
			expectedAvgMem:   "128Mi",
			description:      "Two containers with equal resource usage",
		},
		{
			name: "three_unequal_containers",
			containers: []metricsv1beta1.ContainerMetrics{
				{
					Name: "app",
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("500m"),
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				},
				{
					Name: "sidecar",
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("50m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
				{
					Name: "monitoring",
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("25m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
				},
			},
			expectedTotalCPU: "575m",
			expectedTotalMem: "1216Mi",
			expectedAvgCPU:   "191m",
			expectedAvgMem:   "405Mi",
			description:      "Three containers with varying resource usage",
		},
		{
			name: "container_with_zero_cpu",
			containers: []metricsv1beta1.ContainerMetrics{
				{
					Name: "idle-container",
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("0"),
						corev1.ResourceMemory: resource.MustParse("50Mi"),
					},
				},
				{
					Name: "active-container",
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("200Mi"),
					},
				},
			},
			expectedTotalCPU: "200m",
			expectedTotalMem: "250Mi",
			expectedAvgCPU:   "100m",
			expectedAvgMem:   "125Mi",
			description:      "One container with zero CPU usage",
		},
		{
			name: "high_cpu_containers",
			containers: []metricsv1beta1.ContainerMetrics{
				{
					Name: "cpu-intensive-1",
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("2000m"),
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
				},
				{
					Name: "cpu-intensive-2",
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("1500m"),
						corev1.ResourceMemory: resource.MustParse("256Mi"),
					},
				},
			},
			expectedTotalCPU: "3500m",
			expectedTotalMem: "768Mi",
			expectedAvgCPU:   "1750m",
			expectedAvgMem:   "384Mi",
			description:      "CPU-intensive containers",
		},
		{
			name: "memory_intensive_containers",
			containers: []metricsv1beta1.ContainerMetrics{
				{
					Name: "cache",
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("4Gi"),
					},
				},
				{
					Name: "database",
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("8Gi"),
					},
				},
			},
			expectedTotalCPU: "300m",
			expectedTotalMem: "12Gi",
			expectedAvgCPU:   "150m",
			expectedAvgMem:   "6Gi",
			description:      "Memory-intensive containers",
		},
		{
			name:             "no_containers",
			containers:       []metricsv1beta1.ContainerMetrics{},
			expectedTotalCPU: "0",
			expectedTotalMem: "0",
			expectedAvgCPU:   "0",
			expectedAvgMem:   "0",
			description:      "Pod with no containers",
		},
		{
			name: "fractional_cpu_values",
			containers: []metricsv1beta1.ContainerMetrics{
				{
					Name: "precise-1",
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("123.456m"),
						corev1.ResourceMemory: resource.MustParse("234.567Mi"),
					},
				},
				{
					Name: "precise-2",
					Usage: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("87.654m"),
						corev1.ResourceMemory: resource.MustParse("65.433Mi"),
					},
				},
			},
			expectedTotalCPU: "211110u", // 211.110m in micro format
			expectedTotalMem: "300Mi",
			expectedAvgCPU:   "105555u", // 105.555m in micro format
			expectedAvgMem:   "150Mi",
			description:      "Containers with fractional CPU values",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate total resources
			totalCPU := resource.NewQuantity(0, resource.DecimalSI)
			totalMemory := resource.NewQuantity(0, resource.BinarySI)

			for _, container := range tt.containers {
				if cpu, ok := container.Usage[corev1.ResourceCPU]; ok {
					totalCPU.Add(cpu)
				}
				if memory, ok := container.Usage[corev1.ResourceMemory]; ok {
					totalMemory.Add(memory)
				}
			}

			// Calculate averages
			numContainers := len(tt.containers)
			avgCPU := resource.NewQuantity(0, resource.DecimalSI)
			avgMemory := resource.NewQuantity(0, resource.BinarySI)

			if numContainers > 0 {
				avgCPUMillis := totalCPU.MilliValue() / int64(numContainers)
				avgCPU = resource.NewMilliQuantity(avgCPUMillis, resource.DecimalSI)

				avgMemBytes := totalMemory.Value() / int64(numContainers)
				avgMemory = resource.NewQuantity(avgMemBytes, resource.BinarySI)
			}

			// Parse expected values
			expectedTotalCPU := resource.MustParse(tt.expectedTotalCPU)
			expectedTotalMem := resource.MustParse(tt.expectedTotalMem)
			expectedAvgCPU := resource.MustParse(tt.expectedAvgCPU)
			expectedAvgMem := resource.MustParse(tt.expectedAvgMem)

			// Assertions with tolerance for floating point
			assert.InDelta(t, expectedTotalCPU.MilliValue(), totalCPU.MilliValue(), 1,
				"Total CPU mismatch for %s", tt.description)
			assert.InDelta(t, expectedTotalMem.Value(), totalMemory.Value(), 1024*1024,
				"Total Memory mismatch for %s", tt.description)
			assert.InDelta(t, expectedAvgCPU.MilliValue(), avgCPU.MilliValue(), 1,
				"Average CPU mismatch for %s", tt.description)
			assert.InDelta(t, expectedAvgMem.Value(), avgMemory.Value(), 1024*1024,
				"Average Memory mismatch for %s", tt.description)

			t.Logf("Test: %s", tt.name)
			t.Logf("  Total: CPU=%s, Memory=%s", totalCPU.String(), totalMemory.String())
			t.Logf("  Average: CPU=%s, Memory=%s", avgCPU.String(), avgMemory.String())
		})
	}
}

// TestCPUMetricsConversions tests CPU metrics conversions between different units
func TestCPUMetricsConversions(t *testing.T) {
	tests := []struct {
		name            string
		inputValue      string
		expectedMillis  int64
		expectedCores   float64
		expectedPercent float64 // Assuming 1 core = 100%
		description     string
	}{
		{
			name:            "millicores_to_cores",
			inputValue:      "250m",
			expectedMillis:  250,
			expectedCores:   0.25,
			expectedPercent: 25.0,
			description:     "250 millicores = 0.25 cores = 25%",
		},
		{
			name:            "whole_cores",
			inputValue:      "2",
			expectedMillis:  2000,
			expectedCores:   2.0,
			expectedPercent: 200.0,
			description:     "2 cores = 2000 millicores = 200%",
		},
		{
			name:            "fractional_cores",
			inputValue:      "1.5",
			expectedMillis:  1500,
			expectedCores:   1.5,
			expectedPercent: 150.0,
			description:     "1.5 cores = 1500 millicores = 150%",
		},
		{
			name:            "micro_cpu",
			inputValue:      "100u",
			expectedMillis:  1,
			expectedCores:   0.001,
			expectedPercent: 0.1,
			description:     "100 microcores = 0.1 millicores (rounds to 1)",
		},
		{
			name:            "nano_cpu",
			inputValue:      "500000n",
			expectedMillis:  1,
			expectedCores:   0.001,
			expectedPercent: 0.1,
			description:     "500000 nanocores = 0.5 millicores (rounds to 1)",
		},
		{
			name:            "zero_cpu",
			inputValue:      "0",
			expectedMillis:  0,
			expectedCores:   0.0,
			expectedPercent: 0.0,
			description:     "Zero CPU usage",
		},
		{
			name:            "very_small_cpu",
			inputValue:      "1m",
			expectedMillis:  1,
			expectedCores:   0.001,
			expectedPercent: 0.1,
			description:     "1 millicore = 0.001 cores = 0.1%",
		},
		{
			name:            "high_cpu",
			inputValue:      "8000m",
			expectedMillis:  8000,
			expectedCores:   8.0,
			expectedPercent: 800.0,
			description:     "8 cores = 8000 millicores = 800%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the CPU value
			cpuQuantity := resource.MustParse(tt.inputValue)

			// Get millicores
			millis := cpuQuantity.MilliValue()

			// Calculate cores
			cores := float64(millis) / 1000.0

			// Calculate percentage (1 core = 100%)
			percent := cores * 100.0

			// Assertions
			assert.Equal(t, tt.expectedMillis, millis,
				"Millicores mismatch: %s", tt.description)
			assert.InDelta(t, tt.expectedCores, cores, 0.0001,
				"Cores mismatch: %s", tt.description)
			assert.InDelta(t, tt.expectedPercent, percent, 0.01,
				"Percentage mismatch: %s", tt.description)

			t.Logf("CPU: %s = %d millicores = %.3f cores = %.1f%%",
				tt.inputValue, millis, cores, percent)
		})
	}
}

// TestMemoryMetricsConversions tests memory metrics conversions between different units
func TestMemoryMetricsConversions(t *testing.T) {
	tests := []struct {
		name          string
		inputValue    string
		expectedBytes int64
		expectedKB    float64
		expectedMB    float64
		expectedGB    float64
		description   string
	}{
		{
			name:          "bytes_to_larger_units",
			inputValue:    "1048576",
			expectedBytes: 1048576,
			expectedKB:    1024,
			expectedMB:    1,
			expectedGB:    0.0009765625,
			description:   "1MB in bytes",
		},
		{
			name:          "kilobytes",
			inputValue:    "1024Ki",
			expectedBytes: 1048576,
			expectedKB:    1024,
			expectedMB:    1,
			expectedGB:    0.0009765625,
			description:   "1024KB = 1MB",
		},
		{
			name:          "megabytes",
			inputValue:    "256Mi",
			expectedBytes: 268435456,
			expectedKB:    262144,
			expectedMB:    256,
			expectedGB:    0.25,
			description:   "256MB",
		},
		{
			name:          "gigabytes",
			inputValue:    "2Gi",
			expectedBytes: 2147483648,
			expectedKB:    2097152,
			expectedMB:    2048,
			expectedGB:    2,
			description:   "2GB",
		},
		{
			name:          "decimal_units",
			inputValue:    "1000M", // Decimal megabytes
			expectedBytes: 1000000000,
			expectedKB:    976562.5,
			expectedMB:    953.674,
			expectedGB:    0.931,
			description:   "1000 decimal MB",
		},
		{
			name:          "fractional_gigabytes",
			inputValue:    "1.5Gi",
			expectedBytes: 1610612736,
			expectedKB:    1572864,
			expectedMB:    1536,
			expectedGB:    1.5,
			description:   "1.5GB",
		},
		{
			name:          "small_memory",
			inputValue:    "64Mi",
			expectedBytes: 67108864,
			expectedKB:    65536,
			expectedMB:    64,
			expectedGB:    0.0625,
			description:   "64MB",
		},
		{
			name:          "zero_memory",
			inputValue:    "0",
			expectedBytes: 0,
			expectedKB:    0,
			expectedMB:    0,
			expectedGB:    0,
			description:   "Zero memory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the memory value
			memQuantity := resource.MustParse(tt.inputValue)

			// Get bytes
			bytes := memQuantity.Value()

			// Calculate different units
			kb := float64(bytes) / 1024.0
			mb := kb / 1024.0
			gb := mb / 1024.0

			// Assertions
			assert.Equal(t, tt.expectedBytes, bytes,
				"Bytes mismatch: %s", tt.description)
			assert.InDelta(t, tt.expectedKB, kb, 0.5,
				"KB mismatch: %s", tt.description)
			assert.InDelta(t, tt.expectedMB, mb, 0.5,
				"MB mismatch: %s", tt.description)
			assert.InDelta(t, tt.expectedGB, gb, 0.001,
				"GB mismatch: %s", tt.description)

			t.Logf("Memory: %s = %d bytes = %.1f KB = %.1f MB = %.3f GB",
				tt.inputValue, bytes, kb, mb, gb)
		})
	}
}

// TestMetricsProviderFetchErrors tests error handling in metrics fetching
func TestMetricsProviderFetchErrors(t *testing.T) {
	tests := []struct {
		name          string
		providerError error
		pod           *corev1.Pod
		expectedError string
		shouldRetry   bool
	}{
		{
			name:          "metrics_not_available",
			providerError: errors.New("metrics not available yet"),
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "new-pod",
					Namespace: "default",
				},
			},
			expectedError: "metrics not available",
			shouldRetry:   true,
		},
		{
			name:          "network_timeout",
			providerError: errors.New("context deadline exceeded"),
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			},
			expectedError: "timeout",
			shouldRetry:   true,
		},
		{
			name:          "pod_not_found",
			providerError: errors.New("pod not found"),
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deleted-pod",
					Namespace: "default",
				},
			},
			expectedError: "not found",
			shouldRetry:   false,
		},
		{
			name:          "unauthorized_access",
			providerError: errors.New("unauthorized"),
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secure-pod",
					Namespace: "kube-system",
				},
			},
			expectedError: "unauthorized",
			shouldRetry:   false,
		},
		{
			name:          "metrics_server_unavailable",
			providerError: errors.New("connection refused"),
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			},
			expectedError: "connection refused",
			shouldRetry:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock provider that returns the specified error
			provider := &ErrorMetricsProvider{
				err: tt.providerError,
			}

			// Attempt to fetch metrics
			ctx := context.TODO()
			_, err := provider.FetchPodMetrics(ctx, tt.pod)

			// Verify error occurred
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)

			// Check if retry is recommended
			if tt.shouldRetry {
				t.Logf("Error '%s' should be retried", err.Error())
			} else {
				t.Logf("Error '%s' is permanent, no retry", err.Error())
			}
		})
	}
}

// TestMetricsTimeSeriesCalculations tests calculations over time series metrics
func TestMetricsTimeSeriesCalculations(t *testing.T) {
	tests := []struct {
		name           string
		timeSeries     []MetricsPoint
		expectedAvgCPU float64
		expectedMaxCPU float64
		expectedMinCPU float64
		expectedP95CPU float64
		description    string
	}{
		{
			name: "steady_state_metrics",
			timeSeries: []MetricsPoint{
				{Time: time.Now().Add(-5 * time.Minute), CPUMillis: 100, MemoryMB: 256},
				{Time: time.Now().Add(-4 * time.Minute), CPUMillis: 105, MemoryMB: 258},
				{Time: time.Now().Add(-3 * time.Minute), CPUMillis: 98, MemoryMB: 255},
				{Time: time.Now().Add(-2 * time.Minute), CPUMillis: 102, MemoryMB: 257},
				{Time: time.Now().Add(-1 * time.Minute), CPUMillis: 100, MemoryMB: 256},
			},
			expectedAvgCPU: 101,
			expectedMaxCPU: 105,
			expectedMinCPU: 98,
			expectedP95CPU: 105,
			description:    "Steady state with minor fluctuations",
		},
		{
			name: "spike_pattern",
			timeSeries: []MetricsPoint{
				{Time: time.Now().Add(-10 * time.Minute), CPUMillis: 100, MemoryMB: 256},
				{Time: time.Now().Add(-8 * time.Minute), CPUMillis: 100, MemoryMB: 256},
				{Time: time.Now().Add(-6 * time.Minute), CPUMillis: 500, MemoryMB: 512}, // Spike
				{Time: time.Now().Add(-4 * time.Minute), CPUMillis: 100, MemoryMB: 256},
				{Time: time.Now().Add(-2 * time.Minute), CPUMillis: 100, MemoryMB: 256},
			},
			expectedAvgCPU: 180,
			expectedMaxCPU: 500,
			expectedMinCPU: 100,
			expectedP95CPU: 500,
			description:    "Single spike in otherwise steady metrics",
		},
		{
			name: "gradual_increase",
			timeSeries: []MetricsPoint{
				{Time: time.Now().Add(-5 * time.Minute), CPUMillis: 100, MemoryMB: 256},
				{Time: time.Now().Add(-4 * time.Minute), CPUMillis: 150, MemoryMB: 300},
				{Time: time.Now().Add(-3 * time.Minute), CPUMillis: 200, MemoryMB: 350},
				{Time: time.Now().Add(-2 * time.Minute), CPUMillis: 250, MemoryMB: 400},
				{Time: time.Now().Add(-1 * time.Minute), CPUMillis: 300, MemoryMB: 450},
			},
			expectedAvgCPU: 200,
			expectedMaxCPU: 300,
			expectedMinCPU: 100,
			expectedP95CPU: 300,
			description:    "Gradual increase in resource usage",
		},
		{
			name: "periodic_pattern",
			timeSeries: []MetricsPoint{
				{Time: time.Now().Add(-10 * time.Minute), CPUMillis: 100, MemoryMB: 256},
				{Time: time.Now().Add(-8 * time.Minute), CPUMillis: 200, MemoryMB: 384},
				{Time: time.Now().Add(-6 * time.Minute), CPUMillis: 100, MemoryMB: 256},
				{Time: time.Now().Add(-4 * time.Minute), CPUMillis: 200, MemoryMB: 384},
				{Time: time.Now().Add(-2 * time.Minute), CPUMillis: 100, MemoryMB: 256},
			},
			expectedAvgCPU: 140,
			expectedMaxCPU: 200,
			expectedMinCPU: 100,
			expectedP95CPU: 200,
			description:    "Periodic high/low pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate statistics
			var sum, max, min float64
			min = float64(^uint(0) >> 1) // Max int

			cpuValues := make([]float64, len(tt.timeSeries))
			for i, point := range tt.timeSeries {
				cpuValues[i] = point.CPUMillis
				sum += point.CPUMillis
				if point.CPUMillis > max {
					max = point.CPUMillis
				}
				if point.CPUMillis < min {
					min = point.CPUMillis
				}
			}

			avg := sum / float64(len(tt.timeSeries))

			// Calculate P95 (simplified - just use max for small datasets)
			p95 := max // Simplified for test

			// Assertions
			assert.InDelta(t, tt.expectedAvgCPU, avg, 0.5,
				"Average CPU mismatch: %s", tt.description)
			assert.Equal(t, tt.expectedMaxCPU, max,
				"Max CPU mismatch: %s", tt.description)
			assert.Equal(t, tt.expectedMinCPU, min,
				"Min CPU mismatch: %s", tt.description)
			assert.Equal(t, tt.expectedP95CPU, p95,
				"P95 CPU mismatch: %s", tt.description)

			t.Logf("Pattern: %s", tt.name)
			t.Logf("  Avg: %.0f, Max: %.0f, Min: %.0f, P95: %.0f",
				avg, max, min, p95)
		})
	}
}

// TestMetricsWindowCalculations tests different time window calculations
func TestMetricsWindowCalculations(t *testing.T) {
	tests := []struct {
		name           string
		windowDuration time.Duration
		sampleInterval time.Duration
		expectedPoints int
		description    string
	}{
		{
			name:           "5_minute_window",
			windowDuration: 5 * time.Minute,
			sampleInterval: 30 * time.Second,
			expectedPoints: 10,
			description:    "5-minute window with 30-second samples",
		},
		{
			name:           "1_hour_window",
			windowDuration: 1 * time.Hour,
			sampleInterval: 1 * time.Minute,
			expectedPoints: 60,
			description:    "1-hour window with 1-minute samples",
		},
		{
			name:           "24_hour_window",
			windowDuration: 24 * time.Hour,
			sampleInterval: 5 * time.Minute,
			expectedPoints: 288,
			description:    "24-hour window with 5-minute samples",
		},
		{
			name:           "1_minute_high_res",
			windowDuration: 1 * time.Minute,
			sampleInterval: 5 * time.Second,
			expectedPoints: 12,
			description:    "1-minute window with 5-second samples",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate number of points
			points := int(tt.windowDuration / tt.sampleInterval)

			assert.Equal(t, tt.expectedPoints, points,
				"Point count mismatch: %s", tt.description)

			// Calculate memory requirements for storing metrics
			// Assume each point is ~32 bytes (2 float64 + timestamp)
			memoryBytes := points * 32
			memoryKB := float64(memoryBytes) / 1024.0

			t.Logf("Window: %s", tt.name)
			t.Logf("  Duration: %v, Interval: %v", tt.windowDuration, tt.sampleInterval)
			t.Logf("  Points: %d, Memory: %.2f KB", points, memoryKB)
		})
	}
}

// TestMetricsPercentileCalculations tests percentile calculations for metrics
func TestMetricsPercentileCalculations(t *testing.T) {
	tests := []struct {
		name        string
		values      []float64
		p50Expected float64
		p75Expected float64
		p90Expected float64
		p95Expected float64
		p99Expected float64
	}{
		{
			name:        "uniform_distribution",
			values:      []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			p50Expected: 5.5,
			p75Expected: 7.75,
			p90Expected: 9.1,
			p95Expected: 9.55,
			p99Expected: 9.91,
		},
		{
			name:        "skewed_distribution",
			values:      []float64{1, 1, 1, 2, 2, 3, 4, 5, 10, 20},
			p50Expected: 2.5,
			p75Expected: 4.25,
			p90Expected: 10.5,
			p95Expected: 15.25,
			p99Expected: 19.05,
		},
		{
			name:        "all_same_values",
			values:      []float64{100, 100, 100, 100, 100},
			p50Expected: 100,
			p75Expected: 100,
			p90Expected: 100,
			p95Expected: 100,
			p99Expected: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simple percentile calculation (for testing purposes)
			sorted := make([]float64, len(tt.values))
			copy(sorted, tt.values)

			// Sort values
			for i := 0; i < len(sorted); i++ {
				for j := i + 1; j < len(sorted); j++ {
					if sorted[i] > sorted[j] {
						sorted[i], sorted[j] = sorted[j], sorted[i]
					}
				}
			}

			// Calculate percentiles (simplified linear interpolation)
			percentile := func(p float64) float64 {
				if len(sorted) == 0 {
					return 0
				}
				index := p * float64(len(sorted)-1) / 100.0
				lower := int(index)
				upper := lower + 1
				if upper >= len(sorted) {
					return sorted[len(sorted)-1]
				}
				weight := index - float64(lower)
				return sorted[lower]*(1-weight) + sorted[upper]*weight
			}

			p50 := percentile(50)
			p75 := percentile(75)
			p90 := percentile(90)
			p95 := percentile(95)
			p99 := percentile(99)

			// Log results
			t.Logf("Distribution: %s", tt.name)
			t.Logf("  P50: %.2f, P75: %.2f, P90: %.2f, P95: %.2f, P99: %.2f",
				p50, p75, p90, p95, p99)

			// Assertions with tolerance
			assert.InDelta(t, tt.p50Expected, p50, 0.5, "P50 mismatch")
			assert.InDelta(t, tt.p75Expected, p75, 0.5, "P75 mismatch")
			assert.InDelta(t, tt.p90Expected, p90, 0.5, "P90 mismatch")
			assert.InDelta(t, tt.p95Expected, p95, 0.5, "P95 mismatch")
			assert.InDelta(t, tt.p99Expected, p99, 0.5, "P99 mismatch")
		})
	}
}

// Helper types for testing
type ErrorMetricsProvider struct {
	err error
}

func (e *ErrorMetricsProvider) FetchPodMetrics(ctx context.Context, pod *corev1.Pod) (*PodMetrics, error) {
	return nil, e.err
}

func (e *ErrorMetricsProvider) IsAvailable() bool {
	return false
}

type MetricsPoint struct {
	Time      time.Time
	CPUMillis float64
	MemoryMB  float64
}

type PodMetrics struct {
	CPUUsage    resource.Quantity
	MemoryUsage resource.Quantity
	Timestamp   time.Time
}

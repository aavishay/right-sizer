package metrics

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryPressureLevel_String(t *testing.T) {
	tests := []struct {
		name     string
		level    MemoryPressureLevel
		expected string
	}{
		{
			name:     "none level",
			level:    MemoryPressureNone,
			expected: "none",
		},
		{
			name:     "low level",
			level:    MemoryPressureLow,
			expected: "low",
		},
		{
			name:     "medium level",
			level:    MemoryPressureMedium,
			expected: "medium",
		},
		{
			name:     "high level",
			level:    MemoryPressureHigh,
			expected: "high",
		},
		{
			name:     "critical level",
			level:    MemoryPressureCritical,
			expected: "critical",
		},
		{
			name:     "unknown level",
			level:    MemoryPressureLevel(999),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.level.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewMemoryMetrics(t *testing.T) {
	// Reset singleton for testing
	memoryMetricsOnce = sync.Once{}
	memoryMetricsInstance = nil

	metrics := NewMemoryMetrics()
	require.NotNil(t, metrics)

	// Verify all metrics are initialized
	assert.NotNil(t, metrics.PodMemoryUsageBytes)
	assert.NotNil(t, metrics.PodMemoryWorkingSetBytes)
	assert.NotNil(t, metrics.PodMemoryRSSBytes)
	assert.NotNil(t, metrics.PodMemoryCacheBytes)
	assert.NotNil(t, metrics.PodMemorySwapBytes)
	assert.NotNil(t, metrics.PodMemoryLimitBytes)
	assert.NotNil(t, metrics.PodMemoryRequestBytes)
	assert.NotNil(t, metrics.PodMemoryUtilizationPercentage)
	assert.NotNil(t, metrics.PodMemoryRequestUtilization)
	assert.NotNil(t, metrics.PodMemoryLimitUtilization)
	assert.NotNil(t, metrics.MemoryRecommendationBytes)
	assert.NotNil(t, metrics.MemoryRecommendationRatio)
	assert.NotNil(t, metrics.MemoryPressureEvents)
	assert.NotNil(t, metrics.MemoryPressureLevel)
	assert.NotNil(t, metrics.MemoryOOMKillEvents)
	assert.NotNil(t, metrics.MemoryThrottlingEvents)
	assert.NotNil(t, metrics.MemoryTrendSlope)
	assert.NotNil(t, metrics.MemoryPeakUsageBytes)
	assert.NotNil(t, metrics.MemoryAverageUsageBytes)
	assert.NotNil(t, metrics.MemoryWasteBytes)
	assert.NotNil(t, metrics.MemoryEfficiencyScore)
	assert.NotNil(t, metrics.ContainerMemoryUsageBytes)
	assert.NotNil(t, metrics.ContainerMemoryWorkingSetBytes)
	assert.NotNil(t, metrics.ContainerMemoryRSSBytes)
	assert.NotNil(t, metrics.ContainerMemoryCacheBytes)
	assert.NotNil(t, metrics.MemoryAllocationFailures)
	assert.NotNil(t, metrics.MemoryResizeOperations)
	assert.NotNil(t, metrics.MemoryResizeSuccessRate)
}

func TestNewMemoryMetrics_Singleton(t *testing.T) {
	// Reset singleton for testing
	memoryMetricsOnce = sync.Once{}
	memoryMetricsInstance = nil

	// Create first instance
	metrics1 := NewMemoryMetrics()
	require.NotNil(t, metrics1)

	// Create second instance - should return the same instance
	metrics2 := NewMemoryMetrics()
	require.NotNil(t, metrics2)

	// Verify they are the same instance
	assert.Equal(t, metrics1, metrics2, "Should return the same singleton instance")
}

func TestUpdatePodMemoryMetrics(t *testing.T) {
	memoryMetricsOnce = sync.Once{}
	memoryMetricsInstance = nil

	metrics := NewMemoryMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.UpdatePodMemoryMetrics("default", "test-pod", "app", 512.0, 256.0, 128.0, 64.0, 0.0, 1024.0, 512.0)
	})
}

func TestRecordMemoryRecommendation(t *testing.T) {
	memoryMetricsOnce = sync.Once{}
	memoryMetricsInstance = nil

	metrics := NewMemoryMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.RecordMemoryRecommendation("default", "test-pod", "app", "adaptive", 1024.0, 512.0)
	})
}

func TestDetectAndRecordMemoryPressure(t *testing.T) {
	memoryMetricsOnce = sync.Once{}
	memoryMetricsInstance = nil

	metrics := NewMemoryMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.DetectAndRecordMemoryPressure("default", "test-pod", "app", 512.0, 1024.0)
		metrics.DetectAndRecordMemoryPressure("default", "test-pod", "app", 950.0, 1024.0)
		metrics.DetectAndRecordMemoryPressure("default", "test-pod", "app", 1000.0, 1024.0)
	})
}

func TestRecordOOMKill(t *testing.T) {
	memoryMetricsOnce = sync.Once{}
	memoryMetricsInstance = nil

	metrics := NewMemoryMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.RecordOOMKill("default", "test-pod", "app")
	})
}

func TestRecordMemoryThrottling(t *testing.T) {
	memoryMetricsOnce = sync.Once{}
	memoryMetricsInstance = nil

	metrics := NewMemoryMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.RecordMemoryThrottling("default", "test-pod", "app")
	})
}

func TestUpdateMemoryTrend(t *testing.T) {
	memoryMetricsOnce = sync.Once{}
	memoryMetricsInstance = nil

	metrics := NewMemoryMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.UpdateMemoryTrend("default", "test-pod", "app", 0.5, 1024.0, 512.0, "1h")
	})
}

func TestRecordMemoryResize(t *testing.T) {
	memoryMetricsOnce = sync.Once{}
	memoryMetricsInstance = nil

	metrics := NewMemoryMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.RecordMemoryResize("default", "test-pod", "app", "increase", "success")
		metrics.RecordMemoryResize("default", "test-pod", "app", "decrease", "failure")
	})
}

func TestUpdateContainerMemoryMetrics(t *testing.T) {
	memoryMetricsOnce = sync.Once{}
	memoryMetricsInstance = nil

	metrics := NewMemoryMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.UpdateContainerMemoryMetrics("default", "test-pod", "app", 256.0, 200.0, 150.0, 50.0)
	})
}

func TestLogMemoryPressure(t *testing.T) {
	assert.NotPanics(t, func() {
		LogMemoryPressure("default", "test-pod", "app", "high", 950.0, 1024.0, 92.8)
	})
}

func TestLogOOMKill(t *testing.T) {
	assert.NotPanics(t, func() {
		LogOOMKill("default", "test-pod", "app")
	})
}

func TestLogMemoryThrottling(t *testing.T) {
	assert.NotPanics(t, func() {
		LogMemoryThrottling("default", "test-pod", "app")
	})
}

func TestLogMemoryTrend(t *testing.T) {
	assert.NotPanics(t, func() {
		LogMemoryTrend("default", "test-pod", "app", "increasing", 0.75, 1024.0, 768.0)
	})
}

func TestLogMemoryResize(t *testing.T) {
	assert.NotPanics(t, func() {
		LogMemoryResize("default", "test-pod", "app", "increase", "success")
	})
}

func TestLogMemoryAllocation(t *testing.T) {
	assert.NotPanics(t, func() {
		LogMemoryAllocation("default", "test-pod", "app", 512.0, 1024.0, 768.0)
	})
}

func TestLogMemoryRecommendation(t *testing.T) {
	assert.NotPanics(t, func() {
		LogMemoryRecommendation("default", "test-pod", "app", 512.0, 1024.0, "adaptive sizing")
	})
}

func TestAnalyzeMemoryPattern(t *testing.T) {
	result := AnalyzeMemoryPattern("default", "test-pod", "app", []float64{512.0, 600.0, 550.0, 700.0, 650.0})
	// The function returns pattern types, not the pod name
	assert.NotEmpty(t, result, "Result should not be empty")
}

func TestFormatMemorySize(t *testing.T) {
	tests := []struct {
		name     string
		bytes    float64
		expected string
	}{
		{
			name:     "bytes",
			bytes:    512,
			expected: "512.00B",
		},
		{
			name:     "kilobytes",
			bytes:    2048,
			expected: "2.00Ki",
		},
		{
			name:     "megabytes",
			bytes:    1048576,
			expected: "1.00Mi",
		},
		{
			name:     "gigabytes",
			bytes:    1073741824,
			expected: "1.00Gi",
		},
		{
			name:     "zero",
			bytes:    0,
			expected: "0.00B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatMemorySize(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

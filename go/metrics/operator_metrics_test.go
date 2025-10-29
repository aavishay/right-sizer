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

package metrics

import (
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOperatorMetrics(t *testing.T) {
	// Reset the singleton for testing
	operatorMetricsOnce = sync.Once{}
	operatorMetricsInstance = nil

	// Create metrics instance
	metrics := NewOperatorMetrics()
	require.NotNil(t, metrics, "Metrics should not be nil")

	// Verify all metrics are initialized
	assert.NotNil(t, metrics.PodsProcessedTotal)
	assert.NotNil(t, metrics.PodsResizedTotal)
	assert.NotNil(t, metrics.CPUUsagePercent)
	assert.NotNil(t, metrics.MemoryUsagePercent)
}

func TestNewOperatorMetrics_Singleton(t *testing.T) {
	// Reset the singleton for testing
	operatorMetricsOnce = sync.Once{}
	operatorMetricsInstance = nil

	// Create first instance
	metrics1 := NewOperatorMetrics()
	require.NotNil(t, metrics1)

	// Create second instance - should return the same instance
	metrics2 := NewOperatorMetrics()
	require.NotNil(t, metrics2)

	// Verify they are the same instance
	assert.Equal(t, metrics1, metrics2, "Should return the same singleton instance")
}

func TestSafeRegister(t *testing.T) {
	// Create a test counter
	counter := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "test_safe_register_counter",
		Help: "Test counter for safe registration",
	})

	// First registration should succeed
	safeRegister(counter)

	// Second registration should not panic (already registered)
	assert.NotPanics(t, func() {
		safeRegister(counter)
	}, "Safe register should not panic on duplicate registration")

	// Cleanup
	prometheus.Unregister(counter)
}

func TestRecordPodProcessed(t *testing.T) {
	// Reset the singleton for testing
	operatorMetricsOnce = sync.Once{}
	operatorMetricsInstance = nil

	metrics := NewOperatorMetrics()
	require.NotNil(t, metrics)

	// Record pod processed - should not panic
	assert.NotPanics(t, func() {
		metrics.RecordPodProcessed()
	})
}

func TestUpdateMetrics(t *testing.T) {
	// Reset the singleton for testing
	operatorMetricsOnce = sync.Once{}
	operatorMetricsInstance = nil

	metrics := NewOperatorMetrics()
	require.NotNil(t, metrics)

	// Update all metrics - should not panic
	assert.NotPanics(t, func() {
		metrics.UpdateMetrics(100, 50, 75.5, 80.2, 150.0, 50.0, 77.8)
	})
}

func TestUpdateMetrics_NilMetrics(t *testing.T) {
	var metrics *OperatorMetrics = nil

	// Should not panic with nil metrics
	assert.NotPanics(t, func() {
		metrics.UpdateMetrics(100, 50, 75.5, 80.2, 150.0, 50.0, 77.8)
	}, "UpdateMetrics should handle nil receiver gracefully")
}

func TestTimer(t *testing.T) {
	// Create a new timer
	timer := NewTimer()
	require.NotNil(t, timer)

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Get duration
	duration := timer.Duration()
	assert.GreaterOrEqual(t, duration, 100*time.Millisecond, "Duration should be at least 100ms")
}

func TestRecordPodResized(t *testing.T) {
	operatorMetricsOnce = sync.Once{}
	operatorMetricsInstance = nil

	metrics := NewOperatorMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.RecordPodResized("default", "test-pod", "app", "cpu")
		metrics.RecordPodResized("default", "test-pod", "app", "memory")
	})
}

func TestRecordPodSkipped(t *testing.T) {
	operatorMetricsOnce = sync.Once{}
	operatorMetricsInstance = nil

	metrics := NewOperatorMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.RecordPodSkipped("default", "test-pod", "system_namespace")
		metrics.RecordPodSkipped("kube-system", "kube-proxy", "disabled")
	})
}

func TestRecordProcessingError(t *testing.T) {
	operatorMetricsOnce = sync.Once{}
	operatorMetricsInstance = nil

	metrics := NewOperatorMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.RecordProcessingError("default", "test-pod", "metrics_unavailable")
		metrics.RecordProcessingError("default", "test-pod", "validation_failed")
	})
}

func TestRecordResourceAdjustment(t *testing.T) {
	operatorMetricsOnce = sync.Once{}
	operatorMetricsInstance = nil

	metrics := NewOperatorMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.RecordResourceAdjustment("default", "test-pod", "app", "cpu", "increase", 25.5)
		metrics.RecordResourceAdjustment("default", "test-pod", "app", "memory", "decrease", 10.2)
		metrics.RecordResourceAdjustment("default", "test-pod", "app", "other", "increase", 5.0)
	})
}

func TestRecordProcessingDuration(t *testing.T) {
	operatorMetricsOnce = sync.Once{}
	operatorMetricsInstance = nil

	metrics := NewOperatorMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.RecordProcessingDuration("pod_resize", 250*time.Millisecond)
		metrics.RecordProcessingDuration("metrics_collection", 100*time.Millisecond)
	})
}

func TestRecordAPICall(t *testing.T) {
	operatorMetricsOnce = sync.Once{}
	operatorMetricsInstance = nil

	metrics := NewOperatorMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.RecordAPICall("/api/v1/pods", "GET", 50*time.Millisecond)
		metrics.RecordAPICall("/api/v1/pods/test-pod", "PATCH", 75*time.Millisecond)
	})
}

func TestRecordMetricsCollection(t *testing.T) {
	operatorMetricsOnce = sync.Once{}
	operatorMetricsInstance = nil

	metrics := NewOperatorMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.RecordMetricsCollection(150 * time.Millisecond)
	})
}

func TestRecordSafetyThresholdViolation(t *testing.T) {
	operatorMetricsOnce = sync.Once{}
	operatorMetricsInstance = nil

	metrics := NewOperatorMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.RecordSafetyThresholdViolation("default", "test-pod", "cpu")
		metrics.RecordSafetyThresholdViolation("default", "test-pod", "memory")
	})
}

func TestRecordResourceValidationError(t *testing.T) {
	operatorMetricsOnce = sync.Once{}
	operatorMetricsInstance = nil

	metrics := NewOperatorMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.RecordResourceValidationError("quota", "exceeded")
		metrics.RecordResourceValidationError("limits", "invalid")
	})
}

func TestRecordRetryAttempt(t *testing.T) {
	operatorMetricsOnce = sync.Once{}
	operatorMetricsInstance = nil

	metrics := NewOperatorMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.RecordRetryAttempt("resize_pod", 1)
		metrics.RecordRetryAttempt("resize_pod", 2)
		metrics.RecordRetryAttempt("resize_pod", 3)
	})
}

func TestRecordRetrySuccess(t *testing.T) {
	operatorMetricsOnce = sync.Once{}
	operatorMetricsInstance = nil

	metrics := NewOperatorMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.RecordRetrySuccess("resize_pod")
		metrics.RecordRetrySuccess("update_status")
	})
}

func TestUpdateClusterResourceUtilization(t *testing.T) {
	operatorMetricsOnce = sync.Once{}
	operatorMetricsInstance = nil

	metrics := NewOperatorMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.UpdateClusterResourceUtilization("cpu", "node-1", 75.5)
		metrics.UpdateClusterResourceUtilization("memory", "node-1", 82.3)
		metrics.UpdateClusterResourceUtilization("cpu", "node-2", 45.2)
	})
}

func TestUpdateNodeResourceAvailability(t *testing.T) {
	operatorMetricsOnce = sync.Once{}
	operatorMetricsInstance = nil

	metrics := NewOperatorMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.UpdateNodeResourceAvailability("cpu", "node-1", 2000.0)
		metrics.UpdateNodeResourceAvailability("memory", "node-1", 8192.0)
	})
}

func TestRecordPolicyRuleApplication(t *testing.T) {
	operatorMetricsOnce = sync.Once{}
	operatorMetricsInstance = nil

	metrics := NewOperatorMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.RecordPolicyRuleApplication("default-policy", "cpu_limit", "applied")
		metrics.RecordPolicyRuleApplication("default-policy", "memory_limit", "skipped")
	})
}

func TestRecordConfigurationReload(t *testing.T) {
	operatorMetricsOnce = sync.Once{}
	operatorMetricsInstance = nil

	metrics := NewOperatorMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.RecordConfigurationReload()
		metrics.RecordConfigurationReload()
	})
}

func TestUpdateResourceTrendPrediction(t *testing.T) {
	operatorMetricsOnce = sync.Once{}
	operatorMetricsInstance = nil

	metrics := NewOperatorMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.UpdateResourceTrendPrediction("default", "test-pod", "app", "cpu", "1h", 500.0)
		metrics.UpdateResourceTrendPrediction("default", "test-pod", "app", "memory", "24h", 2048.0)
	})
}

func TestUpdateHistoricalDataPoints(t *testing.T) {
	operatorMetricsOnce = sync.Once{}
	operatorMetricsInstance = nil

	metrics := NewOperatorMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		metrics.UpdateHistoricalDataPoints(1000)
		metrics.UpdateHistoricalDataPoints(2500)
	})
}

func TestObserveDuration(t *testing.T) {
	timer := NewTimer()
	require.NotNil(t, timer)

	time.Sleep(50 * time.Millisecond)

	operatorMetricsOnce = sync.Once{}
	operatorMetricsInstance = nil

	metrics := NewOperatorMetrics()
	require.NotNil(t, metrics)

	assert.NotPanics(t, func() {
		timer.ObserveDuration(metrics.ProcessingDuration.WithLabelValues("test_operation"))
	})
}

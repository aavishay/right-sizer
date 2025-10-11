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

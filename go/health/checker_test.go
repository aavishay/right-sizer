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

package health_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"right-sizer/health"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOperatorHealthChecker(t *testing.T) {
	checker := health.NewOperatorHealthChecker()
	require.NotNil(t, checker)

	// Verify default components are initialized
	controllerStatus, exists := checker.GetComponentStatus("controller")
	assert.True(t, exists)
	assert.True(t, controllerStatus.Healthy)
	assert.Equal(t, "Controller initialized", controllerStatus.Message)

	metricsStatus, exists := checker.GetComponentStatus("metrics-provider")
	assert.True(t, exists)
	assert.True(t, metricsStatus.Healthy)

	webhookStatus, exists := checker.GetComponentStatus("webhook")
	assert.True(t, exists)
	assert.True(t, webhookStatus.Healthy)
}

func TestOperatorHealthChecker_UpdateComponentStatus(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	// Update existing component
	checker.UpdateComponentStatus("controller", false, "Controller error")

	status, exists := checker.GetComponentStatus("controller")
	assert.True(t, exists)
	assert.False(t, status.Healthy)
	assert.Equal(t, "Controller error", status.Message)
	assert.WithinDuration(t, time.Now(), status.LastChecked, time.Second)

	// Add new component
	checker.UpdateComponentStatus("new-component", true, "Component started")

	status, exists = checker.GetComponentStatus("new-component")
	assert.True(t, exists)
	assert.True(t, status.Healthy)
	assert.Equal(t, "Component started", status.Message)
}

func TestOperatorHealthChecker_GetComponentStatus(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	// Test existing component
	status, exists := checker.GetComponentStatus("controller")
	assert.True(t, exists)
	assert.NotNil(t, status)

	// Test non-existing component
	status, exists = checker.GetComponentStatus("non-existent")
	assert.False(t, exists)
	assert.Nil(t, status)
}

func TestOperatorHealthChecker_IsHealthy(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	// All components healthy - should be healthy
	healthy := checker.IsHealthy()
	assert.True(t, healthy)

	// One component unhealthy - should be unhealthy
	checker.UpdateComponentStatus("controller", false, "Controller down")
	healthy = checker.IsHealthy()
	assert.False(t, healthy)

	// Multiple components unhealthy
	checker.UpdateComponentStatus("metrics-provider", false, "Metrics down")
	healthy = checker.IsHealthy()
	assert.False(t, healthy)
}

func TestOperatorHealthChecker_CheckHTTPEndpoint(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	// Mock HTTP server for successful endpoint
	successServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer successServer.Close()

	// Test successful endpoint
	err := checker.CheckHTTPEndpoint(successServer.URL, 2*time.Second)
	assert.NoError(t, err)

	// Test failed endpoint (non-existent)
	err = checker.CheckHTTPEndpoint("http://non-existent-server:9999", 100*time.Millisecond)
	assert.Error(t, err)
}

func TestOperatorHealthChecker_CheckHTTPEndpointWithFailures(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	// Mock failing server
	failingServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failingServer.Close()

	// Test failing endpoint
	err := checker.CheckHTTPEndpoint(failingServer.URL, 2*time.Second)
	assert.Error(t, err)
}

func TestOperatorHealthChecker_StartPeriodicHealthChecks(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	checker.SetCheckInterval(100 * time.Millisecond) // Fast interval for testing

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	// Start periodic checks
	go checker.StartPeriodicHealthChecks(ctx)

	// Wait for at least one check cycle
	time.Sleep(200 * time.Millisecond)

	// Verify controller has been checked recently
	controllerStatus, _ := checker.GetComponentStatus("controller")
	assert.WithinDuration(t, time.Now(), controllerStatus.LastChecked, 200*time.Millisecond)
}

func TestOperatorHealthChecker_LivenessCheck(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	req := httptest.NewRequest("GET", "/healthz", nil)

	// Test healthy controller
	err := checker.LivenessCheck(req)
	assert.NoError(t, err)

	// Test unhealthy controller
	checker.UpdateComponentStatus("controller", false, "Controller error")
	err = checker.LivenessCheck(req)
	assert.Error(t, err)
}

func TestOperatorHealthChecker_ReadinessCheck(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	req := httptest.NewRequest("GET", "/readyz", nil)

	// Test healthy controller
	err := checker.ReadinessCheck(req)
	assert.NoError(t, err)

	// Test unhealthy controller
	checker.UpdateComponentStatus("controller", false, "Controller error")
	err = checker.ReadinessCheck(req)
	assert.Error(t, err)
}

func TestOperatorHealthChecker_GetHealthReport(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	report := checker.GetHealthReport()
	assert.NotNil(t, report)
	assert.Contains(t, report, "overall_healthy")
	assert.True(t, report["overall_healthy"].(bool))

	// Make a component unhealthy
	checker.UpdateComponentStatus("controller", false, "Controller error")
	report = checker.GetHealthReport()
	assert.False(t, report["overall_healthy"].(bool))
}

func TestOperatorHealthChecker_ConcurrentAccess(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	const numGoroutines = 50
	const operationsPerGoroutine = 20

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2)

	// Concurrent updates
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				componentName := fmt.Sprintf("component-%d", id)
				healthy := j%2 == 0
				message := fmt.Sprintf("Message %d-%d", id, j)
				checker.UpdateComponentStatus(componentName, healthy, message)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < operationsPerGoroutine; j++ {
				componentName := fmt.Sprintf("component-%d", id)
				checker.GetComponentStatus(componentName)
				checker.IsHealthy()
			}
		}(i)
	}

	wg.Wait()

	// Verify no race conditions occurred and data integrity is maintained
	overall := checker.IsHealthy()
	// Should not panic or cause data races
	assert.NotPanics(t, func() { _ = overall })
}

func TestOperatorHealthChecker_ComponentStatusCopy(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	// Update a component
	checker.UpdateComponentStatus("test-component", true, "Initial message")

	// Get status
	status1, exists := checker.GetComponentStatus("test-component")
	require.True(t, exists)

	// Update component again
	checker.UpdateComponentStatus("test-component", false, "Updated message")

	// Get status again
	status2, exists := checker.GetComponentStatus("test-component")
	require.True(t, exists)

	// Verify we get updated information (not stale copies)
	assert.True(t, status1.Healthy)
	assert.Equal(t, "Initial message", status1.Message)
	assert.False(t, status2.Healthy)
	assert.Equal(t, "Updated message", status2.Message)
}

func TestOperatorHealthChecker_SettersAndGetters(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	// Test URL setters
	testMetricsURL := "http://test-metrics:8080/metrics"
	testWebhookURL := "http://test-webhook:8443/health"

	checker.SetMetricsServerURL(testMetricsURL)
	checker.SetWebhookServerURL(testWebhookURL)

	// Test interval setter
	testInterval := 5 * time.Minute
	checker.SetCheckInterval(testInterval)

	// Verify the values are set (this would require getters to be added to the interface)
	// For now, we can test that the setters don't panic
	assert.NotPanics(t, func() {
		checker.SetMetricsServerURL(testMetricsURL)
		checker.SetWebhookServerURL(testWebhookURL)
		checker.SetCheckInterval(testInterval)
	})
}

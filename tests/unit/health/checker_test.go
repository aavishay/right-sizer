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
	"net/http"
	"net/http/httptest"
	"right-sizer/health"
	"testing"
	"time"
)

func TestNewOperatorHealthChecker(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	if checker == nil {
		t.Fatal("Expected non-nil health checker")
	}

	// Check initial components
	if len(checker.components) != 3 {
		t.Errorf("Expected 3 initial components, got %d", len(checker.components))
	}

	// Check controller is initially healthy
	if status, exists := checker.GetComponentStatus("controller"); !exists || !status.Healthy {
		t.Error("Controller should be initially healthy")
	}

	// Check metrics provider is initially not healthy
	if status, exists := checker.GetComponentStatus("metrics-provider"); !exists || status.Healthy {
		t.Error("Metrics provider should be initially not healthy")
	}

	// Check webhook is initially not healthy
	if status, exists := checker.GetComponentStatus("webhook"); !exists || status.Healthy {
		t.Error("Webhook should be initially not healthy")
	}
}

func TestUpdateComponentStatus(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	// Update existing component
	checker.UpdateComponentStatus("metrics-provider", true, "Provider initialized")

	status, exists := checker.GetComponentStatus("metrics-provider")
	if !exists {
		t.Fatal("Component should exist")
	}

	if !status.Healthy {
		t.Error("Component should be healthy")
	}

	if status.Message != "Provider initialized" {
		t.Errorf("Expected message 'Provider initialized', got '%s'", status.Message)
	}

	// Add new component
	checker.UpdateComponentStatus("custom-component", true, "Custom component healthy")

	status, exists = checker.GetComponentStatus("custom-component")
	if !exists {
		t.Fatal("New component should exist")
	}

	if !status.Healthy {
		t.Error("New component should be healthy")
	}
}

func TestGetComponentStatus(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	// Get existing component
	status, exists := checker.GetComponentStatus("controller")
	if !exists {
		t.Error("Controller should exist")
	}

	if status == nil {
		t.Fatal("Status should not be nil")
	}

	// Get non-existing component
	status, exists = checker.GetComponentStatus("non-existent")
	if exists {
		t.Error("Non-existent component should not exist")
	}

	if status != nil {
		t.Error("Status should be nil for non-existent component")
	}
}

func TestIsHealthy(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	// Initially not healthy (metrics-provider and webhook are not healthy)
	// But they should be skipped if not initialized
	checker.UpdateComponentStatus("metrics-provider", false, "Not initialized")
	checker.UpdateComponentStatus("webhook", false, "Not enabled")

	if !checker.IsHealthy() {
		t.Error("Should be healthy when optional components are not initialized")
	}

	// Make metrics-provider initialized but unhealthy
	checker.UpdateComponentStatus("metrics-provider", false, "Connection failed")

	if checker.IsHealthy() {
		t.Error("Should not be healthy when initialized component is unhealthy")
	}

	// Make all components healthy
	checker.UpdateComponentStatus("controller", true, "Running")
	checker.UpdateComponentStatus("metrics-provider", true, "Connected")
	checker.UpdateComponentStatus("webhook", true, "Serving")

	if !checker.IsHealthy() {
		t.Error("Should be healthy when all components are healthy")
	}

	// Test stale health check detection
	oldTime := time.Now().Add(-6 * time.Minute)
	checker.components["controller"].LastChecked = oldTime

	if checker.IsHealthy() {
		t.Error("Should not be healthy when component health check is stale")
	}
}

func TestGetHealthReport(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	// Update components
	checker.UpdateComponentStatus("controller", true, "Running")
	checker.UpdateComponentStatus("metrics-provider", false, "Not connected")

	report := checker.GetHealthReport()

	// Check overall health
	if overall, ok := report["overall_healthy"].(bool); !ok || overall {
		t.Error("Overall health should be false")
	}

	// Check components exist
	components, ok := report["components"].(map[string]interface{})
	if !ok {
		t.Fatal("Components should exist in report")
	}

	// Check controller status
	if controller, ok := components["controller"].(map[string]interface{}); ok {
		if healthy, ok := controller["healthy"].(bool); !ok || !healthy {
			t.Error("Controller should be healthy in report")
		}
		if message, ok := controller["message"].(string); !ok || message != "Running" {
			t.Error("Controller message should be 'Running'")
		}
	} else {
		t.Error("Controller should exist in components")
	}

	// Check metrics-provider status
	if metricsProvider, ok := components["metrics-provider"].(map[string]interface{}); ok {
		if healthy, ok := metricsProvider["healthy"].(bool); !ok || healthy {
			t.Error("Metrics provider should not be healthy in report")
		}
	} else {
		t.Error("Metrics provider should exist in components")
	}
}

func TestLivenessCheck(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	// Controller healthy
	checker.UpdateComponentStatus("controller", true, "Running")

	req := httptest.NewRequest("GET", "/healthz", nil)
	err := checker.LivenessCheck(req)

	if err != nil {
		t.Errorf("Liveness check should pass when controller is healthy: %v", err)
	}

	// Controller unhealthy
	checker.UpdateComponentStatus("controller", false, "Failed")

	err = checker.LivenessCheck(req)

	if err == nil {
		t.Error("Liveness check should fail when controller is unhealthy")
	}
}

func TestReadinessCheck(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	// All components healthy
	checker.UpdateComponentStatus("controller", true, "Running")
	checker.UpdateComponentStatus("metrics-provider", true, "Connected")
	checker.UpdateComponentStatus("webhook", true, "Serving")

	req := httptest.NewRequest("GET", "/readyz", nil)
	err := checker.ReadinessCheck(req)

	if err != nil {
		t.Errorf("Readiness check should pass when all components are healthy: %v", err)
	}

	// Metrics provider unhealthy but not initialized
	checker.UpdateComponentStatus("metrics-provider", false, "Not initialized")
	err = checker.ReadinessCheck(req)

	if err != nil {
		t.Error("Readiness check should pass when optional component is not initialized")
	}

	// Metrics provider unhealthy and initialized
	checker.UpdateComponentStatus("metrics-provider", false, "Connection failed")
	err = checker.ReadinessCheck(req)

	if err == nil {
		t.Error("Readiness check should fail when initialized component is unhealthy")
	}
}

func TestCheckHTTPEndpoint(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("healthy"))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Test healthy endpoint
	err := checker.CheckHTTPEndpoint(server.URL+"/health", 2*time.Second)
	if err != nil {
		t.Errorf("Should not error for healthy endpoint: %v", err)
	}

	// Test unhealthy endpoint
	err = checker.CheckHTTPEndpoint(server.URL+"/notfound", 2*time.Second)
	if err == nil {
		t.Error("Should error for unhealthy endpoint")
	}

	// Test unreachable endpoint
	err = checker.CheckHTTPEndpoint("http://localhost:99999", 100*time.Millisecond)
	if err == nil {
		t.Error("Should error for unreachable endpoint")
	}
}

func TestPeriodicHealthChecks(t *testing.T) {
	checker := health.NewOperatorHealthChecker()
	checker.SetCheckInterval(100 * time.Millisecond) // Short interval for testing

	// Create test server for metrics
	metricsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer metricsServer.Close()

	checker.SetMetricsServerURL(metricsServer.URL)

	// Start periodic checks
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	checker.StartPeriodicHealthChecks(ctx)

	// Wait for at least one check cycle
	time.Sleep(200 * time.Millisecond)

	// Controller should be healthy
	status, _ := checker.GetComponentStatus("controller")
	if !status.Healthy {
		t.Error("Controller should be healthy after periodic check")
	}

	// Metrics provider should be healthy (test server is responding)
	status, _ = checker.GetComponentStatus("metrics-provider")
	if !status.Healthy {
		t.Error("Metrics provider should be healthy when server responds")
	}
}

func TestDetailedHealthCheck(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	// Set up components
	checker.UpdateComponentStatus("controller", true, "Running")
	checker.UpdateComponentStatus("metrics-provider", true, "Connected")

	// Create test server for metrics
	metricsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer metricsServer.Close()

	checker.SetMetricsServerURL(metricsServer.URL)

	// Get detailed health check function
	checkFunc := checker.DetailedHealthCheck()

	req := httptest.NewRequest("GET", "/readyz/detailed", nil)
	err := checkFunc(req)

	if err != nil {
		t.Errorf("Detailed health check should pass: %v", err)
	}

	// Make a component unhealthy
	checker.UpdateComponentStatus("metrics-provider", false, "Failed")

	err = checkFunc(req)

	if err == nil {
		t.Error("Detailed health check should fail when component is unhealthy")
	}
}

func TestConcurrentAccess(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	// Run concurrent updates and reads
	done := make(chan bool)

	// Writer goroutines
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				checker.UpdateComponentStatus("controller", j%2 == 0, "Update from goroutine")
				time.Sleep(time.Microsecond)
			}
			done <- true
		}(i)
	}

	// Reader goroutines
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				checker.GetComponentStatus("controller")
				checker.IsHealthy()
				checker.GetHealthReport()
				time.Sleep(time.Microsecond)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// If we get here without panic, concurrent access is safe
	t.Log("Concurrent access test passed")
}

func TestComponentStatusCopy(t *testing.T) {
	checker := health.NewOperatorHealthChecker()

	// Update component
	checker.UpdateComponentStatus("controller", true, "Original message")

	// Get status
	status1, _ := checker.GetComponentStatus("controller")

	// Modify the returned status
	status1.Healthy = false
	status1.Message = "Modified message"

	// Get status again
	status2, _ := checker.GetComponentStatus("controller")

	// Original should not be modified
	if !status2.Healthy {
		t.Error("Original status should not be modified")
	}

	if status2.Message != "Original message" {
		t.Error("Original message should not be modified")
	}
}

func TestHealthCheckWithNilComponents(t *testing.T) {
	checker := health.NewOperatorHealthChecker()
	// Clear all components to test nil handling
	checker.UpdateComponentStatus("controller", false, "")
	checker.UpdateComponentStatus("metrics-provider", false, "")
	checker.UpdateComponentStatus("webhook", false, "")

	// Should handle nil components gracefully
	if checker.IsHealthy() {
		t.Error("Should not be healthy with no components")
	}

	report := checker.GetHealthReport()
	if report == nil {
		t.Error("Report should not be nil even with no components")
	}

	req := httptest.NewRequest("GET", "/healthz", nil)
	err := checker.LivenessCheck(req)
	if err == nil {
		t.Error("Liveness check should fail with no controller component")
	}
}

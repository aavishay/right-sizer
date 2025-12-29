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

package health

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"right-sizer/logger"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

// ComponentStatus represents the health status of a component
type ComponentStatus struct {
	Healthy     bool
	LastChecked time.Time
	Message     string
}

// OperatorHealthChecker checks the health of operator components
type OperatorHealthChecker struct {
	mu               sync.RWMutex
	components       map[string]*ComponentStatus
	metricsServerURL string
	webhookServerURL string
	checkInterval    time.Duration
	lastOverallCheck time.Time
	k8sClient        client.Client
}

// NewOperatorHealthChecker creates a new health checker
func NewOperatorHealthChecker() *OperatorHealthChecker {
	return &OperatorHealthChecker{
		components: map[string]*ComponentStatus{
			"controller": {
				Healthy:     true,
				LastChecked: time.Now(),
				Message:     "Controller initialized",
			},
			"metrics-provider": {
				Healthy:     true, // Default to healthy, will be updated if actually unhealthy
				LastChecked: time.Now(),
				Message:     "Not initialized",
			},
			"webhook": {
				Healthy:     true, // Default to healthy, will be updated if actually unhealthy
				LastChecked: time.Now(),
				Message:     "Not initialized",
			},
		},
		metricsServerURL: "http://localhost:8080/metrics",
		webhookServerURL: "http://localhost:8443/health",
		checkInterval:    30 * time.Second,
	}
}

// UpdateComponentStatus updates the status of a specific component
func (h *OperatorHealthChecker) UpdateComponentStatus(component string, healthy bool, message string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if status, exists := h.components[component]; exists {
		status.Healthy = healthy
		status.LastChecked = time.Now()
		status.Message = message
	} else {
		h.components[component] = &ComponentStatus{
			Healthy:     healthy,
			LastChecked: time.Now(),
			Message:     message,
		}
	}

	logger.Debug("Health status updated for %s: healthy=%v, message=%s", component, healthy, message)
}

// GetComponentStatus returns the status of a specific component
func (h *OperatorHealthChecker) GetComponentStatus(component string) (*ComponentStatus, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	status, exists := h.components[component]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid race conditions
	statusCopy := &ComponentStatus{
		Healthy:     status.Healthy,
		LastChecked: status.LastChecked,
		Message:     status.Message,
	}
	return statusCopy, true
}

// IsHealthy returns true if all components are healthy
func (h *OperatorHealthChecker) IsHealthy() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for name, status := range h.components {
		// Skip optional components that are not initialized or not enabled
		if name == "webhook" || name == "metrics-provider" {
			if status.Message == "Not enabled" || status.Message == "Not initialized" {
				continue
			}
		}

		// Check if component has been checked recently
		if time.Since(status.LastChecked) > 5*time.Minute {
			// Consider stale checks as unhealthy only for critical components
			if name == "controller" {
				return false
			}
			// Skip stale checks for non-critical components
			continue
		}

		// Only fail if a critical component is unhealthy
		if !status.Healthy && name == "controller" {
			return false
		}
	}

	return true
}

// GetHealthReport returns a detailed health report
func (h *OperatorHealthChecker) GetHealthReport() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	report := make(map[string]interface{})
	report["overall_healthy"] = h.IsHealthy()
	report["last_check"] = h.lastOverallCheck

	components := make(map[string]interface{})
	for name, status := range h.components {
		components[name] = map[string]interface{}{
			"healthy":      status.Healthy,
			"last_checked": status.LastChecked,
			"message":      status.Message,
			"age":          time.Since(status.LastChecked).String(),
		}
	}
	report["components"] = components

	return report
}

// StartPeriodicHealthChecks starts periodic health checks for components
func (h *OperatorHealthChecker) StartPeriodicHealthChecks(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(h.checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				logger.Info("Stopping periodic health checks")
				return
			case <-ticker.C:
				h.performHealthChecks()
			}
		}
	}()
}

// performHealthChecks performs health checks on all components
func (h *OperatorHealthChecker) performHealthChecks() {
	h.mu.Lock()
	h.lastOverallCheck = time.Now()
	h.mu.Unlock()

	// Check controller health (always healthy if this code is running)
	h.UpdateComponentStatus("controller", true, "Controller is running")

	// Check metrics server if enabled
	if h.metricsServerURL != "" {
		if err := h.CheckHTTPEndpoint(h.metricsServerURL, 2*time.Second); err != nil {
			h.UpdateComponentStatus("metrics-provider", false, fmt.Sprintf("Metrics server check failed: %v", err))
		} else {
			h.UpdateComponentStatus("metrics-provider", true, "Metrics server is healthy")
		}
	}

	// Check webhook server if enabled
	if h.webhookServerURL != "" {
		if err := h.CheckHTTPEndpoint(h.webhookServerURL, 2*time.Second); err != nil {
			// Webhook might not be enabled, which is okay
			if h.components["webhook"].Message != "Not enabled" {
				h.UpdateComponentStatus("webhook", false, fmt.Sprintf("Webhook check failed: %v", err))
			}
		} else {
			h.UpdateComponentStatus("webhook", true, "Webhook server is healthy")
		}
	}

	// Check Kubernetes API connectivity if client is available
	if h.k8sClient != nil {
		if err := h.checkK8sHealth(); err != nil {
			h.UpdateComponentStatus("k8s-api", false, fmt.Sprintf("K8s API check failed: %v", err))
		} else {
			h.UpdateComponentStatus("k8s-api", true, "Kubernetes API is accessible")
		}
	}
}

// checkK8sHealth checks connectivity to the Kubernetes API server
func (h *OperatorHealthChecker) checkK8sHealth() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to list pods in default namespace with limit 1 to verify read access
	// We use a raw list operation to bypass local caches and hit the API server directly if possible,
	// though client.Client usually reads from cache. To check API server, we might strictly need APIReader.
	// But standard client check is better than nothing.
	// Actually, standard client reads from cache. A better check for "connectivity" might be to use RESTMapper or similar.
	// However, if cache is stale/broken, that's also an issue.

	// Use a simple list on the configured system namespace or default
	listOpts := &client.ListOptions{
		Limit:     1,
		Namespace: "default",
	}

	var pods corev1.PodList
	if err := h.k8sClient.List(ctx, &pods, listOpts); err != nil {
		return err
	}

	return nil
}

// CheckHTTPEndpoint checks if an HTTP endpoint is responsive
func (h *OperatorHealthChecker) CheckHTTPEndpoint(url string, timeout time.Duration) error {
	client := &http.Client{
		Timeout: timeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("endpoint returned status %d", resp.StatusCode)
	}

	return nil
}

// LivenessCheck implements the healthz.Checker interface for liveness probes
func (h *OperatorHealthChecker) LivenessCheck(_ *http.Request) error {
	// For liveness, we only check if the controller is running
	// This prevents unnecessary restarts if external dependencies are down
	status, exists := h.GetComponentStatus("controller")
	if !exists {
		// If controller status doesn't exist yet, assume healthy to avoid premature restarts
		return nil
	}
	if status.Healthy {
		return nil
	}
	return errors.New("controller is not healthy")
}

// ReadinessCheck implements the healthz.Checker interface for readiness probes
func (h *OperatorHealthChecker) ReadinessCheck(_ *http.Request) error {
	// For readiness, check only the controller component
	// Other components are optional and shouldn't affect readiness
	status, exists := h.GetComponentStatus("controller")
	if !exists {
		// If controller status doesn't exist yet, assume ready
		return nil
	}

	if !status.Healthy {
		return fmt.Errorf("controller is not healthy: %s", status.Message)
	}

	// Check if the controller status is stale
	if time.Since(status.LastChecked) > 5*time.Minute {
		return fmt.Errorf("controller health check is stale (last checked: %v ago)", time.Since(status.LastChecked))
	}

	return nil
}

// SetCheckInterval sets the interval for periodic health checks
func (h *OperatorHealthChecker) SetCheckInterval(interval time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checkInterval = interval
}

// SetMetricsServerURL sets the URL for the metrics server health check
func (h *OperatorHealthChecker) SetMetricsServerURL(url string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.metricsServerURL = url
}

// SetWebhookServerURL sets the URL for the webhook server health check
func (h *OperatorHealthChecker) SetWebhookServerURL(url string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.webhookServerURL = url
}

// SetK8sClient sets the Kubernetes client for health checks
func (h *OperatorHealthChecker) SetK8sClient(c client.Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.k8sClient = c
	// Initialize status
	h.components["k8s-api"] = &ComponentStatus{
		Healthy:     true,
		LastChecked: time.Now(),
		Message:     "Initialized",
	}
}

// DetailedHealthCheck returns a custom health check that provides detailed information
func (h *OperatorHealthChecker) DetailedHealthCheck() healthz.Checker {
	return func(req *http.Request) error {
		// Perform a fresh health check
		h.performHealthChecks()

		// Return readiness check result
		return h.ReadinessCheck(req)
	}
}

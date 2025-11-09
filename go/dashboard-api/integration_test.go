package dashboardapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDashboardIntegrationEndToEnd(t *testing.T) {
	// Create a mock dashboard server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify authentication header
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token-123" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Verify User-Agent
		ua := r.Header.Get("User-Agent")
		assert.Contains(t, ua, "right-sizer-operator")

		// Verify Content-Type for POST requests
		if r.Method == "POST" {
			ct := r.Header.Get("Content-Type")
			assert.Equal(t, "application/json", ct)
		}

		switch r.URL.Path {
		case "/api/operator/health":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
		case "/api/operator/events":
			var payload interface{}
			json.NewDecoder(r.Body).Decode(&payload)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
		case "/api/operator/events/batch":
			var payload interface{}
			json.NewDecoder(r.Body).Decode(&payload)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
		case "/api/operator/status":
			var payload Status
			json.NewDecoder(r.Body).Decode(&payload)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
		case "/api/operator/metrics":
			var payload Metrics
			json.NewDecoder(r.Body).Decode(&payload)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create client with mock server
	config := ClientConfig{
		BaseURL:            server.URL,
		APIToken:           "test-token-123",
		ClusterID:          "test-cluster",
		ClusterName:        "test-cluster",
		OperatorVersion:    "test-v1.0.0",
		EnableBatching:     true,
		BatchSize:          2,
		BatchFlushInterval: 100 * time.Millisecond,
		EnableHeartbeat:    true,
		HeartbeatInterval:  200 * time.Millisecond,
	}

	client := NewClient(config)

	// Test health check
	t.Run("HealthCheck", func(t *testing.T) {
		err := client.HealthCheck()
		require.NoError(t, err)
	})

	// Test sending events
	t.Run("SendEvents", func(t *testing.T) {
		event := NewResizeEvent(EventResizeCompleted, "test-ns", "test-pod", "test-container", map[string]interface{}{
			"oldResources": map[string]interface{}{"cpu": "100m", "memory": "128Mi"},
			"newResources": map[string]interface{}{"cpu": "200m", "memory": "256Mi"},
		})

		err := client.SendEvent(event)
		require.NoError(t, err)
	})

	// Test batching
	t.Run("EventBatching", func(t *testing.T) {
		event1 := NewResizeEvent(EventResizeStarted, "test-ns", "test-pod-1", "container-1", nil)
		event2 := NewResizeEvent(EventResizeCompleted, "test-ns", "test-pod-1", "container-1", nil)

		// Send events (should be batched)
		err := client.SendEvent(event1)
		require.NoError(t, err)
		err = client.SendEvent(event2)
		require.NoError(t, err)

		// Wait for batch flush
		time.Sleep(150 * time.Millisecond)
	})

	// Test sending metrics
	t.Run("SendMetrics", func(t *testing.T) {
		metrics := Metrics{
			Namespace:     "test-ns",
			PodName:       "test-pod",
			ContainerName: "test-container",
			Metrics: map[string]interface{}{
				"cpu_milli":      150.0,
				"memory_mb":      200.0,
				"cpu_percent":    75.0,
				"memory_percent": 80.0,
			},
		}

		err := client.SendMetrics(metrics)
		require.NoError(t, err)
	})

	// Test heartbeat with metrics provider
	t.Run("HeartbeatWithMetrics", func(t *testing.T) {
		// Mock metrics provider
		client.SetMetricsProvider(&mockMetricsProvider{})

		// Start client
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		err := client.Start(ctx)
		require.NoError(t, err)

		// Wait for heartbeat
		time.Sleep(300 * time.Millisecond)

		// Stop client
		client.Stop()
	})

	// Test error handling
	t.Run("AuthenticationFailure", func(t *testing.T) {
		badClient := NewClient(ClientConfig{
			BaseURL:     server.URL,
			APIToken:    "wrong-token",
			ClusterID:   "test-cluster",
			ClusterName: "test-cluster",
		})

		err := badClient.HealthCheck()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "401") // Unauthorized
	})
}

// mockMetricsProvider implements MetricsProvider for testing
type mockMetricsProvider struct{}

func (m *mockMetricsProvider) GetStatusMetrics() *StatusMetrics {
	return &StatusMetrics{
		TotalPods:            10,
		ManagedPods:          8,
		OptimizationsApplied: 5,
		AvgCPUUsage:          200.0,
		AvgMemoryUsage:       512.0,
	}
}

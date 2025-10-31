package dashboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"right-sizer/logger"
)

// Client handles communication with the Right-Sizer dashboard
type Client struct {
	baseURL     string
	apiToken    string
	clusterID   string
	clusterName string
	httpClient  *http.Client
	enabled     bool
}

// ClusterStatusUpdate represents a cluster status update request
type ClusterStatusUpdate struct {
	ClusterID   string                 `json:"clusterId"`
	ClusterName string                 `json:"clusterName"`
	Status      string                 `json:"status"`
	LastSeen    time.Time              `json:"lastSeen"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ResourceUpdate represents resource usage data to send to dashboard
type ResourceUpdate struct {
	ClusterID string                 `json:"clusterId"`
	Timestamp time.Time              `json:"timestamp"`
	Namespace string                 `json:"namespace"`
	PodName   string                 `json:"podName"`
	Container string                 `json:"container"`
	Resources map[string]interface{} `json:"resources"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NewClient creates a new dashboard client from environment variables
func NewClient() *Client {
	baseURL := os.Getenv("DASHBOARD_URL")
	apiToken := os.Getenv("DASHBOARD_API_TOKEN")
	clusterID := os.Getenv("CLUSTER_ID")
	clusterName := os.Getenv("CLUSTER_NAME")

	if baseURL == "" || apiToken == "" {
		logger.Info("📊 Dashboard reporting disabled - missing required environment variables")
		logger.Info("   Required: DASHBOARD_URL, DASHBOARD_API_TOKEN")
		logger.Info("   DASHBOARD_URL: %s", baseURL)
		logger.Info("   DASHBOARD_API_TOKEN: %s", maskToken(apiToken))
		return &Client{enabled: false}
	}

	// Set defaults if not provided
	if clusterID == "" {
		clusterID = "default-cluster"
	}
	if clusterName == "" {
		clusterName = "Right-Sizer Cluster"
	}

	logger.Info("📊 Dashboard reporting enabled")
	logger.Info("   Dashboard URL: %s", baseURL)
	logger.Info("   API Token: %s", maskToken(apiToken))
	logger.Info("   Cluster ID: %s", clusterID)
	logger.Info("   Cluster Name: %s", clusterName)

	return &Client{
		baseURL:     baseURL,
		apiToken:    apiToken,
		clusterID:   clusterID,
		clusterName: clusterName,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		enabled: true,
	}
}

// IsEnabled returns true if the dashboard client is enabled
func (c *Client) IsEnabled() bool {
	return c.enabled
}

// UpdateClusterStatus updates the cluster status in the dashboard
func (c *Client) UpdateClusterStatus(status string) error {
	if !c.enabled {
		return nil
	}

	update := ClusterStatusUpdate{
		ClusterID:   c.clusterID,
		ClusterName: c.clusterName,
		Status:      status,
		LastSeen:    time.Now(),
		Metadata: map[string]interface{}{
			"operator_version": os.Getenv("VERSION"),
			"updated_by":       "right-sizer-operator",
		},
	}

	return c.sendRequest("/api/clusters/status", update)
}

// ReportResourceUpdate sends resource usage data to the dashboard
func (c *Client) ReportResourceUpdate(namespace, podName, container string, resources map[string]interface{}) error {
	if !c.enabled {
		return nil
	}

	update := ResourceUpdate{
		ClusterID: c.clusterID,
		Timestamp: time.Now(),
		Namespace: namespace,
		PodName:   podName,
		Container: container,
		Resources: resources,
		Metadata: map[string]interface{}{
			"reported_by": "right-sizer-operator",
		},
	}

	return c.sendRequest("/api/metrics/resources", update)
}

// sendRequest sends a POST request to the dashboard API
func (c *Client) sendRequest(endpoint string, data interface{}) error {
	if !c.enabled {
		return nil
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal request data: %w", err)
	}

	url := c.baseURL + endpoint
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("User-Agent", "right-sizer-operator")

	logger.Debug("📊 Sending dashboard request to %s", url)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logger.Warn("📊 Failed to send dashboard request: %v", err)
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Warn("📊 Dashboard API returned status %d for %s", resp.StatusCode, endpoint)
		return fmt.Errorf("dashboard API returned status %d", resp.StatusCode)
	}

	logger.Debug("📊 Dashboard request successful: %s", endpoint)
	return nil
}

// StartHeartbeat starts sending periodic heartbeat updates to the dashboard
func (c *Client) StartHeartbeat(interval time.Duration) {
	if !c.enabled {
		return
	}

	logger.Info("📊 Starting dashboard heartbeat every %v", interval)

	go func() {
		// Send initial "connected" status
		if err := c.UpdateClusterStatus("connected"); err != nil {
			logger.Warn("📊 Failed to send initial cluster status: %v", err)
		} else {
			logger.Info("📊 Cluster status updated to 'connected'")
		}

		// Send periodic heartbeats
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			if err := c.UpdateClusterStatus("connected"); err != nil {
				logger.Warn("📊 Failed to send heartbeat: %v", err)
			} else {
				logger.Debug("📊 Heartbeat sent successfully")
			}
		}
	}()
}

// maskToken masks the API token for logging purposes
func maskToken(token string) string {
	if token == "" {
		return "<not set>"
	}
	if len(token) <= 8 {
		return "<masked>"
	}
	return token[:8] + "..."
}

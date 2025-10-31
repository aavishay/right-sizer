// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package dashboardapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"right-sizer/logger"
)

// EventType represents the type of operator event
type EventType string

const (
	EventResizeStarted         EventType = "resize_started"
	EventResizeCompleted       EventType = "resize_completed"
	EventResizeFailed          EventType = "resize_failed"
	EventPredictionUpdated     EventType = "prediction_updated"
	EventRecommendationCreated EventType = "recommendation_created"
	EventPolicyViolation       EventType = "policy_violation"
	EventMetricsCollected      EventType = "metrics_collected"
	EventHeartbeat             EventType = "heartbeat"
	EventError                 EventType = "error"
)

// EventSeverity represents the severity level of an event
type EventSeverity string

const (
	SeverityInfo     EventSeverity = "info"
	SeverityWarning  EventSeverity = "warning"
	SeverityError    EventSeverity = "error"
	SeverityCritical EventSeverity = "critical"
)

// Event represents an operator event to be sent to the dashboard
type Event struct {
	Type          EventType              `json:"type"`
	Severity      EventSeverity          `json:"severity"`
	Timestamp     string                 `json:"timestamp"`
	Namespace     string                 `json:"namespace,omitempty"`
	PodName       string                 `json:"podName,omitempty"`
	ContainerName string                 `json:"containerName,omitempty"`
	WorkloadType  string                 `json:"workloadType,omitempty"`
	WorkloadName  string                 `json:"workloadName,omitempty"`
	Message       string                 `json:"message"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	ClusterID     string                 `json:"clusterId,omitempty"`
	ClusterName   string                 `json:"clusterName,omitempty"`
}

// Status represents operator health status
type Status struct {
	ClusterID       string            `json:"clusterId"`
	ClusterName     string            `json:"clusterName,omitempty"`
	OperatorVersion string            `json:"operatorVersion"`
	Status          string            `json:"status"` // healthy, degraded, unhealthy
	Timestamp       string            `json:"timestamp"`
	Metrics         *StatusMetrics    `json:"metrics,omitempty"`
	Conditions      []StatusCondition `json:"conditions,omitempty"`
}

// StatusMetrics contains aggregated metrics for status updates
type StatusMetrics struct {
	TotalPods            int     `json:"totalPods,omitempty"`
	ManagedPods          int     `json:"managedPods,omitempty"`
	OptimizationsApplied int     `json:"optimizationsApplied,omitempty"`
	AvgCPUUsage          float64 `json:"avgCpuUsage,omitempty"`
	AvgMemoryUsage       float64 `json:"avgMemoryUsage,omitempty"`
}

// StatusCondition represents a health condition
type StatusCondition struct {
	Type               string `json:"type"`
	Status             bool   `json:"status"`
	Message            string `json:"message"`
	LastTransitionTime string `json:"lastTransitionTime"`
}

// Metrics represents time-series metrics
type Metrics struct {
	ClusterID     string                 `json:"clusterId"`
	Timestamp     string                 `json:"timestamp"`
	Namespace     string                 `json:"namespace,omitempty"`
	PodName       string                 `json:"podName,omitempty"`
	ContainerName string                 `json:"containerName,omitempty"`
	Metrics       map[string]interface{} `json:"metrics"`
}

// ClientConfig configures the dashboard API client
type ClientConfig struct {
	BaseURL            string        `json:"baseUrl"`
	APIToken           string        `json:"apiToken"`
	ClusterID          string        `json:"clusterId"`
	ClusterName        string        `json:"clusterName"`
	OperatorVersion    string        `json:"operatorVersion"`
	Timeout            time.Duration `json:"timeout"`
	RetryAttempts      int           `json:"retryAttempts"`
	RetryDelay         time.Duration `json:"retryDelay"`
	BatchSize          int           `json:"batchSize"`
	BatchFlushInterval time.Duration `json:"batchFlushInterval"`
	EnableBatching     bool          `json:"enableBatching"`
	EnableHeartbeat    bool          `json:"enableHeartbeat"`
	HeartbeatInterval  time.Duration `json:"heartbeatInterval"`
}

// Client is the dashboard API client
type Client struct {
	config     ClientConfig
	httpClient *http.Client
	mu         sync.RWMutex
	eventQueue []Event
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

// NewClient creates a new dashboard API client
func NewClient(config ClientConfig) *Client {
	// Set defaults
	if config.Timeout == 0 {
		config.Timeout = 10 * time.Second
	}
	if config.RetryAttempts == 0 {
		config.RetryAttempts = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 1 * time.Second
	}
	if config.BatchSize == 0 {
		config.BatchSize = 50
	}
	if config.BatchFlushInterval == 0 {
		config.BatchFlushInterval = 30 * time.Second
	}
	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = 30 * time.Second
	}

	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		eventQueue: make([]Event, 0),
		stopCh:     make(chan struct{}),
	}
}

// Start starts background tasks (batching, heartbeat)
func (c *Client) Start(ctx context.Context) error {
	if c.config.BaseURL == "" {
		logger.Warn("Dashboard URL not configured, integration disabled")
		return nil
	}

	if c.config.APIToken == "" {
		logger.Warn("Dashboard API token not configured, integration disabled")
		return nil
	}

	logger.Info("🔗 Starting dashboard integration url=%s cluster=%s", c.config.BaseURL, c.config.ClusterName)

	// Test connectivity
	if err := c.HealthCheck(); err != nil {
		logger.Error("Dashboard connectivity check failed: %v", err)
		// Don't fail startup, just log the error
	} else {
		logger.Info("✅ Dashboard connectivity verified")
	}

	// Start batch flush goroutine if batching is enabled
	if c.config.EnableBatching {
		c.wg.Add(1)
		go c.batchFlushLoop(ctx)
	}

	// Start heartbeat goroutine if enabled
	if c.config.EnableHeartbeat {
		c.wg.Add(1)
		go c.heartbeatLoop(ctx)
	}

	return nil
}

// Stop stops the client and flushes pending events
func (c *Client) Stop() error {
	close(c.stopCh)
	c.wg.Wait()

	// Flush remaining events
	if err := c.FlushEvents(); err != nil {
		logger.Error("Failed to flush events on shutdown: %v", err)
		return err
	}

	logger.Info("Dashboard integration stopped")
	return nil
}

// SendEvent sends a single event to the dashboard
func (c *Client) SendEvent(event Event) error {
	if c.config.BaseURL == "" {
		return nil // Integration disabled
	}

	// Set cluster info if not provided
	if event.ClusterID == "" {
		event.ClusterID = c.config.ClusterID
	}
	if event.ClusterName == "" {
		event.ClusterName = c.config.ClusterName
	}
	if event.Timestamp == "" {
		event.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	// Add to batch queue if batching enabled
	if c.config.EnableBatching {
		c.mu.Lock()
		c.eventQueue = append(c.eventQueue, event)
		shouldFlush := len(c.eventQueue) >= c.config.BatchSize
		c.mu.Unlock()

		if shouldFlush {
			return c.FlushEvents()
		}
		return nil
	}

	// Send immediately if batching disabled
	return c.sendEventNow(event)
}

// sendEventNow sends a single event immediately
func (c *Client) sendEventNow(event Event) error {
	url := fmt.Sprintf("%s/api/operator/events", c.config.BaseURL)
	return c.doRequest("POST", url, event, nil)
}

// FlushEvents sends all queued events as a batch
func (c *Client) FlushEvents() error {
	c.mu.Lock()
	if len(c.eventQueue) == 0 {
		c.mu.Unlock()
		return nil
	}

	events := make([]Event, len(c.eventQueue))
	copy(events, c.eventQueue)
	c.eventQueue = c.eventQueue[:0] // Clear the queue
	c.mu.Unlock()

	if len(events) == 1 {
		// Send single event directly
		return c.sendEventNow(events[0])
	}

	// Send as batch
	url := fmt.Sprintf("%s/api/operator/events/batch", c.config.BaseURL)
	payload := map[string]interface{}{
		"events": events,
	}
	return c.doRequest("POST", url, payload, nil)
}

// SendStatus sends operator status update (heartbeat)
func (c *Client) SendStatus(status Status) error {
	if c.config.BaseURL == "" {
		return nil // Integration disabled
	}

	// Set cluster info if not provided
	if status.ClusterID == "" {
		status.ClusterID = c.config.ClusterID
	}
	if status.ClusterName == "" {
		status.ClusterName = c.config.ClusterName
	}
	if status.OperatorVersion == "" {
		status.OperatorVersion = c.config.OperatorVersion
	}
	if status.Timestamp == "" {
		status.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	url := fmt.Sprintf("%s/api/operator/status", c.config.BaseURL)
	return c.doRequest("POST", url, status, nil)
}

// SendMetrics sends time-series metrics
func (c *Client) SendMetrics(metrics Metrics) error {
	if c.config.BaseURL == "" {
		return nil // Integration disabled
	}

	if metrics.ClusterID == "" {
		metrics.ClusterID = c.config.ClusterID
	}
	if metrics.Timestamp == "" {
		metrics.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	url := fmt.Sprintf("%s/api/operator/metrics", c.config.BaseURL)
	return c.doRequest("POST", url, metrics, nil)
}

// HealthCheck verifies connectivity to the dashboard
func (c *Client) HealthCheck() error {
	if c.config.BaseURL == "" {
		return fmt.Errorf("dashboard URL not configured")
	}

	url := fmt.Sprintf("%s/api/operator/health", c.config.BaseURL)
	var response map[string]interface{}
	return c.doRequest("GET", url, nil, &response)
}

// doRequest performs an HTTP request with retries
func (c *Client) doRequest(method, url string, payload interface{}, response interface{}) error {
	var lastErr error

	for attempt := 0; attempt <= c.config.RetryAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(c.config.RetryDelay * time.Duration(attempt))
			logger.Debug("Retrying dashboard request attempt=%d url=%s", attempt, url)
		}

		err := c.doRequestOnce(method, url, payload, response)
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry on 4xx errors (client errors)
		if httpErr, ok := err.(*httpError); ok {
			if httpErr.StatusCode >= 400 && httpErr.StatusCode < 500 {
				return err
			}
		}
	}

	return lastErr
}

// doRequestOnce performs a single HTTP request
func (c *Client) doRequestOnce(method, url string, payload interface{}, response interface{}) error {
	var body io.Reader
	if payload != nil {
		jsonData, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}
		body = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.config.APIToken))
	req.Header.Set("User-Agent", fmt.Sprintf("right-sizer-operator/%s", c.config.OperatorVersion))

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &httpError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	// Parse response if output parameter provided
	if response != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, response); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// batchFlushLoop periodically flushes queued events
func (c *Client) batchFlushLoop(ctx context.Context) {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.BatchFlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			if err := c.FlushEvents(); err != nil {
				logger.Error("Failed to flush event batch: %v", err)
			}
		}
	}
}

// heartbeatLoop periodically sends status updates
func (c *Client) heartbeatLoop(ctx context.Context) {
	defer c.wg.Done()

	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			status := Status{
				Status: "healthy", // TODO: Get actual health status
			}
			if err := c.SendStatus(status); err != nil {
				logger.Error("Failed to send heartbeat: %v", err)
			} else {
				logger.Debug("💓 Heartbeat sent to dashboard")
			}
		}
	}
}

// httpError represents an HTTP error response
type httpError struct {
	StatusCode int
	Body       string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}

// Helper functions for creating events

// NewResizeEvent creates a resize event
func NewResizeEvent(eventType EventType, namespace, podName, containerName string, metadata map[string]interface{}) Event {
	var severity EventSeverity
	var message string

	switch eventType {
	case EventResizeStarted:
		severity = SeverityInfo
		message = "Container resize started"
	case EventResizeCompleted:
		severity = SeverityInfo
		message = "Container resize completed successfully"
	case EventResizeFailed:
		severity = SeverityError
		message = "Container resize failed"
	default:
		severity = SeverityInfo
		message = "Container resize event"
	}

	return Event{
		Type:          eventType,
		Severity:      severity,
		Namespace:     namespace,
		PodName:       podName,
		ContainerName: containerName,
		Message:       message,
		Metadata:      metadata,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}
}

// NewErrorEvent creates an error event
func NewErrorEvent(message string, metadata map[string]interface{}) Event {
	return Event{
		Type:      EventError,
		Severity:  SeverityError,
		Message:   message,
		Metadata:  metadata,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// NewRecommendationEvent creates a recommendation event
func NewRecommendationEvent(namespace, podName, containerName string, metadata map[string]interface{}) Event {
	return Event{
		Type:          EventRecommendationCreated,
		Severity:      SeverityInfo,
		Namespace:     namespace,
		PodName:       podName,
		ContainerName: containerName,
		Message:       "New resource recommendation created",
		Metadata:      metadata,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}
}

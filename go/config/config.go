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

// Package config provides configuration management for the right-sizer.
package config

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Config holds all configuration for resource sizing
// This configuration is now loaded from CRDs instead of environment variables
// NotificationConfig holds notification settings
type NotificationConfig struct {
	EnableNotifications bool     // Enable sending notifications
	SlackWebhookURL     string   // Slack webhook URL for notifications
	EmailRecipients     []string // Email addresses to notify
	SMTPHost            string   // SMTP server host
	SMTPPort            int      // SMTP server port
	SMTPUsername        string   // SMTP username
	SMTPPassword        string   // SMTP password
}

type Config struct {
	mu sync.RWMutex

	// Request multipliers - how much to multiply usage to get requests
	CPURequestMultiplier    float64
	MemoryRequestMultiplier float64

	// Request additions - fixed amount to add to usage for requests
	CPURequestAddition    int64 // in millicores
	MemoryRequestAddition int64 // in MB

	// Limit multipliers - how much to multiply requests to get limits
	CPULimitMultiplier    float64
	MemoryLimitMultiplier float64

	// Limit additions - fixed amount to add to requests for limits
	CPULimitAddition    int64 // in millicores
	MemoryLimitAddition int64 // in MB

	// Maximum caps for resources
	MaxCPULimit    int64 // in millicores
	MaxMemoryLimit int64 // in MB

	// Minimum values for resources
	MinCPURequest    int64 // in millicores
	MinMemoryRequest int64 // in MB

	// Algorithm for resource calculation
	Algorithm string // percentile, peak, average

	// Operational configuration
	ResizeInterval time.Duration // How often to check and resize resources
	LogLevel       string        // Log level: debug, info, warn, error
	MaxRetries     int           // Maximum retry attempts for operations
	RetryInterval  time.Duration // Interval between retries
	MetricsEnabled bool          // Enable Prometheus metrics
	MetricsPort    int           // Port for metrics endpoint

	// Rate limiting and concurrency control
	QPS                     float32 // Queries Per Second for K8s API client
	Burst                   int     // Burst capacity for K8s API client
	MaxConcurrentReconciles int     // Max concurrent reconciles per controller
	AuditEnabled            bool    // Enable audit logging for resource changes
	DryRun                  bool    // Only log recommendations without applying changes
	SafetyThreshold         float64 // Safety threshold for resource changes (0-1)

	// Batch processing configuration for API server protection
	BatchSize           int           // Number of pods to process per batch
	DelayBetweenBatches time.Duration // Delay between processing batches
	DelayBetweenPods    time.Duration // Delay between individual pod updates

	// Global constraints
	MaxCPUCores                int  // Global limit for CPU cores
	MaxMemoryGB                int  // Global limit for memory in GB
	PreventOOMKill             bool // Prevent OOM kills globally
	RespectPodDisruptionBudget bool // Respect Pod Disruption Budgets globally

	// Namespace filters
	NamespaceInclude []string // Namespaces to include
	NamespaceExclude []string // Namespaces to exclude
	SystemNamespaces []string // System namespaces to exclude

	// Advanced features
	HistoryDays         int      // Days of history to keep for trend analysis
	CustomMetrics       []string // Custom metrics to consider
	AdmissionController bool     // Enable admission controller for validation

	// Metrics provider configuration
	MetricsProvider       string // "metrics-server" or "prometheus"
	PrometheusURL         string // URL for Prometheus if used
	MetricsServerEndpoint string // Endpoint for metrics server

	// Metrics configuration
	AggregationMethod    string // avg, max, min, sum
	HistoryRetention     string // Duration for metrics history
	IncludeCustomMetrics bool   // Enable custom metrics

	// Feature flags
	EnableInPlaceResize bool // Enable in-place pod resizing (Kubernetes 1.33+)

	// QoS preservation settings
	PreserveGuaranteedQoS      bool // Preserve Guaranteed QoS class during resizing
	ForceGuaranteedForCritical bool // Force Guaranteed QoS for critical workloads
	QoSTransitionWarning       bool // Warn when QoS class would change

	// Observability configuration
	EnableAuditLogging bool // Enable audit logging
	EnableProfiling    bool // Enable profiling
	ProfilingPort      int  // Port for profiling endpoint

	// Operator configuration
	HealthProbePort             int    // Port for health checks
	LeaderElectionLeaseDuration string // Duration for leader election lease
	LeaderElectionRenewDeadline string // Deadline for leader election renewal
	LeaderElectionRetryPeriod   string // Period for leader election retries
	LivenessEndpoint            string // Endpoint for liveness probe
	ReadinessEndpoint           string // Endpoint for readiness probe
	RetryAttempts               int    // Number of retry attempts
	SyncPeriod                  string // Period for reconciliation sync

	// Security configuration
	TLSCertDir            string // Directory for TLS certificates
	WebhookTimeoutSeconds int    // Timeout for webhook requests

	// Scaling thresholds
	MemoryScaleUpThreshold   float64 // Memory usage percentage to trigger scale up (0-1)
	MemoryScaleDownThreshold float64 // Memory usage percentage to trigger scale down (0-1)
	CPUScaleUpThreshold      float64 // CPU usage percentage to trigger scale up (0-1)
	CPUScaleDownThreshold    float64 // CPU usage percentage to trigger scale down (0-1)

	// Notification configuration
	NotificationConfig *NotificationConfig // Notification settings

	// Configuration source tracking
	ConfigSource string // "default" or "crd"
}

// Global config instance with thread-safe access
var (
	Global     *Config
	globalLock sync.RWMutex
)

// GetDefaults returns a new Config with default values
func GetDefaults() *Config {
	return &Config{
		// Default resource sizing values
		CPURequestMultiplier:    1.2,
		MemoryRequestMultiplier: 1.2,
		CPURequestAddition:      0,
		MemoryRequestAddition:   0,
		CPULimitMultiplier:      2.0,
		MemoryLimitMultiplier:   2.0,
		CPULimitAddition:        0,
		MemoryLimitAddition:     0,
		MaxCPULimit:             4000,
		MaxMemoryLimit:          8192,
		MinCPURequest:           10,
		MinMemoryRequest:        64,

		// Default algorithm
		Algorithm: "percentile",

		// Default QoS preservation settings
		PreserveGuaranteedQoS:      true,
		ForceGuaranteedForCritical: false,
		QoSTransitionWarning:       true,

		// Default operational settings
		ResizeInterval: 30 * time.Second,
		LogLevel:       "info",
		MaxRetries:     3,
		RetryInterval:  5 * time.Second,
		MetricsEnabled: true,
		MetricsPort:    9090,

		// Default rate limiting values
		QPS:                     20,
		Burst:                   30,
		MaxConcurrentReconciles: 3,
		AuditEnabled:            true,
		DryRun:                  false,
		SafetyThreshold:         0.5, // 50% change threshold

		// Default batch processing values
		BatchSize:           3,
		DelayBetweenBatches: 5 * time.Second,
		DelayBetweenPods:    500 * time.Millisecond,

		// Default global constraints
		MaxCPUCores:                16,
		MaxMemoryGB:                32,
		PreventOOMKill:             true,
		RespectPodDisruptionBudget: true,

		// Default namespace filters
		NamespaceInclude: []string{},
		NamespaceExclude: []string{},
		SystemNamespaces: []string{
			"kube-system",
			"kube-public",
			"kube-node-lease",
			"cert-manager",
			"ingress-nginx",
			"istio-system",
		},

		// Default advanced features
		HistoryDays:         7,
		AdmissionController: false,

		// Default metrics configuration
		MetricsProvider:       "metrics-server",
		MetricsServerEndpoint: "",
		PrometheusURL:         "http://prometheus:9090",
		AggregationMethod:     "avg",
		HistoryRetention:      "30d",
		IncludeCustomMetrics:  false,

		// Default feature flags
		EnableInPlaceResize: false,

		// Default observability configuration
		EnableAuditLogging: true,
		EnableProfiling:    false,
		ProfilingPort:      6060,

		// Default operator configuration
		HealthProbePort:             8081,
		LeaderElectionLeaseDuration: "15s",
		LeaderElectionRenewDeadline: "10s",
		LeaderElectionRetryPeriod:   "2s",
		LivenessEndpoint:            "/healthz",
		ReadinessEndpoint:           "/readyz",
		RetryAttempts:               3,
		SyncPeriod:                  "30s",

		// Default security configuration
		TLSCertDir:            "/tmp/certs",
		WebhookTimeoutSeconds: 10,

		// Default scaling thresholds
		MemoryScaleUpThreshold:   0.8, // Scale up when memory usage exceeds 80%
		MemoryScaleDownThreshold: 0.3, // Scale down when memory usage is below 30%
		CPUScaleUpThreshold:      0.8, // Scale up when CPU usage exceeds 80%
		CPUScaleDownThreshold:    0.3, // Scale down when CPU usage is below 30%

		// Default notification configuration
		NotificationConfig: &NotificationConfig{
			EnableNotifications: false,
			SlackWebhookURL:     "",
			EmailRecipients:     []string{},
			SMTPHost:            "",
			SMTPPort:            587,
			SMTPUsername:        "",
			SMTPPassword:        "",
		},

		// Mark as default configuration
		ConfigSource: "default",
	}
}

// Load initializes the configuration with defaults
// CRD-based configuration will override these defaults when applied
func Load() *Config {
	globalLock.Lock()
	defer globalLock.Unlock()

	if Global == nil {
		Global = GetDefaults()
	}
	return Global
}

// Get returns the global config instance, loading it if necessary
func Get() *Config {
	globalLock.RLock()
	if Global == nil {
		globalLock.RUnlock()
		globalLock.Lock()
		if Global == nil {
			Global = GetDefaults()
		}
		globalLock.Unlock()
		globalLock.RLock()
	}
	defer globalLock.RUnlock()
	return Global
}

// UpdateFromCRD updates the configuration from a CRD specification
// This is called by the RightSizerConfig controller when a CRD is created or updated
func (c *Config) UpdateFromCRD(
	cpuRequestMultiplier, memoryRequestMultiplier float64,
	cpuRequestAddition, memoryRequestAddition int64,
	cpuLimitMultiplier, memoryLimitMultiplier float64,
	cpuLimitAddition, memoryLimitAddition int64,
	minCPURequest, minMemoryRequest string,
	maxCPULimit, maxMemoryLimit string,
	resizeInterval time.Duration,
	dryRun bool,
	namespaceInclude, namespaceExclude, systemNamespaces []string,
	logLevel string,
	metricsEnabled bool,
	metricsPort int,
	auditEnabled bool,
	maxRetries int,
	retryInterval time.Duration,
	metricsProvider, prometheusURL string,
	enableInPlaceResize bool,
	qps float32, burst, maxConcurrentReconciles int,
	memoryScaleUpThreshold, memoryScaleDownThreshold float64,
	cpuScaleUpThreshold, cpuScaleDownThreshold float64,
	algorithm string,
	maxCPUCores, maxMemoryGB int,
	preventOOMKill, respectPodDisruptionBudget bool,
	aggregationMethod, historyRetention string,
	includeCustomMetrics bool,
	enableAuditLogging, enableProfiling bool,
	profilingPort int,
	healthProbePort int,
	leaderElectionLeaseDuration, leaderElectionRenewDeadline, leaderElectionRetryPeriod string,
	livenessEndpoint, readinessEndpoint string,
	retryAttempts int,
	syncPeriod string,
	tlsCertDir string,
	webhookTimeoutSeconds int,
) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update resource configuration
	if cpuRequestMultiplier > 0 {
		c.CPURequestMultiplier = cpuRequestMultiplier
	}
	if memoryRequestMultiplier > 0 {
		c.MemoryRequestMultiplier = memoryRequestMultiplier
	}
	c.CPURequestAddition = cpuRequestAddition
	c.MemoryRequestAddition = memoryRequestAddition

	if cpuLimitMultiplier > 0 {
		c.CPULimitMultiplier = cpuLimitMultiplier
	}
	if memoryLimitMultiplier > 0 {
		c.MemoryLimitMultiplier = memoryLimitMultiplier
	}
	c.CPULimitAddition = cpuLimitAddition
	c.MemoryLimitAddition = memoryLimitAddition

	if minCPURequest != "" {
		if parsed, err := parseResourceQuantity(minCPURequest, "cpu"); err == nil {
			c.MinCPURequest = parsed
		}
	}
	if minMemoryRequest != "" {
		if parsed, err := parseResourceQuantity(minMemoryRequest, "memory"); err == nil {
			c.MinMemoryRequest = parsed
		}
	}
	if maxCPULimit != "" {
		if parsed, err := parseResourceQuantity(maxCPULimit, "cpu"); err == nil {
			c.MaxCPULimit = parsed
		}
	}
	if maxMemoryLimit != "" {
		if parsed, err := parseResourceQuantity(maxMemoryLimit, "memory"); err == nil {
			c.MaxMemoryLimit = parsed
		}
	}

	// Update operational configuration
	if resizeInterval > 0 {
		c.ResizeInterval = resizeInterval
	}
	c.DryRun = dryRun

	// Update namespace filters
	if len(namespaceInclude) > 0 {
		c.NamespaceInclude = namespaceInclude
	}
	if len(namespaceExclude) > 0 {
		c.NamespaceExclude = namespaceExclude
	}
	if len(systemNamespaces) > 0 {
		c.SystemNamespaces = systemNamespaces
	}

	// Update observability settings
	if logLevel != "" {
		c.LogLevel = logLevel
	}
	c.MetricsEnabled = metricsEnabled
	if metricsPort > 0 {
		c.MetricsPort = metricsPort
	}
	c.AuditEnabled = auditEnabled

	// Update rate limiting configuration
	if qps > 0 {
		c.QPS = qps
	}
	if burst > 0 {
		c.Burst = burst
	}
	if maxConcurrentReconciles > 0 {
		c.MaxConcurrentReconciles = maxConcurrentReconciles
	}

	// Update metrics configuration
	if maxRetries > 0 {
		c.MaxRetries = maxRetries
	}
	if retryInterval > 0 {
		c.RetryInterval = retryInterval
	}

	// Update metrics provider configuration
	if metricsProvider != "" {
		c.MetricsProvider = metricsProvider
	}
	if prometheusURL != "" {
		c.PrometheusURL = prometheusURL
	}

	// Update feature flags
	c.EnableInPlaceResize = enableInPlaceResize

	// Update scaling thresholds
	if memoryScaleUpThreshold > 0 && memoryScaleUpThreshold <= 1 {
		c.MemoryScaleUpThreshold = memoryScaleUpThreshold
	}
	if memoryScaleDownThreshold > 0 && memoryScaleDownThreshold <= 1 {
		c.MemoryScaleDownThreshold = memoryScaleDownThreshold
	}
	if cpuScaleUpThreshold > 0 && cpuScaleUpThreshold <= 1 {
		c.CPUScaleUpThreshold = cpuScaleUpThreshold
	}
	if cpuScaleDownThreshold > 0 && cpuScaleDownThreshold <= 1 {
		c.CPUScaleDownThreshold = cpuScaleDownThreshold
	}

	// Update new fields
	if algorithm != "" {
		c.Algorithm = algorithm
	}
	if maxCPUCores > 0 {
		c.MaxCPUCores = maxCPUCores
	}
	if maxMemoryGB > 0 {
		c.MaxMemoryGB = maxMemoryGB
	}
	c.PreventOOMKill = preventOOMKill
	c.RespectPodDisruptionBudget = respectPodDisruptionBudget
	if aggregationMethod != "" {
		c.AggregationMethod = aggregationMethod
	}
	if historyRetention != "" {
		c.HistoryRetention = historyRetention
	}
	c.IncludeCustomMetrics = includeCustomMetrics
	c.EnableAuditLogging = enableAuditLogging
	c.EnableProfiling = enableProfiling
	if profilingPort > 0 {
		c.ProfilingPort = profilingPort
	}
	if healthProbePort > 0 {
		c.HealthProbePort = healthProbePort
	}
	if leaderElectionLeaseDuration != "" {
		c.LeaderElectionLeaseDuration = leaderElectionLeaseDuration
	}
	if leaderElectionRenewDeadline != "" {
		c.LeaderElectionRenewDeadline = leaderElectionRenewDeadline
	}
	if leaderElectionRetryPeriod != "" {
		c.LeaderElectionRetryPeriod = leaderElectionRetryPeriod
	}
	if livenessEndpoint != "" {
		c.LivenessEndpoint = livenessEndpoint
	}
	if readinessEndpoint != "" {
		c.ReadinessEndpoint = readinessEndpoint
	}
	if retryAttempts > 0 {
		c.RetryAttempts = retryAttempts
	}
	if syncPeriod != "" {
		c.SyncPeriod = syncPeriod
	}
	if tlsCertDir != "" {
		c.TLSCertDir = tlsCertDir
	}
	if webhookTimeoutSeconds > 0 {
		c.WebhookTimeoutSeconds = webhookTimeoutSeconds
	}

	// Mark configuration as coming from CRD
	c.ConfigSource = "crd"
}

// ResetToDefaults resets the configuration to default values
func (c *Config) ResetToDefaults() {
	c.mu.Lock()
	defer c.mu.Unlock()

	defaults := GetDefaults()
	// Copy fields individually to avoid copying the mutex
	c.CPURequestMultiplier = defaults.CPURequestMultiplier
	c.MemoryRequestMultiplier = defaults.MemoryRequestMultiplier
	c.CPURequestAddition = defaults.CPURequestAddition
	c.MemoryRequestAddition = defaults.MemoryRequestAddition
	c.CPULimitMultiplier = defaults.CPULimitMultiplier
	c.MemoryLimitMultiplier = defaults.MemoryLimitMultiplier
	c.CPULimitAddition = defaults.CPULimitAddition
	c.MemoryLimitAddition = defaults.MemoryLimitAddition
	c.MinCPURequest = defaults.MinCPURequest
	c.MinMemoryRequest = defaults.MinMemoryRequest
	c.MaxCPULimit = defaults.MaxCPULimit
	c.MaxMemoryLimit = defaults.MaxMemoryLimit
	c.Algorithm = defaults.Algorithm
	c.ResizeInterval = defaults.ResizeInterval
	c.LogLevel = defaults.LogLevel
	c.MaxRetries = defaults.MaxRetries
	c.RetryInterval = defaults.RetryInterval
	c.MetricsEnabled = defaults.MetricsEnabled
	c.MetricsPort = defaults.MetricsPort
	c.QPS = defaults.QPS
	c.Burst = defaults.Burst
	c.MaxConcurrentReconciles = defaults.MaxConcurrentReconciles
	c.AuditEnabled = defaults.AuditEnabled
	c.DryRun = defaults.DryRun
	c.SafetyThreshold = defaults.SafetyThreshold
	c.MaxCPUCores = defaults.MaxCPUCores
	c.MaxMemoryGB = defaults.MaxMemoryGB
	c.PreventOOMKill = defaults.PreventOOMKill
	c.RespectPodDisruptionBudget = defaults.RespectPodDisruptionBudget
	c.NamespaceInclude = defaults.NamespaceInclude
	c.NamespaceExclude = defaults.NamespaceExclude
	c.SystemNamespaces = defaults.SystemNamespaces
	c.HistoryDays = defaults.HistoryDays
	c.CustomMetrics = defaults.CustomMetrics
	c.AdmissionController = defaults.AdmissionController
	c.MetricsProvider = defaults.MetricsProvider
	c.PrometheusURL = defaults.PrometheusURL
	c.MetricsServerEndpoint = defaults.MetricsServerEndpoint
	c.AggregationMethod = defaults.AggregationMethod
	c.HistoryRetention = defaults.HistoryRetention
	c.IncludeCustomMetrics = defaults.IncludeCustomMetrics
	c.EnableInPlaceResize = defaults.EnableInPlaceResize
	c.PreserveGuaranteedQoS = defaults.PreserveGuaranteedQoS
	c.ForceGuaranteedForCritical = defaults.ForceGuaranteedForCritical
	c.QoSTransitionWarning = defaults.QoSTransitionWarning
	c.EnableAuditLogging = defaults.EnableAuditLogging
	c.EnableProfiling = defaults.EnableProfiling
	c.ProfilingPort = defaults.ProfilingPort
	c.HealthProbePort = defaults.HealthProbePort
	c.LeaderElectionLeaseDuration = defaults.LeaderElectionLeaseDuration
	c.LeaderElectionRenewDeadline = defaults.LeaderElectionRenewDeadline
	c.LeaderElectionRetryPeriod = defaults.LeaderElectionRetryPeriod
	c.LivenessEndpoint = defaults.LivenessEndpoint
	c.ReadinessEndpoint = defaults.ReadinessEndpoint
	c.RetryAttempts = defaults.RetryAttempts
	c.SyncPeriod = defaults.SyncPeriod
	c.TLSCertDir = defaults.TLSCertDir
	c.WebhookTimeoutSeconds = defaults.WebhookTimeoutSeconds
	c.MemoryScaleUpThreshold = defaults.MemoryScaleUpThreshold
	c.MemoryScaleDownThreshold = defaults.MemoryScaleDownThreshold
	c.CPUScaleUpThreshold = defaults.CPUScaleUpThreshold
	c.CPUScaleDownThreshold = defaults.CPUScaleDownThreshold
	c.NotificationConfig = defaults.NotificationConfig
	c.ConfigSource = defaults.ConfigSource
}

// Validate checks the configuration for consistency and validity
func (c *Config) Validate() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var errors []string

	// Validate multipliers
	if c.CPURequestMultiplier <= 0 {
		errors = append(errors, "CPU request multiplier must be positive")
	}
	if c.MemoryRequestMultiplier <= 0 {
		errors = append(errors, "memory request multiplier must be positive")
	}
	if c.CPULimitMultiplier <= 0 {
		errors = append(errors, "CPU limit multiplier must be positive")
	}
	if c.MemoryLimitMultiplier <= 0 {
		errors = append(errors, "memory limit multiplier must be positive")
	}

	// Validate resource boundaries
	if c.MinCPURequest <= 0 {
		errors = append(errors, "minimum CPU request must be positive")
	}
	if c.MinMemoryRequest <= 0 {
		errors = append(errors, "minimum memory request must be positive")
	}
	if c.MaxCPULimit <= c.MinCPURequest {
		errors = append(errors, "maximum CPU limit must be greater than minimum CPU request")
	}
	if c.MaxMemoryLimit <= c.MinMemoryRequest {
		errors = append(errors, "maximum memory limit must be greater than minimum memory request")
	}

	// Validate intervals
	if c.ResizeInterval <= 0 {
		errors = append(errors, "resize interval must be positive")
	}
	if c.RetryInterval <= 0 {
		errors = append(errors, "retry interval must be positive")
	}

	// Validate operational settings
	if c.MaxRetries < 0 {
		errors = append(errors, "max retries cannot be negative")
	}
	if c.MetricsPort <= 0 || c.MetricsPort > 65535 {
		errors = append(errors, "metrics port must be between 1 and 65535")
	}
	if c.SafetyThreshold < 0 || c.SafetyThreshold > 1 {
		errors = append(errors, "safety threshold must be between 0 and 1")
	}
	if c.HistoryDays <= 0 {
		errors = append(errors, "history days must be positive")
	}

	// Validate log level
	validLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLevels[c.LogLevel] {
		errors = append(errors, fmt.Sprintf("invalid log level: %s (must be debug, info, warn, or error)", c.LogLevel))
	}

	// Validate metrics provider
	validProviders := map[string]bool{
		"metrics-server": true, "prometheus": true,
	}
	if !validProviders[c.MetricsProvider] {
		errors = append(errors, fmt.Sprintf("invalid metrics provider: %s (must be metrics-server or prometheus)", c.MetricsProvider))
	}

	// Validate scaling thresholds
	if c.MemoryScaleUpThreshold <= 0 || c.MemoryScaleUpThreshold > 1 {
		errors = append(errors, "memory scale up threshold must be between 0 and 1")
	}
	if c.MemoryScaleDownThreshold <= 0 || c.MemoryScaleDownThreshold > 1 {
		errors = append(errors, "memory scale down threshold must be between 0 and 1")
	}
	if c.MemoryScaleDownThreshold >= c.MemoryScaleUpThreshold {
		errors = append(errors, "memory scale down threshold must be less than scale up threshold")
	}
	if c.CPUScaleUpThreshold <= 0 || c.CPUScaleUpThreshold > 1 {
		errors = append(errors, "CPU scale up threshold must be between 0 and 1")
	}
	if c.CPUScaleDownThreshold <= 0 || c.CPUScaleDownThreshold > 1 {
		errors = append(errors, "CPU scale down threshold must be between 0 and 1")
	}
	if c.CPUScaleDownThreshold >= c.CPUScaleUpThreshold {
		errors = append(errors, "CPU scale down threshold must be less than scale up threshold")
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// IsNamespaceIncluded checks if a namespace should be processed based on include/exclude filters
func (c *Config) IsNamespaceIncluded(namespace string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check system namespaces first - always exclude them
	if len(c.SystemNamespaces) > 0 {
		for _, ns := range c.SystemNamespaces {
			if ns == namespace {
				return false
			}
		}
	}

	// If include list is specified, namespace must be in it
	if len(c.NamespaceInclude) > 0 {
		found := false
		for _, ns := range c.NamespaceInclude {
			if ns == namespace {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// If exclude list is specified, namespace must not be in it
	if len(c.NamespaceExclude) > 0 {
		for _, ns := range c.NamespaceExclude {
			if ns == namespace {
				return false
			}
		}
	}

	return true
}

// GetRetryConfig returns retry configuration for operations
func (c *Config) GetRetryConfig() (maxRetries int, interval time.Duration) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.MaxRetries, c.RetryInterval
}

// IsChangeWithinSafetyThreshold checks if a resource change is within safe limits
func (c *Config) IsChangeWithinSafetyThreshold(current, new int64) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if current == 0 {
		return true // No existing resource, any change is allowed
	}

	change := float64(new-current) / float64(current)
	if change < 0 {
		change = -change // Use absolute value
	}

	return change <= c.SafetyThreshold
}

// Clone creates a deep copy of the configuration
func (c *Config) Clone() *Config {
	c.mu.RLock()
	defer c.mu.RUnlock()

	clone := &Config{
		CPURequestMultiplier:        c.CPURequestMultiplier,
		MemoryRequestMultiplier:     c.MemoryRequestMultiplier,
		CPURequestAddition:          c.CPURequestAddition,
		MemoryRequestAddition:       c.MemoryRequestAddition,
		CPULimitMultiplier:          c.CPULimitMultiplier,
		MemoryLimitMultiplier:       c.MemoryLimitMultiplier,
		CPULimitAddition:            c.CPULimitAddition,
		MemoryLimitAddition:         c.MemoryLimitAddition,
		MaxCPULimit:                 c.MaxCPULimit,
		MaxMemoryLimit:              c.MaxMemoryLimit,
		MinCPURequest:               c.MinCPURequest,
		MinMemoryRequest:            c.MinMemoryRequest,
		Algorithm:                   c.Algorithm,
		ResizeInterval:              c.ResizeInterval,
		LogLevel:                    c.LogLevel,
		MaxRetries:                  c.MaxRetries,
		RetryInterval:               c.RetryInterval,
		MetricsEnabled:              c.MetricsEnabled,
		MetricsPort:                 c.MetricsPort,
		AuditEnabled:                c.AuditEnabled,
		QPS:                         c.QPS,
		Burst:                       c.Burst,
		MaxConcurrentReconciles:     c.MaxConcurrentReconciles,
		DryRun:                      c.DryRun,
		SafetyThreshold:             c.SafetyThreshold,
		MaxCPUCores:                 c.MaxCPUCores,
		MaxMemoryGB:                 c.MaxMemoryGB,
		PreventOOMKill:              c.PreventOOMKill,
		RespectPodDisruptionBudget:  c.RespectPodDisruptionBudget,
		HistoryDays:                 c.HistoryDays,
		AdmissionController:         c.AdmissionController,
		MetricsProvider:             c.MetricsProvider,
		PrometheusURL:               c.PrometheusURL,
		MetricsServerEndpoint:       c.MetricsServerEndpoint,
		AggregationMethod:           c.AggregationMethod,
		HistoryRetention:            c.HistoryRetention,
		IncludeCustomMetrics:        c.IncludeCustomMetrics,
		EnableInPlaceResize:         c.EnableInPlaceResize,
		PreserveGuaranteedQoS:       c.PreserveGuaranteedQoS,
		ForceGuaranteedForCritical:  c.ForceGuaranteedForCritical,
		QoSTransitionWarning:        c.QoSTransitionWarning,
		EnableAuditLogging:          c.EnableAuditLogging,
		EnableProfiling:             c.EnableProfiling,
		ProfilingPort:               c.ProfilingPort,
		HealthProbePort:             c.HealthProbePort,
		LeaderElectionLeaseDuration: c.LeaderElectionLeaseDuration,
		LeaderElectionRenewDeadline: c.LeaderElectionRenewDeadline,
		LeaderElectionRetryPeriod:   c.LeaderElectionRetryPeriod,
		LivenessEndpoint:            c.LivenessEndpoint,
		ReadinessEndpoint:           c.ReadinessEndpoint,
		RetryAttempts:               c.RetryAttempts,
		SyncPeriod:                  c.SyncPeriod,
		TLSCertDir:                  c.TLSCertDir,
		WebhookTimeoutSeconds:       c.WebhookTimeoutSeconds,
		MemoryScaleUpThreshold:      c.MemoryScaleUpThreshold,
		MemoryScaleDownThreshold:    c.MemoryScaleDownThreshold,
		CPUScaleUpThreshold:         c.CPUScaleUpThreshold,
		CPUScaleDownThreshold:       c.CPUScaleDownThreshold,
		ConfigSource:                c.ConfigSource,
	}

	// Deep copy slices
	if len(c.NamespaceInclude) > 0 {
		clone.NamespaceInclude = make([]string, len(c.NamespaceInclude))
		copy(clone.NamespaceInclude, c.NamespaceInclude)
	}
	if len(c.NamespaceExclude) > 0 {
		clone.NamespaceExclude = make([]string, len(c.NamespaceExclude))
		copy(clone.NamespaceExclude, c.NamespaceExclude)
	}
	if len(c.SystemNamespaces) > 0 {
		clone.SystemNamespaces = make([]string, len(c.SystemNamespaces))
		copy(clone.SystemNamespaces, c.SystemNamespaces)
	}
	if len(c.CustomMetrics) > 0 {
		clone.CustomMetrics = make([]string, len(c.CustomMetrics))
		copy(clone.CustomMetrics, c.CustomMetrics)
	}

	// Deep copy notification config
	if c.NotificationConfig != nil {
		clone.NotificationConfig = &NotificationConfig{
			EnableNotifications: c.NotificationConfig.EnableNotifications,
			SlackWebhookURL:     c.NotificationConfig.SlackWebhookURL,
			SMTPHost:            c.NotificationConfig.SMTPHost,
			SMTPPort:            c.NotificationConfig.SMTPPort,
			SMTPUsername:        c.NotificationConfig.SMTPUsername,
			SMTPPassword:        c.NotificationConfig.SMTPPassword,
		}
		if len(c.NotificationConfig.EmailRecipients) > 0 {
			clone.NotificationConfig.EmailRecipients = make([]string, len(c.NotificationConfig.EmailRecipients))
			copy(clone.NotificationConfig.EmailRecipients, c.NotificationConfig.EmailRecipients)
		}
	}

	return clone
}

// GetSafeValue safely retrieves a configuration value with read lock
func (c *Config) GetSafeValue(getter func(*Config) interface{}) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return getter(c)
}

// parseResourceQuantity parses Kubernetes resource quantity strings to int64 values
func parseResourceQuantity(quantity string, resourceType string) (int64, error) {
	if quantity == "" {
		return 0, fmt.Errorf("empty quantity string")
	}

	// Simple parsing for common cases
	// For CPU: "10m" -> 10 (millicores), "1" -> 1000 (millicores)
	// For Memory: "64Mi" -> 64 (MiB), "1Gi" -> 1024 (MiB)

	if resourceType == "cpu" {
		if len(quantity) > 0 && quantity[len(quantity)-1:] == "m" {
			// Parse millicores (e.g., "10m" -> 10)
			return parseIntFromString(quantity[:len(quantity)-1])
		}
		// Assume whole cores, convert to millicores (e.g., "2" -> 2000)
		if val, err := parseIntFromString(quantity); err == nil {
			return val * 1000, nil
		}
	}

	if resourceType == "memory" {
		if len(quantity) >= 2 {
			suffix := quantity[len(quantity)-2:]
			if suffix == "Mi" {
				// Parse MiB (e.g., "64Mi" -> 64)
				return parseIntFromString(quantity[:len(quantity)-2])
			}
			if suffix == "Gi" {
				// Parse GiB, convert to MiB (e.g., "1Gi" -> 1024)
				if val, err := parseIntFromString(quantity[:len(quantity)-2]); err == nil {
					return val * 1024, nil
				}
			}
		}
		// Assume MiB if no suffix
		return parseIntFromString(quantity)
	}

	return 0, fmt.Errorf("unknown resource type or format: %s", quantity)
}

// parseIntFromString is a simple integer parser
func parseIntFromString(s string) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty string")
	}
	var result int64
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("invalid integer: %s", s)
		}
		result = result*10 + int64(ch-'0')
	}
	return result, nil
}

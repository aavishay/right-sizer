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

package config

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Config holds all configuration for resource sizing
// This configuration is now loaded from CRDs instead of environment variables
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

	// Operational configuration
	ResizeInterval  time.Duration // How often to check and resize resources
	LogLevel        string        // Log level: debug, info, warn, error
	MaxRetries      int           // Maximum retry attempts for operations
	RetryInterval   time.Duration // Interval between retries
	MetricsEnabled  bool          // Enable Prometheus metrics
	MetricsPort     int           // Port for metrics endpoint
	AuditEnabled    bool          // Enable audit logging for resource changes
	DryRun          bool          // Only log recommendations without applying changes
	SafetyThreshold float64       // Safety threshold for resource changes (0-1)

	// Namespace filters
	NamespaceInclude []string // Namespaces to include
	NamespaceExclude []string // Namespaces to exclude

	// Advanced features
	HistoryDays         int      // Days of history to keep for trend analysis
	CustomMetrics       []string // Custom metrics to consider
	AdmissionController bool     // Enable admission controller for validation

	// Metrics provider configuration
	MetricsProvider       string // "metrics-server" or "prometheus"
	PrometheusURL         string // URL for Prometheus if used
	MetricsServerEndpoint string // Endpoint for metrics server

	// Feature flags
	EnableInPlaceResize bool // Enable in-place pod resizing (Kubernetes 1.33+)

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

		// Default operational settings
		ResizeInterval:  30 * time.Second,
		LogLevel:        "info",
		MaxRetries:      3,
		RetryInterval:   5 * time.Second,
		MetricsEnabled:  true,
		MetricsPort:     9090,
		AuditEnabled:    true,
		DryRun:          false,
		SafetyThreshold: 0.5, // 50% change threshold

		// Default advanced features
		HistoryDays:         7,
		AdmissionController: false,

		// Default metrics configuration
		MetricsProvider:       "metrics-server",
		MetricsServerEndpoint: "",
		PrometheusURL:         "http://prometheus:9090",

		// Default feature flags
		EnableInPlaceResize: false,

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
	defer globalLock.RUnlock()

	if Global == nil {
		globalLock.RUnlock()
		globalLock.Lock()
		defer globalLock.Unlock()
		if Global == nil {
			Global = GetDefaults()
		}
		globalLock.RLock()
	}
	return Global
}

// UpdateFromCRD updates the configuration from a CRD specification
// This is called by the RightSizerConfig controller when a CRD is created or updated
func (c *Config) UpdateFromCRD(
	cpuRequestMultiplier, memoryRequestMultiplier float64,
	cpuRequestAddition, memoryRequestAddition int64,
	cpuLimitMultiplier, memoryLimitMultiplier float64,
	cpuLimitAddition, memoryLimitAddition int64,
	minCPURequest, minMemoryRequest int64,
	maxCPULimit, maxMemoryLimit int64,
	resizeInterval time.Duration,
	dryRun bool,
	namespaceInclude, namespaceExclude []string,
	logLevel string,
	metricsEnabled bool,
	metricsPort int,
	auditEnabled bool,
	maxRetries int,
	retryInterval time.Duration,
	metricsProvider, prometheusURL string,
	enableInPlaceResize bool,
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

	if minCPURequest > 0 {
		c.MinCPURequest = minCPURequest
	}
	if minMemoryRequest > 0 {
		c.MinMemoryRequest = minMemoryRequest
	}
	if maxCPULimit > 0 {
		c.MaxCPULimit = maxCPULimit
	}
	if maxMemoryLimit > 0 {
		c.MaxMemoryLimit = maxMemoryLimit
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

	// Update observability settings
	if logLevel != "" {
		c.LogLevel = logLevel
	}
	c.MetricsEnabled = metricsEnabled
	if metricsPort > 0 {
		c.MetricsPort = metricsPort
	}
	c.AuditEnabled = auditEnabled

	// Update retry configuration
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

	// Mark configuration as coming from CRD
	c.ConfigSource = "crd"
}

// ResetToDefaults resets the configuration to default values
func (c *Config) ResetToDefaults() {
	c.mu.Lock()
	defer c.mu.Unlock()

	defaults := GetDefaults()
	*c = *defaults
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

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// IsNamespaceIncluded checks if a namespace should be processed based on include/exclude filters
func (c *Config) IsNamespaceIncluded(namespace string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

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
		CPURequestMultiplier:    c.CPURequestMultiplier,
		MemoryRequestMultiplier: c.MemoryRequestMultiplier,
		CPURequestAddition:      c.CPURequestAddition,
		MemoryRequestAddition:   c.MemoryRequestAddition,
		CPULimitMultiplier:      c.CPULimitMultiplier,
		MemoryLimitMultiplier:   c.MemoryLimitMultiplier,
		CPULimitAddition:        c.CPULimitAddition,
		MemoryLimitAddition:     c.MemoryLimitAddition,
		MaxCPULimit:             c.MaxCPULimit,
		MaxMemoryLimit:          c.MaxMemoryLimit,
		MinCPURequest:           c.MinCPURequest,
		MinMemoryRequest:        c.MinMemoryRequest,
		ResizeInterval:          c.ResizeInterval,
		LogLevel:                c.LogLevel,
		MaxRetries:              c.MaxRetries,
		RetryInterval:           c.RetryInterval,
		MetricsEnabled:          c.MetricsEnabled,
		MetricsPort:             c.MetricsPort,
		AuditEnabled:            c.AuditEnabled,
		DryRun:                  c.DryRun,
		SafetyThreshold:         c.SafetyThreshold,
		HistoryDays:             c.HistoryDays,
		AdmissionController:     c.AdmissionController,
		MetricsProvider:         c.MetricsProvider,
		PrometheusURL:           c.PrometheusURL,
		MetricsServerEndpoint:   c.MetricsServerEndpoint,
		EnableInPlaceResize:     c.EnableInPlaceResize,
		ConfigSource:            c.ConfigSource,
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
	if len(c.CustomMetrics) > 0 {
		clone.CustomMetrics = make([]string, len(c.CustomMetrics))
		copy(clone.CustomMetrics, c.CustomMetrics)
	}

	return clone
}

// GetSafeValue safely retrieves a configuration value with read lock
func (c *Config) GetSafeValue(getter func(*Config) interface{}) interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return getter(c)
}

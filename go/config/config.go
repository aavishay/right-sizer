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
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for resource sizing
type Config struct {
	// Request multipliers - how much to multiply usage to get requests
	CPURequestMultiplier    float64
	MemoryRequestMultiplier float64

	// Limit multipliers - how much to multiply requests to get limits
	CPULimitMultiplier    float64
	MemoryLimitMultiplier float64

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
	NamespaceInclude []string // Namespaces to include (from KUBE_NAMESPACE_INCLUDE)
	NamespaceExclude []string // Namespaces to exclude (from KUBE_NAMESPACE_EXCLUDE)

	// Advanced features
	PolicyBasedSizing   bool     // Enable policy-based sizing rules
	HistoryDays         int      // Days of history to keep for trend analysis
	CustomMetrics       []string // Custom metrics to consider
	AdmissionController bool     // Enable admission controller for validation
}

// Global config instance
var Global *Config

// Load initializes the configuration from environment variables
func Load() *Config {
	cfg := &Config{
		// Default values
		CPURequestMultiplier:    1.2,
		MemoryRequestMultiplier: 1.2,
		CPULimitMultiplier:      2.0,
		MemoryLimitMultiplier:   2.0,
		MaxCPULimit:             4000,
		MaxMemoryLimit:          8192,
		MinCPURequest:           10,
		MinMemoryRequest:        64,
		ResizeInterval:          30 * time.Second,
		LogLevel:                "info",
		MaxRetries:              3,
		RetryInterval:           5 * time.Second,
		MetricsEnabled:          true,
		MetricsPort:             9090,
		AuditEnabled:            true,
		DryRun:                  false,
		SafetyThreshold:         0.5, // 50% change threshold
		PolicyBasedSizing:       false,
		HistoryDays:             7,
		AdmissionController:     false,
	}

	// Load from environment variables with defaults
	if val := os.Getenv("CPU_REQUEST_MULTIPLIER"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			cfg.CPURequestMultiplier = f
			log.Printf("CPU_REQUEST_MULTIPLIER set to: %.2f", f)
		} else {
			log.Printf("Warning: Invalid CPU_REQUEST_MULTIPLIER value: %s", val)
		}
	}

	if val := os.Getenv("MEMORY_REQUEST_MULTIPLIER"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			cfg.MemoryRequestMultiplier = f
			log.Printf("MEMORY_REQUEST_MULTIPLIER set to: %.2f", f)
		} else {
			log.Printf("Warning: Invalid MEMORY_REQUEST_MULTIPLIER value: %s", val)
		}
	}

	if val := os.Getenv("CPU_LIMIT_MULTIPLIER"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			cfg.CPULimitMultiplier = f
			log.Printf("CPU_LIMIT_MULTIPLIER set to: %.2f", f)
		} else {
			log.Printf("Warning: Invalid CPU_LIMIT_MULTIPLIER value: %s", val)
		}
	}

	if val := os.Getenv("MEMORY_LIMIT_MULTIPLIER"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			cfg.MemoryLimitMultiplier = f
			log.Printf("MEMORY_LIMIT_MULTIPLIER set to: %.2f", f)
		} else {
			log.Printf("Warning: Invalid MEMORY_LIMIT_MULTIPLIER value: %s", val)
		}
	}

	if val := os.Getenv("MAX_CPU_LIMIT"); val != "" {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			cfg.MaxCPULimit = i
			log.Printf("MAX_CPU_LIMIT set to: %d millicores", i)
		} else {
			log.Printf("Warning: Invalid MAX_CPU_LIMIT value: %s", val)
		}
	}

	if val := os.Getenv("MAX_MEMORY_LIMIT"); val != "" {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			cfg.MaxMemoryLimit = i
			log.Printf("MAX_MEMORY_LIMIT set to: %d MB", i)
		} else {
			log.Printf("Warning: Invalid MAX_MEMORY_LIMIT value: %s", val)
		}
	}

	if val := os.Getenv("MIN_CPU_REQUEST"); val != "" {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			cfg.MinCPURequest = i
			log.Printf("MIN_CPU_REQUEST set to: %d millicores", i)
		} else {
			log.Printf("Warning: Invalid MIN_CPU_REQUEST value: %s", val)
		}
	}

	if val := os.Getenv("MIN_MEMORY_REQUEST"); val != "" {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			cfg.MinMemoryRequest = i
			log.Printf("MIN_MEMORY_REQUEST set to: %d MB", i)
		} else {
			log.Printf("Warning: Invalid MIN_MEMORY_REQUEST value: %s", val)
		}
	}

	// Load RESIZE_INTERVAL
	if val := os.Getenv("RESIZE_INTERVAL"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			cfg.ResizeInterval = duration
			log.Printf("RESIZE_INTERVAL set to: %v", duration)
		} else {
			// Try parsing as seconds if duration parsing fails
			if seconds, err := strconv.Atoi(val); err == nil {
				cfg.ResizeInterval = time.Duration(seconds) * time.Second
				log.Printf("RESIZE_INTERVAL set to: %v", cfg.ResizeInterval)
			} else {
				log.Printf("Warning: Invalid RESIZE_INTERVAL value: %s (use format like '30s', '5m', '1h')", val)
			}
		}
	}

	// Load LOG_LEVEL
	if val := os.Getenv("LOG_LEVEL"); val != "" {
		validLevels := map[string]bool{
			"debug": true,
			"info":  true,
			"warn":  true,
			"error": true,
		}
		if validLevels[val] {
			cfg.LogLevel = val
		}
	}

	// Load KUBE_NAMESPACE_INCLUDE (CSV)
	if val := os.Getenv("KUBE_NAMESPACE_INCLUDE"); val != "" {
		cfg.NamespaceInclude = parseCSV(val)
		log.Printf("KUBE_NAMESPACE_INCLUDE set to: %v", cfg.NamespaceInclude)
	}

	// Load KUBE_NAMESPACE_EXCLUDE (CSV)
	if val := os.Getenv("KUBE_NAMESPACE_EXCLUDE"); val != "" {
		cfg.NamespaceExclude = parseCSV(val)
		log.Printf("KUBE_NAMESPACE_EXCLUDE set to: %v", cfg.NamespaceExclude)
	}

	// Load MAX_RETRIES
	if val := os.Getenv("MAX_RETRIES"); val != "" {
		if i, err := strconv.Atoi(val); err == nil && i > 0 {
			cfg.MaxRetries = i
			log.Printf("MAX_RETRIES set to: %d", i)
		}
	}

	// Load RETRY_INTERVAL
	if val := os.Getenv("RETRY_INTERVAL"); val != "" {
		if duration, err := time.ParseDuration(val); err == nil {
			cfg.RetryInterval = duration
			log.Printf("RETRY_INTERVAL set to: %v", duration)
		}
	}

	// Load METRICS_ENABLED
	if val := os.Getenv("METRICS_ENABLED"); val != "" {
		cfg.MetricsEnabled = strings.ToLower(val) == "true"
		log.Printf("METRICS_ENABLED set to: %v", cfg.MetricsEnabled)
	}

	// Load METRICS_PORT
	if val := os.Getenv("METRICS_PORT"); val != "" {
		if i, err := strconv.Atoi(val); err == nil && i > 0 && i < 65536 {
			cfg.MetricsPort = i
			log.Printf("METRICS_PORT set to: %d", i)
		}
	}

	// Load AUDIT_ENABLED
	if val := os.Getenv("AUDIT_ENABLED"); val != "" {
		cfg.AuditEnabled = strings.ToLower(val) == "true"
		log.Printf("AUDIT_ENABLED set to: %v", cfg.AuditEnabled)
	}

	// Load DRY_RUN
	if val := os.Getenv("DRY_RUN"); val != "" {
		cfg.DryRun = strings.ToLower(val) == "true"
		log.Printf("DRY_RUN set to: %v", cfg.DryRun)
	}

	// Load SAFETY_THRESHOLD
	if val := os.Getenv("SAFETY_THRESHOLD"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil && f >= 0 && f <= 1 {
			cfg.SafetyThreshold = f
			log.Printf("SAFETY_THRESHOLD set to: %.2f", f)
		}
	}

	// Load POLICY_BASED_SIZING
	if val := os.Getenv("POLICY_BASED_SIZING"); val != "" {
		cfg.PolicyBasedSizing = strings.ToLower(val) == "true"
		log.Printf("POLICY_BASED_SIZING set to: %v", cfg.PolicyBasedSizing)
	}

	// Load HISTORY_DAYS
	if val := os.Getenv("HISTORY_DAYS"); val != "" {
		if i, err := strconv.Atoi(val); err == nil && i > 0 {
			cfg.HistoryDays = i
			log.Printf("HISTORY_DAYS set to: %d", i)
		}
	}

	// Load CUSTOM_METRICS (CSV)
	if val := os.Getenv("CUSTOM_METRICS"); val != "" {
		cfg.CustomMetrics = parseCSV(val)
		log.Printf("CUSTOM_METRICS set to: %v", cfg.CustomMetrics)
	}

	// Load ADMISSION_CONTROLLER
	if val := os.Getenv("ADMISSION_CONTROLLER"); val != "" {
		cfg.AdmissionController = strings.ToLower(val) == "true"
		log.Printf("ADMISSION_CONTROLLER set to: %v", cfg.AdmissionController)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Printf("⚠️  Configuration validation failed: %v", err)
		log.Printf("⚠️  Continuing with potentially invalid configuration...")
	}

	Global = cfg
	return cfg
}

// Get returns the global config instance, loading it if necessary
func Get() *Config {
	if Global == nil {
		return Load()
	}
	return Global
}

// Validate checks the configuration for consistency and validity
func (c *Config) Validate() error {
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

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation errors: %s", strings.Join(errors, "; "))
	}

	return nil
}

// IsNamespaceIncluded checks if a namespace should be processed based on include/exclude filters
func (c *Config) IsNamespaceIncluded(namespace string) bool {
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
	return c.MaxRetries, c.RetryInterval
}

// IsChangeWithinSafetyThreshold checks if a resource change is within safe limits
func (c *Config) IsChangeWithinSafetyThreshold(current, new int64) bool {
	if current == 0 {
		return true // No existing resource, any change is allowed
	}

	change := float64(new-current) / float64(current)
	if change < 0 {
		change = -change // Use absolute value
	}

	return change <= c.SafetyThreshold
}

// parseCSV splits a comma-separated string into a slice, trimming spaces
func parseCSV(s string) []string {
	var out []string
	for _, v := range splitAndTrim(s, ',') {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// splitAndTrim splits by sep and trims spaces
func splitAndTrim(s string, sep rune) []string {
	var res []string
	field := ""
	for _, c := range s {
		if c == sep {
			res = append(res, trimSpace(field))
			field = ""
		} else {
			field += string(c)
		}
	}
	res = append(res, trimSpace(field))
	return res
}

// trimSpace trims leading/trailing spaces
func trimSpace(s string) string {
	i, j := 0, len(s)-1
	for i <= j && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	for j >= i && (s[j] == ' ' || s[j] == '\t') {
		j--
	}
	return s[i : j+1]
}

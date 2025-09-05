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
	"sync"
	"testing"
	"time"
)

func TestGetDefaults(t *testing.T) {
	cfg := GetDefaults()

	// Test default values
	if cfg.CPURequestMultiplier != 1.2 {
		t.Errorf("Expected CPURequestMultiplier to be 1.2, got %f", cfg.CPURequestMultiplier)
	}

	if cfg.MemoryRequestMultiplier != 1.2 {
		t.Errorf("Expected MemoryRequestMultiplier to be 1.2, got %f", cfg.MemoryRequestMultiplier)
	}

	if cfg.ResizeInterval != 30*time.Second {
		t.Errorf("Expected ResizeInterval to be 30s, got %v", cfg.ResizeInterval)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("Expected LogLevel to be 'info', got %s", cfg.LogLevel)
	}

	if cfg.ConfigSource != "default" {
		t.Errorf("Expected ConfigSource to be 'default', got %s", cfg.ConfigSource)
	}

	if cfg.MetricsProvider != "metrics-server" {
		t.Errorf("Expected MetricsProvider to be 'metrics-server', got %s", cfg.MetricsProvider)
	}
}

func TestLoad(t *testing.T) {
	// Reset global config for testing
	Global = nil

	cfg := Load()
	if cfg == nil {
		t.Fatal("Load() returned nil")
	}

	// Verify it's cached
	cfg2 := Load()
	if cfg != cfg2 {
		t.Error("Load() should return the same instance when called twice")
	}
}

func TestGet(t *testing.T) {
	// Reset global config for testing
	Global = nil

	cfg := Get()
	if cfg == nil {
		t.Fatal("Get() returned nil")
	}

	// Test that subsequent calls return the same instance
	cfg2 := Get()
	if cfg != cfg2 {
		t.Error("Get() should return the same instance when called multiple times")
	}
}

func TestUpdateFromCRD(t *testing.T) {
	cfg := GetDefaults()

	// Update with CRD values
	cfg.UpdateFromCRD(
		1.5,                               // cpuRequestMultiplier
		1.3,                               // memoryRequestMultiplier
		100,                               // cpuRequestAddition
		256,                               // memoryRequestAddition
		2.5,                               // cpuLimitMultiplier
		2.0,                               // memoryLimitMultiplier
		0,                                 // cpuLimitAddition
		0,                                 // memoryLimitAddition
		10,                                // minCPURequest
		64,                                // minMemoryRequest
		4000,                              // maxCPULimit
		8192,                              // maxMemoryLimit
		60*time.Second,                    // resizeInterval
		true,                              // dryRun
		[]string{"default", "production"}, // namespaceInclude
		[]string{"kube-system"},           // namespaceExclude
		"debug",                           // logLevel
		true,                              // metricsEnabled
		9090,                              // metricsPort
		true,                              // auditEnabled
		5,                                 // maxRetries
		10*time.Second,                    // retryInterval
		"prometheus",                      // metricsProvider
		"http://prom:9090",                // prometheusURL
		true,                              // enableInPlaceResize
		10.0,                              // qps
		20,                                // burst
		5,                                 // maxConcurrentReconciles
		0.8,                               // memoryScaleUpThreshold
		0.3,                               // memoryScaleDownThreshold
		0.8,                               // cpuScaleUpThreshold
		0.3,                               // cpuScaleDownThreshold
	)

	// Verify updates
	if cfg.CPURequestMultiplier != 1.5 {
		t.Errorf("Expected CPURequestMultiplier to be 1.5, got %f", cfg.CPURequestMultiplier)
	}

	if cfg.MemoryRequestMultiplier != 1.3 {
		t.Errorf("Expected MemoryRequestMultiplier to be 1.3, got %f", cfg.MemoryRequestMultiplier)
	}

	if cfg.CPURequestAddition != 100 {
		t.Errorf("Expected CPURequestAddition to be 100, got %d", cfg.CPURequestAddition)
	}

	if cfg.ResizeInterval != 60*time.Second {
		t.Errorf("Expected ResizeInterval to be 60s, got %v", cfg.ResizeInterval)
	}

	if !cfg.DryRun {
		t.Error("Expected DryRun to be true")
	}

	if cfg.LogLevel != "debug" {
		t.Errorf("Expected LogLevel to be 'debug', got %s", cfg.LogLevel)
	}

	if cfg.MetricsProvider != "prometheus" {
		t.Errorf("Expected MetricsProvider to be 'prometheus', got %s", cfg.MetricsProvider)
	}

	if cfg.PrometheusURL != "http://prom:9090" {
		t.Errorf("Expected PrometheusURL to be 'http://prom:9090', got %s", cfg.PrometheusURL)
	}

	if !cfg.EnableInPlaceResize {
		t.Error("Expected EnableInPlaceResize to be true")
	}

	if cfg.ConfigSource != "crd" {
		t.Errorf("Expected ConfigSource to be 'crd', got %s", cfg.ConfigSource)
	}

	if len(cfg.NamespaceInclude) != 2 {
		t.Errorf("Expected 2 included namespaces, got %d", len(cfg.NamespaceInclude))
	}

	if len(cfg.NamespaceExclude) != 1 {
		t.Errorf("Expected 1 excluded namespace, got %d", len(cfg.NamespaceExclude))
	}
}

func TestResetToDefaults(t *testing.T) {
	cfg := GetDefaults()

	// Modify configuration
	cfg.CPURequestMultiplier = 2.0
	cfg.LogLevel = "debug"
	cfg.ConfigSource = "crd"

	// Reset
	cfg.ResetToDefaults()

	// Verify reset
	if cfg.CPURequestMultiplier != 1.2 {
		t.Errorf("Expected CPURequestMultiplier to be reset to 1.2, got %f", cfg.CPURequestMultiplier)
	}

	if cfg.LogLevel != "info" {
		t.Errorf("Expected LogLevel to be reset to 'info', got %s", cfg.LogLevel)
	}

	if cfg.ConfigSource != "default" {
		t.Errorf("Expected ConfigSource to be reset to 'default', got %s", cfg.ConfigSource)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		wantError bool
	}{
		{
			name:      "valid config",
			config:    GetDefaults(),
			wantError: false,
		},
		{
			name: "invalid CPU multiplier",
			config: &Config{
				CPURequestMultiplier:    -1,
				MemoryRequestMultiplier: 1.2,
				MinCPURequest:           10,
				MinMemoryRequest:        64,
				MaxCPULimit:             4000,
				MaxMemoryLimit:          8192,
				ResizeInterval:          30 * time.Second,
				RetryInterval:           5 * time.Second,
				LogLevel:                "info",
				MetricsPort:             9090,
				SafetyThreshold:         0.5,
				HistoryDays:             7,
				MetricsProvider:         "metrics-server",
			},
			wantError: true,
		},
		{
			name: "invalid log level",
			config: &Config{
				CPURequestMultiplier:    1.2,
				MemoryRequestMultiplier: 1.2,
				CPULimitMultiplier:      2.0,
				MemoryLimitMultiplier:   2.0,
				MinCPURequest:           10,
				MinMemoryRequest:        64,
				MaxCPULimit:             4000,
				MaxMemoryLimit:          8192,
				ResizeInterval:          30 * time.Second,
				RetryInterval:           5 * time.Second,
				LogLevel:                "invalid",
				MetricsPort:             9090,
				SafetyThreshold:         0.5,
				HistoryDays:             7,
				MetricsProvider:         "metrics-server",
			},
			wantError: true,
		},
		{
			name: "invalid metrics provider",
			config: &Config{
				CPURequestMultiplier:    1.2,
				MemoryRequestMultiplier: 1.2,
				CPULimitMultiplier:      2.0,
				MemoryLimitMultiplier:   2.0,
				MinCPURequest:           10,
				MinMemoryRequest:        64,
				MaxCPULimit:             4000,
				MaxMemoryLimit:          8192,
				ResizeInterval:          30 * time.Second,
				RetryInterval:           5 * time.Second,
				LogLevel:                "info",
				MetricsPort:             9090,
				SafetyThreshold:         0.5,
				HistoryDays:             7,
				MetricsProvider:         "invalid-provider",
			},
			wantError: true,
		},
		{
			name: "min > max limits",
			config: &Config{
				CPURequestMultiplier:    1.2,
				MemoryRequestMultiplier: 1.2,
				CPULimitMultiplier:      2.0,
				MemoryLimitMultiplier:   2.0,
				MinCPURequest:           5000,
				MinMemoryRequest:        64,
				MaxCPULimit:             4000,
				MaxMemoryLimit:          8192,
				ResizeInterval:          30 * time.Second,
				RetryInterval:           5 * time.Second,
				LogLevel:                "info",
				MetricsPort:             9090,
				SafetyThreshold:         0.5,
				HistoryDays:             7,
				MetricsProvider:         "metrics-server",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestIsNamespaceIncluded(t *testing.T) {
	tests := []struct {
		name             string
		namespaceInclude []string
		namespaceExclude []string
		namespace        string
		expected         bool
	}{
		{
			name:             "no filters",
			namespaceInclude: []string{},
			namespaceExclude: []string{},
			namespace:        "default",
			expected:         true,
		},
		{
			name:             "included namespace",
			namespaceInclude: []string{"default", "production"},
			namespaceExclude: []string{},
			namespace:        "default",
			expected:         true,
		},
		{
			name:             "not included namespace",
			namespaceInclude: []string{"production"},
			namespaceExclude: []string{},
			namespace:        "default",
			expected:         false,
		},
		{
			name:             "excluded namespace",
			namespaceInclude: []string{},
			namespaceExclude: []string{"kube-system"},
			namespace:        "kube-system",
			expected:         false,
		},
		{
			name:             "included but excluded",
			namespaceInclude: []string{"kube-system"},
			namespaceExclude: []string{"kube-system"},
			namespace:        "kube-system",
			expected:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				NamespaceInclude: tt.namespaceInclude,
				NamespaceExclude: tt.namespaceExclude,
			}
			result := cfg.IsNamespaceIncluded(tt.namespace)
			if result != tt.expected {
				t.Errorf("IsNamespaceIncluded(%s) = %v, expected %v", tt.namespace, result, tt.expected)
			}
		})
	}
}

func TestIsChangeWithinSafetyThreshold(t *testing.T) {
	cfg := &Config{
		SafetyThreshold: 0.5, // 50% change threshold
	}

	tests := []struct {
		name     string
		current  int64
		new      int64
		expected bool
	}{
		{
			name:     "no existing resource",
			current:  0,
			new:      1000,
			expected: true,
		},
		{
			name:     "within threshold increase",
			current:  1000,
			new:      1400,
			expected: true,
		},
		{
			name:     "within threshold decrease",
			current:  1000,
			new:      600,
			expected: true,
		},
		{
			name:     "exceeds threshold increase",
			current:  1000,
			new:      1600,
			expected: false,
		},
		{
			name:     "exceeds threshold decrease",
			current:  1000,
			new:      400,
			expected: false,
		},
		{
			name:     "exactly at threshold",
			current:  1000,
			new:      1500,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cfg.IsChangeWithinSafetyThreshold(tt.current, tt.new)
			if result != tt.expected {
				t.Errorf("IsChangeWithinSafetyThreshold(%d, %d) = %v, expected %v",
					tt.current, tt.new, result, tt.expected)
			}
		})
	}
}

func TestClone(t *testing.T) {
	original := &Config{
		CPURequestMultiplier:    1.5,
		MemoryRequestMultiplier: 1.3,
		LogLevel:                "debug",
		NamespaceInclude:        []string{"default", "production"},
		NamespaceExclude:        []string{"kube-system"},
		CustomMetrics:           []string{"custom1", "custom2"},
		ConfigSource:            "crd",
	}

	clone := original.Clone()

	// Verify values are copied
	if clone.CPURequestMultiplier != original.CPURequestMultiplier {
		t.Error("CPURequestMultiplier not cloned correctly")
	}

	if clone.LogLevel != original.LogLevel {
		t.Error("LogLevel not cloned correctly")
	}

	if clone.ConfigSource != original.ConfigSource {
		t.Error("ConfigSource not cloned correctly")
	}

	// Verify slices are deep copied
	if len(clone.NamespaceInclude) != len(original.NamespaceInclude) {
		t.Error("NamespaceInclude not cloned correctly")
	}

	// Modify clone and verify original is unchanged
	clone.NamespaceInclude[0] = "modified"
	if original.NamespaceInclude[0] == "modified" {
		t.Error("Clone modified original NamespaceInclude slice")
	}

	clone.CustomMetrics[0] = "modified"
	if original.CustomMetrics[0] == "modified" {
		t.Error("Clone modified original CustomMetrics slice")
	}
}

func TestGetRetryConfig(t *testing.T) {
	cfg := &Config{
		MaxRetries:    5,
		RetryInterval: 10 * time.Second,
	}

	maxRetries, interval := cfg.GetRetryConfig()

	if maxRetries != 5 {
		t.Errorf("Expected MaxRetries to be 5, got %d", maxRetries)
	}

	if interval != 10*time.Second {
		t.Errorf("Expected RetryInterval to be 10s, got %v", interval)
	}
}

func TestGetSafeValue(t *testing.T) {
	cfg := &Config{
		CPURequestMultiplier: 1.5,
		LogLevel:             "debug",
	}

	// Test safe retrieval
	value := cfg.GetSafeValue(func(c *Config) interface{} {
		return c.CPURequestMultiplier
	})

	if v, ok := value.(float64); !ok || v != 1.5 {
		t.Errorf("GetSafeValue failed, expected 1.5, got %v", value)
	}

	// Test with string value
	strValue := cfg.GetSafeValue(func(c *Config) interface{} {
		return c.LogLevel
	})

	if v, ok := strValue.(string); !ok || v != "debug" {
		t.Errorf("GetSafeValue failed, expected 'debug', got %v", strValue)
	}
}

func TestThreadSafety(t *testing.T) {
	cfg := GetDefaults()

	// Run concurrent operations
	var wg sync.WaitGroup
	operations := 100

	// Concurrent updates
	for i := 0; i < operations; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cfg.UpdateFromCRD(
				float64(idx%3)+1.0, // cpuRequestMultiplier
				1.2,                // memoryRequestMultiplier
				int64(idx),         // cpuRequestAddition
				256,                // memoryRequestAddition
				2.0,                // cpuLimitMultiplier
				2.0,                // memoryLimitMultiplier
				0,                  // cpuLimitAddition
				0,                  // memoryLimitAddition
				10,                 // minCPURequest
				64,                 // minMemoryRequest
				4000,               // maxCPULimit
				8192,               // maxMemoryLimit
				30*time.Second,     // resizeInterval
				false,              // dryRun
				nil,                // namespaceInclude
				nil,                // namespaceExclude
				"info",             // logLevel
				true,               // metricsEnabled
				9090,               // metricsPort
				true,               // auditEnabled
				3,                  // maxRetries
				5*time.Second,      // retryInterval
				"metrics-server",   // metricsProvider
				"",                 // prometheusURL
				false,              // enableInPlaceResize
				10.0,               // qps
				20,                 // burst
				5,                  // maxConcurrentReconciles
				0.8,                // memoryScaleUpThreshold
				0.3,                // memoryScaleDownThreshold
				0.8,                // cpuScaleUpThreshold
				0.3,                // cpuScaleDownThreshold
			)
		}(i)
	}

	// Concurrent reads
	for i := 0; i < operations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cfg.IsNamespaceIncluded("default")
			_ = cfg.IsChangeWithinSafetyThreshold(1000, 1500)
			_, _ = cfg.GetRetryConfig()
			_ = cfg.Validate()
		}()
	}

	// Concurrent clones
	for i := 0; i < operations; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			clone := cfg.Clone()
			_ = clone.Validate()
		}()
	}

	wg.Wait()

	// If we get here without deadlock or panic, thread safety is working
	t.Log("Thread safety test completed successfully")
}

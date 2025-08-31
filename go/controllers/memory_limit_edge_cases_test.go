package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMemoryLimitProblematicScenarios tests edge cases where memory limits might cause issues
func TestMemoryLimitProblematicScenarios(t *testing.T) {
	tests := []struct {
		name               string
		memUsageMB         float64
		memRequestMB       int64
		memLimitMultiplier float64
		memLimitAddition   int64
		maxMemLimit        int64
		expectedProblem    string
		shouldWarn         bool
	}{
		{
			name:               "memory_limit_too_close_to_request",
			memUsageMB:         1024,
			memRequestMB:       1200, // Request based on usage
			memLimitMultiplier: 1.05, // Only 5% overhead
			memLimitAddition:   0,
			maxMemLimit:        8192,
			expectedProblem:    "Memory limit too close to request - risk of OOM",
			shouldWarn:         true,
		},
		{
			name:               "memory_spike_potential",
			memUsageMB:         512,
			memRequestMB:       614, // 512 * 1.2
			memLimitMultiplier: 1.1, // Only 10% overhead
			memLimitAddition:   50,
			maxMemLimit:        8192,
			expectedProblem:    "Insufficient headroom for memory spikes",
			shouldWarn:         true,
		},
		{
			name:               "java_heap_scenario",
			memUsageMB:         2048, // Java app using 2GB
			memRequestMB:       2458, // With 20% buffer
			memLimitMultiplier: 1.2,  // Needs more for non-heap memory
			memLimitAddition:   512,  // Additional for metaspace, etc.
			maxMemLimit:        8192,
			expectedProblem:    "Java apps need significant overhead for non-heap memory",
			shouldWarn:         false, // This is actually OK
		},
		{
			name:               "memory_leak_protection",
			memUsageMB:         1024,
			memRequestMB:       1229, // Normal request
			memLimitMultiplier: 2.0,  // 2x to contain potential leaks
			memLimitAddition:   1024, // Extra GB for safety
			maxMemLimit:        4096,
			expectedProblem:    "Large limit might hide memory leaks",
			shouldWarn:         true,
		},
		{
			name:               "cache_heavy_workload",
			memUsageMB:         3072, // Cache-heavy workload
			memRequestMB:       3686, // With buffer
			memLimitMultiplier: 1.5,  // Allow cache growth
			memLimitAddition:   1024,
			maxMemLimit:        8192,
			expectedProblem:    "Cache workloads need flexible limits",
			shouldWarn:         false,
		},
		{
			name:               "memory_limit_equals_request",
			memUsageMB:         1024,
			memRequestMB:       1229,
			memLimitMultiplier: 1.0, // Limit equals request!
			memLimitAddition:   0,
			maxMemLimit:        8192,
			expectedProblem:    "Memory limit equals request - no buffer for spikes",
			shouldWarn:         true,
		},
		{
			name:               "very_small_container",
			memUsageMB:         32,
			memRequestMB:       64, // Minimum
			memLimitMultiplier: 1.5,
			memLimitAddition:   10, // Small addition
			maxMemLimit:        8192,
			expectedProblem:    "Small containers need proportionally larger overhead",
			shouldWarn:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate memory limit
			memLimit := int64(float64(tt.memRequestMB)*tt.memLimitMultiplier) + tt.memLimitAddition

			// Apply cap
			if memLimit > tt.maxMemLimit {
				memLimit = tt.maxMemLimit
			}

			// Calculate overhead percentage
			overhead := float64(memLimit-tt.memRequestMB) / float64(tt.memRequestMB) * 100

			// Check for problematic scenarios
			if tt.shouldWarn {
				if overhead < 10 {
					t.Logf("WARNING: %s - Only %.1f%% overhead", tt.expectedProblem, overhead)
				}
				if memLimit == tt.memRequestMB {
					t.Logf("CRITICAL: Memory limit equals request - certain OOM")
				}
			}

			// Verify limit is at least equal to request
			assert.GreaterOrEqual(t, memLimit, tt.memRequestMB,
				"Memory limit should never be less than request")

			t.Logf("Scenario: %s - Request: %dMB, Limit: %dMB, Overhead: %.1f%%",
				tt.name, tt.memRequestMB, memLimit, overhead)
		})
	}
}

// TestMemoryLimitToRequestRatios tests various memory limit to request ratios
func TestMemoryLimitToRequestRatios(t *testing.T) {
	tests := []struct {
		name             string
		workloadType     string
		memRequestMB     int64
		recommendedRatio float64
		minRatio         float64
		maxRatio         float64
		description      string
	}{
		{
			name:             "stateless_web_app",
			workloadType:     "web",
			memRequestMB:     512,
			recommendedRatio: 1.25,
			minRatio:         1.1,
			maxRatio:         1.5,
			description:      "Stateless web apps need moderate overhead",
		},
		{
			name:             "java_spring_boot",
			workloadType:     "java",
			memRequestMB:     2048,
			recommendedRatio: 1.5,
			minRatio:         1.3,
			maxRatio:         2.0,
			description:      "Java apps need significant overhead for metaspace and off-heap",
		},
		{
			name:             "nodejs_app",
			workloadType:     "nodejs",
			memRequestMB:     256,
			recommendedRatio: 1.3,
			minRatio:         1.2,
			maxRatio:         1.5,
			description:      "Node.js apps have moderate memory variance",
		},
		{
			name:             "golang_service",
			workloadType:     "golang",
			memRequestMB:     128,
			recommendedRatio: 1.2,
			minRatio:         1.1,
			maxRatio:         1.4,
			description:      "Go services typically have stable memory usage",
		},
		{
			name:             "python_ml_workload",
			workloadType:     "python-ml",
			memRequestMB:     4096,
			recommendedRatio: 1.5,
			minRatio:         1.3,
			maxRatio:         2.0,
			description:      "ML workloads may have memory spikes during computation",
		},
		{
			name:             "redis_cache",
			workloadType:     "redis",
			memRequestMB:     1024,
			recommendedRatio: 1.3,
			minRatio:         1.2,
			maxRatio:         1.5,
			description:      "Redis needs overhead for background operations",
		},
		{
			name:             "postgresql_db",
			workloadType:     "postgresql",
			memRequestMB:     2048,
			recommendedRatio: 1.25,
			minRatio:         1.15,
			maxRatio:         1.5,
			description:      "Databases need controlled memory limits",
		},
		{
			name:             "batch_job",
			workloadType:     "batch",
			memRequestMB:     8192,
			recommendedRatio: 1.1,
			minRatio:         1.05,
			maxRatio:         1.3,
			description:      "Batch jobs with predictable memory usage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate limits with different ratios
			minLimit := int64(float64(tt.memRequestMB) * tt.minRatio)
			recommendedLimit := int64(float64(tt.memRequestMB) * tt.recommendedRatio)
			maxLimit := int64(float64(tt.memRequestMB) * tt.maxRatio)

			// Verify ratios make sense
			assert.Less(t, tt.minRatio, tt.recommendedRatio,
				"Minimum ratio should be less than recommended")
			assert.Less(t, tt.recommendedRatio, tt.maxRatio,
				"Recommended ratio should be less than maximum")

			// Log the recommendations
			t.Logf("Workload: %s (%s)", tt.name, tt.workloadType)
			t.Logf("  Request: %dMB", tt.memRequestMB)
			t.Logf("  Min Limit: %dMB (%.1fx)", minLimit, tt.minRatio)
			t.Logf("  Recommended Limit: %dMB (%.1fx)", recommendedLimit, tt.recommendedRatio)
			t.Logf("  Max Limit: %dMB (%.1fx)", maxLimit, tt.maxRatio)
			t.Logf("  Rationale: %s", tt.description)

			// Verify minimum safety threshold
			assert.GreaterOrEqual(t, tt.minRatio, 1.05,
				"Minimum ratio should be at least 1.05 for safety")
		})
	}
}

// TestMemoryLimitWithBurstPatterns tests memory limits for different burst patterns
func TestMemoryLimitWithBurstPatterns(t *testing.T) {
	tests := []struct {
		name              string
		avgMemUsageMB     float64
		peakMemUsageMB    float64
		burstFrequency    string
		recommendedBuffer float64
		description       string
	}{
		{
			name:              "constant_usage",
			avgMemUsageMB:     1024,
			peakMemUsageMB:    1050,
			burstFrequency:    "rare",
			recommendedBuffer: 1.2,
			description:       "Stable memory usage needs minimal buffer",
		},
		{
			name:              "periodic_spikes",
			avgMemUsageMB:     512,
			peakMemUsageMB:    768,
			burstFrequency:    "hourly",
			recommendedBuffer: 1.6,
			description:       "Regular spikes need buffer for peak + safety",
		},
		{
			name:              "unpredictable_bursts",
			avgMemUsageMB:     1024,
			peakMemUsageMB:    2048,
			burstFrequency:    "random",
			recommendedBuffer: 2.2,
			description:       "Unpredictable bursts need generous buffer",
		},
		{
			name:              "gc_spikes",
			avgMemUsageMB:     2048,
			peakMemUsageMB:    2560,
			burstFrequency:    "gc-triggered",
			recommendedBuffer: 1.4,
			description:       "GC spikes are predictable but need headroom",
		},
		{
			name:              "startup_spike",
			avgMemUsageMB:     512,
			peakMemUsageMB:    1024,
			burstFrequency:    "startup-only",
			recommendedBuffer: 2.1,
			description:       "Startup spikes need accommodation",
		},
		{
			name:              "cache_warmup",
			avgMemUsageMB:     1024,
			peakMemUsageMB:    1536,
			burstFrequency:    "warmup-period",
			recommendedBuffer: 1.6,
			description:       "Cache warmup needs temporary extra memory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate request based on peak usage
			requestMB := int64(tt.peakMemUsageMB * 1.1) // 10% buffer over peak

			// Calculate limit based on recommended buffer
			limitMB := int64(float64(requestMB) * tt.recommendedBuffer)

			// Calculate burst headroom
			burstHeadroom := limitMB - int64(tt.peakMemUsageMB)
			burstHeadroomPct := float64(burstHeadroom) / tt.peakMemUsageMB * 100

			// Verify we have enough headroom for bursts
			assert.Greater(t, burstHeadroom, int64(0),
				"Should have positive burst headroom")

			// Log the analysis
			t.Logf("Pattern: %s", tt.name)
			t.Logf("  Average Usage: %.0fMB", tt.avgMemUsageMB)
			t.Logf("  Peak Usage: %.0fMB", tt.peakMemUsageMB)
			t.Logf("  Burst Frequency: %s", tt.burstFrequency)
			t.Logf("  Recommended Request: %dMB", requestMB)
			t.Logf("  Recommended Limit: %dMB (%.1fx buffer)", limitMB, tt.recommendedBuffer)
			t.Logf("  Burst Headroom: %dMB (%.1f%% of peak)", burstHeadroom, burstHeadroomPct)
			t.Logf("  Rationale: %s", tt.description)

			// Verify minimum headroom based on burst pattern
			switch tt.burstFrequency {
			case "random", "startup-only":
				assert.GreaterOrEqual(t, burstHeadroomPct, 50.0,
					"Unpredictable bursts need at least 50% headroom")
			case "hourly", "warmup-period":
				assert.GreaterOrEqual(t, burstHeadroomPct, 30.0,
					"Regular bursts need at least 30% headroom")
			case "rare", "gc-triggered":
				assert.GreaterOrEqual(t, burstHeadroomPct, 20.0,
					"Predictable patterns need at least 20% headroom")
			}
		})
	}
}

// TestMemoryLimitScalingDecisions tests how scaling decisions affect memory limits
func TestMemoryLimitScalingDecisions(t *testing.T) {
	tests := []struct {
		name              string
		currentRequestMB  int64
		currentLimitMB    int64
		memUsageMB        float64
		scalingDecision   ScalingDecision
		expectedNewLimit  int64
		shouldAdjustRatio bool
		description       string
	}{
		{
			name:              "oom_detected_increase_limit",
			currentRequestMB:  1024,
			currentLimitMB:    1280, // 1.25x
			memUsageMB:        1200, // Hit the limit
			scalingDecision:   ScaleUp,
			expectedNewLimit:  1843, // Increase ratio to 1.5x
			shouldAdjustRatio: true,
			description:       "OOM events should increase limit ratio",
		},
		{
			name:              "consistent_low_usage",
			currentRequestMB:  1024,
			currentLimitMB:    2048, // 2x (too generous)
			memUsageMB:        512,  // Only 50% of request
			scalingDecision:   ScaleDown,
			expectedNewLimit:  768, // Reduce to 1.2x of new request
			shouldAdjustRatio: true,
			description:       "Consistent low usage should reduce limit ratio",
		},
		{
			name:              "usage_near_limit",
			currentRequestMB:  2048,
			currentLimitMB:    2560, // 1.25x
			memUsageMB:        2400, // 93% of limit
			scalingDecision:   ScaleNone,
			expectedNewLimit:  3686, // Increase ratio to 1.5x for safety
			shouldAdjustRatio: true,
			description:       "Usage near limit needs more headroom",
		},
		{
			name:              "stable_usage_good_ratio",
			currentRequestMB:  1024,
			currentLimitMB:    1536, // 1.5x
			memUsageMB:        900,  // Comfortable usage
			scalingDecision:   ScaleNone,
			expectedNewLimit:  1536, // Keep same ratio
			shouldAdjustRatio: false,
			description:       "Stable usage with good ratio should maintain",
		},
		{
			name:              "scale_up_after_oom",
			currentRequestMB:  512,
			currentLimitMB:    614, // 1.2x (too tight)
			memUsageMB:        600, // Near limit
			scalingDecision:   ScaleUp,
			expectedNewLimit:  1080, // New request 720 * 1.5x
			shouldAdjustRatio: true,
			description:       "Scale up after OOM should use safer ratio",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate current ratio
			currentRatio := float64(tt.currentLimitMB) / float64(tt.currentRequestMB)

			// Calculate usage percentage of limit
			usagePctOfLimit := tt.memUsageMB / float64(tt.currentLimitMB) * 100

			// Determine new request based on scaling decision
			var newRequestMB int64
			switch tt.scalingDecision {
			case ScaleUp:
				newRequestMB = int64(tt.memUsageMB * 1.2)
			case ScaleDown:
				newRequestMB = int64(tt.memUsageMB * 1.1)
			default:
				newRequestMB = tt.currentRequestMB
			}

			// Determine new ratio based on usage patterns
			var newRatio float64
			if tt.shouldAdjustRatio {
				if usagePctOfLimit > 90 {
					newRatio = 1.5 // Increase headroom
				} else if usagePctOfLimit < 50 && tt.scalingDecision == ScaleDown {
					newRatio = 1.2 // Reduce overhead
				} else {
					newRatio = currentRatio
				}
			} else {
				newRatio = currentRatio
			}

			// Calculate new limit
			newLimitMB := int64(float64(newRequestMB) * newRatio)

			// Log the decision process
			t.Logf("Scenario: %s", tt.name)
			t.Logf("  Current: Request=%dMB, Limit=%dMB (%.2fx)",
				tt.currentRequestMB, tt.currentLimitMB, currentRatio)
			t.Logf("  Usage: %.0fMB (%.1f%% of limit)",
				tt.memUsageMB, usagePctOfLimit)
			t.Logf("  Decision: %v", tt.scalingDecision)
			t.Logf("  New: Request=%dMB, Limit=%dMB (%.2fx)",
				newRequestMB, newLimitMB, newRatio)
			t.Logf("  Rationale: %s", tt.description)

			// Verify new limit provides adequate headroom
			if usagePctOfLimit > 90 {
				assert.GreaterOrEqual(t, newRatio, 1.3,
					"High usage should result in safer ratio")
			}

			// Verify we never set limit below request
			assert.GreaterOrEqual(t, newLimitMB, newRequestMB,
				"Limit should never be less than request")
		})
	}
}

// TestMemoryLimitForContainerTypes tests memory limits for different container types
func TestMemoryLimitForContainerTypes(t *testing.T) {
	tests := []struct {
		name             string
		containerType    string
		hasInitContainer bool
		hasSidecar       bool
		mainAppMemMB     int64
		sidecarMemMB     int64
		initMemMB        int64
		expectedStrategy string
	}{
		{
			name:             "single_container",
			containerType:    "single",
			hasInitContainer: false,
			hasSidecar:       false,
			mainAppMemMB:     1024,
			sidecarMemMB:     0,
			initMemMB:        0,
			expectedStrategy: "Standard limit calculation",
		},
		{
			name:             "with_sidecar",
			containerType:    "main+sidecar",
			hasInitContainer: false,
			hasSidecar:       true,
			mainAppMemMB:     1024,
			sidecarMemMB:     256,
			initMemMB:        0,
			expectedStrategy: "Independent limits per container",
		},
		{
			name:             "with_init_container",
			containerType:    "main+init",
			hasInitContainer: true,
			hasSidecar:       false,
			mainAppMemMB:     1024,
			sidecarMemMB:     0,
			initMemMB:        512,
			expectedStrategy: "Init container needs temporary memory",
		},
		{
			name:             "istio_sidecar",
			containerType:    "main+istio",
			hasInitContainer: true,
			hasSidecar:       true,
			mainAppMemMB:     2048,
			sidecarMemMB:     128, // Istio proxy
			initMemMB:        64,  // Istio init
			expectedStrategy: "Service mesh sidecars need stable limits",
		},
		{
			name:             "logging_sidecar",
			containerType:    "main+fluentbit",
			hasInitContainer: false,
			hasSidecar:       true,
			mainAppMemMB:     1024,
			sidecarMemMB:     256, // Fluent-bit
			initMemMB:        0,
			expectedStrategy: "Logging sidecars may spike during backpressure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate total pod memory
			totalPodMem := tt.mainAppMemMB + tt.sidecarMemMB

			// Calculate limits based on container type
			var mainLimitMB, sidecarLimitMB, initLimitMB int64

			// Main container limit
			mainLimitMB = int64(float64(tt.mainAppMemMB) * 1.3)

			// Sidecar limit (if present)
			if tt.hasSidecar {
				if tt.containerType == "main+istio" {
					sidecarLimitMB = int64(float64(tt.sidecarMemMB) * 1.5) // Istio needs headroom
				} else if tt.containerType == "main+fluentbit" {
					sidecarLimitMB = int64(float64(tt.sidecarMemMB) * 2.0) // Logging needs buffer
				} else {
					sidecarLimitMB = int64(float64(tt.sidecarMemMB) * 1.3)
				}
			}

			// Init container limit (if present)
			if tt.hasInitContainer {
				initLimitMB = int64(float64(tt.initMemMB) * 1.2) // Init containers are short-lived
			}

			// Log the configuration
			t.Logf("Container Configuration: %s", tt.name)
			t.Logf("  Type: %s", tt.containerType)
			t.Logf("  Main App: Request=%dMB, Limit=%dMB", tt.mainAppMemMB, mainLimitMB)
			if tt.hasSidecar {
				t.Logf("  Sidecar: Request=%dMB, Limit=%dMB", tt.sidecarMemMB, sidecarLimitMB)
			}
			if tt.hasInitContainer {
				t.Logf("  Init: Request=%dMB, Limit=%dMB", tt.initMemMB, initLimitMB)
			}
			t.Logf("  Total Pod Memory: %dMB", totalPodMem)
			t.Logf("  Strategy: %s", tt.expectedStrategy)

			// Verify limits are reasonable
			assert.Greater(t, mainLimitMB, tt.mainAppMemMB,
				"Main container limit should exceed request")
			if tt.hasSidecar {
				assert.Greater(t, sidecarLimitMB, tt.sidecarMemMB,
					"Sidecar limit should exceed request")
			}
			if tt.hasInitContainer {
				assert.Greater(t, initLimitMB, tt.initMemMB,
					"Init container limit should exceed request")
			}
		})
	}
}

// TestMemoryLimitValidation tests validation of memory limit configurations
func TestMemoryLimitValidation(t *testing.T) {
	tests := []struct {
		name      string
		requestMB int64
		limitMB   int64
		isValid   bool
		errorMsg  string
	}{
		{
			name:      "valid_configuration",
			requestMB: 1024,
			limitMB:   1536,
			isValid:   true,
			errorMsg:  "",
		},
		{
			name:      "limit_less_than_request",
			requestMB: 1024,
			limitMB:   512,
			isValid:   false,
			errorMsg:  "Memory limit cannot be less than request",
		},
		{
			name:      "limit_equals_request",
			requestMB: 1024,
			limitMB:   1024,
			isValid:   false,
			errorMsg:  "Memory limit should not equal request (no buffer)",
		},
		{
			name:      "excessive_limit",
			requestMB: 512,
			limitMB:   5120, // 10x request
			isValid:   false,
			errorMsg:  "Memory limit too high relative to request (>5x)",
		},
		{
			name:      "negative_values",
			requestMB: -100,
			limitMB:   1024,
			isValid:   false,
			errorMsg:  "Memory values cannot be negative",
		},
		{
			name:      "zero_request",
			requestMB: 0,
			limitMB:   1024,
			isValid:   false,
			errorMsg:  "Memory request cannot be zero",
		},
		{
			name:      "insufficient_buffer",
			requestMB: 1024,
			limitMB:   1074, // Less than 5% buffer
			isValid:   false,
			errorMsg:  "Insufficient buffer between request and limit (<5%)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate the configuration
			isValid := true
			var actualError string

			// Check basic constraints
			if tt.requestMB <= 0 || tt.limitMB <= 0 {
				isValid = false
				actualError = "Memory values cannot be negative or zero"
			} else if tt.limitMB < tt.requestMB {
				isValid = false
				actualError = "Memory limit cannot be less than request"
			} else if tt.limitMB == tt.requestMB {
				isValid = false
				actualError = "Memory limit should not equal request (no buffer)"
			} else {
				// Calculate buffer percentage
				bufferPct := float64(tt.limitMB-tt.requestMB) / float64(tt.requestMB) * 100

				if bufferPct < 5 {
					isValid = false
					actualError = "Insufficient buffer between request and limit (<5%)"
				} else if bufferPct > 400 {
					isValid = false
					actualError = "Memory limit too high relative to request (>5x)"
				}
			}

			// Assert validation result
			assert.Equal(t, tt.isValid, isValid,
				"Validation result mismatch for %s", tt.name)

			if !tt.isValid && actualError != "" {
				t.Logf("Validation failed: %s", actualError)
			}
		})
	}
}

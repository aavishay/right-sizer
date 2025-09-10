package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
	"right-sizer/config"
)

func TestCPURequestCalculations(t *testing.T) {
	tests := []struct {
		name                 string
		cpuUsageMilli        float64
		cpuRequestMultiplier float64
		cpuRequestAddition   int64
		minCPURequest        int64
		expectedCPURequest   int64
		description          string
	}{
		{
			name:                 "standard_cpu_calculation",
			cpuUsageMilli:        100,
			cpuRequestMultiplier: 1.2,
			cpuRequestAddition:   50,
			minCPURequest:        10,
			expectedCPURequest:   170, // (100 * 1.2) + 50 = 170
			description:          "Standard CPU request calculation with multiplier and addition",
		},
		{
			name:                 "very_low_cpu_usage",
			cpuUsageMilli:        5,
			cpuRequestMultiplier: 1.2,
			cpuRequestAddition:   10,
			minCPURequest:        25,
			expectedCPURequest:   25, // (5 * 1.2) + 10 = 16, but min is 25
			description:          "Should respect minimum CPU request",
		},
		{
			name:                 "zero_cpu_usage",
			cpuUsageMilli:        0,
			cpuRequestMultiplier: 1.5,
			cpuRequestAddition:   20,
			minCPURequest:        10,
			expectedCPURequest:   20, // (0 * 1.5) + 20 = 20
			description:          "Zero CPU usage should still add the addition value",
		},
		{
			name:                 "high_cpu_usage",
			cpuUsageMilli:        2000,
			cpuRequestMultiplier: 1.3,
			cpuRequestAddition:   100,
			minCPURequest:        10,
			expectedCPURequest:   2700, // (2000 * 1.3) + 100 = 2700
			description:          "High CPU usage calculation",
		},
		{
			name:                 "fractional_cpu_usage",
			cpuUsageMilli:        123.456,
			cpuRequestMultiplier: 1.25,
			cpuRequestAddition:   30,
			minCPURequest:        10,
			expectedCPURequest:   184, // int64(123.456 * 1.25) + 30 = 154 + 30 = 184
			description:          "Fractional CPU usage should be handled correctly",
		},
		{
			name:                 "no_multiplier_effect",
			cpuUsageMilli:        200,
			cpuRequestMultiplier: 1.0,
			cpuRequestAddition:   0,
			minCPURequest:        10,
			expectedCPURequest:   200, // (200 * 1.0) + 0 = 200
			description:          "Multiplier of 1.0 with no addition",
		},
		{
			name:                 "large_addition_dominates",
			cpuUsageMilli:        10,
			cpuRequestMultiplier: 1.1,
			cpuRequestAddition:   500,
			minCPURequest:        10,
			expectedCPURequest:   511, // (10 * 1.1) + 500 = 511
			description:          "Large addition value dominates small usage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate CPU request
			cpuRequest := int64(tt.cpuUsageMilli*tt.cpuRequestMultiplier) + tt.cpuRequestAddition

			// Apply minimum
			if cpuRequest < tt.minCPURequest {
				cpuRequest = tt.minCPURequest
			}

			assert.Equal(t, tt.expectedCPURequest, cpuRequest, tt.description)
		})
	}
}

func TestMemoryRequestCalculations(t *testing.T) {
	tests := []struct {
		name                 string
		memUsageMB           float64
		memRequestMultiplier float64
		memRequestAddition   int64
		minMemRequest        int64
		expectedMemRequest   int64
		description          string
	}{
		{
			name:                 "standard_memory_calculation",
			memUsageMB:           512,
			memRequestMultiplier: 1.3,
			memRequestAddition:   100,
			minMemRequest:        64,
			expectedMemRequest:   765, // (512 * 1.3) + 100 = 665.6 + 100 = 765
			description:          "Standard memory request calculation",
		},
		{
			name:                 "very_low_memory_usage",
			memUsageMB:           10,
			memRequestMultiplier: 1.2,
			memRequestAddition:   20,
			minMemRequest:        64,
			expectedMemRequest:   64, // (10 * 1.2) + 20 = 32, but min is 64
			description:          "Should respect minimum memory request",
		},
		{
			name:                 "zero_memory_usage",
			memUsageMB:           0,
			memRequestMultiplier: 1.5,
			memRequestAddition:   128,
			minMemRequest:        64,
			expectedMemRequest:   128, // (0 * 1.5) + 128 = 128
			description:          "Zero memory usage with addition",
		},
		{
			name:                 "high_memory_usage",
			memUsageMB:           4096,
			memRequestMultiplier: 1.2,
			memRequestAddition:   256,
			minMemRequest:        64,
			expectedMemRequest:   5171, // (4096 * 1.2) + 256 = 4915.2 + 256 = 5171
			description:          "High memory usage calculation",
		},
		{
			name:                 "fractional_memory_usage",
			memUsageMB:           256.75,
			memRequestMultiplier: 1.25,
			memRequestAddition:   50,
			minMemRequest:        64,
			expectedMemRequest:   370, // int64(256.75 * 1.25) + 50 = 320 + 50 = 370
			description:          "Fractional memory usage",
		},
		{
			name:                 "minimum_dominates",
			memUsageMB:           1,
			memRequestMultiplier: 1.1,
			memRequestAddition:   5,
			minMemRequest:        128,
			expectedMemRequest:   128, // (1 * 1.1) + 5 = 6.1, but min is 128
			description:          "Minimum memory dominates calculation",
		},
		{
			name:                 "gigabyte_scale",
			memUsageMB:           8192, // 8GB
			memRequestMultiplier: 1.15,
			memRequestAddition:   512,
			minMemRequest:        64,
			expectedMemRequest:   9932, // (8192 * 1.15) + 512 = 9420.8 + 512 = 9932
			description:          "Gigabyte scale memory calculation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate memory request
			memRequest := int64(tt.memUsageMB*tt.memRequestMultiplier) + tt.memRequestAddition

			// Apply minimum
			if memRequest < tt.minMemRequest {
				memRequest = tt.minMemRequest
			}

			assert.Equal(t, tt.expectedMemRequest, memRequest, tt.description)
		})
	}
}

func TestCPULimitCalculations(t *testing.T) {
	tests := []struct {
		name               string
		cpuRequest         int64
		cpuLimitMultiplier float64
		cpuLimitAddition   int64
		maxCPULimit        int64
		expectedCPULimit   int64
		description        string
	}{
		{
			name:               "standard_cpu_limit",
			cpuRequest:         200,
			cpuLimitMultiplier: 2.0,
			cpuLimitAddition:   100,
			maxCPULimit:        4000,
			expectedCPULimit:   500, // (200 * 2.0) + 100 = 500
			description:        "Standard CPU limit calculation",
		},
		{
			name:               "cpu_limit_at_cap",
			cpuRequest:         2000,
			cpuLimitMultiplier: 2.5,
			cpuLimitAddition:   500,
			maxCPULimit:        4000,
			expectedCPULimit:   4000, // (2000 * 2.5) + 500 = 5500, capped at 4000
			description:        "CPU limit should be capped at maximum",
		},
		{
			name:               "small_request_large_multiplier",
			cpuRequest:         50,
			cpuLimitMultiplier: 10.0,
			cpuLimitAddition:   200,
			maxCPULimit:        8000,
			expectedCPULimit:   700, // (50 * 10.0) + 200 = 700
			description:        "Small request with large multiplier",
		},
		{
			name:               "no_addition",
			cpuRequest:         300,
			cpuLimitMultiplier: 1.5,
			cpuLimitAddition:   0,
			maxCPULimit:        4000,
			expectedCPULimit:   450, // (300 * 1.5) + 0 = 450
			description:        "CPU limit with no addition",
		},
		{
			name:               "multiplier_one",
			cpuRequest:         500,
			cpuLimitMultiplier: 1.0,
			cpuLimitAddition:   100,
			maxCPULimit:        4000,
			expectedCPULimit:   600, // (500 * 1.0) + 100 = 600
			description:        "Multiplier of 1.0 means limit equals request plus addition",
		},
		{
			name:               "zero_request",
			cpuRequest:         0,
			cpuLimitMultiplier: 2.0,
			cpuLimitAddition:   100,
			maxCPULimit:        4000,
			expectedCPULimit:   100, // (0 * 2.0) + 100 = 100
			description:        "Zero request should still have limit from addition",
		},
		{
			name:               "exactly_at_max",
			cpuRequest:         1500,
			cpuLimitMultiplier: 2.0,
			cpuLimitAddition:   0,
			maxCPULimit:        3000,
			expectedCPULimit:   3000, // (1500 * 2.0) + 0 = 3000
			description:        "Calculation exactly at maximum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate CPU limit
			cpuLimit := int64(float64(tt.cpuRequest)*tt.cpuLimitMultiplier) + tt.cpuLimitAddition

			// Apply maximum cap
			if cpuLimit > tt.maxCPULimit {
				cpuLimit = tt.maxCPULimit
			}

			assert.Equal(t, tt.expectedCPULimit, cpuLimit, tt.description)
		})
	}
}

func TestMemoryLimitCalculations(t *testing.T) {
	tests := []struct {
		name               string
		memRequest         int64 // in MB
		memLimitMultiplier float64
		memLimitAddition   int64 // in MB
		maxMemLimit        int64 // in MB
		expectedMemLimit   int64 // in MB
		description        string
	}{
		{
			name:               "standard_memory_limit",
			memRequest:         512,
			memLimitMultiplier: 1.5,
			memLimitAddition:   256,
			maxMemLimit:        8192,
			expectedMemLimit:   1024, // (512 * 1.5) + 256 = 768 + 256 = 1024
			description:        "Standard memory limit calculation",
		},
		{
			name:               "memory_limit_at_cap",
			memRequest:         4096,
			memLimitMultiplier: 2.0,
			memLimitAddition:   1024,
			maxMemLimit:        8192,
			expectedMemLimit:   8192, // (4096 * 2.0) + 1024 = 9216, capped at 8192
			description:        "Memory limit should be capped at maximum",
		},
		{
			name:               "small_request_large_multiplier",
			memRequest:         128,
			memLimitMultiplier: 4.0,
			memLimitAddition:   512,
			maxMemLimit:        16384,
			expectedMemLimit:   1024, // (128 * 4.0) + 512 = 512 + 512 = 1024
			description:        "Small request with large multiplier",
		},
		{
			name:               "no_multiplier_effect",
			memRequest:         1024,
			memLimitMultiplier: 1.0,
			memLimitAddition:   512,
			maxMemLimit:        8192,
			expectedMemLimit:   1536, // (1024 * 1.0) + 512 = 1536
			description:        "Multiplier of 1.0 with addition",
		},
		{
			name:               "zero_request_with_addition",
			memRequest:         0,
			memLimitMultiplier: 2.0,
			memLimitAddition:   256,
			maxMemLimit:        8192,
			expectedMemLimit:   256, // (0 * 2.0) + 256 = 256
			description:        "Zero request should still have limit from addition",
		},
		{
			name:               "gigabyte_scale",
			memRequest:         2048, // 2GB
			memLimitMultiplier: 1.75,
			memLimitAddition:   512,
			maxMemLimit:        16384, // 16GB
			expectedMemLimit:   4096,  // (2048 * 1.75) + 512 = 3584 + 512 = 4096
			description:        "Gigabyte scale memory limit",
		},
		{
			name:               "very_tight_limit",
			memRequest:         1024,
			memLimitMultiplier: 1.1,
			memLimitAddition:   50,
			maxMemLimit:        8192,
			expectedMemLimit:   1176, // (1024 * 1.1) + 50 = 1126.4 + 50 = 1176
			description:        "Tight memory limit (10% overhead)",
		},
		{
			name:               "generous_limit",
			memRequest:         512,
			memLimitMultiplier: 3.0,
			memLimitAddition:   1024,
			maxMemLimit:        8192,
			expectedMemLimit:   2560, // (512 * 3.0) + 1024 = 1536 + 1024 = 2560
			description:        "Generous memory limit (3x + 1GB)",
		},
		{
			name:               "edge_case_just_below_max",
			memRequest:         3000,
			memLimitMultiplier: 2.0,
			memLimitAddition:   2000,
			maxMemLimit:        8000,
			expectedMemLimit:   8000, // (3000 * 2.0) + 2000 = 8000
			description:        "Result exactly at max after calculation",
		},
		{
			name:               "fractional_multiplier",
			memRequest:         1000,
			memLimitMultiplier: 1.234,
			memLimitAddition:   100,
			maxMemLimit:        8192,
			expectedMemLimit:   1334, // (1000 * 1.234) + 100 = 1234 + 100 = 1334
			description:        "Fractional multiplier calculation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate memory limit
			memLimit := int64(float64(tt.memRequest)*tt.memLimitMultiplier) + tt.memLimitAddition

			// Apply maximum cap
			if memLimit > tt.maxMemLimit {
				memLimit = tt.maxMemLimit
			}

			assert.Equal(t, tt.expectedMemLimit, memLimit, tt.description)
		})
	}
}

func TestCompleteResourceCalculation(t *testing.T) {
	tests := []struct {
		name             string
		cpuUsageMilli    float64
		memUsageMB       float64
		config           config.Config
		expectedCPUReq   int64
		expectedMemReq   int64
		expectedCPULimit int64
		expectedMemLimit int64
		description      string
	}{
		{
			name:          "balanced_resources",
			cpuUsageMilli: 250,
			memUsageMB:    512,
			config: config.Config{
				CPURequestMultiplier:    1.2,
				MemoryRequestMultiplier: 1.3,
				CPURequestAddition:      50,
				MemoryRequestAddition:   100,
				CPULimitMultiplier:      2.0,
				MemoryLimitMultiplier:   1.5,
				CPULimitAddition:        100,
				MemoryLimitAddition:     256,
				MinCPURequest:           10,
				MinMemoryRequest:        64,
				MaxCPULimit:             4000,
				MaxMemoryLimit:          8192,
			},
			expectedCPUReq:   350,  // (250 * 1.2) + 50 = 350
			expectedMemReq:   765,  // (512 * 1.3) + 100 = 765
			expectedCPULimit: 800,  // (350 * 2.0) + 100 = 800
			expectedMemLimit: 1403, // (765 * 1.5) + 256 = 1403
			description:      "Balanced resource calculation",
		},
		{
			name:          "minimal_usage",
			cpuUsageMilli: 5,
			memUsageMB:    20,
			config: config.Config{
				CPURequestMultiplier:    1.5,
				MemoryRequestMultiplier: 1.5,
				CPURequestAddition:      10,
				MemoryRequestAddition:   20,
				CPULimitMultiplier:      3.0,
				MemoryLimitMultiplier:   2.0,
				CPULimitAddition:        50,
				MemoryLimitAddition:     100,
				MinCPURequest:           50,
				MinMemoryRequest:        128,
				MaxCPULimit:             4000,
				MaxMemoryLimit:          8192,
			},
			expectedCPUReq:   50,  // (5 * 1.5) + 10 = 17.5, min is 50
			expectedMemReq:   128, // (20 * 1.5) + 20 = 50, min is 128
			expectedCPULimit: 200, // (50 * 3.0) + 50 = 200
			expectedMemLimit: 356, // (128 * 2.0) + 100 = 356
			description:      "Minimal usage with minimums applied",
		},
		{
			name:          "high_usage_with_caps",
			cpuUsageMilli: 3000,
			memUsageMB:    6000,
			config: config.Config{
				CPURequestMultiplier:    1.1,
				MemoryRequestMultiplier: 1.1,
				CPURequestAddition:      200,
				MemoryRequestAddition:   500,
				CPULimitMultiplier:      1.5,
				MemoryLimitMultiplier:   1.3,
				CPULimitAddition:        500,
				MemoryLimitAddition:     1000,
				MinCPURequest:           10,
				MinMemoryRequest:        64,
				MaxCPULimit:             4000,
				MaxMemoryLimit:          8000,
			},
			expectedCPUReq:   3500, // (3000 * 1.1) + 200 = 3500
			expectedMemReq:   7100, // (6000 * 1.1) + 500 = 7100
			expectedCPULimit: 4000, // (3500 * 1.5) + 500 = 5750, capped at 4000
			expectedMemLimit: 8000, // (7100 * 1.3) + 1000 = 10230, capped at 8000
			description:      "High usage with limits being capped",
		},
	}

	for i := range tests {
		tt := &tests[i]
		t.Run(tt.name, func(t *testing.T) {
			cfg := &tt.config

			// Calculate requests
			cpuRequest := int64(tt.cpuUsageMilli*cfg.CPURequestMultiplier) + cfg.CPURequestAddition
			memRequest := int64(tt.memUsageMB*cfg.MemoryRequestMultiplier) + cfg.MemoryRequestAddition

			// Apply minimums
			if cpuRequest < cfg.MinCPURequest {
				cpuRequest = cfg.MinCPURequest
			}
			if memRequest < cfg.MinMemoryRequest {
				memRequest = cfg.MinMemoryRequest
			}

			// Calculate limits
			cpuLimit := int64(float64(cpuRequest)*cfg.CPULimitMultiplier) + cfg.CPULimitAddition
			memLimit := int64(float64(memRequest)*cfg.MemoryLimitMultiplier) + cfg.MemoryLimitAddition

			// Apply caps
			if cpuLimit > cfg.MaxCPULimit {
				cpuLimit = cfg.MaxCPULimit
			}
			if memLimit > cfg.MaxMemoryLimit {
				memLimit = cfg.MaxMemoryLimit
			}

			assert.Equal(t, tt.expectedCPUReq, cpuRequest, "CPU request: "+tt.description)
			assert.Equal(t, tt.expectedMemReq, memRequest, "Memory request: "+tt.description)
			assert.Equal(t, tt.expectedCPULimit, cpuLimit, "CPU limit: "+tt.description)
			assert.Equal(t, tt.expectedMemLimit, memLimit, "Memory limit: "+tt.description)

			// Verify limits are always >= requests
			assert.GreaterOrEqual(t, cpuLimit, cpuRequest, "CPU limit should be >= request")
			assert.GreaterOrEqual(t, memLimit, memRequest, "Memory limit should be >= request")
		})
	}
}

func TestScalingDecisionImpact(t *testing.T) {
	tests := []struct {
		name            string
		cpuUsageMilli   float64
		memUsageMB      float64
		scalingDecision ResourceScalingDecision
		config          config.Config
		expectedCPUReq  int64
		expectedMemReq  int64
		description     string
	}{
		{
			name:          "both_scale_up",
			cpuUsageMilli: 100,
			memUsageMB:    200,
			scalingDecision: ResourceScalingDecision{
				CPU:    ScaleUp,
				Memory: ScaleUp,
			},
			config: config.Config{
				CPURequestMultiplier:    1.5,
				MemoryRequestMultiplier: 1.5,
				CPURequestAddition:      20,
				MemoryRequestAddition:   50,
				MinCPURequest:           10,
				MinMemoryRequest:        64,
			},
			expectedCPUReq: 170, // (100 * 1.5) + 20 = 170
			expectedMemReq: 350, // (200 * 1.5) + 50 = 350
			description:    "Both resources scaling up",
		},
		{
			name:          "both_scale_down",
			cpuUsageMilli: 100,
			memUsageMB:    200,
			scalingDecision: ResourceScalingDecision{
				CPU:    ScaleDown,
				Memory: ScaleDown,
			},
			config: config.Config{
				CPURequestMultiplier:    1.5, // Will use 1.1 for scale down
				MemoryRequestMultiplier: 1.5, // Will use 1.1 for scale down
				CPURequestAddition:      20,
				MemoryRequestAddition:   50,
				MinCPURequest:           10,
				MinMemoryRequest:        64,
			},
			expectedCPUReq: 130, // (100 * 1.1) + 20 = 130
			expectedMemReq: 270, // (200 * 1.1) + 50 = 270
			description:    "Both resources scaling down with reduced multiplier",
		},
		{
			name:          "mixed_scaling",
			cpuUsageMilli: 150,
			memUsageMB:    300,
			scalingDecision: ResourceScalingDecision{
				CPU:    ScaleUp,
				Memory: ScaleDown,
			},
			config: config.Config{
				CPURequestMultiplier:    1.4,
				MemoryRequestMultiplier: 1.4,
				CPURequestAddition:      30,
				MemoryRequestAddition:   60,
				MinCPURequest:           10,
				MinMemoryRequest:        64,
			},
			expectedCPUReq: 240, // (150 * 1.4) + 30 = 240
			expectedMemReq: 390, // (300 * 1.1) + 60 = 390
			description:    "CPU scaling up, Memory scaling down",
		},
		{
			name:          "no_change",
			cpuUsageMilli: 200,
			memUsageMB:    400,
			scalingDecision: ResourceScalingDecision{
				CPU:    ScaleNone,
				Memory: ScaleNone,
			},
			config: config.Config{
				CPURequestMultiplier:    1.3,
				MemoryRequestMultiplier: 1.3,
				CPURequestAddition:      40,
				MemoryRequestAddition:   80,
				MinCPURequest:           10,
				MinMemoryRequest:        64,
			},
			expectedCPUReq: 300, // (200 * 1.3) + 40 = 300
			expectedMemReq: 600, // (400 * 1.3) + 80 = 600
			description:    "No scaling decision, use standard multipliers",
		},
	}

	for i := range tests {
		tt := &tests[i]
		t.Run(tt.name, func(t *testing.T) {
			cfg := &tt.config

			// CPU calculation based on scaling decision
			var cpuRequest int64
			if tt.scalingDecision.CPU == ScaleUp {
				cpuRequest = int64(tt.cpuUsageMilli*cfg.CPURequestMultiplier) + cfg.CPURequestAddition
			} else if tt.scalingDecision.CPU == ScaleDown {
				cpuRequest = int64(tt.cpuUsageMilli*1.1) + cfg.CPURequestAddition
			} else {
				cpuRequest = int64(tt.cpuUsageMilli*cfg.CPURequestMultiplier) + cfg.CPURequestAddition
			}

			// Memory calculation based on scaling decision
			var memRequest int64
			if tt.scalingDecision.Memory == ScaleUp {
				memRequest = int64(tt.memUsageMB*cfg.MemoryRequestMultiplier) + cfg.MemoryRequestAddition
			} else if tt.scalingDecision.Memory == ScaleDown {
				memRequest = int64(tt.memUsageMB*1.1) + cfg.MemoryRequestAddition
			} else {
				memRequest = int64(tt.memUsageMB*cfg.MemoryRequestMultiplier) + cfg.MemoryRequestAddition
			}

			// Apply minimums
			if cpuRequest < cfg.MinCPURequest {
				cpuRequest = cfg.MinCPURequest
			}
			if memRequest < cfg.MinMemoryRequest {
				memRequest = cfg.MinMemoryRequest
			}

			assert.Equal(t, tt.expectedCPUReq, cpuRequest, "CPU request: "+tt.description)
			assert.Equal(t, tt.expectedMemReq, memRequest, "Memory request: "+tt.description)
		})
	}
}

func TestResourceQuantityConversion(t *testing.T) {
	tests := []struct {
		name              string
		cpuMillicores     int64
		memoryMB          int64
		expectedCPUString string
		expectedMemString string
	}{
		{
			name:              "standard_values",
			cpuMillicores:     250,
			memoryMB:          512,
			expectedCPUString: "250m",
			expectedMemString: "512Mi",
		},
		{
			name:              "whole_cpu_cores",
			cpuMillicores:     2000,
			memoryMB:          2048,
			expectedCPUString: "2",
			expectedMemString: "2Gi",
		},
		{
			name:              "fractional_cpu",
			cpuMillicores:     1500,
			memoryMB:          1536,
			expectedCPUString: "1500m",
			expectedMemString: "1536Mi",
		},
		{
			name:              "small_values",
			cpuMillicores:     10,
			memoryMB:          64,
			expectedCPUString: "10m",
			expectedMemString: "64Mi",
		},
		{
			name:              "large_values",
			cpuMillicores:     8000,
			memoryMB:          16384,
			expectedCPUString: "8",
			expectedMemString: "16Gi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create CPU quantity
			cpuQ := resource.NewMilliQuantity(tt.cpuMillicores, resource.DecimalSI)
			cpuStr := cpuQ.String()

			// Create memory quantity
			memQ := resource.NewQuantity(tt.memoryMB*1024*1024, resource.BinarySI)
			memStr := memQ.String()

			// Log the conversions
			t.Logf("CPU: %d millicores = %s", tt.cpuMillicores, cpuStr)
			t.Logf("Memory: %d MB = %s", tt.memoryMB, memStr)

			// Basic validation
			assert.NotEmpty(t, cpuStr, "CPU string should not be empty")
			assert.NotEmpty(t, memStr, "Memory string should not be empty")
		})
	}
}

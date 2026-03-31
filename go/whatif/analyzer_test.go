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

package whatif

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"right-sizer/memstore"
)

func TestNewAnalyzer(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	logger := zap.NewNop()

	analyzer := NewAnalyzer(store, nil, logger)

	if analyzer == nil {
		t.Fatal("Analyzer should not be nil")
	}
	if analyzer.store != store {
		t.Fatal("Store should be set")
	}
}

func TestAnalyze_InvalidRequest(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	analyzer := NewAnalyzer(store, nil, zap.NewNop())
	ctx := context.Background()

	// Missing namespace
	_, err := analyzer.Analyze(ctx, AnalysisRequest{
		PodName: "test-pod",
	})
	if err == nil {
		t.Error("Expected error for missing namespace")
	}

	// Missing pod name
	_, err = analyzer.Analyze(ctx, AnalysisRequest{
		Namespace: "default",
	})
	if err == nil {
		t.Error("Expected error for missing pod name")
	}
}

func TestAnalyze_NoHistoricalData(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	analyzer := NewAnalyzer(store, nil, zap.NewNop())
	ctx := context.Background()

	_, err := analyzer.Analyze(ctx, AnalysisRequest{
		Namespace:           "default",
		PodName:             "test-pod",
		ProposedMemoryLimit: 1024,
	})

	if err == nil {
		t.Error("Expected error when no historical data exists")
	}
}

func TestAnalyze_WithData(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	analyzer := NewAnalyzer(store, nil, zap.NewNop())
	ctx := context.Background()

	// Set resource limits
	store.SetLimits("default", "test-pod", &memstore.ResourceLimits{
		CPULimit:    1000, // 1000 millicores
		MemoryLimit: 1024, // 1024 MB
	})

	// Add historical data
	now := time.Now()
	for i := 0; i < 50; i++ {
		store.Record("default", "test-pod", memstore.DataPoint{
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
			CPUMilli:  300 + float64(i%50), // 300-350 millicores
			MemMB:     400 + float64(i%20), // 400-420 MB
		})
	}

	// Test with proposed resource decrease
	result, err := analyzer.Analyze(ctx, AnalysisRequest{
		Namespace:           "default",
		PodName:             "test-pod",
		ProposedCPULimit:    500,  // Lower than current 1000
		ProposedMemoryLimit: 600,  // Lower than current 1024
		TimeHorizon:         24 * time.Hour,
	})

	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	if result.Confidence <= 0 || result.Confidence > 1 {
		t.Errorf("Confidence should be between 0 and 1, got %f", result.Confidence)
	}

	if result.RiskAssessment.OverallRisk == "" {
		t.Error("Risk assessment should have an overall risk level")
	}
}

func TestAnalyze_HighRiskScenario(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	analyzer := NewAnalyzer(store, nil, zap.NewNop())
	ctx := context.Background()

	// Set resource limits
	store.SetLimits("default", "high-mem-pod", &memstore.ResourceLimits{
		CPULimit:    1000,
		MemoryLimit: 2048, // 2GB
	})

	// Add historical data with high memory usage
	now := time.Now()
	for i := 0; i < 50; i++ {
		store.Record("default", "high-mem-pod", memstore.DataPoint{
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
			CPUMilli:  200,
			MemMB:     1800 + float64(i%100), // 1800-1900 MB usage
		})
	}

	// Test with very low proposed memory limit - should be high risk
	result, err := analyzer.Analyze(ctx, AnalysisRequest{
		Namespace:           "default",
		PodName:             "high-mem-pod",
		ProposedMemoryLimit: 1024, // Much lower than usage
	})

	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	if result.RiskAssessment.OOMRisk < 0.7 {
		t.Errorf("Expected high OOM risk, got %f", result.RiskAssessment.OOMRisk)
	}

	if result.RiskAssessment.OverallRisk != "critical" && result.RiskAssessment.OverallRisk != "high" {
		t.Errorf("Expected high or critical risk, got %s", result.RiskAssessment.OverallRisk)
	}

	// Should have recommendations to increase memory
	hasIncreaseRec := false
	for _, rec := range result.Recommendations {
		if rec.Type == "increase" && rec.Resource == "memory" {
			hasIncreaseRec = true
			break
		}
	}
	if !hasIncreaseRec {
		t.Error("Expected recommendation to increase memory limit")
	}
}

func TestCompareScenarios(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	analyzer := NewAnalyzer(store, nil, zap.NewNop())
	ctx := context.Background()

	// Set resource limits
	store.SetLimits("default", "test-pod", &memstore.ResourceLimits{
		CPULimit:    1000,
		MemoryLimit: 1024,
	})

	// Add historical data
	now := time.Now()
	for i := 0; i < 50; i++ {
		store.Record("default", "test-pod", memstore.DataPoint{
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
			CPUMilli:  400,
			MemMB:     500,
		})
	}

	base := AnalysisRequest{
		Namespace: "default",
		PodName:   "test-pod",
	}

	scenarios := []AnalysisRequest{
		{
			Namespace:           "default",
			PodName:             "test-pod",
			ProposedCPULimit:    800,
			ProposedMemoryLimit: 800,
		},
		{
			Namespace:           "default",
			PodName:             "test-pod",
			ProposedCPULimit:    600,
			ProposedMemoryLimit: 600,
		},
	}

	results, err := analyzer.CompareScenarios(ctx, base, scenarios)
	if err != nil {
		t.Fatalf("CompareScenarios failed: %v", err)
	}

	if len(results) != 3 { // base + 2 scenarios
		t.Errorf("Expected 3 results, got %d", len(results))
	}
}

func TestCalculateProjectedMetrics(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	analyzer := NewAnalyzer(store, nil, zap.NewNop())

	stats := &memstore.Stats{
		Count:   50,
		CPUMean: 500,
		CPUMax:  800,
		MemMean: 600,
		MemMax:  900,
	}

	limits := &memstore.ResourceLimits{
		CPULimit:    1000,
		MemoryLimit: 1024,
	}

	req := AnalysisRequest{
		ProposedCPULimit:    800,
		ProposedMemoryLimit: 800,
	}

	pm := analyzer.calculateProjectedMetrics(stats, limits, req)

	// CPU utilization should be higher with lower limit
	expectedCPUAvg := (500.0 / 800.0) * 100
	if pm.CPUUtilizationAvg != expectedCPUAvg {
		t.Errorf("Expected CPU avg %.2f%%, got %.2f%%", expectedCPUAvg, pm.CPUUtilizationAvg)
	}

	// Memory utilization should be higher with lower limit
	expectedMemAvg := (600.0 / 800.0) * 100
	if pm.MemoryUtilizationAvg != expectedMemAvg {
		t.Errorf("Expected memory avg %.2f%%, got %.2f%%", expectedMemAvg, pm.MemoryUtilizationAvg)
	}
}

func TestCalculateImpactScore(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	analyzer := NewAnalyzer(store, nil, zap.NewNop())

	tests := []struct {
		name     string
		pm       ProjectedMetrics
		stats    *memstore.Stats
		expected float64
	}{
		{
			name: "High utilization should be negative",
			pm: ProjectedMetrics{
				CPUUtilizationAvg:    90,
				MemoryUtilizationAvg: 85,
			},
			stats:    &memstore.Stats{},
			expected: -0.75, // -0.5 for CPU, -0.25 for memory
		},
		{
			name: "Low utilization should be positive",
			pm: ProjectedMetrics{
				CPUUtilizationAvg:    25,
				MemoryUtilizationAvg: 30,
				ProjectedSavings:     20,
			},
			stats:    &memstore.Stats{CPUThrottleAvg: 2},
			expected: 0.2, // Just savings contribution
		},
		{
			name: "Balanced utilization",
			pm: ProjectedMetrics{
				CPUUtilizationAvg:    60,
				MemoryUtilizationAvg: 55,
			},
			stats:    &memstore.Stats{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := analyzer.calculateImpactScore(tt.pm, tt.stats)
			// Allow small floating point differences
			if score < tt.expected-0.1 || score > tt.expected+0.1 {
				t.Errorf("Expected impact score around %.2f, got %.2f", tt.expected, score)
			}
		})
	}
}

func TestAssessRisks(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	analyzer := NewAnalyzer(store, nil, zap.NewNop())

	stats := &memstore.Stats{
		CPUMean: 400,
		CPUMax:  600,
		MemMean: 500,
		MemMax:  700,
	}

	limits := &memstore.ResourceLimits{
		CPULimit:    1000,
		MemoryLimit: 1024,
	}

	tests := []struct {
		name           string
		req            AnalysisRequest
		expectedRisk   string
		expectedOOMRisk float64
	}{
		{
			name: "Safe resource limits",
			req: AnalysisRequest{
				ProposedCPULimit:    1000,
				ProposedMemoryLimit: 1024,
			},
			expectedRisk:    "low",
			expectedOOMRisk: 0.1,
		},
		{
			name: "Tight memory limit",
			req: AnalysisRequest{
				ProposedCPULimit:    1000,
				ProposedMemoryLimit: 750, // Below max observed (700)
			},
			expectedRisk:    "high",
			expectedOOMRisk: 0.7, // High risk due to utilization > 90%
		},
		{
			name: "High CPU utilization",
			req: AnalysisRequest{
				ProposedCPULimit:    500, // Will result in 120% CPU peak (600/500)
				ProposedMemoryLimit: 1024,
			},
			expectedRisk:    "critical", // 120% CPU peak = very high throttle risk
			expectedOOMRisk: 0.1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := analyzer.calculateProjectedMetrics(stats, limits, tt.req)
			risk := analyzer.assessRisks(stats, limits, tt.req, pm)

			if risk.OverallRisk != tt.expectedRisk {
				t.Errorf("Expected risk %s, got %s", tt.expectedRisk, risk.OverallRisk)
			}

			if risk.OOMRisk < tt.expectedOOMRisk-0.1 || risk.OOMRisk > tt.expectedOOMRisk+0.1 {
				t.Errorf("Expected OOM risk around %.1f, got %.1f", tt.expectedOOMRisk, risk.OOMRisk)
			}
		})
	}
}

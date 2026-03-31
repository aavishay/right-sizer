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

package capacity

import (
	"context"
	"testing"
	"time"

	"github.com/go-logr/logr"

	"right-sizer/memstore"
)

func TestNewPlanner(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	planner := NewPlanner(store, logr.Discard())

	if planner == nil {
		t.Fatal("Planner should not be nil")
	}
	if planner.store != store {
		t.Error("Store should be set")
	}
}

func TestPlan_InvalidRequest(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	planner := NewPlanner(store, logr.Discard())
	ctx := context.Background()

	// Missing namespace
	_, err := planner.Plan(ctx, PlanRequest{
		PodName: "test-pod",
	})
	if err == nil {
		t.Error("Expected error for missing namespace")
	}

	// Missing pod name
	_, err = planner.Plan(ctx, PlanRequest{
		Namespace: "default",
	})
	if err == nil {
		t.Error("Expected error for missing pod name")
	}
}

func TestPlan_InsufficientData(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	planner := NewPlanner(store, logr.Discard())
	ctx := context.Background()

	// Set limits but no data
	store.SetLimits("default", "test-pod", &memstore.ResourceLimits{
		CPULimit:    1000,
		MemoryLimit: 1024,
	})

	_, err := planner.Plan(ctx, PlanRequest{
		Namespace: "default",
		PodName:   "test-pod",
	})

	if err == nil {
		t.Error("Expected error when insufficient data")
	}
}

func TestPlan_WithData(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	planner := NewPlanner(store, logr.Discard())
	ctx := context.Background()

	// Set limits
	store.SetLimits("default", "test-pod", &memstore.ResourceLimits{
		CPULimit:    1000,
		MemoryLimit: 1024,
	})

	// Add historical data with growth trend
	now := time.Now()
	for i := 0; i < 100; i++ {
		store.Record("default", "test-pod", memstore.DataPoint{
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
			CPUMilli:  300 + float64(i)*2, // Increasing trend
			MemMB:     500 + float64(i),   // Slight growth
		})
	}

	result, err := planner.Plan(ctx, PlanRequest{
		Namespace: "default",
		PodName:   "test-pod",
	})

	if err != nil {
		t.Fatalf("Plan failed: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	if len(result.Forecasts) == 0 {
		t.Error("Should have forecasts")
	}

	if result.Namespace != "default" {
		t.Errorf("Expected namespace 'default', got '%s'", result.Namespace)
	}

	if result.PodName != "test-pod" {
		t.Errorf("Expected pod 'test-pod', got '%s'", result.PodName)
	}
}

func TestAnalyzeTrend_Increasing(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	planner := NewPlanner(store, logr.Discard())

	// Create data with clear increasing trend
	data := []memstore.DataPoint{}
	now := time.Now()
	for i := 0; i < 50; i++ {
		data = append(data, memstore.DataPoint{
			Timestamp: now.Add(-time.Duration(50-i) * time.Minute),
			CPUMilli:  100 + float64(i)*10, // Clear increasing trend
			MemMB:     500,
		})
	}

	trend := planner.analyzeTrend(data, "cpu")

	if trend.Direction != "increasing" {
		t.Errorf("Expected direction 'increasing', got '%s'", trend.Direction)
	}

	if trend.Slope <= 0 {
		t.Errorf("Expected positive slope, got %f", trend.Slope)
	}

	if trend.SampleSize != 50 {
		t.Errorf("Expected sample size 50, got %d", trend.SampleSize)
	}
}

func TestAnalyzeTrend_Decreasing(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	planner := NewPlanner(store, logr.Discard())

	// Create data with clear decreasing trend
	data := []memstore.DataPoint{}
	now := time.Now()
	for i := 0; i < 50; i++ {
		data = append(data, memstore.DataPoint{
			Timestamp: now.Add(-time.Duration(50-i) * time.Minute),
			CPUMilli:  500 - float64(i)*5, // Clear decreasing trend
			MemMB:     500,
		})
	}

	trend := planner.analyzeTrend(data, "cpu")

	if trend.Direction != "decreasing" {
		t.Errorf("Expected direction 'decreasing', got '%s'", trend.Direction)
	}

	if trend.Slope >= 0 {
		t.Errorf("Expected negative slope, got %f", trend.Slope)
	}
}

func TestGenerateForecast(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	planner := NewPlanner(store, logr.Discard())

	stats := &memstore.Stats{
		CPUMean: 400,
		CPUMax:  600,
	}

	trend := TrendAnalysis{
		Direction:  "increasing",
		Slope:      10,
		Confidence: 0.8,
		SampleSize: 100,
		R2Score:    0.85,
	}

	data := []memstore.DataPoint{}
	now := time.Now()
	for i := 0; i < 100; i++ {
		data = append(data, memstore.DataPoint{
			Timestamp: now.Add(-time.Duration(100-i) * time.Minute),
			CPUMilli:  300 + float64(i)*2,
			MemMB:     500,
		})
	}

	forecast := planner.generateForecast("cpu", stats, 1000, trend, 7*24*time.Hour, data)

	if forecast.ResourceType != "cpu" {
		t.Errorf("Expected resource type 'cpu', got '%s'", forecast.ResourceType)
	}

	if forecast.CurrentUsage != 400 {
		t.Errorf("Expected current usage 400, got %f", forecast.CurrentUsage)
	}

	if forecast.Horizon != 7*24*time.Hour {
		t.Errorf("Expected horizon 7 days, got %v", forecast.Horizon)
	}

	if forecast.Confidence != 0.8 {
		t.Errorf("Expected confidence 0.8, got %f", forecast.Confidence)
	}
}

func TestGenerateRecommendations(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	planner := NewPlanner(store, logr.Discard())

	forecasts := []Forecast{
		{
			ResourceType:   "cpu",
			Horizon:        7 * 24 * time.Hour,
			RiskLevel:      "critical",
			CurrentLimit:   500,
			PredictedUsage: 600,
			TrendAnalysis: TrendAnalysis{
				Direction: "increasing",
			},
		},
		{
			ResourceType:   "memory",
			Horizon:        7 * 24 * time.Hour,
			RiskLevel:      "low",
			CurrentLimit:   1024,
			PredictedLimit: 700,
			TrendAnalysis: TrendAnalysis{
				Direction: "decreasing",
			},
		},
	}

	summary := PlanSummary{}

	recs := planner.generateRecommendations(forecasts, summary)

	if len(recs) != 2 {
		t.Errorf("Expected 2 recommendations, got %d", len(recs))
	}

	// Should have one critical CPU and one opportunity for memory
	hasCPURec := false
	hasMemRec := false
	for _, rec := range recs {
		if len(rec) > 9 && rec[:9] == "CRITICAL:" {
			hasCPURec = true
		}
		if len(rec) > 12 && rec[:12] == "OPPORTUNITY:" {
			hasMemRec = true
		}
	}

	if !hasCPURec {
		t.Error("Expected a critical CPU recommendation")
	}
	if !hasMemRec {
		t.Error("Expected an opportunity memory recommendation")
	}
}

func TestGetMultiHorizonForecast(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	planner := NewPlanner(store, logr.Discard())
	ctx := context.Background()

	// Set limits
	store.SetLimits("default", "test-pod", &memstore.ResourceLimits{
		CPULimit:    1000,
		MemoryLimit: 1024,
	})

	// Add historical data
	now := time.Now()
	for i := 0; i < 100; i++ {
		store.Record("default", "test-pod", memstore.DataPoint{
			Timestamp: now.Add(-time.Duration(i) * time.Minute),
			CPUMilli:  400,
			MemMB:     500,
		})
	}

	result, err := planner.GetMultiHorizonForecast(ctx, "default", "test-pod")
	if err != nil {
		t.Fatalf("GetMultiHorizonForecast failed: %v", err)
	}

	// Should have 6 forecasts: cpu/mem x 3 horizons
	if len(result.Forecasts) != 6 {
		t.Errorf("Expected 6 forecasts, got %d", len(result.Forecasts))
	}
}

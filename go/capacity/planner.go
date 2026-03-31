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
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/go-logr/logr"

	"right-sizer/memstore"
)

// Planner provides capacity planning capabilities
type Planner struct {
	store  *memstore.MemoryStore
	logger logr.Logger
}

// NewPlanner creates a new capacity planner
func NewPlanner(store *memstore.MemoryStore, logger logr.Logger) *Planner {
	return &Planner{
		store:  store,
		logger: logger,
	}
}

// Forecast represents a capacity forecast for a resource
type Forecast struct {
	ResourceType    string                 `json:"resourceType"`    // "cpu" or "memory"
	CurrentUsage    float64                `json:"currentUsage"`    // current average usage
	CurrentLimit    float64                `json:"currentLimit"`    // current limit
	CurrentUtilization float64             `json:"currentUtilization"` // percentage
	Horizon         time.Duration          `json:"horizon"`
	PredictedUsage  float64                `json:"predictedUsage"`  // predicted usage at horizon
	PredictedLimit  float64                `json:"predictedLimit"`  // recommended limit
	GrowthRate      float64                `json:"growthRate"`      // daily growth rate
	Confidence      float64                `json:"confidence"`    // 0-1
	RiskLevel       string                 `json:"riskLevel"`       // "low", "medium", "high", "critical"
	Breakdown       ForecastBreakdown      `json:"breakdown"`       // detailed breakdown
	TrendAnalysis   TrendAnalysis          `json:"trendAnalysis"`
}

// ForecastBreakdown contains detailed forecast information
type ForecastBreakdown struct {
	DailyGrowth    float64 `json:"dailyGrowth"`    // Average daily increase
	WeeklyGrowth   float64 `json:"weeklyGrowth"`   // Average weekly increase
	MonthlyGrowth  float64 `json:"monthlyGrowth"`  // Projected monthly increase
	SeasonalFactor float64 `json:"seasonalFactor"` // Seasonal adjustment (1.0 = neutral)
}

// TrendAnalysis provides trend information
type TrendAnalysis struct {
	Direction   string  `json:"direction"`   // "increasing", "decreasing", "stable"
	Slope       float64 `json:"slope"`       // Rate of change
	R2Score     float64 `json:"r2Score"`     // Goodness of fit
	Confidence  float64 `json:"confidence"`  // Trend confidence
	SampleSize  int     `json:"sampleSize"`  // Number of data points
}

// PlanRequest represents a capacity planning request
type PlanRequest struct {
	Namespace      string
	PodName        string
	Horizons       []time.Duration // Forecast horizons (default: 7d, 30d, 90d)
	MinConfidence  float64       // Minimum confidence threshold
}

// PlanResult contains capacity forecasts for a pod
type PlanResult struct {
	Namespace    string      `json:"namespace"`
	PodName      string      `json:"podName"`
	Timestamp    time.Time   `json:"timestamp"`
	Forecasts    []Forecast  `json:"forecasts"` // One per horizon per resource
	Summary      PlanSummary `json:"summary"`
	Recommendations []string `json:"recommendations"`
}

// PlanSummary provides a high-level summary of capacity needs
type PlanSummary struct {
	TotalCurrentCPU        float64 `json:"totalCurrentCPU"`        // millicores
	TotalCurrentMemory     float64 `json:"totalCurrentMemory"`     // MB
	TotalPredictedCPU7d    float64 `json:"totalPredictedCPU7d"`
	TotalPredictedCPU30d   float64 `json:"totalPredictedCPU30d"`
	TotalPredictedCPU90d   float64 `json:"totalPredictedCPU90d"`
	TotalPredictedMem7d    float64 `json:"totalPredictedMem7d"`
	TotalPredictedMem30d   float64 `json:"totalPredictedMem30d"`
	TotalPredictedMem90d   float64 `json:"totalPredictedMem90d"`
	OverallRiskLevel       string  `json:"overallRiskLevel"`
}

// Plan generates capacity forecasts for a pod
func (p *Planner) Plan(ctx context.Context, req PlanRequest) (*PlanResult, error) {
	if req.Namespace == "" || req.PodName == "" {
		return nil, fmt.Errorf("namespace and podName are required")
	}

	// Set default horizons
	if len(req.Horizons) == 0 {
		req.Horizons = []time.Duration{
			7 * 24 * time.Hour,  // 7 days
			30 * 24 * time.Hour, // 30 days
			90 * 24 * time.Hour, // 90 days
		}
	}

	// Set default confidence
	if req.MinConfidence == 0 {
		req.MinConfidence = 0.7
	}

	// Get historical data (up to 30 days)
	data := p.store.GetHistoricalData(req.Namespace, req.PodName, 30*24*time.Hour)
	if len(data) < 48 { // Need at least 2 days of data
		return nil, fmt.Errorf("insufficient historical data: need at least 48 data points, have %d", len(data))
	}

	// Get current limits
	limits := p.store.GetLimits(req.Namespace, req.PodName)
	if limits == nil {
		return nil, fmt.Errorf("no resource limits found for %s/%s", req.Namespace, req.PodName)
	}

	// Get current stats
	stats := p.store.Query(req.Namespace, req.PodName, 24*time.Hour)
	if stats == nil {
		return nil, fmt.Errorf("no recent statistics for %s/%s", req.Namespace, req.PodName)
	}

	forecasts := []Forecast{}

	// Generate forecasts for CPU
	cpuTrend := p.analyzeTrend(data, "cpu")
	for _, horizon := range req.Horizons {
		forecast := p.generateForecast("cpu", stats, limits.CPULimit, cpuTrend, horizon, data)
		forecasts = append(forecasts, forecast)
	}

	// Generate forecasts for Memory
	memTrend := p.analyzeTrend(data, "memory")
	for _, horizon := range req.Horizons {
		forecast := p.generateForecast("memory", stats, limits.MemoryLimit, memTrend, horizon, data)
		forecasts = append(forecasts, forecast)
	}

	// Generate summary
	summary := p.generateSummary(forecasts, stats, limits)

	// Generate recommendations
	recommendations := p.generateRecommendations(forecasts, summary)

	result := &PlanResult{
		Namespace:       req.Namespace,
		PodName:         req.PodName,
		Timestamp:       time.Now(),
		Forecasts:       forecasts,
		Summary:         summary,
		Recommendations: recommendations,
	}

	p.logger.Info("Capacity plan generated",
		"namespace", req.Namespace,
		"pod", req.PodName,
		"forecasts", len(forecasts),
		"riskLevel", summary.OverallRiskLevel,
	)

	return result, nil
}

// analyzeTrend calculates trend for the specified resource
func (p *Planner) analyzeTrend(data []memstore.DataPoint, resourceType string) TrendAnalysis {
	if len(data) < 2 {
		return TrendAnalysis{Direction: "stable"}
	}

	// Sort by timestamp
	sorted := make([]memstore.DataPoint, len(data))
	copy(sorted, data)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	// Perform linear regression
	n := float64(len(sorted))
	var sumX, sumY, sumXY, sumX2 float64

	for i, dp := range sorted {
		x := float64(i)
		y := dp.CPUMilli
		if resourceType == "memory" {
			y = dp.MemMB
		}

		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denominator := (n * sumX2) - (sumX * sumX)
	if denominator == 0 {
		return TrendAnalysis{Direction: "stable"}
	}

	slope := ((n * sumXY) - (sumX * sumY)) / denominator
	intercept := (sumY - slope*sumX) / n

	// Calculate R-squared
	mean := sumY / n
	var ssRes, ssTot float64
	for i, dp := range sorted {
		actual := dp.CPUMilli
		if resourceType == "memory" {
			actual = dp.MemMB
		}
		predicted := slope*float64(i) + intercept

		ssRes += (actual - predicted) * (actual - predicted)
		ssTot += (actual - mean) * (actual - mean)
	}

	r2 := 1.0
	if ssTot > 0 {
		r2 = 1.0 - (ssRes / ssTot)
	}

	// Determine direction
	direction := "stable"
	if slope > 0.01 {
		direction = "increasing"
	} else if slope < -0.01 {
		direction = "decreasing"
	}

	// Calculate confidence based on R-squared and sample size
	confidence := r2 * math.Min(1.0, float64(len(sorted))/100.0)

	return TrendAnalysis{
		Direction:  direction,
		Slope:      slope,
		R2Score:    r2,
		Confidence: confidence,
		SampleSize: len(sorted),
	}
}

// generateForecast creates a forecast for a resource
func (p *Planner) generateForecast(
	resourceType string,
	stats *memstore.Stats,
	currentLimit float64,
	trend TrendAnalysis,
	horizon time.Duration,
	data []memstore.DataPoint,
) Forecast {
	var currentUsage float64
	if resourceType == "cpu" {
		currentUsage = stats.CPUMean
	} else {
		currentUsage = stats.MemMean
	}

	utilization := 0.0
	if currentLimit > 0 {
		utilization = (currentUsage / currentLimit) * 100
	}

	// Calculate predicted usage based on trend
	days := horizon.Hours() / 24
	predictedUsage := currentUsage
	if trend.Direction == "increasing" {
		predictedUsage = currentUsage + (trend.Slope * days)
	} else if trend.Direction == "decreasing" {
		predictedUsage = currentUsage + (trend.Slope * days)
		if predictedUsage < currentUsage*0.7 {
			predictedUsage = currentUsage * 0.7 // Cap at 30% decrease
		}
	}

	// Ensure predicted usage is non-negative
	if predictedUsage < 0 {
		predictedUsage = 0
	}

	// Calculate recommended limit (with 20% headroom)
	recommendedLimit := predictedUsage * 1.2
	if recommendedLimit < currentUsage*1.1 {
		recommendedLimit = currentUsage * 1.1 // At least 10% headroom
	}

	// Calculate growth rate
	var growthRate float64
	if currentUsage > 0 {
		growthRate = (predictedUsage - currentUsage) / currentUsage
	}
	growthRate = growthRate / days // Daily growth rate

	// Calculate breakdown
	breakdown := ForecastBreakdown{
		DailyGrowth:    growthRate,
		WeeklyGrowth:   growthRate * 7,
		MonthlyGrowth:  growthRate * 30,
		SeasonalFactor: 1.0,
	}

	// Determine risk level
	riskLevel := "low"
	predictedUtilization := 0.0
	if recommendedLimit > 0 {
		predictedUtilization = (predictedUsage / recommendedLimit) * 100
	}

	if predictedUtilization > 90 || trend.Confidence < 0.5 {
		riskLevel = "critical"
	} else if predictedUtilization > 80 {
		riskLevel = "high"
	} else if predictedUtilization > 70 {
		riskLevel = "medium"
	}

	return Forecast{
		ResourceType:       resourceType,
		CurrentUsage:       currentUsage,
		CurrentLimit:       currentLimit,
		CurrentUtilization: utilization,
		Horizon:            horizon,
		PredictedUsage:    predictedUsage,
		PredictedLimit:    recommendedLimit,
		GrowthRate:         growthRate,
		Confidence:         trend.Confidence,
		RiskLevel:          riskLevel,
		Breakdown:          breakdown,
		TrendAnalysis:      trend,
	}
}

// generateSummary creates a summary from forecasts
func (p *Planner) generateSummary(forecasts []Forecast, stats *memstore.Stats, limits *memstore.ResourceLimits) PlanSummary {
	summary := PlanSummary{
		TotalCurrentCPU:    stats.CPUMean,
		TotalCurrentMemory: stats.MemMean,
	}

	// Track highest risk level
	highestRisk := "low"
	riskPriority := map[string]int{
		"low":      0,
		"medium":   1,
		"high":     2,
		"critical": 3,
	}

	for _, f := range forecasts {
		// Update highest risk
		if riskPriority[f.RiskLevel] > riskPriority[highestRisk] {
			highestRisk = f.RiskLevel
		}

		// Extract forecasts by horizon
		days := f.Horizon.Hours() / 24
		if f.ResourceType == "cpu" {
			switch {
			case days <= 7:
				summary.TotalPredictedCPU7d = f.PredictedUsage
			case days <= 30:
				summary.TotalPredictedCPU30d = f.PredictedUsage
			default:
				summary.TotalPredictedCPU90d = f.PredictedUsage
			}
		} else {
			switch {
			case days <= 7:
				summary.TotalPredictedMem7d = f.PredictedUsage
			case days <= 30:
				summary.TotalPredictedMem30d = f.PredictedUsage
			default:
				summary.TotalPredictedMem90d = f.PredictedUsage
			}
		}
	}

	summary.OverallRiskLevel = highestRisk
	return summary
}

// generateRecommendations creates capacity recommendations
func (p *Planner) generateRecommendations(forecasts []Forecast, summary PlanSummary) []string {
	recs := []string{}

	// CPU recommendations
	for _, f := range forecasts {
		if f.ResourceType != "cpu" {
			continue
		}
		days := int(f.Horizon.Hours() / 24)

		if f.RiskLevel == "critical" || f.RiskLevel == "high" {
			recs = append(recs, fmt.Sprintf(
				"CRITICAL: Increase CPU limit within %d days. Predicted usage: %.0fm, Current limit: %.0fm",
				days, f.PredictedUsage, f.CurrentLimit,
			))
		} else if f.PredictedLimit < f.CurrentLimit*0.8 && f.TrendAnalysis.Direction == "decreasing" {
			recs = append(recs, fmt.Sprintf(
				"OPPORTUNITY: CPU limit can be reduced by %d days. Consider decreasing from %.0fm to %.0fm",
				days, f.CurrentLimit, f.PredictedLimit,
			))
		}
	}

	// Memory recommendations
	for _, f := range forecasts {
		if f.ResourceType != "memory" {
			continue
		}
		days := int(f.Horizon.Hours() / 24)

		if f.RiskLevel == "critical" || f.RiskLevel == "high" {
			recs = append(recs, fmt.Sprintf(
				"CRITICAL: Increase memory limit within %d days. Predicted usage: %.0fMi, Current limit: %.0fMi",
				days, f.PredictedUsage, f.CurrentLimit,
			))
		} else if f.PredictedLimit < f.CurrentLimit*0.8 && f.TrendAnalysis.Direction == "decreasing" {
			recs = append(recs, fmt.Sprintf(
				"OPPORTUNITY: Memory limit can be reduced by %d days. Consider decreasing from %.0fMi to %.0fMi",
				days, f.CurrentLimit, f.PredictedLimit,
			))
		}
	}

	return recs
}

// GetMultiHorizonForecast generates forecasts for multiple horizons
func (p *Planner) GetMultiHorizonForecast(ctx context.Context, namespace, podName string) (*PlanResult, error) {
	return p.Plan(ctx, PlanRequest{
		Namespace: namespace,
		PodName:   podName,
		Horizons: []time.Duration{
			7 * 24 * time.Hour,
			30 * 24 * time.Hour,
			90 * 24 * time.Hour,
		},
	})
}

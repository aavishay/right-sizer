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
	"fmt"
	"math"
	"time"

	"go.uber.org/zap"

	"right-sizer/memstore"
	"right-sizer/predictor"
)

// Analyzer provides what-if analysis for resource changes
type Analyzer struct {
	store     *memstore.MemoryStore
	predictor *predictor.Engine
	logger    *zap.Logger
}

// NewAnalyzer creates a new what-if analyzer
func NewAnalyzer(store *memstore.MemoryStore, predictorEngine *predictor.Engine, logger *zap.Logger) *Analyzer {
	return &Analyzer{
		store:     store,
		predictor: predictorEngine,
		logger:    logger,
	}
}

// AnalysisRequest represents a request to analyze resource changes
type AnalysisRequest struct {
	Namespace      string        `json:"namespace"`
	PodName        string        `json:"podName"`
	Container      string        `json:"container"`
	ProposedCPULimit    float64  `json:"proposedCPULimit,omitempty"`    // in millicores
	ProposedMemoryLimit float64  `json:"proposedMemoryLimit,omitempty"` // in MB
	TimeHorizon         time.Duration `json:"timeHorizon,omitempty"`
}

// AnalysisResult represents the result of a what-if analysis
type AnalysisResult struct {
	Request         AnalysisRequest          `json:"request"`
	Timestamp       time.Time                `json:"timestamp"`
	Confidence      float64                  `json:"confidence"`
	CurrentMetrics  ResourceMetrics          `json:"currentMetrics"`
	ProjectedMetrics ProjectedMetrics        `json:"projectedMetrics"`
	RiskAssessment   RiskAssessment          `json:"riskAssessment"`
	Recommendations []Recommendation         `json:"recommendations"`
}

// ResourceMetrics holds current resource metrics
type ResourceMetrics struct {
	CPUUtilizationAvg    float64 `json:"cpuUtilizationAvg"`    // percentage
	CPUUtilizationPeak   float64 `json:"cpuUtilizationPeak"`   // percentage
	MemoryUtilizationAvg float64 `json:"memoryUtilizationAvg"` // percentage
	MemoryUtilizationPeak float64 `json:"memoryUtilizationPeak"` // percentage
	CPUThrottleRate      float64 `json:"cpuThrottleRate"`      // percentage
}

// ProjectedMetrics holds projected metrics with proposed resources
type ProjectedMetrics struct {
	CPUUtilizationAvg    float64 `json:"cpuUtilizationAvg"`
	CPUUtilizationPeak   float64 `json:"cpuUtilizationPeak"`
	MemoryUtilizationAvg float64 `json:"memoryUtilizationAvg"`
	MemoryUtilizationPeak float64 `json:"memoryUtilizationPeak"`
	ProjectedSavings     float64 `json:"projectedSavings"` // percentage of resource reduction
	ImpactScore          float64 `json:"impactScore"`      // -1 to 1, negative is bad
}

// RiskAssessment contains risk analysis
type RiskAssessment struct {
	OverallRisk       string  `json:"overallRisk"` // "low", "medium", "high", "critical"
	OOMRisk           float64 `json:"oomRisk"`     // 0-1 probability
	CPUThrottleRisk   float64 `json:"cpuThrottleRisk"`
	PerformanceRisk   float64 `json:"performanceRisk"`
	RiskFactors       []string `json:"riskFactors"`
}

// Recommendation provides actionable advice
type Recommendation struct {
	Type        string  `json:"type"` // "increase", "decrease", "maintain", "investigate"
	Resource    string  `json:"resource"` // "cpu", "memory"
	Priority    string  `json:"priority"` // "low", "medium", "high"
	Description string  `json:"description"`
	Confidence  float64 `json:"confidence"`
}

// Analyze performs what-if analysis for the given request
func (a *Analyzer) Analyze(ctx context.Context, req AnalysisRequest) (*AnalysisResult, error) {
	// Validate request
	if req.Namespace == "" || req.PodName == "" {
		return nil, fmt.Errorf("namespace and podName are required")
	}

	// Set default time horizon
	if req.TimeHorizon == 0 {
		req.TimeHorizon = 24 * time.Hour
	}

	// Get current metrics from store
	currentStats := a.store.Query(req.Namespace, req.PodName, 24*time.Hour)
	if currentStats == nil {
		return nil, fmt.Errorf("no historical data available for %s/%s", req.Namespace, req.PodName)
	}

	// Get current limits
	limits := a.store.GetLimits(req.Namespace, req.PodName)
	if limits == nil {
		return nil, fmt.Errorf("no resource limits available for %s/%s", req.Namespace, req.PodName)
	}

	// Calculate current metrics
	currentMetrics := ResourceMetrics{
		CPUUtilizationAvg:    (currentStats.CPUMean / limits.CPULimit) * 100,
		CPUUtilizationPeak:   (currentStats.CPUMax / limits.CPULimit) * 100,
		MemoryUtilizationAvg: (currentStats.MemMean / limits.MemoryLimit) * 100,
		MemoryUtilizationPeak: (currentStats.MemMax / limits.MemoryLimit) * 100,
		CPUThrottleRate:      currentStats.CPUThrottleAvg,
	}

	// Calculate projected metrics based on proposed resources
	projectedMetrics := a.calculateProjectedMetrics(currentStats, limits, req)

	// Perform risk assessment
	riskAssessment := a.assessRisks(currentStats, limits, req, projectedMetrics)

	// Generate recommendations
	recommendations := a.generateRecommendations(currentMetrics, projectedMetrics, riskAssessment, req)

	// Calculate overall confidence
	confidence := a.calculateConfidence(currentStats, riskAssessment)

	result := &AnalysisResult{
		Request:           req,
		Timestamp:         time.Now(),
		Confidence:        confidence,
		CurrentMetrics:    currentMetrics,
		ProjectedMetrics:  projectedMetrics,
		RiskAssessment:    riskAssessment,
		Recommendations:   recommendations,
	}

	a.logger.Info("What-if analysis completed",
		zap.String("namespace", req.Namespace),
		zap.String("pod", req.PodName),
		zap.Float64("confidence", confidence),
		zap.String("risk", riskAssessment.OverallRisk),
	)

	return result, nil
}

// calculateProjectedMetrics calculates metrics with proposed resource limits
func (a *Analyzer) calculateProjectedMetrics(stats *memstore.Stats, currentLimits *memstore.ResourceLimits, req AnalysisRequest) ProjectedMetrics {
	pm := ProjectedMetrics{}

	// Use proposed limits or current limits if not specified
	proposedCPULimit := req.ProposedCPULimit
	if proposedCPULimit == 0 {
		proposedCPULimit = currentLimits.CPULimit
	}

	proposedMemoryLimit := req.ProposedMemoryLimit
	if proposedMemoryLimit == 0 {
		proposedMemoryLimit = currentLimits.MemoryLimit
	}

	// Calculate projected utilization percentages
	if proposedCPULimit > 0 {
		pm.CPUUtilizationAvg = (stats.CPUMean / proposedCPULimit) * 100
		pm.CPUUtilizationPeak = (stats.CPUMax / proposedCPULimit) * 100
	}

	if proposedMemoryLimit > 0 {
		pm.MemoryUtilizationAvg = (stats.MemMean / proposedMemoryLimit) * 100
		pm.MemoryUtilizationPeak = (stats.MemMax / proposedMemoryLimit) * 100
	}

	// Calculate projected savings
	if currentLimits.CPULimit > 0 && proposedCPULimit > 0 {
		pm.ProjectedSavings = ((currentLimits.CPULimit - proposedCPULimit) / currentLimits.CPULimit) * 100
	}
	if currentLimits.MemoryLimit > 0 && proposedMemoryLimit > 0 {
		memorySavings := ((currentLimits.MemoryLimit - proposedMemoryLimit) / currentLimits.MemoryLimit) * 100
		if memorySavings < pm.ProjectedSavings {
			pm.ProjectedSavings = memorySavings // Take the smaller savings
		}
	}

	// Calculate impact score (-1 to 1, negative means potential problems)
	pm.ImpactScore = a.calculateImpactScore(pm, stats)

	return pm
}

// calculateImpactScore calculates an impact score for the proposed changes
func (a *Analyzer) calculateImpactScore(pm ProjectedMetrics, stats *memstore.Stats) float64 {
	score := 0.0

	// Negative factors (increased risk)
	if pm.CPUUtilizationAvg > 80 {
		score -= (pm.CPUUtilizationAvg - 80) / 20 // 0 to -1
	}
	if pm.MemoryUtilizationAvg > 80 {
		score -= (pm.MemoryUtilizationAvg - 80) / 20
	}
	if pm.CPUUtilizationPeak > 95 {
		score -= 0.5
	}
	if pm.MemoryUtilizationPeak > 95 {
		score -= 0.5
	}

	// Positive factors (improvements)
	if stats.CPUThrottleAvg > 5 && pm.CPUUtilizationAvg < 70 {
		score += 0.3 // Reducing throttling
	}
	if pm.ProjectedSavings > 10 {
		score += 0.2 // Resource savings
	}

	return math.Max(-1, math.Min(1, score))
}

// assessRisks evaluates the risks of the proposed resource changes
func (a *Analyzer) assessRisks(stats *memstore.Stats, currentLimits *memstore.ResourceLimits, req AnalysisRequest, pm ProjectedMetrics) RiskAssessment {
	ra := RiskAssessment{
		OverallRisk: "low",
		RiskFactors: []string{},
	}

	// Calculate OOM risk
	if req.ProposedMemoryLimit > 0 && req.ProposedMemoryLimit < stats.MemMax {
		ra.OOMRisk = 0.9
		ra.RiskFactors = append(ra.RiskFactors, "Proposed memory limit below observed peak usage")
	} else if pm.MemoryUtilizationPeak > 90 {
		ra.OOMRisk = 0.7
		ra.RiskFactors = append(ra.RiskFactors, "Projected memory utilization is very high")
	} else if pm.MemoryUtilizationAvg > 75 {
		ra.OOMRisk = 0.4
	} else {
		ra.OOMRisk = 0.1
	}

	// Calculate CPU throttle risk
	if req.ProposedCPULimit > 0 && req.ProposedCPULimit < stats.CPUMax {
		ra.CPUThrottleRisk = 0.8
		ra.RiskFactors = append(ra.RiskFactors, "Proposed CPU limit below observed peak usage")
	} else if pm.CPUUtilizationPeak > 90 {
		ra.CPUThrottleRisk = 0.6
		ra.RiskFactors = append(ra.RiskFactors, "Projected CPU utilization is very high")
	} else if pm.CPUUtilizationAvg > 70 {
		ra.CPUThrottleRisk = 0.3
	} else {
		ra.CPUThrottleRisk = 0.1
	}

	// Calculate performance risk
	performanceFactors := 0
	if ra.OOMRisk > 0.5 {
		performanceFactors++
	}
	if ra.CPUThrottleRisk > 0.5 {
		performanceFactors++
	}
	if pm.ImpactScore < -0.3 {
		performanceFactors++
	}

	ra.PerformanceRisk = float64(performanceFactors) / 3.0

	// Determine overall risk level
	maxRisk := math.Max(ra.OOMRisk, math.Max(ra.CPUThrottleRisk, ra.PerformanceRisk))
	switch {
	case maxRisk >= 0.8:
		ra.OverallRisk = "critical"
	case maxRisk >= 0.6:
		ra.OverallRisk = "high"
	case maxRisk >= 0.3:
		ra.OverallRisk = "medium"
	default:
		ra.OverallRisk = "low"
	}

	return ra
}

// generateRecommendations creates actionable recommendations
func (a *Analyzer) generateRecommendations(current ResourceMetrics, projected ProjectedMetrics, risk RiskAssessment, req AnalysisRequest) []Recommendation {
	recs := []Recommendation{}

	// Memory recommendations
	if req.ProposedMemoryLimit > 0 {
		if risk.OOMRisk > 0.7 {
			recs = append(recs, Recommendation{
				Type:        "increase",
				Resource:    "memory",
				Priority:    "high",
				Description: fmt.Sprintf("Increase memory limit to at least %.0fMi to prevent OOM kills", projected.MemoryUtilizationPeak*req.ProposedMemoryLimit/100*1.2),
				Confidence:  0.9,
			})
		} else if projected.MemoryUtilizationAvg < 40 && risk.OOMRisk < 0.3 {
			recs = append(recs, Recommendation{
				Type:        "decrease",
				Resource:    "memory",
				Priority:    "low",
				Description: "Memory limit could be reduced further based on current usage patterns",
				Confidence:  0.7,
			})
		} else if projected.MemoryUtilizationAvg >= 40 && projected.MemoryUtilizationAvg <= 70 && risk.OOMRisk < 0.3 {
			recs = append(recs, Recommendation{
				Type:        "maintain",
				Resource:    "memory",
				Priority:    "low",
				Description: "Memory limit is well-optimized for current workload",
				Confidence:  0.8,
			})
		}
	}

	// CPU recommendations
	if req.ProposedCPULimit > 0 {
		if risk.CPUThrottleRisk > 0.7 {
			recs = append(recs, Recommendation{
				Type:        "increase",
				Resource:    "cpu",
				Priority:    "high",
				Description: fmt.Sprintf("Increase CPU limit to at least %.0fm to prevent throttling", projected.CPUUtilizationPeak*req.ProposedCPULimit/100*1.2),
				Confidence:  0.9,
			})
		} else if current.CPUThrottleRate > 10 && projected.CPUUtilizationAvg < 50 {
			recs = append(recs, Recommendation{
				Type:        "investigate",
				Resource:    "cpu",
				Priority:    "medium",
				Description: "Current workload experiencing throttling; consider CPU burst patterns",
				Confidence:  0.75,
			})
		} else if projected.CPUUtilizationAvg < 30 && risk.CPUThrottleRisk < 0.3 {
			recs = append(recs, Recommendation{
				Type:        "decrease",
				Resource:    "cpu",
				Priority:    "low",
				Description: "CPU limit could be reduced for cost savings",
				Confidence:  0.6,
			})
		}
	}

	return recs
}

// calculateConfidence computes an overall confidence score
func (a *Analyzer) calculateConfidence(stats *memstore.Stats, risk RiskAssessment) float64 {
	// Base confidence on data quantity
	confidence := 0.5
	if stats.Count >= 100 {
		confidence = 0.9
	} else if stats.Count >= 50 {
		confidence = 0.8
	} else if stats.Count >= 20 {
		confidence = 0.7
	} else if stats.Count >= 10 {
		confidence = 0.6
	}

	// Reduce confidence for high uncertainty (high risk)
	if risk.OverallRisk == "critical" || risk.OverallRisk == "high" {
		confidence *= 0.8
	}

	return math.Max(0.1, confidence)
}

// CompareScenarios compares multiple resource scenarios
func (a *Analyzer) CompareScenarios(ctx context.Context, base AnalysisRequest, scenarios []AnalysisRequest) ([]*AnalysisResult, error) {
	results := make([]*AnalysisResult, 0, len(scenarios)+1)

	// First, analyze the base scenario
	baseResult, err := a.Analyze(ctx, base)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze base scenario: %w", err)
	}
	results = append(results, baseResult)

	// Analyze each scenario
	for i, scenario := range scenarios {
		result, err := a.Analyze(ctx, scenario)
		if err != nil {
			a.logger.Warn("Failed to analyze scenario",
				zap.Int("scenario_index", i),
				zap.Error(err),
			)
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

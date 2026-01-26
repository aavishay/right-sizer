package whatif

import (
	"fmt"
	"math"
	"right-sizer/memstore"
	"time"
)

// ScenarioResult represents what-if analysis results
type ScenarioResult struct {
	Namespace           string
	PodName             string
	ResourceType        string
	ScenarioType        string
	CurrentAllocation   float64
	ProposedAllocation  float64
	RiskScore           float64
	RiskLevel           string
	Confidence          float64
	EstimatedCostChange float64
	HoursPerMonth       float64
	Timestamp           time.Time
}

// Simulator performs what-if analysis
type Simulator struct {
	store               *memstore.MemoryStore
	riskThresholdMedium float64
	riskThresholdHigh   float64
}

// NewSimulator creates a simulator
func NewSimulator(store *memstore.MemoryStore) *Simulator {
	return &Simulator{
		store:               store,
		riskThresholdMedium: 0.70,
		riskThresholdHigh:   0.90,
	}
}

// SimulateScaleUp simulates scaling up resources
func (s *Simulator) SimulateScaleUp(namespace, podName, resourceType string, current, proposed float64) (*ScenarioResult, error) {
	stats := s.store.Query(namespace, podName, 7*24*time.Hour)
	if stats == nil || stats.Count == 0 {
		return nil, fmt.Errorf("insufficient historical data")
	}

	result := &ScenarioResult{
		Namespace:          namespace,
		PodName:            podName,
		ResourceType:       resourceType,
		ScenarioType:       "scale-up",
		CurrentAllocation:  current,
		ProposedAllocation: proposed,
		Confidence:         math.Min(1.0, float64(stats.Count)/1440.0),
		HoursPerMonth:      730,
		Timestamp:          time.Now(),
	}

	peak := s.pickPeak(stats, resourceType)
	result.RiskScore = (peak / proposed)
	result.RiskLevel = s.calculateRiskLevel(result.RiskScore)
	result.EstimatedCostChange = (proposed - current) * 0.01 * result.HoursPerMonth / 1000

	if result.Confidence > 1.0 {
		result.Confidence = 1.0
	}

	return result, nil
}

// SimulateScaleDown simulates scaling down resources
func (s *Simulator) SimulateScaleDown(namespace, podName, resourceType string, current, proposed float64) (*ScenarioResult, error) {
	stats := s.store.Query(namespace, podName, 7*24*time.Hour)
	if stats == nil || stats.Count == 0 {
		return nil, fmt.Errorf("insufficient historical data")
	}

	result := &ScenarioResult{
		Namespace:          namespace,
		PodName:            podName,
		ResourceType:       resourceType,
		ScenarioType:       "scale-down",
		CurrentAllocation:  current,
		ProposedAllocation: proposed,
		Confidence:         math.Min(1.0, float64(stats.Count)/1440.0),
		HoursPerMonth:      730,
		Timestamp:          time.Now(),
	}

	peak := s.pickPeak(stats, resourceType)
	result.RiskScore = (peak / proposed)
	result.RiskLevel = s.calculateRiskLevel(result.RiskScore)
	result.EstimatedCostChange = (proposed - current) * 0.01 * result.HoursPerMonth / 1000

	if result.Confidence > 1.0 {
		result.Confidence = 1.0
	}

	return result, nil
}

// SimulateMultipleScenarios simulates multiple scaling options
func (s *Simulator) SimulateMultipleScenarios(namespace, podName, resourceType string, current float64, proposals []float64) ([]*ScenarioResult, error) {
	stats := s.store.Query(namespace, podName, 7*24*time.Hour)
	if stats == nil || stats.Count == 0 {
		return nil, fmt.Errorf("insufficient historical data")
	}

	var results []*ScenarioResult
	for _, proposed := range proposals {
		result := &ScenarioResult{
			Namespace:          namespace,
			PodName:            podName,
			ResourceType:       resourceType,
			CurrentAllocation:  current,
			ProposedAllocation: proposed,
			Confidence:         math.Min(1.0, float64(stats.Count)/1440.0),
			HoursPerMonth:      730,
			Timestamp:          time.Now(),
		}

		if proposed > current {
			result.ScenarioType = "scale-up"
		} else if proposed < current {
			result.ScenarioType = "scale-down"
		} else {
			result.ScenarioType = "no-change"
		}

		peak := s.pickPeak(stats, resourceType)
		result.RiskScore = (peak / proposed)
		result.RiskLevel = s.calculateRiskLevel(result.RiskScore)
		result.EstimatedCostChange = (proposed - current) * 0.01 * result.HoursPerMonth / 1000

		results = append(results, result)
	}

	return results, nil
}

// calculateRiskLevel determines risk level from score
func (s *Simulator) calculateRiskLevel(score float64) string {
	if score < s.riskThresholdMedium {
		return "low"
	} else if score < s.riskThresholdHigh {
		return "medium"
	}
	return "high"
}

func (s *Simulator) pickPeak(stats *memstore.Stats, resourceType string) float64 {
	if stats == nil {
		return 0
	}
	switch resourceType {
	case "cpu":
		return stats.CPUMax
	case "memory", "mem", "memoryMi":
		return stats.MemMax
	default:
		return math.Max(stats.CPUMax, stats.MemMax)
	}
}

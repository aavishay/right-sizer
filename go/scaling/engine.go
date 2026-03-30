package scaling

import (
	"fmt"
	"math"
	"right-sizer/memstore"
	"right-sizer/predictor"
	"time"
)

// ScalingDecision captures a recommended adjustment.
type ScalingDecision struct {
	CurrentValue     float64
	RecommendedValue float64
	Confidence       float64
	Reason           string
}

// ShouldScale returns true when the recommendation is meaningful and confident.
func (d *ScalingDecision) ShouldScale() bool {
	if d == nil {
		return false
	}
	if d.Confidence < 0.7 {
		return false
	}
	if d.CurrentValue == 0 {
		return false
	}
	return math.Abs(d.RecommendedValue-d.CurrentValue) > 0.01
}

// ScalePercent returns the percentage delta from current to recommended.
func (d *ScalingDecision) ScalePercent() float64 {
	if d == nil || d.CurrentValue == 0 {
		return 0
	}
	return (d.RecommendedValue - d.CurrentValue) / d.CurrentValue * 100
}

// Engine computes scaling decisions from historical utilization.
type Engine struct {
	store               *memstore.MemoryStore
	lookback           time.Duration
	percentile         float64
	bufferFactor       float64
	confidenceThreshold float64
}

const (
	defaultLookback     = 24 * time.Hour
	defaultPercentile   = 0.95
	defaultBufferFactor = 1.1
)

// NewScalingEngine constructs a scaling engine using memstore stats.
func NewScalingEngine(store *memstore.MemoryStore) *Engine {
	return &Engine{
		store:               store,
		lookback:            defaultLookback,
		percentile:          defaultPercentile,
		bufferFactor:        defaultBufferFactor,
		confidenceThreshold: 0.70,
	}
}

// ComputeScalingDecision calculates a recommendation for a resource.
func (e *Engine) ComputeScalingDecision(namespace, pod, container, resourceType string, current float64) (*ScalingDecision, error) {
	if e == nil || e.store == nil {
		return nil, fmt.Errorf("engine not initialized")
	}
	if current <= 0 {
		return nil, fmt.Errorf("current value must be positive")
	}

	stats := e.store.Query(namespace, pod, e.lookback)
	if stats == nil || stats.Count == 0 {
		return nil, fmt.Errorf("insufficient historical data")
	}

	cpuP, memP := e.store.Percentile(namespace, pod, e.lookback, e.percentile)

	var peak, avgValue, stdDev float64
	switch resourceType {
	case predictor.ResourceTypeCPU:
		peak = math.Max(cpuP, stats.CPUMax)
		avgValue = stats.CPUMean
		stdDev = stats.CPUStdDev
	case predictor.ResourceTypeMemory, "mem", "memoryMi":
		peak = math.Max(memP, stats.MemMax)
		avgValue = stats.MemMean
		stdDev = stats.MemStdDev
	default:
		return nil, fmt.Errorf("unsupported resource type: %s", resourceType)
	}

	recommended := peak * e.bufferFactor
	if recommended <= 0 {
		recommended = current
	}

	confidence := predictor.CalculateConfidence(stats.Count, avgValue, stdDev, 0.6, 1440)
	decision := &ScalingDecision{
		CurrentValue:     current,
		RecommendedValue: recommended,
		Confidence:       confidence,
		Reason:           fmt.Sprintf("peak %.2f with %.0fth percentile and buffer %.2f", peak, e.percentile*100, e.bufferFactor),
	}

	if confidence < e.confidenceThreshold {
		return nil, fmt.Errorf("confidence %.2f below threshold %.2f", confidence, e.confidenceThreshold)
	}
	return decision, nil
}

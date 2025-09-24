//go:build legacy_aiops_unused

package analyzer

import (
	"errors"
	"fmt"
	"math"
	"time"

	"right-sizer/aiops/collector"
	"right-sizer/metrics"
)

// AnalysisResult holds the outcome of the memory analysis.
type AnalysisResult struct {
	IsLeak          bool
	Confidence      float64
	GrowthRateMBMin float64
	DurationMinutes float64
	SampleCount     int
	R2              float64
	CausalEvent     string // e.g., "Deployment of image v1.2.1"
	Reasoning       string // short machine explanation for narrative prompt enrichment
}

// MemoryAnalyzer analyzes historical samples for memory leak patterns.
type MemoryAnalyzer struct {
	metricsProvider metrics.Provider
	store           *HistoricalStore
	cfg             LeakDetectionConfig
}

// LeakDetectionConfig controls thresholds for leak detection.
type LeakDetectionConfig struct {
	MinSamples          int           // minimum samples required to attempt regression
	MinWindow           time.Duration // minimum look-back window
	MaxWindow           time.Duration // maximum look-back window
	R2Threshold         float64       // minimum R² to consider regression meaningful
	MinSlopeMBPerMinute float64       // minimum slope considered a potential leak
	HighSlopeMBPerMin   float64       // slope considered strong leak evidence
}

// DefaultLeakDetectionConfig returns sane defaults.
func DefaultLeakDetectionConfig() LeakDetectionConfig {
	return LeakDetectionConfig{
		MinSamples:          12, // e.g. if sampled every 1-5m
		MinWindow:           15 * time.Minute,
		MaxWindow:           90 * time.Minute,
		R2Threshold:         0.55,
		MinSlopeMBPerMinute: 0.2,  // ~1MB every 5 min
		HighSlopeMBPerMin:   2.00, // very aggressive growth
	}
}

// NewMemoryAnalyzer creates a new MemoryAnalyzer with an internal historical store.
// (A future improvement could inject a shared sampler/store.)
func NewMemoryAnalyzer(provider metrics.Provider) *MemoryAnalyzer {
	return &MemoryAnalyzer{
		metricsProvider: provider,
		store:           NewHistoricalStore(DefaultStoreConfig()),
		cfg:             DefaultLeakDetectionConfig(),
	}
}

// InjectStore allows supplying an external shared historical store (optional).
func (a *MemoryAnalyzer) InjectStore(store *HistoricalStore) {
	if store != nil {
		a.store = store
	}
}

// AddSample allows an external sampler to push samples into the analyzer's store.
func (a *MemoryAnalyzer) AddSample(namespace, pod, container string, cpuMilli, memMB float64, ts time.Time) {
	if a.store == nil {
		return
	}
	a.store.AddSample(namespace, pod, container, cpuMilli, memMB, ts)
}

// AnalyzeForOOMEvent performs regression-based leak detection using historical samples
// preceding the OOM event.
func (a *MemoryAnalyzer) AnalyzeForOOMEvent(event collector.OOMEvent) (*AnalysisResult, error) {
	if a.store == nil {
		return nil, errors.New("historical store not initialized")
	}

	// Select adaptive window: start with max window, shrink if insufficient samples.
	window := a.cfg.MaxWindow
	var samples []Sample
	var err error
	for {
		since := event.Timestamp.Add(-window)
		samples, err = a.store.GetSeries(event.Namespace, event.PodName, event.ContainerName, since)
		if err == nil && len(samples) >= a.cfg.MinSamples {
			break
		}
		// Reduce window size progressively if we have no data
		if window > a.cfg.MinWindow {
			window = window / 2
			if window < a.cfg.MinWindow {
				window = a.cfg.MinWindow
			}
			continue
		}
		// Could not gather enough samples
		return &AnalysisResult{
			IsLeak:     false,
			Confidence: 0.0,
			Reasoning:  "Insufficient historical memory samples for leak determination",
		}, nil
	}

	// Run regression
	reg, regErr := ComputeMemoryGrowth(samples, a.cfg.R2Threshold)
	if regErr != nil {
		return &AnalysisResult{
			IsLeak:     false,
			Confidence: 0.0,
			Reasoning:  fmt.Sprintf("Regression failed: %v", regErr),
		}, nil
	}

	// Determine leak heuristics
	isLeak, confidence, reasoning := a.evaluateLeak(reg)

	result := &AnalysisResult{
		IsLeak:          isLeak,
		Confidence:      confidence,
		GrowthRateMBMin: reg.Slope,
		DurationMinutes: reg.DurationMins,
		SampleCount:     reg.Points,
		R2:              reg.R2,
		CausalEvent:     "", // Placeholder – future correlation with deployments
		Reasoning:       reasoning,
	}

	return result, nil
}

// evaluateLeak converts regression output into leak decision & confidence.
func (a *MemoryAnalyzer) evaluateLeak(reg LinearRegressionResult) (bool, float64, string) {
	if !reg.PositiveTrend {
		return false, 0.0, fmt.Sprintf("Trend not strong enough (slope=%.3f MB/min, R²=%.2f)", reg.Slope, reg.R2)
	}

	// Base confidence from R² (mapped 0.5..1.0)
	r2Component := clamp((reg.R2-a.cfg.R2Threshold)/(1.0-a.cfg.R2Threshold), 0, 1)

	// Slope normalization (relative to HighSlopeMBPerMin)
	slopeNorm := clamp(reg.Slope/a.cfg.HighSlopeMBPerMin, 0, 1)

	// Duration factor: prefer at least half of max window
	durationNorm := clamp(reg.DurationMins/(a.cfg.MaxWindow.Minutes()/2), 0, 1)

	// Combine with weights
	confidence := 0.45*r2Component + 0.35*slopeNorm + 0.20*durationNorm

	// Slight penalty for very short observation
	if reg.DurationMins < a.cfg.MinWindow.Minutes() {
		confidence *= 0.75
	}

	// Enforce minimum leak slope threshold
	if reg.Slope < a.cfg.MinSlopeMBPerMinute {
		return false, confidence * 0.3, fmt.Sprintf("Low slope %.3f MB/min (< %.2f); uncertain leak", reg.Slope, a.cfg.MinSlopeMBPerMinute)
	}

	reason := fmt.Sprintf(
		"Persistent positive memory growth detected: slope=%.3f MB/min over %.1f min (R²=%.2f, samples=%d)",
		reg.Slope, reg.DurationMins, reg.R2, reg.Points,
	)

	return true, round(confidence, 3), reason
}

// Utility helpers
func clamp(v, minV, maxV float64) float64 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func round(v float64, precision int) float64 {
	p := math.Pow(10, float64(precision))
	return math.Round(v*p) / p
}

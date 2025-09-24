package analyzers

import (
	"fmt"
	"time"

	"right-sizer/internal/aiops/core"
)

// MemoryLeakAnalyzerAdapter adapts the existing MemoryAnalyzer implementation
// to the generic core.Analyzer interface so it can participate in the unified
// signal → finding → incident pipeline without importing the parent aiops package
// (avoids import cycle).
type MemoryLeakAnalyzerAdapter struct {
	name                 string
	analyzer             *MemoryAnalyzer
	minFindingConfidence float64 // threshold to emit a PartialFinding
}

// NewMemoryLeakAnalyzerAdapter creates a new adapter.
// minFindingConfidence: minimum confidence (0..1) required to emit a finding (recommend 0.5–0.7).
func NewMemoryLeakAnalyzerAdapter(a *MemoryAnalyzer, minFindingConfidence float64) *MemoryLeakAnalyzerAdapter {
	if minFindingConfidence <= 0 {
		minFindingConfidence = 0.55
	}
	return &MemoryLeakAnalyzerAdapter{
		name:                 "memory_leak",
		analyzer:             a,
		minFindingConfidence: minFindingConfidence,
	}
}

// Name returns a stable analyzer identifier.
func (m *MemoryLeakAnalyzerAdapter) Name() string {
	return m.name
}

// InterestedIn declares the signal types the adapter wants to process.
func (m *MemoryLeakAnalyzerAdapter) InterestedIn() []core.SignalType {
	return []core.SignalType{core.SignalOOMKilled}
}

// Handle converts an incoming Signal into evidence / finding output.
// Contract with Engine / Registry:
//   - If the signal is not relevant, return (nil, nil, nil).
//   - Always return evidence slice when analysis succeeds (even if no leak).
//   - Only return a non-nil PartialFinding when a leak is confidently detected.
func (m *MemoryLeakAnalyzerAdapter) Handle(sig core.Signal, access core.HistoricalAccessor) (*core.PartialFinding, []core.Evidence, error) {
	if sig.Type != core.SignalOOMKilled {
		return nil, nil, nil
	}

	// Generic decoding of expected OOM payload (kept loose to avoid collector import).
	var (
		ns        string
		pod       string
		container string
		eventTime time.Time
	)

	switch v := sig.Payload.(type) {
	case map[string]any:
		if s, ok := v["Namespace"].(string); ok {
			ns = s
		}
		if s, ok := v["PodName"].(string); ok {
			pod = s
		}
		if s, ok := v["ContainerName"].(string); ok {
			container = s
		}
		if ts, ok := v["Timestamp"].(time.Time); ok {
			eventTime = ts
		}
	default:
		ev := core.Evidence{
			ID:          fmt.Sprintf("mem-leak-unsupported-%d", time.Now().UnixNano()),
			Category:    "ANALYSIS",
			Description: fmt.Sprintf("OOM signal payload unsupported (type=%T) – leak analyzer skipped", sig.Payload),
			Confidence:  0,
			Data: map[string]any{
				"payloadType": fmt.Sprintf("%T", sig.Payload),
			},
			Timestamp: time.Now().UTC(),
		}
		return nil, []core.Evidence{ev}, nil
	}

	// If we lack essentials, return informational evidence.
	if ns == "" || pod == "" || container == "" {
		ev := core.Evidence{
			ID:          fmt.Sprintf("mem-leak-incomplete-%d", time.Now().UnixNano()),
			Category:    "ANALYSIS",
			Description: "OOM signal missing namespace/pod/container – leak analysis skipped",
			Confidence:  0,
			Data: map[string]any{
				"namespace":  ns,
				"pod":        pod,
				"container":  container,
				"hasPayload": true,
			},
			Timestamp: time.Now().UTC(),
		}
		return nil, []core.Evidence{ev}, nil
	}

	// Underlying analyzer currently requires a concrete OOM event type (legacy path).
	// Emit placeholder evidence until refactor allows direct regression invocation here.
	ev := core.Evidence{
		ID:          fmt.Sprintf("mem-leak-deferred-%d", time.Now().UnixNano()),
		Category:    "ANALYSIS",
		Description: "Memory leak analysis deferred (collector dependency temporarily removed)",
		Confidence:  0,
		Data: map[string]any{
			"namespace": ns,
			"pod":       pod,
			"container": container,
			"timestamp": eventTime,
			"reason":    "awaiting analyzer refactor to decouple from collector types",
		},
		Timestamp: time.Now().UTC(),
	}
	return nil, []core.Evidence{ev}, nil
}

// humanRegressionDescription creates a succinct, human-friendly summary for Evidence.Description.
func humanRegressionDescription(r *AnalysisResult) string {
	if r == nil {
		return "No memory analysis result available"
	}
	if !r.IsLeak {
		return fmt.Sprintf("Memory growth pattern not classified as leak (slope=%.3f MB/min, R²=%.2f, samples=%d, duration=%.1f min)",
			r.GrowthRateMBMin, r.R2, r.SampleCount, r.DurationMinutes)
	}
	return fmt.Sprintf("Probable memory leak: sustained growth slope=%.3f MB/min over %.1f min (R²=%.2f, samples=%d)",
		r.GrowthRateMBMin, r.DurationMinutes, r.R2, r.SampleCount)
}

// narrativeHint produces a short phrase guiding higher-level narrative generation.
func narrativeHint(r *AnalysisResult) string {
	if r == nil {
		return "No analysis result to describe"
	}
	return fmt.Sprintf("Sustained linear memory increase (%.2f MB/min, R²=%.2f) preceding OOM", r.GrowthRateMBMin, r.R2)
}

// clamp01 bounds a float to [0,1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

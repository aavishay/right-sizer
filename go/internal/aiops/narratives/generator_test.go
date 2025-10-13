package narrative

import (
	"context"
	"right-sizer/internal/aiops/analyzers"
	"right-sizer/internal/aiops/collector"
	"testing"
	"time"
)

// TestGenerateOOMNarrative ensures mock narrative generation returns non-empty text.
func TestGenerateOOMNarrative(t *testing.T) {
	gen := NewNarrativeGenerator(LLMConfig{})
	ev := collector.OOMEvent{PodName: "pod1", Namespace: "ns", ContainerName: "c1", Timestamp: time.Now()}
	result := &analyzers.AnalysisResult{IsLeak: true, Confidence: 0.85, GrowthRateMBMin: 1.2, DurationMinutes: 30, SampleCount: 25, R2: 0.78, Reasoning: "steady growth"}
	out, err := gen.GenerateOOMNarrative(context.Background(), ev, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out == "" {
		t.Fatalf("expected non-empty narrative")
	}
}

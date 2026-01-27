package scaling

import (
	"right-sizer/memstore"
	"testing"
	"time"
)

func TestNewScalingEngine(t *testing.T) {
	store := memstore.NewMemoryStore(7, 10080)
	engine := NewScalingEngine(store)

	if engine == nil {
		t.Fatal("NewScalingEngine returned nil")
	}
	if engine.confidenceThreshold != 0.70 {
		t.Errorf("confidenceThreshold expected 0.70, got %f", engine.confidenceThreshold)
	}
}

func TestScalingDecisionLogic(t *testing.T) {
	dec := &ScalingDecision{
		CurrentValue:     500,
		RecommendedValue: 400,
		Confidence:       0.75,
	}

	if !dec.ShouldScale() {
		t.Error("ShouldScale should be true")
	}

	percent := dec.ScalePercent()
	expected := ((400.0 - 500.0) / 500.0) * 100
	if percent != expected {
		t.Errorf("ScalePercent expected %.2f, got %.2f", expected, percent)
	}
}

func TestComputeScalingDecision_DownscaleCPU(t *testing.T) {
	store := memstore.NewMemoryStore(7, 10080)
	engine := NewScalingEngine(store)

	now := time.Now()
	for i := 0; i < 800; i++ {
		store.Record("default", "pod1", memstore.DataPoint{Timestamp: now.Add(-time.Duration(i) * time.Minute), CPUMilli: 120})
	}

	decision, err := engine.ComputeScalingDecision("default", "pod1", "app", "cpu", 500)
	if err != nil {
		t.Fatalf("expected decision, got error: %v", err)
	}

	if decision.RecommendedValue >= decision.CurrentValue {
		t.Fatalf("expected downscale recommendation, got %.2f >= %.2f", decision.RecommendedValue, decision.CurrentValue)
	}
	if !decision.ShouldScale() {
		t.Fatalf("decision should trigger scaling")
	}
}

func TestComputeScalingDecision_UpscaleMemory(t *testing.T) {
	store := memstore.NewMemoryStore(7, 10080)
	engine := NewScalingEngine(store)

	now := time.Now()
	for i := 0; i < 800; i++ {
		store.Record("default", "pod1", memstore.DataPoint{Timestamp: now.Add(-time.Duration(i) * time.Minute), MemMB: 900})
	}

	decision, err := engine.ComputeScalingDecision("default", "pod1", "app", "memory", 500)
	if err != nil {
		t.Fatalf("expected decision, got error: %v", err)
	}

	if decision.RecommendedValue <= decision.CurrentValue {
		t.Fatalf("expected upscale recommendation, got %.2f <= %.2f", decision.RecommendedValue, decision.CurrentValue)
	}
	if decision.Confidence < 0.7 {
		t.Fatalf("expected confidence >= 0.7, got %.2f", decision.Confidence)
	}
}

func BenchmarkComputeScalingDecision(b *testing.B) {
	store := memstore.NewMemoryStore(7, 10080)
	engine := NewScalingEngine(store)

	now := time.Now()
	for i := 0; i < 100; i++ {
		timestamp := now.Add(-time.Duration(i) * time.Minute)
		store.Record("default", "pod1", memstore.DataPoint{Timestamp: timestamp, CPUMilli: float64(250 + i%100)})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = engine.ComputeScalingDecision("default", "pod1", "app", "cpu", 500)
	}
}

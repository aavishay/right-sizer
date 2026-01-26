package recommendations

import (
	"right-sizer/memstore"
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1000)
	engine := NewEngine(store)

	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}
	if engine.minCPUUtilization != 0.20 {
		t.Errorf("minCPUUtilization expected 0.20, got %f", engine.minCPUUtilization)
	}
}

func TestGenerateRecommendation_NoHistory(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1000)
	engine := NewEngine(store)

	rec, err := engine.GenerateRecommendation("default", "pod1", "container1", "cpu", 500)
	if err == nil {
		t.Error("expected error for no history")
	}
	if rec != nil {
		t.Error("expected nil recommendation")
	}
}

func TestRecommendationExpiration(t *testing.T) {
	rec := &Recommendation{
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}

	if !rec.IsExpired() {
		t.Error("Recommendation should be expired")
	}
}

func TestPriorityLabel(t *testing.T) {
	tests := []struct {
		priority int
		expected string
	}{
		{1, "CRITICAL"},
		{2, "HIGH"},
		{3, "MEDIUM"},
		{4, "LOW"},
	}

	for _, test := range tests {
		rec := &Recommendation{Priority: test.priority}
		if rec.GetPriorityLabel() != test.expected {
			t.Errorf("Priority %d: expected %s, got %s", test.priority, test.expected, rec.GetPriorityLabel())
		}
	}
}

func BenchmarkGenerateRecommendation(b *testing.B) {
	store := memstore.NewMemoryStore(7, 1000)
	now := time.Now()

	for i := 0; i < 100; i++ {
		timestamp := now.Add(-time.Duration(i) * time.Minute)
		store.Record("default", "pod1", memstore.DataPoint{Timestamp: timestamp, CPUMilli: float64(100 + i%200)})
	}

	engine := NewEngine(store)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = engine.GenerateRecommendation("default", "pod1", "container1", "cpu", 500)
	}
}

func TestGenerateRecommendation_Downscale(t *testing.T) {
	store := memstore.NewMemoryStore(7, 12000)
	now := time.Now()

	for i := 0; i < 6000; i++ {
		store.Record("default", "pod-low", memstore.DataPoint{Timestamp: now.Add(-time.Duration(i) * time.Minute), CPUMilli: 120})
	}

	engine := NewEngine(store)
	rec, err := engine.GenerateRecommendation("default", "pod-low", "c1", "cpu", 700)
	if err != nil {
		t.Fatalf("expected recommendation, got error: %v", err)
	}

	if rec.RecommendedValue >= rec.CurrentValue {
		t.Fatalf("expected downscale recommendation, got %.2f >= %.2f", rec.RecommendedValue, rec.CurrentValue)
	}
	if rec.Confidence < 0.7 {
		t.Fatalf("expected confidence >= 0.7, got %.2f", rec.Confidence)
	}

}

func TestGenerateRecommendation_UpscaleSavingsThreshold(t *testing.T) {
	store := memstore.NewMemoryStore(7, 12000)
	now := time.Now()

	for i := 0; i < 6000; i++ {
		store.Record("default", "pod-high", memstore.DataPoint{Timestamp: now.Add(-time.Duration(i) * time.Minute), MemMB: 900})
	}

	engine := NewEngine(store)
	rec, err := engine.GenerateRecommendation("default", "pod-high", "c1", "memory", 500)
	if err == nil {
		t.Fatalf("expected savings threshold error, got recommendation: %+v", rec)
	}
	if rec != nil {
		t.Fatalf("expected nil recommendation when savings below threshold")
	}
}

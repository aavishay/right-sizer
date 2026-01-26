package whatif

import (
	"right-sizer/memstore"
	"testing"
	"time"
)

func TestNewSimulator(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1000)
	sim := NewSimulator(store)

	if sim == nil {
		t.Fatal("NewSimulator returned nil")
	}
	if sim.riskThresholdMedium != 0.70 {
		t.Errorf("riskThresholdMedium expected 0.70, got %f", sim.riskThresholdMedium)
	}
}

func TestSimulateScaleUp(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1000)
	now := time.Now()

	for i := 0; i < 60; i++ {
		timestamp := now.Add(-time.Duration(i) * time.Minute)
		store.Record("default", "pod1", memstore.DataPoint{Timestamp: timestamp, CPUMilli: float64(100 + i%50)})
	}

	sim := NewSimulator(store)
	result, err := sim.SimulateScaleUp("default", "pod1", "cpu", 500, 600)

	if err != nil {
		t.Logf("SimulateScaleUp error: %v", err)
	} else if result != nil {
		if result.CurrentAllocation != 500 {
			t.Errorf("CurrentAllocation expected 500, got %f", result.CurrentAllocation)
		}
	}
}

func TestRiskLevelCalculation(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1000)
	sim := NewSimulator(store)

	tests := []struct {
		score    float64
		expected string
	}{
		{0.5, "low"},
		{0.75, "medium"},
		{0.92, "high"},
	}

	for _, test := range tests {
		result := sim.calculateRiskLevel(test.score)
		if result != test.expected {
			t.Errorf("Risk level %.2f: expected %s, got %s", test.score, test.expected, result)
		}
	}
}

func TestSimulateScaleDownLowRisk(t *testing.T) {
	store := memstore.NewMemoryStore(7, 2000)
	now := time.Now()

	for i := 0; i < 500; i++ {
		store.Record("default", "pod1", memstore.DataPoint{Timestamp: now.Add(-time.Duration(i) * time.Minute), MemMB: 200})
	}

	sim := NewSimulator(store)
	result, err := sim.SimulateScaleDown("default", "pod1", "memory", 600, 400)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.RiskLevel != "low" {
		t.Fatalf("expected low risk, got %s", result.RiskLevel)
	}
	if result.Confidence <= 0 {
		t.Fatalf("expected positive confidence, got %.2f", result.Confidence)
	}
}

func TestSimulateScaleUpHighRisk(t *testing.T) {
	store := memstore.NewMemoryStore(7, 2000)
	now := time.Now()

	for i := 0; i < 500; i++ {
		store.Record("default", "pod1", memstore.DataPoint{Timestamp: now.Add(-time.Duration(i) * time.Minute), CPUMilli: 250})
	}

	sim := NewSimulator(store)
	result, err := sim.SimulateScaleUp("default", "pod1", "cpu", 200, 150)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if result.RiskLevel != "high" {
		t.Fatalf("expected high risk, got %s", result.RiskLevel)
	}
}

func BenchmarkSimulateScaleUp(b *testing.B) {
	store := memstore.NewMemoryStore(7, 1000)
	now := time.Now()

	for i := 0; i < 100; i++ {
		timestamp := now.Add(-time.Duration(i) * time.Minute)
		store.Record("default", "pod1", memstore.DataPoint{Timestamp: timestamp, CPUMilli: float64(100 + i%200)})
	}

	sim := NewSimulator(store)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = sim.SimulateScaleUp("default", "pod1", "cpu", 500, 600)
	}
}

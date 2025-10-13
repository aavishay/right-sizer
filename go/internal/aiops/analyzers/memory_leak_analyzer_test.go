package analyzers

import (
	"testing"
)

// mockStore implements minimal HistoricalStore behavior for tests
// TestEvaluateLeakPositive constructs a synthetic regression result to validate evaluateLeak logic.
func TestEvaluateLeakPositive(t *testing.T) {
	a := &MemoryAnalyzer{cfg: DefaultLeakDetectionConfig()}
	reg := LinearRegressionResult{
		Slope:         a.cfg.MinSlopeMBPerMinute * 2, // above min threshold
		DurationMins:  a.cfg.MinWindow.Minutes() * 1.5,
		Points:        a.cfg.MinSamples + 5,
		R2:            a.cfg.R2Threshold + 0.2,
		PositiveTrend: true,
	}
	isLeak, conf, reasoning := a.evaluateLeak(reg)
	if !isLeak {
		t.Fatalf("expected leak true; got false (conf=%.2f, reasoning=%s)", conf, reasoning)
	}
	if conf <= 0 {
		t.Fatalf("confidence should be >0")
	}
}

// TestEvaluateLeakLowSlope ensures low slope returns non-leak with reduced confidence.
func TestEvaluateLeakLowSlope(t *testing.T) {
	a := &MemoryAnalyzer{cfg: DefaultLeakDetectionConfig()}
	reg := LinearRegressionResult{
		Slope:         a.cfg.MinSlopeMBPerMinute / 4, // below threshold
		DurationMins:  a.cfg.MinWindow.Minutes() * 2,
		Points:        a.cfg.MinSamples + 5,
		R2:            a.cfg.R2Threshold + 0.3,
		PositiveTrend: true,
	}
	isLeak, conf, reasoning := a.evaluateLeak(reg)
	if isLeak {
		t.Fatalf("expected non-leak due to low slope; reasoning=%s", reasoning)
	}
	if conf <= 0 {
		t.Fatalf("confidence should still be >0 even for non-leak")
	}
}

package aiops

import (
	"context"
	"testing"
	"time"

	narrative "right-sizer/internal/aiops/narratives"
	"right-sizer/metrics"
)

// mockMetricsProvider provides deterministic pod metrics
type mockMetricsProvider struct{}

func (m *mockMetricsProvider) FetchPodMetrics(ctx context.Context, namespace, podName string) (metrics.Metrics, error) {
	return metrics.Metrics{CPUMilli: 150, MemMB: 300}, nil
}

// TestEngineStartStop ensures the AIOps engine starts goroutines without panic and stops cleanly.
func TestEngineStartStop(t *testing.T) {
	engine := NewEngine(nil, &mockMetricsProvider{}, narrative.LLMConfig{})
	// Disable OOM listener for unit test (no Kubernetes client available)
	engine.oomListener = nil

	ctx, cancel := context.WithCancel(context.Background())
	go engine.Start(ctx)
	// Allow some time for goroutines to spin up
	time.Sleep(50 * time.Millisecond)
	// Cancel and ensure stop does not block
	cancel()
	// Give shutdown time
	time.Sleep(30 * time.Millisecond)
}

// TestIncidentStoreUpsertList exercises basic incident store lifecycle operations
func TestIncidentStoreUpsertList(t *testing.T) {
	bus := NewInMemoryBus()
	store := NewIncidentStore(DefaultIncidentStoreConfig(), bus)
	defer store.Stop()

	incID := GenerateIncidentID("memory")
	template := NewIncident(incID, IncidentMemoryLeak, SeverityWarning, "default/p1/container1")
	status := StatusAnalyzing
	evidence := []Evidence{{ID: "e1", Category: "METRIC", Description: "spike", Confidence: 0.7, Timestamp: time.Now()}}
	narrative := &Narrative{Title: "Leak", Summary: "probable leak", FullText: "probable leak", Confidence: 0.7}

	updated, created := store.UpsertIncident(template, evidence, narrative, &status)
	if !created {
		t.Fatalf("expected incident creation")
	}
	if updated.Status != StatusAnalyzing {
		t.Fatalf("expected status analyzing, got %s", updated.Status)
	}

	// List incidents
	list := store.List(IncidentFilter{})
	if len(list) == 0 {
		t.Fatalf("expected at least one incident")
	}
}

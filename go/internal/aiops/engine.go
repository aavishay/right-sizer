package aiops // TODO: eventually move to internal/aiops/engine (keep package for existing references)

import (
	"context"
	"log" // TODO: replace with structured logger
	"time"

	"right-sizer/internal/aiops/analyzers"
	"right-sizer/internal/aiops/collector"
	"right-sizer/internal/aiops/core"
	narrative "right-sizer/internal/aiops/narratives"
	"right-sizer/metrics"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Engine orchestrates the AIOps pipeline: collecting signals, running analyzers,
// persisting incidents, and generating narratives.
type Engine struct {
	// Legacy components (still used for OOM event collection / regression)
	oomListener    *collector.OOMListener
	memoryAnalyzer *analyzers.MemoryAnalyzer
	narrativeGen   *narrative.NarrativeGenerator

	// Channels
	oomEventChan chan collector.OOMEvent

	// Core infrastructure
	bus           core.Bus
	incidentStore *IncidentStore
	analyzers     []core.Analyzer

	// External deps
	clientset       kubernetes.Interface
	metricsProvider metrics.Provider

	// Sampling configuration
	sampleInterval time.Duration

	// Control
	stopCh chan struct{}
}

// NewEngine creates and configures a new AIOps Engine with in‑memory bus & incident store.
func NewEngine(clientset kubernetes.Interface, metricsProvider metrics.Provider, llmConfig narrative.LLMConfig) *Engine {
	oomEventChan := make(chan collector.OOMEvent, 100)

	// Core components
	bus := NewInMemoryBus()
	store := NewIncidentStore(DefaultIncidentStoreConfig(), bus)

	memAnalyzer := analyzers.NewMemoryAnalyzer(metricsProvider)
	adapter := analyzers.NewMemoryLeakAnalyzerAdapter(memAnalyzer, 0.60) // threshold confidence

	e := &Engine{
		oomListener:     collector.NewOOMListener(clientset, oomEventChan),
		memoryAnalyzer:  memAnalyzer,
		narrativeGen:    narrative.NewNarrativeGenerator(llmConfig),
		oomEventChan:    oomEventChan,
		clientset:       clientset,
		metricsProvider: metricsProvider,
		bus:             bus,
		incidentStore:   store,
		analyzers:       []core.Analyzer{adapter},
		sampleInterval:  30 * time.Second,
		stopCh:          make(chan struct{}),
	}

	// Register analyzer subscription(s) on the bus.
	for _, an := range e.analyzers {
		types := an.InterestedIn()
		id := "analyzer-" + an.Name()
		_ = e.bus.Subscribe(id, types, e.makeAnalyzerDispatch(an))
	}

	return e
}

// makeAnalyzerDispatch returns a core.Handler that executes analyzer logic for a signal.
func (e *Engine) makeAnalyzerDispatch(an core.Analyzer) core.Handler {
	return func(sig core.Signal) {
		finding, evidence, err := an.Handle(sig, nil) // nil historical accessor for now
		if err != nil {
			log.Printf("[AIOPS] analyzer=%s error=%v", an.Name(), err)
			return
		}
		if finding == nil && len(evidence) == 0 {
			return
		}

		// Always record evidence (attach to an incident if we later correlate).
		if finding != nil {
			// Create or update an incident for the resource scope (CorrelationKey)
			incID := GenerateIncidentID(an.Name())
			inc := NewIncident(incID, IncidentMemoryLeak, SeverityWarning, sig.CorrelationKey)

			// Convert evidence slice (core -> alias) & append
			evCopy := make([]Evidence, 0, len(evidence))
			for _, ev := range evidence {
				evCopy = append(evCopy, ev)
			}

			// Basic narrative (deterministic stub)
			n := &Narrative{
				Title:      "Probable Memory Leak Detected",
				Summary:    finding.NarrativeHint,
				FullText:   finding.NarrativeHint,
				Confidence: finding.Confidence,
			}

			status := StatusAnalyzing
			e.incidentStore.UpsertIncident(inc, evCopy, n, &status)
			return
		}

		// If only evidence (no finding), attach to a generic rolling incident (optional future enhancement).
		if len(evidence) > 0 {
			// For now we log evidence only.
			log.Printf("[AIOPS] analyzer=%s evidence_only count=%d", an.Name(), len(evidence))
		}
	}
}

// Start runs the AIOps engine until context cancellation.
func (e *Engine) Start(ctx context.Context) {
	log.Println("[AIOPS] Engine starting (OOM listener + bus dispatch + sampler)...")
	go e.oomListener.Start(ctx)
	go e.eventIngestLoop(ctx)
	go e.samplingLoop(ctx)
	<-ctx.Done()
	close(e.stopCh)
	log.Println("[AIOPS] Engine stopped.")
}

// samplingLoop periodically samples pod metrics and records them for leak analysis.
func (e *Engine) samplingLoop(ctx context.Context) {
	if e.metricsProvider == nil {
		log.Println("[AIOPS] sampler disabled (no metrics provider)")
		return
	}
	ticker := time.NewTicker(e.sampleInterval)
	defer ticker.Stop()
	log.Printf("[AIOPS] sampler started interval=%s", e.sampleInterval)
	for {
		select {
		case <-ticker.C:
			e.sampleAllPods(ctx)
		case <-ctx.Done():
			log.Println("[AIOPS] sampler stopping (context canceled)")
			return
		case <-e.stopCh:
			log.Println("[AIOPS] sampler stopping (engine stop)")
			return
		}
	}
}

// sampleAllPods lists pods cluster-wide and adds per-container samples.
func (e *Engine) sampleAllPods(ctx context.Context) {
	pods, err := e.clientset.CoreV1().Pods("").List(ctx, v1.ListOptions{})
	if err != nil {
		log.Printf("[AIOPS] sampler list pods error: %v", err)
		return
	}
	now := time.Now().UTC()
	for _, pod := range pods.Items {
		// Skip succeeded / failed pods
		if pod.Status.Phase == "Succeeded" || pod.Status.Phase == "Failed" {
			continue
		}
		metricsData, mErr := e.metricsProvider.FetchPodMetrics(pod.Namespace, pod.Name)
		if mErr != nil {
			continue // quiet skip
		}
		for _, c := range pod.Spec.Containers {
			e.memoryAnalyzer.AddSample(
				pod.Namespace,
				pod.Name,
				c.Name,
				metricsData.CPUMilli,
				metricsData.MemMB,
				now,
			)
		}
	}
}

// eventIngestLoop consumes raw OOM events and publishes normalized signals on the bus.
func (e *Engine) eventIngestLoop(ctx context.Context) {
	for {
		select {
		case ev := <-e.oomEventChan:
			e.publishOOMSignal(ev)
			// Additionally perform legacy regression for now (until adapter does inline regression).
			e.legacyRegression(ctx, ev)
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		}
	}
}

// publishOOMSignal converts an OOMEvent into a core.Signal and publishes it.
func (e *Engine) publishOOMSignal(ev collector.OOMEvent) {
	payload := map[string]any{
		"Namespace":     ev.Namespace,
		"PodName":       ev.PodName,
		"ContainerName": ev.ContainerName,
		"Timestamp":     ev.Timestamp,
	}
	sig := core.Signal{
		Type:           core.SignalOOMKilled,
		Timestamp:      time.Now().UTC(),
		CorrelationKey: ev.Namespace + "/" + ev.PodName + "/" + ev.ContainerName,
		Payload:        payload,
	}
	e.bus.Publish(sig)
}

// legacyRegression keeps the existing regression path for transitional period.
func (e *Engine) legacyRegression(ctx context.Context, ev collector.OOMEvent) {
	result, err := e.memoryAnalyzer.AnalyzeForOOMEvent(ev)
	if err != nil {
		log.Printf("[AIOPS] legacy regression error pod=%s err=%v", ev.PodName, err)
		return
	}
	if !result.IsLeak {
		log.Printf("[AIOPS] legacy regression: no leak (pod=%s slope=%.3f r2=%.2f)", ev.PodName, result.GrowthRateMBMin, result.R2)
		return
	}
	narr, err := e.narrativeGen.GenerateOOMNarrative(ctx, ev, result)
	if err != nil {
		log.Printf("[AIOPS] narrative generation error pod=%s err=%v", ev.PodName, err)
		return
	}
	log.Println("======================================================================")
	log.Printf("[AIOPS] Legacy RCA Report (pod=%s)", ev.PodName)
	log.Println("----------------------------------------------------------------------")
	log.Println(narr)
	log.Println("======================================================================")
}

// IncidentStore exposes the engine's incident store (read-only usage).
func (e *Engine) IncidentStore() *IncidentStore {
	return e.incidentStore
}

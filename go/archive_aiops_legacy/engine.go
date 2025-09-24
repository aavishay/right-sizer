//go:build legacy_aiops_unused

package legacyaiops

import (
	"context"
	"log"

	"right-sizer/internal/aiops/analyzers"
	"right-sizer/internal/aiops/collector"
	narrative "right-sizer/internal/aiops/narratives"
	"right-sizer/metrics"

	"k8s.io/client-go/kubernetes"
)

// Engine orchestrates the AIOps pipeline: collecting events, analyzing them, and generating narratives.
type Engine struct {
	oomListener     *collector.OOMListener
	memoryAnalyzer  *analyzers.MemoryAnalyzer
	narrativeGen    *narrative.NarrativeGenerator
	oomEventChan    chan collector.OOMEvent
	clientset       kubernetes.Interface
	metricsProvider metrics.Provider
}

// NewEngine creates and configures a new AIOps Engine.
func NewEngine(clientset kubernetes.Interface, metricsProvider metrics.Provider, llmConfig narrative.LLMConfig) *Engine {
	oomEventChan := make(chan collector.OOMEvent, 100) // Buffered channel

	return &Engine{
		oomListener:     collector.NewOOMListener(clientset, oomEventChan),
		memoryAnalyzer:  analyzers.NewMemoryAnalyzer(metricsProvider),
		narrativeGen:    narrative.NewNarrativeGenerator(llmConfig),
		oomEventChan:    oomEventChan,
		clientset:       clientset,
		metricsProvider: metricsProvider,
	}
}

// Start runs the AIOps engine. It starts the event listener and begins processing events.
// This function will block until the context is canceled.
func (e *Engine) Start(ctx context.Context) {
	log.Println("Starting AIOps Engine...")

	// Start listening for OOMKilled events in the background.
	go e.oomListener.Start(ctx)

	// Background sampler already started (startSamplingLoop)

	// Start the main processing loop.
	e.processEvents(ctx)

	log.Println("AIOps Engine stopped.")
}

// processEvents is the main loop that waits for events and triggers the RCA pipeline.
func (e *Engine) processEvents(ctx context.Context) {
	log.Println("AIOps event processor started. Waiting for OOM events...")
	for {
		select {
		case oomEvent := <-e.oomEventChan:
			// When an OOM event is received, process it in a separate goroutine
			// to avoid blocking the event channel.
			go e.handleOOMEvent(ctx, oomEvent)
		case <-ctx.Done():
			// If the context is canceled, stop processing.
			log.Println("Shutting down AIOps event processor.")
			return
		}
	}
}

// handleOOMEvent orchestrates the analysis and narrative generation for a single OOM event.
func (e *Engine) handleOOMEvent(ctx context.Context, event collector.OOMEvent) {
	log.Printf("AIOps Pipeline Started: Handling OOM event for Pod: %s, Container: %s", event.PodName, event.ContainerName)

	// Step 1: Analyze the event to find the root cause.
	analysisResult, err := e.memoryAnalyzer.AnalyzeForOOMEvent(event)
	if err != nil {
		log.Printf("Error during memory analysis for pod %s: %v", event.PodName, err)
		return
	}

	log.Printf("Analysis complete for pod %s. IsLeak: %t, Confidence: %.2f", event.PodName, analysisResult.IsLeak, analysisResult.Confidence)

	// Step 2: If the analysis found something significant, generate a narrative.
	if analysisResult.IsLeak {
		narrative, err := e.narrativeGen.GenerateOOMNarrative(ctx, event, analysisResult)
		if err != nil {
			log.Printf("Error generating narrative for pod %s: %v", event.PodName, err)
			return
		}

		// Step 3: Display the final story.
		// In a real system, this would be sent to a dashboard, an alert, or a Slack channel.
		// For now, we just log it.
		log.Println("======================================================================")
		log.Printf("AIOps Root Cause Analysis Report for Pod: %s", event.PodName)
		log.Println("----------------------------------------------------------------------")
		log.Println(narrative)
		log.Println("======================================================================")
	}
}

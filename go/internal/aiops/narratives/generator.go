package narrative

import (
	"context"
	"fmt"

	"right-sizer/internal/aiops/analyzers"
	"right-sizer/internal/aiops/collector"
)

// LLMConfig holds the configuration for a Large Language Model provider.
type LLMConfig struct {
	APIKey    string
	APIURL    string
	ModelName string
}

// NarrativeGenerator generates human-readable RCA narratives.
type NarrativeGenerator struct {
	config LLMConfig
}

// NewNarrativeGenerator creates a new NarrativeGenerator.
func NewNarrativeGenerator(config LLMConfig) *NarrativeGenerator {
	return &NarrativeGenerator{config: config}
}

// GenerateOOMNarrative creates a human-readable story for an OOM event.
func (g *NarrativeGenerator) GenerateOOMNarrative(ctx context.Context, event collector.OOMEvent, result *analyzers.AnalysisResult) (string, error) {
	prompt := g.buildOOMPrompt(event, result)

	// Placeholder for real LLM integration.
	mockNarrative := fmt.Sprintf(
		"ROOT CAUSE ANALYSIS (Mock Narrative)\n\nPod: %s\nContainer: %s\nEvent: OOMKilled\n\nAssessment: Probable memory leak.\nConfidence: %.0f%%\nObserved Growth Rate: %.2f MB/min over %.1f minutes (R²=%.2f, samples=%d)\nAnalysis Reasoning: %s\n\nStory:\nThe %s container in pod %s was terminated after exhausting its memory allocation. Historical analysis of memory usage shows a persistent upward linear growth (%.2f MB/min) sustained over roughly %.1f minutes with correlation strength R²=%.2f. This pattern is consistent with a memory leak rather than a transient usage spike. %s\n\nRecommended Actions:\n1. Inspect recent code changes deploying this container.\n2. Capture heap profiles under similar load.\n3. Add runtime memory instrumentation (pprof / allocation tracing).\n4. Set temporary memory request/limit buffers to prevent immediate recurrence while investigating.\n",
		event.PodName,
		event.ContainerName,
		result.Confidence*100,
		result.GrowthRateMBMin,
		result.DurationMinutes,
		result.R2,
		result.SampleCount,
		result.Reasoning,
		event.ContainerName,
		event.PodName,
		result.GrowthRateMBMin,
		result.DurationMinutes,
		result.R2,
		conditionalCausal(result.CausalEvent),
	)

	fmt.Println("--- LLM PROMPT (mock) ---")
	fmt.Println(prompt)
	fmt.Println("-------------------------")

	return mockNarrative, nil
}

// buildOOMPrompt constructs the LLM prompt with enriched analysis context.
func (g *NarrativeGenerator) buildOOMPrompt(event collector.OOMEvent, result *analyzers.AnalysisResult) string {
	return fmt.Sprintf(
		"ROLE: Senior Kubernetes Site Reliability Engineer\n"+
			"TASK: Produce a clear, developer-friendly Root Cause Analysis for an OOMKilled event.\n\n"+
			"EVENT CONTEXT:\n"+
			"- Pod: %s\n"+
			"- Container: %s\n"+
			"- Event: OOMKilled\n"+
			"- Time Window Analyzed: %.1f minutes prior to event\n"+
			"- Samples: %d\n\n"+
			"ANALYSIS SIGNALS:\n"+
			"- Detected Growth Rate: %.3f MB/min\n"+
			"- Linear Fit R²: %.3f\n"+
			"- Leak Classification: %v\n"+
			"- Confidence: %.0f%%\n"+
			"- Analyzer Reasoning: %s\n"+
			"- Potential Causal Deployment: %s\n\n"+
			"OUTPUT REQUIREMENTS:\n"+
			"1. Begin with a concise summary.\n"+
			"2. Explain why this is likely a leak (not a burst load).\n"+
			"3. Reference the quantitative evidence (growth rate, duration, R², samples).\n"+
			"4. Provide 3-5 actionable remediation steps.\n"+
			"5. Keep it under 250 words.\n"+
			"6. Avoid speculation beyond provided evidence.\n\n"+
			"Generate the narrative now.",
		event.PodName,
		event.ContainerName,
		result.DurationMinutes,
		result.SampleCount,
		result.GrowthRateMBMin,
		result.R2,
		result.IsLeak,
		result.Confidence*100,
		result.Reasoning,
		emptyFallback(result.CausalEvent, "Not correlated (deployment linkage pending)"),
	)
}

// conditionalCausal formats causal event if present.
func conditionalCausal(c string) string {
	if c == "" {
		return "No deployment correlation established yet."
	}
	return "Likely triggered by: " + c
}

// emptyFallback returns fallback if s is empty.
func emptyFallback(s, fb string) string {
	if s == "" {
		return fb
	}
	return s
}

// callLLM is a placeholder for real LLM API integration.
func (g *NarrativeGenerator) callLLM(ctx context.Context, prompt string) (string, error) {
	return "LLM integration not yet implemented.", nil
}

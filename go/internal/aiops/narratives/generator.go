package narrative

import (
	"context"
	"fmt"
	"strings"

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

// GenerateGeneralIncidentNarrative creates a human-readable story for any cluster incident.
func (g *NarrativeGenerator) GenerateGeneralIncidentNarrative(ctx context.Context, incidentType string, message string, namespace string, podName string, logs string, events []collector.K8sEvent) (string, error) {
	prompt := g.buildGeneralPrompt(incidentType, message, namespace, podName, logs, events)

	// Placeholder for real LLM integration.
	mockNarrative := fmt.Sprintf(
		"ROOT CAUSE ANALYSIS (Mock General Narrative)\n\nIncident: %s\nPod: %s/%s\n\nAI Summary:\nThe container is experiencing %s failures. Initial message: \"%s\".\n\nLog Analysis:\nAnalysis of the last 50 lines of logs shows potential issues. (Snippet: %s...)\n\nEvent Analysis:\n%d related Kubernetes events were found, indicating persistent issues.\n\nRecommended Actions:\n1. Check the logs for specific error codes.\n2. Verify resource constraints.\n3. Review recent code or config changes.\n",
		incidentType, namespace, podName, incidentType, message, truncateLogs(logs, 100), len(events),
	)

	fmt.Println("--- LLM GENERAL PROMPT (mock) ---")
	fmt.Println(prompt)
	fmt.Println("-------------------------")

	return mockNarrative, nil
}

func truncateLogs(logs string, n int) string {
	if len(logs) <= n {
		return logs
	}
	return logs[:n]
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

// buildGeneralPrompt constructs the LLM prompt for general incidents.
func (g *NarrativeGenerator) buildGeneralPrompt(incidentType string, message string, namespace string, podName string, logs string, events []collector.K8sEvent) string {
	eventSummary := ""
	for _, e := range events {
		eventSummary += fmt.Sprintf("- [%s] %s: %s (count: %d)\n", e.Type, e.Reason, e.Message, e.Count)
	}

	return fmt.Sprintf(
		"ROLE: Senior Kubernetes Site Reliability Engineer\n"+
			"TASK: Analyze a Kubernetes incident and provide a root cause summary.\n\n"+
			"INCIDENT CONTEXT:\n"+
			"- Type: %s\n"+
			"- Message: %s\n"+
			"- Resource: %s/%s\n\n"+
			"LOG SNIPPETS (Last 50 lines):\n%s\n\n"+
			"RELATED EVENTS:\n%s\n\n"+
			"OUTPUT REQUIREMENTS:\n"+
			"1. Explain what happened based on the logs and events.\n"+
			"2. Identify the likely root cause.\n"+
			"3. provide 3 actionable remediation steps.\n"+
			"4. Keep it concise and technical.\n\n"+
			"Generate the analysis now.",
		incidentType, message, namespace, podName, logs, eventSummary,
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

// GenerateHealthSnapshotSummary generates a high-level summary of cluster health.
func (g *NarrativeGenerator) GenerateHealthSnapshotSummary(ctx context.Context, activeIncidents, recentIncidents int, resourceUsage map[string]float64) (string, int, error) {
	if g.config.APIKey == "" {
		return "AI Analysis disabled (no API key). Cluster seems operational.", 100, nil
	}

	// Calculate a naive score first
	score := 100
	score -= activeIncidents * 10
	score -= (recentIncidents - activeIncidents) * 2
	if score < 0 {
		score = 0
	}

	prompt := fmt.Sprintf("Analyze the following Kubernetes cluster health data and provide a concise (2-3 sentences) executive summary.\nActive Incidents: %d\nRecent Incidents (24h): %d\nResource Usage: %v\n\nCurrent Health Score: %d/100\n\nFormat the output as a plain text summary.", activeIncidents, recentIncidents, resourceUsage, score)

	summary, err := g.callLLM(ctx, prompt)
	if err != nil {
		return "", score, err
	}

	return strings.TrimSpace(summary), score, nil
}

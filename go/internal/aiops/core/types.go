package core

import (
	"time"
)

/*
Package core defines the foundational, dependency–light building blocks for the AIOps
subsystem. These types are intentionally isolated from higher‑level packages (engine,
analyzers, incident store, API server) to prevent import cycles.

Layers (intended usage):
  core        -> (analyzers, engine, incident store, collectors, API)
  analyzers   -> core
  collectors  -> core
  engine      -> core + analyzers + incident store
  api         -> core (+ incident store models for serialization)

No code in this package should import any sibling AIOps subpackages.
*/

// ==============================
// Signal & Event Foundations
// ==============================

// SignalType enumerates the categories of internal signals emitted on the AIOps event bus.
// Each signal is a normalized abstraction over raw Kubernetes or system events.
type SignalType string

const (
	SignalOOMKilled         SignalType = "OOM_KILLED"
	SignalPodMetrics        SignalType = "POD_METRICS"
	SignalDeploymentRollout SignalType = "DEPLOYMENT_ROLLOUT"
	SignalPVUsageSample     SignalType = "PV_USAGE_SAMPLE"
	SignalNetworkErrorSpike SignalType = "NETWORK_ERROR_SPIKE"
	SignalAnalyzerFinding   SignalType = "ANALYZER_FINDING"
	SignalIncidentUpdated   SignalType = "INCIDENT_UPDATED"
	SignalGenericIncident   SignalType = "GENERIC_INCIDENT"
)

// Signal is the transport envelope published on the internal bus.
type Signal struct {
	Type           SignalType `json:"type"`
	Timestamp      time.Time  `json:"timestamp"`
	CorrelationKey string     `json:"correlationKey,omitempty"` // namespace/pod/container or logical key
	// Payload must be JSON‑serializable if forwarded externally.
	// Concrete producers decide the shape. Common payloads should migrate to typed structs in core.
	Payload any `json:"payload,omitempty"`
}

// ==============================
// Incident & Finding Semantics (Core Layer)
// ==============================

// IncidentType defines the high-level classification of an operational issue.
type IncidentType string

const (
	IncidentMemoryLeak         IncidentType = "MEMORY_LEAK"
	IncidentDiskSaturation     IncidentType = "DISK_SATURATION"
	IncidentCPUStarvation      IncidentType = "CPU_STARVATION"
	IncidentNetworkPolicyBlock IncidentType = "NETWORK_POLICY_BLOCK"
	IncidentConfigRegression   IncidentType = "CONFIG_REGRESSION"
	IncidentNoisyNeighbor      IncidentType = "NOISY_NEIGHBOR"
)

// IncidentStatus models lifecycle progression of an incident investigation.
type IncidentStatus string

const (
	StatusDetected    IncidentStatus = "DETECTED"
	StatusAnalyzing   IncidentStatus = "ANALYZING"
	StatusCorrelating IncidentStatus = "CORRELATING"
	StatusExplained   IncidentStatus = "EXPLAINED"
	StatusResolved    IncidentStatus = "RESOLVED"
)

// Severity expresses user impact level.
type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityWarning  Severity = "WARNING"
	SeverityCritical Severity = "CRITICAL"
)

// Evidence represents an atomic fact or measurement contributing to an Incident hypothesis.
type Evidence struct {
	ID          string         `json:"id"`
	Category    string         `json:"category"`             // e.g. METRIC, EVENT, ANALYSIS, CORRELATION
	Description string         `json:"description"`          // human-readable sentence
	Confidence  float64        `json:"confidence,omitempty"` // 0..1 subjective weight
	Data        map[string]any `json:"data,omitempty"`       // structured supporting details
	Timestamp   time.Time      `json:"timestamp"`            // when this evidence was observed / generated
}

// PartialFinding is an intermediate hypothesis emitted by an Analyzer before a full Incident is finalized.
type PartialFinding struct {
	Kind          string         `json:"kind"`                    // e.g. LEAK_SIGNATURE
	Confidence    float64        `json:"confidence"`              // 0..1
	NarrativeHint string         `json:"narrativeHint,omitempty"` // short phrase guiding narrative generation
	Attributes    map[string]any `json:"attributes,omitempty"`    // structured fields (slope, r2, etc.)
}

// ==============================
// Analyzer Contracts
// ==============================

// Analyzer defines the contract for pluggable analysis modules.
// Implementations should be cheap to invoke and concurrency safe.
type Analyzer interface {
	Name() string
	// InterestedIn returns the list of SignalTypes this analyzer wants to receive.
	InterestedIn() []SignalType
	// Handle processes a Signal and optionally returns:
	//  - a PartialFinding (nil if no new hypothesis)
	//  - zero or more Evidence records (may be nil/empty)
	//  - an error if processing failed
	Handle(signal Signal, access HistoricalAccessor) (finding *PartialFinding, evidence []Evidence, err error)
}

// HistoricalAccessor abstracts time-series queries needed by analyzers.
// Implementations are provided by upper layers (e.g., engine) and may source
// data from in-memory stores or external metrics backends.
type HistoricalAccessor interface {
	MemorySeries(namespace, pod, container string, since time.Time) ([]SamplePoint, error)
	CPUSeries(namespace, pod, container string, since time.Time) ([]SamplePoint, error)
	// Series provides a generic fallback for future extensibility.
	Series(kind string, key string, since time.Time) ([]SamplePoint, error)
}

// SamplePoint represents a single time-value data point in a numeric series.
type SamplePoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

// ==============================
// Event Bus Abstraction
// ==============================

// Handler processes a published Signal.
type Handler func(Signal)

// Bus is a minimal pub/sub abstraction. Concrete implementations (e.g., in-memory)
// must be concurrency safe. Downstream we can replace this with a streaming system (NATS/Kafka)
// without modifying analyzers.
type Bus interface {
	Publish(s Signal)
	Subscribe(id string, types []SignalType, h Handler) error
	Unsubscribe(id string)
}

// ==============================
// Utility Helpers
// ==============================

// Clamp01 bounds a float value into [0,1].
func Clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// SeverityRank provides a sortable numeric ranking (higher = more severe).
func SeverityRank(s Severity) int {
	switch s {
	case SeverityCritical:
		return 3
	case SeverityWarning:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}

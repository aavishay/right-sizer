package aiops

import (
	"fmt"
	"sync"
	"time"

	"right-sizer/internal/aiops/core"
)

// NOTE: Core signal, analyzer, evidence, and bus types have been moved to
// internal/aiops/core to prevent import cycles. This file now only retains
// incident-specific structures and helpers that build on those core types.

// (SignalType moved to core)

// (Signal constants moved to core)

// (Signal struct moved to core)

// ==============================
// Incident Domain Model
// ==============================

// IncidentType higher-level classification for human & API consumers (kept here to keep Incident self-contained).
type IncidentType string

const (
	IncidentMemoryLeak         IncidentType = "MEMORY_LEAK"
	IncidentDiskSaturation     IncidentType = "DISK_SATURATION"
	IncidentCPUStarvation      IncidentType = "CPU_STARVATION"
	IncidentNetworkPolicyBlock IncidentType = "NETWORK_POLICY_BLOCK"
	IncidentConfigRegression   IncidentType = "CONFIG_REGRESSION"
	IncidentNoisyNeighbor      IncidentType = "NOISY_NEIGHBOR"
)

// IncidentStatus represents lifecycle progression.
type IncidentStatus string

const (
	StatusDetected    IncidentStatus = "DETECTED"
	StatusAnalyzing   IncidentStatus = "ANALYZING"
	StatusCorrelating IncidentStatus = "CORRELATING"
	StatusExplained   IncidentStatus = "EXPLAINED"
	StatusResolved    IncidentStatus = "RESOLVED"
)

// Severity represents user-impact level.
type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityWarning  Severity = "WARNING"
	SeverityCritical Severity = "CRITICAL"
)

// Evidence now uses core.Evidence; local duplicate removed.
type Evidence = core.Evidence

// PartialFinding now aliases core.PartialFinding.
type PartialFinding = core.PartialFinding

// Narrative is a human-focused explanation that can also drive voice synthesis later.
type Narrative struct {
	Title           string
	Summary         string
	FullText        string
	Confidence      float64
	CauseHypothesis string
	Impact          string
	Recommendations []string
	FollowUps       []string
	GeneratedAt     time.Time
}

// Incident aggregates evidence & explanation.
type Incident struct {
	ID              string
	Type            IncidentType
	Status          IncidentStatus
	Severity        Severity
	FirstSeen       time.Time
	LastUpdated     time.Time
	PrimaryResource string // canonical key: namespace/pod(/container) or other identifier
	InitialMessage  string // The original message from the source event

	Evidence  []core.Evidence
	Narrative *Narrative

	// Internal control fields (not exposed externally by default)
	analysisMeta map[string]any
	mutex        sync.Mutex
}

// AppendEvidence adds new evidence (thread-safe) and updates timestamps.
func (i *Incident) AppendEvidence(ev Evidence) {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	i.Evidence = append(i.Evidence, ev)
	now := time.Now()
	i.LastUpdated = now
	if i.FirstSeen.IsZero() {
		i.FirstSeen = now
	}
}

// SetNarrative sets final or intermediate narrative.
func (i *Incident) SetNarrative(n *Narrative) {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	i.Narrative = n
	i.LastUpdated = time.Now()
}

// Transition updates status in a controlled manner.
func (i *Incident) Transition(newStatus IncidentStatus) {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	i.Status = newStatus
	i.LastUpdated = time.Now()
}

// ==============================
// Analyzer Interfaces
// ==============================

// (Analyzer interface moved to core)

// (HistoricalAccessor moved to core)

// (SamplePoint moved to core)

// ==============================
// Event Bus (In-Process)
// ==============================

// (Handler moved to core)

// (Bus interface moved to core)

// InMemoryBus implements core.Bus using core.Signal.
type InMemoryBus struct {
	mutex       sync.RWMutex
	subscribers map[string]*subscription
}

type subscription struct {
	types   map[core.SignalType]struct{}
	handler core.Handler
}

// NewInMemoryBus creates a new core-compatible bus.
func NewInMemoryBus() *InMemoryBus {
	return &InMemoryBus{
		subscribers: make(map[string]*subscription),
	}
}

// Subscribe registers a handler for given signal types (core.SignalType).
func (b *InMemoryBus) Subscribe(id string, types []core.SignalType, h core.Handler) error {
	if h == nil {
		return fmt.Errorf("handler cannot be nil")
	}
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if _, exists := b.subscribers[id]; exists {
		return fmt.Errorf("subscriber id already exists: %s", id)
	}
	typeSet := make(map[core.SignalType]struct{})
	for _, t := range types {
		typeSet[t] = struct{}{}
	}
	b.subscribers[id] = &subscription{
		types:   typeSet,
		handler: h,
	}
	return nil
}

// Unsubscribe removes a subscription by id.
func (b *InMemoryBus) Unsubscribe(id string) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	delete(b.subscribers, id)
}

// Publish fan-outs the signal to matching subscribers.
func (b *InMemoryBus) Publish(s core.Signal) {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	for _, sub := range b.subscribers {
		if len(sub.types) == 0 {
			go safeInvoke(sub.handler, s)
			continue
		}
		if _, ok := sub.types[s.Type]; ok {
			go safeInvoke(sub.handler, s)
		}
	}
}

func safeInvoke(h core.Handler, s core.Signal) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Recovered panic in subscriber handler: %v\\n", r)
		}
	}()
	h(s)
}

// ==============================
// Utility Helpers
// ==============================

// NewIncident scaffolds a new Incident with defaults.
func NewIncident(id string, t IncidentType, severity Severity, resource string) *Incident {
	now := time.Now()
	return &Incident{
		ID:              id,
		Type:            t,
		Severity:        severity,
		Status:          StatusDetected,
		PrimaryResource: resource,
		FirstSeen:       now,
		LastUpdated:     now,
		Evidence:        []Evidence{},
		analysisMeta:    make(map[string]any),
	}
}

// GenerateIncidentID naive ID generator (placeholder).
// TODO: Replace with ULID or snowflake-style id for ordering.
func GenerateIncidentID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

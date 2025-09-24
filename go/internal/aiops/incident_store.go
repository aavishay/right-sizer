package aiops

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"right-sizer/internal/aiops/core"
)

// IncidentStoreConfig configures retention and pruning behavior.
type IncidentStoreConfig struct {
	// MaxIncidents limits total retained incidents (oldest beyond this are evicted).
	MaxIncidents int
	// Retention limits age; incidents older than now-Retention AND not updated recently are pruned.
	Retention time.Duration
	// PruneInterval controls background pruning frequency (0 disables auto prune loop).
	PruneInterval time.Duration
}

// DefaultIncidentStoreConfig provides reasonable defaults.
func DefaultIncidentStoreConfig() IncidentStoreConfig {
	return IncidentStoreConfig{
		MaxIncidents:  500,
		Retention:     24 * time.Hour,
		PruneInterval: 2 * time.Minute,
	}
}

// IncidentFilter defines filtering options for querying incidents.
type IncidentFilter struct {
	Types      []IncidentType
	Severities []Severity
	Statuses   []IncidentStatus
	// ResourcePrefix filters by PrimaryResource prefix (namespace, namespace/pod, etc).
	ResourcePrefix string
	// UpdatedSince filters incidents updated after this timestamp.
	UpdatedSince *time.Time
	// Limit restricts number returned (after sort).
	Limit int
	// SortBy determines ordering (updated|first|severity|type).
	SortBy string
	// Desc if true reverses ordering (default true).
	Desc *bool
}

// IncidentStore provides thread-safe storage / lifecycle for incidents.
type IncidentStore struct {
	cfg IncidentStoreConfig
	mu  sync.RWMutex
	// incidents keyed by ID
	incidents map[string]*Incident
	// index by primary resource -> incident IDs (allow quick lookups)
	resourceIndex map[string]map[string]struct{}
	// optional bus (may be nil) to publish core.SignalIncidentUpdated
	bus core.Bus
	// stopping / lifecycle
	stopCh chan struct{}
}

// NewIncidentStore creates a new store (optionally starts prune loop).
func NewIncidentStore(cfg IncidentStoreConfig, bus core.Bus) *IncidentStore {
	if cfg.MaxIncidents <= 0 {
		cfg.MaxIncidents = DefaultIncidentStoreConfig().MaxIncidents
	}
	if cfg.Retention <= 0 {
		cfg.Retention = DefaultIncidentStoreConfig().Retention
	}
	store := &IncidentStore{
		cfg:           cfg,
		incidents:     make(map[string]*Incident),
		resourceIndex: make(map[string]map[string]struct{}),
		bus:           bus,
		stopCh:        make(chan struct{}),
	}
	if cfg.PruneInterval > 0 {
		go store.pruneLoop(cfg.PruneInterval)
	}
	return store
}

// Stop stops background prune loop (idempotent).
func (s *IncidentStore) Stop() {
	select {
	case <-s.stopCh:
		return
	default:
		close(s.stopCh)
	}
}

// UpsertIncident merges evidence & updates narrative/status if provided.
// Returns the updated Incident (copy) and whether it was newly created.
func (s *IncidentStore) UpsertIncident(template *Incident, newEvidence []Evidence, narrative *Narrative, newStatus *IncidentStatus) (Incident, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	created := false
	existing, ok := s.incidents[template.ID]
	if !ok {
		// Create fresh
		cpy := cloneIncident(template)
		cpy.Evidence = append([]Evidence{}, template.Evidence...)
		cpy.FirstSeen = now
		cpy.LastUpdated = now
		// Guarantee map
		if template.analysisMeta == nil {
			cpy.analysisMeta = make(map[string]any)
		} else {
			cpy.analysisMeta = make(map[string]any)
			for k, v := range template.analysisMeta {
				cpy.analysisMeta[k] = v
			}
		}
		s.incidents[cpy.ID] = cpy
		s.addResourceIndexLocked(cpy.PrimaryResource, cpy.ID)
		existing = cpy
		created = true
	}

	// Append evidence
	for _, ev := range newEvidence {
		ev.Timestamp = ev.Timestamp.UTC()
		existing.Evidence = append(existing.Evidence, ev)
	}

	// Update narrative if provided
	if narrative != nil {
		narr := *narrative
		narr.GeneratedAt = now
		existing.Narrative = &narr
	}

	// Status transition
	if newStatus != nil && existing.Status != *newStatus {
		existing.Status = *newStatus
	}

	existing.LastUpdated = now

	// Publish update
	s.publishIncidentLocked(existing)

	// Enforce capacity after update
	s.enforceCapacityLocked()

	// Return copy to avoid outside mutation
	return *cloneIncident(existing), created
}

// Get returns a copy of an incident by ID.
func (s *IncidentStore) Get(id string) (Incident, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	inc, ok := s.incidents[id]
	if !ok {
		return Incident{}, false
	}
	return *cloneIncident(inc), true
}

// List returns incidents filtered / sorted per IncidentFilter.
func (s *IncidentStore) List(filter IncidentFilter) []Incident {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Incident
	typeSet := toTypeSet(filter.Types)
	sevSet := toSeveritySet(filter.Severities)
	statSet := toStatusSet(filter.Statuses)
	var desc bool = true
	if filter.Desc != nil {
		desc = *filter.Desc
	}

	for _, inc := range s.incidents {
		if len(typeSet) > 0 {
			if _, ok := typeSet[inc.Type]; !ok {
				continue
			}
		}
		if len(sevSet) > 0 {
			if _, ok := sevSet[inc.Severity]; !ok {
				continue
			}
		}
		if len(statSet) > 0 {
			if _, ok := statSet[inc.Status]; !ok {
				continue
			}
		}
		if filter.ResourcePrefix != "" && !strings.HasPrefix(inc.PrimaryResource, filter.ResourcePrefix) {
			continue
		}
		if filter.UpdatedSince != nil && inc.LastUpdated.Before(filter.UpdatedSince.UTC()) {
			continue
		}
		result = append(result, inc)
	}

	// Sorting
	sort.Slice(result, func(i, j int) bool {
		switch filter.SortBy {
		case "first":
			if desc {
				return result[i].FirstSeen.After(result[j].FirstSeen)
			}
			return result[i].FirstSeen.Before(result[j].FirstSeen)
		case "severity":
			// Critical > Warning > Info (map severity)
			si := severityRank(result[i].Severity)
			sj := severityRank(result[j].Severity)
			if desc {
				return si > sj
			}
			return si < sj
		case "type":
			if desc {
				return result[i].Type > result[j].Type
			}
			return result[i].Type < result[j].Type
		default: // updated
			if desc {
				return result[i].LastUpdated.After(result[j].LastUpdated)
			}
			return result[i].LastUpdated.Before(result[j].LastUpdated)
		}
	})

	// Limit
	limit := filter.Limit
	if limit <= 0 || limit > len(result) {
		limit = len(result)
	}

	// Clone out
	out := make([]Incident, 0, limit)
	for idx := 0; idx < limit; idx++ {
		out = append(out, *cloneIncident(result[idx]))
	}
	return out
}

// pruneLoop runs periodic pruning if configured.
func (s *IncidentStore) pruneLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.Prune()
		}
	}
}

// Prune performs retention-based cleanup.
func (s *IncidentStore) Prune() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.incidents) == 0 {
		return
	}

	cutoff := time.Now().Add(-s.cfg.Retention)
	removed := 0
	for id, inc := range s.incidents {
		if inc.LastUpdated.Before(cutoff) {
			delete(s.incidents, id)
			s.removeResourceIndexLocked(inc.PrimaryResource, id)
			removed++
		}
	}
	if removed > 0 {
		// Capacity check again in case retention didn't reduce enough
		s.enforceCapacityLocked()
	}
}

// enforceCapacityLocked evicts oldest by LastUpdated if MaxIncidents exceeded.
func (s *IncidentStore) enforceCapacityLocked() {
	if s.cfg.MaxIncidents <= 0 || len(s.incidents) <= s.cfg.MaxIncidents {
		return
	}
	type pair struct {
		id  string
		ts  time.Time
		ptr *Incident
	}
	arr := make([]pair, 0, len(s.incidents))
	for id, inc := range s.incidents {
		arr = append(arr, pair{id: id, ts: inc.LastUpdated, ptr: inc})
	}
	// Oldest first
	sort.Slice(arr, func(i, j int) bool { return arr[i].ts.Before(arr[j].ts) })
	excess := len(arr) - s.cfg.MaxIncidents
	if excess <= 0 {
		return
	}
	for i := 0; i < excess; i++ {
		del := arr[i]
		delete(s.incidents, del.id)
		s.removeResourceIndexLocked(del.ptr.PrimaryResource, del.id)
	}
}

// addResourceIndexLocked indexes incident by primary resource.
func (s *IncidentStore) addResourceIndexLocked(resource, id string) {
	if resource == "" {
		return
	}
	idx, ok := s.resourceIndex[resource]
	if !ok {
		idx = make(map[string]struct{})
		s.resourceIndex[resource] = idx
	}
	idx[id] = struct{}{}
}

// removeResourceIndexLocked removes an incident id from resource index.
func (s *IncidentStore) removeResourceIndexLocked(resource, id string) {
	if resource == "" {
		return
	}
	if idx, ok := s.resourceIndex[resource]; ok {
		delete(idx, id)
		if len(idx) == 0 {
			delete(s.resourceIndex, resource)
		}
	}
}

// publishIncidentLocked publishes an update if bus present.
func (s *IncidentStore) publishIncidentLocked(inc *Incident) {
	if s.bus == nil {
		return
	}
	// Fire-and-forget
	s.bus.Publish(core.Signal{
		Type:           core.SignalIncidentUpdated,
		Timestamp:      time.Now(),
		CorrelationKey: inc.PrimaryResource,
		Payload:        *cloneIncident(inc),
	})
}

// Helpers

func cloneIncident(src *Incident) *Incident {
	cpy := &Incident{
		ID:              src.ID,
		Type:            src.Type,
		Status:          src.Status,
		Severity:        src.Severity,
		FirstSeen:       src.FirstSeen,
		LastUpdated:     src.LastUpdated,
		PrimaryResource: src.PrimaryResource,
		Narrative:       src.Narrative,
	}
	if src.Evidence != nil {
		cpy.Evidence = make([]Evidence, len(src.Evidence))
		copy(cpy.Evidence, src.Evidence)
	}
	if src.Narrative != nil {
		nc := *src.Narrative
		cpy.Narrative = &nc
	}
	return cpy
}

func toTypeSet(list []IncidentType) map[IncidentType]struct{} {
	if len(list) == 0 {
		return nil
	}
	m := make(map[IncidentType]struct{}, len(list))
	for _, v := range list {
		m[v] = struct{}{}
	}
	return m
}

func toSeveritySet(list []Severity) map[Severity]struct{} {
	if len(list) == 0 {
		return nil
	}
	m := make(map[Severity]struct{}, len(list))
	for _, v := range list {
		m[v] = struct{}{}
	}
	return m
}

func toStatusSet(list []IncidentStatus) map[IncidentStatus]struct{} {
	if len(list) == 0 {
		return nil
	}
	m := make(map[IncidentStatus]struct{}, len(list))
	for _, v := range list {
		m[v] = struct{}{}
	}
	return m
}

func severityRank(s Severity) int {
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

// Errors
var (
	ErrNotFound = errors.New("incident not found")
)

//go:build legacy_aiops_unused
// +build legacy_aiops_unused

package analyzer

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// StoreConfig configures how much historical data is retained in memory.
type StoreConfig struct {
	// MaxSamplesPerSeries is the upper bound of samples retained per (namespace/pod/container) key.
	// When the limit is exceeded the oldest samples are discarded (ring buffer behavior).
	MaxSamplesPerSeries int

	// Retention specifies how long samples are kept. Samples older than (now - Retention)
	// are pruned opportunistically during writes and explicit Prune() calls.
	Retention time.Duration
}

// DefaultStoreConfig returns a conservative default configuration.
func DefaultStoreConfig() StoreConfig {
	return StoreConfig{
		MaxSamplesPerSeries: 720,            // e.g. 12h if sampled every minute
		Retention:           24 * time.Hour, // keep a rolling day by default
	}
}

// Sample represents a single resource usage observation for a container.
type Sample struct {
	Timestamp time.Time
	CPUMilli  float64 // instantaneous (or short window) CPU usage in millicores
	MemMB     float64 // memory usage in MB
}

// SeriesStats summarizes a time series.
type SeriesStats struct {
	Key                string
	Samples            int
	FirstTimestamp     time.Time
	LastTimestamp      time.Time
	Duration           time.Duration
	ApproxMemoryFootMB float64
}

// series is an internal structure holding samples for a single key.
type series struct {
	samples []Sample
}

// HistoricalStore keeps an in-memory, thread-safe collection of resource usage samples.
type HistoricalStore struct {
	cfg  StoreConfig
	mu   sync.RWMutex
	data map[string]*series
}

// NewHistoricalStore creates a new historical store instance.
func NewHistoricalStore(cfg StoreConfig) *HistoricalStore {
	// Basic defensive defaults.
	if cfg.MaxSamplesPerSeries <= 0 {
		cfg.MaxSamplesPerSeries = DefaultStoreConfig().MaxSamplesPerSeries
	}
	if cfg.Retention <= 0 {
		cfg.Retention = DefaultStoreConfig().Retention
	}

	return &HistoricalStore{
		cfg:  cfg,
		data: make(map[string]*series),
	}
}

// key builds the internal map key for a (namespace, pod, container).
func key(namespace, pod, container string) string {
	return fmt.Sprintf("%s/%s/%s", namespace, pod, container)
}

// AddSample inserts a new sample. This is safe for concurrent use.
//
// Typical usage: called periodically by a sampler that queries the metrics.Provider
// for current usage and stores the result here.
func (h *HistoricalStore) AddSample(namespace, pod, container string, cpuMilli, memMB float64, ts time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()

	k := key(namespace, pod, container)
	s, ok := h.data[k]
	if !ok {
		s = &series{samples: make([]Sample, 0, minInt(h.cfg.MaxSamplesPerSeries, 128))}
		h.data[k] = s
	}

	// Discard samples that violate retention (opportunistic prune)
	cutoff := time.Now().Add(-h.cfg.Retention)
	if len(s.samples) > 0 && s.samples[0].Timestamp.Before(cutoff) {
		// Trim prefix
		firstIdx := 0
		for firstIdx < len(s.samples) && s.samples[firstIdx].Timestamp.Before(cutoff) {
			firstIdx++
		}
		if firstIdx > 0 && firstIdx < len(s.samples) {
			s.samples = append([]Sample(nil), s.samples[firstIdx:]...)
		} else if firstIdx >= len(s.samples) {
			s.samples = s.samples[:0]
		}
	}

	// Append new sample
	s.samples = append(s.samples, Sample{
		Timestamp: ts,
		CPUMilli:  cpuMilli,
		MemMB:     memMB,
	})

	// Enforce capacity (ring buffer semantics)
	if len(s.samples) > h.cfg.MaxSamplesPerSeries {
		overflow := len(s.samples) - h.cfg.MaxSamplesPerSeries
		s.samples = append([]Sample(nil), s.samples[overflow:]...)
	}
}

// GetSeries returns the samples for the given key since the provided start time.
// The returned slice is a copy (caller-safe).
func (h *HistoricalStore) GetSeries(namespace, pod, container string, since time.Time) ([]Sample, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	k := key(namespace, pod, container)
	s, ok := h.data[k]
	if !ok || len(s.samples) == 0 {
		return nil, errors.New("no samples found")
	}

	// Binary search to find first sample >= since
	i := sort.Search(len(s.samples), func(i int) bool {
		return !s.samples[i].Timestamp.Before(since)
	})

	if i >= len(s.samples) {
		return nil, errors.New("no samples in requested time range")
	}

	out := make([]Sample, len(s.samples)-i)
	copy(out, s.samples[i:])
	return out, nil
}

// Prune removes samples that exceed retention across all series.
// This can be called periodically (e.g., via a ticker).
func (h *HistoricalStore) Prune() {
	cutoff := time.Now().Add(-h.cfg.Retention)

	h.mu.Lock()
	defer h.mu.Unlock()

	for k, s := range h.data {
		// Fast path: all samples are recent.
		if len(s.samples) == 0 {
			continue
		}
		if !s.samples[0].Timestamp.Before(cutoff) {
			continue
		}
		// Find first kept sample
		idx := 0
		for idx < len(s.samples) && s.samples[idx].Timestamp.Before(cutoff) {
			idx++
		}
		if idx >= len(s.samples) {
			// Remove series entirely if now empty
			delete(h.data, k)
			continue
		}
		s.samples = append([]Sample(nil), s.samples[idx:]...)
	}
}

// Stats returns a snapshot of series statistics for observability.
func (h *HistoricalStore) Stats() []SeriesStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stats := make([]SeriesStats, 0, len(h.data))
	for k, s := range h.data {
		if len(s.samples) == 0 {
			continue
		}
		first := s.samples[0].Timestamp
		last := s.samples[len(s.samples)-1].Timestamp
		// Rough memory footprint: each Sample ~ (3 float64 + time + overhead).
		// We'll approximate to 64 bytes per sample (not exact, but indicative).
		approx := float64(len(s.samples)*64) / (1024.0 * 1024.0)

		stats = append(stats, SeriesStats{
			Key:                k,
			Samples:            len(s.samples),
			FirstTimestamp:     first,
			LastTimestamp:      last,
			Duration:           last.Sub(first),
			ApproxMemoryFootMB: approx,
		})
	}
	return stats
}

// LinearRegressionResult holds the outcome of a simple linear regression.
type LinearRegressionResult struct {
	Slope        float64 // units per minute (e.g., MB per minute)
	Intercept    float64 // value at time zero
	R2           float64 // coefficient of determination
	Points       int
	DurationMins float64
	// PositiveTrend indicates slope is positive and statistically meaningful (R2 above threshold).
	PositiveTrend bool
}

// ComputeMemoryGrowth performs a least-squares linear regression over memory usage
// samples. It returns the slope (MB per minute), intercept, R², and supplemental data.
// A minimal number of points (>= 5) is required for a stable result.
//
// The caller can interpret a persistent positive slope with sufficient R² as evidence
// suggestive of a memory leak (additional heuristics required for strong confidence).
func ComputeMemoryGrowth(samples []Sample, r2Threshold float64) (LinearRegressionResult, error) {
	if len(samples) < 5 {
		return LinearRegressionResult{}, errors.New("insufficient samples for regression (need >=5)")
	}

	// Use the first timestamp as zero to improve numerical stability.
	t0 := samples[0].Timestamp
	var sumX, sumY, sumXY, sumX2 float64
	n := float64(len(samples))

	for _, s := range samples {
		dtMinutes := s.Timestamp.Sub(t0).Minutes()
		x := dtMinutes
		y := s.MemMB

		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denom := (n*sumX2 - sumX*sumX)
	if math.Abs(denom) < 1e-9 {
		return LinearRegressionResult{}, errors.New("degenerate regression denominator")
	}

	slope := (n*sumXY - sumX*sumY) / denom
	intercept := (sumY - slope*sumX) / n

	// Compute R²
	var ssTot, ssRes float64
	meanY := sumY / n
	for _, s := range samples {
		x := s.Timestamp.Sub(t0).Minutes()
		pred := intercept + slope*x
		dy := s.MemMB - meanY
		res := s.MemMB - pred
		ssTot += dy * dy
		ssRes += res * res
	}

	var r2 float64
	if ssTot > 0 {
		r2 = 1 - (ssRes / ssTot)
	} else {
		r2 = 0
	}

	result := LinearRegressionResult{
		Slope:        slope,
		Intercept:    intercept,
		R2:           r2,
		Points:       len(samples),
		DurationMins: samples[len(samples)-1].Timestamp.Sub(samples[0].Timestamp).Minutes(),
		PositiveTrend: slope > 0 &&
			r2 >= r2Threshold &&
			// Require meaningful magnitude (e.g., > 1MB every 5 minutes ~ 0.2 MB/min)
			slope >= 0.2,
	}

	return result, nil
}

// Helper for minimal integer comparison
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

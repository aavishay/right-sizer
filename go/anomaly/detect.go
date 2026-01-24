// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package anomaly

import (
	"math"
	"time"

	"right-sizer/memstore"
)

// Detector identifies abnormal pod behavior
type Detector struct {
	store       *memstore.MemoryStore
	zThreshold  float64
}

// Result of anomaly detection
type Result struct {
	IsAnomaly   bool
	IsBurst     bool
	Severity    string
	ZScore      float64
	Explanation string
}

// New creates a detector for abnormal patterns
func New(store *memstore.MemoryStore) *Detector {
	return &Detector{
		store:      store,
		zThreshold: 2.5, // 2.5 sigma
	}
}

// DetectCPU checks for abnormal CPU usage
func (d *Detector) DetectCPU(namespace, podName string) *Result {
	// Get baseline from last hour
	baseStats := d.store.Query(namespace, podName, 3600*time.Second)
	if baseStats == nil || baseStats.Count < 5 {
		return &Result{Explanation: "insufficient data"}
	}

	// Get last data points for comparison
	recentStats := d.store.Query(namespace, podName, 60*time.Second)
	if recentStats == nil || recentStats.Count == 0 {
		recentStats = baseStats // Fall back to all data if no recent
	}

	if baseStats.CPUStdDev <= 0 {
		return &Result{Explanation: "no variance in baseline"}
	}

	current := recentStats.CPUMean
	baseline := baseStats.CPUMean
	stdDev := baseStats.CPUStdDev
	zScore := (current - baseline) / stdDev

	result := &Result{ZScore: zScore}

	if math.Abs(zScore) > d.zThreshold {
		result.IsAnomaly = true
		result.Severity = "high"
		result.Explanation = "CPU significantly above baseline"

		p95, _ := d.store.Percentile(namespace, podName, 3600*time.Second, 95)
		if p95 > 0 && current < p95*1.5 {
			result.IsBurst = true
			result.Severity = "medium"
			result.Explanation = "Spike within historical range (likely burst)"
		}
	}

	return result
}

// DetectMemory checks for abnormal memory usage
func (d *Detector) DetectMemory(namespace, podName string) *Result {
	baseStats := d.store.Query(namespace, podName, 3600*time.Second)
	if baseStats == nil || baseStats.Count < 5 {
		return &Result{Explanation: "insufficient data"}
	}

	recentStats := d.store.Query(namespace, podName, 60*time.Second)
	if recentStats == nil || recentStats.Count == 0 {
		recentStats = baseStats
	}

	if baseStats.MemStdDev <= 0 {
		return &Result{Explanation: "no variance in baseline"}
	}

	current := recentStats.MemMean
	baseline := baseStats.MemMean
	stdDev := baseStats.MemStdDev
	zScore := (current - baseline) / stdDev

	result := &Result{ZScore: zScore}

	if math.Abs(zScore) > d.zThreshold {
		result.IsAnomaly = true
		result.Severity = "high"
		result.Explanation = "Memory significantly above baseline"

		p95, _ := d.store.Percentile(namespace, podName, 3600*time.Second, 95)
		if p95 > 0 && current < p95*1.5 {
			result.IsBurst = true
			result.Severity = "medium"
			result.Explanation = "Spike within historical range (likely burst)"
		}
	}

	return result
}

// SetThreshold adjusts detection sensitivity
func (d *Detector) SetThreshold(zScore float64) {
	if zScore > 0 {
		d.zThreshold = zScore
	}
}

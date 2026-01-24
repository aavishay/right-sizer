// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package memstore

import (
	"math"
	"sort"
	"sync"
	"time"
)

// DataPoint stores a single metric observation
type DataPoint struct {
	Timestamp  time.Time
	CPUMilli   float64
	MemMB      float64
	CPUThrottle float64
}

// Stats holds aggregate statistics
type Stats struct {
	Count          int
	CPUMin         float64
	CPUMax         float64
	CPUMean        float64
	CPUStdDev      float64
	MemMin         float64
	MemMax         float64
	MemMean        float64
	MemStdDev      float64
	CPUThrottleAvg float64
}

// Trend represents usage trend over time
type Trend struct {
	Slope      float64
	Direction  string
	Confidence float64
}

// MemoryStore manages pod history across the cluster
type MemoryStore struct {
	mu      sync.RWMutex
	pods    map[string]*podHistory
	maxDays int
	maxPPod int
}

type podHistory struct {
	mu         sync.RWMutex
	namespace  string
	podName    string
	dataPoints []DataPoint
	maxPoints  int
}

// NewMemoryStore creates a memory store for historical pod metrics
func NewMemoryStore(maxDays, maxPointsPerPod int) *MemoryStore {
	ms := &MemoryStore{
		pods:    make(map[string]*podHistory),
		maxDays: maxDays,
		maxPPod: maxPointsPerPod,
	}
	if ms.maxDays <= 0 {
		ms.maxDays = 7
	}
	if ms.maxPPod <= 0 {
		ms.maxPPod = 10080
	}
	go ms.cleanup()
	return ms
}

// Record adds a data point for a pod
func (ms *MemoryStore) Record(namespace, podName string, dp DataPoint) {
	key := namespace + "/" + podName

	ms.mu.RLock()
	ph, exists := ms.pods[key]
	ms.mu.RUnlock()

	if !exists {
		ph = &podHistory{
			namespace:  namespace,
			podName:    podName,
			dataPoints: make([]DataPoint, 0, ms.maxPPod),
			maxPoints:  ms.maxPPod,
		}
		ms.mu.Lock()
		ms.pods[key] = ph
		ms.mu.Unlock()
	}

	ph.mu.Lock()
	defer ph.mu.Unlock()

	if len(ph.dataPoints) >= ph.maxPoints {
		removeCount := ph.maxPoints / 10
		if removeCount == 0 {
			removeCount = 1
		}
		ph.dataPoints = ph.dataPoints[removeCount:]
	}

	ph.dataPoints = append(ph.dataPoints, dp)
}

// Query retrieves statistics for a pod within a time window
func (ms *MemoryStore) Query(namespace, podName string, duration time.Duration) *Stats {
	key := namespace + "/" + podName

	ms.mu.RLock()
	ph, exists := ms.pods[key]
	ms.mu.RUnlock()

	if !exists {
		return nil
	}

	cutoff := time.Now().Add(-duration)

	ph.mu.RLock()
	defer ph.mu.RUnlock()

	var filtered []DataPoint
	for _, dp := range ph.dataPoints {
		if dp.Timestamp.After(cutoff) {
			filtered = append(filtered, dp)
		}
	}

	if len(filtered) == 0 {
		return nil
	}

	return calculateStats(filtered)
}

// GetTrend calculates trend for CPU and memory usage
func (ms *MemoryStore) GetTrend(namespace, podName string, duration time.Duration) (*Trend, *Trend) {
	key := namespace + "/" + podName

	ms.mu.RLock()
	ph, exists := ms.pods[key]
	ms.mu.RUnlock()

	if !exists {
		return nil, nil
	}

	cutoff := time.Now().Add(-duration)

	ph.mu.RLock()
	defer ph.mu.RUnlock()

	var filtered []DataPoint
	for _, dp := range ph.dataPoints {
		if dp.Timestamp.After(cutoff) {
			filtered = append(filtered, dp)
		}
	}

	if len(filtered) < 2 {
		return nil, nil
	}

	cpuTrend := calculateTrend(filtered, func(dp DataPoint) float64 { return dp.CPUMilli })
	memTrend := calculateTrend(filtered, func(dp DataPoint) float64 { return dp.MemMB })

	return cpuTrend, memTrend
}

// Percentile returns the Nth percentile for CPU and memory
func (ms *MemoryStore) Percentile(namespace, podName string, duration time.Duration, p float64) (cpuP, memP float64) {
	key := namespace + "/" + podName

	ms.mu.RLock()
	ph, exists := ms.pods[key]
	ms.mu.RUnlock()

	if !exists {
		return 0, 0
	}

	cutoff := time.Now().Add(-duration)

	ph.mu.RLock()
	defer ph.mu.RUnlock()

	var cpuValues, memValues []float64
	for _, dp := range ph.dataPoints {
		if dp.Timestamp.After(cutoff) {
			cpuValues = append(cpuValues, dp.CPUMilli)
			memValues = append(memValues, dp.MemMB)
		}
	}

	if len(cpuValues) == 0 {
		return 0, 0
	}

	return percentile(cpuValues, p), percentile(memValues, p)
}

// Prune removes old data points and stale pods
func (ms *MemoryStore) Prune() {
	cutoff := time.Now().Add(-time.Duration(ms.maxDays) * 24 * time.Hour)

	ms.mu.Lock()
	defer ms.mu.Unlock()

	for key, ph := range ms.pods {
		ph.mu.Lock()

		var kept []DataPoint
		for _, dp := range ph.dataPoints {
			if dp.Timestamp.After(cutoff) {
				kept = append(kept, dp)
			}
		}

		if len(kept) == 0 {
			delete(ms.pods, key)
			ph.mu.Unlock()
			continue
		}

		ph.dataPoints = kept
		ph.mu.Unlock()
	}
}

func (ms *MemoryStore) cleanup() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		ms.Prune()
	}
}

// Helper functions

func calculateStats(points []DataPoint) *Stats {
	if len(points) == 0 {
		return nil
	}

	stats := &Stats{Count: len(points)}
	cpuValues := make([]float64, len(points))
	memValues := make([]float64, len(points))
	var cpuSum, memSum, throttleSum float64

	for i, dp := range points {
		cpuValues[i] = dp.CPUMilli
		memValues[i] = dp.MemMB
		cpuSum += dp.CPUMilli
		memSum += dp.MemMB
		throttleSum += dp.CPUThrottle
	}

	stats.CPUMean = cpuSum / float64(len(points))
	stats.MemMean = memSum / float64(len(points))
	stats.CPUThrottleAvg = throttleSum / float64(len(points))

	stats.CPUMin = cpuValues[0]
	stats.CPUMax = cpuValues[0]
	stats.MemMin = memValues[0]
	stats.MemMax = memValues[0]

	for i := 0; i < len(points); i++ {
		if cpuValues[i] < stats.CPUMin {
			stats.CPUMin = cpuValues[i]
		}
		if cpuValues[i] > stats.CPUMax {
			stats.CPUMax = cpuValues[i]
		}
		if memValues[i] < stats.MemMin {
			stats.MemMin = memValues[i]
		}
		if memValues[i] > stats.MemMax {
			stats.MemMax = memValues[i]
		}
	}

	stats.CPUStdDev = stdDev(cpuValues, stats.CPUMean)
	stats.MemStdDev = stdDev(memValues, stats.MemMean)

	return stats
}

func stdDev(values []float64, mean float64) float64 {
	if len(values) <= 1 {
		return 0
	}

	var sumSquares float64
	for _, v := range values {
		diff := v - mean
		sumSquares += diff * diff
	}

	return math.Sqrt(sumSquares / float64(len(values)-1))
}

func calculateTrend(points []DataPoint, extractor func(DataPoint) float64) *Trend {
	if len(points) < 2 {
		return nil
	}

	n := float64(len(points))
	var sumX, sumY, sumXY, sumX2 float64

	for i, dp := range points {
		x := float64(i)
		y := extractor(dp)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denominator := (n * sumX2) - (sumX * sumX)
	if denominator == 0 {
		return &Trend{Slope: 0, Direction: "stable", Confidence: 0}
	}

	slope := ((n * sumXY) - (sumX * sumY)) / denominator
	intercept := (sumY - slope*sumX) / n

	var ssRes, ssTot float64
	mean := sumY / n

	for i, dp := range points {
		predicted := slope*float64(i) + intercept
		actual := extractor(dp)
		ssRes += (actual - predicted) * (actual - predicted)
		ssTot += (actual - mean) * (actual - mean)
	}

	rSquared := 1.0
	if ssTot > 0 {
		rSquared = 1.0 - (ssRes / ssTot)
	}

	direction := "stable"
	if slope > 0.01 {
		direction = "increasing"
	} else if slope < -0.01 {
		direction = "decreasing"
	}

	return &Trend{
		Slope:      slope,
		Direction:  direction,
		Confidence: math.Max(0, math.Min(1, rSquared)),
	}
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	index := (p / 100) * float64(len(sorted)-1)
	lower := int(index)
	upper := lower + 1

	if upper >= len(sorted) {
		return sorted[lower]
	}

	weight := index - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}

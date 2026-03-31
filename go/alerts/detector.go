// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package alerts

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"go.uber.org/zap"
	"right-sizer/memstore"
)

// Detector generates alerts based on resource anomalies
type Detector struct {
	store    *memstore.MemoryStore
	manager  *Manager
	logger   *zap.Logger
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewDetector creates an anomaly-based alert detector
func NewDetector(store *memstore.MemoryStore, manager *Manager, logger *zap.Logger) *Detector {
	return &Detector{
		store:    store,
		manager:  manager,
		logger:   logger,
		stopChan: make(chan struct{}),
	}
}

// Start begins continuous anomaly detection
func (d *Detector) Start(ctx context.Context, interval time.Duration) {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-d.stopChan:
				return
			case <-ticker.C:
				d.checkAnomalies(ctx)
			}
		}
	}()
}

// Stop halts the detector
func (d *Detector) Stop() {
	close(d.stopChan)
	d.wg.Wait()
}

// checkAnomalies scans all pods for anomalies
func (d *Detector) checkAnomalies(ctx context.Context) {
	// TODO: Iterate over all monitored pods and call detectors
	// This would typically be integrated with the controller watching pods
}

// DetectCPUAnomaly checks if CPU usage is anomalous
func (d *Detector) DetectCPUAnomaly(ctx context.Context, namespace, podName string) (*Alert, error) {
	// Get 1-hour baseline
	baselineStats := d.store.Query(namespace, podName, 3600*time.Second)
	if baselineStats == nil || baselineStats.Count < 10 {
		return nil, nil // Insufficient data
	}

	// Get last 5 minutes for comparison
	recentStats := d.store.Query(namespace, podName, 5*time.Minute)
	if recentStats == nil || recentStats.Count == 0 {
		return nil, nil // No recent data
	}

	// Calculate Z-score for CPU
	zScore := d.calculateZScore(baselineStats.CPUMean, baselineStats.CPUStdDev, recentStats.CPUMean)

	// Trigger alert if Z-score exceeds threshold (typically 3.0)
	if math.Abs(zScore) >= 3.0 {
		severity := "warning"
		if math.Abs(zScore) >= 4.0 {
			severity = "critical"
		}

		shortfall := d.predictResourceShortfall(ctx, namespace, podName, "cpu", recentStats.CPUMax)

		title := fmt.Sprintf("CPU Usage Anomaly Detected: %.2f%%", recentStats.CPUMax)
		message := fmt.Sprintf("CPU usage is %.2f standard deviations above baseline (Z-score: %.2f).", math.Abs(zScore), zScore)
		if shortfall {
			message += " Pod may run out of allocated CPU."
		}

		alert, err := d.manager.Create(ctx,
			namespace,
			podName,
			"cpu",
			severity,
			title,
			message,
			"anomaly",
			recentStats.CPUMax,
			baselineStats.CPUMean+(baselineStats.CPUStdDev*3.0), // 3-sigma threshold
		)
		if err != nil {
			d.logger.Error("Failed to create alert", zap.Error(err))
			return nil, err
		}

		alert.ZScore = zScore
		return alert, nil
	}

	return nil, nil
}

// DetectMemoryAnomaly checks if memory usage is anomalous
func (d *Detector) DetectMemoryAnomaly(ctx context.Context, namespace, podName string) (*Alert, error) {
	// Get 1-hour baseline
	baselineStats := d.store.Query(namespace, podName, 3600*time.Second)
	if baselineStats == nil || baselineStats.Count < 10 {
		return nil, nil // Insufficient data
	}

	// Get last 5 minutes for comparison
	recentStats := d.store.Query(namespace, podName, 5*time.Minute)
	if recentStats == nil || recentStats.Count == 0 {
		return nil, nil // No recent data
	}

	// Calculate Z-score for Memory
	zScore := d.calculateZScore(baselineStats.MemMean, baselineStats.MemStdDev, recentStats.MemMean)

	// Trigger alert if Z-score exceeds threshold
	if math.Abs(zScore) >= 3.0 {
		severity := "warning"
		if math.Abs(zScore) >= 4.0 {
			severity = "critical"
		}

		shortfall := d.predictResourceShortfall(ctx, namespace, podName, "memory", recentStats.MemMax)

		title := fmt.Sprintf("Memory Usage Anomaly Detected: %.2fMi", recentStats.MemMax)
		message := fmt.Sprintf("Memory usage is %.2f standard deviations above baseline (Z-score: %.2f).", math.Abs(zScore), zScore)
		if shortfall {
			message += " Pod may run out of allocated memory."
		}

		alert, err := d.manager.Create(ctx,
			namespace,
			podName,
			"memory",
			severity,
			title,
			message,
			"anomaly",
			recentStats.MemMax,
			baselineStats.MemMean+(baselineStats.MemStdDev*3.0),
		)
		if err != nil {
			d.logger.Error("Failed to create alert", zap.Error(err))
			return nil, err
		}

		alert.ZScore = zScore
		return alert, nil
	}

	return nil, nil
}

// predictResourceShortfall checks if pod will exceed allocation in near future
func (d *Detector) predictResourceShortfall(ctx context.Context, namespace, podName, resourceType string, currentUsage float64) bool {
	// Get historical data from memstore
	data := d.store.GetHistoricalData(namespace, podName, 24*time.Hour)
	if len(data) < 10 {
		return false // Insufficient data for prediction
	}

	// Get pod resource limits
	limits := d.store.GetLimits(namespace, podName)
	if limits == nil {
		return false
	}

	var limit float64
	if resourceType == "cpu" {
		limit = limits.CPULimit
	} else if resourceType == "memory" {
		limit = limits.MemoryLimit
	}

	if limit <= 0 {
		return false
	}

	// Calculate current utilization percentage
	utilization := currentUsage / limit

	// Calculate trend from historical data
	firstHalf := data[:len(data)/2]
	secondHalf := data[len(data)/2:]

	var avgFirst, avgSecond float64
	if resourceType == "cpu" {
		avgFirst = calculateAverageCPU(firstHalf)
		avgSecond = calculateAverageCPU(secondHalf)
	} else {
		avgFirst = calculateAverageMemory(firstHalf)
		avgSecond = calculateAverageMemory(secondHalf)
	}

	// Calculate growth rate
	var growthRate float64
	if avgFirst > 0 {
		growthRate = (avgSecond - avgFirst) / avgFirst
	}

	// Predict usage in 1 hour based on trend
	// Use simple linear extrapolation
	predictedUsage := currentUsage * (1 + growthRate)

	// If predicted usage exceeds limit, shortfall detected
	if predictedUsage >= limit*0.95 { // 95% threshold
		return true
	}

	// Also check if current utilization is already high
	if utilization >= 0.9 { // 90% threshold for immediate concern
		return true
	}

	return false
}

// calculateAverageCPU calculates the average CPU from data points
func calculateAverageCPU(data []memstore.DataPoint) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := 0.0
	for _, dp := range data {
		sum += dp.CPUMilli
	}
	return sum / float64(len(data))
}

// calculateAverageMemory calculates the average memory from data points
func calculateAverageMemory(data []memstore.DataPoint) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := 0.0
	for _, dp := range data {
		sum += dp.MemMB
	}
	return sum / float64(len(data))
}

// calculateZScore calculates standard score: (x - mean) / std_dev
func (d *Detector) calculateZScore(mean, stdDev, value float64) float64 {
	if stdDev == 0 {
		return 0
	}
	return (value - mean) / stdDev
}

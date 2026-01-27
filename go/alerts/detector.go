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
	"time"

	"go.uber.org/zap"
	"right-sizer/memstore"
)

// Detector generates alerts based on resource anomalies
type Detector struct {
	store    *memstore.MemoryStore
	manager  *Manager
	logger   *zap.Logger
	zScores  map[string]float64 // Pod → Z-Score threshold
	stopChan chan struct{}
}

// NewDetector creates an anomaly-based alert detector
func NewDetector(store *memstore.MemoryStore, manager *Manager, logger *zap.Logger) *Detector {
	return &Detector{
		store:    store,
		manager:  manager,
		logger:   logger,
		zScores:  make(map[string]float64),
		stopChan: make(chan struct{}),
	}
}

// Start begins continuous anomaly detection
func (d *Detector) Start(ctx context.Context, interval time.Duration) {
	go func() {
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
	// TODO: Use predictor to forecast next 1 hour
	// For now, return false
	return false
}

// calculateZScore calculates standard score: (x - mean) / std_dev
func (d *Detector) calculateZScore(mean, stdDev, value float64) float64 {
	if stdDev == 0 {
		return 0
	}
	return (value - mean) / stdDev
}

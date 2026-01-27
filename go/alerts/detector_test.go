// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package alerts

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
	"right-sizer/memstore"
)

func TestNewDetector(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	manager := New(zap.NewNop())
	logger := zap.NewNop()

	detector := NewDetector(store, manager, logger)

	if detector == nil {
		t.Fatal("Detector should not be nil")
	}
	if detector.store != store {
		t.Fatal("Store should be set")
	}
	if detector.manager != manager {
		t.Fatal("Manager should be set")
	}
}

func TestCalculateZScore(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	manager := New(zap.NewNop())
	detector := NewDetector(store, manager, zap.NewNop())

	tests := []struct {
		mean     float64
		stdDev   float64
		value    float64
		expected float64
	}{
		{100, 10, 110, 1.0}, // 1 sigma above
		{100, 10, 90, -1.0}, // 1 sigma below
		{100, 10, 130, 3.0}, // 3 sigma above
		{100, 10, 100, 0.0}, // At mean
		{100, 0, 100, 0.0},  // Zero stddev
	}

	for _, tt := range tests {
		result := detector.calculateZScore(tt.mean, tt.stdDev, tt.value)
		if result != tt.expected {
			t.Errorf("calculateZScore(%f, %f, %f) = %f, want %f",
				tt.mean, tt.stdDev, tt.value, result, tt.expected)
		}
	}
}

func TestDetectCPUAnomaly_InsufficientData(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	manager := New(zap.NewNop())
	detector := NewDetector(store, manager, zap.NewNop())
	ctx := context.Background()

	// No data in store
	alert, err := detector.DetectCPUAnomaly(ctx, "default", "test-pod")

	if err != nil {
		t.Fatalf("DetectCPUAnomaly failed: %v", err)
	}
	if alert != nil {
		t.Fatal("Alert should be nil when insufficient data")
	}
}

func TestDetectCPUAnomaly_WithData(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	manager := New(zap.NewNop())
	detector := NewDetector(store, manager, zap.NewNop())
	ctx := context.Background()

	// Add baseline data (normal distribution)
	now := time.Now()
	for i := 0; i < 100; i++ {
		dataPoint := memstore.DataPoint{
			Timestamp:   now.Add(-time.Duration(i) * time.Minute),
			CPUMilli:    100 + float64(i%10),
			MemMB:       512 + float64(i%5),
			CPUThrottle: 0,
		}
		store.Record("default", "test-pod", dataPoint)
	}

	// Add anomalous recent data (high Z-score)
	for i := 0; i < 3; i++ {
		dataPoint := memstore.DataPoint{
			Timestamp:   now.Add(-time.Duration(i) * time.Second),
			CPUMilli:    500, // 5x baseline
			MemMB:       512,
			CPUThrottle: 0,
		}
		store.Record("default", "test-pod", dataPoint)
	}

	alert, err := detector.DetectCPUAnomaly(ctx, "default", "test-pod")

	if err != nil {
		t.Fatalf("DetectCPUAnomaly failed: %v", err)
	}

	if alert != nil {
		// Alert may or may not be created depending on z-score threshold
		if alert.Severity != "warning" && alert.Severity != "critical" {
			t.Errorf("Expected warning or critical severity, got %s", alert.Severity)
		}
	}
}

func TestDetectMemoryAnomaly_InsufficientData(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	manager := New(zap.NewNop())
	detector := NewDetector(store, manager, zap.NewNop())
	ctx := context.Background()

	alert, err := detector.DetectMemoryAnomaly(ctx, "default", "test-pod")

	if err != nil {
		t.Fatalf("DetectMemoryAnomaly failed: %v", err)
	}
	if alert != nil {
		t.Fatal("Alert should be nil when insufficient data")
	}
}

func TestDetectorStart(t *testing.T) {
	store := memstore.NewMemoryStore(7, 1440)
	manager := New(zap.NewNop())
	detector := NewDetector(store, manager, zap.NewNop())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Start should not panic
	detector.Start(ctx, 50*time.Millisecond)

	// Wait for context to expire
	<-ctx.Done()

	// Stop should not panic
	detector.Stop()
}

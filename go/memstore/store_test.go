// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package memstore

import (
	"testing"
	"time"
)

func TestRecord_and_Query(t *testing.T) {
	store := NewMemoryStore(7, 100)
	now := time.Now()

	for i := 0; i < 10; i++ {
		dp := DataPoint{
			Timestamp:   now.Add(time.Duration(i) * time.Minute),
			CPUMilli:    float64(100 * (i + 1)),
			MemMB:       float64(256 * (i + 1)),
			CPUThrottle: float64(i),
		}
		store.Record("default", "pod1", dp)
	}

	stats := store.Query("default", "pod1", 1*time.Hour)
	if stats == nil {
		t.Fatal("expected stats")
	}

	if stats.Count != 10 {
		t.Errorf("expected 10 points, got %d", stats.Count)
	}

	if stats.CPUMean == 0 {
		t.Error("CPUMean should not be zero")
	}
}

func TestTrend_Increasing(t *testing.T) {
	store := NewMemoryStore(7, 100)
	now := time.Now()

	for i := 0; i < 10; i++ {
		dp := DataPoint{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			CPUMilli:  float64(100 + i*10),
			MemMB:     float64(256 + i*20),
		}
		store.Record("default", "pod1", dp)
	}

	cpuTrend, memTrend := store.GetTrend("default", "pod1", 1*time.Hour)

	if cpuTrend == nil || memTrend == nil {
		t.Fatal("expected trends")
	}

	if cpuTrend.Direction != "increasing" {
		t.Errorf("expected increasing, got %s", cpuTrend.Direction)
	}
}

func TestPercentile(t *testing.T) {
	store := NewMemoryStore(7, 100)
	now := time.Now()

	for i := 0; i < 100; i++ {
		dp := DataPoint{
			Timestamp: now.Add(time.Duration(i) * time.Second),
			CPUMilli:  float64(i),
			MemMB:     float64(i * 2),
		}
		store.Record("default", "pod1", dp)
	}

	p50, _ := store.Percentile("default", "pod1", 1*time.Hour, 50)

	if p50 < 40 || p50 > 60 {
		t.Errorf("p50 expected ~50, got %f", p50)
	}
}

func BenchmarkRecord(b *testing.B) {
	store := NewMemoryStore(7, 10000)
	dp := DataPoint{
		Timestamp:   time.Now(),
		CPUMilli:    100,
		MemMB:       256,
		CPUThrottle: 0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Record("default", "pod1", dp)
	}
}

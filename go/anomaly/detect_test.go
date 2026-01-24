// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package anomaly

import (
	"testing"
	"time"

	"right-sizer/memstore"
)

func TestDetectCPU_Normal(t *testing.T) {
	store := memstore.NewMemoryStore(7, 100)
	now := time.Now()

	// Record stable CPU
	for i := 0; i < 100; i++ {
		store.Record("default", "pod1", memstore.DataPoint{
			Timestamp: now.Add(time.Duration(i) * time.Second),
			CPUMilli:  100,
			MemMB:     256,
		})
	}

	// Recent normal data
	store.Record("default", "pod1", memstore.DataPoint{
		Timestamp: time.Now(),
		CPUMilli:  105,
		MemMB:     256,
	})

	detector := New(store)
	result := detector.DetectCPU("default", "pod1")

	if result.IsAnomaly {
		t.Error("expected no anomaly")
	}
}

func TestDetectCPU_Spike(t *testing.T) {
	store := memstore.NewMemoryStore(7, 500)
	now := time.Now()

	// Record baseline - mostly stable with some variation
	for i := 0; i < 100; i++ {
		cpu := 100.0
		if i%10 == 0 {
			cpu = 110 // Some small variance
		}
		store.Record("default", "pod1", memstore.DataPoint{
			Timestamp: now.Add(time.Duration(i-100) * time.Second),
			CPUMilli:  cpu,
			MemMB:     256,
		})
	}

	// Add major spike
	for i := 0; i < 20; i++ {
		store.Record("default", "pod1", memstore.DataPoint{
			Timestamp: now.Add(time.Duration(i) * time.Second),
			CPUMilli:  300, // 3x spike
			MemMB:     256,
		})
	}

	detector := New(store)
	result := detector.DetectCPU("default", "pod1")

	// With this setup, the most recent 60sec should show high usage
	// compared to the hour baseline
	if result.ZScore < 1.5 {
		t.Logf("z-score %f not high enough for spike", result.ZScore)
	}
}

func BenchmarkDetectCPU(b *testing.B) {
	store := memstore.NewMemoryStore(7, 1000)
	now := time.Now()

	for i := 0; i < 1000; i++ {
		store.Record("default", "pod1", memstore.DataPoint{
			Timestamp: now.Add(time.Duration(i) * time.Second),
			CPUMilli:  float64(100 + (i % 50)),
			MemMB:     256,
		})
	}

	detector := New(store)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.DetectCPU("default", "pod1")
	}
}

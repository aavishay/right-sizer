// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package metrics

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockProvider struct {
	fetchCount int
	metrics    Metrics
	err        error
}

func (m *mockProvider) FetchPodMetrics(ctx context.Context, namespace, podName string) (Metrics, error) {
	m.fetchCount++
	return m.metrics, m.err
}

func TestCachedProvider_HitCache(t *testing.T) {
	mock := &mockProvider{
		metrics: Metrics{CPUMilli: 100, MemMB: 256},
	}

	cached := NewCachedProvider(mock, 1*time.Second)
	ctx := context.Background()

	// First call should fetch from upstream
	m1, err := cached.FetchPodMetrics(ctx, "default", "pod1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m1.CPUMilli != 100 || m1.MemMB != 256 {
		t.Errorf("unexpected metrics: %+v", m1)
	}
	if mock.fetchCount != 1 {
		t.Errorf("expected 1 fetch, got %d", mock.fetchCount)
	}

	// Second call should hit cache
	m2, err := cached.FetchPodMetrics(ctx, "default", "pod1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m2.CPUMilli != 100 || m2.MemMB != 256 {
		t.Errorf("unexpected metrics: %+v", m2)
	}
	if mock.fetchCount != 1 {
		t.Errorf("cache miss: expected 1 fetch, got %d", mock.fetchCount)
	}
}

func TestCachedProvider_Expiration(t *testing.T) {
	mock := &mockProvider{
		metrics: Metrics{CPUMilli: 100, MemMB: 256},
	}

	// Short TTL for test
	cached := NewCachedProvider(mock, 50*time.Millisecond)
	ctx := context.Background()

	// First fetch
	_, err := cached.FetchPodMetrics(ctx, "default", "pod1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Wait for cache to expire
	time.Sleep(100 * time.Millisecond)

	// Second fetch should hit upstream again
	_, err = cached.FetchPodMetrics(ctx, "default", "pod1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.fetchCount != 2 {
		t.Errorf("expected 2 fetches after expiration, got %d", mock.fetchCount)
	}
}

func TestCachedProvider_DifferentPods(t *testing.T) {
	mock := &mockProvider{
		metrics: Metrics{CPUMilli: 100, MemMB: 256},
	}

	cached := NewCachedProvider(mock, 1*time.Second)
	ctx := context.Background()

	// Fetch metrics for two different pods
	_, err := cached.FetchPodMetrics(ctx, "default", "pod1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = cached.FetchPodMetrics(ctx, "default", "pod2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 upstream fetches (different pods)
	if mock.fetchCount != 2 {
		t.Errorf("expected 2 fetches for different pods, got %d", mock.fetchCount)
	}

	// Fetch again for pod1 - should hit cache
	_, err = cached.FetchPodMetrics(ctx, "default", "pod1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.fetchCount != 2 {
		t.Errorf("expected still 2 fetches, got %d", mock.fetchCount)
	}
}

func TestCachedProvider_ErrorPropagation(t *testing.T) {
	mock := &mockProvider{
		err: errors.New("metrics unavailable"),
	}

	cached := NewCachedProvider(mock, 1*time.Second)
	ctx := context.Background()

	// Error should propagate and not be cached
	_, err := cached.FetchPodMetrics(ctx, "default", "pod1")
	if err == nil {
		t.Fatal("expected error")
	}

	// Second call should also hit upstream (errors not cached)
	_, err = cached.FetchPodMetrics(ctx, "default", "pod1")
	if err == nil {
		t.Fatal("expected error")
	}

	if mock.fetchCount != 2 {
		t.Errorf("expected 2 fetches (errors not cached), got %d", mock.fetchCount)
	}
}

func TestCachedProvider_Invalidate(t *testing.T) {
	mock := &mockProvider{
		metrics: Metrics{CPUMilli: 100, MemMB: 256},
	}

	cached := NewCachedProvider(mock, 1*time.Second).(*CachedProvider)
	ctx := context.Background()

	// Fetch and cache
	_, err := cached.FetchPodMetrics(ctx, "default", "pod1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Invalidate cache
	cached.Invalidate("default", "pod1")

	// Next fetch should hit upstream
	_, err = cached.FetchPodMetrics(ctx, "default", "pod1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.fetchCount != 2 {
		t.Errorf("expected 2 fetches after invalidation, got %d", mock.fetchCount)
	}
}

func TestCachedProvider_Clear(t *testing.T) {
	mock := &mockProvider{
		metrics: Metrics{CPUMilli: 100, MemMB: 256},
	}

	cached := NewCachedProvider(mock, 1*time.Second).(*CachedProvider)
	ctx := context.Background()

	// Fetch and cache multiple pods
	_, err := cached.FetchPodMetrics(ctx, "default", "pod1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = cached.FetchPodMetrics(ctx, "default", "pod2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Clear all cache
	cached.Clear()

	// Next fetches should hit upstream
	_, err = cached.FetchPodMetrics(ctx, "default", "pod1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = cached.FetchPodMetrics(ctx, "default", "pod2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.fetchCount != 4 {
		t.Errorf("expected 4 fetches after clear, got %d", mock.fetchCount)
	}
}

func BenchmarkCachedProvider_CacheHit(b *testing.B) {
	mock := &mockProvider{
		metrics: Metrics{CPUMilli: 100, MemMB: 256},
	}

	cached := NewCachedProvider(mock, 1*time.Minute)
	ctx := context.Background()

	// Prime the cache
	_, _ = cached.FetchPodMetrics(ctx, "default", "pod1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cached.FetchPodMetrics(ctx, "default", "pod1")
	}
}

func BenchmarkCachedProvider_CacheMiss(b *testing.B) {
	mock := &mockProvider{
		metrics: Metrics{CPUMilli: 100, MemMB: 256},
	}

	// Short TTL to ensure cache misses
	cached := NewCachedProvider(mock, 1*time.Nanosecond)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cached.FetchPodMetrics(ctx, "default", "pod1")
	}
}

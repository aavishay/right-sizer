package controllers

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"right-sizer/config"
	"right-sizer/metrics"
	"strings"
	"testing"
)

// minimal struct reuse: instantiate with Config only for helper methods
func newAdaptiveTestRig(cfg *config.Config) *AdaptiveRightSizer {
	return &AdaptiveRightSizer{Config: cfg, OperatorMetrics: metrics.NewOperatorMetrics()}
}

// TestShouldMaintainGuaranteedQoS verifies the condition logic
func TestShouldMaintainGuaranteedQoS(t *testing.T) {
	cfg := config.GetDefaults()
	r := newAdaptiveTestRig(cfg)
	if r.shouldMaintainGuaranteedQoS(cfg) {
		t.Fatalf("should be false for defaults (limit multipliers are 2.0)")
	}
	// Adjust config to meet strict guaranteed condition
	cfg.CPULimitMultiplier = 1.0
	cfg.MemoryLimitMultiplier = 1.0
	if !r.shouldMaintainGuaranteedQoS(cfg) {
		t.Fatalf("expected true after setting limit multipliers to 1.0 and additions 0")
	}
}

// TestApplyMinimumConstraints ensures usage-based floor logic works
func TestApplyMinimumConstraints(t *testing.T) {
	cfg := config.GetDefaults()
	cfg.MinCPURequest = 50
	cfg.MinMemoryRequest = 64
	r := newAdaptiveTestRig(cfg)
	// Near-zero usage forces min
	cpu := r.applyMinimumCpuConstraints(metrics.Metrics{CPUMilli: 0.05}, 10, cfg)
	if cpu != cfg.MinCPURequest {
		t.Fatalf("expected CPU min %d got %d", cfg.MinCPURequest, cpu)
	}
	// Non-zero usage enforces 20% buffer
	cpu2 := r.applyMinimumCpuConstraints(metrics.Metrics{CPUMilli: 100}, 90, cfg) // 100*1.2=120
	if cpu2 < 120 {
		t.Fatalf("expected usage-based CPU floor >=120 got %d", cpu2)
	}

	mem := r.applyMinimumMemoryConstraints(metrics.Metrics{MemMB: 0.5}, 10, cfg)
	if mem != cfg.MinMemoryRequest {
		t.Fatalf("expected Mem min %d got %d", cfg.MinMemoryRequest, mem)
	}
	mem2 := r.applyMinimumMemoryConstraints(metrics.Metrics{MemMB: 200}, 150, cfg) // 200*1.2=240
	if mem2 < 240 {
		t.Fatalf("expected usage-based Mem floor >=240 got %d", mem2)
	}
}

// TestApplyMaximumLimits checks caps and lower-bound enforcement
func TestApplyMaximumLimits(t *testing.T) {
	cfg := config.GetDefaults()
	cfg.MaxCPULimit = 500
	cfg.MaxMemoryLimit = 1024
	r := newAdaptiveTestRig(cfg)
	cpuLimit := r.applyMaximumCpuLimits(300, 800, cfg) // cap
	if cpuLimit != 500 {
		t.Fatalf("expected CPU limit capped to 500 got %d", cpuLimit)
	}
	cpuLimit2 := r.applyMaximumCpuLimits(300, 200, cfg) // less than request -> raise
	if cpuLimit2 != 300 {
		t.Fatalf("expected CPU limit raised to request 300 got %d", cpuLimit2)
	}

	memLimit := r.applyMaximumMemoryLimits(400, 3000, cfg)
	if memLimit != 1024 {
		t.Fatalf("expected memory limit capped to 1024 got %d", memLimit)
	}
	memLimit2 := r.applyMaximumMemoryLimits(400, 200, cfg) // less than request
	if memLimit2 != 400 {
		t.Fatalf("expected memory limit raised to request 400 got %d", memLimit2)
	}
}

// TestNeedsAdjustmentWithDecision verifies 10% threshold logic after decision
func TestNeedsAdjustmentWithDecision(t *testing.T) {
	r := newAdaptiveTestRig(config.GetDefaults())
	current := corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("100Mi")}}
	// Small 5% change CPU + memory below threshold
	small := corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("105m"), corev1.ResourceMemory: resource.MustParse("105Mi")}}
	if r.needsAdjustmentWithDecision(current, small, ResourceScalingDecision{CPU: ScaleUp, Memory: ScaleUp}) {
		t.Fatalf("should not need adjustment for <10%% change")
	}
	// 12% CPU increase triggers adjustment
	big := corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("112m"), corev1.ResourceMemory: resource.MustParse("100Mi")}}
	if !r.needsAdjustmentWithDecision(current, big, ResourceScalingDecision{CPU: ScaleUp}) {
		t.Fatalf("expected adjustment for >10%% CPU change")
	}
	// Decision None returns false regardless of diff
	if r.needsAdjustmentWithDecision(current, big, ResourceScalingDecision{CPU: ScaleNone, Memory: ScaleNone}) {
		t.Fatalf("expected false when decision says no scaling")
	}
}

// TestGetAdjustmentReasonWithDecision ensures descriptive reason composition
func TestGetAdjustmentReasonWithDecision(t *testing.T) {
	r := newAdaptiveTestRig(config.GetDefaults())
	current := corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("100Mi")}}
	target := corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("120m"), corev1.ResourceMemory: resource.MustParse("80Mi")}}
	reason := r.getAdjustmentReasonWithDecision(current, target, ResourceScalingDecision{CPU: ScaleUp, Memory: ScaleDown})
	if !strings.Contains(reason, "CPU scale up") || !strings.Contains(reason, "Memory scale down") {
		t.Fatalf("unexpected reason: %s", reason)
	}
	none := r.getAdjustmentReasonWithDecision(current, target, ResourceScalingDecision{CPU: ScaleNone, Memory: ScaleNone})
	if none != "Resource optimization" {
		t.Fatalf("expected default optimization reason got %s", none)
	}
}

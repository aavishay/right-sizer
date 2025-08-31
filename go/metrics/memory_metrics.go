// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog/v2"
)

// MemoryMetrics holds all memory-specific Prometheus metrics
type MemoryMetrics struct {
	// Core memory metrics
	PodMemoryUsageBytes      *prometheus.GaugeVec
	PodMemoryWorkingSetBytes *prometheus.GaugeVec
	PodMemoryRSSBytes        *prometheus.GaugeVec
	PodMemoryCacheBytes      *prometheus.GaugeVec
	PodMemorySwapBytes       *prometheus.GaugeVec

	// Memory limits and requests
	PodMemoryLimitBytes   *prometheus.GaugeVec
	PodMemoryRequestBytes *prometheus.GaugeVec

	// Memory utilization metrics
	PodMemoryUtilizationPercentage *prometheus.GaugeVec
	PodMemoryRequestUtilization    *prometheus.GaugeVec
	PodMemoryLimitUtilization      *prometheus.GaugeVec

	// Memory recommendation metrics
	MemoryRecommendationBytes *prometheus.GaugeVec
	MemoryRecommendationRatio *prometheus.GaugeVec

	// Memory pressure metrics
	MemoryPressureEvents   *prometheus.CounterVec
	MemoryPressureLevel    *prometheus.GaugeVec
	MemoryOOMKillEvents    *prometheus.CounterVec
	MemoryThrottlingEvents *prometheus.CounterVec

	// Memory trend metrics
	MemoryTrendSlope        *prometheus.GaugeVec
	MemoryPeakUsageBytes    *prometheus.GaugeVec
	MemoryAverageUsageBytes *prometheus.GaugeVec

	// Memory efficiency metrics
	MemoryWasteBytes      *prometheus.GaugeVec
	MemoryEfficiencyScore *prometheus.GaugeVec

	// Container-specific memory metrics
	ContainerMemoryUsageBytes      *prometheus.GaugeVec
	ContainerMemoryWorkingSetBytes *prometheus.GaugeVec
	ContainerMemoryRSSBytes        *prometheus.GaugeVec
	ContainerMemoryCacheBytes      *prometheus.GaugeVec

	// Memory allocation metrics
	MemoryAllocationFailures *prometheus.CounterVec
	MemoryResizeOperations   *prometheus.CounterVec
	MemoryResizeSuccessRate  *prometheus.GaugeVec
}

// MemoryPressureLevel represents different levels of memory pressure
type MemoryPressureLevel int

const (
	MemoryPressureNone MemoryPressureLevel = iota
	MemoryPressureLow
	MemoryPressureMedium
	MemoryPressureHigh
	MemoryPressureCritical
)

// String returns the string representation of memory pressure level
func (m MemoryPressureLevel) String() string {
	switch m {
	case MemoryPressureNone:
		return "none"
	case MemoryPressureLow:
		return "low"
	case MemoryPressureMedium:
		return "medium"
	case MemoryPressureHigh:
		return "high"
	case MemoryPressureCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// NewMemoryMetrics creates and registers all memory-specific Prometheus metrics
func NewMemoryMetrics() *MemoryMetrics {
	metrics := &MemoryMetrics{
		PodMemoryUsageBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_pod_memory_usage_bytes",
				Help: "Current memory usage in bytes for pods",
			},
			[]string{"namespace", "pod", "container"},
		),

		PodMemoryWorkingSetBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_pod_memory_working_set_bytes",
				Help: "Current working set memory in bytes for pods",
			},
			[]string{"namespace", "pod", "container"},
		),

		PodMemoryRSSBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_pod_memory_rss_bytes",
				Help: "Current RSS (Resident Set Size) memory in bytes for pods",
			},
			[]string{"namespace", "pod", "container"},
		),

		PodMemoryCacheBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_pod_memory_cache_bytes",
				Help: "Current cache memory in bytes for pods",
			},
			[]string{"namespace", "pod", "container"},
		),

		PodMemorySwapBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_pod_memory_swap_bytes",
				Help: "Current swap memory usage in bytes for pods",
			},
			[]string{"namespace", "pod", "container"},
		),

		PodMemoryLimitBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_pod_memory_limit_bytes",
				Help: "Memory limit in bytes for pods",
			},
			[]string{"namespace", "pod", "container"},
		),

		PodMemoryRequestBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_pod_memory_request_bytes",
				Help: "Memory request in bytes for pods",
			},
			[]string{"namespace", "pod", "container"},
		),

		PodMemoryUtilizationPercentage: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_memory_utilization_percentage",
				Help: "Memory utilization percentage (usage/limit)",
			},
			[]string{"namespace", "pod", "container"},
		),

		PodMemoryRequestUtilization: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_memory_request_utilization",
				Help: "Memory request utilization ratio (usage/request)",
			},
			[]string{"namespace", "pod", "container"},
		),

		PodMemoryLimitUtilization: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_memory_limit_utilization",
				Help: "Memory limit utilization ratio (usage/limit)",
			},
			[]string{"namespace", "pod", "container"},
		),

		MemoryRecommendationBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_memory_recommendation_bytes",
				Help: "Recommended memory allocation in bytes",
			},
			[]string{"namespace", "pod", "container", "recommendation_type"},
		),

		MemoryRecommendationRatio: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_memory_recommendation_ratio",
				Help: "Ratio of recommended memory to current allocation",
			},
			[]string{"namespace", "pod", "container", "recommendation_type"},
		),

		MemoryPressureEvents: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rightsizer_memory_pressure_events_total",
				Help: "Total number of memory pressure events detected",
			},
			[]string{"namespace", "pod", "container", "pressure_level"},
		),

		MemoryPressureLevel: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_memory_pressure_level",
				Help: "Current memory pressure level (0=none, 1=low, 2=medium, 3=high, 4=critical)",
			},
			[]string{"namespace", "pod", "container"},
		),

		MemoryOOMKillEvents: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rightsizer_memory_oom_kill_events_total",
				Help: "Total number of OOM kill events",
			},
			[]string{"namespace", "pod", "container"},
		),

		MemoryThrottlingEvents: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rightsizer_memory_throttling_events_total",
				Help: "Total number of memory throttling events",
			},
			[]string{"namespace", "pod", "container"},
		),

		MemoryTrendSlope: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_memory_trend_slope",
				Help: "Memory usage trend slope (positive=increasing, negative=decreasing)",
			},
			[]string{"namespace", "pod", "container"},
		),

		MemoryPeakUsageBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_memory_peak_usage_bytes",
				Help: "Peak memory usage observed in bytes",
			},
			[]string{"namespace", "pod", "container", "time_window"},
		),

		MemoryAverageUsageBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_memory_average_usage_bytes",
				Help: "Average memory usage in bytes over time window",
			},
			[]string{"namespace", "pod", "container", "time_window"},
		),

		MemoryWasteBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_memory_waste_bytes",
				Help: "Wasted memory in bytes (allocated but unused)",
			},
			[]string{"namespace", "pod", "container"},
		),

		MemoryEfficiencyScore: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_memory_efficiency_score",
				Help: "Memory efficiency score (0-100, higher is better)",
			},
			[]string{"namespace", "pod", "container"},
		),

		ContainerMemoryUsageBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_container_memory_usage_bytes",
				Help: "Container-level memory usage in bytes",
			},
			[]string{"namespace", "pod", "container"},
		),

		ContainerMemoryWorkingSetBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_container_memory_working_set_bytes",
				Help: "Container-level working set memory in bytes",
			},
			[]string{"namespace", "pod", "container"},
		),

		ContainerMemoryRSSBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_container_memory_rss_bytes",
				Help: "Container-level RSS memory in bytes",
			},
			[]string{"namespace", "pod", "container"},
		),

		ContainerMemoryCacheBytes: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_container_memory_cache_bytes",
				Help: "Container-level cache memory in bytes",
			},
			[]string{"namespace", "pod", "container"},
		),

		MemoryAllocationFailures: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rightsizer_memory_allocation_failures_total",
				Help: "Total number of memory allocation failures",
			},
			[]string{"namespace", "pod", "container", "reason"},
		),

		MemoryResizeOperations: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rightsizer_memory_resize_operations_total",
				Help: "Total number of memory resize operations",
			},
			[]string{"namespace", "pod", "container", "direction", "result"},
		),

		MemoryResizeSuccessRate: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_memory_resize_success_rate",
				Help: "Success rate of memory resize operations",
			},
			[]string{"namespace", "pod", "container"},
		),
	}

	// Register all metrics
	prometheus.MustRegister(
		metrics.PodMemoryUsageBytes,
		metrics.PodMemoryWorkingSetBytes,
		metrics.PodMemoryRSSBytes,
		metrics.PodMemoryCacheBytes,
		metrics.PodMemorySwapBytes,
		metrics.PodMemoryLimitBytes,
		metrics.PodMemoryRequestBytes,
		metrics.PodMemoryUtilizationPercentage,
		metrics.PodMemoryRequestUtilization,
		metrics.PodMemoryLimitUtilization,
		metrics.MemoryRecommendationBytes,
		metrics.MemoryRecommendationRatio,
		metrics.MemoryPressureEvents,
		metrics.MemoryPressureLevel,
		metrics.MemoryOOMKillEvents,
		metrics.MemoryThrottlingEvents,
		metrics.MemoryTrendSlope,
		metrics.MemoryPeakUsageBytes,
		metrics.MemoryAverageUsageBytes,
		metrics.MemoryWasteBytes,
		metrics.MemoryEfficiencyScore,
		metrics.ContainerMemoryUsageBytes,
		metrics.ContainerMemoryWorkingSetBytes,
		metrics.ContainerMemoryRSSBytes,
		metrics.ContainerMemoryCacheBytes,
		metrics.MemoryAllocationFailures,
		metrics.MemoryResizeOperations,
		metrics.MemoryResizeSuccessRate,
	)

	return metrics
}

// UpdatePodMemoryMetrics updates all memory metrics for a pod
func (m *MemoryMetrics) UpdatePodMemoryMetrics(namespace, pod, container string, usage, workingSet, rss, cache, swap, limit, request float64) {
	m.PodMemoryUsageBytes.WithLabelValues(namespace, pod, container).Set(usage)
	m.PodMemoryWorkingSetBytes.WithLabelValues(namespace, pod, container).Set(workingSet)
	m.PodMemoryRSSBytes.WithLabelValues(namespace, pod, container).Set(rss)
	m.PodMemoryCacheBytes.WithLabelValues(namespace, pod, container).Set(cache)
	m.PodMemorySwapBytes.WithLabelValues(namespace, pod, container).Set(swap)
	m.PodMemoryLimitBytes.WithLabelValues(namespace, pod, container).Set(limit)
	m.PodMemoryRequestBytes.WithLabelValues(namespace, pod, container).Set(request)

	// Calculate utilization percentages
	if limit > 0 {
		utilizationPct := (usage / limit) * 100
		m.PodMemoryUtilizationPercentage.WithLabelValues(namespace, pod, container).Set(utilizationPct)
		m.PodMemoryLimitUtilization.WithLabelValues(namespace, pod, container).Set(usage / limit)
	}

	if request > 0 {
		m.PodMemoryRequestUtilization.WithLabelValues(namespace, pod, container).Set(usage / request)
	}

	// Calculate waste and efficiency
	if limit > 0 && usage > 0 {
		waste := limit - usage
		m.MemoryWasteBytes.WithLabelValues(namespace, pod, container).Set(waste)

		efficiency := (usage / limit) * 100
		if efficiency > 100 {
			efficiency = 100
		}
		m.MemoryEfficiencyScore.WithLabelValues(namespace, pod, container).Set(efficiency)
	}
}

// RecordMemoryRecommendation records a memory sizing recommendation
func (m *MemoryMetrics) RecordMemoryRecommendation(namespace, pod, container, recommendationType string, recommendedBytes, currentBytes float64) {
	m.MemoryRecommendationBytes.WithLabelValues(namespace, pod, container, recommendationType).Set(recommendedBytes)

	if currentBytes > 0 {
		ratio := recommendedBytes / currentBytes
		m.MemoryRecommendationRatio.WithLabelValues(namespace, pod, container, recommendationType).Set(ratio)
	}
}

// DetectAndRecordMemoryPressure detects memory pressure and records metrics
func (m *MemoryMetrics) DetectAndRecordMemoryPressure(namespace, pod, container string, usage, limit float64) MemoryPressureLevel {
	if limit <= 0 {
		return MemoryPressureNone
	}

	utilizationRatio := usage / limit
	var pressureLevel MemoryPressureLevel

	switch {
	case utilizationRatio >= 0.95:
		pressureLevel = MemoryPressureCritical
		LogMemoryPressure(namespace, pod, container, "CRITICAL", usage, limit, utilizationRatio)
	case utilizationRatio >= 0.90:
		pressureLevel = MemoryPressureHigh
		LogMemoryPressure(namespace, pod, container, "HIGH", usage, limit, utilizationRatio)
	case utilizationRatio >= 0.80:
		pressureLevel = MemoryPressureMedium
		LogMemoryPressure(namespace, pod, container, "MEDIUM", usage, limit, utilizationRatio)
	case utilizationRatio >= 0.70:
		pressureLevel = MemoryPressureLow
		LogMemoryPressure(namespace, pod, container, "LOW", usage, limit, utilizationRatio)
	default:
		pressureLevel = MemoryPressureNone
	}

	if pressureLevel != MemoryPressureNone {
		m.MemoryPressureEvents.WithLabelValues(namespace, pod, container, pressureLevel.String()).Inc()
	}

	m.MemoryPressureLevel.WithLabelValues(namespace, pod, container).Set(float64(pressureLevel))

	return pressureLevel
}

// RecordOOMKill records an OOM kill event
func (m *MemoryMetrics) RecordOOMKill(namespace, pod, container string) {
	m.MemoryOOMKillEvents.WithLabelValues(namespace, pod, container).Inc()
	LogOOMKill(namespace, pod, container)
}

// RecordMemoryThrottling records a memory throttling event
func (m *MemoryMetrics) RecordMemoryThrottling(namespace, pod, container string) {
	m.MemoryThrottlingEvents.WithLabelValues(namespace, pod, container).Inc()
	LogMemoryThrottling(namespace, pod, container)
}

// UpdateMemoryTrend updates memory trend metrics
func (m *MemoryMetrics) UpdateMemoryTrend(namespace, pod, container string, slope, peak, average float64, timeWindow string) {
	m.MemoryTrendSlope.WithLabelValues(namespace, pod, container).Set(slope)
	m.MemoryPeakUsageBytes.WithLabelValues(namespace, pod, container, timeWindow).Set(peak)
	m.MemoryAverageUsageBytes.WithLabelValues(namespace, pod, container, timeWindow).Set(average)

	if slope > 0.1 {
		LogMemoryTrend(namespace, pod, container, "INCREASING", slope, peak, average)
	} else if slope < -0.1 {
		LogMemoryTrend(namespace, pod, container, "DECREASING", slope, peak, average)
	}
}

// RecordMemoryResize records a memory resize operation
func (m *MemoryMetrics) RecordMemoryResize(namespace, pod, container, direction, result string) {
	m.MemoryResizeOperations.WithLabelValues(namespace, pod, container, direction, result).Inc()
	LogMemoryResize(namespace, pod, container, direction, result)
}

// UpdateContainerMemoryMetrics updates container-specific memory metrics
func (m *MemoryMetrics) UpdateContainerMemoryMetrics(namespace, pod, container string, usage, workingSet, rss, cache float64) {
	m.ContainerMemoryUsageBytes.WithLabelValues(namespace, pod, container).Set(usage)
	m.ContainerMemoryWorkingSetBytes.WithLabelValues(namespace, pod, container).Set(workingSet)
	m.ContainerMemoryRSSBytes.WithLabelValues(namespace, pod, container).Set(rss)
	m.ContainerMemoryCacheBytes.WithLabelValues(namespace, pod, container).Set(cache)
}

// Memory Pressure Logging Functions

// LogMemoryPressure logs memory pressure events with detailed information
func LogMemoryPressure(namespace, pod, container, level string, usage, limit, ratio float64) {
	klog.Warningf("[MEMORY_PRESSURE] %s detected for %s/%s/%s - Usage: %.2fMi/%.2fMi (%.1f%% utilization)",
		level, namespace, pod, container,
		usage/1024/1024, limit/1024/1024, ratio*100)

	// Additional context based on pressure level
	switch level {
	case "CRITICAL":
		klog.Errorf("[MEMORY_CRITICAL] Pod %s/%s at risk of OOM kill - Immediate action required", namespace, pod)
	case "HIGH":
		klog.Warningf("[MEMORY_HIGH] Pod %s/%s approaching memory limit - Consider increasing allocation", namespace, pod)
	}
}

// LogOOMKill logs OOM kill events
func LogOOMKill(namespace, pod, container string) {
	klog.Errorf("[OOM_KILL] Container %s/%s/%s was OOM killed - Memory allocation insufficient",
		namespace, pod, container)
}

// LogMemoryThrottling logs memory throttling events
func LogMemoryThrottling(namespace, pod, container string) {
	klog.Warningf("[MEMORY_THROTTLING] Container %s/%s/%s experiencing memory throttling - Performance degraded",
		namespace, pod, container)
}

// LogMemoryTrend logs memory usage trends
func LogMemoryTrend(namespace, pod, container, trend string, slope, peak, average float64) {
	klog.Infof("[MEMORY_TREND] %s trend for %s/%s/%s - Slope: %.2f, Peak: %.2fMi, Avg: %.2fMi",
		trend, namespace, pod, container,
		slope, peak/1024/1024, average/1024/1024)

	if trend == "INCREASING" && slope > 0.5 {
		klog.Warningf("[MEMORY_LEAK] Potential memory leak detected in %s/%s/%s - Rapid increase in memory usage",
			namespace, pod, container)
	}
}

// LogMemoryResize logs memory resize operations
func LogMemoryResize(namespace, pod, container, direction, result string) {
	if result == "success" {
		klog.Infof("[MEMORY_RESIZE] Successfully %s memory for %s/%s/%s",
			direction, namespace, pod, container)
	} else {
		klog.Errorf("[MEMORY_RESIZE] Failed to %s memory for %s/%s/%s: %s",
			direction, namespace, pod, container, result)
	}
}

// LogMemoryAllocation logs memory allocation events
func LogMemoryAllocation(namespace, pod, container string, requested, allocated, usage float64) {
	efficiency := (usage / allocated) * 100
	klog.Infof("[MEMORY_ALLOCATION] %s/%s/%s - Requested: %.2fMi, Allocated: %.2fMi, Used: %.2fMi (%.1f%% efficiency)",
		namespace, pod, container,
		requested/1024/1024, allocated/1024/1024, usage/1024/1024, efficiency)
}

// LogMemoryRecommendation logs memory sizing recommendations
func LogMemoryRecommendation(namespace, pod, container string, current, recommended float64, reason string) {
	changePercent := ((recommended - current) / current) * 100
	klog.Infof("[MEMORY_RECOMMENDATION] %s/%s/%s - Current: %.2fMi, Recommended: %.2fMi (%.1f%% change) - Reason: %s",
		namespace, pod, container,
		current/1024/1024, recommended/1024/1024, changePercent, reason)
}

// AnalyzeMemoryPattern analyzes memory usage patterns
func AnalyzeMemoryPattern(namespace, pod, container string, samples []float64) string {
	if len(samples) < 2 {
		return "insufficient_data"
	}

	// Calculate basic statistics
	var sum, max, min float64
	min = samples[0]
	for _, v := range samples {
		sum += v
		if v > max {
			max = v
		}
		if v < min {
			min = v
		}
	}
	avg := sum / float64(len(samples))

	// Detect patterns
	var pattern string
	variance := max - min
	varianceRatio := variance / avg

	switch {
	case varianceRatio < 0.1:
		pattern = "stable"
		klog.V(4).Infof("[MEMORY_PATTERN] %s/%s/%s - Stable memory usage pattern detected", namespace, pod, container)
	case varianceRatio < 0.3:
		pattern = "moderate_variance"
		klog.V(4).Infof("[MEMORY_PATTERN] %s/%s/%s - Moderate variance in memory usage", namespace, pod, container)
	case varianceRatio < 0.5:
		pattern = "high_variance"
		klog.Infof("[MEMORY_PATTERN] %s/%s/%s - High variance in memory usage - Consider buffer", namespace, pod, container)
	default:
		pattern = "erratic"
		klog.Warningf("[MEMORY_PATTERN] %s/%s/%s - Erratic memory usage pattern - Investigate application behavior", namespace, pod, container)
	}

	// Check for memory leak
	if len(samples) > 10 {
		// Simple linear regression to detect trend
		var sumX, sumY, sumXY, sumX2 float64
		n := float64(len(samples))
		for i, y := range samples {
			x := float64(i)
			sumX += x
			sumY += y
			sumXY += x * y
			sumX2 += x * x
		}
		slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)

		if slope > avg*0.01 { // Growing more than 1% of average per sample
			pattern = "potential_leak"
			klog.Warningf("[MEMORY_LEAK_DETECTION] %s/%s/%s - Potential memory leak detected, slope: %.2f",
				namespace, pod, container, slope)
		}
	}

	return pattern
}

// FormatMemorySize formats memory size in bytes to human-readable format
func FormatMemorySize(bytes float64) string {
	units := []string{"B", "Ki", "Mi", "Gi", "Ti"}
	size := bytes
	unitIndex := 0

	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}

	return fmt.Sprintf("%.2f%s", size, units[unitIndex])
}

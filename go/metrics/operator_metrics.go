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
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// OperatorMetrics holds all Prometheus metrics for the right-sizer operator
type OperatorMetrics struct {
	// Pod processing metrics
	PodsProcessedTotal  prometheus.Counter
	PodsResizedTotal    *prometheus.CounterVec
	PodsSkippedTotal    *prometheus.CounterVec
	PodProcessingErrors *prometheus.CounterVec

	// Resource adjustment metrics
	CPUAdjustmentsTotal    *prometheus.CounterVec
	MemoryAdjustmentsTotal *prometheus.CounterVec
	ResourceChangeSize     *prometheus.HistogramVec

	// Performance metrics
	ProcessingDuration        *prometheus.HistogramVec
	APICallDuration           *prometheus.HistogramVec
	MetricsCollectionDuration prometheus.Histogram

	// Safety and validation metrics
	SafetyThresholdViolations *prometheus.CounterVec
	ResourceValidationErrors  *prometheus.CounterVec

	// Retry and error metrics
	RetryAttemptsTotal *prometheus.CounterVec
	RetrySuccessTotal  *prometheus.CounterVec

	// Cluster resource metrics
	ClusterResourceUtilization *prometheus.GaugeVec
	NodeResourceAvailability   *prometheus.GaugeVec

	// Policy and configuration metrics
	PolicyRuleApplications *prometheus.CounterVec
	ConfigurationReloads   prometheus.Counter

	// Historical trend metrics
	ResourceTrendPredictions *prometheus.GaugeVec
	HistoricalDataPoints     prometheus.Gauge

	// Aggregate metrics gauges
	CPUUsagePercent         prometheus.Gauge // rightsizer_cpu_usage_percent
	MemoryUsagePercent      prometheus.Gauge // rightsizer_memory_usage_percent
	ActivePodsTotal         prometheus.Gauge // rightsizer_active_pods_total
	OptimizedResourcesTotal prometheus.Gauge // rightsizer_optimized_resources_total
	NetworkUsageMbps        prometheus.Gauge // rightsizer_network_usage_mbps
	DiskIOMBps              prometheus.Gauge // rightsizer_disk_io_mbps
	AvgUtilizationPercent   prometheus.Gauge // rightsizer_avg_utilization_percent
}

var (
	operatorMetricsInstance *OperatorMetrics
	operatorMetricsOnce     sync.Once
)

// NewOperatorMetrics creates and registers all Prometheus metrics
// Uses singleton pattern to prevent duplicate registration
func NewOperatorMetrics() *OperatorMetrics {
	operatorMetricsOnce.Do(func() {
		operatorMetricsInstance = createOperatorMetrics()
	})
	return operatorMetricsInstance
}

// createOperatorMetrics creates and registers all Prometheus metrics (internal)
func createOperatorMetrics() *OperatorMetrics {
	metrics := &OperatorMetrics{
		PodsProcessedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "rightsizer_pods_processed_total",
			Help: "Total number of pods processed by the right-sizer operator",
		}),

		PodsResizedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rightsizer_pods_resized_total",
				Help: "Total number of pods that were resized",
			},
			[]string{"namespace", "pod_name", "container_name", "resize_type"},
		),

		PodsSkippedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rightsizer_pods_skipped_total",
				Help: "Total number of pods that were skipped from resizing",
			},
			[]string{"namespace", "pod_name", "reason"},
		),

		PodProcessingErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rightsizer_pod_processing_errors_total",
				Help: "Total number of errors encountered while processing pods",
			},
			[]string{"namespace", "pod_name", "error_type"},
		),

		CPUAdjustmentsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rightsizer_cpu_adjustments_total",
				Help: "Total number of CPU resource adjustments made",
			},
			[]string{"namespace", "pod_name", "container_name", "direction"},
		),

		MemoryAdjustmentsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rightsizer_memory_adjustments_total",
				Help: "Total number of memory resource adjustments made",
			},
			[]string{"namespace", "pod_name", "container_name", "direction"},
		),

		ResourceChangeSize: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rightsizer_resource_change_percentage",
				Help:    "Distribution of resource change percentages",
				Buckets: prometheus.LinearBuckets(0, 10, 11), // 0%, 10%, 20%, ..., 100%
			},
			[]string{"resource_type", "direction"},
		),

		ProcessingDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rightsizer_processing_duration_seconds",
				Help:    "Time spent processing pods for right-sizing",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"},
		),

		APICallDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "rightsizer_api_call_duration_seconds",
				Help:    "Duration of Kubernetes API calls",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"api_endpoint", "method"},
		),

		MetricsCollectionDuration: prometheus.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "rightsizer_metrics_collection_duration_seconds",
				Help:    "Time spent collecting metrics from metrics providers",
				Buckets: prometheus.DefBuckets,
			},
		),

		SafetyThresholdViolations: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rightsizer_safety_threshold_violations_total",
				Help: "Total number of times safety threshold was violated",
			},
			[]string{"namespace", "pod_name", "resource_type"},
		),

		ResourceValidationErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rightsizer_resource_validation_errors_total",
				Help: "Total number of resource validation errors",
			},
			[]string{"validation_type", "error_reason"},
		),

		RetryAttemptsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rightsizer_retry_attempts_total",
				Help: "Total number of retry attempts for operations",
			},
			[]string{"operation", "attempt_number"},
		),

		RetrySuccessTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rightsizer_retry_success_total",
				Help: "Total number of successful retries",
			},
			[]string{"operation"},
		),

		ClusterResourceUtilization: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_cluster_resource_utilization_ratio",
				Help: "Current cluster resource utilization ratio",
			},
			[]string{"resource_type", "node_name"},
		),

		NodeResourceAvailability: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_node_resource_availability",
				Help: "Available resources on cluster nodes",
			},
			[]string{"resource_type", "node_name"},
		),

		PolicyRuleApplications: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "rightsizer_policy_rule_applications_total",
				Help: "Total number of policy rule applications",
			},
			[]string{"policy_name", "rule_type", "result"},
		),

		ConfigurationReloads: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "rightsizer_configuration_reloads_total",
			Help: "Total number of configuration reloads",
		}),

		ResourceTrendPredictions: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "rightsizer_resource_trend_predictions",
				Help: "Predicted resource requirements based on historical trends",
			},
			[]string{"namespace", "pod_name", "container_name", "resource_type", "prediction_horizon"},
		),

		HistoricalDataPoints: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "rightsizer_historical_data_points",
			Help: "Number of historical data points stored",
		}),

		// Aggregate metrics gauges
		CPUUsagePercent: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "rightsizer_cpu_usage_percent",
			Help: "Current average CPU usage percent across managed pods",
		}),
		MemoryUsagePercent: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "rightsizer_memory_usage_percent",
			Help: "Current average memory usage percent across managed pods",
		}),
		ActivePodsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "rightsizer_active_pods_total",
			Help: "Number of active (non-terminating) pods considered by the operator",
		}),
		OptimizedResourcesTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "rightsizer_optimized_resources_total",
			Help: "Total number of resource optimization actions applied",
		}),
		NetworkUsageMbps: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "rightsizer_network_usage_mbps",
			Help: "Estimated aggregate network usage (simulated or collected)",
		}),
		DiskIOMBps: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "rightsizer_disk_io_mbps",
			Help: "Estimated aggregate disk IO in MB/s (simulated or collected)",
		}),
		AvgUtilizationPercent: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "rightsizer_avg_utilization_percent",
			Help: "Average combined resource (CPU/Memory) utilization percent",
		}),
	}

	// Register all metrics with safe registration (handles already registered errors)
	safeRegister(
		metrics.PodsProcessedTotal,
		metrics.PodsResizedTotal,
		metrics.PodsSkippedTotal,
		metrics.PodProcessingErrors,
		metrics.CPUAdjustmentsTotal,
		metrics.MemoryAdjustmentsTotal,
		metrics.ResourceChangeSize,
		metrics.ProcessingDuration,
		metrics.APICallDuration,
		metrics.MetricsCollectionDuration,
		metrics.SafetyThresholdViolations,
		metrics.ResourceValidationErrors,
		metrics.RetryAttemptsTotal,
		metrics.RetrySuccessTotal,
		metrics.ClusterResourceUtilization,
		metrics.NodeResourceAvailability,
		metrics.PolicyRuleApplications,
		metrics.ConfigurationReloads,
		metrics.ResourceTrendPredictions,
		metrics.HistoricalDataPoints,
		metrics.CPUUsagePercent,
		metrics.MemoryUsagePercent,
		metrics.ActivePodsTotal,
		metrics.OptimizedResourcesTotal,
		metrics.NetworkUsageMbps,
		metrics.DiskIOMBps,
		metrics.AvgUtilizationPercent,
	)

	return metrics
}

// safeRegister registers Prometheus collectors, ignoring AlreadyRegisteredError
func safeRegister(collectors ...prometheus.Collector) {
	for _, collector := range collectors {
		if err := prometheus.Register(collector); err != nil {
			// Check if the error is because the metric is already registered
			if _, ok := err.(prometheus.AlreadyRegisteredError); !ok {
				// If it's not an AlreadyRegisteredError, log it but don't panic
				// This allows the operator to continue even if some metrics fail to register
				// In production, you might want to use a proper logger here
				// For now, we'll silently continue to prevent crashes
				continue
			}
			// If it's already registered, that's fine - just continue
		}
	}
}

// UpdateMetrics sets the aggregate gauges used by the metrics API.
// Any negative value parameters are ignored (allowing partial updates).
func (m *OperatorMetrics) UpdateMetrics(
	activePods int,
	optimizedResources int,
	cpuUsagePercent float64,
	memoryUsagePercent float64,
	networkMbps float64,
	diskIOMBps float64,
	avgUtilizationPercent float64,
) {
	if m == nil {
		return
	}
	if activePods >= 0 {
		m.ActivePodsTotal.Set(float64(activePods))
	}
	if optimizedResources >= 0 {
		m.OptimizedResourcesTotal.Set(float64(optimizedResources))
	}
	if cpuUsagePercent >= 0 {
		m.CPUUsagePercent.Set(cpuUsagePercent)
	}
	if memoryUsagePercent >= 0 {
		m.MemoryUsagePercent.Set(memoryUsagePercent)
	}
	if networkMbps >= 0 {
		m.NetworkUsageMbps.Set(networkMbps)
	}
	if diskIOMBps >= 0 {
		m.DiskIOMBps.Set(diskIOMBps)
	}
	if avgUtilizationPercent >= 0 {
		m.AvgUtilizationPercent.Set(avgUtilizationPercent)
	}
}

// RecordPodProcessed records that a pod has been processed
func (m *OperatorMetrics) RecordPodProcessed() {
	m.PodsProcessedTotal.Inc()
}

// RecordPodResized records that a pod has been resized
func (m *OperatorMetrics) RecordPodResized(namespace, podName, containerName, resizeType string) {
	m.PodsResizedTotal.WithLabelValues(namespace, podName, containerName, resizeType).Inc()
}

// RecordPodSkipped records that a pod was skipped
func (m *OperatorMetrics) RecordPodSkipped(namespace, podName, reason string) {
	m.PodsSkippedTotal.WithLabelValues(namespace, podName, reason).Inc()
}

// RecordProcessingError records a processing error
func (m *OperatorMetrics) RecordProcessingError(namespace, podName, errorType string) {
	m.PodProcessingErrors.WithLabelValues(namespace, podName, errorType).Inc()
}

// RecordResourceAdjustment records a resource adjustment
func (m *OperatorMetrics) RecordResourceAdjustment(namespace, podName, containerName, resourceType, direction string, changePercentage float64) {
	if resourceType == "cpu" {
		m.CPUAdjustmentsTotal.WithLabelValues(namespace, podName, containerName, direction).Inc()
	} else if resourceType == "memory" {
		m.MemoryAdjustmentsTotal.WithLabelValues(namespace, podName, containerName, direction).Inc()
	}

	m.ResourceChangeSize.WithLabelValues(resourceType, direction).Observe(changePercentage)
}

// RecordProcessingDuration records the duration of a processing operation
func (m *OperatorMetrics) RecordProcessingDuration(operation string, duration time.Duration) {
	m.ProcessingDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// RecordAPICall records the duration of an API call
func (m *OperatorMetrics) RecordAPICall(endpoint, method string, duration time.Duration) {
	m.APICallDuration.WithLabelValues(endpoint, method).Observe(duration.Seconds())
}

// RecordMetricsCollection records the duration of metrics collection
func (m *OperatorMetrics) RecordMetricsCollection(duration time.Duration) {
	m.MetricsCollectionDuration.Observe(duration.Seconds())
}

// RecordSafetyThresholdViolation records a safety threshold violation
func (m *OperatorMetrics) RecordSafetyThresholdViolation(namespace, podName, resourceType string) {
	m.SafetyThresholdViolations.WithLabelValues(namespace, podName, resourceType).Inc()
}

// RecordResourceValidationError records a resource validation error
func (m *OperatorMetrics) RecordResourceValidationError(validationType, errorReason string) {
	m.ResourceValidationErrors.WithLabelValues(validationType, errorReason).Inc()
}

// RecordRetryAttempt records a retry attempt
func (m *OperatorMetrics) RecordRetryAttempt(operation string, attemptNumber int) {
	m.RetryAttemptsTotal.WithLabelValues(operation, strconv.Itoa(attemptNumber)).Inc()
}

// RecordRetrySuccess records a successful retry
func (m *OperatorMetrics) RecordRetrySuccess(operation string) {
	m.RetrySuccessTotal.WithLabelValues(operation).Inc()
}

// UpdateClusterResourceUtilization updates cluster resource utilization metrics
func (m *OperatorMetrics) UpdateClusterResourceUtilization(resourceType, nodeName string, utilization float64) {
	m.ClusterResourceUtilization.WithLabelValues(resourceType, nodeName).Set(utilization)
}

// UpdateNodeResourceAvailability updates node resource availability metrics
func (m *OperatorMetrics) UpdateNodeResourceAvailability(resourceType, nodeName string, available float64) {
	m.NodeResourceAvailability.WithLabelValues(resourceType, nodeName).Set(available)
}

// RecordPolicyRuleApplication records a policy rule application
func (m *OperatorMetrics) RecordPolicyRuleApplication(policyName, ruleType, result string) {
	m.PolicyRuleApplications.WithLabelValues(policyName, ruleType, result).Inc()
}

// RecordConfigurationReload records a configuration reload
func (m *OperatorMetrics) RecordConfigurationReload() {
	m.ConfigurationReloads.Inc()
}

// UpdateResourceTrendPrediction updates resource trend prediction metrics
func (m *OperatorMetrics) UpdateResourceTrendPrediction(namespace, podName, containerName, resourceType, predictionHorizon string, prediction float64) {
	m.ResourceTrendPredictions.WithLabelValues(namespace, podName, containerName, resourceType, predictionHorizon).Set(prediction)
}

// UpdateHistoricalDataPoints updates the count of historical data points
func (m *OperatorMetrics) UpdateHistoricalDataPoints(count float64) {
	m.HistoricalDataPoints.Set(count)
}

// StartMetricsServer starts the Prometheus metrics HTTP server
func StartMetricsServer(port int) error {
	http.Handle("/metrics", promhttp.Handler())

	// Add custom health check for metrics
	http.HandleFunc("/metrics/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("metrics server healthy"))
	})

	return http.ListenAndServe(":"+strconv.Itoa(port), nil)
}

// Timer is a helper for measuring operation durations
type Timer struct {
	start time.Time
}

// NewTimer creates a new timer
func NewTimer() *Timer {
	return &Timer{start: time.Now()}
}

// Duration returns the elapsed duration since the timer was created
func (t *Timer) Duration() time.Duration {
	return time.Since(t.start)
}

// ObserveDuration observes the duration in the given histogram
func (t *Timer) ObserveDuration(observer prometheus.Observer) {
	observer.Observe(t.Duration().Seconds())
}

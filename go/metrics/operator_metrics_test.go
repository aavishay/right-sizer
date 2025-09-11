package metrics_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"right-sizer/metrics"
)

func TestOperatorMetrics_Singleton(t *testing.T) {
	m1 := metrics.NewOperatorMetrics()
	m2 := metrics.NewOperatorMetrics()
	require.NotNil(t, m1)
	require.NotNil(t, m2)
	assert.Same(t, m1, m2, "expected singleton instance")
}

func TestOperatorMetrics_RecordPodProcessed(t *testing.T) {
	m := metrics.NewOperatorMetrics()
	before := testutil.ToFloat64(m.PodsProcessedTotal)
	m.RecordPodProcessed()
	after := testutil.ToFloat64(m.PodsProcessedTotal)
	assert.Equal(t, 1.0, after-before)
}

func TestOperatorMetrics_RecordPodResizedSkippedErrors(t *testing.T) {
	m := metrics.NewOperatorMetrics()

	// Resized
	rBefore := testutil.ToFloat64(m.PodsResizedTotal.WithLabelValues("ns", "podA", "c1", "cpu"))
	m.RecordPodResized("ns", "podA", "c1", "cpu")
	rAfter := testutil.ToFloat64(m.PodsResizedTotal.WithLabelValues("ns", "podA", "c1", "cpu"))
	assert.Equal(t, 1.0, rAfter-rBefore)

	// Skipped
	sBefore := testutil.ToFloat64(m.PodsSkippedTotal.WithLabelValues("ns", "podA", "reasonX"))
	m.RecordPodSkipped("ns", "podA", "reasonX")
	sAfter := testutil.ToFloat64(m.PodsSkippedTotal.WithLabelValues("ns", "podA", "reasonX"))
	assert.Equal(t, 1.0, sAfter-sBefore)

	// Error
	eBefore := testutil.ToFloat64(m.PodProcessingErrors.WithLabelValues("ns", "podA", "timeout"))
	m.RecordProcessingError("ns", "podA", "timeout")
	eAfter := testutil.ToFloat64(m.PodProcessingErrors.WithLabelValues("ns", "podA", "timeout"))
	assert.Equal(t, 1.0, eAfter-eBefore)
}

func TestOperatorMetrics_RecordResourceAdjustment(t *testing.T) {
	m := metrics.NewOperatorMetrics()

	cpuBefore := testutil.ToFloat64(m.CPUAdjustmentsTotal.WithLabelValues("ns", "podA", "c1", "increase"))
	m.RecordResourceAdjustment("ns", "podA", "c1", "cpu", "increase", 25.0)
	cpuAfter := testutil.ToFloat64(m.CPUAdjustmentsTotal.WithLabelValues("ns", "podA", "c1", "increase"))
	assert.Equal(t, 1.0, cpuAfter-cpuBefore)

	memBefore := testutil.ToFloat64(m.MemoryAdjustmentsTotal.WithLabelValues("ns", "podA", "c1", "decrease"))
	m.RecordResourceAdjustment("ns", "podA", "c1", "memory", "decrease", 10.0)
	memAfter := testutil.ToFloat64(m.MemoryAdjustmentsTotal.WithLabelValues("ns", "podA", "c1", "decrease"))
	assert.Equal(t, 1.0, memAfter-memBefore)

	// Histogram vector: count of collected metric families should increase from 0 to 1
	hBefore := testutil.CollectAndCount(m.ResourceChangeSize)
	m.RecordResourceAdjustment("ns", "podA", "c1", "cpu", "increase", 5.0)
	hAfter := testutil.CollectAndCount(m.ResourceChangeSize)
	assert.GreaterOrEqual(t, hAfter, hBefore)
}

func TestOperatorMetrics_Durations(t *testing.T) {
	m := metrics.NewOperatorMetrics()

	m.RecordProcessingDuration("resize", 123*time.Millisecond)
	m.RecordAPICall("/api/v1/pods", "GET", 10*time.Millisecond)
	m.RecordMetricsCollection(8 * time.Millisecond)

	procCount := testutil.CollectAndCount(m.ProcessingDuration)
	apiCount := testutil.CollectAndCount(m.APICallDuration)
	colCount := testutil.CollectAndCount(m.MetricsCollectionDuration)

	assert.Greater(t, procCount, 0)
	assert.Greater(t, apiCount, 0)
	assert.Greater(t, colCount, 0)

	timer := metrics.NewTimer()
	time.Sleep(5 * time.Millisecond)
	elapsed := timer.Duration()
	assert.GreaterOrEqual(t, elapsed, 5*time.Millisecond)
	before := testutil.CollectAndCount(m.MetricsCollectionDuration)
	timer.ObserveDuration(m.MetricsCollectionDuration)
	after := testutil.CollectAndCount(m.MetricsCollectionDuration)
	assert.GreaterOrEqual(t, after, before)
}

func TestOperatorMetrics_SafetyAndValidation(t *testing.T) {
	m := metrics.NewOperatorMetrics()

	sBefore := testutil.ToFloat64(m.SafetyThresholdViolations.WithLabelValues("ns", "podA", "cpu"))
	m.RecordSafetyThresholdViolation("ns", "podA", "cpu")
	sAfter := testutil.ToFloat64(m.SafetyThresholdViolations.WithLabelValues("ns", "podA", "cpu"))
	assert.Equal(t, 1.0, sAfter-sBefore)

	vBefore := testutil.ToFloat64(m.ResourceValidationErrors.WithLabelValues("limitCheck", "tooHigh"))
	m.RecordResourceValidationError("limitCheck", "tooHigh")
	vAfter := testutil.ToFloat64(m.ResourceValidationErrors.WithLabelValues("limitCheck", "tooHigh"))
	assert.Equal(t, 1.0, vAfter-vBefore)
}

func TestOperatorMetrics_Retry(t *testing.T) {
	m := metrics.NewOperatorMetrics()

	aBefore := testutil.ToFloat64(m.RetryAttemptsTotal.WithLabelValues("patchPod", strconv.Itoa(3)))
	m.RecordRetryAttempt("patchPod", 3)
	aAfter := testutil.ToFloat64(m.RetryAttemptsTotal.WithLabelValues("patchPod", strconv.Itoa(3)))
	assert.Equal(t, 1.0, aAfter-aBefore)

	sBefore := testutil.ToFloat64(m.RetrySuccessTotal.WithLabelValues("patchPod"))
	m.RecordRetrySuccess("patchPod")
	sAfter := testutil.ToFloat64(m.RetrySuccessTotal.WithLabelValues("patchPod"))
	assert.Equal(t, 1.0, sAfter-sBefore)
}

func TestOperatorMetrics_ClusterAndNode(t *testing.T) {
	m := metrics.NewOperatorMetrics()

	m.UpdateClusterResourceUtilization("cpu", "node1", 0.75)
	assert.Equal(t, 0.75, testutil.ToFloat64(m.ClusterResourceUtilization.WithLabelValues("cpu", "node1")))

	m.UpdateNodeResourceAvailability("memory", "node1", 2048)
	assert.Equal(t, 2048.0, testutil.ToFloat64(m.NodeResourceAvailability.WithLabelValues("memory", "node1")))
}

func TestOperatorMetrics_PolicyAndConfig(t *testing.T) {
	m := metrics.NewOperatorMetrics()

	before := testutil.ToFloat64(m.PolicyRuleApplications.WithLabelValues("policyA", "scale", "applied"))
	m.RecordPolicyRuleApplication("policyA", "scale", "applied")
	after := testutil.ToFloat64(m.PolicyRuleApplications.WithLabelValues("policyA", "scale", "applied"))
	assert.Equal(t, 1.0, after-before)

	cBefore := testutil.ToFloat64(m.ConfigurationReloads)
	m.RecordConfigurationReload()
	cAfter := testutil.ToFloat64(m.ConfigurationReloads)
	assert.Equal(t, 1.0, cAfter-cBefore)
}

func TestOperatorMetrics_TrendsAndHistory(t *testing.T) {
	m := metrics.NewOperatorMetrics()

	m.UpdateResourceTrendPrediction("ns", "pod", "c1", "cpu", "1h", 123.45)
	assert.Equal(t, 123.45, testutil.ToFloat64(m.ResourceTrendPredictions.WithLabelValues("ns", "pod", "c1", "cpu", "1h")))

	m.UpdateHistoricalDataPoints(42)
	assert.Equal(t, 42.0, testutil.ToFloat64(m.HistoricalDataPoints))
}

func TestOperatorMetrics_UpdateMetrics(t *testing.T) {
	m := metrics.NewOperatorMetrics()

	m.UpdateMetrics(10, 5, 55.5, 66.6, 1.23, 4.56, 60.0)
	assert.Equal(t, 10.0, testutil.ToFloat64(m.ActivePodsTotal))
	assert.Equal(t, 5.0, testutil.ToFloat64(m.OptimizedResourcesTotal))
	assert.InDelta(t, 55.5, testutil.ToFloat64(m.CPUUsagePercent), 0.0001)
	assert.InDelta(t, 66.6, testutil.ToFloat64(m.MemoryUsagePercent), 0.0001)
	assert.InDelta(t, 1.23, testutil.ToFloat64(m.NetworkUsageMbps), 0.0001)
	assert.InDelta(t, 4.56, testutil.ToFloat64(m.DiskIOMBps), 0.0001)
	assert.InDelta(t, 60.0, testutil.ToFloat64(m.AvgUtilizationPercent), 0.0001)

	// Negative values ignored
	m.UpdateMetrics(-1, -1, -1, -1, -1, -1, -1)
	assert.Equal(t, 10.0, testutil.ToFloat64(m.ActivePodsTotal))
	assert.Equal(t, 5.0, testutil.ToFloat64(m.OptimizedResourcesTotal))

	// Partial update
	m.UpdateMetrics(15, -1, 70.0, -1, -1, 7.89, -1)
	assert.Equal(t, 15.0, testutil.ToFloat64(m.ActivePodsTotal))
	assert.Equal(t, 5.0, testutil.ToFloat64(m.OptimizedResourcesTotal))
	assert.InDelta(t, 70.0, testutil.ToFloat64(m.CPUUsagePercent), 0.0001)
	assert.InDelta(t, 66.6, testutil.ToFloat64(m.MemoryUsagePercent), 0.0001) // unchanged
	assert.InDelta(t, 7.89, testutil.ToFloat64(m.DiskIOMBps), 0.0001)
}

func TestOperatorMetrics_NilSafety_UpdateMetrics(t *testing.T) {
	var m *metrics.OperatorMetrics
	// Should not panic
	m.UpdateMetrics(1, 1, 1, 1, 1, 1, 1)
}

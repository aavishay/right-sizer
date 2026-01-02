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
//
// NOTE: Extended with metrics history & system pod endpoints.

package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"right-sizer/api/v1alpha1"
	"right-sizer/events"
	"right-sizer/logger"
	"right-sizer/metrics"
	"right-sizer/predictor"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	hour1  = time.Hour
	hour6  = 6 * time.Hour
	hour12 = 12 * time.Hour
	hour24 = 24 * time.Hour
	day7   = 7 * 24 * time.Hour
	day14  = 14 * 24 * time.Hour
	day30  = 30 * 24 * time.Hour

	serverReadHeaderTimeout = 30 * time.Second
	serverReadTimeout       = 120 * time.Second
	serverWriteTimeout      = 120 * time.Second
	serverIdleTimeout       = 180 * time.Second

	defaultEventLimit = 20
	logTailLines      = 50

	cpuSavingsFactor   = 0.3
	memSavingsFactor   = 0.3
	mbFactor           = 1024 * 1024
	utilizationDivider = 2.0
	percentMultiplier  = 100.0

	cpuUsageSimulationFactor = 10
	memUsageSimulationFactor = 5
)

// Server represents the API server
type Server struct {
	clientset             kubernetes.Interface
	metricsClient         metricsclient.Interface
	ctrlClient            client.Client
	operatorMetrics       *metrics.OperatorMetrics
	predictor             *predictor.Engine // Resource prediction engine
	recommendationManager *events.RecommendationManager
	optimizationOps       atomic.Uint64 // counts optimization actions applied
}

// MetricSample stores a historical aggregate sample for time range filtering
type MetricSample struct {
	Time               time.Time `json:"time"`
	CPUUsagePercent    float64   `json:"cpu"`
	MemoryUsagePercent float64   `json:"memory"`
	ActivePods         float64   `json:"pods"`
	OptimizedResources float64   `json:"optimized"`
	NetworkUsageMbps   float64   `json:"network"`
	DiskIOMBps         float64   `json:"diskIO"`
	AvgUtilization     float64   `json:"utilization"`
}

var (
	metricsHistory      []MetricSample
	metricsHistoryLimit = 2000
	metricsHistoryMu    sync.Mutex
)

// filterMetricsHistory returns a copy of the stored history optionally
// filtered by a simple time range string: 1h,6h,12h,24h,7d,14d,30d.
// Unknown / empty range returns the full (bounded) history.
func filterMetricsHistory(rangeParam string) []MetricSample {
	if rangeParam == "" {
		metricsHistoryMu.Lock()
		defer metricsHistoryMu.Unlock()
		cp := make([]MetricSample, len(metricsHistory))
		copy(cp, metricsHistory)
		return cp
	}

	now := time.Now()
	var window time.Duration
	switch rangeParam {
	case "1h":
		window = time.Hour
	case "6h":
		window = hour6
	case "12h":
		window = hour12
	case "24h":
		window = hour24
	case "7d":
		window = day7
	case "14d":
		window = day14
	case "30d":
		window = day30
	default:
		metricsHistoryMu.Lock()
		defer metricsHistoryMu.Unlock()
		cp := make([]MetricSample, len(metricsHistory))
		copy(cp, metricsHistory)
		return cp
	}

	cutoff := now.Add(-window)

	metricsHistoryMu.Lock()
	defer metricsHistoryMu.Unlock()
	out := make([]MetricSample, 0, len(metricsHistory))
	for _, s := range metricsHistory {
		if s.Time.After(cutoff) {
			out = append(out, s)
		}
	}
	return out
}

// NewServer creates a new API server instance
func NewServer(clientset kubernetes.Interface, metricsClient metricsclient.Interface, ctrlClient client.Client, predictor *predictor.Engine, recommendationManager *events.RecommendationManager, optMetrics ...*metrics.OperatorMetrics) *Server {
	var m *metrics.OperatorMetrics
	if len(optMetrics) > 0 {
		m = optMetrics[0]
	}
	return &Server{
		clientset:             clientset,
		metricsClient:         metricsClient,
		ctrlClient:            ctrlClient,
		operatorMetrics:       m,
		predictor:             predictor,
		recommendationManager: recommendationManager,
	}
}

// Start starts the API server
func (s *Server) Start(port int) error {
	logger.Info("🌐 Starting API server on port %d", port)

	// Register all endpoints
	s.registerEndpoints()

	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
		WriteTimeout:      serverWriteTimeout,
		IdleTimeout:       serverIdleTimeout,
	}

	logger.Info("✅ API server started on port %d", port)
	return server.ListenAndServe()
}

// registerEndpoints registers all HTTP endpoints
func (s *Server) registerEndpoints() {
	// Basic endpoints
	http.HandleFunc("/api/pods/count", s.handlePodCount)
	http.HandleFunc("/api/health", s.handleHealth)

	// Metrics endpoints
	http.HandleFunc("/api/metrics", s.handleMetrics)
	http.HandleFunc("/api/metrics/history", s.handleMetricsHistory) // NEW: historical samples
	http.HandleFunc("/api/metrics/live", s.handleMetricsLive)       // NEW: live JSON cluster summary

	// Prediction endpoints
	http.HandleFunc("/api/predictions", s.handlePredictions)               // NEW: get predictions for resources
	http.HandleFunc("/api/predictions/historical", s.handleHistoricalData) // NEW: get historical data
	http.HandleFunc("/api/predictions/stats", s.handlePredictionStats)     // NEW: prediction engine stats

	// Optimization events
	http.HandleFunc("/api/optimization-events", s.handleOptimizationEvents)
	http.HandleFunc("/api/recommendations", s.handleGetRecommendations)
	http.HandleFunc("/api/recommendations/stats/summary", s.handleGetRecommendationStats)
	http.HandleFunc("/api/recommendations/approve", s.handleApproveRecommendation)
	http.HandleFunc("/api/recommendations/reject", s.handleRejectRecommendation)
	http.HandleFunc("/api/recommendations/execute", s.handleExecuteRecommendation)
	http.HandleFunc("/api/recommendations/", s.handleRecommendationByID)

	// Proxy endpoints for metrics API
	http.HandleFunc("/apis/metrics.k8s.io/v1beta1/nodes", s.handleNodesProxy)
	http.HandleFunc("/apis/metrics.k8s.io/v1beta1/pods", s.handlePodsProxy)

	// Pod data endpoints
	http.HandleFunc("/api/pods", s.handlePods)
	http.HandleFunc("/api/pods/system", s.handleSystemPods) // NEW: system namespaces only
	http.HandleFunc("/api/v1/pods", s.handlePodsV1)
	http.HandleFunc("/apis/v1/pods", s.handlePodsRedirect)

	// System / support (version & capability baseline)
	http.HandleFunc("/api/system/support", s.handleSystemSupport)

	// AIOps incidents (basic placeholder listing)
	http.HandleFunc("/api/aiops/incidents", s.handleIncidents)

	// Health check
	http.HandleFunc("/health", s.handleHealthCheck)

	// Log streaming
	http.HandleFunc("/api/logs", s.handleLogs)

	// Policy management
	http.HandleFunc("/api/policies", s.handlePolicies)
	http.HandleFunc("/api/policies/", s.handlePolicy)
}

// handleSystemSupport returns a minimal support policy payload.
// Future enhancement: enrich with dynamically detected capabilities.
func (s *Server) handleSystemSupport(w http.ResponseWriter, r *http.Request) {
	disc := s.clientset.Discovery()

	info := map[string]any{
		"supportedMinVersion": "1.33",
		"policy":              "Right-Sizer supports Kubernetes 1.33 and above.",
		"status":              "ok",
	}

	// Cluster version
	if sv, err := disc.ServerVersion(); err == nil {
		info["clusterVersion"] = sv.GitVersion
		info["clusterMajor"] = sv.Major
		info["clusterMinor"] = sv.Minor
	} else {
		info["clusterVersionError"] = err.Error()
	}

	// Capability discovery (best-effort)
	caps := map[string]bool{}
	if groups, resources, _ := disc.ServerGroupsAndResources(); len(groups) > 0 || len(resources) > 0 {
		groupSet := map[string]struct{}{}
		for _, g := range groups {
			groupSet[g.Name] = struct{}{}
		}
		if _, ok := groupSet["metrics.k8s.io"]; ok {
			caps["metricsServer"] = true
		}
		if _, ok := groupSet["resource.k8s.io"]; ok {
			caps["dynamicResourceAllocation"] = true
		}
		for _, rl := range resources {
			if rl == nil {
				continue
			}
			for _, rsc := range rl.APIResources {
				switch strings.ToLower(rsc.Name) {
				case "pods/ephemeralcontainers":
					caps["ephemeralContainers"] = true
				case "pods/resize":
					caps["podResize"] = true
				}
			}
		}
	}
	info["capabilities"] = caps
	info["timestamp"] = time.Now().UTC()

	// Supported flag (parse minor)
	if minorStr, ok := info["clusterMinor"].(string); ok {
		minor := 0
		for i := 0; i < len(minorStr); i++ {
			if minorStr[i] < '0' || minorStr[i] > '9' {
				break
			}
			minor = minor*10 + int(minorStr[i]-'0')
		}
		info["supported"] = (minor >= 33)
	}

	s.writeJSONResponse(w, info)
}

// handleIncidents lists (placeholder) incidents with basic query echoing until
// the incident store is fully integrated with the API server.
func (s *Server) handleIncidents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := q.Get("limit")
	if limit == "" {
		limit = "50"
	}
	resp := map[string]any{
		"items":     []any{},
		"count":     0,
		"filters":   map[string]any{"type": q.Get("type"), "severity": q.Get("severity"), "limit": limit},
		"status":    "not_implemented",
		"message":   "Incident store integration pending; endpoint currently returns no data.",
		"version":   "v1alpha0",
		"timestamp": time.Now().UTC(),
	}
	s.writeJSONResponse(w, resp)
}

// handlePodCount handles /api/pods/count endpoint
func (s *Server) handlePodCount(w http.ResponseWriter, r *http.Request) {
	podList, err := s.clientset.CoreV1().Pods("").List(r.Context(), metav1.ListOptions{})
	if err != nil {
		logger.Error("Failed to get pod count: %v", err)
		http.Error(w, "Failed to get pod count", http.StatusInternalServerError)
		return
	}

	podCount := len(podList.Items)
	response := map[string]int{"count": podCount}

	s.writeJSONResponse(w, response)
}

// handleHealth handles /api/health endpoint
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	response := map[string]string{"status": "ok"}
	s.writeJSONResponse(w, response)
}

// handleMetrics handles /api/metrics endpoint
//
// Added: /api/metrics/live JSON endpoint (handleMetricsLive) for debugging/raw data.
func (s *Server) handleMetricsLive(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Collect fresh pod & node info
	podList, err := s.clientset.CoreV1().Pods("").List(r.Context(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, "failed to collect pods", http.StatusInternalServerError)
		return
	}
	nodeList, err := s.clientset.CoreV1().Nodes().List(r.Context(), metav1.ListOptions{})
	if err != nil {
		http.Error(w, "failed to collect nodes", http.StatusInternalServerError)
		return
	}

	cluster := s.calculateClusterMetrics(podList.Items, nodeList.Items)

	// Fetch latest aggregated sample (if any) from in‑memory history
	var latest *MetricSample
	metricsHistoryMu.Lock()
	if len(metricsHistory) > 0 {
		sample := metricsHistory[len(metricsHistory)-1]
		// copy to avoid race on underlying slice references
		tmp := sample
		latest = &tmp
	}
	metricsHistoryMu.Unlock()

	resp := map[string]interface{}{
		"cluster":       cluster,
		"latestSample":  latest,
		"historyLength": len(metricsHistory),
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
	}

	s.writeJSONResponse(w, resp)
}

// Additional endpoints implemented:
//
//	GET /api/metrics/history
//	    Returns JSON: { "samples": [ {time,...}, ... ] }
//	    Query params:
//	      ?range=1h|6h|12h|24h|7d|14d|30d  (optional)
//	GET /api/pods/system
//	    Returns JSON array of system namespace pods (kube-system, kube-public, kube-node-lease)
//
// NOTE: Ensure registerEndpoints includes the new handlers.
// IMPORTANT: The metrics API expects Prometheus exposition text format, not JSON.
// We emit a minimal set of gauge metrics consumed by the React UI and also
// maintain an in‑memory history slice that the server could expose later if needed.
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	podList, err := s.clientset.CoreV1().Pods("").List(r.Context(), metav1.ListOptions{})
	if err != nil {
		logger.Error("Failed to get pods for metrics: %v", err)
		http.Error(w, "failed to collect pods", http.StatusInternalServerError)
		return
	}

	nodeList, err := s.clientset.CoreV1().Nodes().List(r.Context(), metav1.ListOptions{})
	if err != nil {
		logger.Error("Failed to get nodes for metrics: %v", err)
		http.Error(w, "failed to collect nodes", http.StatusInternalServerError)
		return
	}

	cluster := s.calculateClusterMetrics(podList.Items, nodeList.Items)

	// Extract numeric percentages from strings like "23.4%"
	parsePercent := func(v interface{}) float64 {
		if v == nil {
			return 0
		}
		str, ok := v.(string)
		if !ok {
			return 0
		}
		str = strings.TrimSpace(str)
		str = strings.TrimSuffix(str, "%")
		f, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return 0
		}
		return f
	}

	resources, ok := cluster["resources"].(map[string]interface{})
	if !ok {
		return
	}
	cpuMap := map[string]interface{}{}
	memMap := map[string]interface{}{}
	if resources != nil {
		if v, ok := resources["cpu"].(map[string]interface{}); ok {
			cpuMap = v
		}
		if v, ok := resources["memory"].(map[string]interface{}); ok {
			memMap = v
		}
	}

	cpuUtil := parsePercent(cpuMap["utilization"])
	memUtil := parsePercent(memMap["utilization"])

	activePods := 0
	if v, ok := cluster["managedPods"].(int); ok {
		activePods = v
	} else if vf, ok := cluster["managedPods"].(float64); ok {
		activePods = int(vf)
	}

	// Optimized resources: use internal counter
	optimized := int(s.optimizationOps.Load())

	// Simulated / placeholder values for network & disk for now
	network := 0.0
	diskIO := 0.0

	avgUtil := 0.0
	if cpuUtil > 0 || memUtil > 0 {
		avgUtil = (cpuUtil + memUtil) / 2.0
	}

	sample := MetricSample{
		Time:               time.Now(),
		CPUUsagePercent:    cpuUtil,
		MemoryUsagePercent: memUtil,
		ActivePods:         float64(activePods),
		OptimizedResources: float64(optimized),
		NetworkUsageMbps:   network,
		DiskIOMBps:         diskIO,
		AvgUtilization:     avgUtil,
	}

	// If operator metrics pointer provided, update metrics gauges
	if s.operatorMetrics != nil {
		s.operatorMetrics.UpdateMetrics(
			activePods,
			optimized,
			cpuUtil,
			memUtil,
			network,
			diskIO,
			avgUtil,
		)
	}

	// Persist history (trim if exceeds limit)
	metricsHistoryMu.Lock()
	metricsHistory = append(metricsHistory, sample)
	if len(metricsHistory) > metricsHistoryLimit {
		metricsHistory = metricsHistory[len(metricsHistory)-metricsHistoryLimit:]
	}
	metricsHistoryMu.Unlock()

	// Prometheus exposition format
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(w, "# HELP rightsizer_cpu_usage_percent Average CPU usage percent across managed pods\n")
	fmt.Fprintf(w, "# TYPE rightsizer_cpu_usage_percent gauge\n")
	fmt.Fprintf(w, "rightsizer_cpu_usage_percent %.3f\n", sample.CPUUsagePercent)

	fmt.Fprintf(w, "# HELP rightsizer_memory_usage_percent Average memory usage percent across managed pods\n")
	fmt.Fprintf(w, "# TYPE rightsizer_memory_usage_percent gauge\n")
	fmt.Fprintf(w, "rightsizer_memory_usage_percent %.3f\n", sample.MemoryUsagePercent)

	fmt.Fprintf(w, "# HELP rightsizer_active_pods_total Number of active (non-system) managed pods\n")
	fmt.Fprintf(w, "# TYPE rightsizer_active_pods_total gauge\n")
	fmt.Fprintf(w, "rightsizer_active_pods_total %.0f\n", sample.ActivePods)

	fmt.Fprintf(w, "# HELP rightsizer_optimized_resources_total Total number of optimization actions applied (placeholder)\n")
	fmt.Fprintf(w, "# TYPE rightsizer_optimized_resources_total gauge\n")
	fmt.Fprintf(w, "rightsizer_optimized_resources_total %.0f\n", sample.OptimizedResources)

	fmt.Fprintf(w, "# HELP rightsizer_network_usage_mbps Estimated aggregate network usage (simulated)\n")
	fmt.Fprintf(w, "# TYPE rightsizer_network_usage_mbps gauge\n")
	fmt.Fprintf(w, "rightsizer_network_usage_mbps %.3f\n", sample.NetworkUsageMbps)

	fmt.Fprintf(w, "# HELP rightsizer_disk_io_mbps Estimated aggregate disk IO MB/s (simulated)\n")
	fmt.Fprintf(w, "# TYPE rightsizer_disk_io_mbps gauge\n")
	fmt.Fprintf(w, "rightsizer_disk_io_mbps %.3f\n", sample.DiskIOMBps)

	fmt.Fprintf(w, "# HELP rightsizer_avg_utilization_percent Average of CPU and memory utilization percentages\n")
	fmt.Fprintf(w, "# TYPE rightsizer_avg_utilization_percent gauge\n")
	fmt.Fprintf(w, "rightsizer_avg_utilization_percent %.3f\n", sample.AvgUtilization)
}

// calculateClusterMetrics calculates comprehensive cluster metrics
func (s *Server) calculateClusterMetrics(pods []v1.Pod, nodes []v1.Node) map[string]interface{} {
	// Calculate comprehensive metrics
	var totalCPURequests, totalMemoryRequests int64
	var totalCPULimits, totalMemoryLimits int64
	var podsWithoutRequests, podsWithoutLimits int
	var rightSizerPods, managedPods int
	namespaceBreakdown := make(map[string]int)

	for _, pod := range pods {
		namespaceBreakdown[pod.Namespace]++

		if pod.Namespace == "right-sizer" {
			rightSizerPods++
		}

		// Count managed pods (not in system namespaces)
		if pod.Namespace != "kube-system" && pod.Namespace != "kube-public" && pod.Namespace != "kube-node-lease" {
			managedPods++
		}

		// Calculate resource usage
		for _, container := range pod.Spec.Containers {
			if container.Resources.Requests != nil {
				if cpu := container.Resources.Requests.Cpu(); cpu != nil {
					totalCPURequests += cpu.MilliValue()
				} else {
					podsWithoutRequests++
				}
				if memory := container.Resources.Requests.Memory(); memory != nil {
					totalMemoryRequests += memory.Value()
				}
			} else {
				podsWithoutRequests++
			}

			if container.Resources.Limits != nil {
				if cpu := container.Resources.Limits.Cpu(); cpu != nil {
					totalCPULimits += cpu.MilliValue()
				} else {
					podsWithoutLimits++
				}
				if memory := container.Resources.Limits.Memory(); memory != nil {
					totalMemoryLimits += memory.Value()
				}
			} else {
				podsWithoutLimits++
			}
		}
	}

	// Get node capacity
	var totalNodeCPU, totalNodeMemory int64
	for _, node := range nodes {
		if cpu := node.Status.Capacity.Cpu(); cpu != nil {
			totalNodeCPU += cpu.MilliValue()
		}
		if memory := node.Status.Capacity.Memory(); memory != nil {
			totalNodeMemory += memory.Value()
		}
	}

	metrics := map[string]interface{}{
		"totalPods":          len(pods),
		"totalNodes":         len(nodes),
		"rightSizerPods":     rightSizerPods,
		"managedPods":        managedPods,
		"namespaceBreakdown": namespaceBreakdown,
		"resources": map[string]interface{}{
			"cpu": map[string]interface{}{
				"totalRequests": fmt.Sprintf("%.1fm", float64(totalCPURequests)),
				"totalLimits":   fmt.Sprintf("%.1fm", float64(totalCPULimits)),
				"nodeCapacity":  fmt.Sprintf("%.1fm", float64(totalNodeCPU)),
				"utilization":   fmt.Sprintf("%.1f%%", float64(totalCPURequests)/float64(totalNodeCPU)*percentMultiplier),
			},
			"memory": map[string]interface{}{
				"totalRequests": fmt.Sprintf("%.0fMi", float64(totalMemoryRequests)/(mbFactor)),
				"totalLimits":   fmt.Sprintf("%.0fMi", float64(totalMemoryLimits)/(mbFactor)),
				"nodeCapacity":  fmt.Sprintf("%.0fMi", float64(totalNodeMemory)/(mbFactor)),
				"utilization":   fmt.Sprintf("%.1f%%", float64(totalMemoryRequests)/float64(totalNodeMemory)*percentMultiplier),
			},
		},
		"optimization": map[string]interface{}{
			"podsWithoutRequests": podsWithoutRequests,
			"podsWithoutLimits":   podsWithoutLimits,
			"potentialSavings": map[string]interface{}{
				"cpu":    fmt.Sprintf("%.0fm", float64(totalCPURequests)*cpuSavingsFactor), // Assume 30% savings potential
				"memory": fmt.Sprintf("%.0fMi", float64(totalMemoryRequests)*memSavingsFactor/(mbFactor)),
			},
		},
		"timestamp": time.Now().Unix(),
	}

	return metrics
}

// handleOptimizationEvents handles /api/optimization-events endpoint
func (s *Server) handleOptimizationEvents(w http.ResponseWriter, r *http.Request) {
	events := s.getOptimizationEvents(r.Context())
	response := map[string]interface{}{
		"events": events,
		"total":  len(events),
	}
	s.writeJSONResponse(w, response)
}

// getOptimizationEvents retrieves optimization events from various sources
func (s *Server) getOptimizationEvents(ctx context.Context) []map[string]interface{} {
	events := []map[string]interface{}{}

	// Try to get real events from optimization-events-server
	if resp, err := http.Get("http://optimization-events-server.right-sizer.svc.cluster.local/events.json"); err == nil {
		defer resp.Body.Close()
		if body, err := io.ReadAll(resp.Body); err == nil {
			var serverEvents []map[string]interface{}
			if err := json.Unmarshal(body, &serverEvents); err == nil {
				return serverEvents
			}
		}
	}

	// Try to read from audit log file
	events = append(events, s.getEventsFromAuditLog()...)

	// Fallback: Check Kubernetes events
	events = append(events, s.getEventsFromK8s(ctx)...)

	// Sort and limit events
	s.sortAndLimitEvents(&events, defaultEventLimit)

	return events
}

// getEventsFromAuditLog reads events from audit log file
func (s *Server) getEventsFromAuditLog() []map[string]interface{} {
	events := []map[string]interface{}{}

	auditLogPath := "/tmp/right-sizer-audit.log"
	file, err := os.Open(auditLogPath)
	if err != nil {
		return events
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Process the last 50 lines
	startIdx := len(lines) - logTailLines
	if startIdx < 0 {
		startIdx = 0
	}

	for i := startIdx; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		var auditEvent map[string]interface{}
		if err := json.Unmarshal([]byte(line), &auditEvent); err == nil {
			if eventType, ok := auditEvent["eventType"].(string); ok && eventType == "ResourceChange" {
				event := s.convertAuditEvent(auditEvent)
				events = append(events, event)
			}
		}
	}

	return events
}

// convertAuditEvent converts audit event to API format
func (s *Server) convertAuditEvent(auditEvent map[string]interface{}) map[string]interface{} {
	event := map[string]interface{}{
		"timestamp":     auditEvent["timestamp"],
		"eventId":       auditEvent["eventId"],
		"podName":       auditEvent["podName"],
		"namespace":     auditEvent["namespace"],
		"containerName": auditEvent["containerName"],
		"operation":     auditEvent["operation"],
		"reason":        auditEvent["reason"],
		"status":        auditEvent["status"],
		"action":        "resource_change",
	}

	// Add resource information if available
	s.addResourceInfo(event, auditEvent)
	return event
}

// addResourceInfo adds resource information to event
func (s *Server) addResourceInfo(event, auditEvent map[string]interface{}) {
	if oldRes, ok := auditEvent["oldResources"].(map[string]interface{}); ok {
		if requests, ok := oldRes["requests"].(map[string]interface{}); ok {
			if cpu, ok := requests["cpu"].(string); ok {
				event["previousCPU"] = cpu
			}
			if memory, ok := requests["memory"].(string); ok {
				event["previousMemory"] = memory
			}
		}
	}

	if newRes, ok := auditEvent["newResources"].(map[string]interface{}); ok {
		if requests, ok := newRes["requests"].(map[string]interface{}); ok {
			if cpu, ok := requests["cpu"].(string); ok {
				event["currentCPU"] = cpu
				event["recommendedCPU"] = cpu
			}
			if memory, ok := requests["memory"].(string); ok {
				event["currentMemory"] = memory
				event["recommendedMemory"] = memory
			}
		}
	}

	if event["previousCPU"] != nil && event["currentCPU"] != nil {
		event["optimizationType"] = "resource_optimization"
	}
}

// getEventsFromK8s retrieves events from Kubernetes API
func (s *Server) getEventsFromK8s(ctx context.Context) []map[string]interface{} {
	events := []map[string]interface{}{}

	eventList, err := s.clientset.CoreV1().Events("").List(ctx, metav1.ListOptions{
		FieldSelector: "reason=ResourceOptimized",
		Limit:         20,
	})
	if err != nil {
		return events
	}

	for _, kubeEvent := range eventList.Items {
		if strings.Contains(kubeEvent.Source.Component, "right-sizer") {
			event := map[string]interface{}{
				"timestamp":     kubeEvent.CreationTimestamp.Unix(),
				"eventId":       string(kubeEvent.UID),
				"podName":       kubeEvent.InvolvedObject.Name,
				"namespace":     kubeEvent.Namespace,
				"containerName": "unknown",
				"operation":     "resource_change",
				"reason":        kubeEvent.Reason,
				"status":        "completed",
				"action":        "optimization_applied",
				"message":       kubeEvent.Message,
			}
			events = append(events, event)
		}
	}

	return events
}

// handleLogs streams logs for a specific pod
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	podName := r.URL.Query().Get("pod")
	containerName := r.URL.Query().Get("container")
	tailLinesStr := r.URL.Query().Get("tailLines")
	followStr := r.URL.Query().Get("follow")

	if namespace == "" || podName == "" {
		http.Error(w, "namespace and pod parameters are required", http.StatusBadRequest)
		return
	}

	tailLines := int64(defaultEventLimit)
	if tailLinesStr != "" {
		if val, err := strconv.ParseInt(tailLinesStr, 10, 64); err == nil {
			tailLines = val
		}
	}

	follow := true
	if followStr == "false" {
		follow = false
	}

	logOptions := &v1.PodLogOptions{
		Container:  containerName,
		Follow:     follow,
		TailLines:  &tailLines,
		Timestamps: true,
	}

	req := s.clientset.CoreV1().Pods(namespace).GetLogs(podName, logOptions)
	stream, err := req.Stream(r.Context())
	if err != nil {
		logger.Error("Failed to open log stream: %v", err)
		http.Error(w, fmt.Sprintf("Failed to open log stream: %v", err), http.StatusInternalServerError)
		return
	}
	defer stream.Close()

	// Set headers for streaming
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		logger.Error("ResponseWriter does not support Flushing")
		return
	}

	// Stream logs
	reader := bufio.NewReader(stream)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				logger.Error("Error reading log stream: %v", err)
			}
			break
		}

		_, writeErr := w.Write(line)
		if writeErr != nil {
			break
		}
		flusher.Flush()
	}
}

// sortAndLimitEvents sorts events by timestamp and limits the count
func (s *Server) sortAndLimitEvents(events *[]map[string]interface{}, limit int) {
	if len(*events) == 0 {
		return
	}

	// Sort by timestamp descending
	for i := range len(*events) - 1 {
		for j := i + 1; j < len(*events); j++ {
			var timestamp1, timestamp2 float64

			switch ts1 := (*events)[i]["timestamp"].(type) {
			case string:
				if t, err := time.Parse(time.RFC3339, ts1); err == nil {
					timestamp1 = float64(t.Unix())
				}
			case float64:
				timestamp1 = ts1
			}

			switch ts2 := (*events)[j]["timestamp"].(type) {
			case string:
				if t, err := time.Parse(time.RFC3339, ts2); err == nil {
					timestamp2 = float64(t.Unix())
				}
			case float64:
				timestamp2 = ts2
			}

			if timestamp2 > timestamp1 {
				(*events)[i], (*events)[j] = (*events)[j], (*events)[i]
			}
		}
	}

	// Limit events
	if len(*events) > limit {
		*events = (*events)[:limit]
	}
}

// handleNodesProxy handles /apis/metrics.k8s.io/v1beta1/nodes endpoint
func (s *Server) handleNodesProxy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	nodeList, err := s.clientset.CoreV1().Nodes().List(r.Context(), metav1.ListOptions{})
	if err != nil {
		logger.Error("Failed to get nodes for proxy: %v", err)
		http.Error(w, "Failed to get nodes", http.StatusInternalServerError)
		return
	}

	response := s.convertNodesToMetricsAPI(nodeList.Items)
	s.writeJSONResponse(w, response)
}

// convertNodesToMetricsAPI converts nodes to metrics API format
func (s *Server) convertNodesToMetricsAPI(nodes []v1.Node) map[string]interface{} {
	response := map[string]interface{}{
		"kind":       "NodeMetricsList",
		"apiVersion": "metrics.k8s.io/v1beta1",
		"metadata":   map[string]interface{}{},
		"items":      []map[string]interface{}{},
	}

	for _, node := range nodes {
		nodeMetric := map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": node.Name,
			},
			"timestamp": time.Now().Format(time.RFC3339),
			"window":    "30s",
			"usage": map[string]interface{}{
				"cpu":    node.Status.Capacity.Cpu().String(),
				"memory": node.Status.Capacity.Memory().String(),
			},
		}
		response["items"] = append(response["items"].([]map[string]interface{}), nodeMetric)
	}

	return response
}

// handlePodsProxy handles /apis/metrics.k8s.io/v1beta1/pods endpoint
func (s *Server) handlePodsProxy(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	podList, err := s.clientset.CoreV1().Pods("").List(r.Context(), metav1.ListOptions{})
	if err != nil {
		logger.Error("Failed to get pods for proxy: %v", err)
		http.Error(w, "Failed to get pods", http.StatusInternalServerError)
		return
	}

	response := s.convertPodsToMetricsAPI(podList.Items)
	s.writeJSONResponse(w, response)
}

// convertPodsToMetricsAPI converts pods to metrics API format
func (s *Server) convertPodsToMetricsAPI(pods []v1.Pod) map[string]interface{} {
	response := map[string]interface{}{
		"kind":       "PodMetricsList",
		"apiVersion": "metrics.k8s.io/v1beta1",
		"metadata":   map[string]interface{}{},
		"items":      []map[string]interface{}{},
	}

	for _, pod := range pods {
		if pod.Status.Phase != "Running" {
			continue
		}

		containers := []map[string]interface{}{}
		for _, container := range pod.Spec.Containers {
			containerMetric := map[string]interface{}{
				"name": container.Name,
				"usage": map[string]interface{}{
					"cpu":    "0m", // Would need actual metrics server for real usage
					"memory": "0Mi",
				},
			}
			if container.Resources.Requests != nil {
				if cpu := container.Resources.Requests.Cpu(); cpu != nil {
					containerMetric["usage"].(map[string]interface{})["cpu"] = fmt.Sprintf("%dm", cpu.MilliValue()/cpuUsageSimulationFactor) // Simulate 10% usage
				}
				if memory := container.Resources.Requests.Memory(); memory != nil {
					containerMetric["usage"].(map[string]interface{})["memory"] = fmt.Sprintf("%dMi", memory.Value()/(mbFactor)/memUsageSimulationFactor) // Simulate 20% usage
				}
			}
			containers = append(containers, containerMetric)
		}

		podMetric := map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      pod.Name,
				"namespace": pod.Namespace,
			},
			"timestamp":  time.Now().Format(time.RFC3339),
			"window":     "30s",
			"containers": containers,
		}
		response["items"] = append(response["items"].([]map[string]interface{}), podMetric)
	}

	return response
}

// handlePods handles /api/pods endpoint
func (s *Server) handlePods(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	podList, err := s.clientset.CoreV1().Pods("").List(r.Context(), metav1.ListOptions{})
	if err != nil {
		logger.Error("Failed to get pods: %v", err)
		http.Error(w, "Failed to get pods", http.StatusInternalServerError)
		return
	}

	pods := s.buildEnhancedPodData(r.Context(), podList.Items)
	s.writeJSONResponse(w, pods)
}

// buildEnhancedPodData builds enhanced pod data
func (s *Server) buildEnhancedPodData(ctx context.Context, pods []v1.Pod) []map[string]interface{} {
	// Get metrics for pods if available
	metricsAvailable := false
	var podMetricsList *metricsv1beta1.PodMetricsList
	if s.metricsClient != nil {
		var err error
		podMetricsList, err = s.metricsClient.MetricsV1beta1().PodMetricses("").List(ctx, metav1.ListOptions{})
		if err == nil {
			metricsAvailable = true
		}
	}

	// Create a map of pod metrics for quick lookup
	podMetricsMap := make(map[string]*metricsv1beta1.PodMetrics)
	if metricsAvailable && podMetricsList != nil {
		for i := range podMetricsList.Items {
			pm := &podMetricsList.Items[i]
			key := fmt.Sprintf("%s/%s", pm.Namespace, pm.Name)
			podMetricsMap[key] = pm
		}
	}

	// Build enhanced pod data
	podData := []map[string]interface{}{}
	for _, pod := range pods {
		// Skip pods that are being deleted
		if pod.DeletionTimestamp != nil {
			continue
		}

		// NOTE: Previously skipped system namespaces (kube-system, kube-public, kube-node-lease)
		// to hide infrastructure pods from the API response. This skip block was removed to include
		// all pods (including system namespaces) in the enhanced pod data response.

		// Get metrics for this pod
		podKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
		metrics := podMetricsMap[podKey]

		// Calculate CPU and Memory usage
		cpuUsage := "Not available"
		memoryUsage := "Not available"

		if metrics != nil && len(metrics.Containers) > 0 {
			var totalCPU int64
			var totalMemory int64

			for _, container := range metrics.Containers {
				if cpu, ok := container.Usage["cpu"]; ok {
					totalCPU += cpu.MilliValue()
				}
				if mem, ok := container.Usage["memory"]; ok {
					memBytes := mem.Value()
					totalMemory += memBytes
				}
			}

			if totalCPU > 0 {
				cpuUsage = fmt.Sprintf("%dm", totalCPU)
			}
			if totalMemory > 0 {
				memMi := totalMemory / (1024 * 1024)
				memoryUsage = fmt.Sprintf("%dMi", memMi)
			}
		}

		// Fallback to resource requests if metrics not available
		if cpuUsage == "Not available" && len(pod.Spec.Containers) > 0 {
			if pod.Spec.Containers[0].Resources.Requests != nil {
				if cpu := pod.Spec.Containers[0].Resources.Requests.Cpu(); cpu != nil {
					cpuUsage = cpu.String()
				}
			}
		}
		if memoryUsage == "Not available" && len(pod.Spec.Containers) > 0 {
			if pod.Spec.Containers[0].Resources.Requests != nil {
				if mem := pod.Spec.Containers[0].Resources.Requests.Memory(); mem != nil {
					memoryUsage = mem.String()
				}
			}
		}

		// Calculate restart count
		restartCount := 0
		if pod.Status.ContainerStatuses != nil {
			for _, cs := range pod.Status.ContainerStatuses {
				restartCount += int(cs.RestartCount)
			}
		}

		// Get optimization info
		optimized := false
		optimizationType := ""
		savings := 0.0

		if pod.Annotations != nil {
			if _, ok := pod.Annotations["right-sizer.io/optimized"]; ok {
				optimized = true
				optimizationType = pod.Annotations["right-sizer.io/optimization-type"]
				if savingsStr := pod.Annotations["right-sizer.io/savings"]; savingsStr != "" {
					fmt.Sscanf(savingsStr, "%f", &savings)
				}
			}
		}

		podInfo := map[string]interface{}{
			"name":             pod.Name,
			"namespace":        pod.Namespace,
			"status":           string(pod.Status.Phase),
			"cpuUsage":         cpuUsage,
			"memoryUsage":      memoryUsage,
			"nodeName":         pod.Spec.NodeName,
			"startTime":        pod.Status.StartTime,
			"restartCount":     restartCount,
			"optimized":        optimized,
			"optimizationType": optimizationType,
			"savings":          savings,
		}

		podData = append(podData, podInfo)
	}

	return podData
}

// handlePodsV1 handles /api/v1/pods endpoint
func (s *Server) handlePodsV1(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	podList, err := s.clientset.CoreV1().Pods("").List(r.Context(), metav1.ListOptions{})
	if err != nil {
		logger.Error("Failed to get pods for proxy: %v", err)
		http.Error(w, "Failed to get pods", http.StatusInternalServerError)
		return
	}

	response := s.convertPodsToV1API(podList.Items)
	s.writeJSONResponse(w, response)
}

// convertPodsToV1API converts pods to v1 API format
func (s *Server) convertPodsToV1API(pods []v1.Pod) map[string]interface{} {
	items := []map[string]interface{}{}
	for _, pod := range pods {
		item := map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      pod.Name,
				"namespace": pod.Namespace,
			},
			"status": map[string]interface{}{
				"phase":             pod.Status.Phase,
				"startTime":         pod.Status.StartTime,
				"containerStatuses": pod.Status.ContainerStatuses,
			},
			"spec": map[string]interface{}{
				"nodeName": pod.Spec.NodeName,
				"containers": func() []map[string]interface{} {
					containers := []map[string]interface{}{}
					for _, container := range pod.Spec.Containers {
						containers = append(containers, map[string]interface{}{
							"name": container.Name,
							"resources": map[string]interface{}{
								"requests": func() map[string]interface{} {
									requests := map[string]interface{}{}
									if container.Resources.Requests != nil {
										if cpu := container.Resources.Requests.Cpu(); cpu != nil {
											requests["cpu"] = cpu.String()
										}
										if memory := container.Resources.Requests.Memory(); memory != nil {
											requests["memory"] = memory.String()
										}
									}
									return requests
								}(),
								"limits": func() map[string]interface{} {
									limits := map[string]interface{}{}
									if container.Resources.Limits != nil {
										if cpu := container.Resources.Limits.Cpu(); cpu != nil {
											limits["cpu"] = cpu.String()
										}
										if memory := container.Resources.Limits.Memory(); memory != nil {
											limits["memory"] = memory.String()
										}
									}
									return limits
								}(),
							},
						})
					}
					return containers
				}(),
			},
		}
		items = append(items, item)
	}

	response := map[string]interface{}{
		"kind":       "PodList",
		"apiVersion": "v1",
		"metadata":   map[string]interface{}{},
		"items":      items,
	}

	return response
}

// handlePodsRedirect handles /apis/v1/pods redirect
func (s *Server) handlePodsRedirect(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/api/v1/pods", http.StatusPermanentRedirect)
}

// handleHealthCheck handles /health endpoint
func (s *Server) handleHealthCheck(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("api server healthy"))
}

// handleMetricsHistory returns historical aggregate metric samples collected
// by handleMetrics. Optional query param "range" may be one of:
// 1h,6h,12h,24h,7d,14d,30d
// Response JSON format:
//
//	{
//	  "range": "24h",
//	  "count": 123,
//	  "samples": [
//	    {"time":"2025-09-10T07:05:00Z","cpu":12.3,"memory":44.1,"pods":8,"optimized":2,"network":0,"diskIO":0,"utilization":28.2},
//	    ...
//	  ]
//	}
func (s *Server) handleMetricsHistory(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	rangeParam := r.URL.Query().Get("range")
	samples := filterMetricsHistory(rangeParam)

	type sampleJSON struct {
		Time        string  `json:"time"`
		CPU         float64 `json:"cpu"`
		Memory      float64 `json:"memory"`
		Pods        float64 `json:"pods"`
		Optimized   float64 `json:"optimized"`
		Network     float64 `json:"network"`
		DiskIO      float64 `json:"diskIO"`
		Utilization float64 `json:"utilization"`
	}

	out := make([]sampleJSON, 0, len(samples))
	for _, s := range samples {
		out = append(out, sampleJSON{
			Time:        s.Time.UTC().Format(time.RFC3339),
			CPU:         s.CPUUsagePercent,
			Memory:      s.MemoryUsagePercent,
			Pods:        s.ActivePods,
			Optimized:   s.OptimizedResources,
			Network:     s.NetworkUsageMbps,
			DiskIO:      s.DiskIOMBps,
			Utilization: s.AvgUtilization,
		})
	}

	resp := map[string]interface{}{
		"range":   rangeParam,
		"count":   len(out),
		"samples": out,
	}

	s.writeJSONResponse(w, resp)
}

// handleSystemPods returns ONLY system namespace pods so the UI can
// optionally display or filter them separately without removing them
// from the main /api/pods endpoint.
func (s *Server) handleSystemPods(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	podList, err := s.clientset.CoreV1().Pods("").List(r.Context(), metav1.ListOptions{})
	if err != nil {
		logger.Error("Failed to get system pods: %v", err)
		http.Error(w, "Failed to get system pods", http.StatusInternalServerError)
		return
	}

	systemNamespaces := map[string]bool{
		"kube-system":     true,
		"kube-public":     true,
		"kube-node-lease": true,
		"right-sizer":     false, // treat operator namespace as non-system for visibility
	}

	results := []map[string]interface{}{}
	for _, pod := range podList.Items {
		if !systemNamespaces[pod.Namespace] {
			continue
		}
		if pod.DeletionTimestamp != nil {
			continue
		}
		results = append(results, map[string]interface{}{
			"name":      pod.Name,
			"namespace": pod.Namespace,
			"status":    string(pod.Status.Phase),
			"nodeName":  pod.Spec.NodeName,
			"startTime": pod.Status.StartTime,
			"restarts": func() int {
				total := 0
				for _, cs := range pod.Status.ContainerStatuses {
					total += int(cs.RestartCount)
				}
				return total
			}(),
		})
	}

	s.writeJSONResponse(w, results)
}

// handlePredictions handles /api/predictions endpoint
func (s *Server) handlePredictions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.predictor == nil {
		http.Error(w, "Prediction engine not available", http.StatusServiceUnavailable)
		return
	}

	// Parse query parameters
	namespace := r.URL.Query().Get("namespace")
	podName := r.URL.Query().Get("pod")
	container := r.URL.Query().Get("container")
	resourceType := r.URL.Query().Get("type") // "cpu" or "memory"

	if namespace == "" || podName == "" || container == "" || resourceType == "" {
		http.Error(w, "Missing required parameters: namespace, pod, container, type", http.StatusBadRequest)
		return
	}

	// Create prediction request
	request := predictor.PredictionRequest{
		Namespace:    namespace,
		PodName:      podName,
		Container:    container,
		ResourceType: resourceType,
	}

	// Get predictions
	response, err := s.predictor.Predict(r.Context(), request)
	if err != nil {
		logger.Error("Failed to generate predictions: %v", err)
		http.Error(w, fmt.Sprintf("Failed to generate predictions: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, response)
}

// handleHistoricalData handles /api/predictions/historical endpoint
func (s *Server) handleHistoricalData(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.predictor == nil {
		http.Error(w, "Prediction engine not available", http.StatusServiceUnavailable)
		return
	}

	// Parse query parameters
	namespace := r.URL.Query().Get("namespace")
	podName := r.URL.Query().Get("pod")
	container := r.URL.Query().Get("container")
	resourceType := r.URL.Query().Get("type") // "cpu" or "memory"
	sinceParam := r.URL.Query().Get("since")  // duration like "24h", "7d"

	if namespace == "" || podName == "" || container == "" || resourceType == "" {
		http.Error(w, "Missing required parameters: namespace, pod, container, type", http.StatusBadRequest)
		return
	}

	// Parse since parameter
	var since time.Time
	if sinceParam != "" {
		duration, err := time.ParseDuration(sinceParam)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid since parameter: %v", err), http.StatusBadRequest)
			return
		}
		since = time.Now().Add(-duration)
	} else {
		since = time.Now().Add(-24 * time.Hour) // Default to 24 hours
	}

	// Get historical data
	historicalData, err := s.predictor.GetHistoricalData(namespace, podName, container, resourceType, since)
	if err != nil {
		logger.Error("Failed to get historical data: %v", err)
		http.Error(w, fmt.Sprintf("Failed to get historical data: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeJSONResponse(w, historicalData)
}

// handlePredictionStats handles /api/predictions/stats endpoint
func (s *Server) handlePredictionStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if s.predictor == nil {
		http.Error(w, "Prediction engine not available", http.StatusServiceUnavailable)
		return
	}

	// Get prediction engine statistics
	stats := s.predictor.GetStats()
	s.writeJSONResponse(w, stats)
}

// handleGetRecommendations handles /api/recommendations endpoint
func (s *Server) handleGetRecommendations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.recommendationManager == nil {
		http.Error(w, "Recommendation manager not available", http.StatusServiceUnavailable)
		return
	}

	// Parse query parameters for filtering
	status := r.URL.Query().Get("status")   // pending, approved, rejected, etc.
	urgency := r.URL.Query().Get("urgency") // low, medium, high, critical

	var recommendations []*events.Recommendation

	if status != "" {
		var recStatus events.RecommendationStatus
		switch status {
		case "pending":
			recStatus = events.RecommendationStatusPending
		case "approved":
			recStatus = events.RecommendationStatusApproved
		case "rejected":
			recStatus = events.RecommendationStatusRejected
		case "expired":
			recStatus = events.RecommendationStatusExpired
		case "executing":
			recStatus = events.RecommendationStatusExecuting
		case "completed":
			recStatus = events.RecommendationStatusCompleted
		case "failed":
			recStatus = events.RecommendationStatusFailed
		default:
			http.Error(w, "Invalid status parameter", http.StatusBadRequest)
			return
		}
		recommendations = s.recommendationManager.GetRecommendationsByStatus(recStatus)
	} else {
		recommendations = s.recommendationManager.GetRecommendations()
	}

	// Filter by urgency if specified
	if urgency != "" {
		var filtered []*events.Recommendation
		for _, rec := range recommendations {
			if string(rec.Urgency) == urgency {
				filtered = append(filtered, rec)
			}
		}
		recommendations = filtered
	}

	// Map recommendations to frontend format
	var mappedRecommendations []map[string]interface{}
	for _, rec := range recommendations {
		mapped := map[string]interface{}{
			"id":                        rec.ID,
			"type":                      rec.Action,
			"title":                     rec.Title,
			"description":               rec.Description,
			"impact":                    rec.Description,
			"savings":                   0,
			"severity":                  strings.ToLower(string(rec.Urgency)),
			"status":                    string(rec.Status),
			"cluster_name":              "default-cluster",
			"workload_name":             rec.ResourceName,
			"namespace":                 rec.Namespace,
			"created_at":                rec.CreatedAt.Format(time.RFC3339),
			"estimated_monthly_savings": 0,
		}
		mappedRecommendations = append(mappedRecommendations, mapped)
	}

	response := map[string]interface{}{
		"recommendations": mappedRecommendations,
		"total":           len(mappedRecommendations),
		"timestamp":       time.Now(),
	}

	s.writeJSONResponse(w, response)
}

// handleGetRecommendationStats handles /api/recommendations/stats/summary endpoint
func (s *Server) handleGetRecommendationStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.recommendationManager == nil {
		http.Error(w, "Recommendation manager not available", http.StatusServiceUnavailable)
		return
	}

	recommendations := s.recommendationManager.GetRecommendations()

	// Calculate stats
	stats := map[string]interface{}{
		"total_recommendations":   len(recommendations),
		"pending":                 0,
		"applied":                 0,
		"dismissed":               0,
		"critical":                0,
		"high":                    0,
		"medium":                  0,
		"low":                     0,
		"total_potential_savings": 0,
		"realized_savings":        0,
	}

	for _, rec := range recommendations {
		switch rec.Status {
		case events.RecommendationStatusPending:
			stats["pending"] = stats["pending"].(int) + 1
		case events.RecommendationStatusApproved, events.RecommendationStatusExecuting, events.RecommendationStatusCompleted:
			stats["applied"] = stats["applied"].(int) + 1
		case events.RecommendationStatusRejected:
			stats["dismissed"] = stats["dismissed"].(int) + 1
		}

		switch rec.Urgency {
		case events.UrgencyCritical:
			stats["critical"] = stats["critical"].(int) + 1
		case events.UrgencyHigh:
			stats["high"] = stats["high"].(int) + 1
		case events.UrgencyMedium:
			stats["medium"] = stats["medium"].(int) + 1
		case events.UrgencyLow:
			stats["low"] = stats["low"].(int) + 1
		}
	}

	s.writeJSONResponse(w, stats)
}

// handleRecommendationByID handles /api/recommendations/{id}/{action} endpoints
func (s *Server) handleRecommendationByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/recommendations/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	id := parts[0]
	action := parts[1]

	switch action {
	case "apply":
		s.handleApplyRecommendationByID(w, r, id)
	case "dismiss":
		s.handleDismissRecommendationByID(w, r, id)
	default:
		http.Error(w, "Unknown action", http.StatusBadRequest)
	}
}

// handleApplyRecommendationByID handles apply action for specific recommendation
func (s *Server) handleApplyRecommendationByID(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.recommendationManager == nil {
		http.Error(w, "Recommendation manager not available", http.StatusServiceUnavailable)
		return
	}

	// Parse request body
	var req struct {
		AutoApply bool `json:"autoApply"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Approve the recommendation
	if err := s.recommendationManager.ApproveRecommendation(id, "user"); err != nil {
		logger.Error("Failed to approve recommendation: %v (id=%s)", err, id)
		http.Error(w, "Failed to approve recommendation", http.StatusInternalServerError)
		return
	}

	// If AutoApply is true, execute immediately
	if req.AutoApply {
		if err := s.recommendationManager.ExecuteRecommendation(id); err != nil {
			logger.Error("Failed to execute recommendation automatically: %v (id=%s)", err, id)
			// Don't return error to user as approval already succeeded,
			// but log it since it's an automated background action.
		}
	}

	response := map[string]interface{}{
		"message":          "Recommendation applied successfully",
		"recommendationId": id,
		"appliedAt":        time.Now(),
	}

	s.writeJSONResponse(w, response)
}

// handleDismissRecommendationByID handles dismiss action for specific recommendation
func (s *Server) handleDismissRecommendationByID(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.recommendationManager == nil {
		http.Error(w, "Recommendation manager not available", http.StatusServiceUnavailable)
		return
	}

	// Parse request body
	var req struct {
		Reason    string `json:"reason"`
		Permanent bool   `json:"permanent"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Reject the recommendation
	if err := s.recommendationManager.RejectRecommendation(id, "user", req.Reason); err != nil {
		logger.Error("Failed to reject recommendation: %v (id=%s)", err, id)
		http.Error(w, "Failed to dismiss recommendation", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"message":          "Recommendation dismissed successfully",
		"recommendationId": id,
		"dismissedAt":      time.Now(),
	}

	s.writeJSONResponse(w, response)
}

// handleApproveRecommendation handles /api/recommendations/approve endpoint
func (s *Server) handleApproveRecommendation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.recommendationManager == nil {
		http.Error(w, "Recommendation manager not available", http.StatusServiceUnavailable)
		return
	}

	// Parse request body
	var req struct {
		ID         string `json:"id"`
		ApprovedBy string `json:"approvedBy"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.ID == "" || req.ApprovedBy == "" {
		http.Error(w, "Missing id or approvedBy", http.StatusBadRequest)
		return
	}

	if err := s.recommendationManager.ApproveRecommendation(req.ID, req.ApprovedBy); err != nil {
		logger.Error("Failed to approve recommendation: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := map[string]interface{}{
		"status":    "approved",
		"id":        req.ID,
		"timestamp": time.Now(),
	}

	s.writeJSONResponse(w, response)
}

// handleRejectRecommendation handles /api/recommendations/reject endpoint
func (s *Server) handleRejectRecommendation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.recommendationManager == nil {
		http.Error(w, "Recommendation manager not available", http.StatusServiceUnavailable)
		return
	}

	// Parse request body
	var req struct {
		ID         string `json:"id"`
		RejectedBy string `json:"rejectedBy"`
		Reason     string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.ID == "" || req.RejectedBy == "" {
		http.Error(w, "Missing id or rejectedBy", http.StatusBadRequest)
		return
	}

	if err := s.recommendationManager.RejectRecommendation(req.ID, req.RejectedBy, req.Reason); err != nil {
		logger.Error("Failed to reject recommendation: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	response := map[string]interface{}{
		"status":    "rejected",
		"id":        req.ID,
		"timestamp": time.Now(),
	}

	s.writeJSONResponse(w, response)
}

// handleExecuteRecommendation handles /api/recommendations/execute endpoint
func (s *Server) handleExecuteRecommendation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.recommendationManager == nil {
		http.Error(w, "Recommendation manager not available", http.StatusServiceUnavailable)
		return
	}

	// Parse request body
	var req struct {
		ID string `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.ID == "" {
		http.Error(w, "Missing id", http.StatusBadRequest)
		return
	}

	if err := s.recommendationManager.ExecuteRecommendation(req.ID); err != nil {
		logger.Error("Failed to execute recommendation: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":    "executing",
		"id":        req.ID,
		"timestamp": time.Now(),
	}

	s.writeJSONResponse(w, response)
}

// handlePolicies handles /api/policies
func (s *Server) handlePolicies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listPolicies(w, r)
	case http.MethodPost:
		s.createPolicy(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePolicy handles /api/policies/{namespace}/{name}
func (s *Server) handlePolicy(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/policies/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		http.Error(w, "Invalid path: expected /api/policies/{namespace}/{name}", http.StatusBadRequest)
		return
	}
	namespace := parts[0]
	name := parts[1]

	switch r.Method {
	case http.MethodGet:
		s.getPolicy(w, r, namespace, name)
	case http.MethodPut:
		s.updatePolicy(w, r, namespace, name)
	case http.MethodDelete:
		s.deletePolicy(w, r, namespace, name)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) listPolicies(w http.ResponseWriter, r *http.Request) {
	namespace := r.URL.Query().Get("namespace")
	var list v1alpha1.RightSizerPolicyList
	opts := []client.ListOption{}
	if namespace != "" {
		opts = append(opts, client.InNamespace(namespace))
	}

	if err := s.ctrlClient.List(r.Context(), &list, opts...); err != nil {
		logger.Error("Failed to list policies: %v", err)
		http.Error(w, "Failed to list policies: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.writeJSONResponse(w, list.Items)
}

func (s *Server) getPolicy(w http.ResponseWriter, r *http.Request, namespace, name string) {
	var policy v1alpha1.RightSizerPolicy
	err := s.ctrlClient.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: name}, &policy)
	if err != nil {
		logger.Error("Failed to get policy %s/%s: %v", namespace, name, err)
		http.Error(w, "Failed to get policy: "+err.Error(), http.StatusNotFound)
		return
	}
	s.writeJSONResponse(w, policy)
}

func (s *Server) createPolicy(w http.ResponseWriter, r *http.Request) {
	var policy v1alpha1.RightSizerPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	policy.APIVersion = "rightsizer.io/v1alpha1"
	policy.Kind = "RightSizerPolicy"

	if err := s.ctrlClient.Create(r.Context(), &policy); err != nil {
		logger.Error("Failed to create policy %s/%s: %v", policy.Namespace, policy.Name, err)
		http.Error(w, "Failed to create policy: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.writeJSONResponse(w, policy)
}

func (s *Server) updatePolicy(w http.ResponseWriter, r *http.Request, namespace, name string) {
	var policy v1alpha1.RightSizerPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	policy.Namespace = namespace
	policy.Name = name
	policy.APIVersion = "rightsizer.io/v1alpha1"
	policy.Kind = "RightSizerPolicy"

	if err := s.ctrlClient.Update(r.Context(), &policy); err != nil {
		logger.Error("Failed to update policy %s/%s: %v", namespace, name, err)
		http.Error(w, "Failed to update policy: "+err.Error(), http.StatusInternalServerError)
		return
	}
	s.writeJSONResponse(w, policy)
}

func (s *Server) deletePolicy(w http.ResponseWriter, r *http.Request, namespace, name string) {
	var policy v1alpha1.RightSizerPolicy
	policy.Namespace = namespace
	policy.Name = name

	if err := s.ctrlClient.Delete(r.Context(), &policy); err != nil {
		logger.Error("Failed to delete policy %s/%s: %v", namespace, name, err)
		http.Error(w, "Failed to delete policy: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeJSONResponse writes JSON response
func (s *Server) writeJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logger.Error("Failed to encode JSON response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

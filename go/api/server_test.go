package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

func TestNewServer(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	metricsClient := metricsclient.NewSimpleClientset()

	server := NewServer(clientset, metricsClient, nil) // nil predictor for tests

	assert.NotNil(t, server)
	assert.NotNil(t, server.clientset)
	assert.NotNil(t, server.metricsClient)
	assert.Equal(t, clientset, server.clientset)
	assert.Equal(t, metricsClient, server.metricsClient)
}

func TestNewServer_WithoutMetricsClient(t *testing.T) {
	clientset := fake.NewSimpleClientset()

	server := NewServer(clientset, nil, nil)

	assert.NotNil(t, server)
	assert.NotNil(t, server.clientset)
	assert.Nil(t, server.metricsClient)
}

func TestServer_HandlePodCount(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	server := NewServer(clientset, nil, nil)

	// Create test pods
	pod1 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-1",
			Namespace: "default",
		},
	}
	pod2 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-2",
			Namespace: "kube-system",
		},
	}

	_, err := clientset.CoreV1().Pods("default").Create(context.Background(), pod1, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = clientset.CoreV1().Pods("kube-system").Create(context.Background(), pod2, metav1.CreateOptions{})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/pods/count", nil)
	w := httptest.NewRecorder()

	server.handlePodCount(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]int
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, 2, response["count"])
}

func TestServer_HandleHealth(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	server := NewServer(clientset, nil, nil)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "ok", response["status"])
}

func TestServer_HandleHealthCheck(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	server := NewServer(clientset, nil, nil)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealthCheck(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "api server healthy", w.Body.String())
}

func TestServer_WriteJSONResponse(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	server := NewServer(clientset, nil, nil)

	data := map[string]interface{}{
		"test":  "value",
		"count": 42,
	}

	w := httptest.NewRecorder()

	server.writeJSONResponse(w, data)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "value", response["test"])
	assert.Equal(t, float64(42), response["count"])
}

func TestServer_CalculateClusterMetrics(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	server := NewServer(clientset, nil, nil)

	// Create test pods with resources
	pod1 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-1",
			Namespace: "default",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name: "test-container",
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("100m"),
							v1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Limits: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("200m"),
							v1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				},
			},
		},
	}

	pod2 := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod-2",
			Namespace: "kube-system",
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name: "system-container",
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("50m"),
							v1.ResourceMemory: resource.MustParse("64Mi"),
						},
					},
				},
			},
		},
	}

	// Create test node
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("2000m"),
				v1.ResourceMemory: resource.MustParse("4096Mi"),
			},
		},
	}

	pods := []v1.Pod{*pod1, *pod2}
	nodes := []v1.Node{*node}

	metrics := server.calculateClusterMetrics(pods, nodes)

	assert.NotNil(t, metrics)
	assert.Equal(t, 2, metrics["totalPods"])
	assert.Equal(t, 1, metrics["totalNodes"])
	assert.Equal(t, 1, metrics["managedPods"]) // Only default namespace pod

	resources := metrics["resources"].(map[string]interface{})
	cpu := resources["cpu"].(map[string]interface{})
	memory := resources["memory"].(map[string]interface{})

	assert.Equal(t, "150.0m", cpu["totalRequests"])
	assert.Equal(t, "200.0m", cpu["totalLimits"])
	assert.Equal(t, "2000.0m", cpu["nodeCapacity"])
	assert.Contains(t, cpu["utilization"], "%")

	assert.Contains(t, memory["totalRequests"], "Mi")
	assert.Contains(t, memory["totalLimits"], "Mi")
	assert.Contains(t, memory["nodeCapacity"], "Mi")
	assert.Contains(t, memory["utilization"], "%")
}

func TestServer_ConvertPodsToMetricsAPI(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	server := NewServer(clientset, nil, nil)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name: "test-container",
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("100m"),
							v1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				},
			},
		},
	}

	pods := []v1.Pod{*pod}
	response := server.convertPodsToMetricsAPI(pods)

	assert.NotNil(t, response)
	assert.Equal(t, "PodMetricsList", response["kind"])
	assert.Equal(t, "metrics.k8s.io/v1beta1", response["apiVersion"])

	items := response["items"].([]map[string]interface{})
	assert.Len(t, items, 1)

	item := items[0]
	assert.Equal(t, "test-pod", item["metadata"].(map[string]interface{})["name"])
	assert.Equal(t, "default", item["metadata"].(map[string]interface{})["namespace"])

	containers := item["containers"].([]map[string]interface{})
	assert.Len(t, containers, 1)
	assert.Equal(t, "test-container", containers[0]["name"])
}

func TestServer_ConvertNodesToMetricsAPI(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	server := NewServer(clientset, nil, nil)

	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("2000m"),
				v1.ResourceMemory: resource.MustParse("4096Mi"),
			},
		},
	}

	nodes := []v1.Node{*node}
	response := server.convertNodesToMetricsAPI(nodes)

	assert.NotNil(t, response)
	assert.Equal(t, "NodeMetricsList", response["kind"])
	assert.Equal(t, "metrics.k8s.io/v1beta1", response["apiVersion"])

	items := response["items"].([]map[string]interface{})
	assert.Len(t, items, 1)

	item := items[0]
	assert.Equal(t, "test-node", item["metadata"].(map[string]interface{})["name"])
	assert.Equal(t, "2", item["usage"].(map[string]interface{})["cpu"]) // 2000m converts to canonical form "2"
}

func TestServer_FilterMetricsHistory(t *testing.T) {
	// Clear existing history
	metricsHistoryMu.Lock()
	metricsHistory = nil
	metricsHistoryMu.Unlock()

	// Add test samples
	now := time.Now()
	samples := []MetricSample{
		{Time: now.Add(-30 * time.Minute), CPUUsagePercent: 10.0}, // Within 1 hour
		{Time: now.Add(-2 * time.Hour), CPUUsagePercent: 20.0},    // Within 24 hours but not 1 hour
		{Time: now.Add(-25 * time.Hour), CPUUsagePercent: 30.0},   // Outside 24 hours
	}

	metricsHistoryMu.Lock()
	metricsHistory = append(metricsHistory, samples...)
	metricsHistoryMu.Unlock()

	tests := []struct {
		name        string
		rangeParam  string
		expectedLen int
		expectedCPU float64
	}{
		{
			name:        "no range returns all",
			rangeParam:  "",
			expectedLen: 3,
		},
		{
			name:        "1h range",
			rangeParam:  "1h",
			expectedLen: 1,
			expectedCPU: 10.0,
		},
		{
			name:        "24h range",
			rangeParam:  "24h",
			expectedLen: 2,
			expectedCPU: 10.0,
		},
		{
			name:        "invalid range",
			rangeParam:  "invalid",
			expectedLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterMetricsHistory(tt.rangeParam)
			assert.Len(t, result, tt.expectedLen)

			if tt.expectedLen > 0 && tt.expectedCPU > 0 {
				assert.Equal(t, tt.expectedCPU, result[0].CPUUsagePercent)
			}
		})
	}
}

func TestServer_SortAndLimitEvents(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	server := NewServer(clientset, nil, nil)

	events := []map[string]interface{}{
		{
			"timestamp": float64(1000),
			"eventId":   "event1",
		},
		{
			"timestamp": float64(2000),
			"eventId":   "event2",
		},
		{
			"timestamp": float64(1500),
			"eventId":   "event3",
		},
	}

	server.sortAndLimitEvents(&events, 2)

	assert.Len(t, events, 2)
	assert.Equal(t, float64(2000), events[0]["timestamp"])
	assert.Equal(t, float64(1500), events[1]["timestamp"])
}

func TestServer_ConvertAuditEvent(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	server := NewServer(clientset, nil, nil)

	auditEvent := map[string]interface{}{
		"timestamp":     "2023-01-01T00:00:00Z",
		"eventId":       "test-event-id",
		"podName":       "test-pod",
		"namespace":     "default",
		"containerName": "test-container",
		"operation":     "resource_change",
		"reason":        "optimization",
		"status":        "completed",
		"oldResources": map[string]interface{}{
			"requests": map[string]interface{}{
				"cpu":    "100m",
				"memory": "128Mi",
			},
		},
		"newResources": map[string]interface{}{
			"requests": map[string]interface{}{
				"cpu":    "150m",
				"memory": "192Mi",
			},
		},
	}

	event := server.convertAuditEvent(auditEvent)

	assert.NotNil(t, event)
	assert.Equal(t, "2023-01-01T00:00:00Z", event["timestamp"])
	assert.Equal(t, "test-event-id", event["eventId"])
	assert.Equal(t, "test-pod", event["podName"])
	assert.Equal(t, "default", event["namespace"])
	assert.Equal(t, "test-container", event["containerName"])
	assert.Equal(t, "resource_change", event["operation"])
	assert.Equal(t, "optimization", event["reason"])
	assert.Equal(t, "completed", event["status"])
	assert.Equal(t, "100m", event["previousCPU"])
	assert.Equal(t, "128Mi", event["previousMemory"])
	assert.Equal(t, "150m", event["currentCPU"])
	assert.Equal(t, "192Mi", event["currentMemory"])
	assert.Equal(t, "resource_optimization", event["optimizationType"])
}

func TestServer_HandlePods(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	server := NewServer(clientset, nil, nil)

	// Create test pod
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name: "test-container",
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("100m"),
							v1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				},
			},
		},
	}

	_, err := clientset.CoreV1().Pods("default").Create(context.Background(), pod, metav1.CreateOptions{})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/pods", nil)
	w := httptest.NewRecorder()

	server.handlePods(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))

	var pods []map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &pods)
	assert.NoError(t, err)
	assert.Len(t, pods, 1)
	assert.Equal(t, "test-pod", pods[0]["name"])
	assert.Equal(t, "default", pods[0]["namespace"])
}

func TestServer_HandlePodsV1(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	server := NewServer(clientset, nil, nil)

	// Create test pod
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}

	_, err := clientset.CoreV1().Pods("default").Create(context.Background(), pod, metav1.CreateOptions{})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/v1/pods", nil)
	w := httptest.NewRecorder()

	server.handlePodsV1(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "PodList", response["kind"])
	assert.Equal(t, "v1", response["apiVersion"])

	items := response["items"].([]interface{})
	assert.Len(t, items, 1)
}

func TestServer_HandlePodsRedirect(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	server := NewServer(clientset, nil, nil)

	req := httptest.NewRequest("GET", "/apis/v1/pods", nil)
	w := httptest.NewRecorder()

	server.handlePodsRedirect(w, req)

	assert.Equal(t, http.StatusPermanentRedirect, w.Code)
	location := w.Header().Get("Location")
	assert.Equal(t, "/api/v1/pods", location)
}

func TestServer_HandleMetricsHistory(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	server := NewServer(clientset, nil, nil)

	// Clear any existing metrics history to avoid test interference
	metricsHistoryMu.Lock()
	metricsHistory = []MetricSample{}
	metricsHistoryMu.Unlock()

	// Add test sample to history - ensure it's well within the 1h window
	sample := MetricSample{
		Time:               time.Now().Add(-30 * time.Minute), // 30 minutes ago, well within 1h
		CPUUsagePercent:    25.5,
		MemoryUsagePercent: 45.2,
		ActivePods:         10,
		OptimizedResources: 5,
		NetworkUsageMbps:   100.0,
		DiskIOMBps:         50.0,
		AvgUtilization:     35.35,
	}

	metricsHistoryMu.Lock()
	metricsHistory = append(metricsHistory, sample)
	metricsHistoryMu.Unlock()

	req := httptest.NewRequest("GET", "/api/metrics/history?range=1h", nil)
	w := httptest.NewRecorder()

	server.handleMetricsHistory(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "1h", response["range"])
	assert.Equal(t, float64(1), response["count"])

	samples := response["samples"].([]interface{})
	assert.Len(t, samples, 1)

	sampleData := samples[0].(map[string]interface{})
	assert.Equal(t, 25.5, sampleData["cpu"])
	assert.Equal(t, 45.2, sampleData["memory"])
	assert.Equal(t, float64(10), sampleData["pods"])
	assert.Equal(t, float64(5), sampleData["optimized"])
}

func TestServer_HandleSystemPods(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	server := NewServer(clientset, nil, nil)

	// Create pods in different namespaces
	systemPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "system-pod",
			Namespace: "kube-system",
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}

	userPod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "user-pod",
			Namespace: "default",
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
	}

	_, err := clientset.CoreV1().Pods("kube-system").Create(context.Background(), systemPod, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = clientset.CoreV1().Pods("default").Create(context.Background(), userPod, metav1.CreateOptions{})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/api/pods/system", nil)
	w := httptest.NewRecorder()

	server.handleSystemPods(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var pods []map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &pods)
	assert.NoError(t, err)
	assert.Len(t, pods, 1)
	assert.Equal(t, "system-pod", pods[0]["name"])
	assert.Equal(t, "kube-system", pods[0]["namespace"])
}

func TestServer_HandleOptimizationEvents(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	server := NewServer(clientset, nil, nil)

	req := httptest.NewRequest("GET", "/api/optimization-events", nil)
	w := httptest.NewRecorder()

	server.handleOptimizationEvents(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Contains(t, response, "events")
	assert.Contains(t, response, "total")
	assert.IsType(t, []interface{}{}, response["events"])
}

func TestServer_HandleNodesProxy(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	server := NewServer(clientset, nil, nil)

	// Create test node
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("2000m"),
				v1.ResourceMemory: resource.MustParse("4096Mi"),
			},
		},
	}

	_, err := clientset.CoreV1().Nodes().Create(context.Background(), node, metav1.CreateOptions{})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/apis/metrics.k8s.io/v1beta1/nodes", nil)
	w := httptest.NewRecorder()

	server.handleNodesProxy(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "NodeMetricsList", response["kind"])
	assert.Equal(t, "metrics.k8s.io/v1beta1", response["apiVersion"])

	items := response["items"].([]interface{})
	assert.Len(t, items, 1)
}

func TestServer_HandlePodsProxy(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	server := NewServer(clientset, nil, nil)

	// Create test pod
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name: "test-container",
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("100m"),
							v1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				},
			},
		},
	}

	_, err := clientset.CoreV1().Pods("default").Create(context.Background(), pod, metav1.CreateOptions{})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/apis/metrics.k8s.io/v1beta1/pods", nil)
	w := httptest.NewRecorder()

	server.handlePodsProxy(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "PodMetricsList", response["kind"])
	assert.Equal(t, "metrics.k8s.io/v1beta1", response["apiVersion"])

	items := response["items"].([]interface{})
	assert.Len(t, items, 1)
}

func TestServer_BuildEnhancedPodData(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	server := NewServer(clientset, nil, nil)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				"right-sizer.io/optimized":         "true",
				"right-sizer.io/optimization-type": "cpu_optimization",
				"right-sizer.io/savings":           "25.5",
			},
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			ContainerStatuses: []v1.ContainerStatus{
				{
					Name:         "test-container",
					RestartCount: 2,
				},
			},
		},
		Spec: v1.PodSpec{
			NodeName: "test-node",
			Containers: []v1.Container{
				{
					Name: "test-container",
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("100m"),
							v1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				},
			},
		},
	}

	pods := []v1.Pod{*pod}
	data := server.buildEnhancedPodData(context.Background(), pods)

	assert.Len(t, data, 1)
	podData := data[0]

	assert.Equal(t, "test-pod", podData["name"])
	assert.Equal(t, "default", podData["namespace"])
	assert.Equal(t, "Running", podData["status"])
	assert.Equal(t, "test-node", podData["nodeName"])
	assert.Equal(t, 2, podData["restartCount"])
	assert.Equal(t, true, podData["optimized"])
	assert.Equal(t, "cpu_optimization", podData["optimizationType"])
	assert.Equal(t, 25.5, podData["savings"])
}

func TestServer_ConvertPodsToV1API(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	server := NewServer(clientset, nil, nil)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
		},
		Spec: v1.PodSpec{
			NodeName: "test-node",
			Containers: []v1.Container{
				{
					Name: "test-container",
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("100m"),
							v1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Limits: v1.ResourceList{
							v1.ResourceCPU:    resource.MustParse("200m"),
							v1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				},
			},
		},
	}

	pods := []v1.Pod{*pod}
	response := server.convertPodsToV1API(pods)

	assert.NotNil(t, response)
	assert.Equal(t, "PodList", response["kind"])
	assert.Equal(t, "v1", response["apiVersion"])

	items := response["items"].([]map[string]interface{})
	assert.Len(t, items, 1)

	item := items[0]
	metadata := item["metadata"].(map[string]interface{})
	assert.Equal(t, "test-pod", metadata["name"])
	assert.Equal(t, "default", metadata["namespace"])

	spec := item["spec"].(map[string]interface{})
	assert.Equal(t, "test-node", spec["nodeName"])

	containers := spec["containers"].([]map[string]interface{})
	assert.Len(t, containers, 1)

	container := containers[0]
	assert.Equal(t, "test-container", container["name"])

	resources := container["resources"].(map[string]interface{})
	requests := resources["requests"].(map[string]interface{})
	assert.Equal(t, "100m", requests["cpu"])
	assert.Equal(t, "128Mi", requests["memory"])

	limits := resources["limits"].(map[string]interface{})
	assert.Equal(t, "200m", limits["cpu"])
	assert.Equal(t, "256Mi", limits["memory"])
}

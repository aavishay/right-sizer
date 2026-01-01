package collector

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"right-sizer/logger"
)

// RCACollector gathers context for Root Cause Analysis
type RCACollector struct {
	clientset kubernetes.Interface
}

// NewRCACollector creates a new RCACollector
func NewRCACollector(clientset kubernetes.Interface) *RCACollector {
	return &RCACollector{
		clientset: clientset,
	}
}

// RCAData holds collected troubleshooting context
type RCAData struct {
	RelatedEvents []K8sEvent `json:"relatedEvents"`
	Logs          string     `json:"logs"`
	CollectedAt   time.Time  `json:"collectedAt"`
}

// K8sEvent simplifies the kubernetes event for the dashboard
type K8sEvent struct {
	Type     string    `json:"type"`
	Reason   string    `json:"reason"`
	Message  string    `json:"message"`
	Count    int32     `json:"count"`
	LastSeen time.Time `json:"lastSeen"`
}

// CollectContext gathers logs and events for a specific pod/container
func (c *RCACollector) CollectContext(ctx context.Context, namespace, podName, containerName string) (*RCAData, error) {
	data := &RCAData{
		CollectedAt:   time.Now().UTC(),
		RelatedEvents: []K8sEvent{},
	}

	// 1. Collect Logs
	logs, err := c.collectLogs(ctx, namespace, podName, containerName)
	if err != nil {
		logger.Error("[RCA] Failed to collect logs for %s/%s/%s: %v", namespace, podName, containerName, err)
		data.Logs = fmt.Sprintf("Failed to collect logs: %v", err)
	} else {
		data.Logs = logs
	}

	// 2. Collect Events
	events, err := c.collectEvents(ctx, namespace, podName)
	if err != nil {
		logger.Error("[RCA] Failed to collect events for %s/%s: %v", namespace, podName, err)
	} else {
		data.RelatedEvents = events
	}

	return data, nil
}

func (c *RCACollector) collectLogs(ctx context.Context, namespace, podName, containerName string) (string, error) {
	opts := &corev1.PodLogOptions{
		Container:  containerName,
		TailLines:  int64Ptr(50), // Fetch last 50 lines
		Timestamps: true,
		Previous:   false, // Get current logs, maybe consider previous if crashed?
	}

	req := c.clientset.CoreV1().Pods(namespace).GetLogs(podName, opts)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		// Try previous container if current fails (common for crash loops)
		opts.Previous = true
		req = c.clientset.CoreV1().Pods(namespace).GetLogs(podName, opts)
		podLogs, err = req.Stream(ctx)
		if err != nil {
			return "", err
		}
	}
	defer podLogs.Close()

	buf := new(strings.Builder)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (c *RCACollector) collectEvents(ctx context.Context, namespace, podName string) ([]K8sEvent, error) {
	// List events involving the pod
	events, err := c.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", podName),
	})
	if err != nil {
		return nil, err
	}

	var results []K8sEvent
	for _, e := range events.Items {
		// Filter relevant events (optional: could filter by time window)
		results = append(results, K8sEvent{
			Type:     e.Type,
			Reason:   e.Reason,
			Message:  e.Message,
			Count:    e.Count,
			LastSeen: e.LastTimestamp.Time.UTC(),
		})
	}

	return results, nil
}

func int64Ptr(i int64) *int64 {
	return &i
}

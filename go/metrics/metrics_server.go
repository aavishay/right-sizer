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
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewMetricsServerProvider returns a new metrics-server provider
func NewMetricsServerProvider(client client.Client) Provider {
	// Get the REST config
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fallback for local development
		config = &rest.Config{
			Host: "https://localhost:6443",
		}
	}

	// Create metrics client
	metricsClient, err := metricsclient.NewForConfig(config)
	if err != nil {
		// Return a provider that will fail gracefully
		return &MetricsServerProvider{Client: client, MetricsClient: nil}
	}

	return &MetricsServerProvider{Client: client, MetricsClient: metricsClient}
}

// FetchPodMetrics fetches CPU and memory usage for a pod from metrics-server
func (m *MetricsServerProvider) FetchPodMetrics(namespace, podName string) (Metrics, error) {
	if m.MetricsClient == nil {
		return Metrics{}, fmt.Errorf("metrics client not available")
	}

	// Get pod metrics from metrics-server
	podMetrics, err := m.MetricsClient.MetricsV1beta1().PodMetricses(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return Metrics{}, fmt.Errorf("failed to get pod metrics: %w", err)
	}

	// Sum metrics across all containers
	var totalCPUMilli float64
	var totalMemBytes int64

	for _, container := range podMetrics.Containers {
		// CPU usage in millicores
		if cpuUsage, ok := container.Usage["cpu"]; ok {
			totalCPUMilli += float64(cpuUsage.MilliValue())
		}

		// Memory usage in bytes
		if memUsage, ok := container.Usage["memory"]; ok {
			totalMemBytes += memUsage.Value()
		}
	}

	// Convert memory bytes to MB
	totalMemMB := float64(totalMemBytes) / (1024 * 1024)

	return Metrics{
		CPUMilli: totalCPUMilli,
		MemMB:    totalMemMB,
	}, nil
}

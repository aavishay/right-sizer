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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewMetricsServerProvider returns a new metrics-server provider
func NewMetricsServerProvider(client client.Client) Provider {
	return &MetricsServerProvider{Client: client}
}

// FetchPodMetrics fetches CPU and memory usage for a pod from metrics-server
func (m *MetricsServerProvider) FetchPodMetrics(namespace, podName string) (Metrics, error) {
	// TODO: Implement fetching from metrics.k8s.io/v1beta1 API
	// This is a stub returning variable test values for now

	// Generate variable test data based on pod name hash
	hash := 0
	for _, c := range podName {
		hash += int(c)
	}

	// Create some variation in the metrics
	baseCPU := float64(50 + (hash % 100))  // 50-150 millicores
	baseMem := float64(128 + (hash % 256)) // 128-384 MB

	// Add some time-based variation
	variation := float64(hash%20) / 10.0 // 0-2x multiplier

	return Metrics{
		CPUMilli: baseCPU * (1 + variation*0.5),
		MemMB:    baseMem * (1 + variation*0.3),
	}, nil
}

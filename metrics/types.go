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

// Metrics holds CPU and memory usage values
type Metrics struct {
	CPUMilli float64 // CPU usage in millicores
	MemMB    float64 // Memory usage in MB
}

// Provider interface for metrics sources
type Provider interface {
	FetchPodMetrics(namespace, podName string) (Metrics, error)
}

// MetricsServerProvider fetches metrics from metrics-server
type MetricsServerProvider struct {
	Client client.Client
}

// PrometheusProvider implements Provider for Prometheus
type PrometheusProvider struct {
	URL string
}

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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// NewPrometheusProvider returns a PrometheusProvider
func NewPrometheusProvider(promURL string) Provider {
	return &PrometheusProvider{URL: promURL}
}

// FetchPodMetrics queries Prometheus for CPU and memory usage for a pod
func (p *PrometheusProvider) FetchPodMetrics(ctx context.Context, namespace, podName string) (Metrics, error) {
	// Query CPU usage (millicores)
	cpuQuery := fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{namespace="%s", pod="%s"}[5m])) * 1000`, namespace, podName)
	cpuMilli, err := p.queryPrometheus(ctx, cpuQuery)
	if err != nil {
		return Metrics{}, fmt.Errorf("failed to query CPU metrics: %w", err)
	}

	// Query memory usage (bytes)
	memQuery := fmt.Sprintf(`sum(container_memory_usage_bytes{namespace="%s", pod="%s"})`, namespace, podName)
	memBytes, err := p.queryPrometheus(ctx, memQuery)
	if err != nil {
		return Metrics{}, fmt.Errorf("failed to query memory metrics: %w", err)
	}

	// Query CPU throttling percentage
	// Formula: (sum of increase in throttled time) / (sum of increase in total CPU time) * 100
	throttledQuery := fmt.Sprintf(`
		sum(increase(container_cpu_cfs_throttled_seconds_total{namespace="%s", pod="%s"}[5m]))
		/
		sum(increase(container_cpu_usage_seconds_total{namespace="%s", pod="%s"}[5m]))
		* 100`, namespace, podName, namespace, podName)

	cpuThrottled, err := p.queryPrometheus(ctx, throttledQuery)
	if err != nil {
		// Throttling might not be available or 0 if no usage
		cpuThrottled = 0
	}

	// Convert bytes to MB
	memMB := memBytes / (1024 * 1024)

	return Metrics{
		CPUMilli:     cpuMilli,
		MemMB:        memMB,
		CPUThrottled: cpuThrottled,
	}, nil
}

// queryPrometheus runs a Prometheus instant query and returns the value
func (p *PrometheusProvider) queryPrometheus(ctx context.Context, query string) (float64, error) {
	endpoint := fmt.Sprintf("%s/api/v1/query?query=%s", p.URL, url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Value []interface{} `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}

	if result.Status != "success" || len(result.Data.Result) == 0 {
		return 0, errors.New("no data returned from Prometheus")
	}

	// Value[1] is the string representation of the metric value
	valStr, ok := result.Data.Result[0].Value[1].(string)
	if !ok {
		return 0, errors.New("unexpected value format")
	}

	var val float64
	_, err = fmt.Sscanf(valStr, "%f", &val)
	if err != nil {
		return 0, err
	}
	return val, nil
}

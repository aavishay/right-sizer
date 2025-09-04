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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

// NewPrometheusProvider returns a PrometheusProvider
func NewPrometheusProvider(promURL string) Provider {
	return &PrometheusProvider{URL: promURL}
}

// FetchPodMetrics queries Prometheus for CPU and memory usage for a pod
func (p *PrometheusProvider) FetchPodMetrics(namespace, podName string) (Metrics, error) {
	// Query CPU usage (millicores)
	cpuQuery := fmt.Sprintf(`sum(rate(container_cpu_usage_seconds_total{namespace="%s", pod="%s"}[5m])) * 1000`, namespace, podName)
	cpuMilli, err := p.queryPrometheus(cpuQuery)
	if err != nil {
		return Metrics{}, fmt.Errorf("failed to query CPU metrics: %w", err)
	}

	// Query memory usage (bytes)
	memQuery := fmt.Sprintf(`sum(container_memory_usage_bytes{namespace="%s", pod="%s"})`, namespace, podName)
	memBytes, err := p.queryPrometheus(memQuery)
	if err != nil {
		return Metrics{}, fmt.Errorf("failed to query memory metrics: %w", err)
	}

	// Convert bytes to MB
	memMB := memBytes / (1024 * 1024)

	return Metrics{
		CPUMilli: cpuMilli,
		MemMB:    memMB,
	}, nil
}

// queryPrometheus runs a Prometheus instant query and returns the value
func (p *PrometheusProvider) queryPrometheus(query string) (float64, error) {
	endpoint := fmt.Sprintf("%s/api/v1/query?query=%s", p.URL, url.QueryEscape(query))
	resp, err := http.Get(endpoint)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
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
		return 0, fmt.Errorf("no data returned from Prometheus")
	}

	// Value[1] is the string representation of the metric value
	valStr, ok := result.Data.Result[0].Value[1].(string)
	if !ok {
		return 0, fmt.Errorf("unexpected value format")
	}

	var val float64
	_, err = fmt.Sscanf(valStr, "%f", &val)
	if err != nil {
		return 0, err
	}
	return val, nil
}

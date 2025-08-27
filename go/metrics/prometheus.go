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
	// TODO: Implement real Prometheus queries
	// For now, return test data similar to metrics-server provider

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

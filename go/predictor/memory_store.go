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

package predictor

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// MemoryStore implements an in-memory prediction store
type MemoryStore struct {
	historicalData map[string][]DataPoint          // key: namespace/pod/container/resourceType
	predictions    map[string][]ResourcePrediction // key: namespace/pod/container/resourceType
	config         *Config
	mutex          sync.RWMutex
	lastCleanup    time.Time
}

// NewMemoryStore creates a new in-memory prediction store
func NewMemoryStore(config *Config) *MemoryStore {
	if config == nil {
		config = DefaultConfig()
	}

	return &MemoryStore{
		historicalData: make(map[string][]DataPoint),
		predictions:    make(map[string][]ResourcePrediction),
		config:         config,
		lastCleanup:    time.Now(),
	}
}

// makeKey creates a storage key for a resource
func (s *MemoryStore) makeKey(namespace, podName, container, resourceType string) string {
	return fmt.Sprintf("%s/%s/%s/%s", namespace, podName, container, resourceType)
}

// StoreHistoricalData stores a new data point
func (s *MemoryStore) StoreHistoricalData(namespace, podName, container, resourceType string, dataPoint DataPoint) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	key := s.makeKey(namespace, podName, container, resourceType)

	// Validate data point
	if dataPoint.Timestamp.IsZero() {
		return fmt.Errorf("invalid timestamp")
	}
	if math.IsNaN(dataPoint.Value) || math.IsInf(dataPoint.Value, 0) {
		return fmt.Errorf("invalid value: %f", dataPoint.Value)
	}

	// Add namespace, pod, and container info if not present
	if dataPoint.Namespace == "" {
		dataPoint.Namespace = namespace
	}
	if dataPoint.PodName == "" {
		dataPoint.PodName = podName
	}
	if dataPoint.Container == "" {
		dataPoint.Container = container
	}

	// Get existing data points
	dataPoints := s.historicalData[key]

	// Add new data point
	dataPoints = append(dataPoints, dataPoint)

	// Sort by timestamp
	sort.Slice(dataPoints, func(i, j int) bool {
		return dataPoints[i].Timestamp.Before(dataPoints[j].Timestamp)
	})

	// Remove old data points based on retention policy
	cutoff := time.Now().Add(-s.config.HistoricalDataRetention)
	var filteredData []DataPoint
	for _, dp := range dataPoints {
		if dp.Timestamp.After(cutoff) {
			filteredData = append(filteredData, dp)
		}
	}

	// Store the filtered data
	s.historicalData[key] = filteredData

	// Trigger cleanup if needed
	if time.Since(s.lastCleanup) > time.Hour {
		go s.performCleanup()
	}

	return nil
}

// GetHistoricalData retrieves historical data for a resource
func (s *MemoryStore) GetHistoricalData(namespace, podName, container, resourceType string, since time.Time) (HistoricalData, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	key := s.makeKey(namespace, podName, container, resourceType)
	dataPoints := s.historicalData[key]

	// Filter data points by time
	var filteredData []DataPoint
	for _, dp := range dataPoints {
		if dp.Timestamp.After(since) {
			filteredData = append(filteredData, dp)
		}
	}

	// Calculate min and max values
	var minValue, maxValue float64
	if len(filteredData) > 0 {
		minValue = filteredData[0].Value
		maxValue = filteredData[0].Value

		for _, dp := range filteredData {
			if dp.Value < minValue {
				minValue = dp.Value
			}
			if dp.Value > maxValue {
				maxValue = dp.Value
			}
		}
	}

	return HistoricalData{
		ResourceType: resourceType,
		DataPoints:   filteredData,
		MinValue:     minValue,
		MaxValue:     maxValue,
		LastUpdated:  time.Now(),
	}, nil
}

// StorePrediction stores a prediction result
func (s *MemoryStore) StorePrediction(namespace, podName, container, resourceType string, prediction ResourcePrediction) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	key := s.makeKey(namespace, podName, container, resourceType)

	// Get existing predictions
	predictions := s.predictions[key]

	// Add new prediction
	predictions = append(predictions, prediction)

	// Sort by timestamp (newest first)
	sort.Slice(predictions, func(i, j int) bool {
		return predictions[i].Timestamp.After(predictions[j].Timestamp)
	})

	// Remove old predictions based on retention policy
	cutoff := time.Now().Add(-s.config.PredictionRetention)
	var filteredPredictions []ResourcePrediction
	for _, p := range predictions {
		if p.Timestamp.After(cutoff) {
			filteredPredictions = append(filteredPredictions, p)
		}
	}

	// Store the filtered predictions
	s.predictions[key] = filteredPredictions

	return nil
}

// GetPredictions retrieves stored predictions
func (s *MemoryStore) GetPredictions(namespace, podName, container, resourceType string, since time.Time) ([]ResourcePrediction, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	key := s.makeKey(namespace, podName, container, resourceType)
	predictions := s.predictions[key]

	// Filter predictions by time
	var filteredPredictions []ResourcePrediction
	for _, p := range predictions {
		if p.Timestamp.After(since) {
			filteredPredictions = append(filteredPredictions, p)
		}
	}

	return filteredPredictions, nil
}

// CleanupOldData removes old historical data and predictions
func (s *MemoryStore) CleanupOldData(olderThan time.Time) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Clean up historical data
	for key, dataPoints := range s.historicalData {
		var filteredData []DataPoint
		for _, dp := range dataPoints {
			if dp.Timestamp.After(olderThan) {
				filteredData = append(filteredData, dp)
			}
		}

		if len(filteredData) == 0 {
			delete(s.historicalData, key)
		} else {
			s.historicalData[key] = filteredData
		}
	}

	// Clean up predictions
	for key, predictions := range s.predictions {
		var filteredPredictions []ResourcePrediction
		for _, p := range predictions {
			if p.Timestamp.After(olderThan) {
				filteredPredictions = append(filteredPredictions, p)
			}
		}

		if len(filteredPredictions) == 0 {
			delete(s.predictions, key)
		} else {
			s.predictions[key] = filteredPredictions
		}
	}

	s.lastCleanup = time.Now()
	return nil
}

// performCleanup performs automatic cleanup based on retention policies
func (s *MemoryStore) performCleanup() {
	historicalCutoff := time.Now().Add(-s.config.HistoricalDataRetention)
	predictionCutoff := time.Now().Add(-s.config.PredictionRetention)

	// Use the earliest cutoff time
	cutoff := historicalCutoff
	if predictionCutoff.Before(historicalCutoff) {
		cutoff = predictionCutoff
	}

	if err := s.CleanupOldData(cutoff); err != nil {
		// Log error but don't fail
		fmt.Printf("Error during automatic cleanup: %v\n", err)
	}
}

// GetStats returns statistics about the memory store
func (s *MemoryStore) GetStats() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var totalDataPoints, totalPredictions int
	for _, dataPoints := range s.historicalData {
		totalDataPoints += len(dataPoints)
	}
	for _, predictions := range s.predictions {
		totalPredictions += len(predictions)
	}

	return map[string]interface{}{
		"totalResources":      len(s.historicalData),
		"totalDataPoints":     totalDataPoints,
		"totalPredictions":    totalPredictions,
		"lastCleanup":         s.lastCleanup,
		"dataRetention":       s.config.HistoricalDataRetention.String(),
		"predictionRetention": s.config.PredictionRetention.String(),
	}
}

// GetResourceKeys returns all resource keys currently stored
func (s *MemoryStore) GetResourceKeys() []string {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	keys := make([]string, 0, len(s.historicalData))
	for key := range s.historicalData {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	return keys
}

// GetDataPointCount returns the number of data points for a specific resource
func (s *MemoryStore) GetDataPointCount(namespace, podName, container, resourceType string) int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	key := s.makeKey(namespace, podName, container, resourceType)
	return len(s.historicalData[key])
}

// GetPredictionCount returns the number of predictions for a specific resource
func (s *MemoryStore) GetPredictionCount(namespace, podName, container, resourceType string) int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	key := s.makeKey(namespace, podName, container, resourceType)
	return len(s.predictions[key])
}

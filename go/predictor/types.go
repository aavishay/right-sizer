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
	"time"
)

// ResourcePrediction represents a prediction for a resource (CPU or Memory)
type ResourcePrediction struct {
	Value              float64                `json:"value"`              // Predicted value
	Confidence         float64                `json:"confidence"`         // Confidence score (0-1)
	Horizon            time.Duration          `json:"horizon"`            // How far into the future this prediction is for
	Timestamp          time.Time              `json:"timestamp"`          // When this prediction was made
	Method             PredictionMethod       `json:"method"`             // Which algorithm was used
	ConfidenceInterval *ConfidenceInterval    `json:"confidenceInterval"` // Statistical confidence bounds
	Metadata           map[string]interface{} `json:"metadata"`           // Additional metadata about the prediction
}

// ConfidenceInterval represents statistical bounds for a prediction
type ConfidenceInterval struct {
	Lower      float64 `json:"lower"`      // Lower bound
	Upper      float64 `json:"upper"`      // Upper bound
	Percentage float64 `json:"percentage"` // Confidence percentage (e.g., 95%)
}

// PredictionMethod represents different prediction algorithms
type PredictionMethod string

const (
	PredictionMethodLinearRegression     PredictionMethod = "linear_regression"
	PredictionMethodExponentialSmoothing PredictionMethod = "exponential_smoothing"
	PredictionMethodSeasonal             PredictionMethod = "seasonal"
	PredictionMethodEnsemble             PredictionMethod = "ensemble"
	PredictionMethodSimpleMovingAverage  PredictionMethod = "simple_moving_average"
)

// DataPoint represents a single historical data point
type DataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Namespace string    `json:"namespace,omitempty"`
	PodName   string    `json:"podName,omitempty"`
	Container string    `json:"container,omitempty"`
}

// HistoricalData represents a time series of resource usage data
type HistoricalData struct {
	ResourceType string      `json:"resourceType"` // "cpu" or "memory"
	DataPoints   []DataPoint `json:"dataPoints"`
	MinValue     float64     `json:"minValue"`
	MaxValue     float64     `json:"maxValue"`
	LastUpdated  time.Time   `json:"lastUpdated"`
}

// PredictionRequest represents a request for resource prediction
type PredictionRequest struct {
	Namespace    string             `json:"namespace"`
	PodName      string             `json:"podName"`
	Container    string             `json:"container"`
	ResourceType string             `json:"resourceType"` // "cpu" or "memory"
	Horizons     []time.Duration    `json:"horizons"`     // Multiple prediction horizons
	Methods      []PredictionMethod `json:"methods"`      // Which algorithms to use
}

// PredictionResponse represents the response containing multiple predictions
type PredictionResponse struct {
	Request     PredictionRequest    `json:"request"`
	Predictions []ResourcePrediction `json:"predictions"`
	Timestamp   time.Time            `json:"timestamp"`
	DataPoints  int                  `json:"dataPoints"` // Number of historical data points used
}

// PredictionAccuracy represents accuracy metrics for a prediction
type PredictionAccuracy struct {
	Method                      PredictionMethod `json:"method"`
	MeanAbsoluteError           float64          `json:"meanAbsoluteError"`
	MeanSquaredError            float64          `json:"meanSquaredError"`
	MeanAbsolutePercentageError float64          `json:"meanAbsolutePercentageError"`
	R2Score                     float64          `json:"r2Score"`
	LastEvaluated               time.Time        `json:"lastEvaluated"`
	SampleSize                  int              `json:"sampleSize"`
}

// Predictor interface defines the contract for prediction algorithms
type Predictor interface {
	// Predict generates resource predictions based on historical data
	Predict(data HistoricalData, horizons []time.Duration) ([]ResourcePrediction, error)

	// GetMethod returns the prediction method this predictor implements
	GetMethod() PredictionMethod

	// GetMinDataPoints returns the minimum number of data points required
	GetMinDataPoints() int

	// ValidateData checks if the historical data is suitable for this predictor
	ValidateData(data HistoricalData) error
}

// PredictionStore interface defines storage operations for predictions and historical data
type PredictionStore interface {
	// StoreHistoricalData stores a new data point
	StoreHistoricalData(namespace, podName, container, resourceType string, dataPoint DataPoint) error

	// GetHistoricalData retrieves historical data for a resource
	GetHistoricalData(namespace, podName, container, resourceType string, since time.Time) (HistoricalData, error)

	// StorePrediction stores a prediction result
	StorePrediction(namespace, podName, container, resourceType string, prediction ResourcePrediction) error

	// GetPredictions retrieves stored predictions
	GetPredictions(namespace, podName, container, resourceType string, since time.Time) ([]ResourcePrediction, error)

	// CleanupOldData removes old historical data and predictions
	CleanupOldData(olderThan time.Time) error
}

// Config holds configuration for the prediction system
type Config struct {
	// Data retention
	HistoricalDataRetention time.Duration `json:"historicalDataRetention"` // How long to keep historical data
	PredictionRetention     time.Duration `json:"predictionRetention"`     // How long to keep predictions

	// Data collection
	CollectionInterval time.Duration `json:"collectionInterval"` // How often to collect data points
	MinDataPoints      int           `json:"minDataPoints"`      // Minimum data points for predictions

	// Prediction settings
	DefaultHorizons     []time.Duration    `json:"defaultHorizons"`     // Default prediction horizons
	EnabledMethods      []PredictionMethod `json:"enabledMethods"`      // Which prediction methods to use
	ConfidenceThreshold float64            `json:"confidenceThreshold"` // Minimum confidence for using predictions

	// Performance
	MaxConcurrentPredictions int           `json:"maxConcurrentPredictions"` // Limit concurrent prediction calculations
	PredictionTimeout        time.Duration `json:"predictionTimeout"`        // Timeout for prediction calculations

	// Storage
	StorageDriver string `json:"storageDriver"` // "memory", "prometheus", etc.
}

// DefaultConfig returns a sensible default configuration
func DefaultConfig() *Config {
	return &Config{
		HistoricalDataRetention: 7 * 24 * time.Hour, // 7 days
		PredictionRetention:     24 * time.Hour,     // 1 day
		CollectionInterval:      1 * time.Minute,    // 1 minute
		MinDataPoints:           10,                 // At least 10 data points
		DefaultHorizons: []time.Duration{
			5 * time.Minute,
			15 * time.Minute,
			1 * time.Hour,
			6 * time.Hour,
			24 * time.Hour,
		},
		EnabledMethods: []PredictionMethod{
			PredictionMethodLinearRegression,
			PredictionMethodExponentialSmoothing,
			PredictionMethodSimpleMovingAverage,
		},
		ConfidenceThreshold:      0.6, // 60% confidence minimum
		MaxConcurrentPredictions: 10,
		PredictionTimeout:        30 * time.Second,
		StorageDriver:            "memory",
	}
}

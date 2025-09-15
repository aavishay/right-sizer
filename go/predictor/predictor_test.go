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
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinearRegressionPredictor(t *testing.T) {
	predictor := NewLinearRegressionPredictor()
	
	// Test basic properties
	assert.Equal(t, PredictionMethodLinearRegression, predictor.GetMethod())
	assert.Equal(t, 5, predictor.GetMinDataPoints())
	
	// Create test data with a clear upward trend
	baseTime := time.Now().Add(-1 * time.Hour)
	dataPoints := []DataPoint{
		{Timestamp: baseTime, Value: 100},
		{Timestamp: baseTime.Add(10 * time.Minute), Value: 110},
		{Timestamp: baseTime.Add(20 * time.Minute), Value: 120},
		{Timestamp: baseTime.Add(30 * time.Minute), Value: 130},
		{Timestamp: baseTime.Add(40 * time.Minute), Value: 140},
		{Timestamp: baseTime.Add(50 * time.Minute), Value: 150},
	}
	
	historicalData := HistoricalData{
		ResourceType: "cpu",
		DataPoints:   dataPoints,
		MinValue:     100,
		MaxValue:     150,
		LastUpdated:  time.Now(),
	}
	
	// Test validation
	err := predictor.ValidateData(historicalData)
	assert.NoError(t, err)
	
	// Test prediction
	horizons := []time.Duration{10 * time.Minute, 30 * time.Minute}
	predictions, err := predictor.Predict(historicalData, horizons)
	require.NoError(t, err)
	require.Len(t, predictions, 2)
	
	// Check that predictions follow the upward trend
	for _, pred := range predictions {
		assert.Greater(t, pred.Value, 150.0) // Should be higher than last value
		assert.Greater(t, pred.Confidence, 0.0)
		assert.LessOrEqual(t, pred.Confidence, 1.0)
		assert.Equal(t, PredictionMethodLinearRegression, pred.Method)
		assert.NotNil(t, pred.ConfidenceInterval)
		assert.Contains(t, pred.Metadata, "slope")
		assert.Contains(t, pred.Metadata, "r2")
	}
}

func TestExponentialSmoothingPredictor(t *testing.T) {
	predictor := NewExponentialSmoothingPredictor()
	
	// Test basic properties
	assert.Equal(t, PredictionMethodExponentialSmoothing, predictor.GetMethod())
	assert.Equal(t, 3, predictor.GetMinDataPoints())
	
	// Test parameter setting
	err := predictor.SetAlpha(0.5)
	assert.NoError(t, err)
	assert.Equal(t, 0.5, predictor.GetAlpha())
	
	err = predictor.SetBeta(0.2)
	assert.NoError(t, err)
	assert.Equal(t, 0.2, predictor.GetBeta())
	
	// Test invalid parameters
	err = predictor.SetAlpha(1.5)
	assert.Error(t, err)
	
	err = predictor.SetBeta(-0.1)
	assert.Error(t, err)
	
	// Create test data with some variation
	baseTime := time.Now().Add(-30 * time.Minute)
	dataPoints := []DataPoint{
		{Timestamp: baseTime, Value: 100},
		{Timestamp: baseTime.Add(5 * time.Minute), Value: 105},
		{Timestamp: baseTime.Add(10 * time.Minute), Value: 102},
		{Timestamp: baseTime.Add(15 * time.Minute), Value: 108},
		{Timestamp: baseTime.Add(20 * time.Minute), Value: 112},
	}
	
	historicalData := HistoricalData{
		ResourceType: "memory",
		DataPoints:   dataPoints,
		MinValue:     100,
		MaxValue:     112,
		LastUpdated:  time.Now(),
	}
	
	// Test prediction
	horizons := []time.Duration{5 * time.Minute, 15 * time.Minute}
	predictions, err := predictor.Predict(historicalData, horizons)
	require.NoError(t, err)
	require.Len(t, predictions, 2)
	
	for _, pred := range predictions {
		assert.Greater(t, pred.Value, 0.0)
		assert.Greater(t, pred.Confidence, 0.0)
		assert.LessOrEqual(t, pred.Confidence, 1.0)
		assert.Equal(t, PredictionMethodExponentialSmoothing, pred.Method)
		assert.NotNil(t, pred.ConfidenceInterval)
		assert.Contains(t, pred.Metadata, "level")
		assert.Contains(t, pred.Metadata, "trend")
	}
}

func TestSimpleMovingAveragePredictor(t *testing.T) {
	predictor := NewSimpleMovingAveragePredictor(3)
	
	// Test basic properties
	assert.Equal(t, PredictionMethodSimpleMovingAverage, predictor.GetMethod())
	assert.Equal(t, 3, predictor.GetMinDataPoints())
	assert.Equal(t, 3, predictor.GetWindowSize())
	
	// Test window size setting
	err := predictor.SetWindowSize(5)
	assert.NoError(t, err)
	assert.Equal(t, 5, predictor.GetWindowSize())
	
	err = predictor.SetWindowSize(0)
	assert.Error(t, err)
	
	// Create test data
	baseTime := time.Now().Add(-20 * time.Minute)
	dataPoints := []DataPoint{
		{Timestamp: baseTime, Value: 100},
		{Timestamp: baseTime.Add(5 * time.Minute), Value: 110},
		{Timestamp: baseTime.Add(10 * time.Minute), Value: 105},
		{Timestamp: baseTime.Add(15 * time.Minute), Value: 115},
	}
	
	historicalData := HistoricalData{
		ResourceType: "cpu",
		DataPoints:   dataPoints,
		MinValue:     100,
		MaxValue:     115,
		LastUpdated:  time.Now(),
	}
	
	// Test prediction
	horizons := []time.Duration{5 * time.Minute, 10 * time.Minute}
	predictions, err := predictor.Predict(historicalData, horizons)
	require.NoError(t, err)
	require.Len(t, predictions, 2)
	
	for _, pred := range predictions {
		assert.Greater(t, pred.Value, 0.0)
		assert.Greater(t, pred.Confidence, 0.0)
		assert.LessOrEqual(t, pred.Confidence, 1.0)
		assert.Equal(t, PredictionMethodSimpleMovingAverage, pred.Method)
		assert.NotNil(t, pred.ConfidenceInterval)
		assert.Contains(t, pred.Metadata, "movingAverage")
	}
}

func TestMemoryStore(t *testing.T) {
	config := DefaultConfig()
	config.HistoricalDataRetention = 1 * time.Hour
	config.PredictionRetention = 30 * time.Minute
	
	store := NewMemoryStore(config)
	
	namespace := "test-ns"
	podName := "test-pod"
	container := "test-container"
	resourceType := "cpu"
	
	// Test storing data points
	dataPoint := DataPoint{
		Timestamp: time.Now(),
		Value:     100.5,
		Namespace: namespace,
		PodName:   podName,
		Container: container,
	}
	
	err := store.StoreHistoricalData(namespace, podName, container, resourceType, dataPoint)
	assert.NoError(t, err)
	
	// Test retrieving data
	since := time.Now().Add(-1 * time.Hour)
	historicalData, err := store.GetHistoricalData(namespace, podName, container, resourceType, since)
	require.NoError(t, err)
	assert.Equal(t, resourceType, historicalData.ResourceType)
	assert.Len(t, historicalData.DataPoints, 1)
	assert.Equal(t, 100.5, historicalData.DataPoints[0].Value)
	
	// Test storing predictions
	prediction := ResourcePrediction{
		Value:      120.0,
		Confidence: 0.8,
		Horizon:    10 * time.Minute,
		Timestamp:  time.Now(),
		Method:     PredictionMethodLinearRegression,
	}
	
	err = store.StorePrediction(namespace, podName, container, resourceType, prediction)
	assert.NoError(t, err)
	
	// Test retrieving predictions
	predictions, err := store.GetPredictions(namespace, podName, container, resourceType, since)
	require.NoError(t, err)
	assert.Len(t, predictions, 1)
	assert.Equal(t, 120.0, predictions[0].Value)
	
	// Test stats
	stats := store.GetStats()
	assert.Equal(t, 1, stats["totalResources"])
	assert.Equal(t, 1, stats["totalDataPoints"])
	assert.Equal(t, 1, stats["totalPredictions"])
}

func TestPredictionEngine(t *testing.T) {
	config := DefaultConfig()
	config.EnabledMethods = []PredictionMethod{
		PredictionMethodLinearRegression,
		PredictionMethodSimpleMovingAverage,
	}
	config.MinDataPoints = 3
	config.ConfidenceThreshold = 0.1 // Low threshold for testing
	
	engine, err := NewEngine(config)
	require.NoError(t, err)
	
	ctx := context.Background()
	err = engine.Start(ctx)
	require.NoError(t, err)
	defer engine.Stop()
	
	namespace := "test-ns"
	podName := "test-pod"
	container := "test-container"
	resourceType := "cpu"
	
	// Store some historical data
	baseTime := time.Now().Add(-30 * time.Minute)
	for i := 0; i < 5; i++ {
		timestamp := baseTime.Add(time.Duration(i*5) * time.Minute)
		value := float64(100 + i*10)
		err := engine.StoreDataPoint(namespace, podName, container, resourceType, value, timestamp)
		require.NoError(t, err)
	}
	
	// Test prediction request
	request := PredictionRequest{
		Namespace:    namespace,
		PodName:      podName,
		Container:    container,
		ResourceType: resourceType,
		Horizons:     []time.Duration{5 * time.Minute, 15 * time.Minute},
	}
	
	response, err := engine.Predict(ctx, request)
	require.NoError(t, err)
	assert.Equal(t, 5, response.DataPoints)
	assert.True(t, len(response.Predictions) > 0)
	
	// Test getting best prediction
	bestPred, err := engine.GetBestPrediction(ctx, namespace, podName, container, resourceType, 5*time.Minute)
	require.NoError(t, err)
	assert.NotNil(t, bestPred)
	assert.Equal(t, 5*time.Minute, bestPred.Horizon)
	
	// Test stats
	stats := engine.GetStats()
	assert.True(t, stats["isRunning"].(bool))
	assert.Equal(t, 2, stats["predictors"])
}

func TestDataValidation(t *testing.T) {
	predictor := NewLinearRegressionPredictor()
	
	// Test with insufficient data
	historicalData := HistoricalData{
		ResourceType: "cpu",
		DataPoints:   []DataPoint{{Timestamp: time.Now(), Value: 100}},
	}
	
	err := predictor.ValidateData(historicalData)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient data points")
	
	// Test with invalid timestamp
	historicalData.DataPoints = []DataPoint{
		{Timestamp: time.Time{}, Value: 100},
		{Timestamp: time.Now(), Value: 110},
		{Timestamp: time.Now(), Value: 120},
		{Timestamp: time.Now(), Value: 130},
		{Timestamp: time.Now(), Value: 140},
	}
	
	err = predictor.ValidateData(historicalData)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid timestamp")
	
	// Test with invalid value
	historicalData.DataPoints = []DataPoint{
		{Timestamp: time.Now(), Value: 100},
		{Timestamp: time.Now(), Value: 110},
		{Timestamp: time.Now(), Value: 120},
		{Timestamp: time.Now(), Value: 130},
		{Timestamp: time.Now(), Value: math.Inf(1)}, // Infinity
	}
	
	err = predictor.ValidateData(historicalData)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid value")
}
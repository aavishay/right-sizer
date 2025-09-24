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
	"fmt"
	"sort"
	"sync"
	"time"
)

// Engine coordinates multiple prediction algorithms and manages the prediction pipeline
type Engine struct {
	predictors map[PredictionMethod]Predictor
	store      PredictionStore
	config     *Config
	mutex      sync.RWMutex
	isRunning  bool
	stopChan   chan struct{}
	waitGroup  sync.WaitGroup
}

// NewEngine creates a new prediction engine
func NewEngine(config *Config) (*Engine, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Create store based on configuration
	var store PredictionStore
	switch config.StorageDriver {
	case "memory":
		store = NewMemoryStore(config)
	default:
		return nil, fmt.Errorf("unsupported storage driver: %s", config.StorageDriver)
	}

	engine := &Engine{
		predictors: make(map[PredictionMethod]Predictor),
		store:      store,
		config:     config,
		stopChan:   make(chan struct{}),
	}

	// Initialize enabled predictors
	for _, method := range config.EnabledMethods {
		if err := engine.addPredictor(method); err != nil {
			return nil, fmt.Errorf("failed to add predictor %s: %w", method, err)
		}
	}

	return engine, nil
}

// addPredictor adds a predictor for the specified method
func (e *Engine) addPredictor(method PredictionMethod) error {
	var predictor Predictor

	switch method {
	case PredictionMethodLinearRegression:
		predictor = NewLinearRegressionPredictor()
	case PredictionMethodExponentialSmoothing:
		predictor = NewExponentialSmoothingPredictor()
	case PredictionMethodSimpleMovingAverage:
		predictor = NewSimpleMovingAveragePredictor(5) // Default window size
	default:
		return fmt.Errorf("unsupported prediction method: %s", method)
	}

	e.predictors[method] = predictor
	return nil
}

// Start starts the prediction engine background processes
func (e *Engine) Start(ctx context.Context) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if e.isRunning {
		return fmt.Errorf("engine is already running")
	}

	e.isRunning = true

	// Start cleanup routine
	e.waitGroup.Add(1)
	go e.cleanupRoutine(ctx)

	return nil
}

// Stop stops the prediction engine
func (e *Engine) Stop() error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if !e.isRunning {
		return nil
	}

	e.isRunning = false
	close(e.stopChan)
	e.waitGroup.Wait()

	return nil
}

// StoreDataPoint stores a new historical data point
func (e *Engine) StoreDataPoint(namespace, podName, container, resourceType string, value float64, timestamp time.Time) error {
	dataPoint := DataPoint{
		Timestamp: timestamp,
		Value:     value,
		Namespace: namespace,
		PodName:   podName,
		Container: container,
	}

	return e.store.StoreHistoricalData(namespace, podName, container, resourceType, dataPoint)
}

// Predict generates predictions for a resource using all enabled predictors
func (e *Engine) Predict(ctx context.Context, request PredictionRequest) (*PredictionResponse, error) {
	// Use default horizons if none specified
	horizons := request.Horizons
	if len(horizons) == 0 {
		horizons = e.config.DefaultHorizons
	}

	// Use all enabled methods if none specified
	methods := request.Methods
	if len(methods) == 0 {
		methods = e.config.EnabledMethods
	}

	// Get historical data
	since := time.Now().Add(-e.config.HistoricalDataRetention)
	historicalData, err := e.store.GetHistoricalData(request.Namespace, request.PodName, request.Container, request.ResourceType, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get historical data: %w", err)
	}

	// Check if we have enough data
	if len(historicalData.DataPoints) < e.config.MinDataPoints {
		return &PredictionResponse{
			Request:     request,
			Predictions: []ResourcePrediction{},
			Timestamp:   time.Now(),
			DataPoints:  len(historicalData.DataPoints),
		}, nil
	}

	// Generate predictions using multiple methods
	var allPredictions []ResourcePrediction
	var predictionErrors []error

	// Use context with timeout for prediction calculations
	predCtx, cancel := context.WithTimeout(ctx, e.config.PredictionTimeout)
	defer cancel()

	// Create a channel to collect predictions
	predictionChan := make(chan []ResourcePrediction, len(methods))
	errorChan := make(chan error, len(methods))

	// Start prediction goroutines
	var wg sync.WaitGroup
	for _, method := range methods {
		predictor, exists := e.predictors[method]
		if !exists {
			continue
		}

		wg.Add(1)
		go func(p Predictor, m PredictionMethod) {
			defer wg.Done()

			// Check if context was cancelled
			select {
			case <-predCtx.Done():
				errorChan <- fmt.Errorf("prediction cancelled for method %s", m)
				return
			default:
			}

			// Validate data for this predictor
			if err := p.ValidateData(historicalData); err != nil {
				errorChan <- fmt.Errorf("data validation failed for %s: %w", m, err)
				return
			}

			// Generate predictions
			predictions, err := p.Predict(historicalData, horizons)
			if err != nil {
				errorChan <- fmt.Errorf("prediction failed for %s: %w", m, err)
				return
			}

			predictionChan <- predictions
		}(predictor, method)
	}

	// Close channels when all goroutines complete
	go func() {
		wg.Wait()
		close(predictionChan)
		close(errorChan)
	}()

	// Collect predictions and errors
	for {
		select {
		case predictions := <-predictionChan:
			if predictions != nil {
				allPredictions = append(allPredictions, predictions...)
			}
		case err := <-errorChan:
			if err != nil {
				predictionErrors = append(predictionErrors, err)
			}
		case <-predCtx.Done():
			return nil, fmt.Errorf("prediction timeout exceeded")
		default:
			// Check if both channels are closed
			if predictionChan == nil && errorChan == nil {
				goto collectResults
			}
			// Continue collecting
		}

		// Check if channels are closed
		select {
		case _, ok := <-predictionChan:
			if !ok {
				predictionChan = nil
			}
		default:
		}
		select {
		case _, ok := <-errorChan:
			if !ok {
				errorChan = nil
			}
		default:
		}

		if predictionChan == nil && errorChan == nil {
			break
		}
	}

collectResults:
	// Filter predictions by confidence threshold
	var filteredPredictions []ResourcePrediction
	for _, pred := range allPredictions {
		if pred.Confidence >= e.config.ConfidenceThreshold {
			filteredPredictions = append(filteredPredictions, pred)
		}
	}

	// Sort predictions by confidence (highest first) and then by horizon
	sort.Slice(filteredPredictions, func(i, j int) bool {
		if filteredPredictions[i].Confidence != filteredPredictions[j].Confidence {
			return filteredPredictions[i].Confidence > filteredPredictions[j].Confidence
		}
		return filteredPredictions[i].Horizon < filteredPredictions[j].Horizon
	})

	// Store predictions
	for _, prediction := range filteredPredictions {
		if err := e.store.StorePrediction(request.Namespace, request.PodName, request.Container, request.ResourceType, prediction); err != nil {
			// Log error but don't fail the entire operation
			fmt.Printf("Failed to store prediction: %v\n", err)
		}
	}

	response := &PredictionResponse{
		Request:     request,
		Predictions: filteredPredictions,
		Timestamp:   time.Now(),
		DataPoints:  len(historicalData.DataPoints),
	}

	return response, nil
}

// GetBestPrediction returns the best prediction for a specific horizon
func (e *Engine) GetBestPrediction(ctx context.Context, namespace, podName, container, resourceType string, horizon time.Duration) (*ResourcePrediction, error) {
	request := PredictionRequest{
		Namespace:    namespace,
		PodName:      podName,
		Container:    container,
		ResourceType: resourceType,
		Horizons:     []time.Duration{horizon},
		Methods:      e.config.EnabledMethods,
	}

	response, err := e.Predict(ctx, request)
	if err != nil {
		return nil, err
	}

	if len(response.Predictions) == 0 {
		return nil, fmt.Errorf("no predictions available")
	}

	// Return the prediction with highest confidence for the requested horizon
	var bestPrediction *ResourcePrediction
	for _, pred := range response.Predictions {
		if pred.Horizon == horizon {
			if bestPrediction == nil || pred.Confidence > bestPrediction.Confidence {
				bestPrediction = &pred
			}
		}
	}

	if bestPrediction == nil {
		return nil, fmt.Errorf("no prediction found for horizon %v", horizon)
	}

	return bestPrediction, nil
}

// GetHistoricalData retrieves historical data for a resource
func (e *Engine) GetHistoricalData(namespace, podName, container, resourceType string, since time.Time) (HistoricalData, error) {
	return e.store.GetHistoricalData(namespace, podName, container, resourceType, since)
}

// GetStoredPredictions retrieves previously stored predictions
func (e *Engine) GetStoredPredictions(namespace, podName, container, resourceType string, since time.Time) ([]ResourcePrediction, error) {
	return e.store.GetPredictions(namespace, podName, container, resourceType, since)
}

// cleanupRoutine runs periodic cleanup of old data
func (e *Engine) cleanupRoutine(ctx context.Context) {
	defer e.waitGroup.Done()

	ticker := time.NewTicker(time.Hour) // Cleanup every hour
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopChan:
			return
		case <-ticker.C:
			// Perform cleanup
			cutoff := time.Now().Add(-e.config.HistoricalDataRetention)
			if err := e.store.CleanupOldData(cutoff); err != nil {
				fmt.Printf("Cleanup error: %v\n", err)
			}
		}
	}
}

// GetStats returns statistics about the prediction engine
func (e *Engine) GetStats() map[string]interface{} {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	stats := map[string]interface{}{
		"isRunning":  e.isRunning,
		"predictors": len(e.predictors),
		"methods":    make([]string, 0, len(e.predictors)),
		"config":     e.config,
	}

	for method := range e.predictors {
		stats["methods"] = append(stats["methods"].([]string), string(method))
	}

	// Add store stats if available
	if memoryStore, ok := e.store.(*MemoryStore); ok {
		stats["store"] = memoryStore.GetStats()
	}

	return stats
}

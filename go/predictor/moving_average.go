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
	"time"
)

// SimpleMovingAveragePredictor implements simple moving average for resource prediction
type SimpleMovingAveragePredictor struct {
	minDataPoints int
	windowSize    int
}

// NewSimpleMovingAveragePredictor creates a new simple moving average predictor
func NewSimpleMovingAveragePredictor(windowSize int) *SimpleMovingAveragePredictor {
	if windowSize < 3 {
		windowSize = 5 // Default window size
	}
	return &SimpleMovingAveragePredictor{
		minDataPoints: 3,
		windowSize:    windowSize,
	}
}

// GetMethod returns the prediction method
func (p *SimpleMovingAveragePredictor) GetMethod() PredictionMethod {
	return PredictionMethodSimpleMovingAverage
}

// GetMinDataPoints returns the minimum number of data points required
func (p *SimpleMovingAveragePredictor) GetMinDataPoints() int {
	return p.minDataPoints
}

// ValidateData checks if the historical data is suitable for moving average
func (p *SimpleMovingAveragePredictor) ValidateData(data HistoricalData) error {
	if len(data.DataPoints) < p.minDataPoints {
		return fmt.Errorf("insufficient data points: need at least %d, got %d", p.minDataPoints, len(data.DataPoints))
	}
	
	// Check for valid timestamps and values
	for i, dp := range data.DataPoints {
		if dp.Timestamp.IsZero() {
			return fmt.Errorf("invalid timestamp at index %d", i)
		}
		if math.IsNaN(dp.Value) || math.IsInf(dp.Value, 0) {
			return fmt.Errorf("invalid value at index %d: %f", i, dp.Value)
		}
	}
	
	return nil
}

// Predict generates predictions using simple moving average
func (p *SimpleMovingAveragePredictor) Predict(data HistoricalData, horizons []time.Duration) ([]ResourcePrediction, error) {
	if err := p.ValidateData(data); err != nil {
		return nil, fmt.Errorf("data validation failed: %w", err)
	}
	
	// Sort data points by timestamp
	sortedData := make([]DataPoint, len(data.DataPoints))
	copy(sortedData, data.DataPoints)
	sort.Slice(sortedData, func(i, j int) bool {
		return sortedData[i].Timestamp.Before(sortedData[j].Timestamp)
	})
	
	// Calculate moving average and variance
	movingAverage, variance := p.calculateMovingAverage(sortedData)
	
	// Calculate confidence based on variance and data availability
	confidence := p.calculateConfidence(variance, movingAverage, len(sortedData))
	
	// Generate predictions for each horizon
	predictions := make([]ResourcePrediction, 0, len(horizons))
	
	for _, horizon := range horizons {
		// For simple moving average, the prediction is constant (the current average)
		predictedValue := movingAverage
		
		// Ensure prediction is non-negative for resource values
		if predictedValue < 0 {
			predictedValue = 0
		}
		
		// Calculate confidence interval based on variance
		confidenceInterval := p.calculateConfidenceInterval(predictedValue, variance, len(sortedData))
		
		prediction := ResourcePrediction{
			Value:      predictedValue,
			Confidence: confidence,
			Horizon:    horizon,
			Timestamp:  time.Now(),
			Method:     PredictionMethodSimpleMovingAverage,
			ConfidenceInterval: confidenceInterval,
			Metadata: map[string]interface{}{
				"movingAverage": movingAverage,
				"variance":      variance,
				"windowSize":    p.windowSize,
				"dataPoints":    len(sortedData),
				"actualWindow":  min(p.windowSize, len(sortedData)),
			},
		}
		
		predictions = append(predictions, prediction)
	}
	
	return predictions, nil
}

// calculateMovingAverage calculates the moving average and variance of the most recent data points
func (p *SimpleMovingAveragePredictor) calculateMovingAverage(dataPoints []DataPoint) (average, variance float64) {
	if len(dataPoints) == 0 {
		return 0, 0
	}
	
	// Use the most recent windowSize points, or all points if we have fewer
	windowSize := min(p.windowSize, len(dataPoints))
	startIndex := len(dataPoints) - windowSize
	
	var sum float64
	for i := startIndex; i < len(dataPoints); i++ {
		sum += dataPoints[i].Value
	}
	average = sum / float64(windowSize)
	
	// Calculate variance
	var sumSquaredDiffs float64
	for i := startIndex; i < len(dataPoints); i++ {
		diff := dataPoints[i].Value - average
		sumSquaredDiffs += diff * diff
	}
	
	if windowSize > 1 {
		variance = sumSquaredDiffs / float64(windowSize-1)
	} else {
		variance = 0
	}
	
	return average, variance
}

// calculateConfidence converts variance to a confidence score
func (p *SimpleMovingAveragePredictor) calculateConfidence(variance, average float64, dataPoints int) float64 {
	// Base confidence from variance (lower variance = higher confidence)
	var baseConfidence float64
	if average > 0 {
		// Calculate coefficient of variation (stddev / mean)
		stddev := math.Sqrt(variance)
		cv := stddev / average
		// Convert to confidence (1 - cv), clamped between 0 and 1
		baseConfidence = math.Max(0, math.Min(1, 1-cv))
	} else {
		// If average is 0, use moderate confidence
		baseConfidence = 0.6
	}
	
	// Adjust confidence based on number of data points
	dataPointFactor := math.Min(1.0, float64(dataPoints)/10.0) // Saturate at 10 points
	
	// Adjust confidence based on window size vs available data
	windowFactor := float64(min(p.windowSize, dataPoints)) / float64(p.windowSize)
	
	// Combined confidence score
	confidence := baseConfidence * (0.3 + 0.4*dataPointFactor + 0.3*windowFactor)
	
	// Ensure confidence is between 0 and 1
	return math.Max(0, math.Min(1, confidence))
}

// calculateConfidenceInterval calculates the confidence interval for a prediction
func (p *SimpleMovingAveragePredictor) calculateConfidenceInterval(predictedValue, variance float64, dataPoints int) *ConfidenceInterval {
	if variance == 0 || dataPoints <= 1 {
		// Return narrow interval around the predicted value
		margin := predictedValue * 0.05 // 5% margin
		return &ConfidenceInterval{
			Lower:      math.Max(0, predictedValue-margin),
			Upper:      predictedValue + margin,
			Percentage: 95.0,
		}
	}
	
	// Use standard error for confidence interval
	standardError := math.Sqrt(variance / float64(dataPoints))
	
	// Use t-distribution for small samples, normal for large samples
	var tValue float64
	if dataPoints < 30 {
		tValue = 2.0 // Approximation for 95% confidence
	} else {
		tValue = 1.96 // Normal distribution
	}
	
	margin := tValue * standardError
	
	return &ConfidenceInterval{
		Lower:      math.Max(0, predictedValue-margin), // Resource values can't be negative
		Upper:      predictedValue + margin,
		Percentage: 95.0,
	}
}

// SetWindowSize sets the window size for the moving average
func (p *SimpleMovingAveragePredictor) SetWindowSize(size int) error {
	if size < 1 {
		return fmt.Errorf("window size must be positive, got %d", size)
	}
	p.windowSize = size
	return nil
}

// GetWindowSize returns the current window size
func (p *SimpleMovingAveragePredictor) GetWindowSize() int {
	return p.windowSize
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
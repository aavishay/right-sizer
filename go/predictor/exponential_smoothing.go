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

// ExponentialSmoothingPredictor implements exponential smoothing for resource prediction
type ExponentialSmoothingPredictor struct {
	minDataPoints int
	alpha         float64 // Smoothing parameter (0 < alpha <= 1)
	beta          float64 // Trend smoothing parameter (0 <= beta <= 1)
}

// NewExponentialSmoothingPredictor creates a new exponential smoothing predictor
func NewExponentialSmoothingPredictor() *ExponentialSmoothingPredictor {
	return &ExponentialSmoothingPredictor{
		minDataPoints: 3,   // Minimum 3 data points
		alpha:         0.3, // Default smoothing parameter
		beta:          0.1, // Default trend smoothing parameter
	}
}

// GetMethod returns the prediction method
func (p *ExponentialSmoothingPredictor) GetMethod() PredictionMethod {
	return PredictionMethodExponentialSmoothing
}

// GetMinDataPoints returns the minimum number of data points required
func (p *ExponentialSmoothingPredictor) GetMinDataPoints() int {
	return p.minDataPoints
}

// ValidateData checks if the historical data is suitable for exponential smoothing
func (p *ExponentialSmoothingPredictor) ValidateData(data HistoricalData) error {
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

// Predict generates predictions using double exponential smoothing (Holt's method)
func (p *ExponentialSmoothingPredictor) Predict(data HistoricalData, horizons []time.Duration) ([]ResourcePrediction, error) {
	if err := p.ValidateData(data); err != nil {
		return nil, fmt.Errorf("data validation failed: %w", err)
	}

	// Sort data points by timestamp
	sortedData := make([]DataPoint, len(data.DataPoints))
	copy(sortedData, data.DataPoints)
	sort.Slice(sortedData, func(i, j int) bool {
		return sortedData[i].Timestamp.Before(sortedData[j].Timestamp)
	})

	// Calculate smoothed values and trend using Holt's method
	level, trend, meanAbsError := p.calculateHoltSmoothing(sortedData)

	// Calculate confidence based on prediction accuracy
	confidence := p.calculateConfidence(meanAbsError, level, len(sortedData))

	// Calculate time interval between data points for prediction scaling
	avgInterval := p.calculateAverageInterval(sortedData)

	// Generate predictions for each horizon
	predictions := make([]ResourcePrediction, 0, len(horizons))

	for _, horizon := range horizons {
		// Calculate number of steps ahead based on average interval
		steps := float64(horizon) / float64(avgInterval)

		// Holt's forecasting formula: F(t+h) = Level + h * Trend
		predictedValue := level + steps*trend

		// Ensure prediction is non-negative for resource values
		if predictedValue < 0 {
			predictedValue = 0
		}

		// Calculate confidence interval based on prediction error
		confidenceInterval := p.calculateConfidenceInterval(predictedValue, meanAbsError, steps)

		prediction := ResourcePrediction{
			Value:              predictedValue,
			Confidence:         confidence,
			Horizon:            horizon,
			Timestamp:          time.Now(),
			Method:             PredictionMethodExponentialSmoothing,
			ConfidenceInterval: confidenceInterval,
			Metadata: map[string]interface{}{
				"level":        level,
				"trend":        trend,
				"alpha":        p.alpha,
				"beta":         p.beta,
				"meanAbsError": meanAbsError,
				"steps":        steps,
				"avgInterval":  avgInterval.String(),
				"dataPoints":   len(sortedData),
			},
		}

		predictions = append(predictions, prediction)
	}

	return predictions, nil
}

// calculateHoltSmoothing implements Holt's double exponential smoothing
func (p *ExponentialSmoothingPredictor) calculateHoltSmoothing(dataPoints []DataPoint) (level, trend, meanAbsError float64) {
	if len(dataPoints) < 2 {
		if len(dataPoints) == 1 {
			return dataPoints[0].Value, 0, 0
		}
		return 0, 0, 0
	}

	// Initialize level and trend
	level = dataPoints[0].Value
	if len(dataPoints) > 1 {
		trend = dataPoints[1].Value - dataPoints[0].Value
	}

	var sumAbsError float64
	errorCount := 0

	// Apply Holt's method
	for i := 1; i < len(dataPoints); i++ {
		actualValue := dataPoints[i].Value

		// Calculate forecast for this point (before updating level and trend)
		forecast := level + trend

		// Calculate absolute error for this forecast
		absError := math.Abs(actualValue - forecast)
		sumAbsError += absError
		errorCount++

		// Update level using exponential smoothing
		previousLevel := level
		level = p.alpha*actualValue + (1-p.alpha)*(level+trend)

		// Update trend using exponential smoothing
		trend = p.beta*(level-previousLevel) + (1-p.beta)*trend
	}

	// Calculate mean absolute error
	if errorCount > 0 {
		meanAbsError = sumAbsError / float64(errorCount)
	}

	return level, trend, meanAbsError
}

// calculateConfidence converts prediction error to a confidence score
func (p *ExponentialSmoothingPredictor) calculateConfidence(meanAbsError, currentLevel float64, dataPoints int) float64 {
	// Base confidence from prediction accuracy
	var baseConfidence float64
	if currentLevel > 0 {
		// Calculate relative error as a percentage
		relativeError := meanAbsError / currentLevel
		// Convert to confidence (1 - error), clamped between 0 and 1
		baseConfidence = math.Max(0, math.Min(1, 1-relativeError))
	} else {
		// If current level is 0, use a moderate confidence
		baseConfidence = 0.5
	}

	// Adjust confidence based on number of data points
	dataPointFactor := math.Min(1.0, float64(dataPoints)/15.0) // Saturate at 15 points

	// Combined confidence score
	confidence := baseConfidence * (0.4 + 0.6*dataPointFactor)

	// Ensure confidence is between 0 and 1
	return math.Max(0, math.Min(1, confidence))
}

// calculateAverageInterval calculates the average time interval between data points
func (p *ExponentialSmoothingPredictor) calculateAverageInterval(dataPoints []DataPoint) time.Duration {
	if len(dataPoints) < 2 {
		return time.Minute // Default to 1 minute
	}

	var totalDuration time.Duration
	for i := 1; i < len(dataPoints); i++ {
		interval := dataPoints[i].Timestamp.Sub(dataPoints[i-1].Timestamp)
		totalDuration += interval
	}

	avgInterval := totalDuration / time.Duration(len(dataPoints)-1)

	// Ensure we have a reasonable minimum interval
	if avgInterval < time.Second {
		avgInterval = time.Minute
	}

	return avgInterval
}

// calculateConfidenceInterval calculates the confidence interval for a prediction
func (p *ExponentialSmoothingPredictor) calculateConfidenceInterval(predictedValue, meanAbsError, steps float64) *ConfidenceInterval {
	// Error tends to increase with the number of steps ahead
	// Use a simple model where error grows with sqrt(steps)
	adjustedError := meanAbsError * (1 + 0.1*math.Sqrt(steps))

	// For exponential smoothing, use approximately 1.96 * adjusted error for 95% confidence
	margin := 1.96 * adjustedError

	return &ConfidenceInterval{
		Lower:      math.Max(0, predictedValue-margin), // Resource values can't be negative
		Upper:      predictedValue + margin,
		Percentage: 95.0,
	}
}

// SetAlpha sets the level smoothing parameter
func (p *ExponentialSmoothingPredictor) SetAlpha(alpha float64) error {
	if alpha <= 0 || alpha > 1 {
		return fmt.Errorf("alpha must be between 0 and 1, got %f", alpha)
	}
	p.alpha = alpha
	return nil
}

// SetBeta sets the trend smoothing parameter
func (p *ExponentialSmoothingPredictor) SetBeta(beta float64) error {
	if beta < 0 || beta > 1 {
		return fmt.Errorf("beta must be between 0 and 1, got %f", beta)
	}
	p.beta = beta
	return nil
}

// GetAlpha returns the current alpha parameter
func (p *ExponentialSmoothingPredictor) GetAlpha() float64 {
	return p.alpha
}

// GetBeta returns the current beta parameter
func (p *ExponentialSmoothingPredictor) GetBeta() float64 {
	return p.beta
}

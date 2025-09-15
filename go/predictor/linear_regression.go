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

// LinearRegressionPredictor implements linear regression for resource prediction
type LinearRegressionPredictor struct {
	minDataPoints int
}

// NewLinearRegressionPredictor creates a new linear regression predictor
func NewLinearRegressionPredictor() *LinearRegressionPredictor {
	return &LinearRegressionPredictor{
		minDataPoints: 5, // Minimum 5 data points for linear regression
	}
}

// GetMethod returns the prediction method
func (p *LinearRegressionPredictor) GetMethod() PredictionMethod {
	return PredictionMethodLinearRegression
}

// GetMinDataPoints returns the minimum number of data points required
func (p *LinearRegressionPredictor) GetMinDataPoints() int {
	return p.minDataPoints
}

// ValidateData checks if the historical data is suitable for linear regression
func (p *LinearRegressionPredictor) ValidateData(data HistoricalData) error {
	if len(data.DataPoints) < p.minDataPoints {
		return fmt.Errorf("insufficient data points: need at least %d, got %d", p.minDataPoints, len(data.DataPoints))
	}
	
	// Check for valid timestamps
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

// Predict generates predictions using linear regression
func (p *LinearRegressionPredictor) Predict(data HistoricalData, horizons []time.Duration) ([]ResourcePrediction, error) {
	if err := p.ValidateData(data); err != nil {
		return nil, fmt.Errorf("data validation failed: %w", err)
	}
	
	// Sort data points by timestamp
	sortedData := make([]DataPoint, len(data.DataPoints))
	copy(sortedData, data.DataPoints)
	sort.Slice(sortedData, func(i, j int) bool {
		return sortedData[i].Timestamp.Before(sortedData[j].Timestamp)
	})
	
	// Perform linear regression
	slope, intercept, r2, err := p.calculateLinearRegression(sortedData)
	if err != nil {
		return nil, fmt.Errorf("linear regression calculation failed: %w", err)
	}
	
	// Calculate prediction accuracy/confidence based on R²
	confidence := p.calculateConfidence(r2, len(sortedData))
	
	// Calculate standard error for confidence intervals
	stdError := p.calculateStandardError(sortedData, slope, intercept)
	
	// Generate predictions for each horizon
	predictions := make([]ResourcePrediction, 0, len(horizons))
	baseTime := sortedData[len(sortedData)-1].Timestamp
	
	for _, horizon := range horizons {
		futureTime := baseTime.Add(horizon)
		
		// Convert time to x-coordinate (seconds since base time)
		x := float64(futureTime.Sub(sortedData[0].Timestamp).Seconds())
		
		// Calculate predicted value
		predictedValue := slope*x + intercept
		
		// Ensure prediction is non-negative for resource values
		if predictedValue < 0 {
			predictedValue = 0
		}
		
		// Calculate confidence interval (95% confidence)
		confidenceInterval := p.calculateConfidenceInterval(predictedValue, stdError, len(sortedData), 0.95)
		
		prediction := ResourcePrediction{
			Value:      predictedValue,
			Confidence: confidence,
			Horizon:    horizon,
			Timestamp:  time.Now(),
			Method:     PredictionMethodLinearRegression,
			ConfidenceInterval: confidenceInterval,
			Metadata: map[string]interface{}{
				"slope":        slope,
				"intercept":    intercept,
				"r2":          r2,
				"stdError":    stdError,
				"dataPoints":  len(sortedData),
				"timeRange":   futureTime.Sub(sortedData[0].Timestamp).String(),
			},
		}
		
		predictions = append(predictions, prediction)
	}
	
	return predictions, nil
}

// calculateLinearRegression performs least squares linear regression
func (p *LinearRegressionPredictor) calculateLinearRegression(dataPoints []DataPoint) (slope, intercept, r2 float64, err error) {
	if len(dataPoints) < 2 {
		return 0, 0, 0, fmt.Errorf("need at least 2 data points for regression")
	}
	
	n := float64(len(dataPoints))
	baseTime := dataPoints[0].Timestamp
	
	// Convert timestamps to x-coordinates (seconds since base time)
	var sumX, sumY, sumXY, sumX2, sumY2 float64
	
	for _, dp := range dataPoints {
		x := float64(dp.Timestamp.Sub(baseTime).Seconds())
		y := dp.Value
		
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
		sumY2 += y * y
	}
	
	// Calculate slope and intercept using least squares formulas
	denominator := n*sumX2 - sumX*sumX
	if math.Abs(denominator) < 1e-10 {
		// Handle case where all x values are the same (no time variation)
		// In this case, use the mean value as the prediction
		meanY := sumY / n
		return 0, meanY, 0, nil
	}
	
	slope = (n*sumXY - sumX*sumY) / denominator
	intercept = (sumY - slope*sumX) / n
	
	// Calculate R² (coefficient of determination)
	meanY := sumY / n
	var ssRes, ssTot float64
	
	for _, dp := range dataPoints {
		x := float64(dp.Timestamp.Sub(baseTime).Seconds())
		y := dp.Value
		predicted := slope*x + intercept
		
		ssRes += math.Pow(y-predicted, 2)
		ssTot += math.Pow(y-meanY, 2)
	}
	
	if ssTot > 0 {
		r2 = 1 - (ssRes / ssTot)
	} else {
		r2 = 0
	}
	
	// Ensure R² is between 0 and 1
	if r2 < 0 {
		r2 = 0
	}
	if r2 > 1 {
		r2 = 1
	}
	
	return slope, intercept, r2, nil
}

// calculateConfidence converts R² to a confidence score
func (p *LinearRegressionPredictor) calculateConfidence(r2 float64, dataPoints int) float64 {
	// Base confidence from R²
	baseConfidence := r2
	
	// Adjust confidence based on number of data points
	// More data points increase confidence
	dataPointFactor := math.Min(1.0, float64(dataPoints)/20.0) // Saturate at 20 points
	
	// Combined confidence score
	confidence := baseConfidence * (0.5 + 0.5*dataPointFactor)
	
	// Ensure confidence is between 0 and 1
	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}
	
	return confidence
}

// calculateStandardError calculates the standard error of the regression
func (p *LinearRegressionPredictor) calculateStandardError(dataPoints []DataPoint, slope, intercept float64) float64 {
	if len(dataPoints) <= 2 {
		return 0
	}
	
	baseTime := dataPoints[0].Timestamp
	var sumSquaredErrors float64
	
	for _, dp := range dataPoints {
		x := float64(dp.Timestamp.Sub(baseTime).Seconds())
		predicted := slope*x + intercept
		error := dp.Value - predicted
		sumSquaredErrors += error * error
	}
	
	// Standard error = sqrt(sum of squared errors / (n - 2))
	stdError := math.Sqrt(sumSquaredErrors / float64(len(dataPoints)-2))
	return stdError
}

// calculateConfidenceInterval calculates the confidence interval for a prediction
func (p *LinearRegressionPredictor) calculateConfidenceInterval(predictedValue, stdError float64, n int, confidenceLevel float64) *ConfidenceInterval {
	if stdError == 0 || n <= 2 {
		// Return narrow interval around the predicted value
		margin := predictedValue * 0.1 // 10% margin
		return &ConfidenceInterval{
			Lower:      math.Max(0, predictedValue-margin),
			Upper:      predictedValue + margin,
			Percentage: confidenceLevel * 100,
		}
	}
	
	// Use t-distribution for small samples
	// For simplicity, use approximation for 95% confidence: t ≈ 2 for reasonable sample sizes
	tValue := 2.0
	if n > 30 {
		tValue = 1.96 // Use normal distribution for larger samples
	}
	
	margin := tValue * stdError
	
	return &ConfidenceInterval{
		Lower:      math.Max(0, predictedValue-margin), // Resource values can't be negative
		Upper:      predictedValue + margin,
		Percentage: confidenceLevel * 100,
	}
}
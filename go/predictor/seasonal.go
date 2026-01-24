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
	"right-sizer/memstore"
	"sort"
	"time"
)

// SeasonalPredictor uses historical patterns (daily/weekly) combined with trend analysis
// to make predictions that account for recurring patterns in workload behavior
type SeasonalPredictor struct {
	memstore *memstore.MemoryStore
	minDays  int // Minimum days of data required for seasonal analysis
}

// NewSeasonalPredictor creates a new seasonal prediction algorithm
func NewSeasonalPredictor(store *memstore.MemoryStore) *SeasonalPredictor {
	return &SeasonalPredictor{
		memstore: store,
		minDays:  3, // Need at least 3 days for basic weekly pattern
	}
}

// Predict generates predictions using seasonal patterns
func (sp *SeasonalPredictor) Predict(data HistoricalData, horizons []time.Duration) ([]ResourcePrediction, error) {
	if err := sp.ValidateData(data); err != nil {
		return nil, err
	}

	predictions := make([]ResourcePrediction, 0, len(horizons))

	// Extract daily and weekly patterns from historical data
	dailyPattern, weeklyPattern := sp.extractSeasonalPatterns(data)
	if dailyPattern == nil || weeklyPattern == nil {
		return nil, fmt.Errorf("insufficient data for seasonal pattern extraction")
	}

	// Get current trend
	trend := sp.getTrend(data)

	now := time.Now()

	for _, horizon := range horizons {
		predictionTime := now.Add(horizon)

		// Get baseline for the target time from patterns
		baseline := sp.getBaselineForTimeWithMaps(predictionTime, dailyPattern, weeklyPattern)

		// Apply trend to baseline
		trendAdjustment := sp.applyTrend(baseline, trend, horizon)

		// Calculate confidence
		confidence := sp.calculateConfidence(data, trend, horizon)

		// Calculate confidence interval (95% bounds)
		stdDev := sp.calculateStdDev(data)
		margin := 1.96 * stdDev // 95% confidence interval

		pred := ResourcePrediction{
			Value:      trendAdjustment,
			Confidence: confidence,
			Horizon:    horizon,
			Timestamp:  now,
			Method:     PredictionMethodSeasonal,
			ConfidenceInterval: &ConfidenceInterval{
				Lower:      math.Max(0, trendAdjustment-margin),
				Upper:      trendAdjustment + margin,
				Percentage: 95,
			},
			Metadata: map[string]interface{}{
				"baseline":           baseline,
				"trend_slope":        trend.Slope,
				"trend_direction":    trend.Direction,
				"seasonal_component": sp.getSeasonalComponentWithMaps(predictionTime, dailyPattern, weeklyPattern),
			},
		}

		predictions = append(predictions, pred)
	}

	return predictions, nil
}

// GetMethod returns the prediction method
func (sp *SeasonalPredictor) GetMethod() PredictionMethod {
	return PredictionMethodSeasonal
}

// GetMinDataPoints returns minimum data points required
func (sp *SeasonalPredictor) GetMinDataPoints() int {
	return 72 // Just 3 days * 24 hours (hourly granularity)
}

// ValidateData checks if data is suitable for seasonal prediction
func (sp *SeasonalPredictor) ValidateData(data HistoricalData) error {
	if len(data.DataPoints) < sp.GetMinDataPoints() {
		return fmt.Errorf("insufficient data points for seasonal prediction: have %d, need %d",
			len(data.DataPoints), sp.GetMinDataPoints())
	}

	if len(data.DataPoints) < 2 {
		return fmt.Errorf("need at least 2 data points for trend analysis")
	}

	return nil
}

// trendInfo holds trend analysis results
type trendInfo struct {
	Slope     float64
	Direction string // "increasing", "decreasing", "stable"
	IsBurst   bool
}

// extractSeasonalPatterns extracts daily and weekly patterns from historical data
func (sp *SeasonalPredictor) extractSeasonalPatterns(data HistoricalData) (map[int]float64, map[string]float64) {
	if len(data.DataPoints) == 0 {
		return nil, nil
	}

	// Sort data points by timestamp
	sortedPoints := make([]DataPoint, len(data.DataPoints))
	copy(sortedPoints, data.DataPoints)
	sort.Slice(sortedPoints, func(i, j int) bool {
		return sortedPoints[i].Timestamp.Before(sortedPoints[j].Timestamp)
	})

	// Extract daily pattern (hour of day -> average value)
	dailyPattern := make(map[int]float64)
	dailyCounts := make(map[int]int)

	// Extract weekly pattern (day of week -> average value)
	weeklyPattern := make(map[string]float64)
	weeklyCounts := make(map[string]int)

	for _, dp := range sortedPoints {
		hour := dp.Timestamp.Hour()
		dayOfWeek := dp.Timestamp.Weekday().String()

		// Accumulate for daily pattern
		dailyPattern[hour] += dp.Value
		dailyCounts[hour]++

		// Accumulate for weekly pattern
		weeklyPattern[dayOfWeek] += dp.Value
		weeklyCounts[dayOfWeek]++
	}

	// Calculate averages
	for h := 0; h < 24; h++ {
		if count, ok := dailyCounts[h]; ok && count > 0 {
			dailyPattern[h] /= float64(count)
		} else {
			// Fill missing hours with overall average
			dailyPattern[h] = data.MaxValue/2 + data.MinValue/2
		}
	}

	for _, day := range []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"} {
		if count, ok := weeklyCounts[day]; ok && count > 0 {
			weeklyPattern[day] /= float64(count)
		} else {
			weeklyPattern[day] = data.MaxValue/2 + data.MinValue/2
		}
	}

	return dailyPattern, weeklyPattern
}

// getBaselineForTimeWithMaps returns the expected baseline for a specific time using the pattern maps
func (sp *SeasonalPredictor) getBaselineForTimeWithMaps(t time.Time, daily map[int]float64, weekly map[string]float64) float64 {
	hour := t.Hour()
	dayOfWeek := t.Weekday().String()

	dailyComponent := daily[hour]
	weeklyComponent := weekly[dayOfWeek]

	// Combine daily and weekly patterns (70% daily, 30% weekly influence)
	return dailyComponent*0.7 + weeklyComponent*0.3
}

// getSeasonalComponentWithMaps returns the seasonal contribution to prediction
func (sp *SeasonalPredictor) getSeasonalComponentWithMaps(t time.Time, daily map[int]float64, weekly map[string]float64) float64 {
	hour := t.Hour()
	dayOfWeek := t.Weekday().String()

	dailyComponent := daily[hour]
	weeklyComponent := weekly[dayOfWeek]

	return dailyComponent*0.7 + weeklyComponent*0.3
}

// getTrend calculates trend from historical data
func (sp *SeasonalPredictor) getTrend(data HistoricalData) trendInfo {
	if len(data.DataPoints) < 2 {
		return trendInfo{Direction: "stable", Slope: 0}
	}

	// Sort by timestamp
	sortedPoints := make([]DataPoint, len(data.DataPoints))
	copy(sortedPoints, data.DataPoints)
	sort.Slice(sortedPoints, func(i, j int) bool {
		return sortedPoints[i].Timestamp.Before(sortedPoints[j].Timestamp)
	})

	// Linear regression for trend
	n := float64(len(sortedPoints))
	sumX := 0.0
	sumY := 0.0
	sumXY := 0.0
	sumX2 := 0.0

	for i, dp := range sortedPoints {
		x := float64(i)
		y := dp.Value

		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// Calculate slope
	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)

	// Determine direction
	direction := "stable"
	if slope > 0.01 {
		direction = "increasing"
	} else if slope < -0.01 {
		direction = "decreasing"
	}

	// Detect burst (rapid spike in last 10% of data)
	isBurst := false
	if len(sortedPoints) > 10 {
		lastIdx := len(sortedPoints) - 1
		lastValue := sortedPoints[lastIdx].Value
		avgPrevious := 0.0
		checkSize := int(math.Max(2, float64(len(sortedPoints))/10))
		for i := 1; i <= checkSize && lastIdx-i >= 0; i++ {
			avgPrevious += sortedPoints[lastIdx-i].Value
		}
		avgPrevious /= float64(checkSize)
		if lastValue > avgPrevious*1.5 {
			isBurst = true
		}
	}

	return trendInfo{
		Slope:     slope,
		Direction: direction,
		IsBurst:   isBurst,
	}
}

// applyTrend adjusts baseline prediction with trend information
func (sp *SeasonalPredictor) applyTrend(baseline float64, trend trendInfo, horizon time.Duration) float64 {
	// Don't apply trend to bursts (they're temporary)
	if trend.IsBurst {
		return baseline
	}

	// Apply trend based on direction
	adjustment := 1.0

	switch trend.Direction {
	case "increasing":
		// For increasing trend, scale up slightly
		adjustment = 1.0 + (trend.Slope * 0.1)
	case "decreasing":
		// For decreasing trend, scale down
		adjustment = math.Max(0.1, 1.0+(trend.Slope*0.1))
	}

	return baseline * adjustment
}

// calculateConfidence calculates confidence in the prediction
func (sp *SeasonalPredictor) calculateConfidence(data HistoricalData, trend trendInfo, horizon time.Duration) float64 {
	// Base confidence based on data recency and volume
	baseConfidence := math.Min(0.95, float64(len(data.DataPoints))/float64(sp.GetMinDataPoints())*0.95)

	// Reduce confidence for longer horizons
	horizonPenalty := math.Min(0.5, horizon.Hours()/24.0*0.1)
	confidence := baseConfidence * (1.0 - horizonPenalty)

	// Reduce confidence if trend is unstable
	if trend.Direction != "stable" {
		confidence *= 0.85
	}

	// Reduce confidence if we detected a burst
	if trend.IsBurst {
		confidence *= 0.7
	}

	return math.Max(0.1, confidence)
}

// calculateStdDev calculates standard deviation of historical values
func (sp *SeasonalPredictor) calculateStdDev(data HistoricalData) float64 {
	if len(data.DataPoints) < 2 {
		return 0
	}

	// Calculate mean
	mean := 0.0
	for _, dp := range data.DataPoints {
		mean += dp.Value
	}
	mean /= float64(len(data.DataPoints))

	// Calculate variance
	variance := 0.0
	for _, dp := range data.DataPoints {
		diff := dp.Value - mean
		variance += diff * diff
	}
	variance /= float64(len(data.DataPoints))

	return math.Sqrt(variance)
}

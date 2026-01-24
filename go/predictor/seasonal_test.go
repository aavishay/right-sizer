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
	"math"
	"right-sizer/memstore"
	"testing"
	"time"
)

func TestSeasonalPredictor_Predict_IncreasingTrend(t *testing.T) {
	store := memstore.NewMemoryStore(7, 10080)
	predictor := NewSeasonalPredictor(store)

	// Generate 5 days of data with clear daily pattern and increasing trend
	// Using minute-granularity (every 10 minutes = 144 points per day)
	baseTime := time.Now().Add(-5 * 24 * time.Hour)
	data := HistoricalData{
		ResourceType: "cpu",
		DataPoints:   make([]DataPoint, 0),
		MinValue:     100,
		MaxValue:     500,
		LastUpdated:  time.Now(),
	}

	// Create data with daily pattern: low at night, high during day
	for day := 0; day < 5; day++ {
		for hour := 0; hour < 24; hour++ {
			for minute := 0; minute < 60; minute += 10 {
				timestamp := baseTime.Add(time.Duration(day*24*60+hour*60+minute) * time.Minute)
				var value float64

				if hour >= 9 && hour <= 17 {
					// Business hours: high load
					value = 300 + float64(hour-9)*10 // Increasing through day
				} else {
					// Off hours: low load
					value = 100
				}

				// Add trend: increase by 2% each day
				value *= math.Pow(1.02, float64(day))

				data.DataPoints = append(data.DataPoints, DataPoint{
					Timestamp: timestamp,
					Value:     value,
					Namespace: "default",
					PodName:   "test-pod",
					Container: "app",
				})
			}
		}
	}

	// Predict for next 24 hours
	horizons := []time.Duration{
		1 * time.Hour,
		6 * time.Hour,
		12 * time.Hour,
		24 * time.Hour,
	}

	predictions, err := predictor.Predict(data, horizons)
	if err != nil {
		t.Fatalf("Predict failed: %v", err)
	}

	if len(predictions) != len(horizons) {
		t.Errorf("Expected %d predictions, got %d", len(horizons), len(predictions))
	}

	for i, pred := range predictions {
		if pred.Method != PredictionMethodSeasonal {
			t.Errorf("Prediction %d: wrong method %s", i, pred.Method)
		}

		if pred.Confidence < 0 || pred.Confidence > 1 {
			t.Errorf("Prediction %d: confidence out of range: %f", i, pred.Confidence)
		}

		if pred.ConfidenceInterval == nil {
			t.Errorf("Prediction %d: missing confidence interval", i)
		} else {
			if pred.ConfidenceInterval.Lower >= pred.ConfidenceInterval.Upper {
				t.Errorf("Prediction %d: invalid CI bounds: [%f, %f]",
					i, pred.ConfidenceInterval.Lower, pred.ConfidenceInterval.Upper)
			}

			if pred.Value < pred.ConfidenceInterval.Lower || pred.Value > pred.ConfidenceInterval.Upper {
				t.Errorf("Prediction %d: value %.2f outside CI [%.2f, %.2f]",
					i, pred.Value, pred.ConfidenceInterval.Lower, pred.ConfidenceInterval.Upper)
			}
		}

		if pred.Confidence < 0.5 {
			t.Logf("Prediction %d: low confidence %.2f", i, pred.Confidence)
		}

		t.Logf("Prediction %d (horizon %v): value=%.2f, confidence=%.2f, trend=%v",
			i, pred.Horizon, pred.Value, pred.Confidence,
			pred.Metadata["trend_direction"])
	}
}

func TestSeasonalPredictor_ValidateData_InsufficientData(t *testing.T) {
	store := memstore.NewMemoryStore(7, 10080)
	predictor := NewSeasonalPredictor(store)

	// Create data with too few points (less than 3 days)
	data := HistoricalData{
		ResourceType: "cpu",
		DataPoints:   make([]DataPoint, 10),
		MinValue:     100,
		MaxValue:     500,
		LastUpdated:  time.Now(),
	}

	err := predictor.ValidateData(data)
	if err == nil {
		t.Error("Expected validation error for insufficient data")
	}
}

func TestSeasonalPredictor_ExtractSeasonalPatterns(t *testing.T) {
	store := memstore.NewMemoryStore(7, 10080)
	predictor := NewSeasonalPredictor(store)

	// Generate 7 days of data with clear weekly pattern
	baseTime := time.Now().Add(-7 * 24 * time.Hour)
	data := HistoricalData{
		ResourceType: "memory",
		DataPoints:   make([]DataPoint, 0),
		MinValue:     200,
		MaxValue:     800,
		LastUpdated:  time.Now(),
	}

	// Create weekly pattern: weekdays high, weekends low
	for day := 0; day < 7; day++ {
		dayOfWeek := baseTime.Add(time.Duration(day*24) * time.Hour).Weekday()
		var baseValue float64

		if dayOfWeek >= 1 && dayOfWeek <= 5 { // Monday to Friday
			baseValue = 600
		} else { // Saturday, Sunday
			baseValue = 250
		}

		for hour := 0; hour < 24; hour++ {
			timestamp := baseTime.Add(time.Duration(day*24+hour) * time.Hour)
			// Add daily variation
			hourValue := baseValue + float64(hour-12)*10

			data.DataPoints = append(data.DataPoints, DataPoint{
				Timestamp: timestamp,
				Value:     math.Max(data.MinValue, hourValue),
				Namespace: "default",
				PodName:   "test-pod",
				Container: "app",
			})
		}
	}

	// Extract patterns
	daily, weekly := predictor.extractSeasonalPatterns(data)

	if daily == nil || weekly == nil {
		t.Fatal("Failed to extract patterns")
	}

	if len(daily) != 24 {
		t.Errorf("Expected 24 hours in daily pattern, got %d", len(daily))
	}

	if len(weekly) != 7 {
		t.Errorf("Expected 7 days in weekly pattern, got %d", len(weekly))
	}

	// Verify weekday > weekend for weekly pattern
	weekdayAvg := weekly["Monday"]
	weekendAvg := weekly["Saturday"]

	if weekdayAvg <= weekendAvg {
		t.Errorf("Expected weekday (%.0f) > weekend (%.0f)", weekdayAvg, weekendAvg)
	}

	t.Logf("Weekly pattern - Weekday: %.0f, Weekend: %.0f", weekdayAvg, weekendAvg)
}

func TestSeasonalPredictor_ConfidenceScaling(t *testing.T) {
	store := memstore.NewMemoryStore(7, 10080)
	predictor := NewSeasonalPredictor(store)

	// Generate 4 days of clean data with minute-granularity
	baseTime := time.Now().Add(-4 * 24 * time.Hour)
	data := HistoricalData{
		ResourceType: "cpu",
		DataPoints:   make([]DataPoint, 0),
		MinValue:     100,
		MaxValue:     400,
		LastUpdated:  time.Now(),
	}

	for day := 0; day < 4; day++ {
		for hour := 0; hour < 24; hour++ {
			for minute := 0; minute < 60; minute += 10 {
				timestamp := baseTime.Add(time.Duration(day*24*60+hour*60+minute) * time.Minute)
				value := 250.0 // Stable value

				data.DataPoints = append(data.DataPoints, DataPoint{
					Timestamp: timestamp,
					Value:     value,
					Namespace: "default",
					PodName:   "test-pod",
					Container: "app",
				})
			}
		}
	}

	// Test confidence at different horizons
	horizons := []time.Duration{
		1 * time.Hour,
		1 * 24 * time.Hour,
		7 * 24 * time.Hour,
	}

	predictions, err := predictor.Predict(data, horizons)
	if err != nil {
		t.Fatalf("Predict failed: %v", err)
	}

	// Confidence should decrease with longer horizons
	if predictions[0].Confidence <= predictions[1].Confidence {
		t.Errorf("1h confidence (%.2f) should be > 24h confidence (%.2f)",
			predictions[0].Confidence, predictions[1].Confidence)
	}

	if predictions[1].Confidence <= predictions[2].Confidence {
		t.Errorf("24h confidence (%.2f) should be > 7d confidence (%.2f)",
			predictions[1].Confidence, predictions[2].Confidence)
	}

	t.Logf("Confidence scaling: 1h=%.2f, 24h=%.2f, 7d=%.2f",
		predictions[0].Confidence, predictions[1].Confidence, predictions[2].Confidence)
}

func TestSeasonalPredictor_GetMethod(t *testing.T) {
	store := memstore.NewMemoryStore(7, 10080)
	predictor := NewSeasonalPredictor(store)

	if predictor.GetMethod() != PredictionMethodSeasonal {
		t.Errorf("Expected %s, got %s", PredictionMethodSeasonal, predictor.GetMethod())
	}
}

func TestSeasonalPredictor_GetMinDataPoints(t *testing.T) {
	store := memstore.NewMemoryStore(7, 10080)
	predictor := NewSeasonalPredictor(store)

	minPoints := predictor.GetMinDataPoints()
	expectedMin := 72 // 3 days of hourly data
	if minPoints != expectedMin {
		t.Errorf("Expected min data points %d, got %d", expectedMin, minPoints)
	}
}

func BenchmarkSeasonalPredictor_Predict(b *testing.B) {
	store := memstore.NewMemoryStore(7, 10080)
	predictor := NewSeasonalPredictor(store)

	// Generate 7 days of data
	baseTime := time.Now().Add(-7 * 24 * time.Hour)
	data := HistoricalData{
		ResourceType: "cpu",
		DataPoints:   make([]DataPoint, 0),
		MinValue:     100,
		MaxValue:     500,
		LastUpdated:  time.Now(),
	}

	for day := 0; day < 7; day++ {
		for hour := 0; hour < 24; hour++ {
			timestamp := baseTime.Add(time.Duration(day*24+hour) * time.Hour)
			value := 250 + float64(hour-12)*5

			data.DataPoints = append(data.DataPoints, DataPoint{
				Timestamp: timestamp,
				Value:     math.Max(data.MinValue, value),
				Namespace: "default",
				PodName:   "test-pod",
				Container: "app",
			})
		}
	}

	horizons := []time.Duration{
		1 * time.Hour,
		6 * time.Hour,
		24 * time.Hour,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = predictor.Predict(data, horizons)
	}
}

func BenchmarkSeasonalPredictor_ExtractPatterns(b *testing.B) {
	store := memstore.NewMemoryStore(7, 10080)
	predictor := NewSeasonalPredictor(store)

	// Generate 7 days of data
	baseTime := time.Now().Add(-7 * 24 * time.Hour)
	data := HistoricalData{
		ResourceType: "cpu",
		DataPoints:   make([]DataPoint, 0),
		MinValue:     100,
		MaxValue:     500,
		LastUpdated:  time.Now(),
	}

	for day := 0; day < 7; day++ {
		for hour := 0; hour < 24; hour++ {
			timestamp := baseTime.Add(time.Duration(day*24+hour) * time.Hour)
			value := 250.0

			data.DataPoints = append(data.DataPoints, DataPoint{
				Timestamp: timestamp,
				Value:     value,
				Namespace: "default",
				PodName:   "test-pod",
				Container: "app",
			})
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		predictor.extractSeasonalPatterns(data)
	}
}

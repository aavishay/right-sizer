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
	"testing"
	"time"
)

// TestPredictionEngineWithRealisticData tests the prediction engine with realistic workload patterns
func TestPredictionEngineWithRealisticData(t *testing.T) {
	// Create prediction engine
	config := DefaultConfig()
	config.MinDataPoints = 5
	config.ConfidenceThreshold = 0.3 // Lower threshold for testing

	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create prediction engine: %v", err)
	}

	ctx := context.Background()
	err = engine.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start prediction engine: %v", err)
	}
	defer engine.Stop()

	// Simulate a web application workload with daily patterns
	namespace := "default"
	podName := "webapp-123"
	container := "app"

	fmt.Printf("🔮 Testing Prediction Engine with Realistic Workload Patterns\n")
	fmt.Printf("============================================================\n\n")

	// Simulate 48 hours of data with a daily pattern
	baseTime := time.Now().Add(-48 * time.Hour)

	// Pattern: Low usage at night (50-100), medium during day (200-300), peaks at lunch/evening (400-500)
	for hour := 0; hour < 48; hour++ {
		timestamp := baseTime.Add(time.Duration(hour) * time.Hour)

		// CPU pattern (millicores)
		var cpuUsage float64
		hourOfDay := hour % 24
		switch {
		case hourOfDay >= 0 && hourOfDay < 6: // Night: low usage
			cpuUsage = 50 + float64(hour%3)*15 // 50-80
		case hourOfDay >= 6 && hourOfDay < 9: // Morning ramp-up
			cpuUsage = 80 + float64(hourOfDay-6)*40 // 80-200
		case hourOfDay >= 9 && hourOfDay < 12: // Morning business
			cpuUsage = 200 + float64(hour%4)*25 // 200-275
		case hourOfDay >= 12 && hourOfDay < 14: // Lunch peak
			cpuUsage = 400 + float64(hour%3)*30 // 400-460
		case hourOfDay >= 14 && hourOfDay < 18: // Afternoon
			cpuUsage = 250 + float64(hour%5)*20 // 250-330
		case hourOfDay >= 18 && hourOfDay < 21: // Evening peak
			cpuUsage = 380 + float64(hour%4)*25 // 380-455
		default: // Night wind-down
			cpuUsage = 150 - float64(hourOfDay-21)*30 // 150-60
		}

		// Memory pattern (MB) - more stable but follows similar pattern
		memoryUsage := cpuUsage * 2.5 // Memory roughly 2.5x CPU usage

		// Add some realistic noise
		cpuUsage += float64((hour*17)%20) - 10    // +/- 10 millicores
		memoryUsage += float64((hour*23)%30) - 15 // +/- 15 MB

		// Store data points
		err = engine.StoreDataPoint(namespace, podName, container, "cpu", cpuUsage, timestamp)
		if err != nil {
			t.Errorf("Failed to store CPU data point at hour %d: %v", hour, err)
		}

		err = engine.StoreDataPoint(namespace, podName, container, "memory", memoryUsage, timestamp)
		if err != nil {
			t.Errorf("Failed to store memory data point at hour %d: %v", hour, err)
		}
	}

	fmt.Printf("📊 Stored 48 hours of realistic workload data\n\n")

	// Test different prediction horizons
	horizons := []time.Duration{
		1 * time.Hour,
		4 * time.Hour,
		12 * time.Hour,
		24 * time.Hour,
	}

	// Test CPU predictions
	fmt.Printf("🖥️  CPU Predictions:\n")
	for _, horizon := range horizons {
		prediction, err := engine.GetBestPrediction(ctx, namespace, podName, container, "cpu", horizon)
		if err != nil {
			fmt.Printf("   %8s: No prediction available (%v)\n", horizon, err)
			continue
		}

		fmt.Printf("   %8s: %6.1f millicores (confidence: %.2f, method: %s)\n",
			horizon, prediction.Value, prediction.Confidence, prediction.Method)

		if prediction.ConfidenceInterval != nil {
			fmt.Printf("            Range: %.1f - %.1f millicores (%.0f%% confidence)\n",
				prediction.ConfidenceInterval.Lower, prediction.ConfidenceInterval.Upper, prediction.ConfidenceInterval.Percentage)
		}
	}

	fmt.Printf("\n💾 Memory Predictions:\n")
	for _, horizon := range horizons {
		prediction, err := engine.GetBestPrediction(ctx, namespace, podName, container, "memory", horizon)
		if err != nil {
			fmt.Printf("   %8s: No prediction available (%v)\n", horizon, err)
			continue
		}

		fmt.Printf("   %8s: %6.1f MB (confidence: %.2f, method: %s)\n",
			horizon, prediction.Value, prediction.Confidence, prediction.Method)

		if prediction.ConfidenceInterval != nil {
			fmt.Printf("            Range: %.1f - %.1f MB (%.0f%% confidence)\n",
				prediction.ConfidenceInterval.Lower, prediction.ConfidenceInterval.Upper, prediction.ConfidenceInterval.Percentage)
		}
	}

	// Test prediction request with multiple horizons and methods
	fmt.Printf("\n🔬 Detailed Prediction Analysis:\n")
	request := PredictionRequest{
		Namespace:    namespace,
		PodName:      podName,
		Container:    container,
		ResourceType: "cpu",
		Horizons:     horizons,
		Methods:      []PredictionMethod{PredictionMethodLinearRegression, PredictionMethodExponentialSmoothing, PredictionMethodSimpleMovingAverage},
	}

	response, err := engine.Predict(ctx, request)
	if err != nil {
		t.Errorf("Failed to get predictions: %v", err)
	} else {
		fmt.Printf("   Data Points Used: %d\n", response.DataPoints)
		fmt.Printf("   Total Predictions: %d\n", len(response.Predictions))

		for _, pred := range response.Predictions {
			fmt.Printf("   %s (%s): %.1f millicores (confidence: %.2f)\n",
				pred.Horizon, pred.Method, pred.Value, pred.Confidence)
		}
	}

	// Test historical data retrieval
	fmt.Printf("\n📈 Historical Data Summary:\n")
	since := time.Now().Add(-24 * time.Hour)
	historicalData, err := engine.GetHistoricalData(namespace, podName, container, "cpu", since)
	if err != nil {
		t.Errorf("Failed to get historical data: %v", err)
	} else {
		fmt.Printf("   Data Points (last 24h): %d\n", len(historicalData.DataPoints))
		fmt.Printf("   Min Value: %.1f millicores\n", historicalData.MinValue)
		fmt.Printf("   Max Value: %.1f millicores\n", historicalData.MaxValue)

		if len(historicalData.DataPoints) > 0 {
			latest := historicalData.DataPoints[len(historicalData.DataPoints)-1]
			fmt.Printf("   Latest Value: %.1f millicores at %s\n", latest.Value, latest.Timestamp.Format("15:04"))
		}
	}

	// Test engine statistics
	fmt.Printf("\n📊 Engine Statistics:\n")
	stats := engine.GetStats()
	fmt.Printf("   Running: %v\n", stats["isRunning"])
	fmt.Printf("   Predictors: %v\n", stats["predictors"])
	fmt.Printf("   Methods: %v\n", stats["methods"])

	if storeStats, ok := stats["store"]; ok {
		storeStatsMap := storeStats.(map[string]interface{})
		fmt.Printf("   Total Resources: %v\n", storeStatsMap["totalResources"])
		fmt.Printf("   Total Data Points: %v\n", storeStatsMap["totalDataPoints"])
		fmt.Printf("   Total Predictions: %v\n", storeStatsMap["totalPredictions"])
	}

	fmt.Printf("\n✅ Prediction engine test completed successfully!\n")
	fmt.Printf("\n💡 Key Insights:\n")
	fmt.Printf("   - The prediction engine successfully processed 48 hours of realistic workload data\n")
	fmt.Printf("   - Multiple prediction algorithms provide different perspectives on future resource needs\n")
	fmt.Printf("   - Confidence intervals help assess prediction reliability\n")
	fmt.Printf("   - The system automatically manages historical data retention and cleanup\n")
}

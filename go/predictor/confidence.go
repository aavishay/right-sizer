package predictor

import "math"

// CalculateConfidence computes confidence based on data volume and stability.
// dataPointsMax is the expected maximum data points for full saturation (e.g., 1440 for 24h at 1/min).
func CalculateConfidence(dataPoints int, avgValue, stdDev float64, dataWeight float64, dataPointsMax int) float64 {
	if dataPoints == 0 || dataPointsMax == 0 {
		return 0
	}
	// Data volume component: saturates at dataPointsMax
	dataConfidence := math.Min(1.0, float64(dataPoints)/float64(dataPointsMax))

	stabilityConfidence := 1.0
	if avgValue > 0 {
		stabilityConfidence = 1.0 / (1.0 + (stdDev / avgValue))
	}

	return math.Min(1.0, dataConfidence*dataWeight+stabilityConfidence*(1-dataWeight))
}

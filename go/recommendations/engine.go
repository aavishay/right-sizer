package recommendations

import (
	"fmt"
	"math"
	"right-sizer/memstore"
	"sort"
	"time"
)

// Recommendation represents an optimization suggestion
type Recommendation struct {
	Namespace        string
	PodName          string
	Container        string
	ResourceType     string
	CurrentValue     float64
	RecommendedValue float64
	Savings          float64
	SavingsPercent   float64
	Priority         int
	Confidence       float64
	Reason           string
	ImpactArea       string
	Timestamp        time.Time
	ExpiresAt        time.Time
}

// Engine generates optimization recommendations
type Engine struct {
	store               *memstore.MemoryStore
	minCPUUtilization   float64
	maxCPUUtilization   float64
	minMemUtilization   float64
	maxMemUtilization   float64
	confidenceThreshold float64
	savingsThreshold    float64
}

// NewEngine creates a recommendation engine
func NewEngine(store *memstore.MemoryStore) *Engine {
	return &Engine{
		store:               store,
		minCPUUtilization:   0.20,
		maxCPUUtilization:   0.80,
		minMemUtilization:   0.30,
		maxMemUtilization:   0.85,
		confidenceThreshold: 0.70,
		savingsThreshold:    1.0,
	}
}

// GenerateRecommendation creates a recommendation for a pod
func (e *Engine) GenerateRecommendation(namespace, podName, container, resourceType string, currentValue float64) (*Recommendation, error) {
	stats := e.store.Query(namespace, podName, 7*24*time.Hour)
	if stats == nil || stats.Count == 0 {
		return nil, fmt.Errorf("insufficient historical data")
	}
	cpuP95, memP95 := e.store.Percentile(namespace, podName, 7*24*time.Hour, 95)

	computed := e.calculateStats(stats, cpuP95, memP95, resourceType)

	minUtil := e.minMemUtilization
	maxUtil := e.maxMemUtilization
	if resourceType == "cpu" {
		minUtil = e.minCPUUtilization
		maxUtil = e.maxCPUUtilization
	}

	utilization := computed.MaxValue / currentValue
	confidence := e.calculateConfidence(computed, stats.Count)

	if confidence < e.confidenceThreshold {
		return nil, fmt.Errorf("confidence %.2f below threshold", confidence)
	}

	var recommendation *Recommendation
	if utilization < minUtil {
		recommendation = e.generateDownscaleRec(namespace, podName, container, resourceType, currentValue, computed, confidence)
	} else if utilization > maxUtil {
		recommendation = e.generateUpscaleRec(namespace, podName, container, resourceType, currentValue, computed, confidence)
	} else {
		return nil, fmt.Errorf("resource allocation is optimal")
	}

	if recommendation.Savings < e.savingsThreshold {
		return nil, fmt.Errorf("savings %.2f below threshold", recommendation.Savings)
	}

	return recommendation, nil
}

// GenerateRecommendations generates recommendations for multiple pods
func (e *Engine) GenerateRecommendations(
	pods []struct {
		Namespace    string
		PodName      string
		Container    string
		ResourceType string
		CurrentValue float64
	},
) []*Recommendation {
	var recommendations []*Recommendation

	for _, pod := range pods {
		rec, err := e.GenerateRecommendation(
			pod.Namespace, pod.PodName, pod.Container, pod.ResourceType, pod.CurrentValue)
		if err == nil && rec != nil {
			recommendations = append(recommendations, rec)
		}
	}

	sort.Slice(recommendations, func(i, j int) bool {
		if recommendations[i].Priority != recommendations[j].Priority {
			return recommendations[i].Priority < recommendations[j].Priority
		}
		return recommendations[i].Savings > recommendations[j].Savings
	})

	return recommendations
}

type stats struct {
	AvgValue float64
	MaxValue float64
	MinValue float64
	Stddev   float64
	P95      float64
}

func (e *Engine) calculateStats(data *memstore.Stats, cpuP95, memP95 float64, resourceType string) stats {
	if data == nil {
		return stats{}
	}

	s := stats{}

	switch resourceType {
	case "cpu":
		s.AvgValue = data.CPUMean
		s.MaxValue = data.CPUMax
		s.MinValue = data.CPUMin
		s.Stddev = data.CPUStdDev
		s.P95 = cpuP95
	default:
		s.AvgValue = data.MemMean
		s.MaxValue = data.MemMax
		s.MinValue = data.MemMin
		s.Stddev = data.MemStdDev
		s.P95 = memP95
	}

	return s
}

func (e *Engine) generateDownscaleRec(namespace, podName, container, resourceType string, currentValue float64, s stats, confidence float64) *Recommendation {
	buffer := 1.1
	recommendedValue := s.P95 * buffer
	recommendedValue = math.Max(recommendedValue, s.MaxValue*0.5)

	savings := currentValue - recommendedValue
	savingsPercent := (savings / currentValue) * 100

	return &Recommendation{
		Namespace:        namespace,
		PodName:          podName,
		Container:        container,
		ResourceType:     resourceType,
		CurrentValue:     currentValue,
		RecommendedValue: recommendedValue,
		Savings:          savings,
		SavingsPercent:   savingsPercent,
		Priority:         3,
		Confidence:       confidence,
		Reason:           fmt.Sprintf("Peak usage (%.0f) is only %.0f%% of allocation", s.MaxValue, (s.MaxValue/currentValue)*100),
		ImpactArea:       "cost",
		Timestamp:        time.Now(),
		ExpiresAt:        time.Now().Add(30 * 24 * time.Hour),
	}
}

func (e *Engine) generateUpscaleRec(namespace, podName, container, resourceType string, currentValue float64, s stats, confidence float64) *Recommendation {
	buffer := 1.2
	recommendedValue := s.P95 * buffer
	recommendedValue = math.Min(recommendedValue, currentValue*1.5)

	savings := 0.0 - (recommendedValue - currentValue)

	return &Recommendation{
		Namespace:        namespace,
		PodName:          podName,
		Container:        container,
		ResourceType:     resourceType,
		CurrentValue:     currentValue,
		RecommendedValue: recommendedValue,
		Savings:          savings,
		SavingsPercent:   (savings / currentValue) * 100,
		Priority:         1,
		Confidence:       confidence,
		Reason:           fmt.Sprintf("Peak usage (%.0f) is %.0f%% of allocation - risk of throttling", s.MaxValue, (s.MaxValue/currentValue)*100),
		ImpactArea:       "performance",
		Timestamp:        time.Now(),
		ExpiresAt:        time.Now().Add(30 * 24 * time.Hour),
	}
}

func (e *Engine) calculateConfidence(s stats, dataPoints int) float64 {
	dataConfidence := math.Min(1.0, float64(dataPoints)/float64(7*24*60))
	if s.AvgValue == 0 {
		return dataConfidence * 0.6
	}
	varianceConfidence := 1.0 / (1.0 + (s.Stddev / s.AvgValue))
	return (dataConfidence * 0.6) + (varianceConfidence * 0.4)
}

// IsExpired checks if recommendation is expired
func (r *Recommendation) IsExpired() bool {
	return time.Now().After(r.ExpiresAt)
}

// GetPriorityLabel returns human-readable priority
func (r *Recommendation) GetPriorityLabel() string {
	switch r.Priority {
	case 1:
		return "CRITICAL"
	case 2:
		return "HIGH"
	case 3:
		return "MEDIUM"
	case 4:
		return "LOW"
	default:
		return "UNKNOWN"
	}
}

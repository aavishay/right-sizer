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

package predictive

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"right-sizer/events"
	"right-sizer/memstore"
	"right-sizer/predictor"
)

// Scaler provides predictive scaling capabilities
type Scaler struct {
	k8sClient     kubernetes.Interface
	store         *memstore.MemoryStore
	seasonalPred  *predictor.SeasonalPredictor
	eventBus      *events.EventBus
	logger        logr.Logger

	// Configuration
	config ScalerConfig

	// State
	running  bool
	stopChan chan struct{}
	wg       sync.WaitGroup
	mutex    sync.RWMutex
}

// ScalerConfig holds configuration for predictive scaling
type ScalerConfig struct {
	// How often to check for scaling opportunities
	CheckInterval time.Duration

	// Minimum confidence required to apply scaling
	MinConfidence float64

	// Time window to look ahead for predictions
	PredictionHorizon time.Duration

	// Maximum percentage increase per scaling action
	MaxScaleUpPercent float64

	// Maximum percentage decrease per scaling action
	MaxScaleDownPercent float64

	// Minimum pod age before considering for scaling
	MinPodAge time.Duration

	// Whether to automatically apply scaling
	AutoApply bool

	// Namespaces to include (empty = all)
	IncludeNamespaces []string

	// Namespaces to exclude
	ExcludeNamespaces []string
}

// DefaultScalerConfig returns sensible default configuration
func DefaultScalerConfig() ScalerConfig {
	return ScalerConfig{
		CheckInterval:       5 * time.Minute,
		MinConfidence:       0.75,
		PredictionHorizon:   1 * time.Hour,
		MaxScaleUpPercent:   50,
		MaxScaleDownPercent: 20,
		MinPodAge:           10 * time.Minute,
		AutoApply:           false, // Require approval by default
		IncludeNamespaces:   []string{},
		ExcludeNamespaces:   []string{"kube-system", "kube-public"},
	}
}

// NewScaler creates a new predictive scaler
func NewScaler(
	k8sClient kubernetes.Interface,
	store *memstore.MemoryStore,
	eventBus *events.EventBus,
	logger logr.Logger,
	config ScalerConfig,
) *Scaler {
	return &Scaler{
		k8sClient:    k8sClient,
		store:        store,
		seasonalPred: predictor.NewSeasonalPredictor(),
		eventBus:     eventBus,
		logger:       logger,
		config:       config,
		stopChan:     make(chan struct{}),
	}
}

// Start begins the predictive scaling controller
func (s *Scaler) Start(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return fmt.Errorf("scaler already running")
	}

	s.running = true
	s.wg.Add(1)
	go s.run(ctx)

	s.logger.Info("Predictive scaler started",
		"checkInterval", s.config.CheckInterval,
		"minConfidence", s.config.MinConfidence,
		"autoApply", s.config.AutoApply,
	)
	return nil
}

// Stop stops the predictive scaler
func (s *Scaler) Stop() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return
	}

	s.running = false
	close(s.stopChan)
	s.wg.Wait()

	s.logger.Info("Predictive scaler stopped")
}

// run is the main scaling loop
func (s *Scaler) run(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.CheckInterval)
	defer ticker.Stop()

	// Run initial check
	s.checkScalingOpportunities(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case <-ticker.C:
			s.checkScalingOpportunities(ctx)
		}
	}
}

// checkScalingOpportunities checks all pods for scaling opportunities
func (s *Scaler) checkScalingOpportunities(ctx context.Context) {
	// Get all pods
	podList, err := s.k8sClient.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		s.logger.Error(err, "Failed to list pods")
		return
	}

	for _, pod := range podList.Items {
		if !s.shouldProcessPod(&pod) {
			continue
		}

		s.analyzePodForScaling(ctx, &pod)
	}
}

// shouldProcessPod determines if a pod should be considered for scaling
func (s *Scaler) shouldProcessPod(pod *corev1.Pod) bool {
	// Skip if pod is too new
	if time.Since(pod.CreationTimestamp.Time) < s.config.MinPodAge {
		return false
	}

	// Skip if not running
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	// Check excluded namespaces
	for _, ns := range s.config.ExcludeNamespaces {
		if pod.Namespace == ns {
			return false
		}
	}

	// Check included namespaces (if specified)
	if len(s.config.IncludeNamespaces) > 0 {
		found := false
		for _, ns := range s.config.IncludeNamespaces {
			if pod.Namespace == ns {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// analyzePodForScaling analyzes a pod for scaling opportunities
func (s *Scaler) analyzePodForScaling(ctx context.Context, pod *corev1.Pod) {
	// Get historical data
	data := s.store.GetHistoricalData(pod.Namespace, pod.Name, 7*24*time.Hour) // 7 days
	if len(data) < 72 { // Need at least 3 days of data
		return
	}

	// Get current limits
	limits := s.store.GetLimits(pod.Namespace, pod.Name)
	if limits == nil {
		return
	}

	// Check each container
	for _, container := range pod.Spec.Containers {
		s.analyzeContainerForScaling(ctx, pod, &container, data, limits)
	}
}

// analyzeContainerForScaling analyzes a container for scaling
func (s *Scaler) analyzeContainerForScaling(
	ctx context.Context,
	pod *corev1.Pod,
	container *corev1.Container,
	data []memstore.DataPoint,
	limits *memstore.ResourceLimits,
) {
	containerName := container.Name

	// Convert historical data to predictor format
	historicalData := predictor.HistoricalData{
		ResourceType: "cpu",
		DataPoints:   make([]predictor.DataPoint, 0, len(data)),
	}

	for _, dp := range data {
		historicalData.DataPoints = append(historicalData.DataPoints, predictor.DataPoint{
			Timestamp: dp.Timestamp,
			Value:     dp.CPUMilli,
		})
	}

	// Get predictions
	predictions, err := s.seasonalPred.Predict(historicalData, []time.Duration{
		s.config.PredictionHorizon,
	})
	if err != nil {
		s.logger.V(1).Info("Failed to predict for container",
			"pod", pod.Name,
			"container", containerName,
			"error", err,
		)
		return
	}

	if len(predictions) == 0 {
		return
	}

	prediction := predictions[0]

	// Check if prediction confidence is high enough
	if prediction.Confidence < s.config.MinConfidence {
		return
	}

	// Get current CPU limit
	currentCPULimit := limits.CPULimit
	if currentCPULimit == 0 {
		return
	}

	// Calculate predicted utilization
	predictedUtilization := prediction.Value / currentCPULimit

	s.logger.V(1).Info("Prediction for container",
		"pod", pod.Name,
		"container", containerName,
		"predictedValue", prediction.Value,
		"confidence", prediction.Confidence,
		"predictedUtilization", predictedUtilization,
	)

	// Decide on scaling action
	if predictedUtilization > 0.75 {
		// Predicted high utilization - scale up
		s.proposeScaleUp(ctx, pod, container, prediction, currentCPULimit)
	} else if predictedUtilization < 0.3 {
		// Predicted low utilization - scale down
		s.proposeScaleDown(ctx, pod, container, prediction, currentCPULimit)
	}
}

// proposeScaleUp proposes a scale-up action
func (s *Scaler) proposeScaleUp(
	ctx context.Context,
	pod *corev1.Pod,
	container *corev1.Container,
	prediction predictor.ResourcePrediction,
	currentLimit float64,
) {
	// Calculate new limit
	newLimit := currentLimit * 1.3 // 30% increase
	maxLimit := currentLimit * (1 + s.config.MaxScaleUpPercent/100)
	if newLimit > maxLimit {
		newLimit = maxLimit
	}

	// Create scaling proposal
	proposal := ScalingProposal{
		PodName:       pod.Name,
		Namespace:     pod.Namespace,
		Container:     container.Name,
		CurrentLimit:  currentLimit,
		ProposedLimit: newLimit,
		Reason:        "Predicted high utilization based on seasonal patterns",
		Confidence:    prediction.Confidence,
		Prediction:    prediction,
	}

	s.logger.Info("Proposing scale-up",
		"pod", pod.Name,
		"container", container.Name,
		"currentLimit", currentLimit,
		"proposedLimit", newLimit,
		"confidence", prediction.Confidence,
	)

	// Publish scaling proposal event
	event := events.NewEvent(
		events.EventResourceOptimized,
		"cluster-id",
		pod.Namespace,
		pod.Name,
		events.SeverityInfo,
		fmt.Sprintf("Proposed scale-up for %s: %.0fm -> %.0fm (%.1f%% confidence)",
			container.Name, currentLimit, newLimit, prediction.Confidence*100),
	)
	event.WithDetails(map[string]interface{}{
		"container":     container.Name,
		"currentLimit":  currentLimit,
		"proposedLimit": newLimit,
		"confidence":    prediction.Confidence,
		"prediction":    prediction,
	})
	event.WithTags("predictive", "scale-up", "proposed")

	s.eventBus.Publish(event)

	// Apply if auto-apply is enabled
	if s.config.AutoApply {
		s.applyScaling(ctx, proposal)
	}
}

// proposeScaleDown proposes a scale-down action
func (s *Scaler) proposeScaleDown(
	ctx context.Context,
	pod *corev1.Pod,
	container *corev1.Container,
	prediction predictor.ResourcePrediction,
	currentLimit float64,
) {
	// Calculate new limit
	newLimit := currentLimit * 0.8 // 20% decrease
	minLimit := currentLimit * (1 - s.config.MaxScaleDownPercent/100)
	if newLimit < minLimit {
		newLimit = minLimit
	}

	// Ensure minimum of 100m CPU
	if newLimit < 100 {
		newLimit = 100
	}

	// Create scaling proposal
	proposal := ScalingProposal{
		PodName:       pod.Name,
		Namespace:     pod.Namespace,
		Container:     container.Name,
		CurrentLimit:  currentLimit,
		ProposedLimit: newLimit,
		Reason:        "Predicted low utilization based on seasonal patterns",
		Confidence:    prediction.Confidence,
		Prediction:    prediction,
	}

	s.logger.Info("Proposing scale-down",
		"pod", pod.Name,
		"container", container.Name,
		"currentLimit", currentLimit,
		"proposedLimit", newLimit,
		"confidence", prediction.Confidence,
	)

	// Publish scaling proposal event
	event := events.NewEvent(
		events.EventResourceOptimized,
		"cluster-id",
		pod.Namespace,
		pod.Name,
		events.SeverityInfo,
		fmt.Sprintf("Proposed scale-down for %s: %.0fm -> %.0fm (%.1f%% confidence)",
			container.Name, currentLimit, newLimit, prediction.Confidence*100),
	)
	event.WithDetails(map[string]interface{}{
		"container":     container.Name,
		"currentLimit":  currentLimit,
		"proposedLimit": newLimit,
		"confidence":    prediction.Confidence,
		"prediction":    prediction,
	})
	event.WithTags("predictive", "scale-down", "proposed")

	s.eventBus.Publish(event)

	// Apply if auto-apply is enabled
	if s.config.AutoApply {
		s.applyScaling(ctx, proposal)
	}
}

// ScalingProposal represents a proposed scaling action
type ScalingProposal struct {
	PodName       string
	Namespace     string
	Container     string
	CurrentLimit  float64
	ProposedLimit float64
	Reason        string
	Confidence    float64
	Prediction    predictor.ResourcePrediction
}

// applyScaling applies a scaling proposal to the pod
func (s *Scaler) applyScaling(ctx context.Context, proposal ScalingProposal) error {
	// Get the pod
	pod, err := s.k8sClient.CoreV1().Pods(proposal.Namespace).Get(ctx, proposal.PodName, metav1.GetOptions{})
	if err != nil {
		s.logger.Error(err, "Failed to get pod for scaling",
			"pod", proposal.PodName,
			"namespace", proposal.Namespace,
		)
		return err
	}

	// Update each container
	for i, container := range pod.Spec.Containers {
		if container.Name != proposal.Container {
			continue
		}

		if container.Resources.Limits == nil {
			container.Resources.Limits = corev1.ResourceList{}
		}

		// Update CPU limit
		newCPULimit := int64(proposal.ProposedLimit)
		pod.Spec.Containers[i].Resources.Limits[corev1.ResourceCPU] = *resource.NewMilliQuantity(newCPULimit, resource.DecimalSI)
	}

	// Update the pod
	_, err = s.k8sClient.CoreV1().Pods(proposal.Namespace).Update(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		s.logger.Error(err, "Failed to apply scaling",
			"pod", proposal.PodName,
			"container", proposal.Container,
		)

		// Publish failure event
		event := events.NewEvent(
			events.EventRemediationFailed,
			"cluster-id",
			proposal.Namespace,
			proposal.PodName,
			events.SeverityWarning,
			fmt.Sprintf("Failed to apply scaling to %s: %v", proposal.Container, err),
		)
		s.eventBus.Publish(event)

		return err
	}

	s.logger.Info("Successfully applied scaling",
		"pod", proposal.PodName,
		"container", proposal.Container,
		"newLimit", proposal.ProposedLimit,
	)

	// Publish success event
	event := events.NewEvent(
		events.EventRemediationApplied,
		"cluster-id",
		proposal.Namespace,
		proposal.PodName,
		events.SeverityInfo,
		fmt.Sprintf("Applied scaling to %s: new CPU limit %.0fm", proposal.Container, proposal.ProposedLimit),
	)
	event.WithTags("predictive", "applied")
	s.eventBus.Publish(event)

	return nil
}

// GetConfig returns the current scaler configuration
func (s *Scaler) GetConfig() ScalerConfig {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.config
}

// UpdateConfig updates the scaler configuration
func (s *Scaler) UpdateConfig(config ScalerConfig) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.config = config
	s.logger.Info("Scaler configuration updated")
}

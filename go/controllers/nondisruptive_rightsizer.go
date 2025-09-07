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
package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"right-sizer/config"
	"right-sizer/logger"
	"right-sizer/metrics"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// NonDisruptiveRightSizer performs resource analysis and adds recommendations as annotations
// without actually modifying resources or restarting pods
type NonDisruptiveRightSizer struct {
	Client          client.Client
	MetricsProvider metrics.Provider
	Interval        time.Duration
}

// ResourceRecommendation stores recommended resource values
type ResourceRecommendation struct {
	CPURequest    string    `json:"cpu_request"`
	CPULimit      string    `json:"cpu_limit"`
	MemoryRequest string    `json:"memory_request"`
	MemoryLimit   string    `json:"memory_limit"`
	Timestamp     time.Time `json:"timestamp"`
	BasedOnPods   int       `json:"based_on_pods"`
	AvgCPUUsage   float64   `json:"avg_cpu_usage_milli"`
	AvgMemUsage   float64   `json:"avg_mem_usage_mb"`
}

// Start begins the continuous monitoring and recommendation loop
func (r *NonDisruptiveRightSizer) Start(ctx context.Context) error {
	ticker := time.NewTicker(r.Interval)
	defer ticker.Stop()

	logger.Info("Starting non-disruptive right-sizer with %v interval", r.Interval)

	// Run immediately on start
	r.analyzeAndRecommend(ctx)

	for {
		select {
		case <-ticker.C:
			r.analyzeAndRecommend(ctx)
		case <-ctx.Done():
			log.Println("Stopping non-disruptive right-sizer")
			return nil
		}
	}
}

// analyzeAndRecommend processes all deployments and statefulsets
func (r *NonDisruptiveRightSizer) analyzeAndRecommend(ctx context.Context) {
	// Analyze deployments
	var deployList appsv1.DeploymentList
	if err := r.Client.List(ctx, &deployList); err != nil {
		logger.Error("Error listing deployments: %v", err)
		return
	}

	log.Printf("Analyzing %d deployments for right-sizing recommendations", len(deployList.Items))

	for _, deploy := range deployList.Items {
		if isSystemResource(deploy.Namespace, deploy.Name) {
			continue
		}

		if err := r.analyzeDeployment(ctx, &deploy); err != nil {
			log.Printf("Error analyzing deployment %s/%s: %v",
				deploy.Namespace, deploy.Name, err)
		}
	}

	// Process StatefulSets
	var stsList appsv1.StatefulSetList
	if err := r.Client.List(ctx, &stsList); err != nil {
		log.Printf("Error listing statefulsets: %v", err)
		return
	}

	log.Printf("Analyzing %d statefulsets for right-sizing recommendations", len(stsList.Items))

	for _, sts := range stsList.Items {
		if isSystemResource(sts.Namespace, sts.Name) {
			continue
		}

		if err := r.analyzeStatefulSet(ctx, &sts); err != nil {
			log.Printf("Error analyzing statefulset %s/%s: %v",
				sts.Namespace, sts.Name, err)
		}
	}

	// Also analyze individual pods for immediate feedback
	r.analyzePods(ctx)
}

// analyzeDeployment analyzes a deployment and adds recommendations
func (r *NonDisruptiveRightSizer) analyzeDeployment(ctx context.Context, deploy *appsv1.Deployment) error {
	recommendation, avgMetrics, podCount := r.getResourceRecommendation(ctx, deploy.Namespace, deploy.Spec.Selector.MatchLabels)
	if recommendation == nil {
		return nil
	}

	// Add recommendation as annotation
	if deploy.Annotations == nil {
		deploy.Annotations = make(map[string]string)
	}

	cpuReq := recommendation.Requests[corev1.ResourceCPU]
	cpuLim := recommendation.Limits[corev1.ResourceCPU]
	memReq := recommendation.Requests[corev1.ResourceMemory]
	memLim := recommendation.Limits[corev1.ResourceMemory]

	recommendationData := ResourceRecommendation{
		CPURequest:    cpuReq.String(),
		CPULimit:      cpuLim.String(),
		MemoryRequest: memReq.String(),
		MemoryLimit:   memLim.String(),
		Timestamp:     time.Now(),
		BasedOnPods:   podCount,
		AvgCPUUsage:   avgMetrics.CPUMilli,
		AvgMemUsage:   avgMetrics.MemMB,
	}

	jsonData, err := json.Marshal(recommendationData)
	if err != nil {
		return err
	}

	deploy.Annotations["right-sizer/recommendation"] = string(jsonData)
	deploy.Annotations["right-sizer/last-analysis"] = time.Now().Format(time.RFC3339)

	// Check if current resources differ significantly from recommendations
	if r.needsAdjustment(&deploy.Spec.Template.Spec, recommendation) {
		deploy.Annotations["right-sizer/action-needed"] = "true"
		log.Printf("📊 Deployment %s/%s needs adjustment - Current: %s CPU, %s Memory | Recommended: %s CPU, %s Memory",
			deploy.Namespace, deploy.Name,
			getCurrentResources(&deploy.Spec.Template.Spec, "cpu"),
			getCurrentResources(&deploy.Spec.Template.Spec, "memory"),
			cpuReq.String(),
			memReq.String())
	} else {
		deploy.Annotations["right-sizer/action-needed"] = "false"
	}

	// Update the deployment annotations only
	return r.Client.Update(ctx, deploy)
}

// analyzeStatefulSet analyzes a statefulset and adds recommendations
func (r *NonDisruptiveRightSizer) analyzeStatefulSet(ctx context.Context, sts *appsv1.StatefulSet) error {
	recommendation, avgMetrics, podCount := r.getResourceRecommendation(ctx, sts.Namespace, sts.Spec.Selector.MatchLabels)
	if recommendation == nil {
		return nil
	}

	// Add recommendation as annotation
	if sts.Annotations == nil {
		sts.Annotations = make(map[string]string)
	}

	cpuReq := recommendation.Requests[corev1.ResourceCPU]
	cpuLim := recommendation.Limits[corev1.ResourceCPU]
	memReq := recommendation.Requests[corev1.ResourceMemory]
	memLim := recommendation.Limits[corev1.ResourceMemory]

	recommendationData := ResourceRecommendation{
		CPURequest:    cpuReq.String(),
		CPULimit:      cpuLim.String(),
		MemoryRequest: memReq.String(),
		MemoryLimit:   memLim.String(),
		Timestamp:     time.Now(),
		BasedOnPods:   podCount,
		AvgCPUUsage:   avgMetrics.CPUMilli,
		AvgMemUsage:   avgMetrics.MemMB,
	}

	jsonData, err := json.Marshal(recommendationData)
	if err != nil {
		return err
	}

	sts.Annotations["right-sizer/recommendation"] = string(jsonData)
	sts.Annotations["right-sizer/last-analysis"] = time.Now().Format(time.RFC3339)

	// Check if current resources differ significantly from recommendations
	if r.needsAdjustment(&sts.Spec.Template.Spec, recommendation) {
		sts.Annotations["right-sizer/action-needed"] = "true"
		log.Printf("📊 StatefulSet %s/%s needs adjustment - Current: %s CPU, %s Memory | Recommended: %s CPU, %s Memory",
			sts.Namespace, sts.Name,
			getCurrentResources(&sts.Spec.Template.Spec, "cpu"),
			getCurrentResources(&sts.Spec.Template.Spec, "memory"),
			cpuReq.String(),
			memReq.String())
	} else {
		sts.Annotations["right-sizer/action-needed"] = "false"
	}

	// Update the statefulset annotations only
	return r.Client.Update(ctx, sts)
}

// analyzePods adds recommendations directly to pod annotations
func (r *NonDisruptiveRightSizer) analyzePods(ctx context.Context) {
	var podList corev1.PodList
	if err := r.Client.List(ctx, &podList); err != nil {
		log.Printf("Error listing pods: %v", err)
		return
	}

	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodRunning || isSystemResource(pod.Namespace, pod.Name) {
			continue
		}

		usage, err := r.MetricsProvider.FetchPodMetrics(pod.Namespace, pod.Name)
		if err != nil {
			continue
		}

		recommendation := r.calculateOptimalResources(usage)

		// Update pod annotations with recommendations
		if pod.Annotations == nil {
			pod.Annotations = make(map[string]string)
		}

		cpuRec := recommendation.Requests[corev1.ResourceCPU]
		memRec := recommendation.Requests[corev1.ResourceMemory]

		pod.Annotations["right-sizer/cpu-usage"] = fmt.Sprintf("%.0fm", usage.CPUMilli)
		pod.Annotations["right-sizer/memory-usage"] = fmt.Sprintf("%.0fMi", usage.MemMB)
		pod.Annotations["right-sizer/cpu-recommendation"] = cpuRec.String()
		pod.Annotations["right-sizer/memory-recommendation"] = memRec.String()
		pod.Annotations["right-sizer/last-check"] = time.Now().Format(time.RFC3339)

		// Update the pod annotations
		if err := r.Client.Update(ctx, &pod); err != nil {
			log.Printf("Error updating pod annotations %s/%s: %v", pod.Namespace, pod.Name, err)
		}
	}
}

// getResourceRecommendation calculates recommendations based on pod metrics
func (r *NonDisruptiveRightSizer) getResourceRecommendation(ctx context.Context, namespace string, labels map[string]string) (*corev1.ResourceRequirements, metrics.Metrics, int) {
	var podList corev1.PodList
	matchLabels := client.MatchingLabels(labels)
	if err := r.Client.List(ctx, &podList, matchLabels, client.InNamespace(namespace)); err != nil {
		return nil, metrics.Metrics{}, 0
	}

	if len(podList.Items) == 0 {
		return nil, metrics.Metrics{}, 0
	}

	// Calculate average metrics across all pods
	totalCPU := 0.0
	totalMem := 0.0
	validPods := 0

	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		usage, err := r.MetricsProvider.FetchPodMetrics(pod.Namespace, pod.Name)
		if err != nil {
			continue
		}

		totalCPU += usage.CPUMilli
		totalMem += usage.MemMB
		validPods++
	}

	if validPods == 0 {
		return nil, metrics.Metrics{}, 0
	}

	avgMetrics := metrics.Metrics{
		CPUMilli: totalCPU / float64(validPods),
		MemMB:    totalMem / float64(validPods),
	}

	recommendation := r.calculateOptimalResources(avgMetrics)
	return &recommendation, avgMetrics, validPods
}

// calculateOptimalResources determines optimal resource allocation
func (r *NonDisruptiveRightSizer) calculateOptimalResources(usage metrics.Metrics) corev1.ResourceRequirements {
	cfg := config.Get()

	// Add buffer for requests using configurable multipliers and additions
	cpuRequest := int64(usage.CPUMilli*cfg.CPURequestMultiplier) + cfg.CPURequestAddition
	memRequest := int64(usage.MemMB*cfg.MemoryRequestMultiplier) + cfg.MemoryRequestAddition

	// Ensure minimum values
	if cpuRequest < cfg.MinCPURequest {
		cpuRequest = cfg.MinCPURequest
	}
	if memRequest < cfg.MinMemoryRequest {
		memRequest = cfg.MinMemoryRequest
	}

	// Calculate limits based on requests with multipliers and additions
	cpuLimit := int64(float64(cpuRequest)*cfg.CPULimitMultiplier) + cfg.CPULimitAddition
	memLimit := int64(float64(memRequest)*cfg.MemoryLimitMultiplier) + cfg.MemoryLimitAddition

	// Apply maximum caps
	if cpuLimit > cfg.MaxCPULimit {
		cpuLimit = cfg.MaxCPULimit
	}
	if memLimit > cfg.MaxMemoryLimit {
		memLimit = cfg.MaxMemoryLimit
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewMilliQuantity(cpuRequest, resource.DecimalSI),
			corev1.ResourceMemory: *resource.NewQuantity(memRequest*1024*1024, resource.BinarySI),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewMilliQuantity(cpuLimit, resource.DecimalSI),
			corev1.ResourceMemory: *resource.NewQuantity(memLimit*1024*1024, resource.BinarySI),
		},
	}
}

// needsAdjustment checks if resources need updating
func (r *NonDisruptiveRightSizer) needsAdjustment(podSpec *corev1.PodSpec, newResources *corev1.ResourceRequirements) bool {
	if len(podSpec.Containers) == 0 || newResources == nil {
		return false
	}

	// Check first container (main container)
	container := podSpec.Containers[0]

	// Get current CPU and memory requests
	currentCPU := container.Resources.Requests[corev1.ResourceCPU]
	currentMem := container.Resources.Requests[corev1.ResourceMemory]
	newCPU := newResources.Requests[corev1.ResourceCPU]
	newMem := newResources.Requests[corev1.ResourceMemory]

	// Skip if current resources are not set
	if currentCPU.IsZero() || currentMem.IsZero() {
		return true
	}

	// Calculate percentage difference
	cpuDiff := float64(newCPU.MilliValue()-currentCPU.MilliValue()) / float64(currentCPU.MilliValue()) * 100
	memDiff := float64(newMem.Value()-currentMem.Value()) / float64(currentMem.Value()) * 100

	// Only flag as needing adjustment if difference is more than 15%
	threshold := 15.0
	return (cpuDiff > threshold || cpuDiff < -threshold) || (memDiff > threshold || memDiff < -threshold)
}

// getCurrentResources returns current resource values as string
func getCurrentResources(podSpec *corev1.PodSpec, resourceType string) string {
	if len(podSpec.Containers) == 0 {
		return "not set"
	}

	container := podSpec.Containers[0]
	if resourceType == "cpu" {
		if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
			return cpu.String()
		}
	} else if resourceType == "memory" {
		if mem, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
			return mem.String()
		}
	}
	return "not set"
}

// isSystemResource checks if a resource is a system/infrastructure resource
func isSystemResource(namespace, name string) bool {
	cfg := config.Get()
	for _, ns := range cfg.SystemNamespaces {
		if namespace == ns {
			return true
		}
	}

	// Skip the right-sizer itself
	if name == "right-sizer" {
		return true
	}

	return false
}

// SetupNonDisruptiveRightSizer creates and starts the non-disruptive rightsizer
func SetupNonDisruptiveRightSizer(mgr manager.Manager, provider metrics.Provider) error {
	cfg := config.Get()
	rightsizer := &NonDisruptiveRightSizer{
		Client:          mgr.GetClient(),
		MetricsProvider: provider,
		Interval:        cfg.ResizeInterval,
	}

	// Start the rightsizer in a goroutine
	go func() {
		if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
			return rightsizer.Start(ctx)
		})); err != nil {
			log.Printf("Failed to add non-disruptive rightsizer to manager: %v", err)
		}
	}()

	log.Println("Non-disruptive right-sizer initialized - will add recommendations as annotations without restarting pods")
	return nil
}

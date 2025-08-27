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
	"log"
	"time"

	"right-sizer/config"
	"right-sizer/logger"
	"right-sizer/metrics"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// DeploymentRightSizer adjusts deployment resources based on pod metrics
type DeploymentRightSizer struct {
	Client          client.Client
	MetricsProvider metrics.Provider
	Interval        time.Duration
}

// Start begins the continuous monitoring and adjustment loop
func (r *DeploymentRightSizer) Start(ctx context.Context) error {
	ticker := time.NewTicker(r.Interval)
	defer ticker.Stop()

	logger.Info("Starting deployment right-sizer with %v interval", r.Interval)

	// Run immediately on start
	r.rightSizeDeployments(ctx)

	for {
		select {
		case <-ticker.C:
			r.rightSizeDeployments(ctx)
		case <-ctx.Done():
			log.Println("Stopping deployment right-sizer")
			return nil
		}
	}
}

// rightSizeDeployments processes all deployments in the cluster
func (r *DeploymentRightSizer) rightSizeDeployments(ctx context.Context) {
	var deployList appsv1.DeploymentList
	if err := r.Client.List(ctx, &deployList); err != nil {
		logger.Error("Error listing deployments: %v", err)
		return
	}

	log.Printf("Processing %d deployments for right-sizing", len(deployList.Items))

	for _, deploy := range deployList.Items {
		// Skip system deployments
		if isSystemDeployment(&deploy) {
			continue
		}

		if err := r.rightSizeDeployment(ctx, &deploy); err != nil {
			log.Printf("Error right-sizing deployment %s/%s: %v",
				deploy.Namespace, deploy.Name, err)
		}
	}

	// Also process StatefulSets
	var stsList appsv1.StatefulSetList
	if err := r.Client.List(ctx, &stsList); err != nil {
		log.Printf("Error listing statefulsets: %v", err)
		return
	}

	log.Printf("Processing %d statefulsets for right-sizing", len(stsList.Items))

	for _, sts := range stsList.Items {
		if isSystemStatefulSet(&sts) {
			continue
		}

		if err := r.rightSizeStatefulSet(ctx, &sts); err != nil {
			log.Printf("Error right-sizing statefulset %s/%s: %v",
				sts.Namespace, sts.Name, err)
		}
	}
}

// rightSizeDeployment adjusts resources for a single deployment
func (r *DeploymentRightSizer) rightSizeDeployment(ctx context.Context, deploy *appsv1.Deployment) error {
	// Get pods for this deployment
	var podList corev1.PodList
	labels := client.MatchingLabels(deploy.Spec.Selector.MatchLabels)
	if err := r.Client.List(ctx, &podList, labels, client.InNamespace(deploy.Namespace)); err != nil {
		return err
	}

	if len(podList.Items) == 0 {
		return nil
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
		return nil
	}

	avgCPU := totalCPU / float64(validPods)
	avgMem := totalMem / float64(validPods)

	// Calculate new resources based on average usage
	newResources := r.calculateOptimalResources(metrics.Metrics{
		CPUMilli: avgCPU,
		MemMB:    avgMem,
	})

	// Check if adjustment is needed
	if !r.needsAdjustment(&deploy.Spec.Template.Spec, newResources) {
		return nil
	}

	cpuReq := newResources.Requests[corev1.ResourceCPU]
	memReq := newResources.Requests[corev1.ResourceMemory]
	log.Printf("Adjusting deployment %s/%s - Avg CPU: %.0fm->%s, Avg Memory: %.0fMi->%s",
		deploy.Namespace, deploy.Name,
		avgCPU, cpuReq.String(),
		avgMem, memReq.String())

	// Update all containers in the deployment
	for i := range deploy.Spec.Template.Spec.Containers {
		deploy.Spec.Template.Spec.Containers[i].Resources = newResources
	}

	// Add annotation to track last update
	if deploy.Spec.Template.Annotations == nil {
		deploy.Spec.Template.Annotations = make(map[string]string)
	}
	deploy.Spec.Template.Annotations["right-sizer/last-update"] = time.Now().Format(time.RFC3339)

	// Update the deployment (this will trigger a rolling update)
	return r.Client.Update(ctx, deploy)
}

// rightSizeStatefulSet adjusts resources for a single statefulset
func (r *DeploymentRightSizer) rightSizeStatefulSet(ctx context.Context, sts *appsv1.StatefulSet) error {
	// Get pods for this statefulset
	var podList corev1.PodList
	labels := client.MatchingLabels(sts.Spec.Selector.MatchLabels)
	if err := r.Client.List(ctx, &podList, labels, client.InNamespace(sts.Namespace)); err != nil {
		return err
	}

	if len(podList.Items) == 0 {
		return nil
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
		return nil
	}

	avgCPU := totalCPU / float64(validPods)
	avgMem := totalMem / float64(validPods)

	// Calculate new resources based on average usage
	newResources := r.calculateOptimalResources(metrics.Metrics{
		CPUMilli: avgCPU,
		MemMB:    avgMem,
	})

	// Check if adjustment is needed
	if !r.needsAdjustment(&sts.Spec.Template.Spec, newResources) {
		return nil
	}

	cpuReq := newResources.Requests[corev1.ResourceCPU]
	memReq := newResources.Requests[corev1.ResourceMemory]
	log.Printf("Adjusting statefulset %s/%s - Avg CPU: %.0fm->%s, Avg Memory: %.0fMi->%s",
		sts.Namespace, sts.Name,
		avgCPU, cpuReq.String(),
		avgMem, memReq.String())

	// Update all containers in the statefulset
	for i := range sts.Spec.Template.Spec.Containers {
		sts.Spec.Template.Spec.Containers[i].Resources = newResources
	}

	// Add annotation to track last update
	if sts.Spec.Template.Annotations == nil {
		sts.Spec.Template.Annotations = make(map[string]string)
	}
	sts.Spec.Template.Annotations["right-sizer/last-update"] = time.Now().Format(time.RFC3339)

	// Update the statefulset (this will trigger a rolling update)
	return r.Client.Update(ctx, sts)
}

// calculateOptimalResources determines optimal resource allocation
func (r *DeploymentRightSizer) calculateOptimalResources(usage metrics.Metrics) corev1.ResourceRequirements {
	cfg := config.Get()

	// Add buffer for requests using configurable multipliers
	cpuRequest := int64(usage.CPUMilli * cfg.CPURequestMultiplier)
	memRequest := int64(usage.MemMB * cfg.MemoryRequestMultiplier)

	// Ensure minimum values
	if cpuRequest < cfg.MinCPURequest {
		cpuRequest = cfg.MinCPURequest
	}
	if memRequest < cfg.MinMemoryRequest {
		memRequest = cfg.MinMemoryRequest
	}

	// Set limits using configurable multipliers
	cpuLimit := int64(float64(cpuRequest) * cfg.CPULimitMultiplier)
	memLimit := int64(float64(memRequest) * cfg.MemoryLimitMultiplier)

	// Cap at configurable maximums
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

// needsAdjustment checks if pod template resources need updating
func (r *DeploymentRightSizer) needsAdjustment(podSpec *corev1.PodSpec, newResources corev1.ResourceRequirements) bool {
	if len(podSpec.Containers) == 0 {
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

	// Only adjust if difference is more than 10%
	threshold := 10.0
	return (cpuDiff > threshold || cpuDiff < -threshold) || (memDiff > threshold || memDiff < -threshold)
}

// isSystemDeployment checks if a deployment is a system/infrastructure deployment
func isSystemDeployment(deploy *appsv1.Deployment) bool {
	systemNamespaces := []string{"kube-system", "kube-public", "kube-node-lease"}
	for _, ns := range systemNamespaces {
		if deploy.Namespace == ns {
			return true
		}
	}

	// Skip the right-sizer itself
	if deploy.Name == "right-sizer" {
		return true
	}

	return false
}

// isSystemStatefulSet checks if a statefulset is a system/infrastructure statefulset
func isSystemStatefulSet(sts *appsv1.StatefulSet) bool {
	systemNamespaces := []string{"kube-system", "kube-public", "kube-node-lease"}
	for _, ns := range systemNamespaces {
		if sts.Namespace == ns {
			return true
		}
	}

	return false
}

// SetupDeploymentRightSizer creates and starts the deployment rightsizer
func SetupDeploymentRightSizer(mgr manager.Manager, provider metrics.Provider) error {
	cfg := config.Get()
	rightsizer := &DeploymentRightSizer{
		Client:          mgr.GetClient(),
		MetricsProvider: provider,
		Interval:        cfg.ResizeInterval,
	}

	// Start the rightsizer in a goroutine
	go func() {
		if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
			return rightsizer.Start(ctx)
		})); err != nil {
			log.Printf("Failed to add deployment rightsizer to manager: %v", err)
		}
	}()

	return nil
}

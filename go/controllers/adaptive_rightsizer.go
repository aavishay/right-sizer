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

	"right-sizer/config"
	"right-sizer/logger"
	"right-sizer/metrics"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// AdaptiveRightSizer performs resource optimization with support for both
// in-place updates (when available) and deployment updates as fallback
type AdaptiveRightSizer struct {
	Client          client.Client
	MetricsProvider metrics.Provider
	Interval        time.Duration
	InPlaceEnabled  bool // Will be auto-detected
	DryRun          bool // If true, only log recommendations without applying
}

// ResourceUpdate represents a pending resource update
type ResourceUpdate struct {
	Namespace     string
	Name          string
	ResourceType  string // "Pod", "Deployment", "StatefulSet"
	ContainerName string
	OldResources  corev1.ResourceRequirements
	NewResources  corev1.ResourceRequirements
	Method        string // "in-place" or "rolling-update"
	Reason        string
}

// Start begins the adaptive rightsizing loop
func (r *AdaptiveRightSizer) Start(ctx context.Context) error {
	ticker := time.NewTicker(r.Interval)
	defer ticker.Stop()

	// Test for in-place resize capability
	r.InPlaceEnabled = r.testInPlaceCapability(ctx)

	if r.InPlaceEnabled {
		logger.Info("✅ In-place pod resizing is available - pods can be resized without restarts")
	} else {
		logger.Warn("⚠️  In-place pod resizing not available - will use rolling updates")
	}

	logger.Info("Starting adaptive right-sizer with %v interval (DryRun: %v)", r.Interval, r.DryRun)

	// Run immediately on start
	r.performRightSizing(ctx)

	for {
		select {
		case <-ticker.C:
			r.performRightSizing(ctx)
		case <-ctx.Done():
			log.Println("Stopping adaptive right-sizer")
			return nil
		}
	}
}

// testInPlaceCapability checks if in-place resize is supported
func (r *AdaptiveRightSizer) testInPlaceCapability(ctx context.Context) bool {
	// Try to create a test pod with resizePolicy
	testPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "resize-capability-test-" + fmt.Sprintf("%d", time.Now().Unix()),
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "test",
					Image:   "busybox:latest",
					Command: []string{"sleep", "10"},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    *resource.NewMilliQuantity(10, resource.DecimalSI),
							corev1.ResourceMemory: *resource.NewQuantity(10*1024*1024, resource.BinarySI),
						},
					},
				},
			},
		},
	}

	// Create test pod
	if err := r.Client.Create(ctx, testPod); err != nil {
		return false
	}
	defer r.Client.Delete(ctx, testPod)

	// In K8s 1.27+ with feature enabled, resize subresource would be available
	// For now, we'll return false as the feature isn't fully available
	return false
}

// performRightSizing processes all workloads for optimization
func (r *AdaptiveRightSizer) performRightSizing(ctx context.Context) {
	updates := []ResourceUpdate{}

	// Process Deployments
	deployments, err := r.analyzeDeployments(ctx)
	if err != nil {
		log.Printf("Error analyzing deployments: %v", err)
	} else {
		updates = append(updates, deployments...)
	}

	// Process StatefulSets
	statefulSets, err := r.analyzeStatefulSets(ctx)
	if err != nil {
		log.Printf("Error analyzing statefulsets: %v", err)
	} else {
		updates = append(updates, statefulSets...)
	}

	// Process standalone Pods if in-place is enabled
	if r.InPlaceEnabled {
		pods, err := r.analyzeStandalonePods(ctx)
		if err != nil {
			log.Printf("Error analyzing pods: %v", err)
		} else {
			updates = append(updates, pods...)
		}
	}

	// Apply updates
	r.applyUpdates(ctx, updates)
}

// analyzeDeployments analyzes all deployments for resource optimization
func (r *AdaptiveRightSizer) analyzeDeployments(ctx context.Context) ([]ResourceUpdate, error) {
	var deployList appsv1.DeploymentList
	if err := r.Client.List(ctx, &deployList); err != nil {
		return nil, err
	}

	updates := []ResourceUpdate{}

	for _, deploy := range deployList.Items {
		if r.isSystemWorkload(deploy.Namespace, deploy.Name) {
			continue
		}

		// Get pods for this deployment
		pods, err := r.getPodsForWorkload(ctx, deploy.Namespace, deploy.Spec.Selector.MatchLabels)
		if err != nil {
			continue
		}

		if len(pods) == 0 {
			continue
		}

		// Calculate average metrics
		avgMetrics := r.calculateAverageMetrics(pods)
		if avgMetrics == nil {
			continue
		}

		// Check each container
		for i, container := range deploy.Spec.Template.Spec.Containers {
			newResources := r.calculateOptimalResources(*avgMetrics)

			if r.needsAdjustment(container.Resources, newResources) {
				updates = append(updates, ResourceUpdate{
					Namespace:     deploy.Namespace,
					Name:          deploy.Name,
					ResourceType:  "Deployment",
					ContainerName: container.Name,
					OldResources:  container.Resources,
					NewResources:  newResources,
					Method:        "rolling-update",
					Reason:        r.getAdjustmentReason(container.Resources, newResources),
				})

				// Update the deployment spec for later application
				deploy.Spec.Template.Spec.Containers[i].Resources = newResources
			}
		}
	}

	return updates, nil
}

// analyzeStatefulSets analyzes all statefulsets for resource optimization
func (r *AdaptiveRightSizer) analyzeStatefulSets(ctx context.Context) ([]ResourceUpdate, error) {
	var stsList appsv1.StatefulSetList
	if err := r.Client.List(ctx, &stsList); err != nil {
		return nil, err
	}

	updates := []ResourceUpdate{}

	for _, sts := range stsList.Items {
		if r.isSystemWorkload(sts.Namespace, sts.Name) {
			continue
		}

		// Get pods for this statefulset
		pods, err := r.getPodsForWorkload(ctx, sts.Namespace, sts.Spec.Selector.MatchLabels)
		if err != nil {
			continue
		}

		if len(pods) == 0 {
			continue
		}

		// Calculate average metrics
		avgMetrics := r.calculateAverageMetrics(pods)
		if avgMetrics == nil {
			continue
		}

		// Check each container
		for i, container := range sts.Spec.Template.Spec.Containers {
			newResources := r.calculateOptimalResources(*avgMetrics)

			if r.needsAdjustment(container.Resources, newResources) {
				updates = append(updates, ResourceUpdate{
					Namespace:     sts.Namespace,
					Name:          sts.Name,
					ResourceType:  "StatefulSet",
					ContainerName: container.Name,
					OldResources:  container.Resources,
					NewResources:  newResources,
					Method:        "rolling-update",
					Reason:        r.getAdjustmentReason(container.Resources, newResources),
				})

				// Update the statefulset spec for later application
				sts.Spec.Template.Spec.Containers[i].Resources = newResources
			}
		}
	}

	return updates, nil
}

// analyzeStandalonePods analyzes standalone pods (not managed by controllers)
func (r *AdaptiveRightSizer) analyzeStandalonePods(ctx context.Context) ([]ResourceUpdate, error) {
	var podList corev1.PodList
	if err := r.Client.List(ctx, &podList); err != nil {
		return nil, err
	}

	updates := []ResourceUpdate{}

	for _, pod := range podList.Items {
		// Skip if managed by a controller
		if len(pod.OwnerReferences) > 0 {
			continue
		}

		if r.isSystemWorkload(pod.Namespace, pod.Name) {
			continue
		}

		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		// Get metrics for this pod
		// Get current pod metrics
		metrics, err := r.MetricsProvider.FetchPodMetrics(pod.Namespace, pod.Name)
		if err != nil {
			logger.Debug("No metrics for pod %s/%s", pod.Namespace, pod.Name)
			continue
		}

		// Check each container
		for _, container := range pod.Spec.Containers {
			newResources := r.calculateOptimalResources(metrics)

			if r.needsAdjustment(container.Resources, newResources) {
				updates = append(updates, ResourceUpdate{
					Namespace:     pod.Namespace,
					Name:          pod.Name,
					ResourceType:  "Pod",
					ContainerName: container.Name,
					OldResources:  container.Resources,
					NewResources:  newResources,
					Method:        "in-place",
					Reason:        r.getAdjustmentReason(container.Resources, newResources),
				})
			}
		}
	}

	return updates, nil
}

// applyUpdates applies the calculated resource updates
func (r *AdaptiveRightSizer) applyUpdates(ctx context.Context, updates []ResourceUpdate) {
	if len(updates) == 0 {
		return
	}

	log.Printf("📊 Found %d resources needing adjustment", len(updates))

	for _, update := range updates {
		if r.DryRun {
			r.logUpdate(update, true)
			continue
		}

		r.logUpdate(update, false)

		switch update.ResourceType {
		case "Deployment":
			if err := r.updateDeployment(ctx, update); err != nil {
				log.Printf("Error updating deployment %s/%s: %v", update.Namespace, update.Name, err)
			}
		case "StatefulSet":
			if err := r.updateStatefulSet(ctx, update); err != nil {
				log.Printf("Error updating statefulset %s/%s: %v", update.Namespace, update.Name, err)
			}
		case "Pod":
			if r.InPlaceEnabled {
				if err := r.updatePodInPlace(ctx, update); err != nil {
					log.Printf("Error updating pod %s/%s: %v", update.Namespace, update.Name, err)
				}
			}
		}
	}
}

// updateDeployment updates a deployment's resources
func (r *AdaptiveRightSizer) updateDeployment(ctx context.Context, update ResourceUpdate) error {
	var deploy appsv1.Deployment
	if err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: update.Namespace,
		Name:      update.Name,
	}, &deploy); err != nil {
		return err
	}

	// Update container resources
	for i := range deploy.Spec.Template.Spec.Containers {
		if deploy.Spec.Template.Spec.Containers[i].Name == update.ContainerName {
			deploy.Spec.Template.Spec.Containers[i].Resources = update.NewResources
			break
		}
	}

	// Add annotation
	if deploy.Spec.Template.Annotations == nil {
		deploy.Spec.Template.Annotations = make(map[string]string)
	}
	deploy.Spec.Template.Annotations["right-sizer/last-update"] = time.Now().Format(time.RFC3339)
	deploy.Spec.Template.Annotations["right-sizer/reason"] = update.Reason

	return r.Client.Update(ctx, &deploy)
}

// updateStatefulSet updates a statefulset's resources
func (r *AdaptiveRightSizer) updateStatefulSet(ctx context.Context, update ResourceUpdate) error {
	var sts appsv1.StatefulSet
	if err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: update.Namespace,
		Name:      update.Name,
	}, &sts); err != nil {
		return err
	}

	// Update container resources
	for i := range sts.Spec.Template.Spec.Containers {
		if sts.Spec.Template.Spec.Containers[i].Name == update.ContainerName {
			sts.Spec.Template.Spec.Containers[i].Resources = update.NewResources
			break
		}
	}

	// Add annotation
	if sts.Spec.Template.Annotations == nil {
		sts.Spec.Template.Annotations = make(map[string]string)
	}
	sts.Spec.Template.Annotations["right-sizer/last-update"] = time.Now().Format(time.RFC3339)
	sts.Spec.Template.Annotations["right-sizer/reason"] = update.Reason

	return r.Client.Update(ctx, &sts)
}

// updatePodInPlace attempts to update pod resources in-place
func (r *AdaptiveRightSizer) updatePodInPlace(ctx context.Context, update ResourceUpdate) error {
	// This would use the resize subresource when available
	// For now, we'll just annotate the pod
	var pod corev1.Pod
	if err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: update.Namespace,
		Name:      update.Name,
	}, &pod); err != nil {
		return err
	}

	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}

	recommendationData, _ := json.Marshal(map[string]interface{}{
		"container":    update.ContainerName,
		"newResources": update.NewResources,
		"reason":       update.Reason,
		"timestamp":    time.Now().Format(time.RFC3339),
	})

	pod.Annotations["right-sizer/recommendation"] = string(recommendationData)

	return r.Client.Update(ctx, &pod)
}

// Helper functions

func (r *AdaptiveRightSizer) getPodsForWorkload(ctx context.Context, namespace string, labels map[string]string) ([]corev1.Pod, error) {
	var podList corev1.PodList
	if err := r.Client.List(ctx, &podList,
		client.InNamespace(namespace),
		client.MatchingLabels(labels)); err != nil {
		return nil, err
	}

	runningPods := []corev1.Pod{}
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodRunning {
			runningPods = append(runningPods, pod)
		}
	}
	return runningPods, nil
}

func (r *AdaptiveRightSizer) calculateAverageMetrics(pods []corev1.Pod) *metrics.Metrics {
	if len(pods) == 0 {
		return nil
	}

	totalCPU := 0.0
	totalMem := 0.0
	validPods := 0

	for _, pod := range pods {
		m, err := r.MetricsProvider.FetchPodMetrics(pod.Namespace, pod.Name)
		if err != nil {
			continue
		}
		totalCPU += m.CPUMilli
		totalMem += m.MemMB
		validPods++
	}

	if validPods == 0 {
		return nil
	}

	return &metrics.Metrics{
		CPUMilli: totalCPU / float64(validPods),
		MemMB:    totalMem / float64(validPods),
	}
}

func (r *AdaptiveRightSizer) calculateOptimalResources(usage metrics.Metrics) corev1.ResourceRequirements {
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

func (r *AdaptiveRightSizer) needsAdjustment(current, new corev1.ResourceRequirements) bool {
	// Get current values
	currentCPU := current.Requests[corev1.ResourceCPU]
	currentMem := current.Requests[corev1.ResourceMemory]
	newCPU := new.Requests[corev1.ResourceCPU]
	newMem := new.Requests[corev1.ResourceMemory]

	// Skip if not set
	if currentCPU.IsZero() || currentMem.IsZero() {
		return true
	}

	// Calculate percentage difference
	cpuDiff := float64(newCPU.MilliValue()-currentCPU.MilliValue()) / float64(currentCPU.MilliValue()) * 100
	memDiff := float64(newMem.Value()-currentMem.Value()) / float64(currentMem.Value()) * 100

	// Adjust if difference > 15%
	threshold := 15.0
	return (cpuDiff > threshold || cpuDiff < -threshold) ||
		(memDiff > threshold || memDiff < -threshold)
}

func (r *AdaptiveRightSizer) getAdjustmentReason(current, new corev1.ResourceRequirements) string {
	currentCPU := current.Requests[corev1.ResourceCPU]
	currentMem := current.Requests[corev1.ResourceMemory]
	newCPU := new.Requests[corev1.ResourceCPU]
	newMem := new.Requests[corev1.ResourceMemory]

	cpuChange := "no change"
	if newCPU.MilliValue() > currentCPU.MilliValue() {
		cpuChange = fmt.Sprintf("increase from %s to %s", currentCPU.String(), newCPU.String())
	} else if newCPU.MilliValue() < currentCPU.MilliValue() {
		cpuChange = fmt.Sprintf("decrease from %s to %s", currentCPU.String(), newCPU.String())
	}

	memChange := "no change"
	if newMem.Value() > currentMem.Value() {
		memChange = fmt.Sprintf("increase from %s to %s", currentMem.String(), newMem.String())
	} else if newMem.Value() < currentMem.Value() {
		memChange = fmt.Sprintf("decrease from %s to %s", currentMem.String(), newMem.String())
	}

	return fmt.Sprintf("CPU %s, Memory %s", cpuChange, memChange)
}

func (r *AdaptiveRightSizer) isSystemWorkload(namespace, name string) bool {
	systemNamespaces := []string{"kube-system", "kube-public", "kube-node-lease"}
	for _, ns := range systemNamespaces {
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

func (r *AdaptiveRightSizer) logUpdate(update ResourceUpdate, dryRun bool) {
	mode := ""
	if dryRun {
		mode = "[DRY RUN] "
	}

	cpuReq := update.NewResources.Requests[corev1.ResourceCPU]
	memReq := update.NewResources.Requests[corev1.ResourceMemory]
	oldCpuReq := update.OldResources.Requests[corev1.ResourceCPU]
	oldMemReq := update.OldResources.Requests[corev1.ResourceMemory]

	log.Printf("%s%s %s/%s/%s - CPU: %s→%s, Memory: %s→%s (%s)",
		mode,
		update.ResourceType,
		update.Namespace,
		update.Name,
		update.ContainerName,
		oldCpuReq.String(),
		cpuReq.String(),
		oldMemReq.String(),
		memReq.String(),
		update.Method,
	)
}

// SetupAdaptiveRightSizer creates and starts the adaptive rightsizer
func SetupAdaptiveRightSizer(mgr manager.Manager, provider metrics.Provider, dryRun bool) error {
	cfg := config.Get()
	rightsizer := &AdaptiveRightSizer{
		Client:          mgr.GetClient(),
		MetricsProvider: provider,
		Interval:        cfg.ResizeInterval,
		DryRun:          dryRun,
	}

	// Start the rightsizer
	go func() {
		if err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
			return rightsizer.Start(ctx)
		})); err != nil {
			log.Printf("Failed to add adaptive rightsizer to manager: %v", err)
		}
	}()

	return nil
}

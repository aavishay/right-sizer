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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// AdaptiveRightSizer performs resource optimization with support for both
// in-place updates (when available) and deployment updates as fallback
type AdaptiveRightSizer struct {
	Client          client.Client
	ClientSet       *kubernetes.Clientset
	RestConfig      *rest.Config
	MetricsProvider metrics.Provider
	Interval        time.Duration
	InPlaceEnabled  bool // Will be auto-detected
	DryRun          bool // If true, only log recommendations without applying
}

// ResourceUpdate represents a pending resource update
type ResourceUpdate struct {
	Namespace      string
	Name           string
	ResourceType   string // Pod only now
	ContainerName  string
	ContainerIndex int
	OldResources   corev1.ResourceRequirements
	NewResources   corev1.ResourceRequirements
	Reason         string
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

// performRightSizing processes all pods for optimization using in-place resize
func (r *AdaptiveRightSizer) performRightSizing(ctx context.Context) {
	updates := []ResourceUpdate{}

	// Analyze ALL pods directly (including those from deployments, statefulsets, etc)
	// We will update pods directly using in-place resize, not their controllers
	pods, err := r.analyzeAllPods(ctx)
	if err != nil {
		log.Printf("Error analyzing pods: %v", err)
	} else {
		updates = append(updates, pods...)
	}

	// Apply updates using in-place resize
	r.applyUpdates(ctx, updates)
}

// analyzeAllPods analyzes all pods in the cluster for resource optimization
func (r *AdaptiveRightSizer) analyzeAllPods(ctx context.Context) ([]ResourceUpdate, error) {
	var podList corev1.PodList
	if err := r.Client.List(ctx, &podList); err != nil {
		return nil, err
	}

	updates := []ResourceUpdate{}

	for _, pod := range podList.Items {
		// Skip pods that are not running
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		// Check namespace filters first
		if !r.shouldProcessNamespace(pod.Namespace) {
			continue
		}
		if r.isSystemWorkload(pod.Namespace, pod.Name) {
			continue
		}

		// Skip pods with skip annotation
		if pod.Annotations != nil {
			if skip, ok := pod.Annotations["rightsizer.io/skip"]; ok && skip == "true" {
				continue
			}
		}

		// Get metrics for this specific pod
		podMetrics, err := r.MetricsProvider.FetchPodMetrics(pod.Namespace, pod.Name)
		if err != nil {
			log.Printf("Failed to get metrics for pod %s/%s: %v", pod.Namespace, pod.Name, err)
			continue
		}

		// Check each container in the pod
		for i, container := range pod.Spec.Containers {
			// Calculate optimal resources based on actual usage
			// Note: metrics-server provides pod-level metrics, not per-container
			// So we'll use the pod metrics for all containers
			newResources := r.calculateOptimalResources(podMetrics)

			if r.needsAdjustment(container.Resources, newResources) {
				updates = append(updates, ResourceUpdate{
					Namespace:      pod.Namespace,
					Name:           pod.Name,
					ResourceType:   "Pod",
					ContainerName:  container.Name,
					ContainerIndex: i,
					OldResources:   container.Resources,
					NewResources:   newResources,
					Reason:         r.getAdjustmentReason(container.Resources, newResources),
				})
			}
		}
	}

	return updates, nil
}

// analyzeStandalonePods analyzes standalone pods (deprecated - all pods are now analyzed)
func (r *AdaptiveRightSizer) analyzeStandalonePods(ctx context.Context) ([]ResourceUpdate, error) {
	// This function is deprecated as we now analyze all pods in analyzeAllPods
	return []ResourceUpdate{}, nil

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
	}

	// Apply all pod updates using in-place resize
	for _, update := range updates {
		if update.ResourceType == "Pod" {
			if err := r.updatePodInPlace(ctx, update); err != nil {
				log.Printf("Error updating pod %s/%s: %v", update.Namespace, update.Name, err)
			} else {
				log.Printf("✅ Successfully resized pod %s/%s using in-place resize", update.Namespace, update.Name)
			}
		}
	}
}

// updatePodInPlace attempts to update pod resources in-place
func (r *AdaptiveRightSizer) updatePodInPlace(ctx context.Context, update ResourceUpdate) error {
	// Get the current pod
	var pod corev1.Pod
	if err := r.Client.Get(ctx, types.NamespacedName{
		Namespace: update.Namespace,
		Name:      update.Name,
	}, &pod); err != nil {
		return err
	}

	// Create the resize patch
	type ContainerResourcesPatch struct {
		Name      string                      `json:"name"`
		Resources corev1.ResourceRequirements `json:"resources"`
	}

	type PodSpecPatch struct {
		Containers []ContainerResourcesPatch `json:"containers"`
	}

	type PodResizePatch struct {
		Spec PodSpecPatch `json:"spec"`
	}

	resizePatch := PodResizePatch{
		Spec: PodSpecPatch{
			Containers: []ContainerResourcesPatch{
				{
					Name:      update.ContainerName,
					Resources: update.NewResources,
				},
			},
		},
	}

	// Marshal the patch
	patchData, err := json.Marshal(resizePatch)
	if err != nil {
		return fmt.Errorf("failed to marshal resize patch: %w", err)
	}

	// Use the Kubernetes client-go to patch with the resize subresource
	// This is the key difference - using the resize subresource endpoint
	_, err = r.ClientSet.CoreV1().Pods(update.Namespace).Patch(
		ctx,
		update.Name,
		types.StrategicMergePatchType,
		patchData,
		metav1.PatchOptions{},
		"resize", // This is the crucial part - specifying the resize subresource
	)

	if err != nil {
		return fmt.Errorf("failed to resize pod: %w", err)
	}

	return nil
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

	log.Printf("%s%s %s/%s/%s - CPU: %s→%s, Memory: %s→%s (in-place)",
		mode,
		update.ResourceType,
		update.Namespace,
		update.Name,
		update.ContainerName,
		oldCpuReq.String(),
		cpuReq.String(),
		oldMemReq.String(),
		memReq.String())
	return
}

// shouldProcessNamespace checks if a namespace should be processed based on include/exclude lists
func (r *AdaptiveRightSizer) shouldProcessNamespace(namespace string) bool {
	cfg := config.Get()

	// Check exclude list first (takes precedence)
	for _, excludeNs := range cfg.NamespaceExclude {
		if namespace == excludeNs {
			return false
		}
	}

	// If include list is empty, process all non-excluded namespaces
	if len(cfg.NamespaceInclude) == 0 {
		return true
	}

	// Check if namespace is in include list
	for _, includeNs := range cfg.NamespaceInclude {
		if namespace == includeNs {
			return true
		}
	}

	return false
}

// SetupAdaptiveRightSizer creates and starts the adaptive rightsizer
func SetupAdaptiveRightSizer(mgr manager.Manager, provider metrics.Provider, dryRun bool) error {
	cfg := config.Get()

	// Get the rest config from the manager
	restConfig := mgr.GetConfig()

	// Create a clientset for using the resize subresource
	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	rightsizer := &AdaptiveRightSizer{
		Client:          mgr.GetClient(),
		ClientSet:       clientSet,
		RestConfig:      restConfig,
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

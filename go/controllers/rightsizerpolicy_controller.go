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
	"fmt"
	"sort"
	"time"

	"right-sizer/api/v1alpha1"
	"right-sizer/config"
	"right-sizer/logger"
	"right-sizer/metrics"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// RightSizerPolicyReconciler reconciles a RightSizerPolicy object
type RightSizerPolicyReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	MetricsProvider metrics.Provider
	Config          *config.Config
}

// +kubebuilder:rbac:groups=rightsizer.io,resources=rightsizerpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rightsizer.io,resources=rightsizerpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rightsizer.io,resources=rightsizerpolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments;statefulsets;daemonsets;replicasets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs;cronjobs,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *RightSizerPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logger.GetLogger()
	log.Info("Reconciling RightSizerPolicy", "name", req.Name, "namespace", req.Namespace)

	// Fetch the RightSizerPolicy instance
	policy := &v1alpha1.RightSizerPolicy{}
	err := r.Get(ctx, req.NamespacedName, policy)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected.
			log.Info("RightSizerPolicy resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error("Failed to get RightSizerPolicy: %v", err)
		return ctrl.Result{}, err
	}

	// Check if the policy is enabled
	if !policy.Spec.Enabled {
		log.Info("RightSizerPolicy is disabled, skipping reconciliation", "name", policy.Name)
		return r.updatePolicyStatus(ctx, policy, "Disabled", "Policy is disabled")
	}

	// Initialize status if needed
	if policy.Status.Phase == "" {
		policy.Status.Phase = "Pending"
		policy.Status.ResourcesAffected = 0
		policy.Status.ResourcesResized = 0
		if err := r.Status().Update(ctx, policy); err != nil {
			log.Error("Failed to update initial status: %v", err)
			return ctrl.Result{}, err
		}
	}

	// Check if this policy should be processed based on global namespace filters
	if !r.shouldProcessPolicy(ctx, policy) {
		log.Info("Policy skipped due to namespace filters", "name", policy.Name)
		return r.updatePolicyStatus(ctx, policy, "Skipped", "Policy namespace not included in global configuration")
	}

	// Process the policy
	result, err := r.processPolicyTargets(ctx, policy)
	if err != nil {
		log.Error("Failed to process policy targets: %v", err)
		return r.updatePolicyStatus(ctx, policy, "Failed", fmt.Sprintf("Error: %v", err))
	}

	// Update status with results
	policy.Status.Phase = "Active"
	policy.Status.LastAppliedTime = &metav1.Time{Time: time.Now()}
	policy.Status.ResourcesAffected = result.affected
	policy.Status.ResourcesResized = result.resized
	policy.Status.ObservedGeneration = policy.Generation
	policy.Status.Message = fmt.Sprintf("Successfully processed %d resources, resized %d", result.affected, result.resized)

	// Calculate savings if applicable
	if result.cpuSaved > 0 || result.memorySaved > 0 {
		policy.Status.TotalSavings = v1alpha1.ResourceSavings{
			CPUSaved:    result.cpuSaved,
			MemorySaved: result.memorySaved,
		}
	}

	if err := r.Status().Update(ctx, policy); err != nil {
		log.Error("Failed to update policy status: %v", err)
		return ctrl.Result{}, err
	}

	// Requeue based on schedule
	requeueAfter := r.getRequeueInterval(policy)
	log.Info("Successfully reconciled RightSizerPolicy", "name", policy.Name, "requeueAfter", requeueAfter)
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

type processResult struct {
	affected    int32
	resized     int32
	cpuSaved    int64
	memorySaved int64
}

// processPolicyTargets processes all resources targeted by the policy
func (r *RightSizerPolicyReconciler) processPolicyTargets(ctx context.Context, policy *v1alpha1.RightSizerPolicy) (*processResult, error) {
	result := &processResult{}
	targetRef := policy.Spec.TargetRef

	// Get all matching resources
	resources, err := r.getMatchingResources(ctx, targetRef)
	if err != nil {
		return nil, err
	}

	result.affected = int32(len(resources))

	// Process each resource
	for _, res := range resources {
		resized, cpuSaved, memorySaved, err := r.processResource(ctx, policy, res)
		if err != nil {
			logger.Error("Failed to process resource %s/%s: %v", res.GetNamespace(), res.GetName(), err)
			continue
		}
		if resized {
			result.resized++
			result.cpuSaved += cpuSaved
			result.memorySaved += memorySaved
		}
	}

	return result, nil
}

// getMatchingResources returns all resources that match the target reference
func (r *RightSizerPolicyReconciler) getMatchingResources(ctx context.Context, targetRef v1alpha1.TargetReference) ([]client.Object, error) {
	var resources []client.Object

	// Build label selector
	var selector labels.Selector
	if targetRef.LabelSelector != nil {
		var err error
		selector, err = metav1.LabelSelectorAsSelector(targetRef.LabelSelector)
		if err != nil {
			return nil, err
		}
	} else {
		selector = labels.Everything()
	}

	// Get namespaces to search
	namespaces := r.getTargetNamespaces(ctx, targetRef)

	// Get resources based on kind
	for _, ns := range namespaces {
		switch targetRef.Kind {
		case "Deployment":
			deployments := &appsv1.DeploymentList{}
			opts := []client.ListOption{
				client.InNamespace(ns),
				client.MatchingLabelsSelector{Selector: selector},
			}
			if err := r.List(ctx, deployments, opts...); err != nil {
				return nil, err
			}
			for i := range deployments.Items {
				if r.matchesTargetRef(&deployments.Items[i], targetRef) {
					resources = append(resources, &deployments.Items[i])
				}
			}

		case "StatefulSet":
			statefulsets := &appsv1.StatefulSetList{}
			opts := []client.ListOption{
				client.InNamespace(ns),
				client.MatchingLabelsSelector{Selector: selector},
			}
			if err := r.List(ctx, statefulsets, opts...); err != nil {
				return nil, err
			}
			for i := range statefulsets.Items {
				if r.matchesTargetRef(&statefulsets.Items[i], targetRef) {
					resources = append(resources, &statefulsets.Items[i])
				}
			}

		case "DaemonSet":
			daemonsets := &appsv1.DaemonSetList{}
			opts := []client.ListOption{
				client.InNamespace(ns),
				client.MatchingLabelsSelector{Selector: selector},
			}
			if err := r.List(ctx, daemonsets, opts...); err != nil {
				return nil, err
			}
			for i := range daemonsets.Items {
				if r.matchesTargetRef(&daemonsets.Items[i], targetRef) {
					resources = append(resources, &daemonsets.Items[i])
				}
			}

		case "Pod":
			pods := &corev1.PodList{}
			opts := []client.ListOption{
				client.InNamespace(ns),
				client.MatchingLabelsSelector{Selector: selector},
			}
			if err := r.List(ctx, pods, opts...); err != nil {
				return nil, err
			}
			for i := range pods.Items {
				if r.matchesTargetRef(&pods.Items[i], targetRef) {
					resources = append(resources, &pods.Items[i])
				}
			}

		case "Job":
			jobs := &batchv1.JobList{}
			opts := []client.ListOption{
				client.InNamespace(ns),
				client.MatchingLabelsSelector{Selector: selector},
			}
			if err := r.List(ctx, jobs, opts...); err != nil {
				return nil, err
			}
			for i := range jobs.Items {
				if r.matchesTargetRef(&jobs.Items[i], targetRef) {
					resources = append(resources, &jobs.Items[i])
				}
			}

		case "CronJob":
			cronjobs := &batchv1.CronJobList{}
			opts := []client.ListOption{
				client.InNamespace(ns),
				client.MatchingLabelsSelector{Selector: selector},
			}
			if err := r.List(ctx, cronjobs, opts...); err != nil {
				return nil, err
			}
			for i := range cronjobs.Items {
				if r.matchesTargetRef(&cronjobs.Items[i], targetRef) {
					resources = append(resources, &cronjobs.Items[i])
				}
			}
		}
	}

	return resources, nil
}

// getTargetNamespaces returns the list of namespaces to search
func (r *RightSizerPolicyReconciler) getTargetNamespaces(ctx context.Context, targetRef v1alpha1.TargetReference) []string {
	var namespaces []string

	// Start with namespaces from policy targetRef or global config
	if len(targetRef.Namespaces) > 0 {
		// Use specified namespaces from policy
		namespaces = targetRef.Namespaces
	} else if len(r.Config.NamespaceInclude) > 0 {
		// Use global namespace include list
		namespaces = r.Config.NamespaceInclude
	} else {
		// Get all namespaces and filter
		nsList := &corev1.NamespaceList{}
		if err := r.List(ctx, nsList); err != nil {
			logger.Error("Failed to list namespaces: %v", err)
			return []string{}
		}

		for _, ns := range nsList.Items {
			namespaces = append(namespaces, ns.Name)
		}
	}

	// Apply exclusions - merge policy and global exclusions
	excludeSet := make(map[string]bool)

	// Add policy-level exclusions
	for _, excludeNs := range targetRef.ExcludeNamespaces {
		excludeSet[excludeNs] = true
	}

	// Add global exclusions (these always apply)
	for _, excludeNs := range r.Config.NamespaceExclude {
		excludeSet[excludeNs] = true
	}

	// Filter out excluded namespaces
	filteredNamespaces := []string{}
	for _, ns := range namespaces {
		if !excludeSet[ns] {
			filteredNamespaces = append(filteredNamespaces, ns)
		}
	}

	return filteredNamespaces
}

// matchesTargetRef checks if a resource matches the target reference criteria
func (r *RightSizerPolicyReconciler) matchesTargetRef(obj client.Object, targetRef v1alpha1.TargetReference) bool {
	// Check name inclusion/exclusion
	name := obj.GetName()
	if len(targetRef.Names) > 0 {
		found := false
		for _, n := range targetRef.Names {
			if n == name {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	for _, excludeName := range targetRef.ExcludeNames {
		if excludeName == name {
			return false
		}
	}

	// Check annotation selector
	if len(targetRef.AnnotationSelector) > 0 {
		annotations := obj.GetAnnotations()
		for key, value := range targetRef.AnnotationSelector {
			if annotations[key] != value {
				return false
			}
		}
	}

	return true
}

// processResource processes a single resource according to the policy
func (r *RightSizerPolicyReconciler) processResource(ctx context.Context, policy *v1alpha1.RightSizerPolicy, obj client.Object) (bool, int64, int64, error) {
	// Skip if dry-run mode
	if policy.Spec.DryRun {
		logger.Info("Dry-run mode: would resize %s/%s", obj.GetNamespace(), obj.GetName())
		return false, 0, 0, nil
	}

	// Get current resource specifications
	var podTemplate *corev1.PodTemplateSpec
	switch res := obj.(type) {
	case *appsv1.Deployment:
		podTemplate = &res.Spec.Template
	case *appsv1.StatefulSet:
		podTemplate = &res.Spec.Template
	case *appsv1.DaemonSet:
		podTemplate = &res.Spec.Template
	case *batchv1.Job:
		podTemplate = &res.Spec.Template
	case *batchv1.CronJob:
		podTemplate = &res.Spec.JobTemplate.Spec.Template
	case *corev1.Pod:
		// For pods, we need to use in-place resize if available
		return r.processPod(ctx, policy, res)
	default:
		return false, 0, 0, fmt.Errorf("unsupported resource type: %T", res)
	}

	// Calculate new resources based on policy
	newResources, cpuSaved, memorySaved, err := r.calculateNewResources(ctx, policy, obj, podTemplate)
	if err != nil {
		return false, 0, 0, err
	}

	// Check if resources need to be updated
	if !r.needsUpdate(podTemplate, newResources) {
		return false, 0, 0, nil
	}

	// Apply resource changes
	for i := range podTemplate.Spec.Containers {
		if containerResources, ok := newResources[podTemplate.Spec.Containers[i].Name]; ok {
			podTemplate.Spec.Containers[i].Resources = containerResources
		}
	}

	// Add annotations
	if policy.Spec.ResourceAnnotations != nil {
		if podTemplate.Annotations == nil {
			podTemplate.Annotations = make(map[string]string)
		}
		for k, v := range policy.Spec.ResourceAnnotations {
			podTemplate.Annotations[k] = v
		}
		podTemplate.Annotations["rightsizer.io/last-resized"] = time.Now().Format(time.RFC3339)
		podTemplate.Annotations["rightsizer.io/policy"] = policy.Name
	}

	// Update the resource
	if err := r.Update(ctx, obj); err != nil {
		return false, 0, 0, err
	}

	// Create an event
	r.createEvent(ctx, obj, policy, "ResourceResized", fmt.Sprintf("Resized by policy %s", policy.Name))

	return true, cpuSaved, memorySaved, nil
}

// processPod handles in-place pod resizing
func (r *RightSizerPolicyReconciler) processPod(ctx context.Context, policy *v1alpha1.RightSizerPolicy, pod *corev1.Pod) (bool, int64, int64, error) {
	// Check if pod supports in-place resize
	if !r.supportsInPlaceResize(pod) {
		return false, 0, 0, nil
	}

	// Calculate new resources
	newResources := make(map[string]corev1.ResourceRequirements)
	var cpuSaved, memorySaved int64

	for _, container := range pod.Spec.Containers {
		usage, err := r.MetricsProvider.FetchPodMetrics(pod.Namespace, pod.Name)
		if err != nil {
			logger.Warn("Failed to fetch metrics for pod %s/%s: %v", pod.Namespace, pod.Name, err)
			continue
		}

		newReqs := r.calculateOptimalResourcesFromPolicy(policy, usage)
		newResources[container.Name] = newReqs

		// Calculate savings
		if container.Resources.Requests != nil {
			oldCPU := container.Resources.Requests.Cpu().MilliValue()
			newCPU := newReqs.Requests.Cpu().MilliValue()
			cpuSaved += oldCPU - newCPU

			oldMem := container.Resources.Requests.Memory().Value() / (1024 * 1024)
			newMem := newReqs.Requests.Memory().Value() / (1024 * 1024)
			memorySaved += oldMem - newMem
		}
	}

	// Apply the resize
	pod.Spec.Containers[0].Resources = newResources[pod.Spec.Containers[0].Name]

	if err := r.Update(ctx, pod); err != nil {
		return false, 0, 0, err
	}

	return true, cpuSaved, memorySaved, nil
}

// calculateNewResources calculates new resource requirements based on policy
func (r *RightSizerPolicyReconciler) calculateNewResources(ctx context.Context, policy *v1alpha1.RightSizerPolicy, obj client.Object, podTemplate *corev1.PodTemplateSpec) (map[string]corev1.ResourceRequirements, int64, int64, error) {
	newResources := make(map[string]corev1.ResourceRequirements)
	var totalCPUSaved, totalMemorySaved int64

	// Get pods for this workload to fetch metrics
	podList := &corev1.PodList{}
	if err := r.List(ctx, podList, client.InNamespace(obj.GetNamespace()), client.MatchingLabels(podTemplate.Labels)); err != nil {
		return nil, 0, 0, err
	}

	if len(podList.Items) == 0 {
		return newResources, 0, 0, nil
	}

	// Aggregate metrics from all pods
	var totalCPU, totalMem float64
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
		return newResources, 0, 0, nil
	}

	// Calculate average usage
	avgUsage := metrics.Metrics{
		CPUMilli: totalCPU / float64(validPods),
		MemMB:    totalMem / float64(validPods),
	}

	// Calculate new resources for each container
	for _, container := range podTemplate.Spec.Containers {
		newReqs := r.calculateOptimalResourcesFromPolicy(policy, avgUsage)
		newResources[container.Name] = newReqs

		// Calculate savings
		if container.Resources.Requests != nil {
			oldCPU := container.Resources.Requests.Cpu().MilliValue()
			newCPU := newReqs.Requests.Cpu().MilliValue()
			totalCPUSaved += oldCPU - newCPU

			oldMem := container.Resources.Requests.Memory().Value() / (1024 * 1024)
			newMem := newReqs.Requests.Memory().Value() / (1024 * 1024)
			totalMemorySaved += oldMem - newMem
		}
	}

	return newResources, totalCPUSaved, totalMemorySaved, nil
}

// calculateOptimalResourcesFromPolicy calculates resources based on policy settings
func (r *RightSizerPolicyReconciler) calculateOptimalResourcesFromPolicy(policy *v1alpha1.RightSizerPolicy, usage metrics.Metrics) corev1.ResourceRequirements {
	strategy := policy.Spec.ResourceStrategy

	// Get multipliers and additions from policy or use defaults
	cpuRequestMultiplier := r.Config.CPURequestMultiplier
	memoryRequestMultiplier := r.Config.MemoryRequestMultiplier
	cpuRequestAddition := r.Config.CPURequestAddition
	memoryRequestAddition := r.Config.MemoryRequestAddition
	cpuLimitMultiplier := r.Config.CPULimitMultiplier
	memoryLimitMultiplier := r.Config.MemoryLimitMultiplier
	cpuLimitAddition := r.Config.CPULimitAddition
	memoryLimitAddition := r.Config.MemoryLimitAddition

	// Override with policy-specific values if provided
	if strategy.CPU.RequestMultiplier != nil {
		cpuRequestMultiplier = *strategy.CPU.RequestMultiplier
	}
	if strategy.CPU.RequestAddition != nil {
		cpuRequestAddition = *strategy.CPU.RequestAddition
	}
	if strategy.CPU.LimitMultiplier != nil {
		cpuLimitMultiplier = *strategy.CPU.LimitMultiplier
	}
	if strategy.CPU.LimitAddition != nil {
		cpuLimitAddition = *strategy.CPU.LimitAddition
	}

	if strategy.Memory.RequestMultiplier != nil {
		memoryRequestMultiplier = *strategy.Memory.RequestMultiplier
	}
	if strategy.Memory.RequestAddition != nil {
		memoryRequestAddition = *strategy.Memory.RequestAddition
	}
	if strategy.Memory.LimitMultiplier != nil {
		memoryLimitMultiplier = *strategy.Memory.LimitMultiplier
	}
	if strategy.Memory.LimitAddition != nil {
		memoryLimitAddition = *strategy.Memory.LimitAddition
	}

	// Calculate requests with multipliers and additions
	cpuRequest := int64(usage.CPUMilli*cpuRequestMultiplier) + cpuRequestAddition
	memRequest := int64(usage.MemMB*memoryRequestMultiplier) + memoryRequestAddition

	// Apply minimum values
	minCPU := r.Config.MinCPURequest
	minMem := r.Config.MinMemoryRequest
	if strategy.CPU.MinRequest != nil {
		minCPU = *strategy.CPU.MinRequest
	}
	if strategy.Memory.MinRequest != nil {
		minMem = *strategy.Memory.MinRequest
	}

	if cpuRequest < minCPU {
		cpuRequest = minCPU
	}
	if memRequest < minMem {
		memRequest = minMem
	}

	// Calculate limits
	cpuLimit := int64(float64(cpuRequest)*cpuLimitMultiplier) + cpuLimitAddition
	memLimit := int64(float64(memRequest)*memoryLimitMultiplier) + memoryLimitAddition

	// Apply maximum caps
	maxCPU := r.Config.MaxCPULimit
	maxMem := r.Config.MaxMemoryLimit
	if strategy.CPU.MaxLimit != nil {
		maxCPU = *strategy.CPU.MaxLimit
	}
	if strategy.Memory.MaxLimit != nil {
		maxMem = *strategy.Memory.MaxLimit
	}

	if cpuLimit > maxCPU {
		cpuLimit = maxCPU
	}
	if memLimit > maxMem {
		memLimit = maxMem
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

// needsUpdate checks if resources need to be updated
func (r *RightSizerPolicyReconciler) needsUpdate(podTemplate *corev1.PodTemplateSpec, newResources map[string]corev1.ResourceRequirements) bool {
	for _, container := range podTemplate.Spec.Containers {
		if newReqs, ok := newResources[container.Name]; ok {
			if !resourcesEqual(container.Resources, newReqs) {
				return true
			}
		}
	}
	return false
}

// resourcesEqual compares two resource requirements
func resourcesEqual(a, b corev1.ResourceRequirements) bool {
	if !a.Requests.Cpu().Equal(*b.Requests.Cpu()) {
		return false
	}
	if !a.Requests.Memory().Equal(*b.Requests.Memory()) {
		return false
	}
	if !a.Limits.Cpu().Equal(*b.Limits.Cpu()) {
		return false
	}
	if !a.Limits.Memory().Equal(*b.Limits.Memory()) {
		return false
	}
	return true
}

// supportsInPlaceResize checks if a pod supports in-place resize
func (r *RightSizerPolicyReconciler) supportsInPlaceResize(pod *corev1.Pod) bool {
	// Check if pod has resize policy defined
	for _, container := range pod.Spec.Containers {
		if container.ResizePolicy != nil && len(container.ResizePolicy) > 0 {
			return true
		}
	}
	return false
}

// getRequeueInterval determines the requeue interval based on policy schedule
func (r *RightSizerPolicyReconciler) getRequeueInterval(policy *v1alpha1.RightSizerPolicy) time.Duration {
	if policy.Spec.Schedule.Interval != "" {
		duration, err := time.ParseDuration(policy.Spec.Schedule.Interval)
		if err == nil {
			return duration
		}
	}
	// Default to 1 minute
	return time.Minute
}

// updatePolicyStatus updates the policy status
func (r *RightSizerPolicyReconciler) updatePolicyStatus(ctx context.Context, policy *v1alpha1.RightSizerPolicy, phase, message string) (ctrl.Result, error) {
	policy.Status.Phase = phase
	policy.Status.Message = message
	policy.Status.LastEvaluationTime = &metav1.Time{Time: time.Now()}
	policy.Status.ObservedGeneration = policy.Generation

	if err := r.Status().Update(ctx, policy); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue after default interval
	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

// createEvent creates a Kubernetes event for the resource
func (r *RightSizerPolicyReconciler) createEvent(ctx context.Context, obj client.Object, policy *v1alpha1.RightSizerPolicy, reason, message string) {
	// Implementation would create a Kubernetes event
	// This is simplified for brevity
	logger.Info("Event: %s/%s - %s: %s", obj.GetNamespace(), obj.GetName(), reason, message)
}

// shouldProcessPolicy checks if a policy should be processed based on namespace filters
func (r *RightSizerPolicyReconciler) shouldProcessPolicy(ctx context.Context, policy *v1alpha1.RightSizerPolicy) bool {
	// If policy has specific namespaces, check if any are allowed
	if len(policy.Spec.TargetRef.Namespaces) > 0 {
		targetNamespaces := r.getTargetNamespaces(ctx, policy.Spec.TargetRef)
		return len(targetNamespaces) > 0
	}

	// If policy is in a namespace that's excluded globally, skip it
	for _, excludeNs := range r.Config.NamespaceExclude {
		if policy.Namespace == excludeNs {
			return false
		}
	}

	// If we have a global include list and policy namespace is not in it, skip
	if len(r.Config.NamespaceInclude) > 0 {
		found := false
		for _, includeNs := range r.Config.NamespaceInclude {
			if policy.Namespace == includeNs {
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

// SetupWithManager sets up the controller with the Manager
func (r *RightSizerPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create controller
	c, err := controller.New("rightsizerpolicy-controller", mgr, controller.Options{
		Reconciler: r,
	})
	if err != nil {
		return err
	}

	// Watch RightSizerPolicy resources
	err = c.Watch(&source.Kind{Type: &v1alpha1.RightSizerPolicy{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch target resources and enqueue policies that target them
	// Watch Deployments
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(obj client.Object) []reconcile.Request {
			return r.findPoliciesForResource(obj)
		}),
	})
	if err != nil {
		return err
	}

	// Watch StatefulSets
	err = c.Watch(&source.Kind{Type: &appsv1.StatefulSet{}}, &handler.EnqueueRequestsFromMapFunc{
		ToRequests: handler.ToRequestsFunc(func(obj client.Object) []reconcile.Request {
			return r.findPoliciesForResource(obj)
		}),
	})
	if err != nil {
		return err
	}

	// Watch DaemonSets
	err = c.

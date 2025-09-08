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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"right-sizer/config"
	"right-sizer/metrics"
)

// RightSizer reconciles Pods and updates parent controller resource requests/limits
type RightSizer struct {
	client.Client
	MetricsProvider metrics.Provider
}

// Reconcile is called periodically for each Pod
func (r *RightSizer) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	var pod corev1.Pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		return reconcile.Result{}, client.IgnoreNotFound(err)
	}

	ns := pod.Namespace
	cfg := config.Get()
	if !cfg.IsNamespaceIncluded(ns) {
		return reconcile.Result{}, nil
	}

	owner := getOwnerRef(&pod)
	if owner == nil {
		return reconcile.Result{}, nil
	}

	usage, err := r.MetricsProvider.FetchPodMetrics(pod.Namespace, pod.Name)
	if err != nil {
		return reconcile.Result{}, err
	}

	newResources := calculateResources(usage)

	if err := patchController(r.Client, owner, pod.Namespace, newResources); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: 10 * time.Minute}, nil
}

// SetupRightSizerController registers the controller with the manager
func SetupRightSizerController(mgr manager.Manager, provider metrics.Provider) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(&RightSizer{Client: mgr.GetClient(), MetricsProvider: provider})
}

// getOwnerRef returns the first controller owner reference (Deployment/StatefulSet)
func getOwnerRef(pod *corev1.Pod) *metav1.OwnerReference {
	for _, owner := range pod.OwnerReferences {
		if owner.Controller != nil && *owner.Controller {
			if owner.Kind == "Deployment" || owner.Kind == "StatefulSet" {
				return &owner
			}
		}
	}
	return nil
}

// calculateResources computes new resource requests/limits based on usage
func calculateResources(usage metrics.Metrics) corev1.ResourceRequirements {
	cfg := config.Get()

	cpuRequest := int64(usage.CPUMilli*cfg.CPURequestMultiplier) + cfg.CPURequestAddition
	memRequest := int64(usage.MemMB*cfg.MemoryRequestMultiplier) + cfg.MemoryRequestAddition
	cpuLimit := int64(float64(cpuRequest)*cfg.CPULimitMultiplier) + cfg.CPULimitAddition
	memLimit := int64(float64(memRequest)*cfg.MemoryLimitMultiplier) + cfg.MemoryLimitAddition

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

// patchController updates the parent controller (Deployment/StatefulSet) with new resources
func patchController(c client.Client, owner *metav1.OwnerReference, namespace string, resources corev1.ResourceRequirements) error {
	switch owner.Kind {
	case "Deployment":
		var deploy appsv1.Deployment
		if err := c.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: owner.Name}, &deploy); err != nil {
			return err
		}
		for i := range deploy.Spec.Template.Spec.Containers {
			deploy.Spec.Template.Spec.Containers[i].Resources = resources
		}
		return c.Update(context.TODO(), &deploy)
	case "StatefulSet":
		var sts appsv1.StatefulSet
		if err := c.Get(context.TODO(), client.ObjectKey{Namespace: namespace, Name: owner.Name}, &sts); err != nil {
			return err
		}
		for i := range sts.Spec.Template.Spec.Containers {
			sts.Spec.Template.Spec.Containers[i].Resources = resources
		}
		return c.Update(context.TODO(), &sts)
	default:
		return fmt.Errorf("unsupported owner kind: %s", owner.Kind)
	}
}

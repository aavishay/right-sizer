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

	"right-sizer/api/v1alpha1"
	"right-sizer/config"
	"right-sizer/logger"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// RightSizerConfigReconciler reconciles a RightSizerConfig object
type RightSizerConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *config.Config
}

// +kubebuilder:rbac:groups=rightsizer.io,resources=rightsizerconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rightsizer.io,resources=rightsizerconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=rightsizer.io,resources=rightsizerconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups=rightsizer.io,resources=rightsizerpolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop
func (r *RightSizerConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logger.GetLogger()
	log.Info("Reconciling RightSizerConfig", "name", req.Name)

	// Fetch the RightSizerConfig instance
	rsc := &v1alpha1.RightSizerConfig{}
	err := r.Get(ctx, req.NamespacedName, rsc)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Reset to default configuration
			log.Info("RightSizerConfig resource not found. Resetting to default configuration")
			r.resetToDefaultConfig()
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Error("Failed to get RightSizerConfig: %v", err)
		return ctrl.Result{}, err
	}

	// Initialize status if needed
	if rsc.Status.Phase == "" {
		rsc.Status.Phase = "Pending"
		rsc.Status.OperatorVersion = "v1.0.0" // You may want to get this from build info
		if err := r.Status().Update(ctx, rsc); err != nil {
			log.Error("Failed to update initial status: %v", err)
			return ctrl.Result{}, err
		}
	}

	// Apply configuration from CRD
	if err := r.applyConfiguration(ctx, rsc); err != nil {
		log.Error("Failed to apply configuration: %v", err)
		return r.updateConfigStatus(ctx, rsc, "Failed", fmt.Sprintf("Error: %v", err))
	}

	// Update metrics about the system
	if err := r.updateSystemMetrics(ctx, rsc); err != nil {
		log.Warn("Failed to update system metrics: %v", err)
	}

	// Update status
	rsc.Status.Phase = "Active"
	rsc.Status.LastAppliedTime = &metav1.Time{Time: time.Now()}
	rsc.Status.ObservedGeneration = rsc.Generation
	rsc.Status.Message = "Configuration successfully applied"

	// Update system health
	rsc.Status.SystemHealth = r.getSystemHealth(ctx)

	if err := r.Status().Update(ctx, rsc); err != nil {
		log.Error("Failed to update config status: %v", err)
		return ctrl.Result{}, err
	}

	log.Info("Successfully reconciled RightSizerConfig", "name", rsc.Name)

	// Requeue after 5 minutes to refresh status
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// applyConfiguration applies the configuration from the CRD to the global config
func (r *RightSizerConfigReconciler) applyConfiguration(ctx context.Context, rsc *v1alpha1.RightSizerConfig) error {
	spec := rsc.Spec

	// Check if operator is enabled
	if !spec.Enabled {
		logger.Info("RightSizer operator is disabled globally")
		r.Config.DryRun = true // Effectively disable by setting dry-run
		return nil
	}

	// Apply resize interval
	if spec.ResizeInterval != "" {
		duration, err := time.ParseDuration(spec.ResizeInterval)
		if err != nil {
			return fmt.Errorf("invalid resize interval: %v", err)
		}
		r.Config.ResizeInterval = duration
	}

	// Apply dry-run setting
	r.Config.DryRun = spec.DryRun

	// Apply default resource strategy
	if spec.DefaultResourceStrategy.CPU.RequestMultiplier != 0 {
		r.Config.CPURequestMultiplier = spec.DefaultResourceStrategy.CPU.RequestMultiplier
	}
	if spec.DefaultResourceStrategy.CPU.RequestAddition != 0 {
		r.Config.CPURequestAddition = spec.DefaultResourceStrategy.CPU.RequestAddition
	}
	if spec.DefaultResourceStrategy.CPU.LimitMultiplier != 0 {
		r.Config.CPULimitMultiplier = spec.DefaultResourceStrategy.CPU.LimitMultiplier
	}
	if spec.DefaultResourceStrategy.CPU.LimitAddition != 0 {
		r.Config.CPULimitAddition = spec.DefaultResourceStrategy.CPU.LimitAddition
	}
	if spec.DefaultResourceStrategy.CPU.MinRequest != 0 {
		r.Config.MinCPURequest = spec.DefaultResourceStrategy.CPU.MinRequest
	}
	if spec.DefaultResourceStrategy.CPU.MaxLimit != 0 {
		r.Config.MaxCPULimit = spec.DefaultResourceStrategy.CPU.MaxLimit
	}

	// Apply memory strategy
	if spec.DefaultResourceStrategy.Memory.RequestMultiplier != 0 {
		r.Config.MemoryRequestMultiplier = spec.DefaultResourceStrategy.Memory.RequestMultiplier
	}
	if spec.DefaultResourceStrategy.Memory.RequestAddition != 0 {
		r.Config.MemoryRequestAddition = spec.DefaultResourceStrategy.Memory.RequestAddition
	}
	if spec.DefaultResourceStrategy.Memory.LimitMultiplier != 0 {
		r.Config.MemoryLimitMultiplier = spec.DefaultResourceStrategy.Memory.LimitMultiplier
	}
	if spec.DefaultResourceStrategy.Memory.LimitAddition != 0 {
		r.Config.MemoryLimitAddition = spec.DefaultResourceStrategy.Memory.LimitAddition
	}
	if spec.DefaultResourceStrategy.Memory.MinRequest != 0 {
		r.Config.MinMemoryRequest = spec.DefaultResourceStrategy.Memory.MinRequest
	}
	if spec.DefaultResourceStrategy.Memory.MaxLimit != 0 {
		r.Config.MaxMemoryLimit = spec.DefaultResourceStrategy.Memory.MaxLimit
	}

	// Apply global constraints
	if spec.GlobalConstraints.MaxChangePercentage > 0 {
		r.Config.SafetyThreshold = float64(spec.GlobalConstraints.MaxChangePercentage) / 100.0
	}

	// Apply observability configuration
	if spec.ObservabilityConfig.LogLevel != "" {
		r.Config.LogLevel = spec.ObservabilityConfig.LogLevel
		logger.Init(spec.ObservabilityConfig.LogLevel)
	}
	r.Config.AuditEnabled = spec.ObservabilityConfig.EnableAuditLog
	r.Config.MetricsEnabled = spec.ObservabilityConfig.EnableMetricsExport
	if spec.ObservabilityConfig.MetricsPort > 0 {
		r.Config.MetricsPort = int(spec.ObservabilityConfig.MetricsPort)
	}

	// Apply security configuration
	r.Config.AdmissionController = spec.SecurityConfig.EnableAdmissionController

	// Apply operator configuration
	if spec.OperatorConfig.MaxRetries > 0 {
		r.Config.MaxRetries = int(spec.OperatorConfig.MaxRetries)
	}
	if spec.OperatorConfig.RetryInterval != "" {
		duration, err := time.ParseDuration(spec.OperatorConfig.RetryInterval)
		if err == nil {
			r.Config.RetryInterval = duration
		}
	}

	// Apply namespace configuration
	// Always apply include namespaces (empty means all)
	r.Config.NamespaceInclude = spec.NamespaceConfig.IncludeNamespaces

	// Combine exclude namespaces with system namespaces
	excludeSet := make(map[string]bool)
	for _, ns := range spec.NamespaceConfig.ExcludeNamespaces {
		excludeSet[ns] = true
	}
	// Always exclude system namespaces
	for _, ns := range spec.NamespaceConfig.SystemNamespaces {
		excludeSet[ns] = true
	}

	// Convert back to slice
	r.Config.NamespaceExclude = []string{}
	for ns := range excludeSet {
		r.Config.NamespaceExclude = append(r.Config.NamespaceExclude, ns)
	}

	// Log namespace configuration
	if len(r.Config.NamespaceInclude) > 0 {
		logger.Info("Namespace include filter applied: %v", r.Config.NamespaceInclude)
	} else {
		logger.Info("No namespace include filter - monitoring all namespaces")
	}
	logger.Info("Namespace exclude filter applied: %v", r.Config.NamespaceExclude)

	// Validate namespace configuration
	if err := r.validateNamespaceConfig(ctx); err != nil {
		logger.Warn("Namespace configuration validation warning: %v", err)
	}

	// Store the configuration globally
	config.Global = r.Config

	logger.Info("Applied configuration from RightSizerConfig CRD")
	return nil
}

// resetToDefaultConfig resets to default configuration when CRD is deleted
func (r *RightSizerConfigReconciler) resetToDefaultConfig() {
	r.Config = config.Load()
	config.Global = r.Config
	logger.Info("Reset to default configuration")
}

// updateSystemMetrics updates system-wide metrics in the status
func (r *RightSizerConfigReconciler) updateSystemMetrics(ctx context.Context, rsc *v1alpha1.RightSizerConfig) error {
	// Count active policies
	policies := &v1alpha1.RightSizerPolicyList{}
	if err := r.List(ctx, policies); err != nil {
		return err
	}

	activePolicies := int32(0)
	for _, policy := range policies.Items {
		if policy.Spec.Enabled {
			activePolicies++
		}
	}
	rsc.Status.ActivePolicies = activePolicies

	// Count monitored resources (simplified - you'd want to count actual workloads)
	// This is a placeholder - implement actual counting logic
	rsc.Status.TotalResourcesMonitored = 0
	rsc.Status.TotalResourcesResized = 0

	return nil
}

// getSystemHealth returns the current system health status
func (r *RightSizerConfigReconciler) getSystemHealth(ctx context.Context) *v1alpha1.SystemHealthStatus {
	health := &v1alpha1.SystemHealthStatus{
		LastHealthCheck: &metav1.Time{Time: time.Now()},
		Errors:          0,
		Warnings:        0,
	}

	// Check metrics provider health (simplified)
	health.MetricsProviderHealthy = true

	// Check webhook health (if enabled)
	if r.Config.AdmissionController {
		health.WebhookHealthy = true // You'd implement actual health check
	}

	// Check leader election status (if enabled)
	// This would require integration with leader election
	health.LeaderElectionActive = false
	health.IsLeader = false

	return health
}

// updateConfigStatus updates the config status
func (r *RightSizerConfigReconciler) updateConfigStatus(ctx context.Context, rsc *v1alpha1.RightSizerConfig, phase, message string) (ctrl.Result, error) {
	rsc.Status.Phase = phase
	rsc.Status.Message = message
	rsc.Status.ObservedGeneration = rsc.Generation

	if err := r.Status().Update(ctx, rsc); err != nil {
		return ctrl.Result{}, err
	}

	// Requeue after 1 minute
	return ctrl.Result{RequeueAfter: time.Minute}, nil
}

// SetupWithManager sets up the controller with the Manager
func (r *RightSizerConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create controller
	c, err := controller.New("rightsizerconfig-controller", mgr, controller.Options{
		Reconciler: r,
		// Only one RightSizerConfig should exist, so we can limit concurrency
		MaxConcurrentReconciles: 1,
	})
	if err != nil {
		return err
	}

	// Watch RightSizerConfig resources
	err = c.Watch(&source.Kind{Type: &v1alpha1.RightSizerConfig{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	logger.Info("RightSizerConfig controller setup complete")
	return nil
}

// validateNamespaceConfig validates that namespace configuration is valid
func (r *RightSizerConfigReconciler) validateNamespaceConfig(ctx context.Context) error {
	// Check for conflicts between include and exclude
	if len(r.Config.NamespaceInclude) > 0 && len(r.Config.NamespaceExclude) > 0 {
		includeSet := make(map[string]bool)
		for _, ns := range r.Config.NamespaceInclude {
			includeSet[ns] = true
		}

		conflicts := []string{}
		for _, ns := range r.Config.NamespaceExclude {
			if includeSet[ns] {
				conflicts = append(conflicts, ns)
			}
		}

		if len(conflicts) > 0 {
			return fmt.Errorf("namespaces %v are in both include and exclude lists", conflicts)
		}
	}

	// Verify that included namespaces exist
	if len(r.Config.NamespaceInclude) > 0 {
		for _, ns := range r.Config.NamespaceInclude {
			namespace := &corev1.Namespace{}
			if err := r.Get(ctx, types.NamespacedName{Name: ns}, namespace); err != nil {
				if errors.IsNotFound(err) {
					logger.Warn("Included namespace %s does not exist", ns)
				}
			}
		}
	}

	return nil
}

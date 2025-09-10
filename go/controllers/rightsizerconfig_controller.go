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
	"errors"
	"fmt"
	"time"

	"right-sizer/admission"
	"right-sizer/api/v1alpha1"
	"right-sizer/audit"
	"right-sizer/config"
	"right-sizer/health"
	"right-sizer/logger"
	"right-sizer/metrics"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

// RightSizerConfigReconciler reconciles a RightSizerConfig object
type RightSizerConfigReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Config          *config.Config
	MetricsProvider *metrics.Provider
	AuditLogger     *audit.AuditLogger
	WebhookManager  *admission.WebhookManager
	HealthChecker   *health.OperatorHealthChecker
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
	log.Info("Reconciling RightSizerConfig: name=%s, namespace=%s", req.Name, req.Namespace)

	// Fetch the RightSizerConfig instance
	rsc := &v1alpha1.RightSizerConfig{}
	err := r.Get(ctx, req.NamespacedName, rsc)
	if err != nil {
		if k8serrors.IsNotFound(err) {
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
		rsc.Status.Phase = "Active"
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

	// Update metrics provider if needed
	if err := r.updateMetricsProvider(ctx, rsc); err != nil {
		log.Error("Failed to update metrics provider: %v", err)
		return r.updateConfigStatus(ctx, rsc, "Failed", fmt.Sprintf("Metrics provider error: %v", err))
	}

	// Update feature components based on configuration
	if err := r.updateFeatureComponents(ctx, rsc); err != nil {
		log.Warn("Failed to update feature components: %v", err)
		// Don't fail the reconciliation, just log the warning
	}

	// Update system metrics
	if err := r.updateSystemMetrics(ctx, rsc); err != nil {
		log.Warn("Failed to update system metrics: %v", err)
	}

	// Update status to Active
	rsc.Status.Phase = "Active"
	rsc.Status.LastAppliedTime = &metav1.Time{Time: time.Now()}
	rsc.Status.ObservedGeneration = rsc.Generation
	rsc.Status.Message = "Configuration successfully applied"

	// Update system health status
	rsc.Status.SystemHealth = r.getSystemHealth(ctx)

	if err := r.Status().Update(ctx, rsc); err != nil {
		log.Error("Failed to update status: %v", err)
		return ctrl.Result{}, err
	}

	// Requeue after the configured resize interval to refresh status
	requeueAfter := 60 * time.Second
	if rsc.Spec.ResizeInterval != "" {
		if duration, err := time.ParseDuration(rsc.Spec.ResizeInterval); err == nil {
			requeueAfter = duration
		}
	}

	log.Info("Successfully reconciled RightSizerConfig: name=%s, requeueAfter=%v", req.Name, requeueAfter)
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// applyConfiguration applies the configuration from the CRD to the global config
func (r *RightSizerConfigReconciler) applyConfiguration(ctx context.Context, rsc *v1alpha1.RightSizerConfig) error {
	log := logger.GetLogger()
	log.Info("Applying configuration from RightSizerConfig CRD")

	// Parse resize interval
	resizeInterval := 30 * time.Second
	if rsc.Spec.ResizeInterval != "" {
		if duration, err := time.ParseDuration(rsc.Spec.ResizeInterval); err == nil {
			resizeInterval = duration
		}
	}

	// Parse retry interval
	retryInterval := 5 * time.Second
	if rsc.Spec.OperatorConfig.RetryInterval != "" {
		if duration, err := time.ParseDuration(rsc.Spec.OperatorConfig.RetryInterval); err == nil {
			retryInterval = duration
		}
	}

	// Extract values with safe defaults
	cpuRequestMultiplier := 1.2
	if rsc.Spec.DefaultResourceStrategy.CPU.RequestMultiplier != 0 {
		cpuRequestMultiplier = rsc.Spec.DefaultResourceStrategy.CPU.RequestMultiplier
	}

	memoryRequestMultiplier := 1.2
	if rsc.Spec.DefaultResourceStrategy.Memory.RequestMultiplier != 0 {
		memoryRequestMultiplier = rsc.Spec.DefaultResourceStrategy.Memory.RequestMultiplier
	}

	cpuRequestAddition := rsc.Spec.DefaultResourceStrategy.CPU.RequestAddition

	memoryRequestAddition := rsc.Spec.DefaultResourceStrategy.Memory.RequestAddition

	cpuLimitMultiplier := 2.0
	if rsc.Spec.DefaultResourceStrategy.CPU.LimitMultiplier != 0 {
		cpuLimitMultiplier = rsc.Spec.DefaultResourceStrategy.CPU.LimitMultiplier
	}

	memoryLimitMultiplier := 2.0
	if rsc.Spec.DefaultResourceStrategy.Memory.LimitMultiplier != 0 {
		memoryLimitMultiplier = rsc.Spec.DefaultResourceStrategy.Memory.LimitMultiplier
	}

	cpuLimitAddition := rsc.Spec.DefaultResourceStrategy.CPU.LimitAddition

	memoryLimitAddition := rsc.Spec.DefaultResourceStrategy.Memory.LimitAddition

	// Parse resource quantity strings
	minCPURequest := "10m"
	if rsc.Spec.DefaultResourceStrategy.CPU.MinRequest != "" {
		minCPURequest = rsc.Spec.DefaultResourceStrategy.CPU.MinRequest
	}

	minMemoryRequest := "64Mi"
	if rsc.Spec.DefaultResourceStrategy.Memory.MinRequest != "" {
		minMemoryRequest = rsc.Spec.DefaultResourceStrategy.Memory.MinRequest
	}

	maxCPULimit := "4000m"
	if rsc.Spec.DefaultResourceStrategy.CPU.MaxLimit != "" {
		maxCPULimit = rsc.Spec.DefaultResourceStrategy.CPU.MaxLimit
	}

	maxMemoryLimit := "8192Mi"
	if rsc.Spec.DefaultResourceStrategy.Memory.MaxLimit != "" {
		maxMemoryLimit = rsc.Spec.DefaultResourceStrategy.Memory.MaxLimit
	}

	// Extract scaling thresholds
	memoryScaleUpThreshold := 0.8
	if rsc.Spec.DefaultResourceStrategy.Memory.ScaleUpThreshold != 0 {
		memoryScaleUpThreshold = rsc.Spec.DefaultResourceStrategy.Memory.ScaleUpThreshold
	}

	memoryScaleDownThreshold := 0.3
	if rsc.Spec.DefaultResourceStrategy.Memory.ScaleDownThreshold != 0 {
		memoryScaleDownThreshold = rsc.Spec.DefaultResourceStrategy.Memory.ScaleDownThreshold
	}

	cpuScaleUpThreshold := 0.8
	if rsc.Spec.DefaultResourceStrategy.CPU.ScaleUpThreshold != 0 {
		cpuScaleUpThreshold = rsc.Spec.DefaultResourceStrategy.CPU.ScaleUpThreshold
	}

	cpuScaleDownThreshold := 0.3
	if rsc.Spec.DefaultResourceStrategy.CPU.ScaleDownThreshold != 0 {
		cpuScaleDownThreshold = rsc.Spec.DefaultResourceStrategy.CPU.ScaleDownThreshold
	}

	// Extract metrics provider configuration
	metricsProvider := "metrics-server"
	if rsc.Spec.MetricsConfig.Provider != "" {
		metricsProvider = rsc.Spec.MetricsConfig.Provider
	}

	prometheusURL := ""
	if rsc.Spec.MetricsConfig.PrometheusEndpoint != "" {
		prometheusURL = rsc.Spec.MetricsConfig.PrometheusEndpoint
	}

	// Extract feature flags
	updateResizePolicy := false
	if rsc.Spec.FeatureGates != nil {
		updateResizePolicy = rsc.Spec.FeatureGates["UpdateResizePolicy"]
	}

	// Extract new fields
	algorithm := "percentile"
	if rsc.Spec.DefaultResourceStrategy.Algorithm != "" {
		algorithm = rsc.Spec.DefaultResourceStrategy.Algorithm
	}

	maxCPUCores := 16
	if rsc.Spec.GlobalConstraints.MaxCPUCores != 0 {
		maxCPUCores = int(rsc.Spec.GlobalConstraints.MaxCPUCores)
	}

	maxMemoryGB := 32
	if rsc.Spec.GlobalConstraints.MaxMemoryGB != 0 {
		maxMemoryGB = int(rsc.Spec.GlobalConstraints.MaxMemoryGB)
	}

	preventOOMKill := true
	if rsc.Spec.GlobalConstraints.RespectPDB {
		preventOOMKill = rsc.Spec.GlobalConstraints.RespectPDB
	}

	respectPodDisruptionBudget := true
	if rsc.Spec.GlobalConstraints.RespectPDB {
		respectPodDisruptionBudget = rsc.Spec.GlobalConstraints.RespectPDB
	}

	aggregationMethod := "avg"
	if rsc.Spec.MetricsConfig.AggregationMethod != "" {
		aggregationMethod = rsc.Spec.MetricsConfig.AggregationMethod
	}

	historyRetention := "30d"
	if rsc.Spec.MetricsConfig.HistoryRetention != "" {
		historyRetention = rsc.Spec.MetricsConfig.HistoryRetention
	}

	includeCustomMetrics := false
	if rsc.Spec.MetricsConfig.IncludeCustomMetrics {
		includeCustomMetrics = rsc.Spec.MetricsConfig.IncludeCustomMetrics
	}

	enableAuditLogging := true
	if rsc.Spec.ObservabilityConfig.EnableAuditLog {
		enableAuditLogging = rsc.Spec.ObservabilityConfig.EnableAuditLog
	}

	enableProfiling := false
	if rsc.Spec.ObservabilityConfig.EnableProfiling {
		enableProfiling = rsc.Spec.ObservabilityConfig.EnableProfiling
	}

	profilingPort := 6060
	if rsc.Spec.ObservabilityConfig.ProfilingPort != 0 {
		profilingPort = int(rsc.Spec.ObservabilityConfig.ProfilingPort)
	}

	healthProbePort := 8081
	if rsc.Spec.OperatorConfig.HealthProbePort != 0 {
		healthProbePort = int(rsc.Spec.OperatorConfig.HealthProbePort)
	}

	leaderElectionLeaseDuration := "15s"
	if rsc.Spec.OperatorConfig.LeaderElectionLeaseDuration != "" {
		leaderElectionLeaseDuration = rsc.Spec.OperatorConfig.LeaderElectionLeaseDuration
	}

	leaderElectionRenewDeadline := "10s"
	if rsc.Spec.OperatorConfig.LeaderElectionRenewDeadline != "" {
		leaderElectionRenewDeadline = rsc.Spec.OperatorConfig.LeaderElectionRenewDeadline
	}

	leaderElectionRetryPeriod := "2s"
	if rsc.Spec.OperatorConfig.LeaderElectionRetryPeriod != "" {
		leaderElectionRetryPeriod = rsc.Spec.OperatorConfig.LeaderElectionRetryPeriod
	}

	livenessEndpoint := "/healthz"
	if rsc.Spec.OperatorConfig.LivenessEndpoint != "" {
		livenessEndpoint = rsc.Spec.OperatorConfig.LivenessEndpoint
	}

	readinessEndpoint := "/readyz"
	if rsc.Spec.OperatorConfig.ReadinessEndpoint != "" {
		readinessEndpoint = rsc.Spec.OperatorConfig.ReadinessEndpoint
	}

	retryAttempts := 3
	if rsc.Spec.OperatorConfig.RetryAttempts != 0 {
		retryAttempts = int(rsc.Spec.OperatorConfig.RetryAttempts)
	}

	syncPeriod := "30s"
	if rsc.Spec.OperatorConfig.SyncPeriod != "" {
		syncPeriod = rsc.Spec.OperatorConfig.SyncPeriod
	}

	tlsCertDir := "/tmp/certs"
	if rsc.Spec.SecurityConfig.TLSCertDir != "" {
		tlsCertDir = rsc.Spec.SecurityConfig.TLSCertDir
	}

	webhookTimeoutSeconds := 10
	if rsc.Spec.SecurityConfig.WebhookTimeoutSeconds != 0 {
		webhookTimeoutSeconds = int(rsc.Spec.SecurityConfig.WebhookTimeoutSeconds)
	}

	// Update the global configuration
	r.Config.UpdateFromCRD(
		cpuRequestMultiplier,
		memoryRequestMultiplier,
		cpuRequestAddition,
		memoryRequestAddition,
		cpuLimitMultiplier,
		memoryLimitMultiplier,
		cpuLimitAddition,
		memoryLimitAddition,
		minCPURequest,
		minMemoryRequest,
		maxCPULimit,
		maxMemoryLimit,
		resizeInterval,
		rsc.Spec.DryRun,
		rsc.Spec.NamespaceConfig.IncludeNamespaces,
		rsc.Spec.NamespaceConfig.ExcludeNamespaces,
		rsc.Spec.NamespaceConfig.SystemNamespaces,
		rsc.Spec.ObservabilityConfig.LogLevel,
		rsc.Spec.ObservabilityConfig.EnableMetricsExport,
		int(rsc.Spec.ObservabilityConfig.MetricsPort),
		rsc.Spec.ObservabilityConfig.EnableAuditLog,
		int(rsc.Spec.OperatorConfig.MaxRetries),
		retryInterval,
		metricsProvider,
		prometheusURL,
		updateResizePolicy,
		rsc.Spec.OperatorConfig.QPS,
		int(rsc.Spec.OperatorConfig.Burst),
		int(rsc.Spec.OperatorConfig.MaxConcurrentReconciles),
		memoryScaleUpThreshold,
		memoryScaleDownThreshold,
		cpuScaleUpThreshold,
		cpuScaleDownThreshold,
		algorithm,
		maxCPUCores,
		maxMemoryGB,
		preventOOMKill,
		respectPodDisruptionBudget,
		aggregationMethod,
		historyRetention,
		includeCustomMetrics,
		enableAuditLogging,
		enableProfiling,
		profilingPort,
		healthProbePort,
		leaderElectionLeaseDuration,
		leaderElectionRenewDeadline,
		leaderElectionRetryPeriod,
		livenessEndpoint,
		readinessEndpoint,
		retryAttempts,
		syncPeriod,
		tlsCertDir,
		webhookTimeoutSeconds,
	)

	// Update logger level if changed
	if rsc.Spec.ObservabilityConfig.LogLevel != "" {
		logger.Init(rsc.Spec.ObservabilityConfig.LogLevel)
	}

	log.Info("Configuration applied successfully from CRD")
	return nil
}

// updateMetricsProvider updates the metrics provider based on configuration
func (r *RightSizerConfigReconciler) updateMetricsProvider(ctx context.Context, rsc *v1alpha1.RightSizerConfig) error {
	log := logger.GetLogger()

	if r.MetricsProvider == nil {
		log.Warn("MetricsProvider reference is nil, skipping update")
		return nil
	}

	desiredProvider := rsc.Spec.MetricsConfig.Provider
	if desiredProvider == "" {
		desiredProvider = "metrics-server"
	}

	// Check if we need to switch providers
	currentProviderType := "unknown"
	if _, ok := (*r.MetricsProvider).(*metrics.MetricsServerProvider); ok {
		currentProviderType = "metrics-server"
	} else if _, ok := (*r.MetricsProvider).(*metrics.PrometheusProvider); ok {
		currentProviderType = "prometheus"
	}

	if currentProviderType != desiredProvider {
		log.Info("Switching metrics provider from %s to %s", currentProviderType, desiredProvider)

		var newProvider metrics.Provider
		if desiredProvider == "prometheus" && rsc.Spec.MetricsConfig.PrometheusEndpoint != "" {
			newProvider = metrics.NewPrometheusProvider(rsc.Spec.MetricsConfig.PrometheusEndpoint)
			log.Info("Switched to Prometheus metrics provider: endpoint=%s", rsc.Spec.MetricsConfig.PrometheusEndpoint)
			if r.HealthChecker != nil {
				r.HealthChecker.UpdateComponentStatus("metrics-provider", true, "Prometheus provider initialized")
			}
		} else {
			newProvider = metrics.NewMetricsServerProvider(r.Client)
			log.Info("Switched to metrics-server provider")
			if r.HealthChecker != nil {
				r.HealthChecker.UpdateComponentStatus("metrics-provider", true, "Metrics-server provider initialized")
			}
		}

		*r.MetricsProvider = newProvider
	}

	return nil
}

// updateFeatureComponents updates feature components based on configuration
func (r *RightSizerConfigReconciler) updateFeatureComponents(ctx context.Context, rsc *v1alpha1.RightSizerConfig) error {
	log := logger.GetLogger()

	// Update audit logger
	if r.AuditLogger != nil {
		if rsc.Spec.ObservabilityConfig.EnableAuditLog {
			log.Info("Audit logging is enabled")
		} else {
			log.Info("Audit logging is disabled")
		}
	}

	// Update admission webhook
	if r.WebhookManager != nil {
		if rsc.Spec.SecurityConfig.EnableAdmissionController {
			log.Info("Admission controller is enabled")
			// The webhook manager will be started in main.go based on config
			if r.HealthChecker != nil {
				r.HealthChecker.UpdateComponentStatus("webhook", false, "Webhook enabled, waiting to start")
			}
		} else {
			log.Info("Admission controller is disabled")
			if r.HealthChecker != nil {
				r.HealthChecker.UpdateComponentStatus("webhook", false, "Not enabled")
			}
		}
	}

	// Update metrics export
	if rsc.Spec.ObservabilityConfig.EnableMetricsExport {
		log.Info("Metrics export enabled on port %d", rsc.Spec.ObservabilityConfig.MetricsPort)
		// Metrics server will be started in main.go based on config
	} else {
		log.Info("Metrics export disabled")
	}

	return nil
}

// updateSystemMetrics updates system-wide metrics
func (r *RightSizerConfigReconciler) updateSystemMetrics(ctx context.Context, rsc *v1alpha1.RightSizerConfig) error {
	log := logger.GetLogger()

	// Count active policies
	policies := &v1alpha1.RightSizerPolicyList{}
	if err := r.List(ctx, policies); err != nil {
		log.Warn("Failed to list policies for metrics: %v", err)
		return err
	}

	activePolicies := 0
	for _, policy := range policies.Items {
		if policy.Spec.Enabled {
			activePolicies++
		}
	}
	rsc.Status.ActivePolicies = int32(activePolicies)

	// Count monitored resources
	// This is a simplified count - in production you'd count actual workloads
	deployments := 0
	statefulsets := 0
	daemonsets := 0

	// Update status counts
	rsc.Status.TotalResourcesMonitored = int32(deployments + statefulsets + daemonsets)

	log.Info("System metrics updated: activePolicies=%d, resourcesMonitored=%d",
		activePolicies, rsc.Status.TotalResourcesMonitored)

	return nil
}

// getSystemHealth returns the current system health status
func (r *RightSizerConfigReconciler) getSystemHealth(ctx context.Context) *v1alpha1.SystemHealthStatus {
	health := &v1alpha1.SystemHealthStatus{
		MetricsProviderHealthy: false,
		WebhookHealthy:         false,
		LeaderElectionActive:   false, // Not implemented in this example
		IsLeader:               true,  // Assuming single instance
		LastHealthCheck:        &metav1.Time{Time: time.Now()},
		Errors:                 0,
		Warnings:               0,
	}

	// Get actual health status from health checker if available
	if r.HealthChecker != nil {
		// Check metrics provider health
		if status, exists := r.HealthChecker.GetComponentStatus("metrics-provider"); exists {
			health.MetricsProviderHealthy = status.Healthy
			if !status.Healthy && status.Message != "Not enabled" && status.Message != "Not initialized" {
				health.Errors++
			}
		}

		// Check webhook health
		if status, exists := r.HealthChecker.GetComponentStatus("webhook"); exists {
			health.WebhookHealthy = status.Healthy
			if !status.Healthy && status.Message != "Not enabled" {
				health.Warnings++
			}
		}

		// Check controller health
		if status, exists := r.HealthChecker.GetComponentStatus("controller"); exists {
			if !status.Healthy {
				health.Errors++
			}
		}
	} else {
		// Fallback if health checker is not available
		if r.MetricsProvider != nil && *r.MetricsProvider != nil {
			health.MetricsProviderHealthy = true
		}

		if r.WebhookManager != nil && r.Config.AdmissionController {
			health.WebhookHealthy = true
		}
	}

	return health
}

// resetToDefaultConfig resets the configuration to defaults when CRD is deleted
func (r *RightSizerConfigReconciler) resetToDefaultConfig() {
	log := logger.GetLogger()
	log.Info("Resetting configuration to defaults")

	r.Config.ResetToDefaults()

	// Reset metrics provider to default
	if r.MetricsProvider != nil {
		*r.MetricsProvider = metrics.NewMetricsServerProvider(r.Client)
	}

	log.Info("Configuration reset to defaults")
}

// updateConfigStatus updates the status of the RightSizerConfig
func (r *RightSizerConfigReconciler) updateConfigStatus(ctx context.Context, rsc *v1alpha1.RightSizerConfig, phase string, message string) (ctrl.Result, error) {
	rsc.Status.Phase = phase
	rsc.Status.Message = message
	rsc.Status.ObservedGeneration = rsc.Generation

	if phase == "Failed" {
		// Add to errors in system health
		if rsc.Status.SystemHealth == nil {
			rsc.Status.SystemHealth = &v1alpha1.SystemHealthStatus{}
		}
		rsc.Status.SystemHealth.Errors++
	}

	if err := r.Status().Update(ctx, rsc); err != nil {
		logger.Error("Failed to update status: %v", err)
		return ctrl.Result{}, err
	}

	// Requeue after a delay for failed states
	if phase == "Failed" {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *RightSizerConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.RightSizerConfig{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1, // Only one config should be processed at a time
		}).
		Complete(r)
}

// parseResourceQuantity parses Kubernetes resource quantity strings to int64 values
func parseResourceQuantity(quantity string, resourceType string) (int64, error) {
	if quantity == "" {
		return 0, errors.New("empty quantity string")
	}

	// Simple parsing for common cases
	// For CPU: "10m" -> 10 (millicores), "1" -> 1000 (millicores)
	// For Memory: "64Mi" -> 64 (MiB), "1Gi" -> 1024 (MiB)

	if resourceType == "cpu" {
		if quantity[len(quantity)-1:] == "m" {
			// Parse millicores (e.g., "10m" -> 10)
			return parseIntFromString(quantity[:len(quantity)-1])
		}
		// Assume whole cores, convert to millicores (e.g., "2" -> 2000)
		if val, err := parseIntFromString(quantity); err == nil {
			return val * 1000, nil
		}
	}

	if resourceType == "memory" {
		if len(quantity) >= 2 {
			suffix := quantity[len(quantity)-2:]
			if suffix == "Mi" {
				// Parse MiB (e.g., "64Mi" -> 64)
				return parseIntFromString(quantity[:len(quantity)-2])
			}
			if suffix == "Gi" {
				// Parse GiB, convert to MiB (e.g., "1Gi" -> 1024)
				if val, err := parseIntFromString(quantity[:len(quantity)-2]); err == nil {
					return val * 1024, nil
				}
			}
		}
		// Assume MiB if no suffix
		return parseIntFromString(quantity)
	}

	return 0, fmt.Errorf("unknown resource type or format: %s", quantity)
}

// parseIntFromString is a simple integer parser
func parseIntFromString(s string) (int64, error) {
	var result int64
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("invalid integer: %s", s)
		}
		result = result*10 + int64(ch-'0')
	}
	return result, nil
}

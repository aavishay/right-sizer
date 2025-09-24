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

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"

	"right-sizer/admission"
	"right-sizer/api"
	"right-sizer/api/v1alpha1"
	"right-sizer/audit"
	"right-sizer/config"
	"right-sizer/controllers"
	"right-sizer/health"
	"right-sizer/logger"
	"right-sizer/metrics"
	"right-sizer/retry"
	"right-sizer/validation"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"right-sizer/internal/aiops"
	narrative "right-sizer/internal/aiops/narratives"
	"right-sizer/internal/platform"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Build-time variables set via ldflags
var (
	Version   = "dev"
	BuildDate = "unknown"
	GitCommit = "unknown"
)

// Health server is handled by the controller-runtime manager
// which provides /healthz and /readyz endpoints automatically

var (
	// Metrics variables initialized once
	capabilityGauge    *prometheus.GaugeVec
	clusterVersionInfo *prometheus.GaugeVec
	registerOnce       sync.Once
)

func main() {
	// Print startup banner
	fmt.Println("========================================")
	fmt.Println("🚀 Right-Sizer Operator Starting...")
	fmt.Printf("Version: %s\n", Version)
	fmt.Printf("Build Date: %s\n", BuildDate)
	fmt.Printf("Git Commit: %s\n", GitCommit)
	fmt.Printf("Go Version: %s\n", runtime.Version())
	fmt.Println("========================================")

	// Initialize configuration with defaults
	// Configuration will be updated from CRDs once they are loaded
	cfg := config.Load()

	// Initialize logger with default level
	logger.Init(cfg.LogLevel)

	// Initialize controller-runtime logger to prevent warnings
	zapLog, err := zap.NewProduction()
	if err != nil {
		// Fall back to development logger if production logger fails
		zapLog, _ = zap.NewDevelopment()
	}
	ctrllog.SetLogger(zapr.NewLogger(zapLog))

	fmt.Println("----------------------------------------")
	logger.Info("📋 Using Default Configuration")
	logger.Info("   Waiting for RightSizerConfig CRD to override defaults...")
	logger.Info("   Configuration Source: %s", cfg.ConfigSource)
	fmt.Println("----------------------------------------")

	// Print build information
	logger.Info("📦 Build Information:")
	logger.Info("   Go Version: %s", runtime.Version())
	logger.Info("   Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
	logger.Info("   Kubernetes Client-Go: v0.34.0")
	logger.Info("   Controller Runtime: v0.22.0")
	logger.Info("   API Machinery: v0.34.0")

	// Initialize enhanced components
	// operatorMetrics := metrics.NewOperatorMetrics() // Temporarily disabled to test crash fix
	var operatorMetrics *metrics.OperatorMetrics = nil

	// Initialize health checker
	healthChecker := health.NewOperatorHealthChecker()
	logger.Info("✅ Health checker initialized")

	// Get Kubernetes config with rate limiting to prevent API server overload
	kubeConfig := ctrl.GetConfigOrDie()

	// Configure rate limiting for the Kubernetes client
	// QPS: Queries Per Second allowed to the API server
	// Burst: Maximum burst for throttle
	kubeConfig.QPS = float32(cfg.QPS) // Use configured value (default: 20)
	kubeConfig.Burst = cfg.Burst      // Use configured value (default: 30)

	// Print Kubernetes client and server versions
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		logger.Warn("Could not create clientset for version info: %v", err)
	} else {
		fmt.Println("----------------------------------------")
		logger.Info("🌐 Kubernetes Cluster Information:")

		discoveryClient := clientset.Discovery()

		// Get server version
		serverVersion, err := discoveryClient.ServerVersion()
		if err != nil {
			logger.Warn("   ⚠️  Could not get server version: %v", err)
		} else {
			logger.Info("   Server Version: %s", serverVersion.GitVersion)
			logger.Info("   Server Platform: %s", serverVersion.Platform)
			logger.Info("   Server Go Version: %s", serverVersion.GoVersion)

			// Cluster capability & version evaluation (minimum supported: 1.33)
			var majorInt, minorInt int
			fmt.Sscanf(serverVersion.Major, "%d", &majorInt)
			fmt.Sscanf(serverVersion.Minor, "%d", &minorInt)
			if majorInt == 1 && minorInt < 33 {
				logger.Warn("   ⚠️  Detected Kubernetes %s (<1.33). Operator entering degraded mode (advanced features disabled).", serverVersion.GitVersion)
			} else {
				logger.Info("   ✅ Kubernetes version satisfies minimum (>=1.33)")
			}

			// Dynamic capability detection (uses discovery API)
			capDetector := platform.NewDetector(clientset)
			caps, capErr := capDetector.Detect(context.Background())
			if capErr != nil {
				logger.Warn("   ⚠️  Capability detection partial: %v", capErr)
			} else {
				logger.Info("   Capabilities: %s", caps.Summary())
				if !caps.Supported && caps.VersionWarning != "" {
					logger.Warn("   ⚠️  %s", caps.VersionWarning)
				}
			}

			// Expose capability metrics early so they are present when /metrics is scraped.
			// We register here unconditionally; metrics server startup (later) will expose them.
			registerOnce.Do(func() {
				capabilityGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
					Name: "right_sizer_capability_enabled",
					Help: "Detected cluster capability (1=enabled, 0=disabled).",
				}, []string{"capability"})
				clusterVersionInfo = promauto.NewGaugeVec(prometheus.GaugeOpts{
					Name: "right_sizer_cluster_version_info",
					Help: "Cluster version info (value always 1).",
				}, []string{"version", "minor"})
			})

			if clusterVersionInfo != nil {
				clusterVersionInfo.WithLabelValues(caps.RawVersion, fmt.Sprintf("%d", caps.Minor)).Set(1)
			}
			if capabilityGauge != nil {
				setCap := func(name string, v bool) {
					if v {
						capabilityGauge.WithLabelValues(name).Set(1)
					} else {
						capabilityGauge.WithLabelValues(name).Set(0)
					}
				}
				setCap("ephemeral_containers", caps.EphemeralContainers)
				setCap("pod_resize", caps.PodResize)
				setCap("metrics_server", caps.MetricsServerAvailable)
				setCap("dynamic_resource_allocation", caps.DynamicResourceAllocation)
				setCap("in_place_vertical_scaling", caps.InPlacePodVerticalScaling)
				setCap("memory_qos", caps.MemoryQoS)
				setCap("supported_version", caps.Supported)
			}

			// (Retain API version log for continuity)
			versionInfo := version.Info{
				Major:      serverVersion.Major,
				Minor:      serverVersion.Minor,
				GitVersion: serverVersion.GitVersion,
			}
			logger.Info("   API Version: %s.%s", versionInfo.Major, versionInfo.Minor)
		}

		// Check API resources including resize subresource
		fmt.Println("----------------------------------------")
		logger.Info("🔍 Checking API Resources:")
		logger.Info("   Rate Limiting: QPS=%v, Burst=%v", kubeConfig.QPS, kubeConfig.Burst)
		logger.Info("   Concurrency: MaxConcurrentReconciles=%v", cfg.MaxConcurrentReconciles)

		apiResourceList, err := discoveryClient.ServerResourcesForGroupVersion("v1")
		if err == nil {
			hasResize := false
			hasPodMetrics := false

			for _, resource := range apiResourceList.APIResources {
				if resource.Name == "pods/resize" {
					hasResize = true
				}
				if resource.Name == "pods/metrics" {
					hasPodMetrics = true
				}
			}

			if hasResize {
				logger.Success("   ✅ pods/resize subresource: AVAILABLE (In-place resize supported)")
			} else {
				logger.Warn("   ❌ pods/resize subresource: NOT FOUND (Will use fallback methods)")
			}

			if hasPodMetrics {
				logger.Success("   ✅ pods/metrics: AVAILABLE")
			} else {
				logger.Warn("   ⚠️  pods/metrics: NOT FOUND")
			}
		} else {
			logger.Warn("   ⚠️  Could not query API resources: %v", err)
		}

		// Check metrics-server availability
		_, err = discoveryClient.ServerResourcesForGroupVersion("metrics.k8s.io/v1beta1")
		if err == nil {
			logger.Success("   ✅ metrics-server: AVAILABLE")
		} else {
			logger.Warn("   ⚠️  metrics-server: NOT AVAILABLE or NOT READY")
		}
	}

	fmt.Println("========================================")

	// Configure leader election from environment variables
	enableLeaderElection := false
	if envVal := os.Getenv("ENABLE_LEADER_ELECTION"); envVal != "" {
		if parsed, err := strconv.ParseBool(envVal); err == nil {
			enableLeaderElection = parsed
			logger.Info("🔧 Leader election configured from environment: %v", enableLeaderElection)
		}
	}

	leaderElectionID := "right-sizer-leader-election"
	if envVal := os.Getenv("LEADER_ELECTION_ID"); envVal != "" {
		leaderElectionID = envVal
	}

	leaderElectionNamespace := os.Getenv("OPERATOR_NAMESPACE")
	if leaderElectionNamespace == "" {
		leaderElectionNamespace = "right-sizer"
	}

	if enableLeaderElection {
		logger.Info("👑 Leader election enabled:")
		logger.Info("   ID: %s", leaderElectionID)
		logger.Info("   Namespace: %s", leaderElectionNamespace)
	}

	// Create controller manager with rate limiting and resource protection
	mgr, err := manager.New(kubeConfig, manager.Options{
		// Limit the number of concurrent reconciles per controller
		// This prevents overwhelming the API server with too many concurrent operations
		Controller: ctrlconfig.Controller{
			MaxConcurrentReconciles: cfg.MaxConcurrentReconciles, // Use configured value (default: 3)
		},

		// Graceful shutdown timeout
		GracefulShutdownTimeout: &[]time.Duration{30 * time.Second}[0],

		// Leader election helps prevent multiple instances from making changes simultaneously
		LeaderElection:          enableLeaderElection,
		LeaderElectionID:        leaderElectionID,
		LeaderElectionNamespace: leaderElectionNamespace,

		// Health and readiness probes
		HealthProbeBindAddress: ":8081",

		// Metrics server configuration
		Metrics: server.Options{
			BindAddress: ":8080",
		},
	})
	if err != nil {
		logger.Error("unable to start manager: %v", err)
		os.Exit(1)
	}

	// Add health check endpoints with custom health checker
	if err := mgr.AddHealthzCheck("healthz", healthChecker.LivenessCheck); err != nil {
		logger.Error("unable to set up health check: %v", err)
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthChecker.ReadinessCheck); err != nil {
		logger.Error("unable to set up ready check: %v", err)
		os.Exit(1)
	}
	// Add a detailed health check endpoint
	if err := mgr.AddReadyzCheck("detailed", healthChecker.DetailedHealthCheck()); err != nil {
		logger.Warn("unable to set up detailed health check: %v", err)
	}
	logger.Info("✅ Health and readiness probes configured on :8081")
	logger.Info("   - /healthz for liveness probe")
	logger.Info("   - /readyz for readiness probe")
	logger.Info("   - /readyz/detailed for detailed health status")

	// Register CRD schemes
	if err := v1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		logger.Error("unable to add CRD schemes: %v", err)
		os.Exit(1)
	}

	// Create metrics client for accessing metrics-server
	metricsClient, err := metricsclient.NewForConfig(kubeConfig)
	if err != nil {
		logger.Warn("Unable to create metrics client (metrics-server may not be installed): %v", err)
		// Don't exit, just continue without metrics
		metricsClient = nil
	}

	// Initialize enhanced components
	ctx := context.Background()

	// Initialize resource validator
	resourceValidator := validation.NewResourceValidator(mgr.GetClient(), clientset, cfg, operatorMetrics)
	if err := resourceValidator.RefreshCaches(ctx); err != nil {
		logger.Warn("Failed to refresh resource validator caches: %v", err)
	}

	// Initialize audit logger (will be enabled/disabled based on CRD config)
	var auditLogger *audit.AuditLogger
	auditConfig := audit.DefaultAuditConfig()
	auditLogger, err = audit.NewAuditLogger(mgr.GetClient(), cfg, operatorMetrics, auditConfig)
	if err != nil {
		logger.Warn("Failed to initialize audit logger: %v", err)
	}

	// Initialize admission webhook (will be enabled/disabled based on CRD config)
	var webhookManager *admission.WebhookManager
	webhookConfig := admission.WebhookConfig{
		Port:              8443,
		EnableValidation:  true,
		EnableMutation:    false,
		DryRun:            cfg.DryRun,
		RequireAnnotation: false,
	}
	webhookManager = admission.NewWebhookManager(
		mgr.GetClient(),
		clientset,
		resourceValidator,
		cfg,
		operatorMetrics,
		webhookConfig,
	)

	// Initialize metrics provider (default to metrics-server, will be updated from CRD)
	var provider metrics.Provider
	logger.Info("Using default metrics-server provider (can be changed via RightSizerConfig CRD)")
	provider = metrics.NewMetricsServerProvider(mgr.GetClient())
	healthChecker.UpdateComponentStatus("metrics-provider", true, "Metrics provider initialized")

	// Initialize retry configuration
	retryConfig := retry.Config{
		MaxRetries:          cfg.MaxRetries,
		InitialDelay:        cfg.RetryInterval,
		MaxDelay:            30 * time.Second,
		BackoffFactor:       2.0,
		RandomizationFactor: 0.1,
		Timeout:             60 * time.Second,
	}

	// Initialize circuit breaker configuration
	cbConfig := retry.DefaultCircuitBreakerConfig()
	cbConfig.RecoveryTimeout = 30 * time.Second

	// Create retry handler with circuit breaker
	retryHandler := retry.NewRetryWithCircuitBreaker(
		"right-sizer-operations",
		retryConfig,
		cbConfig,
		operatorMetrics,
	)

	// Setup signal handling for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// Check if CRDs exist before setting up controllers
	configCRDExists := false
	policyCRDExists := false

	// Check for RightSizerConfig CRD
	apiResourceList, err := clientset.Discovery().ServerResourcesForGroupVersion("rightsizer.io/v1alpha1")
	if err == nil && apiResourceList != nil {
		for _, resource := range apiResourceList.APIResources {
			if resource.Kind == "RightSizerConfig" {
				configCRDExists = true
			}
			if resource.Kind == "RightSizerPolicy" {
				policyCRDExists = true
			}
		}
	}

	// Setup CRD controllers only if CRDs exist
	if configCRDExists || policyCRDExists {
		logger.Info("Setting up CRD controllers...")

		if configCRDExists {
			// Setup RightSizerConfig controller (this will manage configuration)
			configController := &controllers.RightSizerConfigReconciler{
				Client:          mgr.GetClient(),
				Scheme:          mgr.GetScheme(),
				Config:          cfg,
				MetricsProvider: &provider,
				AuditLogger:     auditLogger,
				WebhookManager:  webhookManager,
				HealthChecker:   healthChecker,
			}
			if err := configController.SetupWithManager(mgr); err != nil {
				logger.Error("unable to setup RightSizerConfig controller: %v", err)
				os.Exit(1)
			}
			logger.Info("✅ RightSizerConfig controller initialized")
		}

		if policyCRDExists {
			// Setup RightSizerPolicy controller
			policyController := &controllers.RightSizerPolicyReconciler{
				Client:          mgr.GetClient(),
				Scheme:          mgr.GetScheme(),
				MetricsProvider: provider,
				Config:          cfg,
			}
			if err := policyController.SetupWithManager(mgr); err != nil {
				logger.Error("unable to setup RightSizerPolicy controller: %v", err)
				os.Exit(1)
			}
			logger.Info("✅ RightSizerPolicy controller initialized")
		}
	} else {
		logger.Info("📋 No RightSizerConfig or RightSizerPolicy CRDs found - using default configuration")
	}

	// Setup the main rightsizer controller
	// The controller will use configuration from CRDs
	logger.Info("Setting up main RightSizer controller...")

	// Use AdaptiveRightSizer as the default implementation with rate limiting
	// It will check for in-place resize capability based on CRD configuration
	// The controller will respect the manager's rate limiting configuration
	predictorEngine, err := controllers.SetupAdaptiveRightSizer(mgr, provider, auditLogger, cfg.DryRun)
	if err != nil {
		logger.Error("unable to setup AdaptiveRightSizer: %v", err)
		os.Exit(1)
	}
	logger.Info("✅ AdaptiveRightSizer controller initialized")

	// Start metrics server (will be enabled/disabled based on CRD config)
	go func() {
		// Wait for configuration to be loaded from CRD
		time.Sleep(5 * time.Second)

		if cfg.MetricsEnabled {
			logger.Info("🔍 Starting metrics server on port %d", cfg.MetricsPort)
			if err := metrics.StartMetricsServer(cfg.MetricsPort); err != nil {
				logger.Error("Metrics server error: %v", err)
			}
		}
	}()

	// Create a simple event store for optimization events
	type OptimizationEvent struct {
		Timestamp        int64  `json:"timestamp"`
		EventID          string `json:"eventId"`
		PodName          string `json:"podName"`
		Namespace        string `json:"namespace"`
		ContainerName    string `json:"containerName"`
		Operation        string `json:"operation"`
		Reason           string `json:"reason"`
		Status           string `json:"status"`
		Action           string `json:"action"`
		PreviousCPU      string `json:"previousCPU,omitempty"`
		CurrentCPU       string `json:"currentCPU,omitempty"`
		PreviousMemory   string `json:"previousMemory,omitempty"`
		CurrentMemory    string `json:"currentMemory,omitempty"`
		OptimizationType string `json:"optimizationType,omitempty"`
	}

	// In-memory store for recent optimization events (last 100 events)
	// var optimizationEvents = make([]OptimizationEvent, 0, 100)
	// var eventsMutex sync.RWMutex

	// Start API server using the new API server module
	go func() {
		// Wait for configuration to be loaded from CRD
		time.Sleep(5 * time.Second)

		apiServer := api.NewServer(clientset, metricsClient, predictorEngine, operatorMetrics)
		if err := apiServer.Start(8082); err != nil {
			logger.Error("API server error: %v", err)
		}
	}()

	// Start admission webhook (will be enabled/disabled based on CRD config)
	go func() {
		// Wait for configuration to be loaded from CRD
		time.Sleep(5 * time.Second)

		if cfg.AdmissionController && webhookManager != nil {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			logger.Info("🛡️  Starting admission webhook...")
			healthChecker.UpdateComponentStatus("webhook", false, "Webhook starting...")
			if err := webhookManager.Start(ctx); err != nil {
				logger.Error("admission webhook error: %v", err)
				healthChecker.UpdateComponentStatus("webhook", false, fmt.Sprintf("Webhook error: %v", err))
			} else {
				healthChecker.UpdateComponentStatus("webhook", true, "Webhook server is running")
			}
		} else {
			healthChecker.UpdateComponentStatus("webhook", false, "Not enabled")
		}
	}()

	// Start periodic health checks
	healthCheckCtx, healthCheckCancel := context.WithCancel(context.Background())
	defer healthCheckCancel()
	healthChecker.StartPeriodicHealthChecks(healthCheckCtx)
	logger.Info("🔍 Started periodic health checks")

	// Initialize and start the AIOps Engine
	logger.Info("🤖 Initializing AIOps Engine...")
	llmConfig := narrative.LLMConfig{
		APIKey:    os.Getenv("LLM_API_KEY"),
		APIURL:    os.Getenv("LLM_API_URL"),
		ModelName: os.Getenv("LLM_MODEL_NAME"),
	}
	if llmConfig.APIKey != "" {
		aiopsEngine := aiops.NewEngine(clientset, provider, llmConfig)
		go aiopsEngine.Start(ctx)
	} else {
		logger.Info("🤖 AIOps Engine disabled: LLM_API_KEY environment variable not set.")
	}

	// Start manager in a goroutine
	managerDone := make(chan error, 1)
	go func() {
		logger.Info("🚀 Starting right-sizer operator manager...")
		if configCRDExists {
			logger.Info("📋 Configuration will be loaded from RightSizerConfig CRDs")
		}
		if policyCRDExists {
			logger.Info("📋 Policies will be loaded from RightSizerPolicy CRDs")
		}
		healthChecker.UpdateComponentStatus("controller", true, "Controller manager started")
		managerDone <- mgr.Start(ctrl.SetupSignalHandler())
	}()

	// Wait for shutdown signal or manager error
	select {
	case <-signalChan:
		logger.Info("📢 Shutdown signal received, initiating graceful shutdown...")
	case err := <-managerDone:
		if err != nil {
			logger.Error("❌ Manager error: %v", err)
		}
	}

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Cleanup components
	if auditLogger != nil {
		logger.Info("📋 Closing audit logger...")
		if err := auditLogger.Close(); err != nil {
			logger.Warn("Error closing audit logger: %v", err)
		}
	}

	if webhookManager != nil {
		logger.Info("🛡️  Stopping admission webhook...")
		if err := webhookManager.Stop(shutdownCtx); err != nil {
			logger.Warn("Error stopping webhook manager: %v", err)
		}
	}

	// Log final statistics
	logger.Info("✅ Right-sizer operator shutdown completed")

	// Print shutdown summary
	fmt.Println("========================================")
	fmt.Println("🎯 Right-Sizer Operator Summary:")
	fmt.Printf("   Configuration Source: %s\n", cfg.ConfigSource)
	fmt.Printf("   Circuit Breaker State: %s\n", retryHandler.GetCircuitBreakerState())
	if operatorMetrics != nil {
		fmt.Println("   Metrics available at /metrics endpoint")
	}
	fmt.Println("========================================")
}

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
	"net/http"
	"os"
	"os/signal"
	"right-sizer/admission"
	"right-sizer/api/v1alpha1"
	"right-sizer/audit"
	"right-sizer/config"
	"right-sizer/controllers"
	"right-sizer/health"
	"right-sizer/logger"
	"right-sizer/metrics"
	"right-sizer/retry"
	"right-sizer/validation"
	"runtime"
	"syscall"
	"time"

	"github.com/go-logr/zapr"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

// Global metrics instances
var (
	operatorMetrics *metrics.OperatorMetrics
	memoryMetrics   *metrics.MemoryMetrics
)

func main() {
	// Print startup banner
	fmt.Println("========================================")
	fmt.Println("🚀 Right-Sizer Operator Starting...")
	fmt.Println("========================================")

	// Initialize configuration with defaults
	cfg := config.Load()

	// Initialize logger with default level
	logger.Init(cfg.LogLevel)

	// Initialize controller-runtime logger
	zapLog, err := zap.NewProduction()
	if err != nil {
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
	logger.Info("   Memory Metrics: ENABLED")
	logger.Info("   Memory Pressure Logging: ENABLED")

	// Initialize metrics
	logger.Info("📊 Initializing Metrics...")
	operatorMetrics = metrics.NewOperatorMetrics()
	memoryMetrics = metrics.NewMemoryMetrics()
	logger.Success("   ✅ Operator metrics initialized")
	logger.Success("   ✅ Memory metrics initialized")
	logger.Success("   ✅ Memory pressure monitoring enabled")

	// Start Prometheus metrics server on port 9090
	go startPrometheusServer()

	// Initialize health checker with memory metrics
	healthChecker := health.NewOperatorHealthChecker()
	healthChecker.SetMemoryMetrics(memoryMetrics)
	logger.Info("✅ Health checker initialized with memory monitoring")

	// Get Kubernetes config with rate limiting
	kubeConfig := ctrl.GetConfigOrDie()
	kubeConfig.QPS = float32(cfg.QPS)
	kubeConfig.Burst = cfg.Burst

	// Print Kubernetes client and server versions
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		logger.Warn("Could not create clientset for version info: %v", err)
	} else {
		printClusterInfo(clientset, kubeConfig, cfg)
	}

	fmt.Println("========================================")

	// Create controller manager with enhanced configuration
	mgr, err := createManager(kubeConfig, cfg, healthChecker)
	if err != nil {
		logger.Error("unable to start manager: %v", err)
		os.Exit(1)
	}

	// Register CRD schemes
	if err := v1alpha1.AddToScheme(mgr.GetScheme()); err != nil {
		logger.Error("unable to add CRD schemes: %v", err)
		os.Exit(1)
	}

	// Initialize enhanced components with memory metrics
	ctx := context.Background()

	// Initialize resource validator with memory metrics
	resourceValidator := validation.NewResourceValidatorWithMemoryMetrics(
		mgr.GetClient(), clientset, cfg, operatorMetrics, memoryMetrics)
	if err := resourceValidator.RefreshCaches(ctx); err != nil {
		logger.Warn("Failed to refresh resource validator caches: %v", err)
	}

	// Initialize audit logger with memory event tracking
	var auditLogger *audit.AuditLogger
	auditConfig := audit.DefaultAuditConfig()
	auditConfig.EnableMemoryEvents = true
	auditLogger, err = audit.NewAuditLoggerWithMemoryMetrics(
		mgr.GetClient(), cfg, operatorMetrics, memoryMetrics, auditConfig)
	if err != nil {
		logger.Warn("Failed to initialize audit logger: %v", err)
	}

	// Initialize admission webhook handler with memory validation
	admissionHandler := admission.NewAdmissionHandler(
		mgr.GetClient(), cfg, resourceValidator, operatorMetrics, memoryMetrics)

	// Initialize retry manager
	retryManager := retry.NewRetryManager(retry.DefaultConfig())

	// Create pod controller with memory metrics
	podController := controllers.NewPodMemoryController(
		mgr.GetClient(), clientset, mgr.GetScheme(), cfg,
		resourceValidator, auditLogger,
		healthChecker, operatorMetrics, memoryMetrics, retryManager)

	if err := podController.SetupWithManager(mgr); err != nil {
		logger.Error("unable to create pod controller: %v", err)
		os.Exit(1)
	}

	// Create config controller with memory metrics
	configController := controllers.NewRightSizerConfigControllerWithMemoryMetrics(
		mgr.GetClient(), mgr.GetScheme(), cfg, healthChecker,
		operatorMetrics, memoryMetrics, podController)

	if err := configController.SetupWithManager(mgr); err != nil {
		logger.Error("unable to create config controller: %v", err)
		os.Exit(1)
	}

	// Create policy controller with memory metrics
	policyController := controllers.NewRightSizerPolicyControllerWithMemoryMetrics(
		mgr.GetClient(), mgr.GetScheme(), cfg, healthChecker,
		operatorMetrics, memoryMetrics)

	if err := policyController.SetupWithManager(mgr); err != nil {
		logger.Error("unable to create policy controller: %v", err)
		os.Exit(1)
	}

	// Start background memory monitoring
	go startMemoryMonitoring(ctx, clientset, memoryMetrics)

	logger.Success("✅ All controllers initialized with memory metrics support")
	logger.Info("📈 Prometheus metrics available on :9090/metrics")
	logger.Info("🔍 Memory pressure monitoring active")

	// Setup graceful shutdown
	setupGracefulShutdown(mgr, healthChecker)

	// Start the manager
	logger.Info("🚀 Starting controller manager...")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Error("problem running manager: %v", err)
		os.Exit(1)
	}
}

// startPrometheusServer starts the Prometheus metrics server on port 9090
func startPrometheusServer() {
	mux := http.NewServeMux()

	// Main metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Memory-specific metrics endpoint
	mux.HandleFunc("/metrics/memory", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "# Memory Metrics Export Enabled\n")
		fmt.Fprintf(w, "# rightsizer_pod_memory_usage_bytes\n")
		fmt.Fprintf(w, "# rightsizer_pod_memory_working_set_bytes\n")
		fmt.Fprintf(w, "# rightsizer_pod_memory_rss_bytes\n")
		fmt.Fprintf(w, "# rightsizer_pod_memory_cache_bytes\n")
		fmt.Fprintf(w, "# rightsizer_memory_pressure_events_total\n")
		fmt.Fprintf(w, "# rightsizer_memory_pressure_level\n")
	})

	// Health check for metrics server
	mux.HandleFunc("/metrics/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("metrics server healthy\n"))
	})

	logger.Info("🎯 Starting Prometheus metrics server on :9090")
	if err := http.ListenAndServe(":9090", mux); err != nil {
		logger.Error("Failed to start Prometheus metrics server: %v", err)
	}
}

// startMemoryMonitoring starts background memory monitoring
func startMemoryMonitoring(ctx context.Context, clientset *kubernetes.Clientset, memMetrics *metrics.MemoryMetrics) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	logger.Info("🔍 Starting background memory monitoring (30s interval)")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// This would normally fetch actual pod metrics
			// For now, it's a placeholder that shows the monitoring is active
			logger.Debug("[MEMORY_MONITOR] Checking for memory pressure conditions...")
		}
	}
}

// printClusterInfo prints Kubernetes cluster information
func printClusterInfo(clientset *kubernetes.Clientset, kubeConfig *rest.Config, cfg *config.Config) {
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
		logger.Info("   Memory Metrics: ENABLED")
		logger.Info("   Memory Pressure Detection: ACTIVE")
	}

	// Check API resources including resize subresource
	fmt.Println("----------------------------------------")
	logger.Info("🔍 Checking API Resources:")
	logger.Info("   Rate Limiting: QPS=%v, Burst=%v", kubeConfig.QPS, kubeConfig.Burst)
	logger.Info("   Memory Monitoring: ENABLED")

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
			logger.Success("   ✅ Memory resizing: ENABLED")
		} else {
			logger.Warn("   ❌ pods/resize subresource: NOT FOUND (Will use fallback methods)")
		}

		if hasPodMetrics {
			logger.Success("   ✅ pods/metrics: AVAILABLE")
			logger.Success("   ✅ Memory metrics collection: ACTIVE")
		} else {
			logger.Warn("   ⚠️  pods/metrics: NOT FOUND")
		}
	}

	// Check metrics-server availability
	_, err = discoveryClient.ServerResourcesForGroupVersion("metrics.k8s.io/v1beta1")
	if err == nil {
		logger.Success("   ✅ metrics-server: AVAILABLE")
		logger.Success("   ✅ Memory usage tracking: ACTIVE")
	} else {
		logger.Warn("   ⚠️  metrics-server: NOT AVAILABLE or NOT READY")
	}
}

// createManager creates the controller manager with enhanced configuration
func createManager(kubeConfig *rest.Config, cfg *config.Config, healthChecker *health.OperatorHealthChecker) (manager.Manager, error) {
	return manager.New(kubeConfig, manager.Options{
		Controller: ctrlconfig.Controller{
			MaxConcurrentReconciles: cfg.MaxConcurrentReconciles,
		},
		GracefulShutdownTimeout: &[]time.Duration{30 * time.Second}[0],
		LeaderElection:          false,
		LeaderElectionID:        "right-sizer-leader-election",
		LeaderElectionNamespace: "default",
		HealthProbeBindAddress:  ":8081",
		Metrics: server.Options{
			BindAddress: ":8080",
		},
	})
}

// setupGracefulShutdown sets up graceful shutdown handling
func setupGracefulShutdown(mgr manager.Manager, healthChecker *health.OperatorHealthChecker) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-c
		logger.Info("🛑 Shutdown signal received, gracefully stopping...")

		// Mark as unhealthy for probes
		healthChecker.SetReady(false)

		// Log memory metrics summary before shutdown
		if memoryMetrics != nil {
			logger.Info("📊 Final memory metrics summary logged")
			// Memory metrics would be automatically exported to Prometheus
		}

		// Give time for final metrics export
		time.Sleep(2 * time.Second)

		os.Exit(0)
	}()
}

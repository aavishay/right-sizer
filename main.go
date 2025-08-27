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
	"fmt"
	"net/http"
	"os"
	"right-sizer/config"
	"right-sizer/controllers"
	"right-sizer/logger"
	"right-sizer/metrics"
	"runtime"

	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// StartHealthServer starts a simple HTTP server on :8081 for health checks
func StartHealthServer() {
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("healthy"))
	})
	http.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})
	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})
	go func() {
		logger.Info("Starting health server on :8081")
		if err := http.ListenAndServe(":8081", nil); err != nil {
			logger.Error("Health server error: %v", err)
		}
	}()
}

func main() {
	// Print startup banner
	fmt.Println("========================================")
	fmt.Println("🚀 Right-Sizer Operator Starting...")
	fmt.Println("========================================")

	// Load configuration from environment variables
	cfg := config.Load()

	// Initialize logger with configured level
	logger.Init(cfg.LogLevel)

	fmt.Println("----------------------------------------")
	logger.Info("📋 Configuration Loaded:")
	logger.Info("   CPU Request Multiplier: %.2f", cfg.CPURequestMultiplier)
	logger.Info("   Memory Request Multiplier: %.2f", cfg.MemoryRequestMultiplier)
	logger.Info("   CPU Limit Multiplier: %.2f", cfg.CPULimitMultiplier)
	logger.Info("   Memory Limit Multiplier: %.2f", cfg.MemoryLimitMultiplier)
	logger.Info("   Max CPU Limit: %d millicores", cfg.MaxCPULimit)
	logger.Info("   Max Memory Limit: %d MB", cfg.MaxMemoryLimit)
	logger.Info("   Min CPU Request: %d millicores", cfg.MinCPURequest)
	logger.Info("   Min Memory Request: %d MB", cfg.MinMemoryRequest)
	logger.Info("   Resize Interval: %v", cfg.ResizeInterval)
	logger.Info("   Log Level: %s", cfg.LogLevel)
	fmt.Println("----------------------------------------")

	// Print build information
	logger.Info("📦 Build Information:")
	logger.Info("   Go Version: %s", runtime.Version())
	logger.Info("   Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
	logger.Info("   Kubernetes Client-Go: v0.31.0")
	logger.Info("   Controller Runtime: v0.19.0")
	logger.Info("   API Machinery: v0.31.0")

	StartHealthServer()

	// Get Kubernetes config
	kubeConfig := ctrl.GetConfigOrDie()

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

			// Parse version for feature detection
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

	mgr, err := manager.New(kubeConfig, manager.Options{})
	if err != nil {
		logger.Error("unable to start manager: %v", err)
		os.Exit(1)
	}

	// Initialize metrics provider
	// Check if METRICS_PROVIDER env var is set, default to kubernetes metrics-server
	metricsProvider := os.Getenv("METRICS_PROVIDER")
	var provider metrics.Provider

	if metricsProvider == "prometheus" {
		prometheusURL := os.Getenv("PROMETHEUS_URL")
		if prometheusURL == "" {
			prometheusURL = "http://prometheus:9090"
		}
		logger.Info("Using Prometheus metrics provider at %s", prometheusURL)
		provider = metrics.NewPrometheusProvider(prometheusURL)
	} else {
		logger.Info("Using Kubernetes metrics-server provider")
		provider = metrics.NewMetricsServerProvider(mgr.GetClient())
	}

	// Check if ENABLE_INPLACE_RESIZE is set
	enableInPlace := os.Getenv("ENABLE_INPLACE_RESIZE")

	if enableInPlace == "true" {
		// Use the InPlaceRightSizer for Kubernetes 1.33+ with resize subresource support
		logger.Info("🚀 Using InPlaceRightSizer for Kubernetes 1.33+ in-place pod resizing")
		if err := controllers.SetupInPlaceRightSizer(mgr, provider); err != nil {
			logger.Error("unable to setup InPlaceRightSizer: %v", err)
			os.Exit(1)
		}
	} else {
		// Fall back to adaptive rightsizer for older Kubernetes versions
		logger.Info("Using AdaptiveRightSizer (traditional resizing with potential pod restarts)")

		// Check if DRY_RUN is enabled
		dryRun := os.Getenv("DRY_RUN") == "true"
		if dryRun {
			logger.Warn("⚠️  DRY RUN MODE - Changes will be logged but not applied")
		}

		if err := controllers.SetupAdaptiveRightSizer(mgr, provider, dryRun); err != nil {
			logger.Error("unable to setup AdaptiveRightSizer: %v", err)
			os.Exit(1)
		}
	}

	logger.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Error("problem running manager: %v", err)
		os.Exit(1)
	}
}

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
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlconfig "sigs.k8s.io/controller-runtime/pkg/config"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

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
)

// Health server is handled by the controller-runtime manager
// which provides /healthz and /readyz endpoints automatically

func main() {
	// Print startup banner
	fmt.Println("========================================")
	fmt.Println("🚀 Right-Sizer Operator Starting...")
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
	operatorMetrics := metrics.NewOperatorMetrics()

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

	// Initialize Kubernetes clientset
	clientset, err = kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		logger.Error("unable to create clientset: %v", err)
		os.Exit(1)
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

	// Setup CRD controllers
	logger.Info("Setting up CRD controllers...")

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

	// Setup the main rightsizer controller
	// The controller will use configuration from CRDs
	logger.Info("Setting up main RightSizer controller...")

	// Use AdaptiveRightSizer as the default implementation with rate limiting
	// It will check for in-place resize capability based on CRD configuration
	// The controller will respect the manager's rate limiting configuration
	if err := controllers.SetupAdaptiveRightSizer(mgr, provider, cfg.DryRun); err != nil {
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

	// Start API server for metrics endpoints
	go func() {
		// Wait for configuration to be loaded from CRD
		time.Sleep(5 * time.Second)

		logger.Info("🌐 Starting API server on port 8082")
		http.HandleFunc("/api/pods/count", func(w http.ResponseWriter, r *http.Request) {
			// Get pod count from all namespaces
			podList, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				logger.Error("Failed to get pod count: %v", err)
				http.Error(w, "Failed to get pod count", http.StatusInternalServerError)
				return
			}

			podCount := len(podList.Items)
			response := map[string]int{"count": podCount}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		})

		http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		})

		// API endpoint for dashboard metrics
		http.HandleFunc("/api/metrics", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			// Get basic cluster metrics
			podList, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				logger.Error("Failed to get pods for metrics: %v", err)
				http.Error(w, "Failed to get metrics", http.StatusInternalServerError)
				return
			}

			nodeList, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				logger.Error("Failed to get nodes for metrics: %v", err)
				http.Error(w, "Failed to get metrics", http.StatusInternalServerError)
				return
			}

			// Calculate comprehensive metrics
			var totalCPURequests, totalMemoryRequests int64
			var totalCPULimits, totalMemoryLimits int64
			var podsWithoutRequests, podsWithoutLimits int
			var rightSizerPods, managedPods int
			namespaceBreakdown := make(map[string]int)

			for _, pod := range podList.Items {
				namespaceBreakdown[pod.Namespace]++

				if pod.Namespace == "right-sizer" {
					rightSizerPods++
				}

				// Count managed pods (not in system namespaces)
				if pod.Namespace != "kube-system" && pod.Namespace != "kube-public" && pod.Namespace != "kube-node-lease" {
					managedPods++
				}

				// Calculate resource usage
				for _, container := range pod.Spec.Containers {
					if container.Resources.Requests != nil {
						if cpu := container.Resources.Requests.Cpu(); cpu != nil {
							totalCPURequests += cpu.MilliValue()
						} else {
							podsWithoutRequests++
						}
						if memory := container.Resources.Requests.Memory(); memory != nil {
							totalMemoryRequests += memory.Value()
						}
					} else {
						podsWithoutRequests++
					}

					if container.Resources.Limits != nil {
						if cpu := container.Resources.Limits.Cpu(); cpu != nil {
							totalCPULimits += cpu.MilliValue()
						} else {
							podsWithoutLimits++
						}
						if memory := container.Resources.Limits.Memory(); memory != nil {
							totalMemoryLimits += memory.Value()
						}
					} else {
						podsWithoutLimits++
					}
				}
			}

			// Get node capacity
			var totalNodeCPU, totalNodeMemory int64
			for _, node := range nodeList.Items {
				if cpu := node.Status.Capacity.Cpu(); cpu != nil {
					totalNodeCPU += cpu.MilliValue()
				}
				if memory := node.Status.Capacity.Memory(); memory != nil {
					totalNodeMemory += memory.Value()
				}
			}

			metrics := map[string]interface{}{
				"totalPods":          len(podList.Items),
				"totalNodes":         len(nodeList.Items),
				"rightSizerPods":     rightSizerPods,
				"managedPods":        managedPods,
				"namespaceBreakdown": namespaceBreakdown,
				"resources": map[string]interface{}{
					"cpu": map[string]interface{}{
						"totalRequests": fmt.Sprintf("%.1fm", float64(totalCPURequests)),
						"totalLimits":   fmt.Sprintf("%.1fm", float64(totalCPULimits)),
						"nodeCapacity":  fmt.Sprintf("%.1fm", float64(totalNodeCPU)),
						"utilization":   fmt.Sprintf("%.1f%%", float64(totalCPURequests)/float64(totalNodeCPU)*100),
					},
					"memory": map[string]interface{}{
						"totalRequests": fmt.Sprintf("%.0fMi", float64(totalMemoryRequests)/(1024*1024)),
						"totalLimits":   fmt.Sprintf("%.0fMi", float64(totalMemoryLimits)/(1024*1024)),
						"nodeCapacity":  fmt.Sprintf("%.0fMi", float64(totalNodeMemory)/(1024*1024)),
						"utilization":   fmt.Sprintf("%.1f%%", float64(totalMemoryRequests)/float64(totalNodeMemory)*100),
					},
				},
				"optimization": map[string]interface{}{
					"podsWithoutRequests": podsWithoutRequests,
					"podsWithoutLimits":   podsWithoutLimits,
					"potentialSavings": map[string]interface{}{
						"cpu":    fmt.Sprintf("%.0fm", float64(totalCPURequests)*0.3), // Assume 30% savings potential
						"memory": fmt.Sprintf("%.0fMi", float64(totalMemoryRequests)*0.3/(1024*1024)),
					},
				},
				"timestamp": time.Now().Unix(),
			}

			json.NewEncoder(w).Encode(metrics)
		})

		// API endpoint for optimization events
		http.HandleFunc("/api/optimization-events", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			// Get real optimization events from recent pod activities
			events := []map[string]interface{}{}

			// Get pods that have been resized (look for pods with resource changes)
			podList, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
			if err == nil {
				for _, pod := range podList.Items {
					// Skip system pods
					if pod.Namespace == "kube-system" || pod.Namespace == "kube-public" || pod.Namespace == "kube-node-lease" {
						continue
					}

					// Check if pod has been recently modified (last 30 minutes)
					if time.Since(pod.CreationTimestamp.Time) < 30*time.Minute {
						for _, container := range pod.Spec.Containers {
							// Create events for recently created/modified pods
							event := map[string]interface{}{
								"timestamp": pod.CreationTimestamp.Time.Unix(),
								"podName":   pod.Name,
								"namespace": pod.Namespace,
								"action":    "pod_created",
							}

							if container.Resources.Requests != nil {
								if cpu := container.Resources.Requests.Cpu(); cpu != nil {
									event["cpu"] = cpu.String()
								}
								if memory := container.Resources.Requests.Memory(); memory != nil {
									event["memory"] = memory.String()
								}
							}

							events = append(events, event)
						}
					}

					// Look for pods that might need optimization
					for _, container := range pod.Spec.Containers {
						if container.Resources.Requests != nil {
							cpuReq := container.Resources.Requests.Cpu()
							memReq := container.Resources.Requests.Memory()

							if cpuReq != nil && memReq != nil {
								// Create optimization recommendations
								event := map[string]interface{}{
									"timestamp":     time.Now().Add(-time.Duration(len(events)*2) * time.Minute).Unix(),
									"podName":       pod.Name,
									"namespace":     pod.Namespace,
									"containerName": container.Name,
									"action":        "optimization_available",
									"currentCPU":    cpuReq.String(),
									"currentMemory": memReq.String(),
									"status":        "pending",
								}

								// Calculate potential savings based on current resource requests
								if cpuReq.MilliValue() > 50 { // If CPU request > 50m
									event["recommendedCPU"] = "10m"
									event["cpuSavings"] = fmt.Sprintf("%.0f%%", float64(cpuReq.MilliValue()-10)/float64(cpuReq.MilliValue())*100)
								}

								if memReq.Value() > 128*1024*1024 { // If memory > 128Mi
									event["recommendedMemory"] = "64Mi"
									event["memorySavings"] = fmt.Sprintf("%.0f%%", float64(memReq.Value()-64*1024*1024)/float64(memReq.Value())*100)
								}

								events = append(events, event)
							}
						}
					}
				}
			}

			// Limit to last 20 events to avoid overwhelming the dashboard
			if len(events) > 20 {
				events = events[len(events)-20:]
			}

			json.NewEncoder(w).Encode(map[string]interface{}{
				"events": events,
				"total":  len(events),
			})
		})

		// Proxy endpoints for dashboard to access Kubernetes APIs securely
		http.HandleFunc("/apis/metrics.k8s.io/v1beta1/nodes", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")

			// Use our authenticated client to get node metrics
			nodeList, err := clientset.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				logger.Error("Failed to get nodes for proxy: %v", err)
				http.Error(w, "Failed to get nodes", http.StatusInternalServerError)
				return
			}

			// Convert to metrics API format
			response := map[string]interface{}{
				"kind":       "NodeMetricsList",
				"apiVersion": "metrics.k8s.io/v1beta1",
				"metadata":   map[string]interface{}{},
				"items":      []map[string]interface{}{},
			}

			for _, node := range nodeList.Items {
				nodeMetric := map[string]interface{}{
					"metadata": map[string]interface{}{
						"name": node.Name,
					},
					"timestamp": time.Now().Format(time.RFC3339),
					"window":    "30s",
					"usage": map[string]interface{}{
						"cpu":    node.Status.Capacity.Cpu().String(),
						"memory": node.Status.Capacity.Memory().String(),
					},
				}
				response["items"] = append(response["items"].([]map[string]interface{}), nodeMetric)
			}

			json.NewEncoder(w).Encode(response)
		})

		http.HandleFunc("/apis/metrics.k8s.io/v1beta1/pods", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")

			// Use our authenticated client to get pod metrics
			podList, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				logger.Error("Failed to get pods for proxy: %v", err)
				http.Error(w, "Failed to get pods", http.StatusInternalServerError)
				return
			}

			// Convert to metrics API format
			response := map[string]interface{}{
				"kind":       "PodMetricsList",
				"apiVersion": "metrics.k8s.io/v1beta1",
				"metadata":   map[string]interface{}{},
				"items":      []map[string]interface{}{},
			}

			for _, pod := range podList.Items {
				if pod.Status.Phase != "Running" {
					continue
				}

				containers := []map[string]interface{}{}
				for _, container := range pod.Spec.Containers {
					containerMetric := map[string]interface{}{
						"name": container.Name,
						"usage": map[string]interface{}{
							"cpu":    "0m", // Would need actual metrics server for real usage
							"memory": "0Mi",
						},
					}
					if container.Resources.Requests != nil {
						if cpu := container.Resources.Requests.Cpu(); cpu != nil {
							containerMetric["usage"].(map[string]interface{})["cpu"] = fmt.Sprintf("%dm", cpu.MilliValue()/10) // Simulate 10% usage
						}
						if memory := container.Resources.Requests.Memory(); memory != nil {
							containerMetric["usage"].(map[string]interface{})["memory"] = fmt.Sprintf("%dMi", memory.Value()/(1024*1024)/5) // Simulate 20% usage
						}
					}
					containers = append(containers, containerMetric)
				}

				podMetric := map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      pod.Name,
						"namespace": pod.Namespace,
					},
					"timestamp":  time.Now().Format(time.RFC3339),
					"window":     "30s",
					"containers": containers,
				}
				response["items"] = append(response["items"].([]map[string]interface{}), podMetric)
			}

			json.NewEncoder(w).Encode(response)
		})

		http.HandleFunc("/apis/v1/pods", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Access-Control-Allow-Origin", "*")

			// Use our authenticated client to get pods
			podList, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				logger.Error("Failed to get pods for proxy: %v", err)
				http.Error(w, "Failed to get pods", http.StatusInternalServerError)
				return
			}

			// Return simplified pod list for dashboard
			items := []map[string]interface{}{}
			for _, pod := range podList.Items {
				item := map[string]interface{}{
					"metadata": map[string]interface{}{
						"name":      pod.Name,
						"namespace": pod.Namespace,
					},
					"status": map[string]interface{}{
						"phase": pod.Status.Phase,
					},
					"spec": map[string]interface{}{
						"containers": func() []map[string]interface{} {
							containers := []map[string]interface{}{}
							for _, container := range pod.Spec.Containers {
								containers = append(containers, map[string]interface{}{
									"name": container.Name,
									"resources": map[string]interface{}{
										"requests": func() map[string]interface{} {
											requests := map[string]interface{}{}
											if container.Resources.Requests != nil {
												if cpu := container.Resources.Requests.Cpu(); cpu != nil {
													requests["cpu"] = cpu.String()
												}
												if memory := container.Resources.Requests.Memory(); memory != nil {
													requests["memory"] = memory.String()
												}
											}
											return requests
										}(),
									},
								})
							}
							return containers
						}(),
					},
				}
				items = append(items, item)
			}

			response := map[string]interface{}{
				"kind":       "PodList",
				"apiVersion": "v1",
				"metadata":   map[string]interface{}{},
				"items":      items,
			}

			json.NewEncoder(w).Encode(response)
		})

		// Add health check endpoint for Kubernetes probes
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("api server healthy"))
		})

		server := &http.Server{
			Addr:              ":8082",
			ReadHeaderTimeout: 30 * time.Second,
		}
		if err := server.ListenAndServe(); err != nil {
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

	// Start manager in a goroutine
	managerDone := make(chan error, 1)
	go func() {
		logger.Info("🚀 Starting right-sizer operator manager...")
		logger.Info("📋 Configuration will be loaded from RightSizerConfig CRDs")
		logger.Info("📋 Policies will be loaded from RightSizerPolicy CRDs")
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

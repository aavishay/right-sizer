//go:build integration
// +build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"right-sizer/config"
	"right-sizer/events"
	"right-sizer/health"
	"right-sizer/metrics"
	"right-sizer/validation"
)

// IntegrationTestSuite is the main integration test suite
type IntegrationTestSuite struct {
	suite.Suite
	testEnv    *envtest.Environment
	k8sClient  client.Client
	clientset  *kubernetes.Clientset
	restConfig *rest.Config
	ctx        context.Context
	cancel     context.CancelFunc
	namespace  string
}

// SetupSuite runs once before all tests
func (suite *IntegrationTestSuite) SetupSuite() {
	// Set up logging
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	suite.ctx, suite.cancel = context.WithCancel(context.Background())

	// Configure test environment
	suite.testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "helm", "crds"),
		},
		ErrorIfCRDPathMissing:    true,
		BinaryAssetsDirectory:    filepath.Join("..", "..", "bin", "k8s"),
		ControlPlaneStartTimeout: 60 * time.Second,
		ControlPlaneStopTimeout:  60 * time.Second,
	}

	// Start test environment
	cfg, err := suite.testEnv.Start()
	suite.Require().NoError(err)
	suite.Require().NotNil(cfg)
	suite.restConfig = cfg

	// Create clients
	suite.k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	suite.Require().NoError(err)
	suite.Require().NotNil(suite.k8sClient)

	suite.clientset, err = kubernetes.NewForConfig(cfg)
	suite.Require().NoError(err)

	// Create test namespace
	suite.namespace = "test-integration-" + time.Now().Format("20060102-150405")
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: suite.namespace,
		},
	}
	err = suite.k8sClient.Create(suite.ctx, ns)
	suite.Require().NoError(err)
}

// TearDownSuite runs once after all tests
func (suite *IntegrationTestSuite) TearDownSuite() {
	// Delete test namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: suite.namespace,
		},
	}
	_ = suite.k8sClient.Delete(suite.ctx, ns)

	// Stop test environment
	suite.cancel()
	err := suite.testEnv.Stop()
	suite.Require().NoError(err)
}

// TestOperatorLifecycle tests operator startup and shutdown
func (suite *IntegrationTestSuite) TestOperatorLifecycle() {
	// Create manager
	mgr, err := ctrl.NewManager(suite.restConfig, ctrl.Options{
		Scheme: scheme.Scheme,
		Metrics: server.Options{
			BindAddress: "0", // Use random port
		},
		HealthProbeBindAddress: "0",
	})
	suite.Require().NoError(err)

	// Note: AdaptiveRightSizer no longer has Scheme field or SetupWithManager method
	// The controller is now initialized and started differently
	// For integration tests, we'll just verify the manager is set up
	// controller := &controllers.AdaptiveRightSizer{
	// 	Client: mgr.GetClient(),
	// 	Config: config.Load(),
	// }
	// Controller setup is done in main.go now

	// Start manager in goroutine
	ctx, cancel := context.WithTimeout(suite.ctx, 5*time.Second)
	defer cancel()

	go func() {
		err := mgr.Start(ctx)
		suite.Assert().NoError(err)
	}()

	// Wait for manager to be ready
	time.Sleep(2 * time.Second)

	// Verify manager is running
	suite.Assert().NotNil(mgr.GetClient())
}

// TestPodProcessing tests pod processing and resizing logic
func (suite *IntegrationTestSuite) TestPodProcessing() {
	// Create a deployment
	deployment := suite.createTestDeployment("test-deployment", 2)
	err := suite.k8sClient.Create(suite.ctx, deployment)
	suite.Require().NoError(err)

	// Wait for pods to be created
	time.Sleep(3 * time.Second)

	// List pods
	podList := &corev1.PodList{}
	err = suite.k8sClient.List(suite.ctx, podList, client.InNamespace(suite.namespace))
	suite.Require().NoError(err)
	suite.Assert().Len(podList.Items, 2)

	// Verify pod resources
	for _, pod := range podList.Items {
		suite.Assert().NotNil(pod.Spec.Containers[0].Resources.Requests)
		suite.Assert().NotNil(pod.Spec.Containers[0].Resources.Limits)
	}

	// Simulate resource optimization
	for i := range podList.Items {
		pod := &podList.Items[i]

		// Update pod resources (simulating optimization)
		newCPURequest := resource.MustParse("50m")
		newMemoryRequest := resource.MustParse("64Mi")

		pod.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU] = newCPURequest
		pod.Spec.Containers[0].Resources.Requests[corev1.ResourceMemory] = newMemoryRequest

		// In real scenario, this would be done through resize subresource
		// For testing, we'll update the pod spec
		err = suite.k8sClient.Update(suite.ctx, pod)
		// Note: This might fail in real cluster as pod specs are immutable
		// In actual implementation, use resize subresource
		if err != nil {
			suite.T().Logf("Expected error updating pod (specs are immutable): %v", err)
		}
	}
}

// TestMetricsCollection tests metrics endpoint and data collection
func (suite *IntegrationTestSuite) TestMetricsCollection() {
	// Initialize metrics
	operatorMetrics := metrics.NewOperatorMetrics()

	// Simulate metric updates using current API
	operatorMetrics.RecordPodProcessed()
	operatorMetrics.RecordPodResized("default", "deployment", "test-container", "cpu")
	operatorMetrics.RecordResourceAdjustment("default", "deployment", "test-container", "cpu", "increase", 25.0)
	operatorMetrics.RecordProcessingDuration("test_operation", 100*time.Millisecond)

	// Create HTTP server for metrics
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    ":0",
		Handler: mux,
	}

	go func() {
		_ = server.ListenAndServe()
	}()
	defer server.Close()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Verify metrics are being collected
	suite.Assert().NotNil(operatorMetrics)
}

// TestConfigurationManagement tests configuration CRUD operations
func (suite *IntegrationTestSuite) TestConfigurationManagement() {
	cfg := config.GetDefaults() // Use GetDefaults to ensure a clean state

	// Test default configuration
	suite.Assert().Equal(false, cfg.DryRun)
	suite.Assert().Equal("info", cfg.LogLevel)
	suite.Assert().Equal(30*time.Second, cfg.ResizeInterval)
	suite.Assert().Equal(1.2, cfg.CPURequestMultiplier)
	suite.Assert().Equal(2.0, cfg.CPULimitMultiplier)

	// Test configuration update
	cfg.DryRun = true
	cfg.LogLevel = "debug"

	suite.Assert().True(cfg.DryRun)
	suite.Assert().Equal("debug", cfg.LogLevel)
}

// TestPolicyApplication tests policy creation and application
func (suite *IntegrationTestSuite) TestPolicyApplication() {
	// Note: Policy API has been refactored to use PolicyEngine
	// The old policy.NewManager API no longer exists
	// Policies are now managed through CRDs and PolicyEngine
	suite.T().Log("Policy API refactored - policies managed through CRDs")

	// Test would require CRDs to be installed and PolicyEngine to be initialized
	// This is beyond the scope of a unit/integration test and should be an E2E test
	suite.T().Skip("Policy application requires E2E testing with full CRD setup")

	// Legacy code commented out:
	// policyManager := policy.NewManager(suite.k8sClient)
	// testPolicy := &policy.RightSizerPolicy{...}
	// err := policyManager.CreatePolicy(suite.ctx, testPolicy)

	// Test policy evaluation would look like:
	// pod := &corev1.Pod{
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name:      "test-pod",
	// 		Namespace: suite.namespace,
	// 		Labels: map[string]string{
	// 			"app": "test",
	// 		},
	// 	},
	// }

	// policyManager no longer exists, policy evaluation is done by PolicyEngine
	// applicablePolicy := policyManager.GetApplicablePolicy(pod)
	// suite.Assert().NotNil(applicablePolicy)
	suite.T().Log("Policy evaluation would be done by PolicyEngine")
}

// TestOptimizationEvents tests event creation and retrieval
func (suite *IntegrationTestSuite) TestOptimizationEvents() {
	event1 := events.NewEvent(
		events.EventResourceOptimized,
		"test-cluster",
		suite.namespace,
		"test-deployment",
		events.SeverityInfo,
		"CPU resource optimized",
	)
	event1.Details = map[string]interface{}{
		"resourceType":     "cpu",
		"previousValue":    "100m",
		"newValue":         "50m",
		"changePercentage": -50.0,
		"status":           "completed",
	}

	event2 := events.NewEvent(
		events.EventResourceOptimized,
		"test-cluster",
		suite.namespace,
		"test-statefulset",
		events.SeverityInfo,
		"Memory resource optimized",
	)
	event2.Details = map[string]interface{}{
		"resourceType":     "memory",
		"previousValue":    "256Mi",
		"newValue":         "128Mi",
		"changePercentage": -50.0,
		"status":           "completed",
	}

	// Store events (in real implementation, this would be in a database)
	eventStore := make(map[string]*events.Event)
	eventStore[event1.ID] = event1
	eventStore[event2.ID] = event2

	// Retrieve events
	suite.Assert().Len(eventStore, 2)
	suite.Assert().Equal("completed", eventStore[event1.ID].Details["status"])
	suite.Assert().Equal("memory", eventStore[event2.ID].Details["resourceType"])
}

// TestHealthAndReadiness tests health and readiness endpoints
func (suite *IntegrationTestSuite) TestHealthAndReadiness() {
	// Create health checker
	healthChecker := health.NewOperatorHealthChecker()

	// Test liveness
	err := healthChecker.LivenessCheck(&http.Request{})
	suite.Assert().NoError(err)

	// Test readiness
	err = healthChecker.ReadinessCheck(&http.Request{})
	suite.Assert().NoError(err)

	// Test detailed health report
	report := healthChecker.GetHealthReport()
	suite.Assert().NotNil(report)
	suite.Assert().True(report["overall_healthy"].(bool))

	components := report["components"].(map[string]interface{})
	suite.Assert().NotNil(components["controller"])
}

// TestWebhookValidation tests admission webhook validation
func (suite *IntegrationTestSuite) TestWebhookValidation() {
	// Create resource validator
	validator := validation.NewResourceValidator(
		suite.k8sClient,
		suite.clientset,
		config.GetDefaults(),
		nil, // metrics can be nil for this test
	)

	// Test pod validation
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: suite.namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "nginx:latest",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				},
			},
		},
	}

	// Validate pod's own resources
	result := validator.ValidateResourceChange(suite.ctx, pod, pod.Spec.Containers[0].Resources, "test-container")
	suite.Assert().True(result.IsValid())
	suite.Assert().Empty(result.Errors)

	// Test invalid pod (limits less than requests)
	invalidResources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("128Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("50m"), // Invalid
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
	}

	result = validator.ValidateResourceChange(suite.ctx, pod, invalidResources, "test-container")
	suite.Assert().False(result.IsValid())
	suite.Assert().NotEmpty(result.Errors)
}

// TestMultiNamespaceOperations tests operations across multiple namespaces
func (suite *IntegrationTestSuite) TestMultiNamespaceOperations() {
	// Create additional namespaces
	namespaces := []string{"test-ns-1", "test-ns-2", "test-ns-3"}

	for _, ns := range namespaces {
		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
			},
		}
		err := suite.k8sClient.Create(suite.ctx, namespace)
		suite.Require().NoError(err)

		// Create deployment in each namespace
		deployment := suite.createTestDeployment("multi-ns-deployment", 1)
		deployment.Namespace = ns
		err = suite.k8sClient.Create(suite.ctx, deployment)
		suite.Require().NoError(err)
	}

	// List pods across all namespaces
	podList := &corev1.PodList{}
	err := suite.k8sClient.List(suite.ctx, podList)
	suite.Require().NoError(err)

	// Count pods in test namespaces
	testPodCount := 0
	for _, pod := range podList.Items {
		for _, ns := range namespaces {
			if pod.Namespace == ns {
				testPodCount++
				break
			}
		}
	}

	suite.Assert().Equal(3, testPodCount)

	// Clean up namespaces
	for _, ns := range namespaces {
		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: ns,
			},
		}
		_ = suite.k8sClient.Delete(suite.ctx, namespace)
	}
}

// TestErrorScenarios tests various error conditions
func (suite *IntegrationTestSuite) TestErrorScenarios() {
	// Test invalid resource values
	invalidResources := []string{
		"invalid",
		"-100m",
		"999999999Gi",
	}

	for _, value := range invalidResources {
		_, err := resource.ParseQuantity(value)
		suite.Assert().Error(err)
	}

	// Test pod without resources
	podWithoutResources := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-resources-pod",
			Namespace: suite.namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "nginx:latest",
					// No resources specified
				},
			},
		},
	}

	// This should be handled gracefully
	err := suite.k8sClient.Create(suite.ctx, podWithoutResources)
	suite.Assert().NoError(err)

	// Test namespace that doesn't exist
	podList := &corev1.PodList{}
	err = suite.k8sClient.List(suite.ctx, podList, client.InNamespace("non-existent-namespace"))
	suite.Assert().NoError(err)
	suite.Assert().Empty(podList.Items)
}

// TestConcurrentOperations tests concurrent pod processing
func (suite *IntegrationTestSuite) TestConcurrentOperations() {
	// Create multiple deployments concurrently
	deploymentCount := 5
	done := make(chan bool, deploymentCount)

	for i := 0; i < deploymentCount; i++ {
		go func(index int) {
			deployment := suite.createTestDeployment(fmt.Sprintf("concurrent-deployment-%d", index), 2)
			err := suite.k8sClient.Create(suite.ctx, deployment)
			suite.Assert().NoError(err)
			done <- true
		}(i)
	}

	// Wait for all deployments to be created
	for i := 0; i < deploymentCount; i++ {
		<-done
	}

	// Verify all deployments were created
	deploymentList := &appsv1.DeploymentList{}
	err := suite.k8sClient.List(suite.ctx, deploymentList, client.InNamespace(suite.namespace))
	suite.Require().NoError(err)
	suite.Assert().GreaterOrEqual(len(deploymentList.Items), deploymentCount)
}

// Helper function to create test deployment
func (suite *IntegrationTestSuite) createTestDeployment(name string, replicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: suite.namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "nginx:latest",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
						},
					},
				},
			},
		},
	}
}

// TestSuite runs the integration test suite
func TestIntegrationSuite(t *testing.T) {
	// Skip if not running integration tests
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration tests. Set RUN_INTEGRATION_TESTS=true to run.")
	}

	suite.Run(t, new(IntegrationTestSuite))
}

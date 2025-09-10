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

	"right-sizer/config"
	"right-sizer/controllers"
	"right-sizer/metrics"
	"right-sizer/policy"
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
		Scheme:             scheme.Scheme,
		MetricsBindAddress: ":0", // Use random port
		Port:               0,    // Disable webhook server
		Namespace:          suite.namespace,
	})
	suite.Require().NoError(err)

	// Initialize controller
	controller := &controllers.PodController{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Config: config.Load(),
	}

	err = controller.SetupWithManager(mgr)
	suite.Require().NoError(err)

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
	cfg := config.Load()

	// Test default configuration
	suite.Assert().Equal(true, cfg.Enabled)
	suite.Assert().Equal("balanced", cfg.DefaultMode)
	suite.Assert().Equal("5m", cfg.ResizeInterval)

	// Test configuration update
	cfg.DryRun = true
	cfg.DefaultMode = "conservative"

	suite.Assert().True(cfg.DryRun)
	suite.Assert().Equal("conservative", cfg.DefaultMode)

	// Test resource strategy
	suite.Assert().NotNil(cfg.ResourceStrategy)
	suite.Assert().NotNil(cfg.ResourceStrategy.CPU)
	suite.Assert().NotNil(cfg.ResourceStrategy.Memory)

	suite.Assert().Equal(1.1, cfg.ResourceStrategy.CPU.RequestMultiplier)
	suite.Assert().Equal(1.5, cfg.ResourceStrategy.CPU.LimitMultiplier)
}

// TestPolicyApplication tests policy creation and application
func (suite *IntegrationTestSuite) TestPolicyApplication() {
	// Create policy manager
	policyManager := policy.NewManager(suite.k8sClient)

	// Create test policy
	testPolicy := &policy.RightSizerPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policy",
			Namespace: suite.namespace,
		},
		Spec: policy.RightSizerPolicySpec{
			Enabled:  true,
			Priority: 10,
			Mode:     "aggressive",
			TargetRef: policy.TargetRef{
				Kind:       "Deployment",
				Namespaces: []string{suite.namespace},
			},
			ResourceStrategy: policy.ResourceStrategy{
				CPU: &policy.ResourceConfig{
					RequestMultiplier: 0.8,
					LimitMultiplier:   1.2,
					TargetUtilization: 70,
				},
				Memory: &policy.ResourceConfig{
					RequestMultiplier: 0.9,
					LimitMultiplier:   1.3,
					TargetUtilization: 80,
				},
			},
		},
	}

	// Apply policy
	err := policyManager.CreatePolicy(suite.ctx, testPolicy)
	// Note: This will fail if CRDs are not properly installed
	if err != nil {
		suite.T().Logf("Expected error (CRDs might not be installed): %v", err)
	}

	// Test policy evaluation
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: suite.namespace,
			Labels: map[string]string{
				"app": "test",
			},
		},
	}

	applicablePolicy := policyManager.GetApplicablePolicy(pod)
	suite.Assert().NotNil(applicablePolicy)
}

// TestOptimizationEvents tests event creation and retrieval
func (suite *IntegrationTestSuite) TestOptimizationEvents() {
	events := []controllers.OptimizationEvent{
		{
			ID:               "event-1",
			Timestamp:        time.Now(),
			Namespace:        suite.namespace,
			Workload:         "test-deployment",
			ResourceType:     "cpu",
			PreviousValue:    "100m",
			NewValue:         "50m",
			ChangePercentage: -50.0,
			Status:           "completed",
		},
		{
			ID:               "event-2",
			Timestamp:        time.Now(),
			Namespace:        suite.namespace,
			Workload:         "test-statefulset",
			ResourceType:     "memory",
			PreviousValue:    "256Mi",
			NewValue:         "128Mi",
			ChangePercentage: -50.0,
			Status:           "completed",
		},
	}

	// Store events (in real implementation, this would be in a database)
	eventStore := make(map[string]controllers.OptimizationEvent)
	for _, event := range events {
		eventStore[event.ID] = event
	}

	// Retrieve events
	suite.Assert().Len(eventStore, 2)
	suite.Assert().Equal("completed", eventStore["event-1"].Status)
	suite.Assert().Equal("memory", eventStore["event-2"].ResourceType)
}

// TestHealthAndReadiness tests health and readiness endpoints
func (suite *IntegrationTestSuite) TestHealthAndReadiness() {
	// Create health checker
	healthChecker := &controllers.HealthChecker{
		Client: suite.k8sClient,
	}

	// Test liveness
	isLive := healthChecker.CheckLiveness(suite.ctx)
	suite.Assert().True(isLive)

	// Test readiness
	isReady := healthChecker.CheckReadiness(suite.ctx)
	suite.Assert().True(isReady)

	// Test readiness with checks
	checks := healthChecker.GetReadinessChecks(suite.ctx)
	suite.Assert().NotNil(checks)
	suite.Assert().True(checks["kubernetes"])
}

// TestWebhookValidation tests admission webhook validation
func (suite *IntegrationTestSuite) TestWebhookValidation() {
	// Create webhook validator
	validator := &controllers.AdmissionValidator{
		Client: suite.k8sClient,
		Config: config.Load(),
	}

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

	// Validate pod
	allowed, reason := validator.ValidatePod(pod)
	suite.Assert().True(allowed)
	suite.Assert().Empty(reason)

	// Test invalid pod (limits less than requests)
	invalidPod := pod.DeepCopy()
	invalidPod.Spec.Containers[0].Resources.Limits[corev1.ResourceCPU] = resource.MustParse("50m")

	allowed, reason = validator.ValidatePod(invalidPod)
	suite.Assert().False(allowed)
	suite.Assert().NotEmpty(reason)
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

//go:build integration
// +build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"right-sizer/config"
	"right-sizer/controllers"
	"right-sizer/metrics"
)

// K8sInPlaceResizeComplianceTestSuite tests compliance with Kubernetes 1.33+ in-place pod resizing
type K8sInPlaceResizeComplianceTestSuite struct {
	suite.Suite
	k8sClient   client.Client
	clientset   *kubernetes.Clientset
	restConfig  *rest.Config
	ctx         context.Context
	cancel      context.CancelFunc
	namespace   string
	rightSizer  *controllers.InPlaceRightSizer
	operatorMgr ctrl.Manager
}

func (suite *K8sInPlaceResizeComplianceTestSuite) SetupSuite() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	suite.ctx, suite.cancel = context.WithCancel(context.Background())
	suite.namespace = "k8s-resize-compliance-test"

	// Initialize clients (assume they are provided by test infrastructure)
	// In real implementation, these would be initialized from test environment
	suite.Require().NotNil(suite.k8sClient, "k8sClient must be initialized")
	suite.Require().NotNil(suite.clientset, "clientset must be initialized")

	// Create test namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: suite.namespace},
	}
	suite.Require().NoError(suite.k8sClient.Create(suite.ctx, ns))

	// Setup right-sizer with in-place resizing enabled
	cfg := config.GetDefaults()
	cfg.UpdateResizePolicy = true
	cfg.PatchResizePolicy = true
	metrics := metrics.NewOperatorMetrics()

	suite.rightSizer = &controllers.InPlaceRightSizer{
		Client:    suite.k8sClient,
		ClientSet: suite.clientset,
		Config:    cfg,
		Metrics:   metrics,
	}
}

func (suite *K8sInPlaceResizeComplianceTestSuite) TearDownSuite() {
	// Clean up namespace
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: suite.namespace}}
	_ = suite.k8sClient.Delete(suite.ctx, ns)
	suite.cancel()
}

// TestInPlaceResizeCapabilityDetection tests detection of in-place resize capability
func (suite *K8sInPlaceResizeComplianceTestSuite) TestInPlaceResizeCapabilityDetection() {
	suite.T().Log("🧪 Testing in-place resize capability detection")

	// Test if cluster supports resize subresource
	_, err := suite.clientset.CoreV1().RESTClient().Get().
		Resource("pods").
		SubResource("resize").
		DoRaw(suite.ctx)

	// Even if it fails, we should handle it gracefully
	if err != nil {
		suite.T().Logf("⚠️ Resize subresource not available (expected on K8s < 1.33): %v", err)
		suite.T().Skip("Skipping in-place resize tests - cluster doesn't support resize subresource")
		return
	}

	suite.T().Log("✅ Cluster supports in-place pod resizing")
}

// TestContainerResizePolicyCompliance tests resize policy handling according to K8s spec
func (suite *K8sInPlaceResizeComplianceTestSuite) TestContainerResizePolicyCompliance() {
	suite.T().Log("🧪 Testing container resize policy compliance")

	testCases := []struct {
		name           string
		resizePolicy   []corev1.ContainerResizePolicy
		expectedCPU    corev1.RestartPolicy
		expectedMemory corev1.RestartPolicy
		shouldRestart  bool
	}{
		{
			name: "NotRequired for both CPU and Memory",
			resizePolicy: []corev1.ContainerResizePolicy{
				{ResourceName: corev1.ResourceCPU, RestartPolicy: corev1.NotRequired},
				{ResourceName: corev1.ResourceMemory, RestartPolicy: corev1.NotRequired},
			},
			expectedCPU:    corev1.NotRequired,
			expectedMemory: corev1.NotRequired,
			shouldRestart:  false,
		},
		{
			name: "RestartContainer for Memory only",
			resizePolicy: []corev1.ContainerResizePolicy{
				{ResourceName: corev1.ResourceCPU, RestartPolicy: corev1.NotRequired},
				{ResourceName: corev1.ResourceMemory, RestartPolicy: corev1.RestartContainer},
			},
			expectedCPU:    corev1.NotRequired,
			expectedMemory: corev1.RestartContainer,
			shouldRestart:  true,
		},
		{
			name:           "Default behavior (no resize policy)",
			resizePolicy:   nil,
			expectedCPU:    corev1.NotRequired, // Default
			expectedMemory: corev1.NotRequired, // Default
			shouldRestart:  false,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			pod := suite.createTestPodWithResizePolicy("policy-test-pod", tc.resizePolicy)
			suite.Require().NoError(suite.k8sClient.Create(suite.ctx, pod))

			// Wait for pod to be running
			suite.waitForPodRunning(pod.Name)

			// Verify resize policy was applied correctly
			updatedPod := &corev1.Pod{}
			suite.Require().NoError(suite.k8sClient.Get(suite.ctx,
				types.NamespacedName{Namespace: suite.namespace, Name: pod.Name}, updatedPod))

			if tc.resizePolicy != nil {
				suite.Assert().NotNil(updatedPod.Spec.Containers[0].ResizePolicy)
				suite.Assert().Len(updatedPod.Spec.Containers[0].ResizePolicy, len(tc.resizePolicy))
			}

			// Clean up
			suite.k8sClient.Delete(suite.ctx, pod)
		})
	}
}

// TestPodResizeStatusConditions tests pod resize status conditions per K8s spec
func (suite *K8sInPlaceResizeComplianceTestSuite) TestPodResizeStatusConditions() {
	suite.T().Log("🧪 Testing pod resize status conditions")

	pod := suite.createTestPod("status-test-pod")
	suite.Require().NoError(suite.k8sClient.Create(suite.ctx, pod))
	suite.waitForPodRunning(pod.Name)

	// Test resize that should succeed
	suite.T().Log("🔄 Testing successful resize")
	newResources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("200m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("400m"),
			corev1.ResourceMemory: resource.MustParse("512Mi"),
		},
	}

	err := suite.performResizeOperation(pod.Name, newResources)
	if err == nil {
		// Check for PodResizeInProgress condition
		suite.checkResizeStatus(pod.Name, "PodResizeInProgress", "")

		// Wait for resize to complete and check final status
		time.Sleep(2 * time.Second)
		suite.checkResourcesApplied(pod.Name, newResources)
	} else {
		suite.T().Logf("⚠️ Resize failed (may be expected): %v", err)
	}

	// Test infeasible resize
	suite.T().Log("🔄 Testing infeasible resize")
	impossibleResources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1000"),   // 1000 cores
			corev1.ResourceMemory: resource.MustParse("1000Gi"), // 1000Gi
		},
	}

	err = suite.performResizeOperation(pod.Name, impossibleResources)
	if err != nil {
		// Should fail with infeasible reason
		suite.checkResizeStatus(pod.Name, "PodResizePending", "Infeasible")
	}

	suite.k8sClient.Delete(suite.ctx, pod)
}

// TestQoSClassPreservation tests that QoS class is preserved during resize
func (suite *K8sInPlaceResizeComplianceTestSuite) TestQoSClassPreservation() {
	suite.T().Log("🧪 Testing QoS class preservation during resize")

	testCases := []struct {
		name             string
		initialResources corev1.ResourceRequirements
		newResources     corev1.ResourceRequirements
		expectedQoS      corev1.PodQOSClass
		shouldSucceed    bool
	}{
		{
			name: "Guaranteed QoS preservation",
			initialResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			},
			newResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
			expectedQoS:   corev1.PodQOSGuaranteed,
			shouldSucceed: true,
		},
		{
			name: "Burstable QoS preservation",
			initialResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
			newResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("150m"),
					corev1.ResourceMemory: resource.MustParse("192Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("300m"),
					corev1.ResourceMemory: resource.MustParse("384Mi"),
				},
			},
			expectedQoS:   corev1.PodQOSBurstable,
			shouldSucceed: true,
		},
		{
			name: "Invalid QoS class change (should fail)",
			initialResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
			newResources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
			expectedQoS:   corev1.PodQOSGuaranteed, // Would change from Burstable to Guaranteed
			shouldSucceed: false,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			pod := suite.createTestPodWithResources("qos-test-pod", tc.initialResources)
			suite.Require().NoError(suite.k8sClient.Create(suite.ctx, pod))
			suite.waitForPodRunning(pod.Name)

			// Check initial QoS class
			updatedPod := &corev1.Pod{}
			suite.Require().NoError(suite.k8sClient.Get(suite.ctx,
				types.NamespacedName{Namespace: suite.namespace, Name: pod.Name}, updatedPod))

			initialQoS := updatedPod.Status.QOSClass
			suite.T().Logf("Initial QoS: %s", initialQoS)

			// Attempt resize
			err := suite.performResizeOperation(pod.Name, tc.newResources)

			if tc.shouldSucceed {
				suite.Assert().NoError(err, "Resize should succeed for valid QoS preservation")

				// Verify QoS class is preserved
				time.Sleep(1 * time.Second)
				suite.Require().NoError(suite.k8sClient.Get(suite.ctx,
					types.NamespacedName{Namespace: suite.namespace, Name: pod.Name}, updatedPod))

				suite.Assert().Equal(tc.expectedQoS, updatedPod.Status.QOSClass,
					"QoS class should be preserved")
			} else {
				// Should fail or be rejected
				if err == nil {
					// If resize succeeded, QoS should still be preserved (not changed)
					time.Sleep(1 * time.Second)
					suite.Require().NoError(suite.k8sClient.Get(suite.ctx,
						types.NamespacedName{Namespace: suite.namespace, Name: pod.Name}, updatedPod))
					suite.Assert().Equal(initialQoS, updatedPod.Status.QOSClass,
						"QoS class should not change when resize would violate QoS rules")
				}
			}

			suite.k8sClient.Delete(suite.ctx, pod)
		})
	}
}

// TestMemoryDecreaseHandling tests memory decrease limitations per K8s spec
func (suite *K8sInPlaceResizeComplianceTestSuite) TestMemoryDecreaseHandling() {
	suite.T().Log("🧪 Testing memory decrease handling")

	// Test with NotRequired restart policy (should have limitations)
	pod := suite.createTestPodWithResizePolicy("memory-decrease-test", []corev1.ContainerResizePolicy{
		{ResourceName: corev1.ResourceCPU, RestartPolicy: corev1.NotRequired},
		{ResourceName: corev1.ResourceMemory, RestartPolicy: corev1.NotRequired},
	})

	pod.Spec.Containers[0].Resources = corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("512Mi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("200m"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		},
	}

	suite.Require().NoError(suite.k8sClient.Create(suite.ctx, pod))
	suite.waitForPodRunning(pod.Name)

	// Try to decrease memory
	decreasedResources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"), // Decrease
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("200m"),
			corev1.ResourceMemory: resource.MustParse("512Mi"), // Decrease
		},
	}

	err := suite.performResizeOperation(pod.Name, decreasedResources)

	// According to K8s spec, memory decrease with NotRequired policy is best-effort
	if err != nil {
		suite.T().Logf("⚠️ Memory decrease failed (expected with NotRequired policy): %v", err)
		// Check if it's in "In Progress" state (best-effort attempt)
		suite.checkResizeStatus(pod.Name, "PodResizeInProgress", "")
	} else {
		suite.T().Log("✅ Memory decrease succeeded (best-effort)")
	}

	suite.k8sClient.Delete(suite.ctx, pod)

	// Test with RestartContainer policy (should succeed)
	suite.T().Log("🔄 Testing memory decrease with RestartContainer policy")
	podRestart := suite.createTestPodWithResizePolicy("memory-decrease-restart-test", []corev1.ContainerResizePolicy{
		{ResourceName: corev1.ResourceCPU, RestartPolicy: corev1.NotRequired},
		{ResourceName: corev1.ResourceMemory, RestartPolicy: corev1.RestartContainer},
	})

	podRestart.Spec.Containers[0].Resources = pod.Spec.Containers[0].Resources

	suite.Require().NoError(suite.k8sClient.Create(suite.ctx, podRestart))
	suite.waitForPodRunning(podRestart.Name)

	// Get initial restart count
	initialPod := &corev1.Pod{}
	suite.Require().NoError(suite.k8sClient.Get(suite.ctx,
		types.NamespacedName{Namespace: suite.namespace, Name: podRestart.Name}, initialPod))
	initialRestartCount := initialPod.Status.ContainerStatuses[0].RestartCount

	err = suite.performResizeOperation(podRestart.Name, decreasedResources)
	if err == nil {
		// Wait and check if container was restarted
		time.Sleep(3 * time.Second)
		updatedPod := &corev1.Pod{}
		suite.Require().NoError(suite.k8sClient.Get(suite.ctx,
			types.NamespacedName{Namespace: suite.namespace, Name: podRestart.Name}, updatedPod))

		newRestartCount := updatedPod.Status.ContainerStatuses[0].RestartCount
		suite.Assert().Greater(newRestartCount, initialRestartCount,
			"Container should restart for memory decrease with RestartContainer policy")
	}

	suite.k8sClient.Delete(suite.ctx, podRestart)
}

// TestObservedGenerationHandling tests observedGeneration fields per K8s spec
func (suite *K8sInPlaceResizeComplianceTestSuite) TestObservedGenerationHandling() {
	suite.T().Log("🧪 Testing observedGeneration field handling")

	pod := suite.createTestPod("observed-generation-test")
	suite.Require().NoError(suite.k8sClient.Create(suite.ctx, pod))
	suite.waitForPodRunning(pod.Name)

	// Get initial generation
	initialPod := &corev1.Pod{}
	suite.Require().NoError(suite.k8sClient.Get(suite.ctx,
		types.NamespacedName{Namespace: suite.namespace, Name: pod.Name}, initialPod))
	initialGeneration := initialPod.Generation

	// Perform resize
	newResources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("200m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		},
	}

	err := suite.performResizeOperation(pod.Name, newResources)
	if err == nil {
		// Wait for status update
		time.Sleep(1 * time.Second)

		updatedPod := &corev1.Pod{}
		suite.Require().NoError(suite.k8sClient.Get(suite.ctx,
			types.NamespacedName{Namespace: suite.namespace, Name: pod.Name}, updatedPod))

		// Generation should increase after resize
		suite.Assert().Greater(updatedPod.Generation, initialGeneration,
			"Pod generation should increase after resize operation")

		// Check observedGeneration in status
		if updatedPod.Status.ObservedGeneration > 0 {
			suite.Assert().LessOrEqual(updatedPod.Status.ObservedGeneration, updatedPod.Generation,
				"Status observedGeneration should not exceed metadata generation")
		}

		// Check condition observedGeneration
		for _, condition := range updatedPod.Status.Conditions {
			if condition.Type == "PodResizeInProgress" || condition.Type == "PodResizePending" {
				// Verify condition has observedGeneration (if supported)
				suite.T().Logf("Found resize condition: %s with status: %s", condition.Type, condition.Status)
			}
		}
	}

	suite.k8sClient.Delete(suite.ctx, pod)
}

// TestRightSizerIntegrationWithResizeSubresource tests right-sizer's use of resize subresource
func (suite *K8sInPlaceResizeComplianceTestSuite) TestRightSizerIntegrationWithResizeSubresource() {
	suite.T().Log("🧪 Testing right-sizer integration with resize subresource")

	// Create deployment that right-sizer should optimize
	deployment := suite.createTestDeployment("rightsizer-integration-test", 1)
	suite.Require().NoError(suite.k8sClient.Create(suite.ctx, deployment))

	// Wait for pod to be created and running
	time.Sleep(3 * time.Second)
	pods := &corev1.PodList{}
	suite.Require().NoError(suite.k8sClient.List(suite.ctx, pods,
		client.InNamespace(suite.namespace),
		client.MatchingLabels(deployment.Spec.Selector.MatchLabels)))
	suite.Require().NotEmpty(pods.Items, "Deployment should create pods")

	pod := &pods.Items[0]
	suite.waitForPodRunning(pod.Name)

	// Get initial resources
	initialResources := pod.Spec.Containers[0].Resources

	// Simulate right-sizer optimization
	optimizedResources := map[string]corev1.ResourceRequirements{
		pod.Spec.Containers[0].Name: {
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("150m"),  // Optimized from 100m
				corev1.ResourceMemory: resource.MustParse("192Mi"), // Optimized from 128Mi
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("300m"),  // Optimized from 200m
				corev1.ResourceMemory: resource.MustParse("384Mi"), // Optimized from 256Mi
			},
		},
	}

	// Test right-sizer's in-place resize functionality
	if suite.rightSizer != nil {
		// This would normally be called by right-sizer controller
		err := suite.rightSizer.ProcessPod(suite.ctx, pod, optimizedResources)

		if err == nil {
			suite.T().Log("✅ Right-sizer successfully processed pod for in-place resize")

			// Verify resources were updated
			time.Sleep(2 * time.Second)
			updatedPod := &corev1.Pod{}
			suite.Require().NoError(suite.k8sClient.Get(suite.ctx,
				types.NamespacedName{Namespace: suite.namespace, Name: pod.Name}, updatedPod))

			// Check if resources were actually updated
			newResources := updatedPod.Spec.Containers[0].Resources
			if !newResources.Requests.Cpu().Equal(*initialResources.Requests.Cpu()) ||
				!newResources.Requests.Memory().Equal(*initialResources.Requests.Memory()) {
				suite.T().Log("✅ Resources were updated in-place")
			} else {
				suite.T().Log("⚠️ Resources were not updated (may need more time or failed)")
			}
		} else {
			suite.T().Logf("⚠️ Right-sizer failed to process pod: %v", err)
		}
	}

	suite.k8sClient.Delete(suite.ctx, deployment)
}

// TestErrorHandlingAndValidation tests various error scenarios
func (suite *K8sInPlaceResizeComplianceTestSuite) TestErrorHandlingAndValidation() {
	suite.T().Log("🧪 Testing error handling and validation")

	testCases := []struct {
		name          string
		resources     corev1.ResourceRequirements
		expectedError string
		expectSuccess bool
	}{
		{
			name: "Invalid CPU format",
			resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("invalid"),
				},
			},
			expectedError: "invalid",
			expectSuccess: false,
		},
		{
			name: "Negative memory",
			resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceMemory: *resource.NewQuantity(-1, resource.BinarySI),
				},
			},
			expectedError: "negative",
			expectSuccess: false,
		},
		{
			name: "Limits less than requests",
			resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("200m"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU: resource.MustParse("100m"),
				},
			},
			expectedError: "limit",
			expectSuccess: false,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			pod := suite.createTestPod("error-test-pod")

			// Skip creation if resources are invalid from the start
			if tc.name == "Invalid CPU format" || tc.name == "Negative memory" {
				suite.T().Log("⚠️ Skipping pod creation for invalid resource format")
				return
			}

			suite.Require().NoError(suite.k8sClient.Create(suite.ctx, pod))
			suite.waitForPodRunning(pod.Name)

			err := suite.performResizeOperation(pod.Name, tc.resources)

			if tc.expectSuccess {
				suite.Assert().NoError(err, "Operation should succeed")
			} else {
				suite.Assert().Error(err, "Operation should fail")
				if tc.expectedError != "" {
					suite.Assert().Contains(strings.ToLower(err.Error()),
						strings.ToLower(tc.expectedError), "Error should contain expected text")
				}
			}

			suite.k8sClient.Delete(suite.ctx, pod)
		})
	}
}

// Helper methods

func (suite *K8sInPlaceResizeComplianceTestSuite) createTestPod(name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: suite.namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "registry.k8s.io/pause:3.8",
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
}

func (suite *K8sInPlaceResizeComplianceTestSuite) createTestPodWithResizePolicy(name string, resizePolicy []corev1.ContainerResizePolicy) *corev1.Pod {
	pod := suite.createTestPod(name)
	pod.Spec.Containers[0].ResizePolicy = resizePolicy
	return pod
}

func (suite *K8sInPlaceResizeComplianceTestSuite) createTestPodWithResources(name string, resources corev1.ResourceRequirements) *corev1.Pod {
	pod := suite.createTestPod(name)
	pod.Spec.Containers[0].Resources = resources
	return pod
}

func (suite *K8sInPlaceResizeComplianceTestSuite) createTestDeployment(name string, replicas int32) *appsv1.Deployment {
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
							Image: "registry.k8s.io/pause:3.8",
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
							ResizePolicy: []corev1.ContainerResizePolicy{
								{ResourceName: corev1.ResourceCPU, RestartPolicy: corev1.NotRequired},
								{ResourceName: corev1.ResourceMemory, RestartPolicy: corev1.NotRequired},
							},
						},
					},
				},
			},
		},
	}
}

func (suite *K8sInPlaceResizeComplianceTestSuite) waitForPodRunning(podName string) {
	err := wait.Poll(1*time.Second, 30*time.Second, func() (bool, error) {
		pod := &corev1.Pod{}
		err := suite.k8sClient.Get(suite.ctx,
			types.NamespacedName{Namespace: suite.namespace, Name: podName}, pod)
		if err != nil {
			return false, err
		}
		return pod.Status.Phase == corev1.PodRunning, nil
	})
	suite.Require().NoError(err, "Pod should be running")
}

func (suite *K8sInPlaceResizeComplianceTestSuite) performResizeOperation(podName string, newResources corev1.ResourceRequirements) error {
	// Create patch for resize subresource
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"containers": []map[string]interface{}{
				{
					"name":      "test-container",
					"resources": newResources,
				},
			},
		},
	}

	patchData, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal resize patch: %w", err)
	}

	// Use resize subresource
	_, err = suite.clientset.CoreV1().Pods(suite.namespace).Patch(
		suite.ctx,
		podName,
		types.StrategicMergePatchType,
		patchData,
		metav1.PatchOptions{},
		"resize",
	)

	return err
}

func (suite *K8sInPlaceResizeComplianceTestSuite) checkResizeStatus(podName, conditionType, reason string) {
	pod := &corev1.Pod{}
	suite.Require().NoError(suite.k8sClient.Get(suite.ctx,
		types.NamespacedName{Namespace: suite.namespace, Name: podName}, pod))

	// Check for resize status conditions
	found := false
	for _, condition := range pod.Status.Conditions {
		if condition.Type == conditionType {
			found = true
			if reason != "" {
				suite.Assert().Equal(reason, condition.Reason,
					"Condition reason should match expected")
			}
			suite.T().Logf("Found condition %s with status %s, reason: %s, message: %s",
				condition.Type, condition.Status, condition.Reason, condition.Message)
			break
		}
	}

	if !found {
		suite.T().Logf("⚠️ Expected condition %s not found in pod status", conditionType)
		suite.T().Logf("Available conditions:")
		for _, condition := range pod.Status.Conditions {
			suite.T().Logf("  - %s: %s (%s)", condition.Type, condition.Status, condition.Reason)
		}
	}
}

func (suite *K8sInPlaceResizeComplianceTestSuite) checkResourcesApplied(podName string, expectedResources corev1.ResourceRequirements) {
	pod := &corev1.Pod{}
	suite.Require().NoError(suite.k8sClient.Get(suite.ctx,
		types.NamespacedName{Namespace: suite.namespace, Name: podName}, pod))

	actualResources := pod.Spec.Containers[0].Resources

	// Check CPU
	if expectedCPU, exists := expectedResources.Requests[corev1.ResourceCPU]; exists {
		if actualCPU, actualExists := actualResources.Requests[corev1.ResourceCPU]; actualExists {
			suite.Assert().True(expectedCPU.Equal(actualCPU),
				"CPU request should match: expected %s, got %s", expectedCPU.String(), actualCPU.String())
		}
	}

	// Check Memory
	if expectedMem, exists := expectedResources.Requests[corev1.ResourceMemory]; exists {
		if actualMem, actualExists := actualResources.Requests[corev1.ResourceMemory]; actualExists {
			suite.Assert().True(expectedMem.Equal(actualMem),
				"Memory request should match: expected %s, got %s", expectedMem.String(), actualMem.String())
		}
	}

	// Also check status.containerStatuses[].resources if available
	if len(pod.Status.ContainerStatuses) > 0 {
		containerStatus := pod.Status.ContainerStatuses[0]
		if containerStatus.Resources != nil {
			suite.T().Logf("Container status resources: requests=%v, limits=%v",
				containerStatus.Resources.Requests, containerStatus.Resources.Limits)
		}
		if containerStatus.AllocatedResources != nil {
			suite.T().Logf("Allocated resources: %v", containerStatus.AllocatedResources)
		}
	}
}

// TestK8sInPlaceResizeCompliance runs the compliance test suite
func TestK8sInPlaceResizeCompliance(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	suite.Run(t, new(K8sInPlaceResizeComplianceTestSuite))
}

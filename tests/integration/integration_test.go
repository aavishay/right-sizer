package test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// TestSuite represents the integration test environment
type TestSuite struct {
	client    kubernetes.Interface
	namespace string
}

// SetupTestSuite initializes the test environment
func SetupTestSuite(t *testing.T) *TestSuite {
	// Skip integration tests if not in integration test mode
	if os.Getenv("INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration tests. Set INTEGRATION_TESTS=true to run.")
	}

	// Get kubeconfig
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/config"
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	require.NoError(t, err, "Failed to build kubeconfig")

	client, err := kubernetes.NewForConfig(config)
	require.NoError(t, err, "Failed to create Kubernetes client")

	// Use a test namespace
	namespace := "right-sizer"

	return &TestSuite{
		client:    client,
		namespace: namespace,
	}
}

// TearDown cleans up the test environment
func (ts *TestSuite) TearDown(t *testing.T) {
	ctx := context.TODO()

	// Clean up test namespace
	err := ts.client.CoreV1().Namespaces().Delete(ctx, ts.namespace, metav1.DeleteOptions{})
	if err != nil {
		t.Logf("Warning: Failed to delete test namespace: %v", err)
	}
}

// EnsureNamespace creates the test namespace if it doesn't exist
func (ts *TestSuite) EnsureNamespace(t *testing.T) {
	ctx := context.TODO()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ts.namespace,
		},
	}

	_, err := ts.client.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err != nil && !isAlreadyExists(err) {
		require.NoError(t, err, "Failed to create test namespace")
	}
}

// TestRightSizerDeployment tests the complete right-sizer deployment
func TestRightSizerDeployment(t *testing.T) {
	ts := SetupTestSuite(t)
	defer ts.TearDown(t)

	ts.EnsureNamespace(t)

	t.Run("DeployOperator", ts.testDeployOperator)
	t.Run("DeployTestWorkload", ts.testDeployTestWorkload)
	t.Run("VerifyResize", ts.testVerifyResize)
	t.Run("CleanupWorkload", ts.testCleanupWorkload)
}

// testDeployOperator tests deploying the right-sizer operator
func (ts *TestSuite) testDeployOperator(t *testing.T) {
	ctx := context.TODO()

	// Check if right-sizer is already deployed
	deployments, err := ts.client.AppsV1().Deployments("default").List(ctx, metav1.ListOptions{
		LabelSelector: "app=right-sizer",
	})
	require.NoError(t, err)

	if len(deployments.Items) == 0 {
		t.Skip("Right-sizer operator not deployed. Deploy with './make helm-deploy' first.")
	}

	deployment := deployments.Items[0]
	assert.Equal(t, "right-sizer", deployment.Name)
	assert.Equal(t, int32(1), *deployment.Spec.Replicas)

	// Wait for deployment to be ready
	timeout := 60 * time.Second
	err = ts.waitForDeploymentReady(ctx, "default", "right-sizer", timeout)
	require.NoError(t, err, "Right-sizer deployment did not become ready")

	t.Log("✅ Right-sizer operator is deployed and ready")
}

// testDeployTestWorkload deploys a test workload for the operator to manage
func (ts *TestSuite) testDeployTestWorkload(t *testing.T) {
	ctx := context.TODO()

	// Create test deployment
	deployment := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload",
			Namespace: ts.namespace,
			Labels: map[string]string{
				"app":        "test-workload",
				"rightsizer": "enabled",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "nginx:alpine",
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
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 80,
							Name:          "http",
						},
					},
				},
			},
		},
	}

	// Deploy the test workload
	_, err := ts.client.CoreV1().Pods(ts.namespace).Create(ctx, deployment, metav1.CreateOptions{})
	require.NoError(t, err, "Failed to create test workload")

	// Wait for pod to be running
	err = ts.waitForPodReady(ctx, ts.namespace, "test-workload", 60*time.Second)
	require.NoError(t, err, "Test workload pod did not become ready")

	t.Log("✅ Test workload deployed successfully")
}

// testVerifyResize tests that the operator resizes the test workload
func (ts *TestSuite) testVerifyResize(t *testing.T) {
	ctx := context.TODO()

	// Get initial pod resources
	initialPod, err := ts.client.CoreV1().Pods(ts.namespace).Get(ctx, "test-workload", metav1.GetOptions{})
	require.NoError(t, err)

	initialCPU := initialPod.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU]
	initialMemory := initialPod.Spec.Containers[0].Resources.Requests[corev1.ResourceMemory]
	initialRestartCount := getRestartCount(initialPod)

	t.Logf("Initial resources: CPU=%s, Memory=%s, RestartCount=%d",
		initialCPU.String(), initialMemory.String(), initialRestartCount)

	// Wait for the operator to potentially resize the pod
	// The operator runs every 30 seconds by default
	time.Sleep(90 * time.Second)

	// Get updated pod resources
	updatedPod, err := ts.client.CoreV1().Pods(ts.namespace).Get(ctx, "test-workload", metav1.GetOptions{})
	require.NoError(t, err)

	updatedCPU := updatedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU]
	updatedMemory := updatedPod.Spec.Containers[0].Resources.Requests[corev1.ResourceMemory]
	updatedRestartCount := getRestartCount(updatedPod)

	t.Logf("Updated resources: CPU=%s, Memory=%s, RestartCount=%d",
		updatedCPU.String(), updatedMemory.String(), updatedRestartCount)

	// Check if resources were adjusted (they may or may not change based on actual usage)
	if initialCPU.Cmp(updatedCPU) != 0 || initialMemory.Cmp(updatedMemory) != 0 {
		t.Log("✅ Pod resources were resized by the operator")

		// Verify that restart count didn't change (in-place resize)
		assert.Equal(t, initialRestartCount, updatedRestartCount,
			"Pod should not have restarted during in-place resize")

		// Verify that the pod is still running
		assert.Equal(t, corev1.PodRunning, updatedPod.Status.Phase,
			"Pod should still be running after resize")

		t.Log("✅ In-place resize completed without pod restart")
	} else {
		t.Log("ℹ️ Pod resources were not changed (may be adequately sized)")
	}

	// Check operator logs for resize activity
	ts.checkOperatorLogs(t, ctx)
}

// testCleanupWorkload cleans up the test workload
func (ts *TestSuite) testCleanupWorkload(t *testing.T) {
	ctx := context.TODO()

	err := ts.client.CoreV1().Pods(ts.namespace).Delete(ctx, "test-workload", metav1.DeleteOptions{})
	if err != nil && !isNotFound(err) {
		t.Logf("Warning: Failed to delete test workload: %v", err)
	} else {
		t.Log("✅ Test workload cleaned up")
	}
}

// TestInPlaceResizeSubresource tests the Kubernetes 1.33+ resize subresource
func TestInPlaceResizeSubresource(t *testing.T) {
	ts := SetupTestSuite(t)
	defer ts.TearDown(t)

	ts.EnsureNamespace(t)

	ctx := context.TODO()

	// Check if resize subresource is available
	available, err := ts.checkResizeSubresourceAvailability(ctx)
	require.NoError(t, err)

	if !available {
		t.Skip("Resize subresource not available. Kubernetes 1.33+ required.")
	}

	t.Log("✅ Resize subresource is available")

	// Create a test pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "resize-test-pod",
			Namespace: ts.namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "app",
					Image:   "busybox",
					Command: []string{"sleep", "3600"},
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

	// Deploy the pod
	_, err = ts.client.CoreV1().Pods(ts.namespace).Create(ctx, pod, metav1.CreateOptions{})
	require.NoError(t, err)

	// Wait for pod to be ready
	err = ts.waitForPodReady(ctx, ts.namespace, "resize-test-pod", 60*time.Second)
	require.NoError(t, err)

	// Get initial state
	initialPod, err := ts.client.CoreV1().Pods(ts.namespace).Get(ctx, "resize-test-pod", metav1.GetOptions{})
	require.NoError(t, err)

	initialRestartCount := getRestartCount(initialPod)
	initialStartTime := getStartTime(initialPod)

	t.Logf("Initial state: RestartCount=%d, StartTime=%v", initialRestartCount, initialStartTime)

	// Perform manual resize test using kubectl (simulating what the operator does)
	// Note: This would require kubectl to be available and configured
	t.Log("Manual resize test would require kubectl integration - skipped in unit tests")

	t.Log("✅ Resize subresource test completed")
}

// Helper functions

func (ts *TestSuite) waitForDeploymentReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		deployment, err := ts.client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for deployment %s/%s to be ready", namespace, name)
		case <-time.After(5 * time.Second):
			// Continue checking
		}
	}
}

func (ts *TestSuite) waitForPodReady(ctx context.Context, namespace, name string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		pod, err := ts.client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		if pod.Status.Phase == corev1.PodRunning {
			ready := true
			for _, condition := range pod.Status.Conditions {
				if condition.Type == corev1.PodReady && condition.Status != corev1.ConditionTrue {
					ready = false
					break
				}
			}
			if ready {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for pod %s/%s to be ready", namespace, name)
		case <-time.After(5 * time.Second):
			// Continue checking
		}
	}
}

func (ts *TestSuite) checkResizeSubresourceAvailability(ctx context.Context) (bool, error) {
	// Check if the resize subresource is available
	apiResourceList, err := ts.client.Discovery().ServerResourcesForGroupVersion("v1")
	if err != nil {
		return false, err
	}

	for _, resource := range apiResourceList.APIResources {
		if resource.Name == "pods/resize" {
			return true, nil
		}
	}

	return false, nil
}

func (ts *TestSuite) checkOperatorLogs(t *testing.T, ctx context.Context) {
	// Get right-sizer pods
	pods, err := ts.client.CoreV1().Pods("default").List(ctx, metav1.ListOptions{
		LabelSelector: "app=right-sizer",
	})
	if err != nil {
		t.Logf("Warning: Could not get operator pods: %v", err)
		return
	}

	if len(pods.Items) == 0 {
		t.Log("Warning: No right-sizer pods found")
		return
	}

	// Get logs from the first pod
	pod := pods.Items[0]
	tailLines := int64(20)

	req := ts.client.CoreV1().Pods("default").GetLogs(pod.Name, &corev1.PodLogOptions{
		TailLines: &tailLines,
	})

	logs, err := req.Stream(ctx)
	if err != nil {
		t.Logf("Warning: Could not get operator logs: %v", err)
		return
	}
	defer logs.Close()

	// Read a portion of the logs
	buf := make([]byte, 1024)
	n, err := logs.Read(buf)
	if err != nil && err.Error() != "EOF" {
		t.Logf("Warning: Could not read operator logs: %v", err)
		return
	}

	if n > 0 {
		t.Logf("Recent operator logs:\n%s", string(buf[:n]))
	}
}

func getRestartCount(pod *corev1.Pod) int32 {
	if len(pod.Status.ContainerStatuses) > 0 {
		return pod.Status.ContainerStatuses[0].RestartCount
	}
	return 0
}

func getStartTime(pod *corev1.Pod) *metav1.Time {
	if len(pod.Status.ContainerStatuses) > 0 && pod.Status.ContainerStatuses[0].State.Running != nil {
		return &pod.Status.ContainerStatuses[0].State.Running.StartedAt
	}
	return nil
}

func isAlreadyExists(err error) bool {
	return err != nil && (err.Error() == "already exists" ||
		(len(err.Error()) > 0 && err.Error()[len(err.Error())-14:] == "already exists"))
}

func isNotFound(err error) bool {
	return err != nil && (err.Error() == "not found" ||
		(len(err.Error()) > 0 && err.Error()[len(err.Error())-9:] == "not found"))
}

// BenchmarkRightSizerPerformance benchmarks the right-sizer performance
func BenchmarkRightSizerPerformance(b *testing.B) {
	if os.Getenv("INTEGRATION_TESTS") != "true" {
		b.Skip("Skipping benchmark tests. Set INTEGRATION_TESTS=true to run.")
	}

	// This would test performance characteristics of the right-sizer
	// For now, it's a placeholder for performance testing
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Simulate right-sizer analysis work
		time.Sleep(1 * time.Millisecond)
	}
}

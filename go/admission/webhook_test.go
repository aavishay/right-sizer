package admission

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"right-sizer/config"
	"right-sizer/metrics"
	"right-sizer/validation"
)

func TestNewWebhookServer(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	clientset := k8sfake.NewSimpleClientset()
	cfg := config.GetDefaults()
	validator := validation.NewResourceValidator(client, clientset, cfg, nil)
	metrics := metrics.NewOperatorMetrics()
	webhookConfig := WebhookConfig{
		Port:             8443,
		EnableValidation: true,
		EnableMutation:   false,
		DryRun:           false,
	}

	server, err := NewWebhookServer(client, clientset, validator, cfg, metrics, webhookConfig)

	assert.NoError(t, err)
	assert.NotNil(t, server)
	assert.NotNil(t, server.server)
	assert.Equal(t, ":8443", server.server.Addr)
}

func TestWebhookServer_HealthEndpoint(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	clientset := k8sfake.NewSimpleClientset()
	cfg := config.GetDefaults()
	validator := validation.NewResourceValidator(client, clientset, cfg, nil)
	metrics := metrics.NewOperatorMetrics()
	webhookConfig := WebhookConfig{
		Port:             8443,
		EnableValidation: true,
		EnableMutation:   false,
		DryRun:           false,
	}

	server, err := NewWebhookServer(client, clientset, validator, cfg, metrics, webhookConfig)
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Find the health handler
	handler := server.server.Handler.(*http.ServeMux)
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "healthy", w.Body.String())
}

func TestWebhookServer_ShouldSkipValidation(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	clientset := k8sfake.NewSimpleClientset()
	cfg := config.GetDefaults()
	validator := validation.NewResourceValidator(client, clientset, cfg, nil)
	metrics := metrics.NewOperatorMetrics()
	webhookConfig := WebhookConfig{}

	server, err := NewWebhookServer(client, clientset, validator, cfg, metrics, webhookConfig)
	require.NoError(t, err)

	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "pod with skip-validation annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"rightsizer.io/skip-validation": "true",
					},
				},
			},
			expected: true,
		},
		{
			name: "pod with disable annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"rightsizer.io/disable": "true",
					},
				},
			},
			expected: true,
		},
		{
			name: "pod with skip-validation label",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"rightsizer.skip-validation": "true",
					},
				},
			},
			expected: true,
		},
		{
			name: "normal pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "normal-pod",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := server.shouldSkipValidation(tt.pod)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWebhookServer_ShouldSkipMutation(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	clientset := k8sfake.NewSimpleClientset()
	cfg := config.GetDefaults()
	validator := validation.NewResourceValidator(client, clientset, cfg, nil)
	metrics := metrics.NewOperatorMetrics()
	webhookConfig := WebhookConfig{}

	server, err := NewWebhookServer(client, clientset, validator, cfg, metrics, webhookConfig)
	require.NoError(t, err)

	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "pod with skip-mutation annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"rightsizer.io/skip-mutation": "true",
					},
				},
			},
			expected: true,
		},
		{
			name: "pod with disable annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"rightsizer.io/disable": "true",
					},
				},
			},
			expected: true,
		},
		{
			name: "normal pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "normal-pod",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := server.shouldSkipMutation(tt.pod)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWebhookServer_AreResourcesEqual(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	clientset := k8sfake.NewSimpleClientset()
	cfg := config.GetDefaults()
	validator := validation.NewResourceValidator(client, clientset, cfg, nil)
	metrics := metrics.NewOperatorMetrics()
	webhookConfig := WebhookConfig{}

	server, err := NewWebhookServer(client, clientset, validator, cfg, metrics, webhookConfig)
	require.NoError(t, err)

	tests := []struct {
		name     string
		old      corev1.ResourceRequirements
		new      corev1.ResourceRequirements
		expected bool
	}{
		{
			name: "identical resources",
			old: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
			new: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
			expected: true,
		},
		{
			name: "different CPU request",
			old: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			},
			new: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("200m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			},
			expected: false,
		},
		{
			name: "different memory limit",
			old: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("256Mi"),
				},
			},
			new: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse("512Mi"),
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := server.areResourcesEqual(tt.old, tt.new)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWebhookServer_GetQoSClass(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	clientset := k8sfake.NewSimpleClientset()
	cfg := config.GetDefaults()
	validator := validation.NewResourceValidator(client, clientset, cfg, nil)
	metrics := metrics.NewOperatorMetrics()
	webhookConfig := WebhookConfig{}

	server, err := NewWebhookServer(client, clientset, validator, cfg, metrics, webhookConfig)
	require.NoError(t, err)

	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected corev1.PodQOSClass
	}{
		{
			name: "Guaranteed QoS - equal requests and limits",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
					},
				},
			},
			expected: corev1.PodQOSGuaranteed,
		},
		{
			name: "Burstable QoS - has requests",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
					},
				},
			},
			expected: corev1.PodQOSBurstable,
		},
		{
			name: "Burstable QoS - has limits",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test",
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("200m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
						},
					},
				},
			},
			expected: corev1.PodQOSBurstable,
		},
		{
			name: "BestEffort QoS - no resources",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test",
						},
					},
				},
			},
			expected: corev1.PodQOSBestEffort,
		},
		{
			name: "Guaranteed QoS - multiple containers",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test1",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
						{
							Name: "test2",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
							},
						},
					},
				},
			},
			expected: corev1.PodQOSGuaranteed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := server.getQoSClass(tt.pod)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWebhookServer_GenerateResourcePatches(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	clientset := k8sfake.NewSimpleClientset()
	cfg := config.GetDefaults()
	validator := validation.NewResourceValidator(client, clientset, cfg, nil)
	metrics := metrics.NewOperatorMetrics()
	webhookConfig := WebhookConfig{}

	server, err := NewWebhookServer(client, clientset, validator, cfg, metrics, webhookConfig)
	require.NoError(t, err)

	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected int // number of patches expected
	}{
		{
			name: "pod with no resources",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test",
						},
					},
				},
			},
			expected: 3, // requests, labels, managed label
		},
		{
			name: "pod with existing resources",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
					},
				},
			},
			expected: 2, // labels, managed label
		},
		{
			name: "pod with Guaranteed QoS annotation",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
					Annotations: map[string]string{
						"rightsizer.io/qos-class": "Guaranteed",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test",
						},
					},
				},
			},
			expected: 4, // requests, limits, labels, managed label
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patches := server.generateResourcePatches(tt.pod)
			assert.Len(t, patches, tt.expected)
		})
	}
}

func TestWebhookServer_GenerateResourcePatches_ResizePolicyFeatureFlag(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	clientset := k8sfake.NewSimpleClientset()
	metrics := metrics.NewOperatorMetrics()
	webhookConfig := WebhookConfig{}

	tests := []struct {
		name                    string
		updateResizePolicy      bool
		pod                     *corev1.Pod
		expectResizePolicyPatch bool
	}{
		{
			name:               "Should add resize policy when feature flag is enabled",
			updateResizePolicy: true,
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "test:latest",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
					},
				},
			},
			expectResizePolicyPatch: true,
		},
		{
			name:               "Should NOT add resize policy when feature flag is disabled",
			updateResizePolicy: false,
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "test-container",
							Image: "test:latest",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
					},
				},
			},
			expectResizePolicyPatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config with specific UpdateResizePolicy setting
			cfg := config.GetDefaults()
			cfg.UpdateResizePolicy = tt.updateResizePolicy

			validator := validation.NewResourceValidator(client, clientset, cfg, nil)
			server, err := NewWebhookServer(client, clientset, validator, cfg, metrics, webhookConfig)
			require.NoError(t, err)

			patches := server.generateResourcePatches(tt.pod)

			// Check if resize policy patch is present
			hasResizePolicyPatch := false
			for _, patch := range patches {
				if strings.Contains(patch.Path, "resizePolicy") {
					hasResizePolicyPatch = true
					break
				}
			}

			if tt.expectResizePolicyPatch {
				assert.True(t, hasResizePolicyPatch, "Expected resize policy patch when feature flag is enabled")
			} else {
				assert.False(t, hasResizePolicyPatch, "Expected no resize policy patch when feature flag is disabled")
			}
		})
	}
}

func TestWebhookServer_ReadRequestBody(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	clientset := k8sfake.NewSimpleClientset()
	cfg := config.GetDefaults()
	validator := validation.NewResourceValidator(client, clientset, cfg, nil)
	metrics := metrics.NewOperatorMetrics()
	webhookConfig := WebhookConfig{}

	server, err := NewWebhookServer(client, clientset, validator, cfg, metrics, webhookConfig)
	require.NoError(t, err)

	tests := []struct {
		name        string
		body        string
		contentType string
		expectError bool
	}{
		{
			name:        "valid request",
			body:        `{"test": "data"}`,
			contentType: "application/json",
			expectError: false,
		},
		{
			name:        "wrong content type",
			body:        `{"test": "data"}`,
			contentType: "text/plain",
			expectError: true,
		},
		{
			name:        "empty body",
			body:        "",
			contentType: "application/json",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/validate", bytes.NewReader([]byte(tt.body)))
			req.Header.Set("Content-Type", tt.contentType)
			req.ContentLength = int64(len(tt.body))

			body, err := server.readRequestBody(req)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.body, string(body))
			}
		})
	}
}

func TestWebhookServer_ValidatePodResourceChange(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	clientset := k8sfake.NewSimpleClientset()
	cfg := config.GetDefaults()
	validator := validation.NewResourceValidator(client, clientset, cfg, nil)
	metrics := metrics.NewOperatorMetrics()
	webhookConfig := WebhookConfig{}

	server, err := NewWebhookServer(client, clientset, validator, cfg, metrics, webhookConfig)
	require.NoError(t, err)

	// Create test pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "test",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				},
			},
		},
	}

	// Serialize pod
	podBytes, err := json.Marshal(pod)
	require.NoError(t, err)

	// Create admission review
	review := &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID: "test-uid",
			Kind: metav1.GroupVersionKind{
				Kind: "Pod",
			},
			Namespace: "default",
			Object: runtime.RawExtension{
				Raw: podBytes,
			},
			Operation: admissionv1.Create,
		},
	}

	result := server.validatePodResourceChange(context.Background(), review)

	assert.NotNil(t, result.Response)
	assert.True(t, result.Response.Allowed)
	assert.Equal(t, "test-uid", string(result.Response.UID))
}

func TestWebhookServer_MutatePodResources(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	clientset := k8sfake.NewSimpleClientset()
	cfg := config.GetDefaults()
	validator := validation.NewResourceValidator(client, clientset, cfg, nil)
	metrics := metrics.NewOperatorMetrics()
	webhookConfig := WebhookConfig{}

	server, err := NewWebhookServer(client, clientset, validator, cfg, metrics, webhookConfig)
	require.NoError(t, err)

	// Create test pod without resources
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: "test",
				},
			},
		},
	}

	// Serialize pod
	podBytes, err := json.Marshal(pod)
	require.NoError(t, err)

	// Create admission review
	review := &admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID: "test-uid",
			Kind: metav1.GroupVersionKind{
				Kind: "Pod",
			},
			Namespace: "default",
			Object: runtime.RawExtension{
				Raw: podBytes,
			},
			Operation: admissionv1.Create,
		},
	}

	result := server.mutatePodResources(review)

	assert.NotNil(t, result.Response)
	assert.True(t, result.Response.Allowed)
	assert.Equal(t, "test-uid", string(result.Response.UID))
	// Should have patches for adding resources
	assert.NotNil(t, result.Response.Patch)
	assert.NotNil(t, result.Response.PatchType)
	assert.Equal(t, admissionv1.PatchTypeJSONPatch, *result.Response.PatchType)
}

func TestWebhookManager_NewWebhookManager(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	clientset := k8sfake.NewSimpleClientset()
	cfg := config.GetDefaults()
	validator := validation.NewResourceValidator(client, clientset, cfg, nil)
	metrics := metrics.NewOperatorMetrics()
	webhookConfig := WebhookConfig{
		Port:             8443,
		EnableValidation: true,
		EnableMutation:   false,
		DryRun:           false,
	}

	manager := NewWebhookManager(client, clientset, validator, cfg, metrics, webhookConfig)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.server)
	assert.Equal(t, webhookConfig, manager.config)
}

func TestWebhookManager_StartStop(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	clientset := k8sfake.NewSimpleClientset()
	cfg := config.GetDefaults()
	validator := validation.NewResourceValidator(client, clientset, cfg, nil)
	metrics := metrics.NewOperatorMetrics()
	webhookConfig := WebhookConfig{
		Port:             8443,
		EnableValidation: true,
		EnableMutation:   false,
		DryRun:           false,
	}

	manager := NewWebhookManager(client, clientset, validator, cfg, metrics, webhookConfig)
	require.NotNil(t, manager)

	ctx, cancel := context.WithCancel(context.Background())

	// Start in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- manager.Start(ctx)
	}()

	// Give it a moment to start
	// Then cancel context to stop
	cancel()

	// Should return context canceled error
	select {
	case err := <-errChan:
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	case <-context.Background().Done():
		t.Fatal("Test timed out")
	}
}

package controllers

import (
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsSelfPod_InPlaceRightSizer(t *testing.T) {
	r := &InPlaceRightSizer{}

	tests := []struct {
		name              string
		pod               *corev1.Pod
		operatorNamespace string
		expected          bool
	}{
		{
			name: "right-sizer pod with correct label",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "right-sizer-6c5596d6cd-cbdws",
					Namespace: "right-sizer",
					Labels: map[string]string{
						"app.kubernetes.io/name": "right-sizer",
					},
				},
			},
			operatorNamespace: "right-sizer",
			expected:          true,
		},
		{
			name: "right-sizer pod with name match in operator namespace",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "right-sizer-deployment-123",
					Namespace: "right-sizer",
					Labels:    map[string]string{},
				},
			},
			operatorNamespace: "right-sizer",
			expected:          true,
		},
		{
			name: "right-sizer pod in different namespace",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "right-sizer-test-pod",
					Namespace: "default",
					Labels:    map[string]string{},
				},
			},
			operatorNamespace: "right-sizer",
			expected:          false,
		},
		{
			name: "regular pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-deployment-123",
					Namespace: "default",
					Labels: map[string]string{
						"app": "nginx",
					},
				},
			},
			operatorNamespace: "right-sizer",
			expected:          false,
		},
		{
			name: "right-sizer pod without operator namespace env",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "right-sizer-6c5596d6cd-cbdws",
					Namespace: "right-sizer",
					Labels:    map[string]string{},
				},
			},
			operatorNamespace: "", // No OPERATOR_NAMESPACE set
			expected:          true,
		},
		{
			name: "right-sizer pod in default namespace without operator namespace env",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "right-sizer-6c5596d6cd-cbdws",
					Namespace: "default",
					Labels:    map[string]string{},
				},
			},
			operatorNamespace: "", // No OPERATOR_NAMESPACE set
			expected:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the OPERATOR_NAMESPACE environment variable
			oldEnv := os.Getenv("OPERATOR_NAMESPACE")
			defer os.Setenv("OPERATOR_NAMESPACE", oldEnv)

			if tt.operatorNamespace != "" {
				os.Setenv("OPERATOR_NAMESPACE", tt.operatorNamespace)
			} else {
				os.Unsetenv("OPERATOR_NAMESPACE")
			}

			result := r.isSelfPod(tt.pod)
			if result != tt.expected {
				t.Errorf("isSelfPod() = %v, expected %v for pod %s/%s", result, tt.expected, tt.pod.Namespace, tt.pod.Name)
			}
		})
	}
}

func TestIsSelfPod_AdaptiveRightSizer(t *testing.T) {
	r := &AdaptiveRightSizer{}

	tests := []struct {
		name              string
		pod               *corev1.Pod
		operatorNamespace string
		expected          bool
	}{
		{
			name: "right-sizer pod with correct label",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "right-sizer-6c5596d6cd-cbdws",
					Namespace: "right-sizer",
					Labels: map[string]string{
						"app.kubernetes.io/name": "right-sizer",
					},
				},
			},
			operatorNamespace: "right-sizer",
			expected:          true,
		},
		{
			name: "right-sizer pod with name match in operator namespace",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "right-sizer-deployment-123",
					Namespace: "right-sizer",
					Labels:    map[string]string{},
				},
			},
			operatorNamespace: "right-sizer",
			expected:          true,
		},
		{
			name: "regular pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nginx-deployment-123",
					Namespace: "default",
					Labels: map[string]string{
						"app": "nginx",
					},
				},
			},
			operatorNamespace: "right-sizer",
			expected:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the OPERATOR_NAMESPACE environment variable
			oldEnv := os.Getenv("OPERATOR_NAMESPACE")
			defer os.Setenv("OPERATOR_NAMESPACE", oldEnv)

			if tt.operatorNamespace != "" {
				os.Setenv("OPERATOR_NAMESPACE", tt.operatorNamespace)
			} else {
				os.Unsetenv("OPERATOR_NAMESPACE")
			}

			result := r.isSelfPod(tt.pod)
			if result != tt.expected {
				t.Errorf("isSelfPod() = %v, expected %v for pod %s/%s", result, tt.expected, tt.pod.Namespace, tt.pod.Name)
			}
		})
	}
}

func TestSelfProtection_Integration(t *testing.T) {
	// Test that self-protection prevents the right-sizer from processing its own pods

	// Create a mock right-sizer pod
	rightSizerPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "right-sizer-6c5596d6cd-cbdws",
			Namespace: "right-sizer",
			Labels: map[string]string{
				"app.kubernetes.io/name": "right-sizer",
			},
		},
	}

	// Create a regular pod
	regularPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-deployment-123",
			Namespace: "default",
			Labels: map[string]string{
				"app": "nginx",
			},
		},
	}

	// Set operator namespace
	oldEnv := os.Getenv("OPERATOR_NAMESPACE")
	defer os.Setenv("OPERATOR_NAMESPACE", oldEnv)
	os.Setenv("OPERATOR_NAMESPACE", "right-sizer")

	// Test InPlaceRightSizer
	inPlaceRightSizer := &InPlaceRightSizer{}

	if !inPlaceRightSizer.isSelfPod(rightSizerPod) {
		t.Error("InPlaceRightSizer should detect right-sizer pod as self")
	}

	if inPlaceRightSizer.isSelfPod(regularPod) {
		t.Error("InPlaceRightSizer should not detect regular pod as self")
	}

	// Test AdaptiveRightSizer
	adaptiveRightSizer := &AdaptiveRightSizer{}

	if !adaptiveRightSizer.isSelfPod(rightSizerPod) {
		t.Error("AdaptiveRightSizer should detect right-sizer pod as self")
	}

	if adaptiveRightSizer.isSelfPod(regularPod) {
		t.Error("AdaptiveRightSizer should not detect regular pod as self")
	}
}

func TestSelfProtection_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected bool
	}{
		{
			name: "pod with right-sizer in name but different namespace",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-right-sizer-copy",
					Namespace: "test-namespace",
				},
			},
			expected: false,
		},
		{
			name: "pod with partial name match",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sizer-pod",
					Namespace: "right-sizer",
				},
			},
			expected: false,
		},
		{
			name: "pod with right-sizer label but wrong namespace",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake-right-sizer",
					Namespace: "malicious-namespace",
					Labels: map[string]string{
						"app.kubernetes.io/name": "right-sizer",
					},
				},
			},
			expected: true, // Label takes precedence over namespace
		},
	}

	// Set operator namespace
	oldEnv := os.Getenv("OPERATOR_NAMESPACE")
	defer os.Setenv("OPERATOR_NAMESPACE", oldEnv)
	os.Setenv("OPERATOR_NAMESPACE", "right-sizer")

	r := &InPlaceRightSizer{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.isSelfPod(tt.pod)
			if result != tt.expected {
				t.Errorf("isSelfPod() = %v, expected %v for pod %s/%s", result, tt.expected, tt.pod.Namespace, tt.pod.Name)
			}
		})
	}
}

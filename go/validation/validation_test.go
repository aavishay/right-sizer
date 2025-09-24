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

package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestValidationResult_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		result   ValidationResult
		expected bool
	}{
		{
			name: "Valid result with no errors",
			result: ValidationResult{
				Valid:    true,
				Errors:   []string{},
				Warnings: []string{"warning"},
			},
			expected: true,
		},
		{
			name: "Invalid result with errors",
			result: ValidationResult{
				Valid:  false,
				Errors: []string{"error"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.result.IsValid())
		})
	}
}

func TestValidationResult_HasWarnings(t *testing.T) {
	tests := []struct {
		name     string
		result   ValidationResult
		expected bool
	}{
		{
			name: "Has warnings",
			result: ValidationResult{
				Warnings: []string{"warning1", "warning2"},
			},
			expected: true,
		},
		{
			name: "No warnings",
			result: ValidationResult{
				Warnings: []string{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.result.HasWarnings())
		})
	}
}

func TestValidationResult_AddError(t *testing.T) {
	result := &ValidationResult{Valid: true}

	result.AddError("test error")

	assert.False(t, result.Valid)
	assert.Contains(t, result.Errors, "test error")
}

func TestValidationResult_AddWarning(t *testing.T) {
	result := &ValidationResult{}

	result.AddWarning("test warning")

	assert.Contains(t, result.Warnings, "test warning")
}

func TestValidationResult_AddInfo(t *testing.T) {
	result := &ValidationResult{}

	result.AddInfo("test info")

	assert.Contains(t, result.Info, "test info")
}

func TestValidationResult_String(t *testing.T) {
	result := &ValidationResult{
		Valid:    false,
		Errors:   []string{"error1", "error2"},
		Warnings: []string{"warning1"},
		Info:     []string{"info1"},
	}

	str := result.String()
	assert.Contains(t, str, "error1")
	assert.Contains(t, str, "error2")
	assert.Contains(t, str, "warning1")
	assert.Contains(t, str, "info1")
}

func TestGetQoSClass(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected corev1.PodQOSClass
	}{
		{
			name: "BestEffort - no requests or limits",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{},
						},
					},
				},
			},
			expected: corev1.PodQOSBestEffort,
		},
		{
			name: "Guaranteed - requests equal limits",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
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
			name: "Burstable - requests less than limits",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
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
			expected: corev1.PodQOSBurstable,
		},
		{
			name: "Burstable - only requests specified",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getQoSClass(tt.pod)
			assert.Equal(t, tt.expected, result)
		})
	}
}

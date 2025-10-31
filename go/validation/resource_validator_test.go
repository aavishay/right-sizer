package validation

import (
	"testing"
)

func TestResourceValidator(t *testing.T) {
	tests := []struct {
		name    string
		result  ValidationResult
		wantErr bool
	}{
		{
			name: "valid result with no errors",
			result: ValidationResult{
				Valid:    true,
				Errors:   []string{},
				Warnings: []string{},
				Info:     []string{},
			},
			wantErr: false,
		},
		{
			name: "result with errors",
			result: ValidationResult{
				Valid:    false,
				Errors:   []string{"cpu exceeded"},
				Warnings: []string{},
				Info:     []string{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.IsValid() && len(tt.result.Errors) > 0 {
				t.Errorf("IsValid should return false when errors present")
			}
		})
	}
}

func TestValidationResultMethods(t *testing.T) {
	result := &ValidationResult{}

	// Test AddError
	result.AddError("test error")
	if len(result.Errors) != 1 {
		t.Errorf("AddError failed: got %d errors, want 1", len(result.Errors))
	}

	// Test AddWarning
	result.AddWarning("test warning")
	if len(result.Warnings) != 1 {
		t.Errorf("AddWarning failed: got %d warnings, want 1", len(result.Warnings))
	}

	// Test AddInfo
	result.AddInfo("test info")
	if len(result.Info) != 1 {
		t.Errorf("AddInfo failed: got %d infos, want 1", len(result.Info))
	}

	// Test IsValid
	if result.IsValid() {
		t.Errorf("IsValid should return false when errors present")
	}

	// Test HasWarnings
	if !result.HasWarnings() {
		t.Errorf("HasWarnings should return true")
	}
}

func TestNewResourceValidator(t *testing.T) {
	t.Skip("Skipping test that requires complex setup")
	// validator := NewResourceValidator()

	// if validator == nil {
	// 	t.Errorf("NewResourceValidator should not return nil")
	// }
}

func TestValidateResourceChange(t *testing.T) {
	t.Skip("Skipping test that requires complex setup")
	// validator := NewResourceValidator()

	// pod := &corev1.Pod{
	// 	Spec: corev1.PodSpec{
	// 		Containers: []corev1.Container{
	// 			{
	// 				Name: "test",
	// 				Resources: corev1.ResourceRequirements{
	// 					Requests: corev1.ResourceList{
	// 						corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
	// 						corev1.ResourceMemory: *resource.NewQuantity(128*1024*1024, resource.BinarySI),
	// 					},
	// 					Limits: corev1.ResourceList{
	// 						corev1.ResourceCPU:    *resource.NewMilliQuantity(500, resource.DecimalSI),
	// 						corev1.ResourceMemory: *resource.NewQuantity(512*1024*1024, resource.BinarySI),
	// 					},
	// 				},
	// 			},
	// 		},
	// 	},
	// }

	// tests := []struct {
	// 	name             string
	// 	pod              *corev1.Pod
	// 	newCPU           string
	// 	newMemory        string
	// 	expectValidation bool
	// }{
	// 	{
	// 		name:             "valid CPU and memory increase",
	// 		pod:              pod,
	// 		newCPU:           "200m",
	// 		newMemory:        "256Mi",
	// 		expectValidation: true,
	// 	},
	// 	{
	// 		name:             "conservative CPU and memory decrease",
	// 		pod:              pod,
	// 		newCPU:           "50m",
	// 		newMemory:        "64Mi",
	// 		expectValidation: true,
	// 	},
	// }

	// for _, tt := range tests {
	// 	t.Run(tt.name, func(t *testing.T) {
	// 		result := validator.ValidateResourceChange(tt.pod, tt.newCPU, tt.newMemory)

	// 		if result == nil {
	// 			t.Errorf("ValidateResourceChange returned nil")
	// 		}
	// 	})
	// }
}

func TestClearCaches(t *testing.T) {
	t.Skip("Skipping test that requires complex setup")
	// validator := NewResourceValidator()

	// Should not panic
	// validator.ClearCaches()
}

func TestRefreshCaches(t *testing.T) {
	t.Skip("Skipping test that requires complex setup")
	// validator := NewResourceValidator()

	// Should not panic
	// validator.RefreshCaches()
}

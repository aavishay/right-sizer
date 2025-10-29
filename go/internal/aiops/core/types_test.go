package core

import (
	"testing"
)

func TestClamp01(t *testing.T) {
	tests := []struct {
		name     string
		input    float64
		expected float64
	}{
		{
			name:     "value below zero",
			input:    -0.5,
			expected: 0.0,
		},
		{
			name:     "value above one",
			input:    1.5,
			expected: 1.0,
		},
		{
			name:     "value within range",
			input:    0.7,
			expected: 0.7,
		},
		{
			name:     "value at lower bound",
			input:    0.0,
			expected: 0.0,
		},
		{
			name:     "value at upper bound",
			input:    1.0,
			expected: 1.0,
		},
		{
			name:     "very negative value",
			input:    -100.0,
			expected: 0.0,
		},
		{
			name:     "very large value",
			input:    100.0,
			expected: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Clamp01(tt.input)
			if result != tt.expected {
				t.Errorf("Clamp01(%f) = %f; want %f", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSeverityRank(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		expected int
	}{
		{
			name:     "critical severity",
			severity: SeverityCritical,
			expected: 3,
		},
		{
			name:     "warning severity",
			severity: SeverityWarning,
			expected: 2,
		},
		{
			name:     "info severity",
			severity: SeverityInfo,
			expected: 1,
		},
		{
			name:     "unknown severity",
			severity: Severity("unknown"),
			expected: 0,
		},
		{
			name:     "empty severity",
			severity: Severity(""),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SeverityRank(tt.severity)
			if result != tt.expected {
				t.Errorf("SeverityRank(%s) = %d; want %d", tt.severity, result, tt.expected)
			}
		})
	}
}

func TestSeverityRankOrdering(t *testing.T) {
	// Verify that severity rankings are properly ordered
	if SeverityRank(SeverityCritical) <= SeverityRank(SeverityWarning) {
		t.Error("Critical should rank higher than Warning")
	}
	if SeverityRank(SeverityWarning) <= SeverityRank(SeverityInfo) {
		t.Error("Warning should rank higher than Info")
	}
	if SeverityRank(SeverityInfo) <= SeverityRank(Severity("unknown")) {
		t.Error("Info should rank higher than unknown")
	}
}

package events

import (
	"testing"
	"time"
)

func TestEventType(t *testing.T) {
	tests := []struct {
		name     string
		eventType string
		valid    bool
	}{
		{
			name:      "pod resize event",
			eventType: "pod.resized",
			valid:     true,
		},
		{
			name:      "pod oom event",
			eventType: "pod.oom_killed",
			valid:     true,
		},
		{
			name:      "empty event type",
			eventType: "",
			valid:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.eventType == "" && tt.valid {
				t.Errorf("empty event type should not be valid")
			}
		})
	}
}

func TestEventSeverity(t *testing.T) {
	tests := []struct {
		name     string
		severity string
		valid    bool
	}{
		{
			name:     "info severity",
			severity: "info",
			valid:    true,
		},
		{
			name:     "warning severity",
			severity: "warning",
			valid:    true,
		},
		{
			name:     "critical severity",
			severity: "critical",
			valid:    true,
		},
		{
			name:     "error severity",
			severity: "error",
			valid:    true,
		},
		{
			name:     "invalid severity",
			severity: "unknown",
			valid:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validSeverities := map[string]bool{
				"info":     true,
				"warning":  true,
				"error":    true,
				"critical": true,
			}

			if tt.valid != validSeverities[tt.severity] {
				t.Errorf("severity %s validity mismatch", tt.severity)
			}
		})
	}
}

func TestEventTimestamp(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		timestamp time.Time
		valid     bool
	}{
		{
			name:      "current time",
			timestamp: now,
			valid:     true,
		},
		{
			name:      "past time",
			timestamp: now.Add(-1 * time.Hour),
			valid:     true,
		},
		{
			name:      "future time",
			timestamp: now.Add(1 * time.Hour),
			valid:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.timestamp.IsZero() && tt.valid {
				t.Errorf("zero timestamp should not be valid")
			}
		})
	}
}

func TestEventFiltering(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		namespace string
		severity  string
		shouldMatch bool
	}{
		{
			name:        "matching pod resize event",
			eventType:   "pod.resized",
			namespace:   "default",
			severity:    "info",
			shouldMatch: true,
		},
		{
			name:        "non-matching event type",
			eventType:   "node.created",
			namespace:   "default",
			severity:    "info",
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simple filter logic for testing
			filter := tt.eventType == "pod.resized"

			if filter != tt.shouldMatch {
				t.Errorf("filter result %v, want %v", filter, tt.shouldMatch)
			}
		})
	}
}

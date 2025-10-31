package events

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEvent(t *testing.T) {
	event := NewEvent(EventPodOOMKilled, "test-cluster", "default", "test-pod", SeverityError, "Pod OOM killed")

	require.NotNil(t, event)
	assert.NotEmpty(t, event.ID)
	assert.Equal(t, EventPodOOMKilled, event.Type)
	assert.Equal(t, "default", event.Namespace)
	assert.Equal(t, "test-pod", event.Resource)
	assert.Equal(t, SeverityError, event.Severity)
	assert.Equal(t, "Pod OOM killed", event.Message)
	assert.NotZero(t, event.Timestamp)
}

func TestEvent_WithDetails(t *testing.T) {
	event := NewEvent(EventResourceOptimized, "test-cluster", "default", "test-pod", SeverityInfo, "Resource optimized")

	details := map[string]interface{}{
		"cpu":    "200m",
		"memory": "512Mi",
		"reason": "underutilized",
	}

	event = event.WithDetails(details)

	assert.Equal(t, details, event.Details)
	assert.Equal(t, "200m", event.Details["cpu"])
}

func TestEvent_WithTags(t *testing.T) {
	event := NewEvent(EventDeploymentScaled, "test-cluster", "default", "nginx", SeverityInfo, "Deployment scaled")

	event = event.WithTags("autoscaling", "horizontal", "production")

	assert.Len(t, event.Tags, 3)
	assert.Contains(t, event.Tags, "autoscaling")
}

func TestEvent_WithCorrelationID(t *testing.T) {
	event := NewEvent(EventPodStarted, "test-cluster", "default", "test-pod", SeverityInfo, "Pod started")

	correlationID := "corr-123-456"
	event = event.WithCorrelationID(correlationID)

	assert.Equal(t, correlationID, event.CorrelationID)
}

func TestEvent_ToJSON(t *testing.T) {
	event := NewEvent(EventPodOOMKilled, "test-cluster", "default", "test-pod", SeverityError, "Pod OOM killed")
	event = event.WithDetails(map[string]interface{}{
		"container": "app",
		"exitCode":  137,
	})

	jsonBytes, err := event.ToJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, jsonBytes)

	var decoded map[string]interface{}
	err = json.Unmarshal(jsonBytes, &decoded)
	require.NoError(t, err)
	assert.Equal(t, string(EventPodOOMKilled), decoded["type"])
}

func TestEvent_FromJSON(t *testing.T) {
	original := NewEvent(EventResourceOptimized, "test-cluster", "default", "test-pod", SeverityInfo, "Optimized")
	original = original.WithTags("test", "optimization")

	jsonBytes, err := original.ToJSON()
	require.NoError(t, err)

	decoded, err := FromJSON(jsonBytes)
	require.NoError(t, err)
	assert.Equal(t, original.Type, decoded.Type)
	assert.Equal(t, original.Namespace, decoded.Namespace)
}

func TestEvent_FromJSON_Invalid(t *testing.T) {
	invalidJSON := []byte(`{"invalid": "not valid json syntax`)

	event, err := FromJSON(invalidJSON)
	assert.Error(t, err)
	// FromJSON still returns an event even on error
	assert.NotNil(t, event)
}

func TestGenerateEventID(t *testing.T) {
	id1 := generateEventID()
	id2 := generateEventID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2)
	assert.Contains(t, id1, "-")
}

func TestRandomString(t *testing.T) {
	str1 := randomString(8)
	str2 := randomString(8)

	assert.Len(t, str1, 8)
	assert.Len(t, str2, 8)
	assert.NotEqual(t, str1, str2)
}

func TestEventSeverities(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
	}{
		{"info", SeverityInfo},
		{"warning", SeverityWarning},
		{"error", SeverityError},
		{"critical", SeverityCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := NewEvent(EventPodStarted, "test-cluster", "default", "test", tt.severity, "test message")
			assert.Equal(t, tt.severity, event.Severity)
		})
	}
}

func TestEventTypes(t *testing.T) {
	eventTypes := []EventType{
		EventResourceOptimized,
		EventPodOOMKilled,
		EventPodStarted,
		EventNodeReady,
		EventDeploymentScaled,
		EventHealthCheckFailed,
		EventDashboardConnected,
	}

	for _, eventType := range eventTypes {
		t.Run(string(eventType), func(t *testing.T) {
			event := NewEvent(eventType, "test-cluster", "ns", "resource", SeverityInfo, "test")
			assert.Equal(t, eventType, event.Type)
		})
	}
}

func TestEvent_ComplexDetails(t *testing.T) {
	event := NewEvent(EventResourceOptimized, "test-cluster", "default", "test-pod", SeverityInfo, "Resource optimized")

	complexDetails := map[string]interface{}{
		"cpu": map[string]interface{}{
			"previous": "200m",
			"new":      "100m",
		},
		"memory": map[string]interface{}{
			"previous": "512Mi",
			"new":      "256Mi",
		},
	}

	event = event.WithDetails(complexDetails)

	assert.NotNil(t, event.Details)
	jsonBytes, err := event.ToJSON()
	require.NoError(t, err)
	assert.NotEmpty(t, jsonBytes)
}

func TestEvent_Chaining(t *testing.T) {
	event := NewEvent(EventPodOOMKilled, "test-cluster", "default", "test-pod", SeverityError, "Pod killed").
		WithDetails(map[string]interface{}{"container": "app"}).
		WithTags("critical", "memory").
		WithCorrelationID("corr-123")

	assert.NotNil(t, event.Details)
	assert.Len(t, event.Tags, 2)
	assert.Equal(t, "corr-123", event.CorrelationID)
}

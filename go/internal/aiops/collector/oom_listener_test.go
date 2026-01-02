package collector

import (
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestGetContainerNameFromMessage verifies container name extraction logic.
func TestGetContainerNameFromMessage(t *testing.T) {
	name := getContainerNameFromMessage("Container api was OOMKilled")
	if name != "api" {
		t.Fatalf("expected api, got %s", name)
	}
	unknown := getContainerNameFromMessage("something else")
	if unknown != "unknown" {
		t.Fatalf("expected unknown fallback, got %s", unknown)
	}
}

// TestHandleOOMEventNonBlocking simulates channel full behaviour.
func TestHandleOOMEventNonBlocking(t *testing.T) {
	ch := make(chan OOMEvent, 1)
	clientset := fake.NewSimpleClientset()
	listener := &OOMListener{
		oomChan:      ch,
		rcaCollector: NewRCACollector(clientset),
	}
	// Fill channel
	ch <- OOMEvent{PodName: "p1", Namespace: "ns", ContainerName: "c", Timestamp: time.Now()}
	// Construct event object
	ev := &v1.Event{}
	ev.InvolvedObject.Name = "p2"
	ev.InvolvedObject.Namespace = "ns"
	ev.Reason = oomKilledReason
	ev.Message = "Container web was OOMKilled"
	ev.LastTimestamp.Time = time.Now()
	// Should not block or panic when channel is full
	listener.handleOOMEvent(ev)
}

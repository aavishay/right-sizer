//go:build legacy_aiops_unused
// +build legacy_aiops_unused

//
// This legacy OOM listener implementation is disabled by default.
// It remains for historical reference and can be re-enabled by building
// with the 'legacy_aiops_unused' build tag.

package legacycollector

import (
	"context"
	"fmt"
	"time"

	"right-sizer/logger"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	// oomKilledReason is the event reason for an OOMKilled event.
	oomKilledReason = "OOMKilled"
	// resyncPeriod specifies how often the informer will re-list and process all objects.
	resyncPeriod = 10 * time.Minute
)

// OOMEvent holds structured information about an OOMKilled event.
type OOMEvent struct {
	PodName       string
	Namespace     string
	ContainerName string
	Timestamp     time.Time
}

// OOMListener watches for OOMKilled events in a Kubernetes cluster.
type OOMListener struct {
	clientset kubernetes.Interface
	oomChan   chan<- OOMEvent
}

// NewOOMListener creates a new listener for OOMKilled events.
// It takes a Kubernetes clientset and a channel to send events to.
func NewOOMListener(clientset kubernetes.Interface, oomChan chan<- OOMEvent) *OOMListener {
	return &OOMListener{
		clientset: clientset,
		oomChan:   oomChan,
	}
}

// Start begins the process of watching for Kubernetes events.
// It runs until the provided context is canceled.
func (l *OOMListener) Start(ctx context.Context) {
	// Create a ListWatch for events across all namespaces.
	eventListWatcher := cache.NewListWatchFromClient(
		l.clientset.CoreV1().RESTClient(),
		"events",
		v1.NamespaceAll,
		nil, // No field selector here; we filter in the handler.
	)

	// Create a new shared informer to watch for events.
	informer := cache.NewSharedInformer(eventListWatcher, &v1.Event{}, resyncPeriod)

	// Register an event handler for the informer.
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			event, ok := obj.(*v1.Event)
			if !ok {
				return
			}
			// Check if the event is an OOMKilled event.
			if event.Reason == oomKilledReason {
				logger.Info("Detected OOMKilled event for Pod: %s in Namespace: %s", event.InvolvedObject.Name, event.InvolvedObject.Namespace)
				l.handleOOMEvent(event)
			}
		},
		// UpdateFunc is included to catch events that might be updated.
		UpdateFunc: func(oldObj, newObj interface{}) {
			event, ok := newObj.(*v1.Event)
			if !ok {
				return
			}
			if event.Reason == oomKilledReason {
				logger.Info("Detected updated OOMKilled event for Pod: %s in Namespace: %s", event.InvolvedObject.Name, event.InvolvedObject.Namespace)
				l.handleOOMEvent(event)
			}
		},
	})

	logger.Info("Starting OOMKilled event listener...")
	// Run the informer until the context is canceled.
	informer.Run(ctx.Done())
	logger.Info("OOMKilled event listener stopped.")
}

// handleOOMEvent processes a raw Kubernetes event and sends a structured OOMEvent to the channel.
func (l *OOMListener) handleOOMEvent(event *v1.Event) {
	// The InvolvedObject for a pod event is the pod itself.
	// We need to get the container name from the event message, as it's not a first-class field.
	// A typical message is: "Container container-name was OOMKilled".
	// This is a simplification; a more robust solution would use regex.
	containerName := getContainerNameFromMessage(event.Message)

	oomEvent := OOMEvent{
		PodName:       event.InvolvedObject.Name,
		Namespace:     event.InvolvedObject.Namespace,
		ContainerName: containerName,
		Timestamp:     event.LastTimestamp.Time,
	}

	// Send the structured event to the processing channel.
	// This is a non-blocking send to prevent the informer from getting stuck.
	select {
	case l.oomChan <- oomEvent:
	default:
		logger.Warn("Warning: OOM event channel is full. Dropping event for pod %s.", oomEvent.PodName)
	}
}

// getContainerNameFromMessage is a helper function to extract the container name
// from the event message. This is a simple implementation and may need to be
// made more robust.
// Example message: "Container my-app-container was OOMKilled"
func getContainerNameFromMessage(message string) string {
	var containerName string
	// sscanf is a convenient way to parse a formatted string.
	// We are looking for the string between "Container " and " was OOMKilled".
	n, err := fmt.Sscanf(message, "Container %s was OOMKilled", &containerName)
	if err != nil || n != 1 {
		// Fallback for different message formats or if parsing fails.
		return "unknown"
	}
	return containerName
}

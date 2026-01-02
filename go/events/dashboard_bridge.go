// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package events

import (
	"context"
	"fmt"
	dashboardapi "right-sizer/dashboard-api"
	"right-sizer/logger"
)

// DashboardBridge forwards internal events to the dashboard API
type DashboardBridge struct {
	eventBus        *EventBus
	dashboardClient *dashboardapi.Client
	subscriberID    string
}

// NewDashboardBridge creates a new dashboard bridge
func NewDashboardBridge(eventBus *EventBus, dashboardClient *dashboardapi.Client) *DashboardBridge {
	return &DashboardBridge{
		eventBus:        eventBus,
		dashboardClient: dashboardClient,
	}
}

// Start starts listening for events and forwarding them
func (b *DashboardBridge) Start(ctx context.Context) error {
	if b.dashboardClient == nil {
		logger.Warn("Dashboard integration disabled, bridge will not start")
		return nil
	}

	b.subscriberID = "dashboard-bridge"
	b.eventBus.Subscribe(b.subscriberID, func(event *Event) {
		b.forwardEvent(event)
	})

	logger.Info("🌉 Dashboard bridge registered")
	return nil
}

// Stop stops the bridge
func (b *DashboardBridge) Stop() {
	if b.subscriberID != "" {
		b.eventBus.Unsubscribe(b.subscriberID)
	}
}

// forwardEvent maps and forwards an internal event to the dashboard API
func (b *DashboardBridge) forwardEvent(event *Event) {
	var dashType dashboardapi.EventType
	forward := false

	// Map internal event types to dashboard event types
	switch event.Type {
	case EventPodOOMKilled:
		dashType = dashboardapi.EventOOMKilled
		forward = true
	case EventPodCrashLoop:
		dashType = dashboardapi.EventCrashLoopBackOff
		forward = true
	case EventPodFailedScheduling:
		dashType = dashboardapi.EventFailedScheduling
		forward = true
	case EventPodCPUThrottled:
		dashType = dashboardapi.EventCPUThrottling
		forward = true
	case EventPodLivenessFailed:
		dashType = dashboardapi.EventLivenessFailed
		forward = true
	case EventPodReadinessFailed:
		dashType = dashboardapi.EventReadinessFailed
		forward = true
	case EventPodAffinityIssue:
		dashType = dashboardapi.EventAffinityIssue
		forward = true
	case EventPodImagePullIssue:
		dashType = dashboardapi.EventImagePullIssue
		forward = true
	case EventPodEvicted:
		dashType = dashboardapi.EventPodEvicted
		forward = true
	case EventPodRestarts:
		dashType = dashboardapi.EventPodRestarts
		forward = true
	case EventDiskPressure:
		dashType = dashboardapi.EventDiskPressure
		forward = true
	case EventResourceExhaustion:
		dashType = dashboardapi.EventPolicyViolation
		forward = true
	case EventRemediationFailed:
		dashType = dashboardapi.EventError
		forward = true
	}

	if !forward {
		return
	}

	// Create dashboard event
	dashEvent := dashboardapi.Event{
		Type:          dashType,
		Severity:      dashboardapi.EventSeverity(event.Severity),
		Timestamp:     event.Timestamp.UTC().Format("2006-01-02T15:04:05Z07:00"),
		Namespace:     event.Namespace,
		PodName:       event.Resource,
		Message:       event.Message,
		Metadata:      event.Details,
		ClusterID:     event.ClusterID,
		ContainerName: fmt.Sprintf("%v", event.Details["container"]),
	}

	// Send to dashboard
	if err := b.dashboardClient.SendEvent(dashEvent); err != nil {
		logger.Debug("Failed to forward event to dashboard: %v", err)
	}
}

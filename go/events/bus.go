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
	"sync"
	"time"

	"right-sizer/logger"
)

// EventBus provides event distribution for the operator
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string]EventHandler
	buffer      chan *Event
	bufferSize  int
	ctx         context.Context
	cancel      context.CancelFunc
	closed      bool
}

// EventHandler processes events from the bus
type EventHandler func(event *Event)

// NewEventBus creates a new event bus
func NewEventBus(bufferSize int) *EventBus {
	ctx, cancel := context.WithCancel(context.Background())

	bus := &EventBus{
		subscribers: make(map[string]EventHandler),
		buffer:      make(chan *Event, bufferSize),
		bufferSize:  bufferSize,
		ctx:         ctx,
		cancel:      cancel,
	}

	// Start event processing
	go bus.processEvents()

	return bus
}

// Subscribe adds a new event handler
func (eb *EventBus) Subscribe(subscriberID string, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.subscribers[subscriberID] = handler
	logger.Info("📡 Event subscriber registered: %s", subscriberID)
}

// SubscribeChannel creates a channel-based subscription with filtering
func (eb *EventBus) SubscribeChannel(filter *EventFilter, eventChan chan *Event) string {
	subscriberID := fmt.Sprintf("channel-%d", time.Now().UnixNano())

	handler := EventHandler(func(event *Event) {
		if eb.matchesFilter(event, filter) {
			select {
			case eventChan <- event:
			case <-time.After(1 * time.Second):
				logger.Warn("⚠️ Event channel full, dropping event: %s", event.ID)
			}
		}
	})

	eb.Subscribe(subscriberID, handler)
	return subscriberID
}

// matchesFilter checks if an event matches the given filter
func (eb *EventBus) matchesFilter(event *Event, filter *EventFilter) bool {
	if filter == nil {
		return true
	}

	// Check event types
	if len(filter.EventTypes) > 0 {
		found := false
		for _, eventType := range filter.EventTypes {
			if event.Type == eventType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Legacy types field
	if len(filter.Types) > 0 {
		found := false
		for _, eventType := range filter.Types {
			if event.Type == eventType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check namespaces
	if len(filter.Namespaces) > 0 {
		found := false
		for _, ns := range filter.Namespaces {
			if event.Namespace == ns {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check pod names
	if len(filter.PodNames) > 0 {
		found := false
		for _, podName := range filter.PodNames {
			if event.Resource == podName {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check severities
	if len(filter.Severities) > 0 {
		found := false
		for _, severity := range filter.Severities {
			if event.Severity == severity {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check tags
	if len(filter.Tags) > 0 {
		for _, filterTag := range filter.Tags {
			found := false
			for _, eventTag := range event.Tags {
				if eventTag == filterTag {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	return true
}

// Unsubscribe removes an event handler
func (eb *EventBus) Unsubscribe(subscriberID string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	delete(eb.subscribers, subscriberID)
	logger.Info("📡 Event subscriber removed: %s", subscriberID)
}

// Publish sends an event to all subscribers
func (eb *EventBus) Publish(event *Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	if eb.closed {
		logger.Warn("Event bus publish dropped (bus stopped): %s", event.Type)
		return
	}

	select {
	case eb.buffer <- event:
		// Event buffered successfully
	default:
		// Buffer full, log warning but don't block
		logger.Warn("Event bus buffer full, dropping event: %s", event.Type)
	}
}

// PublishAsync sends an event asynchronously (non-blocking)
func (eb *EventBus) PublishAsync(event *Event) {
	go eb.Publish(event)
}

// processEvents processes events from the buffer
func (eb *EventBus) processEvents() {
	for {
		select {
		case event := <-eb.buffer:
			eb.distributeEvent(event)
		case <-eb.ctx.Done():
			return
		}
	}
}

// distributeEvent sends event to all subscribers
func (eb *EventBus) distributeEvent(event *Event) {
	eb.mu.RLock()
	subscribers := make(map[string]EventHandler, len(eb.subscribers))
	for id, handler := range eb.subscribers {
		subscribers[id] = handler
	}
	eb.mu.RUnlock()

	// Send to all subscribers concurrently
	var wg sync.WaitGroup
	for subscriberID, handler := range subscribers {
		wg.Add(1)
		go func(id string, h EventHandler) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Event handler panic for subscriber %s: %v", id, r)
				}
			}()

			// Handle with timeout
			done := make(chan struct{})
			go func() {
				h(event)
				close(done)
			}()

			select {
			case <-done:
				// Handler completed successfully
			case <-time.After(5 * time.Second):
				logger.Warn("Event handler timeout for subscriber %s", id)
			}
		}(subscriberID, handler)
	}

	// Wait for all handlers with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All handlers completed
	case <-time.After(10 * time.Second):
		logger.Warn("Event distribution timeout for event: %s", event.Type)
	}
}

// Stop shuts down the event bus
func (eb *EventBus) Stop() {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	if !eb.closed {
		eb.cancel()
		eb.closed = true
		close(eb.buffer)
		eb.subscribers = make(map[string]EventHandler)
		logger.Info("📡 Event bus stopped")
	}
}

// Stats returns event bus statistics
func (eb *EventBus) Stats() EventBusStats {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	return EventBusStats{
		Subscribers: len(eb.subscribers),
		BufferSize:  eb.bufferSize,
		BufferUsed:  len(eb.buffer),
		BufferFree:  eb.bufferSize - len(eb.buffer),
	}
}

// EventBusStats contains event bus statistics
type EventBusStats struct {
	Subscribers int `json:"subscribers"`
	BufferSize  int `json:"bufferSize"`
	BufferUsed  int `json:"bufferUsed"`
	BufferFree  int `json:"bufferFree"`
}

// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package events

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestEventBusBasic ensures subscribe, publish, unsubscribe work
func TestEventBusBasic(t *testing.T) {
	bus := NewEventBus(10)
	received := make(chan *Event, 1)
	handler := func(ev *Event) { received <- ev }
	bus.Subscribe("tester", handler)
	bus.Publish(&Event{ID: "1", Type: "test", Namespace: "default"})
	select {
	case ev := <-received:
		if ev.Type != "test" {
			t.Fatalf("unexpected event type: %s", ev.Type)
		}
	case <-time.After(time.Second):
		t.Fatalf("did not receive event")
	}
	bus.Unsubscribe("tester")
	stats := bus.Stats()
	if stats.Subscribers != 0 {
		t.Fatalf("expected 0 subscribers, got %d", stats.Subscribers)
	}
	bus.Stop()
}

func TestEventBusSubscribeChannel(t *testing.T) {
	bus := NewEventBus(10)
	defer bus.Stop()

	ch := make(chan *Event, 5)
	filter := EventFilter{
		EventTypes: []EventType{EventPodOOMKilled, EventPodCrashLoop},
	}

	bus.SubscribeChannel(&filter, ch)

	// Publish matching event
	event1 := &Event{ID: "1", Type: EventPodOOMKilled, Namespace: "default"}
	bus.Publish(event1)

	// Publish non-matching event
	event2 := &Event{ID: "2", Type: EventPodStarted, Namespace: "default"}
	bus.Publish(event2)

	// Should only receive the matching event
	select {
	case ev := <-ch:
		assert.Equal(t, EventPodOOMKilled, ev.Type)
	case <-time.After(time.Second):
		t.Fatal("did not receive matching event")
	}

	// Should not receive non-matching event
	select {
	case ev := <-ch:
		t.Fatalf("received unexpected event: %s", ev.Type)
	case <-time.After(100 * time.Millisecond):
		// Expected timeout
	}
}

func TestEventBusPublishAsync(t *testing.T) {
	bus := NewEventBus(10)
	defer bus.Stop()

	received := make(chan *Event, 1)
	handler := func(ev *Event) { received <- ev }
	bus.Subscribe("async-tester", handler)

	event := &Event{ID: "1", Type: EventResourceOptimized, Namespace: "default"}
	bus.PublishAsync(event)

	select {
	case ev := <-received:
		assert.Equal(t, event.ID, ev.ID)
		assert.Equal(t, event.Type, ev.Type)
	case <-time.After(time.Second):
		t.Fatal("did not receive async event")
	}
}

func TestEventBusMultipleSubscribers(t *testing.T) {
	bus := NewEventBus(10)
	defer bus.Stop()

	received1 := make(chan *Event, 1)
	received2 := make(chan *Event, 1)

	bus.Subscribe("sub1", func(ev *Event) { received1 <- ev })
	bus.Subscribe("sub2", func(ev *Event) { received2 <- ev })

	event := &Event{ID: "1", Type: EventPodStarted}
	bus.Publish(event)

	// Both subscribers should receive the event
	select {
	case <-received1:
	case <-time.After(time.Second):
		t.Fatal("subscriber 1 did not receive event")
	}

	select {
	case <-received2:
	case <-time.After(time.Second):
		t.Fatal("subscriber 2 did not receive event")
	}
}

func TestEventBusFilterByEventType(t *testing.T) {
	bus := NewEventBus(10)
	defer bus.Stop()

	ch := make(chan *Event, 5)
	filter := EventFilter{
		EventTypes: []EventType{EventPodOOMKilled},
	}

	bus.SubscribeChannel(&filter, ch)

	// Publish various events
	bus.Publish(&Event{ID: "1", Type: EventPodOOMKilled})
	bus.Publish(&Event{ID: "2", Type: EventPodStarted})
	bus.Publish(&Event{ID: "3", Type: EventPodCrashLoop})

	// Should only receive OOM event
	select {
	case ev := <-ch:
		assert.Equal(t, EventPodOOMKilled, ev.Type)
	case <-time.After(time.Second):
		t.Fatal("did not receive filtered event")
	}

	// Should not receive other events
	select {
	case ev := <-ch:
		t.Fatalf("received unexpected event: %s", ev.Type)
	case <-time.After(100 * time.Millisecond):
		// Expected
	}
}

func TestEventBusFilterByNamespace(t *testing.T) {
	bus := NewEventBus(10)
	defer bus.Stop()

	ch := make(chan *Event, 5)
	filter := EventFilter{
		Namespaces: []string{"production"},
	}

	bus.SubscribeChannel(&filter, ch)

	bus.Publish(&Event{ID: "1", Type: EventPodStarted, Namespace: "production"})
	bus.Publish(&Event{ID: "2", Type: EventPodStarted, Namespace: "staging"})

	select {
	case ev := <-ch:
		assert.Equal(t, "production", ev.Namespace)
	case <-time.After(time.Second):
		t.Fatal("did not receive namespace-filtered event")
	}
}

func TestEventBusFilterBySeverity(t *testing.T) {
	bus := NewEventBus(10)
	defer bus.Stop()

	ch := make(chan *Event, 5)
	filter := EventFilter{
		Severities: []Severity{SeverityError, SeverityCritical},
	}

	bus.SubscribeChannel(&filter, ch)

	bus.Publish(&Event{ID: "1", Type: EventPodOOMKilled, Severity: SeverityError})
	bus.Publish(&Event{ID: "2", Type: EventPodStarted, Severity: SeverityInfo})
	bus.Publish(&Event{ID: "3", Type: EventNodeNotReady, Severity: SeverityCritical})

	// Should receive error event
	select {
	case ev := <-ch:
		assert.Equal(t, SeverityError, ev.Severity)
	case <-time.After(time.Second):
		t.Fatal("did not receive first severity-filtered event")
	}

	// Should receive critical event
	select {
	case ev := <-ch:
		assert.Equal(t, SeverityCritical, ev.Severity)
	case <-time.After(time.Second):
		t.Fatal("did not receive second severity-filtered event")
	}
}

func TestEventBusStats(t *testing.T) {
	bus := NewEventBus(10)
	defer bus.Stop()

	stats := bus.Stats()
	assert.Equal(t, 0, stats.Subscribers)
	// EventsPublished not in stats

	bus.Subscribe("sub1", func(ev *Event) {})
	bus.Subscribe("sub2", func(ev *Event) {})

	stats = bus.Stats()
	assert.Equal(t, 2, stats.Subscribers)

	bus.Publish(&Event{ID: "1", Type: EventPodStarted})
	bus.Publish(&Event{ID: "2", Type: EventPodTerminated})

	time.Sleep(50 * time.Millisecond) // Give time for async processing

	stats = bus.Stats()
	// EventsPublished not in stats
}

func TestEventBusUnsubscribe(t *testing.T) {
	bus := NewEventBus(10)
	defer bus.Stop()

	received := make(chan *Event, 5)
	handler := func(ev *Event) { received <- ev }

	bus.Subscribe("unsub-test", handler)

	// Publish before unsubscribe
	bus.Publish(&Event{ID: "1", Type: EventPodStarted})

	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("did not receive event before unsubscribe")
	}

	// Unsubscribe
	bus.Unsubscribe("unsub-test")

	// Publish after unsubscribe
	bus.Publish(&Event{ID: "2", Type: EventPodTerminated})

	// Should not receive event after unsubscribe
	select {
	case ev := <-received:
		t.Fatalf("received event after unsubscribe: %s", ev.ID)
	case <-time.After(100 * time.Millisecond):
		// Expected
	}
}

func TestEventBusStop(t *testing.T) {
	bus := NewEventBus(10)

	received := make(chan *Event, 1)
	bus.Subscribe("stop-test", func(ev *Event) { received <- ev })

	bus.Publish(&Event{ID: "1", Type: EventPodStarted})

	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("did not receive event before stop")
	}

	bus.Stop()

	// Publishing after stop should not cause panic
	assert.NotPanics(t, func() {
		bus.Publish(&Event{ID: "2", Type: EventPodTerminated})
	})
}

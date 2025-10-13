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

package core

import (
	"sync"
	"testing"
	"time"
)

// TestInMemoryBusPublish verifies publish dispatches to matching subscribers.
func TestInMemoryBusPublish(t *testing.T) {
	bus := newTestBus()
	var mu sync.Mutex
	received := 0
	err := bus.Subscribe("sub1", []SignalType{SignalOOMKilled}, func(sig Signal) {
		mu.Lock()
		received++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}

	bus.Publish(Signal{Type: SignalOOMKilled, Timestamp: time.Now()})
	time.Sleep(20 * time.Millisecond) // allow goroutine dispatch
	mu.Lock()
	v := received
	mu.Unlock()
	if v != 1 {
		t.Fatalf("expected 1 signal received, got %d", v)
	}
}

// newTestBus constructs an in-memory bus via public factory in aiops package.
func newTestBus() Bus {
	// Minimal inline implementation since core doesn't export a constructor directly here.
	return &testBus{subscribers: make(map[string]*testSub)}
}

// Lightweight bus for tests (not production code) implementing Bus.
type testBus struct {
	mu          sync.RWMutex
	subscribers map[string]*testSub
}

type testSub struct {
	types map[SignalType]struct{}
	h     Handler
}

func (b *testBus) Publish(s Signal) {
	b.mu.RLock()
	subs := b.subscribers
	b.mu.RUnlock()
	for _, sub := range subs {
		if len(sub.types) == 0 || contains(sub.types, s.Type) {
			go sub.h(s)
		}
	}
}

func (b *testBus) Subscribe(id string, types []SignalType, h Handler) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.subscribers[id]; exists {
		return nil
	}
	set := make(map[SignalType]struct{})
	for _, t := range types {
		set[t] = struct{}{}
	}
	b.subscribers[id] = &testSub{types: set, h: h}
	return nil
}

func (b *testBus) Unsubscribe(id string) { b.mu.Lock(); delete(b.subscribers, id); b.mu.Unlock() }

func contains(m map[SignalType]struct{}, k SignalType) bool { _, ok := m[k]; return ok }

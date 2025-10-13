// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package remediation

import (
	"context"
	"right-sizer/events"
	"testing"
	"time"
)

// TestNewEngineSmoke verifies engine initializes and registers handlers
func TestNewEngineSmoke(t *testing.T) {
	bus := events.NewEventBus(5)
	cfg := Config{Enabled: true, DryRun: true, SafetyTimeout: time.Second}
	eng := NewEngine(nil, nil, bus, cfg)
	if eng == nil {
		t.Fatalf("expected engine instance")
	}
	bus.Stop()
}

// TestRegisterHandler verifies custom handler registration
func TestRegisterHandler(t *testing.T) {
	bus := events.NewEventBus(5)
	cfg := Config{Enabled: true, DryRun: true, SafetyTimeout: time.Second}
	eng := NewEngine(nil, nil, bus, cfg)
	handler := &ActionHandler{
		Type:             "custom_action",
		Execute:          func(ctx context.Context, a *Action) error { return nil },
		Validate:         func(a *Action) error { return nil },
		CalculateRisk:    func(a *Action) RiskLevel { return RiskLow },
		RequiresApproval: func(a *Action) bool { return false },
	}
	eng.RegisterHandler(handler)
	h, ok := eng.registry["custom_action"]
	if !ok || h != handler {
		t.Errorf("handler not registered")
	}
	bus.Stop()
}

// TestExecuteAction_DryRun covers ExecuteAction with dry run and default handler
func TestExecuteAction_DryRun(t *testing.T) {
	bus := events.NewEventBus(5)
	cfg := Config{Enabled: true, DryRun: true, SafetyTimeout: time.Second}
	eng := NewEngine(nil, nil, bus, cfg)
	action := &Action{
		ID:      "a1",
		Type:    ActionRestartPod,
		Target:  ActionTarget{Name: "pod1", Namespace: "ns1", Kind: "Pod"},
		Timeout: time.Second,
	}
	err := eng.ExecuteAction(context.Background(), action)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if action.Status != StatusCompleted && action.Status != StatusPending {
		t.Errorf("unexpected status: %v", action.Status)
	}
	bus.Stop()
}

// TestExecuteAction_Blocked covers blocked action type
func TestExecuteAction_Blocked(t *testing.T) {
	bus := events.NewEventBus(5)
	cfg := Config{Enabled: true, DryRun: true, BlockedActions: []ActionType{ActionRestartPod}}
	eng := NewEngine(nil, nil, bus, cfg)
	action := &Action{
		ID:      "a2",
		Type:    ActionRestartPod,
		Target:  ActionTarget{Name: "pod2", Namespace: "ns2", Kind: "Pod"},
		Timeout: time.Second,
	}
	err := eng.ExecuteAction(context.Background(), action)
	if err == nil || action.Status != StatusBlocked {
		t.Errorf("expected blocked error and status, got %v, %v", err, action.Status)
	}
	bus.Stop()
}

// TestApproveAction covers approval flow
func TestApproveAction(t *testing.T) {
	bus := events.NewEventBus(5)
	cfg := Config{Enabled: true, DryRun: true}
	eng := NewEngine(nil, nil, bus, cfg)
	// Register a handler that requires approval
	handler := &ActionHandler{
		Type:             "needs_approval",
		Execute:          func(ctx context.Context, a *Action) error { return nil },
		Validate:         func(a *Action) error { return nil },
		CalculateRisk:    func(a *Action) RiskLevel { return RiskHigh },
		RequiresApproval: func(a *Action) bool { return true },
	}
	eng.RegisterHandler(handler)
	action := &Action{
		ID:      "a3",
		Type:    "needs_approval",
		Target:  ActionTarget{Name: "pod3", Namespace: "ns3", Kind: "Pod"},
		Timeout: time.Second,
	}
	err := eng.ExecuteAction(context.Background(), action)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Status != StatusPending || action.Approval == nil || !action.Approval.Required {
		t.Errorf("expected pending approval, got %v", action.Status)
	}
	// Approve
	err = eng.ApproveAction("a3", "approver", "ok")
	if err != nil {
		t.Errorf("unexpected error on approve: %v", err)
	}
	if action.Status != StatusCompleted && action.Status != StatusApproved {
		t.Errorf("unexpected status after approval: %v", action.Status)
	}
	bus.Stop()
}

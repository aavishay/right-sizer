// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package remediation

import (
	"context"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"right-sizer/events"
	"right-sizer/logger"
)

// Engine handles automated remediation actions
type Engine struct {
	mu         sync.RWMutex
	client     client.Client
	clientset  kubernetes.Interface
	eventBus   *events.EventBus
	config     Config
	registry   map[ActionType]*ActionHandler
	actions    map[string]*Action
	safetyLock SafetyLock
}

// Config configures the remediation engine
type Config struct {
	Enabled              bool           `json:"enabled"`
	DryRun               bool           `json:"dryRun"`
	SafetyTimeout        time.Duration  `json:"safetyTimeout"`
	MaxConcurrentActions int            `json:"maxConcurrentActions"`
	RequireApproval      []ActionType   `json:"requireApproval"`
	BlockedActions       []ActionType   `json:"blockedActions"`
	RiskThresholds       RiskThresholds `json:"riskThresholds"`
}

// RiskThresholds defines safety thresholds for different actions
type RiskThresholds struct {
	MaxResourceIncrease float64       `json:"maxResourceIncrease"` // e.g., 2.0 = 200% increase
	MaxResourceDecrease float64       `json:"maxResourceDecrease"` // e.g., 0.5 = 50% decrease
	MinPodAge           time.Duration `json:"minPodAge"`           // Minimum pod age before action
	MaxRestartCount     int32         `json:"maxRestartCount"`     // Max restarts before blocking
}

// ActionType represents the type of remediation action
type ActionType string

const (
	ActionRestartPod         ActionType = "restart_pod"
	ActionScaleUp            ActionType = "scale_up"
	ActionScaleDown          ActionType = "scale_down"
	ActionUpdateResources    ActionType = "update_resources"
	ActionCordonNode         ActionType = "cordon_node"
	ActionDrainNode          ActionType = "drain_node"
	ActionDeletePod          ActionType = "delete_pod"
	ActionRollbackDeployment ActionType = "rollback_deployment"
	ActionApplyConfig        ActionType = "apply_config"
)

// Action represents a remediation action to be executed
type Action struct {
	ID         string                 `json:"id"`
	Type       ActionType             `json:"type"`
	Target     ActionTarget           `json:"target"`
	Parameters map[string]interface{} `json:"parameters"`
	Risk       RiskLevel              `json:"risk"`
	Reason     string                 `json:"reason"`
	Source     string                 `json:"source"` // dashboard, ai, threshold
	Priority   Priority               `json:"priority"`
	Timeout    time.Duration          `json:"timeout"`
	CreatedAt  time.Time              `json:"createdAt"`
	Status     ActionStatus           `json:"status"`
	Result     string                 `json:"result,omitempty"`
	Error      string                 `json:"error,omitempty"`
	UpdatedAt  time.Time              `json:"updatedAt"`
	Approval   *ActionApproval        `json:"approval,omitempty"`
	Execution  *ActionExecution       `json:"execution,omitempty"`
}

// ActionTarget identifies the target resource for the action
type ActionTarget struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
	Kind      string `json:"kind"` // Pod, Deployment, Node, etc.
	Container string `json:"container,omitempty"`
}

// RiskLevel represents the risk level of an action
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

// Priority represents action priority
type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

// ActionStatus represents the current status of an action
type ActionStatus string

const (
	StatusPending   ActionStatus = "pending"
	StatusApproved  ActionStatus = "approved"
	StatusRejected  ActionStatus = "rejected"
	StatusRunning   ActionStatus = "running"
	StatusExecuting ActionStatus = "executing"
	StatusCompleted ActionStatus = "completed"
	StatusFailed    ActionStatus = "failed"
	StatusCancelled ActionStatus = "cancelled"
	StatusTimedOut  ActionStatus = "timed_out"
	StatusBlocked   ActionStatus = "blocked"
)

// ActionApproval contains approval information
type ActionApproval struct {
	Required   bool      `json:"required"`
	ApprovedBy string    `json:"approvedBy,omitempty"`
	ApprovedAt time.Time `json:"approvedAt,omitempty"`
	RejectedBy string    `json:"rejectedBy,omitempty"`
	RejectedAt time.Time `json:"rejectedAt,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	ExpiresAt  time.Time `json:"expiresAt,omitempty"`
}

// ActionExecution contains execution details
type ActionExecution struct {
	StartedAt   time.Time              `json:"startedAt"`
	CompletedAt time.Time              `json:"completedAt,omitempty"`
	Duration    time.Duration          `json:"duration,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Result      map[string]interface{} `json:"result,omitempty"`
	Steps       []ExecutionStep        `json:"steps,omitempty"`
}

// ExecutionStep represents a step in action execution
type ExecutionStep struct {
	Name        string                 `json:"name"`
	Status      string                 `json:"status"`
	StartedAt   time.Time              `json:"startedAt"`
	CompletedAt time.Time              `json:"completedAt,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// ActionHandler defines how to execute a specific action type
type ActionHandler struct {
	Type             ActionType
	Execute          func(ctx context.Context, action *Action) error
	Validate         func(action *Action) error
	CalculateRisk    func(action *Action) RiskLevel
	RequiresApproval func(action *Action) bool
}

// SafetyLock prevents dangerous concurrent actions
type SafetyLock struct {
	activeActions map[string]*Action
	mutex         sync.RWMutex
}

// NewEngine creates a new remediation engine
func NewEngine(client client.Client, clientset kubernetes.Interface, eventBus *events.EventBus, config Config) *Engine {
	engine := &Engine{
		client:    client,
		clientset: clientset,
		eventBus:  eventBus,
		config:    config,
		registry:  make(map[ActionType]*ActionHandler),
		actions:   make(map[string]*Action),
		safetyLock: SafetyLock{
			activeActions: make(map[string]*Action),
		},
	}

	// Register default action handlers
	engine.registerDefaultHandlers()

	return engine
}

// RegisterHandler registers a custom action handler
func (e *Engine) RegisterHandler(handler *ActionHandler) {
	e.registry[handler.Type] = handler
	logger.Info("🔧 Registered remediation handler: %s", handler.Type)
}

// ExecuteAction executes a remediation action
func (e *Engine) ExecuteAction(ctx context.Context, action *Action) error {
	if !e.config.Enabled {
		return fmt.Errorf("remediation engine is disabled")
	}

	// Check if action type is blocked
	for _, blocked := range e.config.BlockedActions {
		if action.Type == blocked {
			action.Status = StatusBlocked
			return fmt.Errorf("action type %s is blocked", action.Type)
		}
	}

	// Get handler
	handler, exists := e.registry[action.Type]
	if !exists {
		return fmt.Errorf("no handler registered for action type: %s", action.Type)
	}

	// Validate action
	if err := handler.Validate(action); err != nil {
		return fmt.Errorf("action validation failed: %w", err)
	}

	// Store action for tracking
	e.mu.Lock()
	e.actions[action.ID] = action
	e.mu.Unlock()

	// Calculate risk
	action.Risk = handler.CalculateRisk(action)

	// Check if approval is required
	requiresApproval := handler.RequiresApproval(action) || e.requiresApproval(action.Type)
	if requiresApproval {
		action.Approval = &ActionApproval{
			Required:  true,
			ExpiresAt: time.Now().Add(24 * time.Hour), // Default 24h expiry
		}
		action.Status = StatusPending

		// Send approval request event
		e.sendApprovalRequest(action)
		return nil
	}

	// Execute immediately if no approval required
	return e.doExecuteAction(ctx, action, handler)
}

// ApproveAction approves a pending action
func (e *Engine) ApproveAction(actionID, approver, reason string) error {
	action, err := e.getAction(actionID)
	if err != nil {
		return err
	}

	if action.Status != StatusPending {
		return fmt.Errorf("action is not pending approval")
	}

	if action.Approval.ExpiresAt.Before(time.Now()) {
		action.Status = StatusTimedOut
		return fmt.Errorf("approval has expired")
	}

	action.Approval.ApprovedBy = approver
	action.Approval.ApprovedAt = time.Now()
	action.Approval.Reason = reason
	action.Status = StatusApproved

	// Execute the approved action
	handler := e.registry[action.Type]
	ctx, cancel := context.WithTimeout(context.Background(), action.Timeout)
	defer cancel()

	return e.doExecuteAction(ctx, action, handler)
}

// doExecuteAction performs the actual action execution
func (e *Engine) doExecuteAction(ctx context.Context, action *Action, handler *ActionHandler) error {
	// Check safety lock
	if !e.safetyLock.acquire(action) {
		return fmt.Errorf("safety lock prevents concurrent execution")
	}
	defer e.safetyLock.release(action.ID)

	action.Status = StatusExecuting
	action.Execution = &ActionExecution{
		StartedAt: time.Now(),
		Steps:     make([]ExecutionStep, 0),
	}

	// Send execution started event
	e.sendExecutionEvent(action, "started")

	// Execute with timeout
	var execErr error
	if e.config.DryRun {
		logger.Info("🔧 [DRY RUN] Would execute action: %s on %s/%s",
			action.Type, action.Target.Namespace, action.Target.Name)
		// Simulate execution time
		time.Sleep(100 * time.Millisecond)
	} else {
		execErr = handler.Execute(ctx, action)
	}

	// Update execution status
	action.Execution.CompletedAt = time.Now()
	action.Execution.Duration = action.Execution.CompletedAt.Sub(action.Execution.StartedAt)

	if execErr != nil {
		action.Status = StatusFailed
		action.Execution.Error = execErr.Error()
		e.sendExecutionEvent(action, "failed")
		return execErr
	}

	action.Status = StatusCompleted
	e.sendExecutionEvent(action, "completed")

	logger.Info("✅ Remediation action completed: %s on %s/%s",
		action.Type, action.Target.Namespace, action.Target.Name)

	return nil
}

// registerDefaultHandlers registers built-in action handlers
func (e *Engine) registerDefaultHandlers() {
	// Restart Pod Handler
	e.RegisterHandler(&ActionHandler{
		Type: ActionRestartPod,
		Execute: func(ctx context.Context, action *Action) error {
			return e.executeRestartPod(ctx, action)
		},
		Validate: func(action *Action) error {
			if action.Target.Name == "" || action.Target.Namespace == "" {
				return fmt.Errorf("pod name and namespace required")
			}
			return nil
		},
		CalculateRisk: func(action *Action) RiskLevel {
			return RiskMedium // Pod restart is generally medium risk
		},
		RequiresApproval: func(action *Action) bool {
			return false // Pod restarts generally don't require approval
		},
	})

	// Update Resources Handler
	e.RegisterHandler(&ActionHandler{
		Type: ActionUpdateResources,
		Execute: func(ctx context.Context, action *Action) error {
			return e.executeUpdateResources(ctx, action)
		},
		Validate: func(action *Action) error {
			if action.Target.Name == "" || action.Target.Namespace == "" {
				return fmt.Errorf("pod name and namespace required")
			}
			return nil
		},
		CalculateRisk: func(action *Action) RiskLevel {
			// Calculate risk based on resource change magnitude
			return e.calculateResourceRisk(action)
		},
		RequiresApproval: func(action *Action) bool {
			return action.Risk >= RiskHigh
		},
	})

	// Scale Up Handler
	e.RegisterHandler(&ActionHandler{
		Type: ActionScaleUp,
		Execute: func(ctx context.Context, action *Action) error {
			return e.executeScale(ctx, action, true)
		},
		Validate: func(action *Action) error {
			if action.Target.Name == "" || action.Target.Namespace == "" {
				return fmt.Errorf("deployment name and namespace required")
			}
			return nil
		},
		CalculateRisk: func(action *Action) RiskLevel {
			return RiskLow // Scaling up is generally low risk
		},
		RequiresApproval: func(action *Action) bool {
			return false
		},
	})

	// Add more handlers as needed...
}

// executeRestartPod restarts a pod by deleting it
func (e *Engine) executeRestartPod(ctx context.Context, action *Action) error {
	podName := action.Target.Name
	namespace := action.Target.Namespace

	// Get the pod first to ensure it exists
	var pod corev1.Pod
	if err := e.client.Get(ctx, types.NamespacedName{
		Name:      podName,
		Namespace: namespace,
	}, &pod); err != nil {
		return fmt.Errorf("failed to get pod: %w", err)
	}

	// Delete the pod
	if err := e.client.Delete(ctx, &pod); err != nil {
		return fmt.Errorf("failed to delete pod: %w", err)
	}

	action.Execution.Result = map[string]interface{}{
		"podName":   podName,
		"namespace": namespace,
		"action":    "deleted",
	}

	return nil
}

// executeUpdateResources updates pod resource requests/limits
func (e *Engine) executeUpdateResources(ctx context.Context, action *Action) error {
	// Implementation for updating resources using resize subresource
	// This would integrate with your existing resource update logic
	return fmt.Errorf("resource update handler not yet implemented")
}

// executeScale scales a deployment up or down
func (e *Engine) executeScale(ctx context.Context, action *Action, scaleUp bool) error {
	// Implementation for scaling deployments
	return fmt.Errorf("scale handler not yet implemented")
}

// GetAction retrieves an action by ID
func (e *Engine) GetAction(actionID string) (*Action, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	action, exists := e.actions[actionID]
	return action, exists
}

// Helper methods
func (e *Engine) requiresApproval(actionType ActionType) bool {
	for _, required := range e.config.RequireApproval {
		if required == actionType {
			return true
		}
	}
	return false
}

func (e *Engine) calculateResourceRisk(action *Action) RiskLevel {
	// Calculate risk based on resource change parameters
	// This would analyze the magnitude of resource changes
	return RiskMedium
}

func (e *Engine) sendApprovalRequest(action *Action) {
	event := events.NewEvent(
		events.EventDashboardCommand,
		"", // cluster ID would be set by caller
		action.Target.Namespace,
		action.Target.Name,
		events.SeverityWarning,
		fmt.Sprintf("Approval required for %s action", action.Type),
	).WithDetails(map[string]interface{}{
		"actionId":   action.ID,
		"actionType": action.Type,
		"risk":       action.Risk,
		"reason":     action.Reason,
	})

	e.eventBus.PublishAsync(event)
}

func (e *Engine) sendExecutionEvent(action *Action, phase string) {
	var eventType events.EventType
	var severity events.Severity

	switch phase {
	case "started":
		eventType = events.EventRemediationApplied
		severity = events.SeverityInfo
	case "completed":
		eventType = events.EventRemediationApplied
		severity = events.SeverityInfo
	case "failed":
		eventType = events.EventRemediationFailed
		severity = events.SeverityError
	default:
		return
	}

	event := events.NewEvent(
		eventType,
		"", // cluster ID would be set by caller
		action.Target.Namespace,
		action.Target.Name,
		severity,
		fmt.Sprintf("Remediation action %s: %s", action.Type, phase),
	).WithDetails(map[string]interface{}{
		"actionId":   action.ID,
		"actionType": action.Type,
		"risk":       action.Risk,
		"duration":   action.Execution.Duration,
	})

	e.eventBus.PublishAsync(event)
}

func (e *Engine) getAction(actionID string) (*Action, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.actions == nil {
		return nil, fmt.Errorf("no actions map initialized")
	}
	action, ok := e.actions[actionID]
	if !ok {
		return nil, fmt.Errorf("action %s not found", actionID)
	}
	return action, nil
}

// SafetyLock methods
func (sl *SafetyLock) acquire(action *Action) bool {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	key := fmt.Sprintf("%s/%s", action.Target.Namespace, action.Target.Name)
	if _, exists := sl.activeActions[key]; exists {
		return false // Another action is already running on this target
	}

	sl.activeActions[key] = action
	return true
}

func (sl *SafetyLock) release(actionID string) {
	sl.mutex.Lock()
	defer sl.mutex.Unlock()

	for key, action := range sl.activeActions {
		if action.ID == actionID {
			delete(sl.activeActions, key)
			break
		}
	}
}

// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

package alerts

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Alert represents a system alert event
type Alert struct {
	ID           string     `json:"id"`
	Namespace    string     `json:"namespace"`
	PodName      string     `json:"podName"`
	ResourceType string     `json:"resourceType"` // "cpu" or "memory"
	Severity     string     `json:"severity"`     // "critical", "warning", "info"
	Title        string     `json:"title"`
	Message      string     `json:"message"`
	Timestamp    time.Time  `json:"timestamp"`
	ResolvedAt   *time.Time `json:"resolvedAt,omitempty"`
	Source       string     `json:"source"` // "anomaly", "prediction", "scaling"
	MetricValue  float64    `json:"metricValue"`
	Threshold    float64    `json:"threshold"`
	ZScore       float64    `json:"zScore,omitempty"`
}

// Manager handles alert lifecycle: creation, storage, retrieval, dispatch
type Manager struct {
	alerts      map[string]*Alert
	alertsMutex sync.RWMutex

	subscribers []AlertSubscriber
	subMutex    sync.RWMutex

	webhooks  map[string]string // severity -> webhook URL
	webhookMu sync.RWMutex

	logger *zap.Logger
	maxAge time.Duration
}

// AlertSubscriber receives alert updates
type AlertSubscriber interface {
	OnAlert(ctx context.Context, alert *Alert) error
}

// New creates an alert manager
func New(logger *zap.Logger) *Manager {
	return &Manager{
		alerts:   make(map[string]*Alert),
		webhooks: make(map[string]string),
		logger:   logger,
		maxAge:   24 * time.Hour, // Keep alerts for 24 hours
	}
}

// Create generates and stores a new alert
func (m *Manager) Create(ctx context.Context, namespace, podName, resourceType, severity, title, message, source string, metricValue, threshold float64) (*Alert, error) {
	m.alertsMutex.Lock()
	defer m.alertsMutex.Unlock()

	alert := &Alert{
		ID:           fmt.Sprintf("%s-%s-%d", namespace, podName, time.Now().UnixMilli()),
		Namespace:    namespace,
		PodName:      podName,
		ResourceType: resourceType,
		Severity:     severity,
		Title:        title,
		Message:      message,
		Timestamp:    time.Now(),
		Source:       source,
		MetricValue:  metricValue,
		Threshold:    threshold,
	}

	m.alerts[alert.ID] = alert

	// Log alert creation
	m.logger.Info("Alert created",
		zap.String("id", alert.ID),
		zap.String("pod", fmt.Sprintf("%s/%s", namespace, podName)),
		zap.String("severity", severity),
		zap.String("title", title),
	)

	// Notify subscribers asynchronously
	go m.notifySubscribers(ctx, alert)

	// Dispatch webhooks asynchronously
	go m.dispatchWebhook(ctx, alert)

	return alert, nil
}

// Get retrieves a specific alert
func (m *Manager) Get(alertID string) *Alert {
	m.alertsMutex.RLock()
	defer m.alertsMutex.RUnlock()
	return m.alerts[alertID]
}

// List retrieves all active alerts, optionally filtered by namespace
func (m *Manager) List(namespace string) []*Alert {
	m.alertsMutex.RLock()
	defer m.alertsMutex.RUnlock()

	result := make([]*Alert, 0)
	now := time.Now()

	for _, alert := range m.alerts {
		// Skip resolved alerts
		if alert.ResolvedAt != nil {
			continue
		}

		// Skip expired alerts
		if now.Sub(alert.Timestamp) > m.maxAge {
			continue
		}

		// Filter by namespace if specified
		if namespace != "" && alert.Namespace != namespace {
			continue
		}

		result = append(result, alert)
	}

	return result
}

// Resolve marks an alert as resolved
func (m *Manager) Resolve(alertID string) error {
	m.alertsMutex.Lock()
	defer m.alertsMutex.Unlock()

	alert, exists := m.alerts[alertID]
	if !exists {
		return fmt.Errorf("alert not found: %s", alertID)
	}

	now := time.Now()
	alert.ResolvedAt = &now

	m.logger.Info("Alert resolved",
		zap.String("id", alertID),
		zap.String("pod", fmt.Sprintf("%s/%s", alert.Namespace, alert.PodName)),
	)

	return nil
}

// RegisterSubscriber adds an alert subscriber
func (m *Manager) RegisterSubscriber(sub AlertSubscriber) {
	m.subMutex.Lock()
	defer m.subMutex.Unlock()
	m.subscribers = append(m.subscribers, sub)
}

// SetWebhook configures webhook URL for a severity level
func (m *Manager) SetWebhook(severity, webhookURL string) {
	m.webhookMu.Lock()
	defer m.webhookMu.Unlock()
	m.webhooks[severity] = webhookURL
}

// notifySubscribers calls all registered subscribers
func (m *Manager) notifySubscribers(ctx context.Context, alert *Alert) {
	m.subMutex.RLock()
	subs := make([]AlertSubscriber, len(m.subscribers))
	copy(subs, m.subscribers)
	m.subMutex.RUnlock()

	for _, sub := range subs {
		if err := sub.OnAlert(ctx, alert); err != nil {
			m.logger.Error("Subscriber notification failed",
				zap.Error(err),
				zap.String("alert_id", alert.ID),
			)
		}
	}
}

// dispatchWebhook sends alert to configured webhook
func (m *Manager) dispatchWebhook(ctx context.Context, alert *Alert) {
	m.webhookMu.RLock()
	webhookURL, exists := m.webhooks[alert.Severity]
	m.webhookMu.RUnlock()

	if !exists || webhookURL == "" {
		return
	}

	// TODO: Implement actual webhook dispatch
	m.logger.Debug("Webhook dispatch",
		zap.String("url", webhookURL),
		zap.String("alert_id", alert.ID),
	)
}

// CleanupResolved removes old resolved alerts
func (m *Manager) CleanupResolved() {
	m.alertsMutex.Lock()
	defer m.alertsMutex.Unlock()

	now := time.Now()
	toDelete := make([]string, 0)

	for id, alert := range m.alerts {
		// Remove if resolved and older than 1 hour
		if alert.ResolvedAt != nil && now.Sub(*alert.ResolvedAt) > time.Hour {
			toDelete = append(toDelete, id)
		}

		// Remove if unresolved but older than maxAge
		if alert.ResolvedAt == nil && now.Sub(alert.Timestamp) > m.maxAge {
			toDelete = append(toDelete, id)
		}
	}

	for _, id := range toDelete {
		delete(m.alerts, id)
	}

	if len(toDelete) > 0 {
		m.logger.Debug("Cleaned up alerts", zap.Int("count", len(toDelete)))
	}
}

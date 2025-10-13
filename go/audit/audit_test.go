// Copyright (C) 2024 right-sizer contributors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package audit

import (
	"context"
	"right-sizer/config"
	"right-sizer/metrics"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestAuditLoggerInitialization ensures the audit logger initializes without error using defaults.
func TestAuditLoggerInitialization(t *testing.T) {
	cfg := config.GetDefaults()
	opMetrics := metrics.NewOperatorMetrics()
	auditCfg := DefaultAuditConfig()
	logger, err := NewAuditLogger(nil, cfg, opMetrics, auditCfg)
	if err != nil {
		t.Fatalf("expected no error initializing audit logger: %v", err)
	}
	defer func() {
		if err := logger.Close(); err != nil {
			t.Errorf("Failed to close audit logger: %v", err)
		}
	}()

	// Create a fake pod and log a resource change (uses nil client path)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default", UID: "uid-1"}}
	oldReqs := corev1.ResourceRequirements{}
	newReqs := corev1.ResourceRequirements{}
	logger.LogResourceChange(context.TODO(), pod, "app", oldReqs, newReqs, "resize", "initial", "success", 10*time.Millisecond, nil)
}

// TestDefaultAuditConfig verifies default values are sane.
func TestDefaultAuditConfig(t *testing.T) {
	cfg := DefaultAuditConfig()
	if cfg.BufferSize <= 0 || cfg.FlushInterval <= 0 {
		t.Fatalf("invalid defaults: %#v", cfg)
	}
}

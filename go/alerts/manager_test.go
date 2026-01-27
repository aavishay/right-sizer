package alerts

import (
	"context"
	"testing"

	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	logger := zap.NewNop()
	manager := New(logger)

	if manager == nil {
		t.Fatal("Manager should not be nil")
	}
}

func TestCreateAlert(t *testing.T) {
	logger := zap.NewNop()
	manager := New(logger)
	ctx := context.Background()

	alert, err := manager.Create(ctx, "default", "test-pod", "cpu", "warning", "High CPU", "msg", "anomaly", 800.0, 500.0)

	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if alert == nil {
		t.Fatal("Alert should not be nil")
	}
}

func TestListAlerts(t *testing.T) {
	logger := zap.NewNop()
	manager := New(logger)
	ctx := context.Background()

	manager.Create(ctx, "default", "pod1", "cpu", "warning", "Title1", "Msg", "anomaly", 800.0, 500.0)
	manager.Create(ctx, "kube-system", "pod2", "memory", "critical", "Title2", "Msg", "anomaly", 2000.0, 1000.0)

	all := manager.List("")
	if len(all) != 2 {
		t.Errorf("Expected 2 alerts, got %d", len(all))
	}
}

func TestResolveAlert(t *testing.T) {
	logger := zap.NewNop()
	manager := New(logger)
	ctx := context.Background()

	alert, _ := manager.Create(ctx, "default", "test-pod", "cpu", "warning", "High CPU", "msg", "anomaly", 800.0, 500.0)

	manager.Resolve(alert.ID)
	resolved := manager.Get(alert.ID)
	if resolved.ResolvedAt == nil {
		t.Fatal("Alert should be resolved")
	}
}

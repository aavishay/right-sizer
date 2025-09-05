package health_test

import (
	"testing"
	"right-sizer/health"
)

func TestNewOperatorHealthChecker(t *testing.T) {
	checker := health.NewOperatorHealthChecker()
	if checker == nil {
		t.Fatal("Expected non-nil health checker")
	}
}

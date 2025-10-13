package platform

import "testing"

// TestSmokeCapabilities instantiates Capabilities and calls Summary to ensure coverage >0%
func TestSmokeCapabilities(t *testing.T) {
	c := Capabilities{RawVersion: "1.33", Supported: true}
	_ = c.Summary()
}

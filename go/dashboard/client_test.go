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
package dashboard

import (
	"os"
	"testing"
	"time"
)

// TestNewClientDisabled ensures client is disabled when env vars missing
func TestNewClientDisabled(t *testing.T) {
	_ = os.Unsetenv("DASHBOARD_URL")
	_ = os.Unsetenv("DASHBOARD_API_TOKEN")
	_ = os.Unsetenv("CLUSTER_ID")
	c := NewClient()
	if c.IsEnabled() {
		t.Fatalf("expected client disabled when env vars missing")
	}
}

// TestNewClientEnabled ensures client enabled when env vars present
func TestNewClientEnabled(t *testing.T) {
	_ = os.Setenv("DASHBOARD_URL", "https://example.local")
	_ = os.Setenv("DASHBOARD_API_TOKEN", "abcdef1234567890")
	_ = os.Setenv("CLUSTER_ID", "cluster-1")
	_ = os.Setenv("CLUSTER_NAME", "dev-cluster")
	c := NewClient()
	if !c.IsEnabled() {
		t.Fatalf("expected client enabled with env vars set")
	}
}

// TestMaskToken verifies masking logic
func TestMaskToken(t *testing.T) {
	if maskToken("") != "<not set>" {
		t.Fatalf("empty token masking failed")
	}
	if maskToken("short") != "<masked>" {
		t.Fatalf("short token masking failed")
	}
	masked := maskToken("abcdefghijklmnop")
	if masked != "abcdefgh..." {
		t.Fatalf("long token masking failed: %s", masked)
	}
}

// TestStartHeartbeat no panic when disabled
func TestStartHeartbeatDisabled(t *testing.T) {
	_ = os.Unsetenv("DASHBOARD_URL")
	_ = os.Unsetenv("DASHBOARD_API_TOKEN")
	_ = os.Unsetenv("CLUSTER_ID")
	c := NewClient()
	c.StartHeartbeat(50 * time.Millisecond) // should return immediately
}

// Copyright (C) 2025 right-sizer contributors
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

// Package platform contains runtime Kubernetes cluster capability discovery and
// version support evaluation. It is intentionally self‑contained so that other
// packages (controllers, analyzers, API) can depend on a stable abstraction
// without pulling in higher‑level logic.
package platform

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
)

// MinimumSupportedMinor is the lowest Kubernetes minor version (for major=1)
// that the operator actively supports at runtime. The assignment here enforces
// "1.33 and above" as requested.
const MinimumSupportedMinor = 33

// Capabilities enumerates detected cluster features. All boolean fields are
// optimistic (true means confidently supported).
type Capabilities struct {
	// RawVersion is Major.Minor as reported by the apiserver.
	RawVersion string
	Major      int
	Minor      int

	// Supported indicates the cluster version satisfies the operator's declared
	// minimum (>=1.MinSupported). If false, some functionality may be disabled.
	Supported bool
	// VersionWarning provides human readable context if Supported=false (or other soft incompatibility).
	VersionWarning string

	// Feature / API surface toggles:
	EphemeralContainers       bool // pods/ephemeralcontainers subresource
	PodResize                 bool // pods/resize subresource (in-place resource resize)
	MetricsServerAvailable    bool // metrics.k8s.io group discoverable
	DynamicResourceAllocation bool // resource.k8s.io group (DRA)
	// Future placeholders (wire when upstream surfaces stable signals):
	InPlacePodVerticalScaling bool // hypothetical GA alias to PodResize
	MemoryQoS                 bool // heuristic / detection stub (cgroup + feature gate)
}

// Detector performs capability discovery using the Kubernetes discovery API.
type Detector struct {
	disc discovery.DiscoveryInterface
}

// NewDetector constructs a Detector from a client-go kubernetes.Interface.
func NewDetector(cs kubernetes.Interface) *Detector {
	return &Detector{disc: cs.Discovery()}
}

// Detect queries the apiserver and populates Capabilities. It never panics.
// It prefers to degrade (Supported=false with a warning) rather than fail the operator.
// A hard error is returned only if server version cannot be retrieved at all.
func (d *Detector) Detect(ctx context.Context) (Capabilities, error) {
	var caps Capabilities

	// 1. Server version
	sv, err := d.disc.ServerVersion()
	if err != nil {
		return caps, fmt.Errorf("fetch server version: %w", err)
	}

	major, minor, parseWarn := parseVersion(sv.Major, sv.Minor)
	caps.Major = major
	caps.Minor = minor
	caps.RawVersion = fmt.Sprintf("%d.%d", major, minor)

	// Evaluate support window (only enforcing minimum)
	if major == 1 && minor >= MinimumSupportedMinor {
		caps.Supported = true
	} else {
		caps.Supported = false
		caps.VersionWarning = fmt.Sprintf("cluster %s < required 1.%d (reduced functionality)", caps.RawVersion, MinimumSupportedMinor)
	}

	if parseWarn != "" {
		if caps.VersionWarning == "" {
			caps.VersionWarning = parseWarn
		} else {
			caps.VersionWarning = caps.VersionWarning + "; " + parseWarn
		}
	}

	// 2. API group/resource discovery
	groups, resourceLists, err := d.disc.ServerGroupsAndResources()
	if err != nil {
		// Partial discovery is still usable (may return a GroupDiscoveryFailedError)
		if !discovery.IsGroupDiscoveryFailedError(err) {
			if caps.VersionWarning == "" {
				caps.VersionWarning = fmt.Sprintf("partial discovery error: %v", err)
			} else {
				caps.VersionWarning += fmt.Sprintf("; partial discovery error: %v", err)
			}
			return caps, nil
		}
	}

	// Build a quick set of discovered group names
	groupSet := make(map[string]struct{}, len(groups))
	for _, g := range groups {
		groupSet[g.Name] = struct{}{}
	}

	// Group presence flags
	if _, ok := groupSet["metrics.k8s.io"]; ok {
		caps.MetricsServerAvailable = true
	}
	if _, ok := groupSet["resource.k8s.io"]; ok {
		caps.DynamicResourceAllocation = true
	}

	// Iterate all resource lists (groupVersion scoped)
	for _, rl := range resourceLists {
		if rl == nil {
			continue
		}
		// Extract group portion from GroupVersion (e.g. "metrics.k8s.io/v1beta1" -> "metrics.k8s.io")
		groupVersion := rl.GroupVersion
		groupName := groupVersion
		if idx := strings.Index(groupVersion, "/"); idx >= 0 {
			groupName = groupVersion[:idx]
		}
		// Core group appears as "v1"
		if groupName == "v1" {
			groupName = ""
		}

		for _, r := range rl.APIResources {
			full := strings.ToLower(r.Name)
			switch full {
			case "pods/ephemeralcontainers":
				caps.EphemeralContainers = true
			case "pods/resize":
				caps.PodResize = true
				caps.InPlacePodVerticalScaling = true
			}
		}
	}

	// 3. Memory QoS heuristic (placeholder)
	// The MemoryQoS feature when enabled influences memory eviction / reclaim behavior.
	// A robust detection may require node feature gate inspection or cgroup layout introspection.
	// For now mark true only for >=1.33 (heuristic).
	if caps.Supported && caps.Minor >= 33 {
		caps.MemoryQoS = true
	}

	return caps, nil
}

// ValidateOrError returns an error if cluster version is below supported minimum.
// This can be called by startup code to decide whether to abort or just warn.
func (c Capabilities) ValidateOrError(enforce bool) error {
	if c.Supported {
		return nil
	}
	if enforce {
		return fmt.Errorf("unsupported cluster version %s: need >=1.%d", c.RawVersion, MinimumSupportedMinor)
	}
	return nil
}

// Summary produces a short, human-readable summary line.
func (c Capabilities) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "k8s=%s (supported=%t", c.RawVersion, c.Supported)
	if c.VersionWarning != "" {
		fmt.Fprintf(&b, " warn=%q", c.VersionWarning)
	}
	fmt.Fprintf(&b, ") feats:[")
	feats := []string{}
	add := func(name string, v bool) {
		if v {
			feats = append(feats, name)
		}
	}
	add("ephemeralContainers", c.EphemeralContainers)
	add("podResize", c.PodResize)
	add("metricsServer", c.MetricsServerAvailable)
	add("dra", c.DynamicResourceAllocation)
	add("memQoS", c.MemoryQoS)
	add("inPlaceVS", c.InPlacePodVerticalScaling)
	b.WriteString(strings.Join(feats, ","))
	b.WriteString("]")
	return b.String()
}

// parseVersion normalizes the Major/Minor strings returned by the apiserver.
// Kubernetes Minor sometimes includes trailing "+" or labels (e.g. "34+").
func parseVersion(majorStr, minorStr string) (int, int, string) {
	warn := ""
	major, err := strconv.Atoi(strings.TrimSpace(majorStr))
	if err != nil {
		warn = fmt.Sprintf("cannot parse major version %q", majorStr)
		major = 0
	}
	// Strip non-digit suffix
	digits := regexp.MustCompile(`^(\d+)`).FindString(minorStr)
	if digits == "" {
		if warn == "" {
			warn = fmt.Sprintf("cannot parse minor version %q", minorStr)
		} else {
			warn += fmt.Sprintf("; cannot parse minor version %q", minorStr)
		}
		return major, 0, warn
	}
	minor, err2 := strconv.Atoi(digits)
	if err2 != nil && warn == "" {
		warn = fmt.Sprintf("cannot parse minor version %q", minorStr)
	} else if err2 != nil {
		warn += fmt.Sprintf("; cannot parse minor version %q", minorStr)
	}
	return major, minor, warn
}

// ErrUnsupportedVersion is returned when strict enforcement is requested and
// the cluster version is too old.
var ErrUnsupportedVersion = errors.New("unsupported kubernetes version")

// EnforceMinimum returns ErrUnsupportedVersion if capabilities indicate the
// cluster is below minimum. Provided for convenience.
func EnforceMinimum(c Capabilities) error {
	if !c.Supported {
		return fmt.Errorf("%w: %s (need >=1.%d)", ErrUnsupportedVersion, c.RawVersion, MinimumSupportedMinor)
	}
	return nil
}

// MergeCapabilities merges two capability structs, preferring 'primary' but
// allowing secondary to fill unset (false) booleans. Useful if you extend
// detection with out-of-band sources (node agent, config map, etc.).
func MergeCapabilities(primary, secondary Capabilities) Capabilities {
	out := primary
	if !out.EphemeralContainers && secondary.EphemeralContainers {
		out.EphemeralContainers = true
	}
	if !out.PodResize && secondary.PodResize {
		out.PodResize = true
	}
	if !out.MetricsServerAvailable && secondary.MetricsServerAvailable {
		out.MetricsServerAvailable = true
	}
	if !out.DynamicResourceAllocation && secondary.DynamicResourceAllocation {
		out.DynamicResourceAllocation = true
	}
	if !out.InPlacePodVerticalScaling && secondary.InPlacePodVerticalScaling {
		out.InPlacePodVerticalScaling = true
	}
	if !out.MemoryQoS && secondary.MemoryQoS {
		out.MemoryQoS = true
	}
	return out
}

// WithTimeout convenience wrapper to bound detection time.
func (d *Detector) WithTimeout(parent context.Context, dur time.Duration) (Capabilities, error) {
	ctx, cancel := context.WithTimeout(parent, dur)
	defer cancel()
	return d.Detect(ctx)
}

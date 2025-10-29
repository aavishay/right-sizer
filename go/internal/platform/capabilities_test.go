package platform

import (
"testing"

"github.com/stretchr/testify/assert"
)

func TestCapabilities_Summary(t *testing.T) {
tests := []struct {
name string
caps Capabilities
}{
{
name: "supported version with all features",
caps: Capabilities{
RawVersion:                "1.33",
Major:                     1,
Minor:                     33,
Supported:                 true,
EphemeralContainers:       true,
PodResize:                 true,
MetricsServerAvailable:    true,
DynamicResourceAllocation: true,
InPlacePodVerticalScaling: true,
MemoryQoS:                 true,
},
},
{
name: "unsupported version",
caps: Capabilities{
RawVersion:     "1.30",
Major:          1,
Minor:          30,
Supported:      false,
VersionWarning: "cluster 1.30 < required 1.33",
},
},
{
name: "minimal supported version",
caps: Capabilities{
RawVersion: "1.33",
Major:      1,
Minor:      33,
Supported:  true,
},
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
summary := tt.caps.Summary()
assert.NotEmpty(t, summary)
assert.Contains(t, summary, tt.caps.RawVersion)
})
}
}

func TestCapabilities_ValidateOrError(t *testing.T) {
tests := []struct {
name    string
caps    Capabilities
enforce bool
wantErr bool
}{
{
name: "supported version, enforce true",
caps: Capabilities{
Supported: true,
},
enforce: true,
wantErr: false,
},
{
name: "unsupported version, enforce false",
caps: Capabilities{
Supported:      false,
VersionWarning: "version too old",
},
enforce: false,
wantErr: false,
},
{
name: "unsupported version, enforce true",
caps: Capabilities{
Supported:      false,
VersionWarning: "version too old",
},
enforce: true,
wantErr: true,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
err := tt.caps.ValidateOrError(tt.enforce)
if tt.wantErr {
assert.Error(t, err)
} else {
assert.NoError(t, err)
}
})
}
}

func TestEnforceMinimum(t *testing.T) {
tests := []struct {
name    string
caps    Capabilities
wantErr bool
}{
{
name: "supported version",
caps: Capabilities{
Supported: true,
},
wantErr: false,
},
{
name: "unsupported version",
caps: Capabilities{
Supported:      false,
VersionWarning: "version too old",
},
wantErr: true,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
err := EnforceMinimum(tt.caps)
if tt.wantErr {
assert.Error(t, err)
} else {
assert.NoError(t, err)
}
})
}
}

func TestMergeCapabilities(t *testing.T) {
primary := Capabilities{
RawVersion:             "1.33",
Major:                  1,
Minor:                  33,
Supported:              true,
EphemeralContainers:    true,
PodResize:              false,
MetricsServerAvailable: true,
}

secondary := Capabilities{
RawVersion:             "1.33",
Major:                  1,
Minor:                  33,
Supported:              true,
EphemeralContainers:    false,
PodResize:              true,
MetricsServerAvailable: false,
}

merged := MergeCapabilities(primary, secondary)

// Verify primary values take precedence
assert.Equal(t, primary.RawVersion, merged.RawVersion)
assert.Equal(t, primary.Major, merged.Major)
assert.Equal(t, primary.Minor, merged.Minor)
}

func TestMinimumSupportedMinor(t *testing.T) {
assert.Equal(t, 33, MinimumSupportedMinor)
}

func TestCapabilitiesFields(t *testing.T) {
caps := Capabilities{
RawVersion:                "1.34",
Major:                     1,
Minor:                     34,
Supported:                 true,
VersionWarning:            "",
EphemeralContainers:       true,
PodResize:                 true,
MetricsServerAvailable:    true,
DynamicResourceAllocation: false,
InPlacePodVerticalScaling: true,
MemoryQoS:                 false,
}

assert.Equal(t, "1.34", caps.RawVersion)
assert.Equal(t, 1, caps.Major)
assert.Equal(t, 34, caps.Minor)
assert.True(t, caps.Supported)
assert.True(t, caps.EphemeralContainers)
assert.True(t, caps.PodResize)
assert.True(t, caps.MetricsServerAvailable)
assert.False(t, caps.DynamicResourceAllocation)
}

package policy

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"right-sizer/config"
	"right-sizer/metrics"
)

func TestNewPolicyEngine(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	assert.NotNil(t, engine)
	assert.NotNil(t, engine.client)
	assert.Equal(t, cfg, engine.config)
	assert.Equal(t, metrics, engine.metrics)
	assert.Empty(t, engine.rules)
}

func TestPolicyEngine_AddRule(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	rule := Rule{
		Name:     "test-rule",
		Priority: 100,
		Enabled:  true,
	}

	engine.AddRule(rule)

	assert.Len(t, engine.rules, 1)
	assert.Equal(t, "test-rule", engine.rules[0].Name)
}

func TestPolicyEngine_GetRules(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	rule1 := Rule{Name: "rule1", Enabled: true}
	rule2 := Rule{Name: "rule2", Enabled: false}

	engine.AddRule(rule1)
	engine.AddRule(rule2)

	rules := engine.GetRules()
	assert.Len(t, rules, 2)
	assert.Equal(t, "rule1", rules[0].Name)
	assert.Equal(t, "rule2", rules[1].Name)
}

func TestPolicyEngine_GetActiveRules(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	rule1 := Rule{Name: "rule1", Enabled: true}
	rule2 := Rule{Name: "rule2", Enabled: false}
	rule3 := Rule{Name: "rule3", Enabled: true}

	engine.AddRule(rule1)
	engine.AddRule(rule2)
	engine.AddRule(rule3)

	activeRules := engine.GetActiveRules()
	assert.Len(t, activeRules, 2)
	assert.Equal(t, "rule1", activeRules[0].Name)
	assert.Equal(t, "rule3", activeRules[1].Name)
}

func TestPolicyEngine_EvaluatePolicy_NoRules(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	result := engine.EvaluatePolicy(context.Background(), pod, "test-container", nil)

	assert.NotNil(t, result)
	assert.Empty(t, result.AppliedRules)
	assert.False(t, result.Skip)
}

func TestPolicyEngine_EvaluatePolicy_WithMatchingRule(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	rule := Rule{
		Name:     "cpu-boost",
		Priority: 100,
		Enabled:  true,
		Selectors: RuleSelectors{
			Labels: map[string]string{
				"app": "test",
			},
		},
		Actions: RuleActions{
			CPUMultiplier: floatPtr(1.5),
		},
	}

	engine.AddRule(rule)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
		},
	}

	currentUsage := map[string]*resource.Quantity{
		"cpu": func() *resource.Quantity { q := resource.MustParse("100m"); return &q }(),
	}

	result := engine.EvaluatePolicy(context.Background(), pod, "test-container", currentUsage)

	assert.NotNil(t, result)
	assert.Len(t, result.AppliedRules, 1)
	assert.Equal(t, "cpu-boost", result.AppliedRules[0])
	assert.NotNil(t, result.RecommendedCPU)
	assert.Equal(t, int64(150), result.RecommendedCPU.MilliValue())
}

func TestPolicyEngine_EvaluatePolicy_SkipRule(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	rule := Rule{
		Name:     "skip-system",
		Priority: 200,
		Enabled:  true,
		Selectors: RuleSelectors{
			Namespaces: []string{"kube-system"},
		},
		Actions: RuleActions{
			Skip: true,
		},
	}

	engine.AddRule(rule)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "system-pod",
			Namespace: "kube-system",
		},
	}

	result := engine.EvaluatePolicy(context.Background(), pod, "test-container", nil)

	assert.NotNil(t, result)
	assert.Len(t, result.AppliedRules, 1)
	assert.Equal(t, "skip-system", result.AppliedRules[0])
	assert.True(t, result.Skip)
	assert.Contains(t, result.Reason, "skip-system")
}

func TestPolicyEngine_RuleMatches_Namespace(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	rule := Rule{
		Name:    "namespace-rule",
		Enabled: true,
		Selectors: RuleSelectors{
			Namespaces: []string{"production", "staging"},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "production",
		},
	}

	matches, reason := engine.ruleMatches(context.Background(), rule, pod, "test-container")
	assert.True(t, matches)
	assert.Empty(t, reason)
}

func TestPolicyEngine_RuleMatches_NamespaceNoMatch(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	rule := Rule{
		Name:    "namespace-rule",
		Enabled: true,
		Selectors: RuleSelectors{
			Namespaces: []string{"production", "staging"},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "development",
		},
	}

	matches, reason := engine.ruleMatches(context.Background(), rule, pod, "test-container")
	assert.False(t, matches)
	assert.Equal(t, "namespace not in selector list", reason)
}

func TestPolicyEngine_RuleMatches_Labels(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	rule := Rule{
		Name:    "label-rule",
		Enabled: true,
		Selectors: RuleSelectors{
			Labels: map[string]string{
				"app":  "web",
				"tier": "test-tier",
			},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Labels: map[string]string{
				"app":     "web",
				"tier":    "test-tier",
				"version": "v1",
			},
		},
	}

	matches, reason := engine.ruleMatches(context.Background(), rule, pod, "test-container")
	assert.True(t, matches)
	assert.Empty(t, reason)
}

func TestPolicyEngine_RuleMatches_LabelsNoMatch(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	rule := Rule{
		Name:    "label-rule",
		Enabled: true,
		Selectors: RuleSelectors{
			Labels: map[string]string{
				"app":  "web",
				"tier": "test-tier",
			},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Labels: map[string]string{
				"app":  "api",
				"tier": "backend",
			},
		},
	}

	matches, reason := engine.ruleMatches(context.Background(), rule, pod, "test-container")
	assert.False(t, matches)
	assert.Equal(t, "labels do not match", reason)
}

func TestPolicyEngine_RuleMatches_Annotations(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	rule := Rule{
		Name:    "annotation-rule",
		Enabled: true,
		Selectors: RuleSelectors{
			Annotations: map[string]string{
				"rightsizer.io/enabled": "true",
			},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Annotations: map[string]string{
				"rightsizer.io/enabled": "true",
				"other/annotation":      "value",
			},
		},
	}

	matches, reason := engine.ruleMatches(context.Background(), rule, pod, "test-container")
	assert.True(t, matches)
	assert.Empty(t, reason)
}

func TestPolicyEngine_RuleMatches_PodNameRegex(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	rule := Rule{
		Name:    "regex-rule",
		Enabled: true,
		Selectors: RuleSelectors{
			PodNameRegex: "^web-.*",
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web-app-123", // Changed to match the regex "^web-.*"
			Namespace: "default",
		},
	}

	matches, reason := engine.ruleMatches(context.Background(), rule, pod, "test-container")
	assert.True(t, matches)
	assert.Empty(t, reason)
}

func TestPolicyEngine_RuleMatches_PodNameRegexNoMatch(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	rule := Rule{
		Name:    "regex-rule",
		Enabled: true,
		Selectors: RuleSelectors{
			PodNameRegex: "^web-.*",
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api-backend-456",
			Namespace: "default",
		},
	}

	matches, reason := engine.ruleMatches(context.Background(), rule, pod, "test-container")
	assert.False(t, matches)
	assert.Equal(t, "pod name does not match regex", reason)
}

func TestPolicyEngine_RuleMatches_ContainerName(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	rule := Rule{
		Name:    "container-rule",
		Enabled: true,
		Selectors: RuleSelectors{
			ContainerName: "nginx",
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	matches, reason := engine.ruleMatches(context.Background(), rule, pod, "nginx")
	assert.True(t, matches)
	assert.Empty(t, reason)
}

func TestPolicyEngine_RuleMatches_ContainerNameNoMatch(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	rule := Rule{
		Name:    "container-rule",
		Enabled: true,
		Selectors: RuleSelectors{
			ContainerName: "nginx",
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	matches, reason := engine.ruleMatches(context.Background(), rule, pod, "apache")
	assert.False(t, matches)
	assert.Equal(t, "container name does not match", reason)
}

func TestPolicyEngine_GetQoSClass(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected corev1.PodQOSClass
	}{
		{
			name: "Guaranteed QoS",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
					},
				},
			},
			expected: corev1.PodQOSGuaranteed,
		},
		{
			name: "Burstable QoS - has requests",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
							},
						},
					},
				},
			},
			expected: corev1.PodQOSBurstable,
		},
		{
			name: "BestEffort QoS - no resources",
			pod: &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test",
						},
					},
				},
			},
			expected: corev1.PodQOSBestEffort,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getQoSClass(tt.pod)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPolicyEngine_GetWorkloadType(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	tests := []struct {
		name     string
		pod      *corev1.Pod
		expected string
	}{
		{
			name: "Deployment workload",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "Deployment",
							Name: "test-deployment",
						},
					},
				},
			},
			expected: "Deployment",
		},
		{
			name: "StatefulSet workload",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "StatefulSet",
							Name: "test-statefulset",
						},
					},
				},
			},
			expected: "StatefulSet",
		},
		{
			name: "Job workload",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							Kind: "Job",
							Name: "test-job",
						},
					},
				},
			},
			expected: "Job",
		},
		{
			name: "Workload from labels",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/component": "database",
					},
				},
			},
			expected: "database",
		},
		{
			name: "Default to Pod",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "standalone-pod",
				},
			},
			expected: "Pod",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.getWorkloadType(tt.pod)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPolicyEngine_SortRulesByPriority(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	rules := []Rule{
		{Name: "low-priority", Priority: 10},
		{Name: "high-priority", Priority: 100},
		{Name: "medium-priority", Priority: 50},
	}

	engine.rules = rules
	sorted := engine.sortRulesByPriority()

	assert.Len(t, sorted, 3)
	assert.Equal(t, "high-priority", sorted[0].Name)
	assert.Equal(t, "medium-priority", sorted[1].Name)
	assert.Equal(t, "low-priority", sorted[2].Name)
}

func TestPolicyEngine_IsTimeInRange(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	tests := []struct {
		name     string
		current  string
		start    string
		end      string
		expected bool
	}{
		{
			name:     "within range",
			current:  "10:30",
			start:    "09:00",
			end:      "11:00",
			expected: true,
		},
		{
			name:     "before range",
			current:  "08:30",
			start:    "09:00",
			end:      "11:00",
			expected: false,
		},
		{
			name:     "after range",
			current:  "12:00",
			start:    "09:00",
			end:      "11:00",
			expected: false,
		},
		{
			name:     "at start boundary",
			current:  "09:00",
			start:    "09:00",
			end:      "11:00",
			expected: true,
		},
		{
			name:     "at end boundary",
			current:  "11:00",
			start:    "09:00",
			end:      "11:00",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.isTimeInRange(tt.current, tt.start, tt.end)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPolicyEngine_ValidateRules(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	validRule := Rule{
		Name: "valid-rule",
		Actions: RuleActions{
			MinCPU:    stringPtr("100m"),
			MaxCPU:    stringPtr("200m"),
			MinMemory: stringPtr("128Mi"),
			MaxMemory: stringPtr("256Mi"),
		},
	}

	invalidRule := Rule{
		Name: "", // Empty name
	}

	engine.AddRule(validRule)
	engine.AddRule(invalidRule)

	err := engine.ValidateRules()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "rule name cannot be empty")
}

func TestPolicyEngine_ValidateRule(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	tests := []struct {
		name     string
		rule     Rule
		expected error
	}{
		{
			name: "valid rule",
			rule: Rule{
				Name: "valid-rule",
				Actions: RuleActions{
					MinCPU:    stringPtr("100m"),
					MaxCPU:    stringPtr("200m"),
					MinMemory: stringPtr("128Mi"),
					MaxMemory: stringPtr("256Mi"),
				},
			},
			expected: nil,
		},
		{
			name: "empty name",
			rule: Rule{
				Name: "",
			},
			expected: errors.New("rule name cannot be empty"),
		},
		{
			name: "invalid regex",
			rule: Rule{
				Name: "invalid-regex",
				Selectors: RuleSelectors{
					PodNameRegex: "[invalid",
				},
			},
			expected: errors.New("invalid pod name regex"),
		},
		{
			name: "invalid CPU quantity",
			rule: Rule{
				Name: "invalid-cpu",
				Actions: RuleActions{
					MinCPU: stringPtr("invalid"),
				},
			},
			expected: errors.New("invalid minCPU"),
		},
		{
			name: "invalid memory quantity",
			rule: Rule{
				Name: "invalid-memory",
				Actions: RuleActions{
					MinMemory: stringPtr("invalid"),
				},
			},
			expected: errors.New("invalid minMemory"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.validateRule(tt.rule)
			if tt.expected == nil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expected.Error())
			}
		})
	}
}

func TestPolicyEngine_ApplyRuleActions(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}

	currentUsage := map[string]*resource.Quantity{
		"cpu":    func() *resource.Quantity { q := resource.MustParse("100m"); return &q }(),
		"memory": func() *resource.Quantity { q := resource.MustParse("128Mi"); return &q }(),
	}

	result := &PolicyEvaluationResult{}

	rule := Rule{
		Name: "test-rule",
		Actions: RuleActions{
			CPUMultiplier: func() *float64 { f := 1.5; return &f }(),
		},
	}

	engine.applyRuleActions(rule, pod, "test-container", currentUsage, result)

	assert.NotNil(t, result.RecommendedCPU)
	assert.Equal(t, int64(150), result.RecommendedCPU.MilliValue()) // 100m * 1.5

	assert.NotNil(t, result.RecommendedMemory)
	assert.Equal(t, int64(256*1024*1024), result.RecommendedMemory.Value()) // 128Mi * 2.0
}

func TestPolicyEngine_ApplyMinMaxConstraints(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	result := &PolicyEvaluationResult{
		RecommendedCPU:    resourcePtr(resource.MustParse("300m")),
		RecommendedMemory: resourcePtr(resource.MustParse("1Gi")),
	}

	actions := RuleActions{
		MinCPU:    stringPtr("200m"),
		MaxCPU:    stringPtr("500m"),
		MinMemory: stringPtr("512Mi"),
		MaxMemory: stringPtr("2Gi"),
	}

	engine.applyMinMaxConstraints(actions, result)

	// CPU should be constrained to min (200m > 300m? No, 300m is already >= 200m)
	assert.Equal(t, int64(300), result.RecommendedCPU.MilliValue())

	// Memory should be constrained to max (1Gi = 1073741824 bytes, 2Gi = 2147483648 bytes)
	// 1Gi should be <= 2Gi, so no change
	assert.Equal(t, int64(1073741824), result.RecommendedMemory.Value())
}

func TestPolicyEngine_IsRuleScheduleActive(t *testing.T) {
	client := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	cfg := config.GetDefaults()
	metrics := metrics.NewOperatorMetrics()

	engine := NewPolicyEngine(client, cfg, metrics)

	// Test with no schedule (should be active)
	schedule := &RuleSchedule{}
	active := engine.isRuleScheduleActive(schedule)
	assert.True(t, active)

	// Test with time range
	now := time.Now()
	startHour := now.Add(-time.Hour).Format("15:04")
	endHour := now.Add(time.Hour).Format("15:04")

	scheduleWithTime := &RuleSchedule{
		TimeRanges: []TimeRange{
			{
				Start: startHour,
				End:   endHour,
			},
		},
	}

	active = engine.isRuleScheduleActive(scheduleWithTime)
	assert.True(t, active)
}

// Helper function for tests
func resourcePtr(r resource.Quantity) *resource.Quantity {
	return &r
}

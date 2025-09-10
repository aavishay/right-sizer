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

package policy

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"right-sizer/config"
	"right-sizer/logger"
	"right-sizer/metrics"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PolicyEngine manages and evaluates resource sizing policies
type PolicyEngine struct {
	client  client.Client
	config  *config.Config
	metrics *metrics.OperatorMetrics
	rules   []Rule
}

// Rule represents a resource sizing rule
type Rule struct {
	Name        string        `json:"name"               yaml:"name"`
	Description string        `json:"description"        yaml:"description"`
	Priority    int           `json:"priority"           yaml:"priority"`
	Selectors   RuleSelectors `json:"selectors"          yaml:"selectors"`
	Actions     RuleActions   `json:"actions"            yaml:"actions"`
	Schedule    *RuleSchedule `json:"schedule,omitempty" yaml:"schedule,omitempty"`
	Enabled     bool          `json:"enabled"            yaml:"enabled"`
}

// RuleSelectors defines criteria for rule matching
type RuleSelectors struct {
	Namespaces    []string          `json:"namespaces,omitempty"    yaml:"namespaces,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"        yaml:"labels,omitempty"`
	Annotations   map[string]string `json:"annotations,omitempty"   yaml:"annotations,omitempty"`
	PodNameRegex  string            `json:"podNameRegex,omitempty"  yaml:"podNameRegex,omitempty"`
	ContainerName string            `json:"containerName,omitempty" yaml:"containerName,omitempty"`
	QoSClass      string            `json:"qosClass,omitempty"      yaml:"qosClass,omitempty"`
	WorkloadType  string            `json:"workloadType,omitempty"  yaml:"workloadType,omitempty"`
}

// RuleActions defines what actions to take when rule matches
type RuleActions struct {
	CPUMultiplier    *float64 `json:"cpuMultiplier,omitempty"    yaml:"cpuMultiplier,omitempty"`
	MemoryMultiplier *float64 `json:"memoryMultiplier,omitempty" yaml:"memoryMultiplier,omitempty"`
	MinCPU           *string  `json:"minCPU,omitempty"           yaml:"minCPU,omitempty"`
	MaxCPU           *string  `json:"maxCPU,omitempty"           yaml:"maxCPU,omitempty"`
	MinMemory        *string  `json:"minMemory,omitempty"        yaml:"minMemory,omitempty"`
	MaxMemory        *string  `json:"maxMemory,omitempty"        yaml:"maxMemory,omitempty"`
	SetCPURequest    *string  `json:"setCPURequest,omitempty"    yaml:"setCPURequest,omitempty"`
	SetMemoryRequest *string  `json:"setMemoryRequest,omitempty" yaml:"setMemoryRequest,omitempty"`
	SetCPULimit      *string  `json:"setCPULimit,omitempty"      yaml:"setCPULimit,omitempty"`
	SetMemoryLimit   *string  `json:"setMemoryLimit,omitempty"   yaml:"setMemoryLimit,omitempty"`
	Skip             bool     `json:"skip,omitempty"             yaml:"skip,omitempty"`
}

// RuleSchedule defines when a rule should be active
type RuleSchedule struct {
	TimeRanges []TimeRange `json:"timeRanges,omitempty" yaml:"timeRanges,omitempty"`
	DaysOfWeek []string    `json:"daysOfWeek,omitempty" yaml:"daysOfWeek,omitempty"`
	Timezone   string      `json:"timezone,omitempty"   yaml:"timezone,omitempty"`
}

// TimeRange represents a time range during which a rule is active
type TimeRange struct {
	Start string `json:"start" yaml:"start"` // HH:MM format
	End   string `json:"end"   yaml:"end"`   // HH:MM format
}

// PolicyEvaluationResult contains the result of policy evaluation
type PolicyEvaluationResult struct {
	AppliedRules      []string
	RecommendedCPU    *resource.Quantity
	RecommendedMemory *resource.Quantity
	CPULimit          *resource.Quantity
	MemoryLimit       *resource.Quantity
	Skip              bool
	Reason            string
}

// NewPolicyEngine creates a new policy engine
func NewPolicyEngine(client client.Client, cfg *config.Config, metrics *metrics.OperatorMetrics) *PolicyEngine {
	return &PolicyEngine{
		client:  client,
		config:  cfg,
		metrics: metrics,
		rules:   []Rule{},
	}
}

// LoadRules loads rules from ConfigMap
func (pe *PolicyEngine) LoadRules(ctx context.Context, configMapName, namespace string) error {
	configMap := &corev1.ConfigMap{}
	err := pe.client.Get(ctx, client.ObjectKey{
		Name:      configMapName,
		Namespace: namespace,
	}, configMap)
	if err != nil {
		return fmt.Errorf("failed to load policy ConfigMap %s/%s: %w", namespace, configMapName, err)
	}

	// Parse rules from ConfigMap data
	rules, err := pe.parseRulesFromConfigMap(configMap)
	if err != nil {
		return fmt.Errorf("failed to parse rules: %w", err)
	}

	pe.rules = rules
	logger.Info("Loaded %d policy rules from ConfigMap %s/%s", len(rules), namespace, configMapName)

	return nil
}

// AddRule adds a rule to the policy engine
func (pe *PolicyEngine) AddRule(rule Rule) {
	pe.rules = append(pe.rules, rule)
	logger.Debug("Added policy rule: %s", rule.Name)
}

// EvaluatePolicy evaluates policies for a pod and returns sizing recommendations
func (pe *PolicyEngine) EvaluatePolicy(ctx context.Context, pod *corev1.Pod, containerName string, currentUsage map[string]*resource.Quantity) *PolicyEvaluationResult {
	result := &PolicyEvaluationResult{
		AppliedRules: []string{},
	}

	// Sort rules by priority (higher priority first)
	sortedRules := pe.sortRulesByPriority()

	for _, rule := range sortedRules {
		if !rule.Enabled {
			continue
		}

		// Check if rule matches
		if matches, reason := pe.ruleMatches(ctx, rule, pod, containerName); matches {
			result.AppliedRules = append(result.AppliedRules, rule.Name)

			// Apply rule actions
			pe.applyRuleActions(rule, pod, containerName, currentUsage, result)

			// Record metrics
			if pe.metrics != nil {
				pe.metrics.RecordPolicyRuleApplication(rule.Name, "resource_sizing", "applied")
			}

			logger.Debug("Applied policy rule %s to pod %s/%s container %s",
				rule.Name, pod.Namespace, pod.Name, containerName)

			// If rule says to skip, break early
			if result.Skip {
				result.Reason = "Skipped by policy rule: " + rule.Name
				break
			}
		} else if reason != "" {
			logger.Debug("Rule %s did not match pod %s/%s: %s", rule.Name, pod.Namespace, pod.Name, reason)
		}
	}

	return result
}

// ruleMatches checks if a rule matches the given pod and container
func (pe *PolicyEngine) ruleMatches(ctx context.Context, rule Rule, pod *corev1.Pod, containerName string) (bool, string) {
	selectors := rule.Selectors

	// Check schedule
	if rule.Schedule != nil && !pe.isRuleScheduleActive(rule.Schedule) {
		return false, "rule not active according to schedule"
	}

	// Check namespace
	if len(selectors.Namespaces) > 0 {
		found := false
		for _, ns := range selectors.Namespaces {
			if ns == pod.Namespace {
				found = true
				break
			}
		}
		if !found {
			return false, "namespace not in selector list"
		}
	}

	// Check labels
	if len(selectors.Labels) > 0 {
		podLabels := labels.Set(pod.Labels)
		ruleLabels := labels.Set(selectors.Labels)
		if !ruleLabels.AsSelector().Matches(podLabels) {
			return false, "labels do not match"
		}
	}

	// Check annotations
	if len(selectors.Annotations) > 0 {
		for key, value := range selectors.Annotations {
			if podValue, exists := pod.Annotations[key]; !exists || podValue != value {
				return false, fmt.Sprintf("annotation %s does not match", key)
			}
		}
	}

	// Check pod name regex
	if selectors.PodNameRegex != "" {
		matched, err := regexp.MatchString(selectors.PodNameRegex, pod.Name)
		if err != nil {
			logger.Warn("Invalid regex in rule %s: %v", rule.Name, err)
			return false, "invalid regex pattern"
		}
		if !matched {
			return false, "pod name does not match regex"
		}
	}

	// Check container name
	if selectors.ContainerName != "" && selectors.ContainerName != containerName {
		return false, "container name does not match"
	}

	// Check QoS class
	if selectors.QoSClass != "" {
		podQoS := string(getQoSClass(pod))
		if selectors.QoSClass != podQoS {
			return false, fmt.Sprintf("QoS class %s does not match %s", podQoS, selectors.QoSClass)
		}
	}

	// Check workload type
	if selectors.WorkloadType != "" {
		workloadType := pe.getWorkloadType(pod)
		if selectors.WorkloadType != workloadType {
			return false, fmt.Sprintf("workload type %s does not match %s", workloadType, selectors.WorkloadType)
		}
	}

	return true, ""
}

// applyRuleActions applies the actions of a matched rule
func (pe *PolicyEngine) applyRuleActions(rule Rule, pod *corev1.Pod, containerName string, currentUsage map[string]*resource.Quantity, result *PolicyEvaluationResult) {
	actions := rule.Actions

	// Check if we should skip
	if actions.Skip {
		result.Skip = true
		return
	}

	// Get current CPU and memory usage
	var cpuUsage, memoryUsage *resource.Quantity
	if usage, exists := currentUsage["cpu"]; exists {
		cpuUsage = usage
	}
	if usage, exists := currentUsage["memory"]; exists {
		memoryUsage = usage
	}

	// Apply CPU multiplier
	if actions.CPUMultiplier != nil && cpuUsage != nil {
		newCPU := resource.NewMilliQuantity(
			int64(float64(cpuUsage.MilliValue())*(*actions.CPUMultiplier)),
			resource.DecimalSI,
		)
		result.RecommendedCPU = newCPU
	}

	// Apply memory multiplier
	if actions.MemoryMultiplier != nil && memoryUsage != nil {
		newMemory := resource.NewQuantity(
			int64(float64(memoryUsage.Value())*(*actions.MemoryMultiplier)),
			resource.BinarySI,
		)
		result.RecommendedMemory = newMemory
	}

	// Apply fixed CPU request
	if actions.SetCPURequest != nil {
		if cpu, err := resource.ParseQuantity(*actions.SetCPURequest); err == nil {
			result.RecommendedCPU = &cpu
		} else {
			logger.Warn("Invalid CPU request in rule %s: %v", rule.Name, err)
		}
	}

	// Apply fixed memory request
	if actions.SetMemoryRequest != nil {
		if mem, err := resource.ParseQuantity(*actions.SetMemoryRequest); err == nil {
			result.RecommendedMemory = &mem
		} else {
			logger.Warn("Invalid memory request in rule %s: %v", rule.Name, err)
		}
	}

	// Apply CPU limit
	if actions.SetCPULimit != nil {
		if cpu, err := resource.ParseQuantity(*actions.SetCPULimit); err == nil {
			result.CPULimit = &cpu
		} else {
			logger.Warn("Invalid CPU limit in rule %s: %v", rule.Name, err)
		}
	}

	// Apply memory limit
	if actions.SetMemoryLimit != nil {
		if mem, err := resource.ParseQuantity(*actions.SetMemoryLimit); err == nil {
			result.MemoryLimit = &mem
		} else {
			logger.Warn("Invalid memory limit in rule %s: %v", rule.Name, err)
		}
	}

	// Apply min/max constraints
	pe.applyMinMaxConstraints(actions, result)
}

// applyMinMaxConstraints applies minimum and maximum resource constraints
func (pe *PolicyEngine) applyMinMaxConstraints(actions RuleActions, result *PolicyEvaluationResult) {
	// Apply CPU constraints
	if result.RecommendedCPU != nil {
		if actions.MinCPU != nil {
			if minCPU, err := resource.ParseQuantity(*actions.MinCPU); err == nil {
				if result.RecommendedCPU.Cmp(minCPU) < 0 {
					result.RecommendedCPU = &minCPU
				}
			}
		}
		if actions.MaxCPU != nil {
			if maxCPU, err := resource.ParseQuantity(*actions.MaxCPU); err == nil {
				if result.RecommendedCPU.Cmp(maxCPU) > 0 {
					result.RecommendedCPU = &maxCPU
				}
			}
		}
	}

	// Apply memory constraints
	if result.RecommendedMemory != nil {
		if actions.MinMemory != nil {
			if minMem, err := resource.ParseQuantity(*actions.MinMemory); err == nil {
				if result.RecommendedMemory.Cmp(minMem) < 0 {
					result.RecommendedMemory = &minMem
				}
			}
		}
		if actions.MaxMemory != nil {
			if maxMem, err := resource.ParseQuantity(*actions.MaxMemory); err == nil {
				if result.RecommendedMemory.Cmp(maxMem) > 0 {
					result.RecommendedMemory = &maxMem
				}
			}
		}
	}
}

// isRuleScheduleActive checks if a rule is currently active based on its schedule
func (pe *PolicyEngine) isRuleScheduleActive(schedule *RuleSchedule) bool {
	now := time.Now()

	// Load timezone if specified
	if schedule.Timezone != "" {
		if loc, err := time.LoadLocation(schedule.Timezone); err == nil {
			now = now.In(loc)
		} else {
			logger.Warn("Invalid timezone %s, using local time", schedule.Timezone)
		}
	}

	// Check days of week
	if len(schedule.DaysOfWeek) > 0 {
		currentDay := now.Weekday().String()
		dayMatch := false
		for _, day := range schedule.DaysOfWeek {
			if strings.EqualFold(day, currentDay) {
				dayMatch = true
				break
			}
		}
		if !dayMatch {
			return false
		}
	}

	// Check time ranges
	if len(schedule.TimeRanges) > 0 {
		currentTime := now.Format("15:04")
		timeMatch := false
		for _, timeRange := range schedule.TimeRanges {
			if pe.isTimeInRange(currentTime, timeRange.Start, timeRange.End) {
				timeMatch = true
				break
			}
		}
		if !timeMatch {
			return false
		}
	}

	return true
}

// isTimeInRange checks if a time is within a given range
func (pe *PolicyEngine) isTimeInRange(current, start, end string) bool {
	return strings.Compare(current, start) >= 0 && strings.Compare(current, end) <= 0
}

// sortRulesByPriority sorts rules by priority (descending)
func (pe *PolicyEngine) sortRulesByPriority() []Rule {
	rules := make([]Rule, len(pe.rules))
	copy(rules, pe.rules)

	// Simple bubble sort by priority (higher priority first)
	for i := range len(rules) - 1 {
		for j := range len(rules) - i - 1 {
			if rules[j].Priority < rules[j+1].Priority {
				rules[j], rules[j+1] = rules[j+1], rules[j]
			}
		}
	}

	return rules
}

// getWorkloadType determines the workload type of a pod
func (pe *PolicyEngine) getWorkloadType(pod *corev1.Pod) string {
	// Check owner references to determine workload type
	for _, owner := range pod.OwnerReferences {
		switch owner.Kind {
		case "Deployment":
			return "Deployment"
		case "StatefulSet":
			return "StatefulSet"
		case "DaemonSet":
			return "DaemonSet"
		case "Job":
			return "Job"
		case "CronJob":
			return "CronJob"
		case "ReplicaSet":
			return "ReplicaSet"
		}
	}

	// Check for specific annotations or labels
	if pod.Labels != nil {
		if workload, exists := pod.Labels["app.kubernetes.io/component"]; exists {
			return workload
		}
	}

	return "Pod"
}

// getQoSClass determines the QoS class of a pod
func getQoSClass(pod *corev1.Pod) corev1.PodQOSClass {
	requests := make(corev1.ResourceList)
	limits := make(corev1.ResourceList)
	zeroQuantity := resource.MustParse("0")
	isGuaranteed := true

	for _, container := range pod.Spec.Containers {
		// Accumulate requests
		for name, quantity := range container.Resources.Requests {
			if value, exists := requests[name]; !exists {
				requests[name] = quantity.DeepCopy()
			} else {
				value.Add(quantity)
				requests[name] = value
			}
		}

		// Accumulate limits
		for name, quantity := range container.Resources.Limits {
			if value, exists := limits[name]; !exists {
				limits[name] = quantity.DeepCopy()
			} else {
				value.Add(quantity)
				limits[name] = value
			}
		}
	}

	// Check if guaranteed
	if len(requests) == 0 || len(limits) == 0 {
		isGuaranteed = false
	} else {
		for name, req := range requests {
			if limit, exists := limits[name]; !exists || limit.Cmp(req) != 0 {
				isGuaranteed = false
				break
			}
		}
	}

	if isGuaranteed {
		return corev1.PodQOSGuaranteed
	}

	// Check if burstable (has some requests or limits)
	for _, req := range requests {
		if req.Cmp(zeroQuantity) != 0 {
			return corev1.PodQOSBurstable
		}
	}

	for _, limit := range limits {
		if limit.Cmp(zeroQuantity) != 0 {
			return corev1.PodQOSBurstable
		}
	}

	return corev1.PodQOSBestEffort
}

// parseRulesFromConfigMap parses rules from a ConfigMap
func (pe *PolicyEngine) parseRulesFromConfigMap(configMap *corev1.ConfigMap) ([]Rule, error) {
	var rules []Rule

	// For simplicity, expect rules in YAML format in the "rules.yaml" key
	if _, exists := configMap.Data["rules.yaml"]; exists {
		// In a real implementation, you would use a YAML parser here
		// For now, we'll create some example rules
		rules = pe.createDefaultRules()
		logger.Info("Using default rules (YAML parsing not implemented in this example)")
	} else {
		// Create some default rules if no configuration is found
		rules = pe.createDefaultRules()
		logger.Info("No rules configuration found, using default rules")
	}

	return rules, nil
}

// createDefaultRules creates a set of default policy rules
func (pe *PolicyEngine) createDefaultRules() []Rule {
	return []Rule{
		{
			Name:        "high-priority-workloads",
			Description: "Higher resource allocation for high priority workloads",
			Priority:    100,
			Selectors: RuleSelectors{
				Labels: map[string]string{
					"priority": "high",
				},
			},
			Actions: RuleActions{
				CPUMultiplier:    floatPtr(1.5),
				MemoryMultiplier: floatPtr(1.3),
			},
			Enabled: true,
		},
		{
			Name:        "database-workloads",
			Description: "Special handling for database workloads",
			Priority:    90,
			Selectors: RuleSelectors{
				Labels: map[string]string{
					"app.kubernetes.io/component": "database",
				},
			},
			Actions: RuleActions{
				CPUMultiplier:    floatPtr(1.2),
				MemoryMultiplier: floatPtr(2.0),
				MinMemory:        stringPtr("1Gi"),
			},
			Enabled: true,
		},
		{
			Name:        "batch-jobs",
			Description: "Resource limits for batch processing jobs",
			Priority:    50,
			Selectors: RuleSelectors{
				WorkloadType: "Job",
			},
			Actions: RuleActions{
				CPUMultiplier:    floatPtr(1.0),
				MemoryMultiplier: floatPtr(1.1),
				MaxCPU:           stringPtr("2"),
				MaxMemory:        stringPtr("4Gi"),
			},
			Enabled: true,
		},
		{
			Name:        "development-namespaces",
			Description: "Conservative resource allocation for dev environments",
			Priority:    20,
			Selectors: RuleSelectors{
				Namespaces: []string{"development", "dev", "staging"},
			},
			Actions: RuleActions{
				CPUMultiplier:    floatPtr(1.0),
				MemoryMultiplier: floatPtr(1.0),
				MaxCPU:           stringPtr("500m"),
				MaxMemory:        stringPtr("1Gi"),
			},
			Enabled: true,
		},
		{
			Name:        "skip-system-pods",
			Description: "Skip right-sizing for system pods",
			Priority:    200,
			Selectors: RuleSelectors{
				Namespaces: []string{"kube-system", "kube-public"},
			},
			Actions: RuleActions{
				Skip: true,
			},
			Enabled: true,
		},
	}
}

// Helper functions for creating pointers
func floatPtr(f float64) *float64 {
	return &f
}

func stringPtr(s string) *string {
	return &s
}

// GetRules returns all loaded rules
func (pe *PolicyEngine) GetRules() []Rule {
	return pe.rules
}

// GetActiveRules returns only enabled rules
func (pe *PolicyEngine) GetActiveRules() []Rule {
	var activeRules []Rule
	for _, rule := range pe.rules {
		if rule.Enabled {
			activeRules = append(activeRules, rule)
		}
	}
	return activeRules
}

// ValidateRules validates all loaded rules
func (pe *PolicyEngine) ValidateRules() error {
	for _, rule := range pe.rules {
		if err := pe.validateRule(rule); err != nil {
			return fmt.Errorf("rule %s is invalid: %w", rule.Name, err)
		}
	}
	return nil
}

// validateRule validates a single rule
func (pe *PolicyEngine) validateRule(rule Rule) error {
	if rule.Name == "" {
		return errors.New("rule name cannot be empty")
	}

	// Validate regex if present
	if rule.Selectors.PodNameRegex != "" {
		if _, err := regexp.Compile(rule.Selectors.PodNameRegex); err != nil {
			return fmt.Errorf("invalid pod name regex: %w", err)
		}
	}

	// Validate resource quantities in actions
	actions := rule.Actions
	if actions.MinCPU != nil {
		if _, err := resource.ParseQuantity(*actions.MinCPU); err != nil {
			return fmt.Errorf("invalid minCPU: %w", err)
		}
	}
	if actions.MaxCPU != nil {
		if _, err := resource.ParseQuantity(*actions.MaxCPU); err != nil {
			return fmt.Errorf("invalid maxCPU: %w", err)
		}
	}
	if actions.MinMemory != nil {
		if _, err := resource.ParseQuantity(*actions.MinMemory); err != nil {
			return fmt.Errorf("invalid minMemory: %w", err)
		}
	}
	if actions.MaxMemory != nil {
		if _, err := resource.ParseQuantity(*actions.MaxMemory); err != nil {
			return fmt.Errorf("invalid maxMemory: %w", err)
		}
	}

	return nil
}

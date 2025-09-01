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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=rsp
// +kubebuilder:printcolumn:name="Enabled",type=boolean,JSONPath=`.spec.enabled`
// +kubebuilder:printcolumn:name="Mode",type=string,JSONPath=`.spec.mode`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Last Applied",type=date,JSONPath=`.status.lastAppliedTime`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// RightSizerPolicy is the Schema for the rightsizerpolicies API
type RightSizerPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RightSizerPolicySpec   `json:"spec,omitempty"`
	Status RightSizerPolicyStatus `json:"status,omitempty"`
}

// RightSizerPolicySpec defines the desired state of RightSizerPolicy
type RightSizerPolicySpec struct {
	// Enabled indicates if this policy is active
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// Priority determines the order of policy application (higher priority wins)
	// +kubebuilder:default=100
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1000
	Priority int32 `json:"priority,omitempty"`

	// Mode defines the sizing mode for this policy
	// +kubebuilder:validation:Enum=aggressive;balanced;conservative;custom
	// +kubebuilder:default=balanced
	Mode string `json:"mode,omitempty"`

	// DryRun enables dry-run mode for this policy
	// +kubebuilder:default=false
	DryRun bool `json:"dryRun,omitempty"`

	// TargetRef defines which resources this policy applies to
	TargetRef TargetReference `json:"targetRef"`

	// ResourceStrategy defines how resources should be calculated
	ResourceStrategy ResourceStrategy `json:"resourceStrategy,omitempty"`

	// Schedule defines when this policy should be evaluated
	Schedule ScheduleSpec `json:"schedule,omitempty"`

	// Constraints defines resource constraints and limits
	Constraints ResourceConstraints `json:"constraints,omitempty"`

	// Webhooks defines webhook notifications for policy events
	Webhooks []WebhookSpec `json:"webhooks,omitempty"`

	// Annotations to add to resized resources
	ResourceAnnotations map[string]string `json:"resourceAnnotations,omitempty"`
}

// TargetReference defines which resources the policy applies to
type TargetReference struct {
	// Kind of resources to target (Deployment, StatefulSet, DaemonSet, Pod)
	// +kubebuilder:validation:Enum=Deployment;StatefulSet;DaemonSet;Pod;ReplicaSet;Job;CronJob
	Kind string `json:"kind,omitempty"`

	// APIVersion of the target resource
	// +kubebuilder:default="apps/v1"
	APIVersion string `json:"apiVersion,omitempty"`

	// Namespaces to include (empty means all namespaces)
	Namespaces []string `json:"namespaces,omitempty"`

	// ExcludeNamespaces to exclude from this policy
	ExcludeNamespaces []string `json:"excludeNamespaces,omitempty"`

	// LabelSelector for selecting resources
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// AnnotationSelector for selecting resources based on annotations
	AnnotationSelector map[string]string `json:"annotationSelector,omitempty"`

	// Names of specific resources to target
	Names []string `json:"names,omitempty"`

	// ExcludeNames of specific resources to exclude
	ExcludeNames []string `json:"excludeNames,omitempty"`
}

// ResourceStrategy defines how resources should be calculated
type ResourceStrategy struct {
	// CPU request calculation strategy
	CPU CPUStrategy `json:"cpu,omitempty"`

	// Memory calculation strategy
	Memory MemoryStrategy `json:"memory,omitempty"`

	// MetricsSource defines where to get metrics from
	// +kubebuilder:validation:Enum=metrics-server;prometheus;custom
	// +kubebuilder:default=metrics-server
	MetricsSource string `json:"metricsSource,omitempty"`

	// PrometheusConfig for Prometheus metrics source
	PrometheusConfig *PrometheusConfig `json:"prometheusConfig,omitempty"`

	// HistoryWindow defines how much historical data to consider
	// +kubebuilder:default="7d"
	HistoryWindow string `json:"historyWindow,omitempty"`

	// Percentile to use for resource calculations (50, 90, 95, 99)
	// +kubebuilder:default=95
	// +kubebuilder:validation:Enum=50;90;95;99
	Percentile int32 `json:"percentile,omitempty"`

	// UpdateMode defines how updates should be applied
	// +kubebuilder:validation:Enum=immediate;rolling;scheduled
	// +kubebuilder:default=rolling
	UpdateMode string `json:"updateMode,omitempty"`
}

// CPUStrategy defines CPU resource calculation strategy
type CPUStrategy struct {
	// RequestMultiplier for CPU requests
	// +kubebuilder:validation:Minimum=0.1
	// +kubebuilder:validation:Maximum=10
	RequestMultiplier *float64 `json:"requestMultiplier,omitempty"`

	// RequestAddition in millicores to add to CPU requests
	// +kubebuilder:validation:Minimum=0
	RequestAddition *int64 `json:"requestAddition,omitempty"`

	// LimitMultiplier for CPU limits
	// +kubebuilder:validation:Minimum=0.1
	// +kubebuilder:validation:Maximum=10
	LimitMultiplier *float64 `json:"limitMultiplier,omitempty"`

	// LimitAddition in millicores to add to CPU limits
	// +kubebuilder:validation:Minimum=0
	LimitAddition *int64 `json:"limitAddition,omitempty"`

	// MinRequest in millicores
	// +kubebuilder:validation:Minimum=0
	MinRequest *int64 `json:"minRequest,omitempty"`

	// MaxLimit in millicores
	// +kubebuilder:validation:Minimum=0
	MaxLimit *int64 `json:"maxLimit,omitempty"`

	// TargetUtilization percentage (0-100)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	TargetUtilization *int32 `json:"targetUtilization,omitempty"`
}

// MemoryStrategy defines Memory resource calculation strategy
type MemoryStrategy struct {
	// RequestMultiplier for memory requests
	// +kubebuilder:validation:Minimum=0.1
	// +kubebuilder:validation:Maximum=10
	RequestMultiplier *float64 `json:"requestMultiplier,omitempty"`

	// RequestAddition in MB to add to memory requests
	// +kubebuilder:validation:Minimum=0
	RequestAddition *int64 `json:"requestAddition,omitempty"`

	// LimitMultiplier for memory limits
	// +kubebuilder:validation:Minimum=0.1
	// +kubebuilder:validation:Maximum=10
	LimitMultiplier *float64 `json:"limitMultiplier,omitempty"`

	// LimitAddition in MB to add to memory limits
	// +kubebuilder:validation:Minimum=0
	LimitAddition *int64 `json:"limitAddition,omitempty"`

	// MinRequest in MB
	// +kubebuilder:validation:Minimum=0
	MinRequest *int64 `json:"minRequest,omitempty"`

	// MaxLimit in MB
	// +kubebuilder:validation:Minimum=0
	MaxLimit *int64 `json:"maxLimit,omitempty"`

	// TargetUtilization percentage (0-100)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	TargetUtilization *int32 `json:"targetUtilization,omitempty"`
}

// PrometheusConfig defines Prometheus configuration
type PrometheusConfig struct {
	// URL of Prometheus server
	URL string `json:"url"`

	// CPUQuery for fetching CPU metrics
	CPUQuery string `json:"cpuQuery,omitempty"`

	// MemoryQuery for fetching memory metrics
	MemoryQuery string `json:"memoryQuery,omitempty"`

	// Auth configuration for Prometheus
	Auth *PrometheusAuth `json:"auth,omitempty"`
}

// PrometheusAuth defines authentication for Prometheus
type PrometheusAuth struct {
	// BasicAuth configuration
	BasicAuth *BasicAuth `json:"basicAuth,omitempty"`

	// BearerToken for authentication
	BearerToken string `json:"bearerToken,omitempty"`

	// TLSConfig for TLS configuration
	TLSConfig *TLSConfig `json:"tlsConfig,omitempty"`
}

// BasicAuth defines basic authentication
type BasicAuth struct {
	// Username for basic auth
	Username string `json:"username"`

	// Password reference from secret
	PasswordSecretRef corev1.SecretKeySelector `json:"passwordSecretRef"`
}

// TLSConfig defines TLS configuration
type TLSConfig struct {
	// CAFile path or secret reference
	CASecretRef *corev1.SecretKeySelector `json:"caSecretRef,omitempty"`

	// InsecureSkipVerify disables TLS verification
	InsecureSkipVerify bool `json:"insecureSkipVerify,omitempty"`
}

// ScheduleSpec defines when the policy should be evaluated
type ScheduleSpec struct {
	// Interval between evaluations (e.g., "30s", "5m", "1h")
	// +kubebuilder:default="1m"
	Interval string `json:"interval,omitempty"`

	// CronSchedule for cron-based evaluation
	CronSchedule string `json:"cronSchedule,omitempty"`

	// TimeWindows when the policy is active
	TimeWindows []TimeWindow `json:"timeWindows,omitempty"`
}

// TimeWindow defines a time window when the policy is active
type TimeWindow struct {
	// Start time in format "HH:MM"
	Start string `json:"start"`

	// End time in format "HH:MM"
	End string `json:"end"`

	// DaysOfWeek when this window is active
	// +kubebuilder:validation:Enum=Monday;Tuesday;Wednesday;Thursday;Friday;Saturday;Sunday
	DaysOfWeek []string `json:"daysOfWeek,omitempty"`

	// Timezone for the time window
	// +kubebuilder:default="UTC"
	Timezone string `json:"timezone,omitempty"`
}

// ResourceConstraints defines constraints for resource adjustments
type ResourceConstraints struct {
	// MaxChangePercentage limits how much resources can change in one adjustment
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	MaxChangePercentage *int32 `json:"maxChangePercentage,omitempty"`

	// MinChangeThreshold below which changes are not applied (percentage)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	MinChangeThreshold *int32 `json:"minChangeThreshold,omitempty"`

	// CooldownPeriod between adjustments
	// +kubebuilder:default="5m"
	CooldownPeriod string `json:"cooldownPeriod,omitempty"`

	// RespectPDB ensures PodDisruptionBudgets are respected
	// +kubebuilder:default=true
	RespectPDB bool `json:"respectPDB,omitempty"`

	// RespectHPA ensures HorizontalPodAutoscalers are not conflicted
	// +kubebuilder:default=true
	RespectHPA bool `json:"respectHPA,omitempty"`

	// RespectVPA ensures VerticalPodAutoscalers are not conflicted
	// +kubebuilder:default=true
	RespectVPA bool `json:"respectVPA,omitempty"`
}

// WebhookSpec defines webhook notification configuration
type WebhookSpec struct {
	// URL of the webhook endpoint
	URL string `json:"url"`

	// Events to send notifications for
	// +kubebuilder:validation:Enum=resize;error;warning;info
	Events []string `json:"events"`

	// Headers to include in webhook requests
	Headers map[string]string `json:"headers,omitempty"`

	// RetryPolicy for failed webhook calls
	RetryPolicy *WebhookRetryPolicy `json:"retryPolicy,omitempty"`
}

// WebhookRetryPolicy defines retry policy for webhooks
type WebhookRetryPolicy struct {
	// MaxRetries for failed webhook calls
	// +kubebuilder:default=3
	MaxRetries int32 `json:"maxRetries,omitempty"`

	// RetryInterval between attempts
	// +kubebuilder:default="5s"
	RetryInterval string `json:"retryInterval,omitempty"`
}

// RightSizerPolicyStatus defines the observed state of RightSizerPolicy
type RightSizerPolicyStatus struct {
	// Phase of the policy (Pending, Active, Failed, Suspended)
	Phase string `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastAppliedTime when the policy was last applied
	LastAppliedTime *metav1.Time `json:"lastAppliedTime,omitempty"`

	// LastEvaluationTime when the policy was last evaluated
	LastEvaluationTime *metav1.Time `json:"lastEvaluationTime,omitempty"`

	// ResourcesAffected count of resources affected by this policy
	ResourcesAffected int32 `json:"resourcesAffected,omitempty"`

	// ResourcesResized count of resources actually resized
	ResourcesResized int32 `json:"resourcesResized,omitempty"`

	// TotalSavings estimated resource savings
	TotalSavings ResourceSavings `json:"totalSavings,omitempty"`

	// ObservedGeneration for tracking spec changes
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Message provides additional status information
	Message string `json:"message,omitempty"`

	// Metrics provides current metrics summary
	Metrics *MetricsSummary `json:"metrics,omitempty"`
}

// ResourceSavings tracks resource savings
type ResourceSavings struct {
	// CPUSaved in millicores
	CPUSaved int64 `json:"cpuSaved,omitempty"`

	// MemorySaved in MB
	MemorySaved int64 `json:"memorySaved,omitempty"`

	// CostSaved estimated cost savings
	CostSaved string `json:"costSaved,omitempty"`
}

// MetricsSummary provides metrics summary
type MetricsSummary struct {
	// AverageCPUUtilization across affected resources
	AverageCPUUtilization int32 `json:"averageCPUUtilization,omitempty"`

	// AverageMemoryUtilization across affected resources
	AverageMemoryUtilization int32 `json:"averageMemoryUtilization,omitempty"`

	// TotalCPURequests in millicores
	TotalCPURequests int64 `json:"totalCPURequests,omitempty"`

	// TotalMemoryRequests in MB
	TotalMemoryRequests int64 `json:"totalMemoryRequests,omitempty"`

	// LastUpdated timestamp
	LastUpdated *metav1.Time `json:"lastUpdated,omitempty"`
}

// +kubebuilder:object:root=true

// RightSizerPolicyList contains a list of RightSizerPolicy
type RightSizerPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RightSizerPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RightSizerPolicy{}, &RightSizerPolicyList{})
}

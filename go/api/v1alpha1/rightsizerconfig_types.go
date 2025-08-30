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

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,shortName=rsc
// +kubebuilder:printcolumn:name="Enabled",type=boolean,JSONPath=`.spec.enabled`
// +kubebuilder:printcolumn:name="Mode",type=string,JSONPath=`.spec.defaultMode`
// +kubebuilder:printcolumn:name="Interval",type=string,JSONPath=`.spec.resizeInterval`
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// RightSizerConfig is the Schema for the rightsizerconfigs API
// This is a cluster-scoped resource that configures the global behavior of the right-sizer operator
type RightSizerConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RightSizerConfigSpec   `json:"spec,omitempty"`
	Status RightSizerConfigStatus `json:"status,omitempty"`
}

// RightSizerConfigSpec defines the desired state of RightSizerConfig
type RightSizerConfigSpec struct {
	// Enabled indicates if the right-sizer operator is enabled globally
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// DefaultMode sets the default sizing mode when not specified in policies
	// +kubebuilder:validation:Enum=aggressive;balanced;conservative;custom
	// +kubebuilder:default=balanced
	DefaultMode string `json:"defaultMode,omitempty"`

	// ResizeInterval defines how often to check and resize resources globally
	// +kubebuilder:default="1m"
	ResizeInterval string `json:"resizeInterval,omitempty"`

	// DryRun enables global dry-run mode
	// +kubebuilder:default=false
	DryRun bool `json:"dryRun,omitempty"`

	// DefaultResourceStrategy defines default resource calculation strategy
	DefaultResourceStrategy DefaultResourceStrategySpec `json:"defaultResourceStrategy,omitempty"`

	// GlobalConstraints defines global resource constraints
	GlobalConstraints GlobalConstraintsSpec `json:"globalConstraints,omitempty"`

	// MetricsConfig configures metrics collection
	MetricsConfig MetricsConfigSpec `json:"metricsConfig,omitempty"`

	// ObservabilityConfig configures observability features
	ObservabilityConfig ObservabilityConfigSpec `json:"observabilityConfig,omitempty"`

	// SecurityConfig configures security features
	SecurityConfig SecurityConfigSpec `json:"securityConfig,omitempty"`

	// OperatorConfig configures operator behavior
	OperatorConfig OperatorConfigSpec `json:"operatorConfig,omitempty"`

	// NamespaceConfig defines global namespace inclusion/exclusion
	NamespaceConfig NamespaceConfigSpec `json:"namespaceConfig,omitempty"`

	// NotificationConfig configures notifications
	NotificationConfig NotificationConfigSpec `json:"notificationConfig,omitempty"`

	// FeatureGates enables/disables specific features
	FeatureGates map[string]bool `json:"featureGates,omitempty"`
}

// DefaultResourceStrategySpec defines default resource calculation parameters
type DefaultResourceStrategySpec struct {
	// CPU default strategy
	CPU DefaultCPUStrategy `json:"cpu,omitempty"`

	// Memory default strategy
	Memory DefaultMemoryStrategy `json:"memory,omitempty"`

	// HistoryWindow default for how much historical data to consider
	// +kubebuilder:default="7d"
	HistoryWindow string `json:"historyWindow,omitempty"`

	// Percentile default to use for resource calculations
	// +kubebuilder:default=95
	// +kubebuilder:validation:Enum=50;90;95;99
	Percentile int32 `json:"percentile,omitempty"`

	// UpdateMode default for how updates should be applied
	// +kubebuilder:validation:Enum=immediate;rolling;scheduled
	// +kubebuilder:default=rolling
	UpdateMode string `json:"updateMode,omitempty"`
}

// DefaultCPUStrategy defines default CPU resource calculation
type DefaultCPUStrategy struct {
	// RequestMultiplier default for CPU requests
	// +kubebuilder:default=1.2
	// +kubebuilder:validation:Minimum=0.1
	// +kubebuilder:validation:Maximum=10
	RequestMultiplier float64 `json:"requestMultiplier,omitempty"`

	// RequestAddition default in millicores
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	RequestAddition int64 `json:"requestAddition,omitempty"`

	// LimitMultiplier default for CPU limits
	// +kubebuilder:default=2.0
	// +kubebuilder:validation:Minimum=0.1
	// +kubebuilder:validation:Maximum=10
	LimitMultiplier float64 `json:"limitMultiplier,omitempty"`

	// LimitAddition default in millicores
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	LimitAddition int64 `json:"limitAddition,omitempty"`

	// MinRequest default in millicores
	// +kubebuilder:default=10
	// +kubebuilder:validation:Minimum=0
	MinRequest int64 `json:"minRequest,omitempty"`

	// MaxLimit default in millicores
	// +kubebuilder:default=4000
	// +kubebuilder:validation:Minimum=0
	MaxLimit int64 `json:"maxLimit,omitempty"`
}

// DefaultMemoryStrategy defines default Memory resource calculation
type DefaultMemoryStrategy struct {
	// RequestMultiplier default for memory requests
	// +kubebuilder:default=1.2
	// +kubebuilder:validation:Minimum=0.1
	// +kubebuilder:validation:Maximum=10
	RequestMultiplier float64 `json:"requestMultiplier,omitempty"`

	// RequestAddition default in MB
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	RequestAddition int64 `json:"requestAddition,omitempty"`

	// LimitMultiplier default for memory limits
	// +kubebuilder:default=2.0
	// +kubebuilder:validation:Minimum=0.1
	// +kubebuilder:validation:Maximum=10
	LimitMultiplier float64 `json:"limitMultiplier,omitempty"`

	// LimitAddition default in MB
	// +kubebuilder:default=0
	// +kubebuilder:validation:Minimum=0
	LimitAddition int64 `json:"limitAddition,omitempty"`

	// MinRequest default in MB
	// +kubebuilder:default=64
	// +kubebuilder:validation:Minimum=0
	MinRequest int64 `json:"minRequest,omitempty"`

	// MaxLimit default in MB
	// +kubebuilder:default=8192
	// +kubebuilder:validation:Minimum=0
	MaxLimit int64 `json:"maxLimit,omitempty"`
}

// GlobalConstraintsSpec defines global constraints for the operator
type GlobalConstraintsSpec struct {
	// MaxChangePercentage global limit for resource changes
	// +kubebuilder:default=50
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	MaxChangePercentage int32 `json:"maxChangePercentage,omitempty"`

	// MinChangeThreshold global minimum change threshold
	// +kubebuilder:default=5
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	MinChangeThreshold int32 `json:"minChangeThreshold,omitempty"`

	// CooldownPeriod global cooldown between adjustments
	// +kubebuilder:default="5m"
	CooldownPeriod string `json:"cooldownPeriod,omitempty"`

	// MaxConcurrentResizes limits concurrent resize operations
	// +kubebuilder:default=10
	// +kubebuilder:validation:Minimum=1
	MaxConcurrentResizes int32 `json:"maxConcurrentResizes,omitempty"`

	// MaxRestartsPerHour global limit for pod restarts
	// +kubebuilder:default=5
	// +kubebuilder:validation:Minimum=0
	MaxRestartsPerHour int32 `json:"maxRestartsPerHour,omitempty"`

	// RespectPDB globally ensures PodDisruptionBudgets are respected
	// +kubebuilder:default=true
	RespectPDB bool `json:"respectPDB,omitempty"`

	// RespectHPA globally ensures HorizontalPodAutoscalers are not conflicted
	// +kubebuilder:default=true
	RespectHPA bool `json:"respectHPA,omitempty"`

	// RespectVPA globally ensures VerticalPodAutoscalers are not conflicted
	// +kubebuilder:default=true
	RespectVPA bool `json:"respectVPA,omitempty"`
}

// MetricsConfigSpec configures metrics collection
type MetricsConfigSpec struct {
	// Provider defines the metrics provider to use
	// +kubebuilder:validation:Enum=metrics-server;prometheus;custom
	// +kubebuilder:default=metrics-server
	Provider string `json:"provider,omitempty"`

	// PrometheusEndpoint for Prometheus metrics
	PrometheusEndpoint string `json:"prometheusEndpoint,omitempty"`

	// MetricsServerEndpoint for custom metrics server
	MetricsServerEndpoint string `json:"metricsServerEndpoint,omitempty"`

	// ScrapeInterval for metrics collection
	// +kubebuilder:default="30s"
	ScrapeInterval string `json:"scrapeInterval,omitempty"`

	// RetentionPeriod for metrics history
	// +kubebuilder:default="30d"
	RetentionPeriod string `json:"retentionPeriod,omitempty"`

	// CustomQueries for custom metrics
	CustomQueries map[string]string `json:"customQueries,omitempty"`

	// EnableProfiling enables CPU and memory profiling
	// +kubebuilder:default=false
	EnableProfiling bool `json:"enableProfiling,omitempty"`
}

// ObservabilityConfigSpec configures observability features
type ObservabilityConfigSpec struct {
	// LogLevel for the operator
	// +kubebuilder:validation:Enum=debug;info;warn;error
	// +kubebuilder:default=info
	LogLevel string `json:"logLevel,omitempty"`

	// LogFormat for log output
	// +kubebuilder:validation:Enum=json;text
	// +kubebuilder:default=json
	LogFormat string `json:"logFormat,omitempty"`

	// EnableAuditLog enables audit logging
	// +kubebuilder:default=true
	EnableAuditLog bool `json:"enableAuditLog,omitempty"`

	// AuditLogPath for audit log files
	// +kubebuilder:default="/var/log/right-sizer/audit.log"
	AuditLogPath string `json:"auditLogPath,omitempty"`

	// EnableMetricsExport enables Prometheus metrics export
	// +kubebuilder:default=true
	EnableMetricsExport bool `json:"enableMetricsExport,omitempty"`

	// MetricsPort for Prometheus metrics
	// +kubebuilder:default=9090
	MetricsPort int32 `json:"metricsPort,omitempty"`

	// EnableTracing enables distributed tracing
	// +kubebuilder:default=false
	EnableTracing bool `json:"enableTracing,omitempty"`

	// TracingEndpoint for tracing collector
	TracingEndpoint string `json:"tracingEndpoint,omitempty"`

	// EnableEvents enables Kubernetes event generation
	// +kubebuilder:default=true
	EnableEvents bool `json:"enableEvents,omitempty"`
}

// SecurityConfigSpec configures security features
type SecurityConfigSpec struct {
	// EnableAdmissionController enables admission webhook
	// +kubebuilder:default=false
	EnableAdmissionController bool `json:"enableAdmissionController,omitempty"`

	// AdmissionWebhookPort for admission webhook
	// +kubebuilder:default=8443
	AdmissionWebhookPort int32 `json:"admissionWebhookPort,omitempty"`

	// RequireAnnotation requires explicit annotation for resizing
	// +kubebuilder:default=false
	RequireAnnotation bool `json:"requireAnnotation,omitempty"`

	// AnnotationKey to look for when RequireAnnotation is true
	// +kubebuilder:default="right-sizer.io/enabled"
	AnnotationKey string `json:"annotationKey,omitempty"`

	// EnableMutatingWebhook enables mutating admission webhook
	// +kubebuilder:default=false
	EnableMutatingWebhook bool `json:"enableMutatingWebhook,omitempty"`

	// EnableValidatingWebhook enables validating admission webhook
	// +kubebuilder:default=true
	EnableValidatingWebhook bool `json:"enableValidatingWebhook,omitempty"`

	// TLSConfig for webhook TLS configuration
	TLSConfig *WebhookTLSConfig `json:"tlsConfig,omitempty"`
}

// WebhookTLSConfig defines TLS configuration for webhooks
type WebhookTLSConfig struct {
	// CertSecretName containing TLS certificate
	CertSecretName string `json:"certSecretName,omitempty"`

	// CertPath to TLS certificate file
	// +kubebuilder:default="/etc/certs/tls.crt"
	CertPath string `json:"certPath,omitempty"`

	// KeyPath to TLS key file
	// +kubebuilder:default="/etc/certs/tls.key"
	KeyPath string `json:"keyPath,omitempty"`

	// CAPath to CA certificate file
	CAPath string `json:"caPath,omitempty"`

	// AutoGenerate certificates if not provided
	// +kubebuilder:default=true
	AutoGenerate bool `json:"autoGenerate,omitempty"`
}

// OperatorConfigSpec configures operator behavior
type OperatorConfigSpec struct {
	// LeaderElection enables leader election for HA
	// +kubebuilder:default=true
	LeaderElection bool `json:"leaderElection,omitempty"`

	// LeaderElectionNamespace for leader election
	// +kubebuilder:default="right-sizer-system"
	LeaderElectionNamespace string `json:"leaderElectionNamespace,omitempty"`

	// LeaderElectionID for leader election
	// +kubebuilder:default="right-sizer-leader"
	LeaderElectionID string `json:"leaderElectionID,omitempty"`

	// MaxRetries for failed operations
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=0
	MaxRetries int32 `json:"maxRetries,omitempty"`

	// RetryInterval between retry attempts
	// +kubebuilder:default="5s"
	RetryInterval string `json:"retryInterval,omitempty"`

	// EnableCircuitBreaker enables circuit breaker pattern
	// +kubebuilder:default=true
	EnableCircuitBreaker bool `json:"enableCircuitBreaker,omitempty"`

	// CircuitBreakerThreshold for circuit breaker
	// +kubebuilder:default=5
	// +kubebuilder:validation:Minimum=1
	CircuitBreakerThreshold int32 `json:"circuitBreakerThreshold,omitempty"`

	// ReconcileInterval for reconciliation loop
	// +kubebuilder:default="10m"
	ReconcileInterval string `json:"reconcileInterval,omitempty"`

	// WorkerThreads for concurrent processing
	// +kubebuilder:default=5
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=50
	WorkerThreads int32 `json:"workerThreads,omitempty"`
}

// NamespaceConfigSpec defines namespace inclusion/exclusion
type NamespaceConfigSpec struct {
	// IncludeNamespaces to monitor (empty means all)
	IncludeNamespaces []string `json:"includeNamespaces,omitempty"`

	// ExcludeNamespaces to exclude from monitoring
	ExcludeNamespaces []string `json:"excludeNamespaces,omitempty"`

	// SystemNamespaces that should never be modified
	SystemNamespaces []string `json:"systemNamespaces,omitempty"`

	// NamespaceLabels to select namespaces by labels
	NamespaceLabels map[string]string `json:"namespaceLabels,omitempty"`
}

// NotificationConfigSpec configures notifications
type NotificationConfigSpec struct {
	// EnableNotifications globally enables notifications
	// +kubebuilder:default=false
	EnableNotifications bool `json:"enableNotifications,omitempty"`

	// SlackConfig for Slack notifications
	SlackConfig *SlackNotificationConfig `json:"slackConfig,omitempty"`

	// EmailConfig for email notifications
	EmailConfig *EmailNotificationConfig `json:"emailConfig,omitempty"`

	// WebhookConfigs for generic webhook notifications
	WebhookConfigs []WebhookNotificationConfig `json:"webhookConfigs,omitempty"`

	// NotificationLevel minimum level for notifications
	// +kubebuilder:validation:Enum=debug;info;warning;error
	// +kubebuilder:default=warning
	NotificationLevel string `json:"notificationLevel,omitempty"`
}

// SlackNotificationConfig defines Slack notification settings
type SlackNotificationConfig struct {
	// WebhookURL for Slack webhook
	WebhookURL string `json:"webhookURL"`

	// Channel to send notifications to
	Channel string `json:"channel,omitempty"`

	// Username for bot
	// +kubebuilder:default="RightSizer"
	Username string `json:"username,omitempty"`

	// IconEmoji for bot avatar
	// +kubebuilder:default=":robot_face:"
	IconEmoji string `json:"iconEmoji,omitempty"`
}

// EmailNotificationConfig defines email notification settings
type EmailNotificationConfig struct {
	// SMTPServer address
	SMTPServer string `json:"smtpServer"`

	// SMTPPort for SMTP server
	// +kubebuilder:default=587
	SMTPPort int32 `json:"smtpPort,omitempty"`

	// From email address
	From string `json:"from"`

	// To email addresses
	To []string `json:"to"`

	// UseTLS for SMTP connection
	// +kubebuilder:default=true
	UseTLS bool `json:"useTLS,omitempty"`

	// AuthSecretRef for SMTP authentication
	AuthSecretRef *corev1.SecretKeySelector `json:"authSecretRef,omitempty"`
}

// WebhookNotificationConfig defines webhook notification settings
type WebhookNotificationConfig struct {
	// Name of this webhook configuration
	Name string `json:"name"`

	// URL of the webhook endpoint
	URL string `json:"url"`

	// Method HTTP method to use
	// +kubebuilder:validation:Enum=GET;POST;PUT
	// +kubebuilder:default=POST
	Method string `json:"method,omitempty"`

	// Headers to include in requests
	Headers map[string]string `json:"headers,omitempty"`

	// Timeout for webhook requests
	// +kubebuilder:default="30s"
	Timeout string `json:"timeout,omitempty"`

	// RetryCount for failed requests
	// +kubebuilder:default=3
	RetryCount int32 `json:"retryCount,omitempty"`
}

// RightSizerConfigStatus defines the observed state of RightSizerConfig
type RightSizerConfigStatus struct {
	// Phase of the configuration (Pending, Active, Failed)
	Phase string `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// LastAppliedTime when the configuration was last applied
	LastAppliedTime *metav1.Time `json:"lastAppliedTime,omitempty"`

	// ActivePolicies count of active policies
	ActivePolicies int32 `json:"activePolicies,omitempty"`

	// TotalResourcesMonitored being monitored
	TotalResourcesMonitored int32 `json:"totalResourcesMonitored,omitempty"`

	// TotalResourcesResized that have been resized
	TotalResourcesResized int32 `json:"totalResourcesResized,omitempty"`

	// OperatorVersion of the running operator
	OperatorVersion string `json:"operatorVersion,omitempty"`

	// ObservedGeneration for tracking spec changes
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Message provides additional status information
	Message string `json:"message,omitempty"`

	// SystemHealth provides system health status
	SystemHealth *SystemHealthStatus `json:"systemHealth,omitempty"`
}

// SystemHealthStatus provides system health information
type SystemHealthStatus struct {
	// MetricsProviderHealthy indicates metrics provider health
	MetricsProviderHealthy bool `json:"metricsProviderHealthy,omitempty"`

	// WebhookHealthy indicates webhook health
	WebhookHealthy bool `json:"webhookHealthy,omitempty"`

	// LeaderElectionActive indicates if leader election is active
	LeaderElectionActive bool `json:"leaderElectionActive,omitempty"`

	// IsLeader indicates if this instance is the leader
	IsLeader bool `json:"isLeader,omitempty"`

	// LastHealthCheck timestamp
	LastHealthCheck *metav1.Time `json:"lastHealthCheck,omitempty"`

	// Errors current error count
	Errors int32 `json:"errors,omitempty"`

	// Warnings current warning count
	Warnings int32 `json:"warnings,omitempty"`
}

// +kubebuilder:object:root=true

// RightSizerConfigList contains a list of RightSizerConfig
type RightSizerConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RightSizerConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RightSizerConfig{}, &RightSizerConfigList{})
}

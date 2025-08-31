# right-sizer Operator

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

A Kubernetes operator for automatic pod resource right-sizing with comprehensive observability, policy-based rules, and enterprise-grade security features.

## Key Features

### Core Functionality
- **In-Place Pod Resizing (Kubernetes 1.33+)**: Dynamically adjust pod resources without restarts using the resize subresource
- **Multiple Sizing Strategies**: Adaptive, non-disruptive, and in-place resizing strategies
- **Multi-Source Metrics**: Supports both Kubernetes metrics-server and Prometheus for data collection
- **Comprehensive Validation**: Resource validation against node capacity, quotas, and limit ranges

### Policy & Intelligence
- **CRD-Based Configuration**: Policy rules and configuration managed through Kubernetes Custom Resources
- **Priority-Based Policies**: Rule-based resource allocation with complex selectors and priorities
- **Historical Trend Analysis**: Track resource usage patterns over time for intelligent predictions
- **Custom Metrics Support**: Integrate custom metrics beyond CPU and memory for sizing decisions
- **Safety Thresholds**: Configurable limits to prevent excessive resource changes

### Enterprise Security
- **Admission Controllers**: Validate and optionally mutate resource requests before pod creation
- **Comprehensive Audit Logging**: Track all resource changes with detailed audit trails
- **RBAC Integration**: Fine-grained permissions with least-privilege access ([detailed guide](docs/RBAC.md))
- **Network Policies**: Secure network communication with predefined policies

### Observability & Reliability
- **Prometheus Metrics**: Extensive metrics for monitoring operator performance and decisions
- **Circuit Breakers**: Automatic failure handling with exponential backoff and retry logic
- **Health Checks**: Comprehensive health monitoring system with liveness/readiness probes
  - `/healthz` - Liveness probe for Kubernetes health checks
  - `/readyz` - Readiness probe to ensure operator is fully operational
  - `/readyz/detailed` - Detailed component health status for debugging
  - Automatic component monitoring (controller, metrics provider, webhook)
  - Periodic health checks with configurable intervals
- **High Availability**: Multi-replica deployment with pod disruption budgets

---

## Health Monitoring

The operator includes a comprehensive health check system:

- **Liveness Probe** (`/healthz`): Ensures the operator is running and responsive
- **Readiness Probe** (`/readyz`): Validates all critical components are operational
- **Detailed Health** (`/readyz/detailed`): Provides component-level health information
- **Component Monitoring**: Tracks health of controller, metrics provider, and webhook server
- **Automatic Recovery**: Kubernetes automatically restarts unhealthy pods based on probe configuration

For detailed health check documentation, see [Health Checks Guide](docs/HEALTH_CHECKS.md).

To test health endpoints:
```bash
# Run the health check test script
./tests/scripts/test-health.sh

# Or manually check endpoints
kubectl port-forward -n default pod/right-sizer-xxx 8081:8081
curl http://localhost:8081/healthz
curl http://localhost:8081/readyz
```

---

## Quick Start

### Prerequisites

- Kubernetes 1.33+ (required for in-place resize feature)
- Helm 3.0+
- Metrics Server or Prometheus
- kubectl configured for your cluster

### Installation

```bash
# For local development, build the Docker image first
# (Skip this step if using a published image)
docker build -t right-sizer:latest .
# If using Minikube, load the image:
# minikube image load right-sizer:latest

# Install from local Helm chart (CRDs are automatically installed)
helm install right-sizer ./helm \
  --namespace right-sizer \
  --create-namespace

# Future: Install from Helm repository (when published)
# helm repo add right-sizer https://right-sizer.github.io/charts
# helm install right-sizer right-sizer/right-sizer \
#   --namespace right-sizer \
#   --create-namespace
```

> **Note**: CRDs are automatically installed as part of the Helm chart. The `crds.install` value is set to `true` by default.
>
> **Local Development**: If the pod shows `ErrImagePull`, you need to build the Docker image locally first or use a custom image repository.

### Verify Installation

```bash
# Check operator status
kubectl get pods -n right-sizer
kubectl logs -l app=right-sizer -n right-sizer -f

# Check CRDs
kubectl get crds | grep rightsizer

# View default configuration
kubectl get rightsizerconfigs -n right-sizer
kubectl get rightsizerpolicies -n right-sizer
```

---

## Kubernetes 1.33+ In-Place Resize Support

Starting with Kubernetes 1.33, the operator uses the native resize subresource to perform true in-place pod resizing without restarts. This feature allows for:

- **Zero-Downtime Updates**: Resources are adjusted while pods continue running
- **Immediate Application**: Changes take effect without pod recreation
- **Improved Efficiency**: No disruption to running workloads

### Requirements

- Kubernetes 1.33 or later
- InPlacePodVerticalScaling feature gate enabled (enabled by default in 1.33+)
- Pods must not have restart policy constraints for resources

### How It Works

The operator uses the Kubernetes resize subresource API:

```bash
kubectl patch pod <pod-name> --subresource resize --patch '{"spec": {"containers": [{"name": "<container>", "resources": {...}}]}}'
```

The operator automatically detects if your cluster supports this feature and will use it when available, falling back to traditional methods on older clusters.

### Example In-Place Resize

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: default
spec:
  featureGates:
    EnableInPlaceResize: true
  strategies:
    inplace:
      enabled: true
      interval: "30s"
```

---

## CRD-Based Configuration

The right-sizer operator uses Custom Resource Definitions (CRDs) for configuration management:

### RightSizerConfig CRD

The global configuration for the operator:

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: default
spec:
  enabled: true
  defaultMode: balanced
  resizeInterval: 30s
  dryRun: false

  resourceStrategy:
    cpu:
      requestMultiplier: 1.2
      limitMultiplier: 2.0
      minRequest: 10
      maxLimit: 4000
    memory:
      requestMultiplier: 1.2
      limitMultiplier: 2.0
      minRequest: 64
      maxLimit: 8192

  constraints:
    maxChangePercentage: 50
    cooldownPeriod: 5m
    respectPDB: true
    respectHPA: true

  observability:
    logLevel: info
    enableAuditLog: true
    enableMetricsExport: true

  security:
    enableAdmissionController: true
    enableValidatingWebhook: true
    enableMutatingWebhook: false
```

### RightSizerPolicy CRD

Define specific policies for different workloads:

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: production-policy
spec:
  enabled: true
  priority: 100
  mode: conservative

  targetRef:
    kind: Deployment
    namespaces: ["production"]
    labelSelector:
      matchLabels:
        tier: critical

  resourceStrategy:
    cpu:
      requestMultiplier: 1.5
      limitMultiplier: 2.5
      targetUtilization: 60
    memory:
      requestMultiplier: 1.4
      limitMultiplier: 2.0
      targetUtilization: 70

  schedule:
    interval: 15m
    timeWindows:
      - start: "08:00"
        end: "20:00"
        daysOfWeek: ["Monday", "Tuesday", "Wednesday", "Thursday", "Friday"]

  constraints:
    maxChangePercentage: 20
    cooldownPeriod: 30m
    maxRestartsPerHour: 1
```

See [examples/](examples/) for more comprehensive CRD configurations.

---

## Helm Chart Configuration

### Core Values

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of operator replicas | `1` |
| `image.repository` | Operator image repository | `right-sizer` |
| `image.tag` | Operator image tag | `latest` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `resources.requests.cpu` | CPU request for operator | `100m` |
| `resources.requests.memory` | Memory request for operator | `128Mi` |
| `resources.limits.cpu` | CPU limit for operator | `500m` |
| `resources.limits.memory` | Memory limit for operator | `256Mi` |

### Feature Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `createDefaultConfig` | Create default RightSizerConfig CRD instance after CRDs are installed | `true` |
| `createExamplePolicies` | Create example RightSizerPolicy CRD instances | `false` |
| `crds.install` | Automatically install CRDs during Helm installation | `true` |
| `crds.keep` | Retain CRDs when Helm chart is uninstalled (prevents data loss) | `true` |
| `webhook.enabled` | Enable admission webhooks | `true` |
| `webhook.certManager.enabled` | Use cert-manager for webhook certificates | `false` |
| `monitoring.serviceMonitor.enabled` | Create ServiceMonitor for Prometheus | `false` |

> **Note**: The CRDs (`RightSizerConfig` and `RightSizerPolicy`) are automatically installed by the Helm chart when `crds.install` is `true` (default). No separate CRD installation is required.

### Security Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `rbac.create` | Create RBAC resources | `true` |
| `serviceAccount.create` | Create service account | `true` |
| `serviceAccount.name` | Service account name | `right-sizer` |
| `podSecurityContext.runAsNonRoot` | Run as non-root user | `true` |
| `podSecurityContext.runAsUser` | User ID | `65532` |
| `securityContext.readOnlyRootFilesystem` | Read-only root filesystem | `true` |

### Advanced Installation Examples

```bash
# Production deployment with HA and monitoring
helm install right-sizer ./helm \
  --set replicaCount=3 \
  --set monitoring.serviceMonitor.enabled=true \
  --set webhook.certManager.enabled=true \
  --set defaultConfig.observability.logLevel=debug \
  --set defaultConfig.security.enableAdmissionController=true

# Development deployment with example policies
helm install right-sizer ./helm \
  --set createExamplePolicies=true \
  --set defaultConfig.dryRun=true \
  --set defaultConfig.observability.logLevel=debug
```

---

## Admission Webhooks

The operator includes both validating and mutating admission webhooks:

### Validating Webhook
- Validates resource changes against cluster constraints
- Checks quota compliance
- Ensures changes don't exceed safety thresholds
- Validates against node capacity

### Mutating Webhook
- Applies default resource requests/limits
- Corrects invalid resource specifications
- Adds required annotations and labels

### Configuration

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: default
spec:
  security:
    enableAdmissionController: true
    admissionWebhookPort: 8443
    enableValidatingWebhook: true
    enableMutatingWebhook: false
    requireAnnotation: false
    annotationKey: "rightsizer.io/enable"
```

---

## Policy-Based Resource Sizing

The operator supports sophisticated policy-based resource allocation using CRDs:

### Policy Features
- **Priority-Based Rules**: Higher priority rules override lower priority ones
- **Complex Selectors**: Match pods by namespace, labels, annotations, and workload type
- **Flexible Actions**: Set multipliers, fixed values, or min/max constraints
- **Scheduling Support**: Time-based and day-of-week rule activation
- **Skip Conditions**: Exclude specific pods from processing

### Example Policies

#### Aggressive Policy for Development
```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: dev-aggressive
spec:
  priority: 50
  mode: aggressive
  targetRef:
    namespaces: ["development"]
  resourceStrategy:
    cpu:
      requestMultiplier: 1.1
      targetUtilization: 80
    memory:
      requestMultiplier: 1.1
      targetUtilization: 85
```

#### Conservative Policy for Production
```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: prod-conservative
spec:
  priority: 150
  mode: conservative
  targetRef:
    namespaces: ["production"]
    labelSelector:
      matchLabels:
        tier: critical
  resourceStrategy:
    cpu:
      requestMultiplier: 1.5
      targetUtilization: 60
    memory:
      requestMultiplier: 1.4
      targetUtilization: 70
  constraints:
    maxChangePercentage: 20
    maxRestartsPerHour: 1
```

---

## Security & Compliance

### RBAC Integration
Minimal required permissions with fine-grained access control. For comprehensive RBAC documentation, see [docs/RBAC.md](docs/RBAC.md).

#### Quick RBAC Fix
If you encounter permission errors:
```bash
# Apply RBAC fixes automatically
./scripts/rbac/apply-rbac-fix.sh

# Verify all permissions
./scripts/rbac/verify-permissions.sh
```

### Audit Logging
Track all operator decisions and changes:
- Resource change events with before/after values
- Policy application decisions
- Validation results and errors
- Security events from admission controllers

### Network Policies
Example network policy for the operator:
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: right-sizer-network-policy
  namespace: right-sizer
spec:
  podSelector:
    matchLabels:
      app: right-sizer
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector: {}
    ports:
    - protocol: TCP
      port: 8443  # Webhook port
  egress:
  - to:
    - namespaceSelector: {}
    ports:
    - protocol: TCP
      port: 443  # Kubernetes API
```

---

## Observability & Monitoring

### Prometheus Metrics
The operator exposes comprehensive metrics on port 9090:

- `rightsizer_pods_processed_total`: Total pods processed
- `rightsizer_pods_resized_total`: Successful resource adjustments
- `rightsizer_resource_change_percentage`: Distribution of change sizes
- `rightsizer_safety_threshold_violations_total`: Safety violations
- `rightsizer_processing_duration_seconds`: Operation timing
- `rightsizer_admission_requests_total`: Admission webhook requests
- `rightsizer_policy_evaluations_total`: Policy evaluation count

### Grafana Dashboard
Import the provided dashboard for visualization:
```bash
kubectl create configmap rightsizer-dashboard \
  --from-file=dashboards/rightsizer.json \
  -n monitoring
```

### Health Endpoints
- `/healthz`: Liveness probe endpoint
- `/readyz`: Readiness probe endpoint
- `/metrics`: Prometheus metrics endpoint

---

## Operational Modes

### Adaptive Strategy
Continuously adjusts resources based on usage patterns:
```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: default
spec:
  strategies:
    adaptive:
      enabled: true
      learningPeriod: "7d"
      adjustmentInterval: "1h"
```

### Non-Disruptive Strategy
Minimizes pod restarts by batching changes:
```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: default
spec:
  strategies:
    nonDisruptive:
      enabled: true
      batchWindow: "6h"
      maxRestartsPerWindow: 2
```

### In-Place Strategy
Uses Kubernetes 1.33+ resize subresource:
```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: default
spec:
  strategies:
    inplace:
      enabled: true
      interval: "30s"
  featureGates:
    EnableInPlaceResize: true
```

---

## Troubleshooting

### Common Issues

#### 1. Pod Not Being Resized
- Check if namespace is included in configuration
- Verify pod doesn't have skip annotations
- Review policy rules and priorities
- Check operator logs for errors

#### 2. Permission Errors
```bash
# Quick fix for RBAC issues
./scripts/rbac/apply-rbac-fix.sh

# Verify permissions
./scripts/rbac/verify-permissions.sh
```

#### 3. Webhook Certificate Issues
```bash
# Check webhook configuration
kubectl get validatingwebhookconfigurations
kubectl get mutatingwebhookconfigurations

# Verify certificates
kubectl get secrets -n right-sizer | grep tls
```

#### 4. CRD Field Validation Errors ("unknown field" in logs)
If you see "unknown field" warnings in the operator logs:
```bash
# Quick fix using the provided script
./scripts/fix-crd-fields.sh

# Or manually apply the correct CRDs
kubectl apply -f helm/crds/rightsizer.io_rightsizerconfigs.yaml
kubectl apply -f helm/crds/rightsizer.io_rightsizerpolicies.yaml
```

See [CRD Field Troubleshooting Guide](docs/TROUBLESHOOTING_CRD_FIELDS.md) for detailed information.

#### 5. Configuring Log Levels
To reduce log verbosity, set the log level to `warn` or `error`:

```bash
# Quick method using the provided script
./scripts/set-log-level.sh warn

# Set to warn and restart operator immediately
./scripts/set-log-level.sh --restart warn

# Or patch the CRD directly
kubectl patch rightsizerconfig default --type='merge' \
  -p '{"spec":{"observabilityConfig":{"logLevel":"warn"}}}'

# Via Helm values
helm upgrade right-sizer ./helm \
  --set defaultConfig.observability.logLevel=warn
```

Available log levels: `debug`, `info` (default), `warn`, `error`

See [Log Level Configuration Guide](docs/CONFIGURE_LOG_LEVEL.md) for detailed information.

### Diagnostic Commands

```bash
# Check operator status
kubectl get pods -n right-sizer
kubectl logs -l app=right-sizer -n right-sizer --tail=100

# View current configuration
kubectl get rightsizerconfigs -A -o yaml
kubectl get rightsizerpolicies -A -o yaml

# Check metrics
kubectl port-forward -n right-sizer svc/right-sizer 9090:9090
curl http://localhost:9090/metrics | grep rightsizer

# View events
kubectl get events -n right-sizer --sort-by='.lastTimestamp'
```

---

## Local Development

### Prerequisites
- Go 1.24+
- Docker
- Kubernetes 1.33+ (Minikube recommended)
- Make

### Setup Development Environment

```bash
# Start Minikube with Kubernetes 1.33+
minikube start --kubernetes-version=v1.33.0 --memory=4096 --cpus=2

# Build and deploy (if using Helm, CRDs are included)
make build
make docker-build
make deploy

# If deploying without Helm for development, install CRDs manually:
# kubectl apply -f deploy/crds/

# Watch logs
kubectl logs -f deployment/right-sizer-operator -n right-sizer

# Run tests
make test

# Generate code
make generate
```

### Project Structure
```
right-sizer/
├── go/                     # Go source code
│   ├── api/               # CRD API definitions
│   ├── controllers/       # Kubernetes controllers
│   ├── admission/         # Admission webhooks
│   ├── audit/            # Audit logging
│   ├── config/           # Configuration management
│   ├── metrics/          # Metrics collection
│   ├── policy/           # Policy engine
│   ├── retry/            # Retry and circuit breaker
│   └── validation/       # Resource validation
├── helm/                  # Helm chart
├── examples/             # Example configurations
├── scripts/              # Utility scripts
│   └── rbac/            # RBAC management scripts
├── docs/                 # Documentation
└── tests/               # Test files
```

---

## Documentation

- [Configuration Guide](docs/CONFIGURATION.md) - Complete configuration reference
- [CRD Configuration Guide](docs/CONFIGURATION-CRD.md) - CRD-specific configuration
- [Build Guide](docs/BUILD.md) - Build and deployment instructions
- [RBAC Guide](docs/RBAC.md) - Comprehensive RBAC documentation
- [Contributing](docs/CONTRIBUTING.md) - Development and contribution guidelines
- [Examples](examples/) - Sample configurations and policies

---

## Version Compatibility

| Component | Minimum Version | Recommended Version | Notes |
|-----------|----------------|-------------------|--------|
| Kubernetes | 1.33 | 1.33+ | In-place resize requires 1.33+ |
| Helm | 3.0 | 3.12+ | Latest features |
| Metrics Server | 0.5.0 | 0.6.0+ | Or Prometheus |
| cert-manager | 1.5.0 | 1.13.0+ | If using webhook certificates |

---

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](docs/CONTRIBUTING.md) for details on:
- Code of conduct
- Development process
- Submitting pull requests
- Reporting issues

---

## License

This project is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**. See [LICENSE](LICENSE) for details.

---

## Support

- **Documentation**: [https://right-sizer.io/docs](https://right-sizer.io/docs)
- **Issues**: [GitHub Issues](https://github.com/right-sizer/right-sizer/issues)
- **Discussions**: [GitHub Discussions](https://github.com/right-sizer/right-sizer/discussions)
- **Security**: Report security vulnerabilities to security@right-sizer.io

# right-sizer Operator

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

A Kubernetes operator for automatic pod resource right-sizing with comprehensive observability, policy-based rules, and enterprise-grade security features.

## Key Features

### Core Functionality
- **In-Place Pod Resizing (Kubernetes 1.33+)**: Dynamically adjust pod resources without restarts using the new resize subresource
- **Multiple Strategies**: Adaptive, non-disruptive, and in-place resizing strategies
- **Multi-Source Metrics**: Supports both Kubernetes metrics-server and Prometheus for data collection
- **Comprehensive Validation**: Resource validation against node capacity, quotas, and limit ranges

### Policy & Intelligence
- **Policy-Based Sizing**: Rule-based resource allocation with priority, scheduling, and complex selectors
- **Historical Trend Analysis**: Track resource usage patterns over time for intelligent predictions
- **Custom Metrics Support**: Integrate custom metrics beyond CPU and memory for sizing decisions
- **Safety Thresholds**: Configurable limits to prevent excessive resource changes

### Enterprise Security
- **Admission Controllers**: Validate and optionally mutate resource requests before pod creation
- **Comprehensive Audit Logging**: Track all resource changes with detailed audit trails
- **RBAC Integration**: Fine-grained permissions with least-privilege access
- **Network Policies**: Secure network communication with predefined policies

### Observability & Reliability
- **Prometheus Metrics**: Extensive metrics for monitoring operator performance and decisions
- **Circuit Breakers**: Automatic failure handling with exponential backoff and retry logic
- **Health Checks**: Multiple health endpoints for monitoring and alerting
- **High Availability**: Multi-replica deployment with pod disruption budgets

---

## Helm Chart Usage

You can deploy the right-sizer operator using the provided Helm chart.

### Install

```sh
helm install right-sizer ./helm
```

### Upgrade

```sh
helm upgrade right-sizer ./helm
```

### Uninstall

```sh
helm uninstall right-sizer
```

### Configuration

You can override values in `values.yaml` using `--set` or by editing the file.

Example:

```sh
helm install right-sizer ./helm \
  --set image.repository=myrepo/right-sizer \
  --set image.tag=v1.2.3 \
  --set prometheusUrl=http://prometheus.mynamespace.svc:9090
```

### Helm Values

| Name                | Description                       | Default                                  |
|---------------------|-----------------------------------|------------------------------------------|
| `image.repository`  | Docker image repository           | `right-sizer`                            |
| `image.tag`         | Docker image tag                  | `latest`                                 |
| `image.pullPolicy`  | Image pull policy                 | `Always`                                 |
| `prometheusUrl`     | Prometheus endpoint for metrics   | `http://prometheus:9090`                 |
| `namespaceInclude`  | Comma-separated list of namespaces to include (e.g., `"default,kube-system"`) | `default` |
| `namespaceExclude`  | Comma-separated list of namespaces to exclude (e.g., `"test,dev"`) | `kube-system` |
| `resources.requests.cpu`    | Pod CPU request            | `100m`                                   |
| `resources.requests.memory` | Pod memory request         | `128Mi`                                  |
| `resources.limits.cpu`      | Pod CPU limit              | `500m`                                   |
| `resources.limits.memory`   | Pod memory limit           | `512Mi`                                  |

---

#### Namespace Filtering

- `namespaceInclude`: Only pods in these namespaces will be monitored. Example: `"default,kube-system"`
- `namespaceExclude`: Pods in these namespaces will be excluded from monitoring. Example: `"test,dev"`
- If both are set, only namespaces in the include list are monitored, except those in the exclude list.

---

## Kubernetes 1.33+ In-Place Resize Support

Starting with Kubernetes 1.33, the operator uses the native resize subresource to perform true in-place pod resizing without restarts. This feature allows for:

- **Zero-Downtime Updates**: Resources are adjusted while pods continue running
- **Immediate Application**: Changes take effect without pod recreation
- **Improved Efficiency**: No disruption to running workloads

### Requirements

- Kubernetes 1.33 or later
- kubectl 1.33+ for manual operations
- In-place resize feature gate enabled (enabled by default in 1.33+)

### How It Works

The operator uses the Kubernetes resize subresource API:

```bash
kubectl patch pod <pod-name> --subresource resize --patch '{"spec": {"containers": [{"name": "<container>", "resources": {...}}]}}'
```

The operator automatically detects if your cluster supports this feature and will use it when available, falling back to traditional methods on older clusters.

---

## Policy-Based Resource Sizing

The right-sizer operator supports sophisticated policy-based resource allocation using configurable rules:

### Policy Features
- **Priority-Based Rules**: Higher priority rules override lower priority ones
- **Complex Selectors**: Match pods by namespace, labels, annotations, regex patterns, QoS class, and workload type
- **Flexible Actions**: Set multipliers, fixed values, or min/max constraints
- **Scheduling Support**: Time-based and day-of-week rule activation
- **Skip Conditions**: Exclude specific pods from processing

### Example Policy Rule
```yaml
- name: high-priority-production
  description: Enhanced resources for high priority production workloads
  priority: 150
  selectors:
    namespaces: ["production"]
    labels:
      priority: high
      environment: production
  actions:
    cpuMultiplier: 1.8
    memoryMultiplier: 1.6
    minCPU: "100m"
    maxMemory: "16Gi"
```

See [examples/policy-rules-example.yaml](examples/policy-rules-example.yaml) for comprehensive policy configurations.

## Security & Compliance

### Admission Controller
Optional admission webhook for validating and mutating pod resource requests:
- **Validation**: Ensure resource changes comply with cluster policies
- **Mutation**: Automatically apply default resources or corrections
- **Security Events**: Audit all admission decisions

### Comprehensive Audit Logging
Track all operator decisions and changes:
- **Resource Changes**: Before/after values with detailed context
- **Policy Applications**: Which rules were applied and why
- **Validation Results**: Success/failure with error details
- **Security Events**: Admission controller decisions and user actions

### RBAC Integration
Minimal required permissions with fine-grained access control:
```yaml
rules:
- apiGroups: [""]
  resources: ["pods", "pods/resize", "events"]
  verbs: ["get", "list", "watch", "patch", "update", "create"]
```

## Observability & Monitoring

### Prometheus Metrics
Comprehensive metrics for monitoring operator performance:
- `rightsizer_pods_processed_total`: Total pods processed
- `rightsizer_pods_resized_total`: Successful resource adjustments
- `rightsizer_resource_change_percentage`: Distribution of change sizes
- `rightsizer_safety_threshold_violations_total`: Safety violations
- `rightsizer_processing_duration_seconds`: Operation timing

### Circuit Breakers & Retry Logic
Automatic failure handling with configurable parameters:
- **Exponential Backoff**: Intelligent retry timing
- **Circuit Breaker Pattern**: Fail fast during outages
- **Configurable Thresholds**: Customize failure handling

## Agent Mode

The right-sizer operator runs continuously, monitoring pods and adjusting resources based on usage patterns and policy rules. The operator supports multiple operational modes:

### Standard Agent Mode
- Continuous monitoring with configurable intervals
- Automatic resource adjustments based on metrics
- Policy-driven decision making

### High Availability Mode
- Multi-replica deployment with leader election
- Pod disruption budgets for zero-downtime updates
- Health checks and automatic recovery

---

## Configuration

The right-sizer operator can be configured using environment variables to customize resource sizing behavior:

### Resource Multipliers

### Resource Multipliers

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `CPU_REQUEST_MULTIPLIER` | `1.2` | Multiplier applied to CPU usage to calculate CPU requests |
| `MEMORY_REQUEST_MULTIPLIER` | `1.2` | Multiplier applied to memory usage to calculate memory requests |
| `CPU_LIMIT_MULTIPLIER` | `2.0` | Multiplier applied to CPU requests to calculate CPU limits |
| `MEMORY_LIMIT_MULTIPLIER` | `2.0` | Multiplier applied to memory requests to calculate memory limits |

### Resource Boundaries

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `MAX_CPU_LIMIT` | `4000` | Maximum CPU limit in millicores (4000 = 4 cores) |
| `MAX_MEMORY_LIMIT` | `8192` | Maximum memory limit in MB (8192 = 8GB) |
| `MIN_CPU_REQUEST` | `10` | Minimum CPU request in millicores |
| `MIN_MEMORY_REQUEST` | `64` | Minimum memory request in MB |

### Operational Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `METRICS_PROVIDER` | `kubernetes` | Metrics source: `kubernetes` (metrics-server) or `prometheus` |
| `PROMETHEUS_URL` | `http://prometheus:9090` | Prometheus URL (when using Prometheus provider) |
| `ENABLE_INPLACE_RESIZE` | `true` | Enable in-place pod resizing for Kubernetes 1.33+ |
| `DRY_RUN` | `false` | If `true`, only log recommendations without applying changes |
| `RESIZE_INTERVAL` | `30s` | How often to check and resize resources |
| `LOG_LEVEL` | `info` | Logging verbosity: `debug`, `info`, `warn`, `error` |

### Enhanced Features

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `MAX_RETRIES` | `3` | Maximum retry attempts for failed operations |
| `RETRY_INTERVAL` | `5s` | Interval between retry attempts |
| `SAFETY_THRESHOLD` | `0.5` | Maximum allowed resource change percentage (0-1) |
| `POLICY_BASED_SIZING` | `false` | Enable policy-based resource sizing rules |
| `HISTORY_DAYS` | `7` | Number of days to retain historical data |
| `CUSTOM_METRICS` | `""` | Comma-separated list of custom metrics to consider |
| `ADMISSION_CONTROLLER` | `false` | Enable admission controller for validation |

### Observability

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `METRICS_ENABLED` | `true` | Enable Prometheus metrics export |
| `METRICS_PORT` | `9090` | Port for metrics endpoint |
| `AUDIT_ENABLED` | `true` | Enable comprehensive audit logging |

### Namespace Filtering

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `KUBE_NAMESPACE_INCLUDE` | `default` | Only monitor pods in these namespaces (CSV) |
| `KUBE_NAMESPACE_EXCLUDE` | `kube-system` | Exclude pods in these namespaces (CSV) |

For detailed configuration options, see [CONFIGURATION.md](CONFIGURATION.md).

## Quick Start

### Prerequisites

- Go 1.22+
- Docker
- Kubernetes 1.33+ (for in-place resize feature)
- Minikube (for local development)

### Local Development

**Note**: All Go source code is located in the `go/` directory.

#### 1. Setup Environment

```bash
# Start Minikube with Kubernetes 1.33+ for in-place resize support
minikube start --kubernetes-version=v1.33.1

# Optional: Use development helper script
./scripts/dev.sh setup
```

#### 2. Build and Deploy

```bash
# Build the operator binary (builds from go/ directory)
./make build

# Build Docker image in Minikube's registry
./make minikube-build

# Deploy using Helm
./make helm-deploy
```

#### 3. Verify Deployment

```bash
# Check operator status
kubectl get pods -l app=right-sizer
kubectl logs -l app=right-sizer -f

# Test with sample workload
kubectl apply -f examples/sample-deployment.yaml

# Watch resize operations
kubectl get events --sort-by='.lastTimestamp' | grep resize
```

#### 4. Development Workflow

```bash
# Auto-rebuild on changes
./scripts/dev.sh watch

# Run tests
./make test

# Clean rebuild
./make clean && ./make build
```

For detailed build instructions, see [BUILD.md](BUILD.md).

**Project Structure**: Go source code is organized in the `go/` directory, with build scripts operating from the project root.

## Advanced Examples

### Production Deployment with All Features
```bash
# Deploy with policy engine, admission controller, and audit logging
helm install right-sizer ./helm \
  --set policyEngine.enabled=true \
  --set security.admissionWebhook.enabled=true \
  --set observability.auditEnabled=true \
  --set config.safetyThreshold=0.3
```

### Policy-Based Configuration
```bash
# Create policy rules
kubectl apply -f examples/policy-rules-example.yaml

# Enable policy-based sizing
helm upgrade right-sizer ./helm \
  --set config.policyBasedSizing=true \
  --set policyEngine.configMapName=right-sizer-policies
```

### High Availability Setup
```bash
# Deploy with HA configuration
helm install right-sizer ./helm \
  --set replicaCount=3 \
  --set resources.requests.cpu=200m \
  --set resources.requests.memory=256Mi \
  --set circuitBreaker.enabled=true
```

## Troubleshooting

### Common Issues

1. **Pod Not Being Resized**
   - Check if namespace is included in filtering rules
   - Verify pod doesn't have skip annotations (`rightsizer.io/disable: "true"`)
   - Review policy rules that might be skipping the pod

2. **Safety Threshold Violations**
   - Adjust `SAFETY_THRESHOLD` environment variable
   - Review policy rules for conflicting multipliers
   - Check audit logs for detailed violation reasons

3. **Admission Webhook Failures**
   - Verify TLS certificates are valid and not expired
   - Check webhook service is accessible from API server
   - Review admission webhook logs for detailed errors

### Diagnostic Commands

```bash
# Check operator status
kubectl get pods -l app=right-sizer -o wide
kubectl logs -l app=right-sizer --tail=100

# Review metrics
kubectl port-forward svc/right-sizer-operator 9090:9090
curl http://localhost:9090/metrics | grep rightsizer

# Check policy applications
kubectl get events --field-selector reason=PolicyApplied

# Review audit logs (if PVC mounted)
kubectl exec -it deployment/right-sizer-operator -- tail -f /var/log/right-sizer/audit.log
```

## Documentation

- [Configuration Guide](CONFIGURATION.md) - Complete configuration reference
- [Build Guide](BUILD.md) - Build and deployment instructions
- [Contributing](CONTRIBUTING.md) - Development and contribution guidelines
- [Project Structure](PROJECT-STRUCTURE.md) - Code organization and architecture
- [Policy Examples](examples/policy-rules-example.yaml) - Comprehensive policy configurations
- [Advanced Configuration](examples/advanced-configuration.yaml) - Production-ready deployment examples

## License

This project is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**. See [LICENSE](LICENSE) for details.

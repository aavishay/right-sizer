# Right-Sizer Helm Chart

A Helm chart for deploying the Right-Sizer Kubernetes operator, which automatically optimizes pod resource requests and limits based on actual usage patterns.

## Overview

Right-Sizer is a Kubernetes operator that monitors pod resource usage and automatically adjusts CPU and memory requests/limits to optimize cluster resource utilization while maintaining application performance and reliability.

## Prerequisites

- Kubernetes 1.19+
- Helm 3.2.0+
- Metrics Server installed in the cluster (for resource usage data)

## Installation

### Add the Helm Repository

```bash
helm repo add right-sizer https://aavishay.github.io/right-sizer/charts
helm repo update
```

### Install the Chart

```bash
# Install with default values
helm install right-sizer right-sizer/right-sizer

# Install in a specific namespace
helm install right-sizer right-sizer/right-sizer --namespace right-sizer --create-namespace

# Install with custom values
helm install right-sizer right-sizer/right-sizer -f values.yaml
```

### Install from Source

```bash
git clone https://github.com/aavishay/right-sizer.git
cd right-sizer
helm install right-sizer ./helm
```

## Configuration

The following table lists the configurable parameters of the Right-Sizer chart and their default values.

### Global Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `nameOverride` | Override the name of the chart | `""` |
| `fullnameOverride` | Override the full name of the chart | `""` |
| `commonLabels` | Labels to add to all deployed objects | `{}` |
| `commonAnnotations` | Annotations to add to all deployed objects | `{}` |

### Image Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | Right-sizer image repository | `aavishay/right-sizer` |
| `image.tag` | Right-sizer image tag | `0.2.0` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `image.pullSecrets` | Image pull secrets | `[]` |

### Deployment Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas | `1` |
| `resources.limits.cpu` | CPU limit | `500m` |
| `resources.limits.memory` | Memory limit | `512Mi` |
| `resources.requests.cpu` | CPU request | `100m` |
| `resources.requests.memory` | Memory request | `128Mi` |
| `nodeSelector` | Node labels for pod assignment | `{}` |
| `tolerations` | Tolerations for pod assignment | `[]` |
| `affinity` | Affinity settings for pod assignment | `{}` |

### Service Account Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `serviceAccount.create` | Create service account | `true` |
| `serviceAccount.annotations` | Service account annotations | `{}` |
| `serviceAccount.name` | Service account name | `""` |

### RBAC Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `rbac.create` | Create RBAC resources | `true` |

### Right-Sizer Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `rightsizerConfig.enabled` | Enable right-sizing | `true` |
| `rightsizerConfig.defaultMode` | Default sizing mode | `balanced` |
| `rightsizerConfig.resizeInterval` | Resource resize interval | `5m` |
| `rightsizerConfig.dryRun` | Enable dry-run mode | `false` |

#### Resource Strategy Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `rightsizerConfig.defaultResourceStrategy.cpu.requestMultiplier` | CPU request multiplier | `1.2` |
| `rightsizerConfig.defaultResourceStrategy.cpu.limitMultiplier` | CPU limit multiplier | `2.0` |
| `rightsizerConfig.defaultResourceStrategy.cpu.minRequest` | Minimum CPU request | `10m` |
| `rightsizerConfig.defaultResourceStrategy.cpu.maxLimit` | Maximum CPU limit | `4000m` |
| `rightsizerConfig.defaultResourceStrategy.memory.requestMultiplier` | Memory request multiplier | `1.2` |
| `rightsizerConfig.defaultResourceStrategy.memory.limitMultiplier` | Memory limit multiplier | `2.0` |
| `rightsizerConfig.defaultResourceStrategy.memory.minRequest` | Minimum memory request | `64Mi` |
| `rightsizerConfig.defaultResourceStrategy.memory.maxLimit` | Maximum memory limit | `8192Mi` |

#### Global Constraints

| Parameter | Description | Default |
|-----------|-------------|---------|
| `rightsizerConfig.globalConstraints.maxChangePercentage` | Maximum change percentage per resize | `50` |
| `rightsizerConfig.globalConstraints.cooldownPeriod` | Cooldown period between resizes | `5m` |
| `rightsizerConfig.globalConstraints.respectPDB` | Respect Pod Disruption Budgets | `true` |
| `rightsizerConfig.globalConstraints.respectHPA` | Respect Horizontal Pod Autoscalers | `true` |

#### Metrics Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `rightsizerConfig.metricsConfig.provider` | Metrics provider | `metrics-server` |
| `rightsizerConfig.metricsConfig.scrapeInterval` | Metrics scrape interval | `30s` |
| `rightsizerConfig.metricsConfig.historyRetention` | Metrics history retention | `30d` |

#### Observability Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `rightsizerConfig.observabilityConfig.logLevel` | Log level | `info` |
| `rightsizerConfig.observabilityConfig.enableMetricsExport` | Enable metrics export | `true` |
| `rightsizerConfig.observabilityConfig.metricsPort` | Metrics port | `9090` |

#### Namespace Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `rightsizerConfig.namespaceConfig.excludeNamespaces` | Namespaces to exclude | `["kube-system", "kube-public", ...]` |

## Usage Examples

### Basic Installation

```bash
helm install right-sizer right-sizer/right-sizer
```

### Install with Custom Resource Limits

```yaml
# values-custom.yaml
resources:
  limits:
    cpu: 1000m
    memory: 1Gi
  requests:
    cpu: 200m
    memory: 256Mi

rightsizerConfig:
  defaultResourceStrategy:
    cpu:
      requestMultiplier: 1.5
      maxLimit: "8000m"
    memory:
      requestMultiplier: 1.3
      maxLimit: "16384Mi"
```

```bash
helm install right-sizer right-sizer/right-sizer -f values-custom.yaml
```

### Install in Dry-Run Mode

```yaml
# values-dryrun.yaml
rightsizerConfig:
  dryRun: true
  observabilityConfig:
    logLevel: "debug"
```

```bash
helm install right-sizer right-sizer/right-sizer -f values-dryrun.yaml
```

### Install with Namespace Restrictions

```yaml
# values-namespaces.yaml
rightsizerConfig:
  namespaceConfig:
    includeNamespaces:
      - "production"
      - "staging"
    excludeNamespaces:
      - "kube-system"
      - "monitoring"
      - "logging"
```

## Upgrading

### Upgrade the Chart

```bash
helm upgrade right-sizer right-sizer/right-sizer
```

### Upgrade with New Values

```bash
helm upgrade right-sizer right-sizer/right-sizer -f new-values.yaml
```

## Uninstalling

To uninstall/delete the `right-sizer` deployment:

```bash
helm uninstall right-sizer
```

This command removes all Kubernetes components associated with the chart and deletes the release.

## Monitoring and Observability

### Metrics

Right-Sizer exposes Prometheus metrics on port 9090 by default:

- Resource usage metrics
- Resize operation metrics
- Performance and health metrics

### Logs

The operator provides structured JSON logs with configurable log levels:

- `debug` - Detailed debugging information
- `info` - General operational information (default)
- `warn` - Warning messages
- `error` - Error messages

### Health Checks

The operator provides health check endpoints:

- `/healthz` - Liveness probe (port 8081)
- `/readyz` - Readiness probe (port 8081)

## Troubleshooting

### Common Issues

1. **Operator not starting**: Check resource limits and RBAC permissions
2. **No metrics data**: Ensure Metrics Server is installed and accessible
3. **Pods not being resized**: Check namespace inclusion/exclusion settings
4. **Permission errors**: Verify RBAC configuration and service account

### Debug Mode

Enable debug logging:

```yaml
rightsizerConfig:
  observabilityConfig:
    logLevel: "debug"
```

### Dry-Run Mode

Test configuration without making changes:

```yaml
rightsizerConfig:
  dryRun: true
```

## Development

### Building from Source

```bash
git clone https://github.com/aavishay/right-sizer.git
cd right-sizer
helm lint ./helm
helm template right-sizer ./helm
```

### Running Tests

```bash
helm test right-sizer
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes and test
4. Submit a pull request

## License

This project is licensed under the AGPL-3.0 License - see the [LICENSE](https://github.com/aavishay/right-sizer/blob/main/LICENSE) file for details.

## Support

- [GitHub Issues](https://github.com/aavishay/right-sizer/issues)
- [Documentation](https://github.com/aavishay/right-sizer/tree/main/docs)
- [Community Discussions](https://github.com/aavishay/right-sizer/discussions)

## Chart Versioning

This chart follows [Semantic Versioning](https://semver.org/). Version numbers are independent of the Right-Sizer application version.

| Chart Version | App Version | Kubernetes Version | Notes |
|---------------|-------------|--------------------|-------|
| 0.2.0 | 0.2.0 | 1.33+ | ARM64 support, enhanced testing |
| 0.1.19 | 0.1.19 | 1.19+ | Initial release |

## Related Projects

- [Kubernetes Metrics Server](https://github.com/kubernetes-sigs/metrics-server)
- [Vertical Pod Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/vertical-pod-autoscaler)
- [Prometheus](https://prometheus.io/) for metrics collection

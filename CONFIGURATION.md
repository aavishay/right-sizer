# Right-Sizer Configuration Guide

## Overview

The right-sizer operator supports comprehensive configuration through environment variables, allowing you to customize resource sizing behavior, operational settings, security features, policy-based rules, and observability options without rebuilding the application. This guide documents all available configuration options and provides examples for different use cases and deployment scenarios.

## Configuration Changes Summary

### Enhanced Features Added

1. **Enhanced Configuration Management**
   - Centralized configuration with comprehensive validation
   - Environment variable parsing with type safety and bounds checking
   - Default values with intelligent override capability
   - Global configuration singleton with hot-reload support

2. **Advanced Resource Management**
   - Policy-based sizing with complex rule evaluation
   - Safety thresholds to prevent excessive changes
   - Custom metrics integration beyond CPU/memory
   - Historical trend analysis and prediction
   - Multiple sizing strategies with fallback mechanisms

3. **Enterprise Security & Compliance**
   - Admission controller for resource validation and mutation
   - Comprehensive audit logging with configurable retention
   - RBAC integration with minimal privilege principles
   - Security event tracking and alerting

4. **Enhanced Observability**
   - Prometheus metrics with detailed operator insights
   - Circuit breaker patterns for reliability
   - Health check endpoints with detailed status
   - Distributed tracing support
   - Performance monitoring and alerting

5. **Reliability & Operations**
   - Exponential backoff retry mechanisms
   - Circuit breakers for failure handling
   - High availability with leader election
   - Graceful shutdown and resource cleanup
   - Configuration validation and error recovery

## Environment Variables

### Core Resource Calculation

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `CPU_REQUEST_MULTIPLIER` | `1.2` | Multiplier applied to actual CPU usage to calculate CPU requests | `1.5` = 50% buffer |
| `CPU_REQUEST_ADDITION` | `0` | Fixed CPU amount (millicores) to add to usage after multiplier for requests | `100` = add 100m to each request |
| `MEMORY_REQUEST_MULTIPLIER` | `1.2` | Multiplier applied to actual memory usage to calculate memory requests | `1.3` = 30% buffer |
| `MEMORY_REQUEST_ADDITION` | `0` | Fixed memory amount (MB) to add to usage after multiplier for requests | `256` = add 256MB to each request |
| `CPU_LIMIT_MULTIPLIER` | `2.0` | Multiplier applied to CPU requests to calculate CPU limits | `3.0` = 3x burst capacity |
| `CPU_LIMIT_ADDITION` | `0` | Fixed CPU amount (millicores) to add to requests after multiplier for limits | `500` = add 500m to each limit |
| `MEMORY_LIMIT_MULTIPLIER` | `2.0` | Multiplier applied to memory requests to calculate memory limits | `2.5` = 2.5x burst capacity |
| `MEMORY_LIMIT_ADDITION` | `0` | Fixed memory amount (MB) to add to requests after multiplier for limits | `512` = add 512MB to each limit |

### Resource Calculation Formula

The right-sizer calculates resources using both multipliers and additions for fine-grained control:

**For Requests:**
```
CPU Request = (Actual CPU Usage × CPU_REQUEST_MULTIPLIER) + CPU_REQUEST_ADDITION
Memory Request = (Actual Memory Usage × MEMORY_REQUEST_MULTIPLIER) + MEMORY_REQUEST_ADDITION
```

**For Limits:**
```
CPU Limit = (CPU Request × CPU_LIMIT_MULTIPLIER) + CPU_LIMIT_ADDITION
Memory Limit = (Memory Request × MEMORY_LIMIT_MULTIPLIER) + MEMORY_LIMIT_ADDITION
```

**Example Calculation:**
- Actual CPU usage: 100m
- Actual Memory usage: 500MB
- Settings: CPU_REQUEST_MULTIPLIER=1.2, CPU_REQUEST_ADDITION=50, CPU_LIMIT_MULTIPLIER=2.0, CPU_LIMIT_ADDITION=100

Results:
- CPU Request = (100 × 1.2) + 50 = 170m
- CPU Limit = (170 × 2.0) + 100 = 440m

This dual approach allows you to:
- Use multipliers for percentage-based buffers (e.g., 20% overhead)
- Use additions for fixed overhead (e.g., always add 100m CPU for background tasks)
- Combine both for sophisticated sizing strategies

### Namespace Filtering

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `KUBE_NAMESPACE_INCLUDE` | (empty) | Comma-separated list of namespaces to monitor (only these) | `default,prod,staging` |
| `KUBE_NAMESPACE_EXCLUDE` | (empty) | Comma-separated list of namespaces to exclude from monitoring | `kube-system,dev` |

### Resource Boundaries

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `MAX_CPU_LIMIT` | `4000` | Maximum CPU limit in millicores | `8000` = 8 cores max |
| `MAX_MEMORY_LIMIT` | `8192` | Maximum memory limit in MB | `16384` = 16GB max |
| `MIN_CPU_REQUEST` | `10` | Minimum CPU request in millicores | `50` = 50m minimum |
| `MIN_MEMORY_REQUEST` | `64` | Minimum memory request in MB | `128` = 128Mi minimum |

### Operational Settings

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `RESIZE_INTERVAL` | `30s` | How often to check and resize resources | `10s`, `1m`, `5m`, `1h` |
| `LOG_LEVEL` | `info` | Logging verbosity level | `debug`, `info`, `warn`, `error` |
| `METRICS_PROVIDER` | `kubernetes` | Source for metrics collection | `kubernetes`, `prometheus` |
| `PROMETHEUS_URL` | `http://prometheus:9090` | Prometheus endpoint URL | `http://prometheus.monitoring:9090` |
| `ENABLE_INPLACE_RESIZE` | `true` | Enable Kubernetes 1.33+ in-place pod resizing | `true`, `false` |
| `DRY_RUN` | `false` | Only log recommendations without applying changes | `true`, `false` |

### Enhanced Reliability Settings

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `MAX_RETRIES` | `3` | Maximum retry attempts for failed operations | `5`, `10` |
| `RETRY_INTERVAL` | `5s` | Base interval between retry attempts | `3s`, `10s` |
| `SAFETY_THRESHOLD` | `0.5` | Maximum allowed resource change percentage (0-1) | `0.3` = 30% max change |

### Advanced Features

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `POLICY_BASED_SIZING` | `false` | Enable policy-based resource sizing rules | `true`, `false` |
| `HISTORY_DAYS` | `7` | Number of days to retain historical data | `14`, `30` |
| `CUSTOM_METRICS` | `""` | Comma-separated list of custom metrics to consider | `network_rx,disk_io` |
| `ADMISSION_CONTROLLER` | `false` | Enable admission controller for validation | `true`, `false` |

### Observability Configuration

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `METRICS_ENABLED` | `true` | Enable Prometheus metrics export | `true`, `false` |
| `METRICS_PORT` | `9090` | Port for Prometheus metrics endpoint | `8080`, `9090` |
| `AUDIT_ENABLED` | `true` | Enable comprehensive audit logging | `true`, `false` |

### Security Settings

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `ADMISSION_WEBHOOK_PORT` | `8443` | Port for admission webhook server | `8443`, `9443` |
| `ADMISSION_CERT_PATH` | `/etc/certs/tls.crt` | Path to TLS certificate | `/etc/ssl/certs/webhook.crt` |
| `ADMISSION_KEY_PATH` | `/etc/certs/tls.key` | Path to TLS private key | `/etc/ssl/certs/webhook.key` |

## How Resource Calculation Works
## Namespace Filtering

The operator supports filtering which namespaces are monitored using two environment variables:

- `KUBE_NAMESPACE_INCLUDE`: If set, only pods in these namespaces will be monitored. Accepts a comma-separated list (CSV).
- `KUBE_NAMESPACE_EXCLUDE`: If set, pods in these namespaces will be excluded from monitoring. Accepts a comma-separated list (CSV).

**Inclusion takes priority:** If `KUBE_NAMESPACE_INCLUDE` is set, only those namespaces are monitored, regardless of the exclude list.

**Example:**

```yaml
env:
   - name: KUBE_NAMESPACE_INCLUDE
      value: "default,prod"
   - name: KUBE_NAMESPACE_EXCLUDE
      value: "dev,kube-system"
```

This will monitor only pods in `default` and `prod`, and ignore all others.

The operator calculates resources using the following formula:

1. **Requests Calculation**:
   ```
   CPU Request = Actual CPU Usage × CPU_REQUEST_MULTIPLIER
   Memory Request = Actual Memory Usage × MEMORY_REQUEST_MULTIPLIER
   ```

2. **Limits Calculation**:
   ```
   CPU Limit = CPU Request × CPU_LIMIT_MULTIPLIER
   Memory Limit = Memory Request × MEMORY_LIMIT_MULTIPLIER
   ```

3. **Boundaries Applied**:
   - Requests are bounded by minimum values (`MIN_CPU_REQUEST`, `MIN_MEMORY_REQUEST`)
   - Limits are capped at maximum values (`MAX_CPU_LIMIT`, `MAX_MEMORY_LIMIT`)

4. **Operational Behavior**:
   - Checks run every `RESIZE_INTERVAL` duration
   - Logs are filtered based on `LOG_LEVEL` setting

## Deployment Examples

### Using Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: right-sizer
spec:
  template:
    spec:
      containers:
        - name: right-sizer
          image: right-sizer:latest
          env:
            - name: RESIZE_INTERVAL
              value: "1m"
            - name: LOG_LEVEL
              value: "info"
            - name: CPU_REQUEST_MULTIPLIER
              value: "1.5"
            - name: MEMORY_REQUEST_MULTIPLIER
              value: "1.3"
            - name: CPU_LIMIT_MULTIPLIER
              value: "2.5"
            - name: MEMORY_LIMIT_MULTIPLIER
              value: "2.0"
            - name: MAX_CPU_LIMIT
              value: "8000"
            - name: MAX_MEMORY_LIMIT
              value: "16384"
```

### Using Helm

The Helm chart has been updated to support these configurations:

```bash
helm install right-sizer ./helm \
  --set resizeInterval=1m \
  --set logLevel=info \
  --set config.cpuRequestMultiplier=1.5 \
  --set config.cpuRequestAddition=100 \
  --set config.memoryRequestMultiplier=1.3 \
  --set config.memoryRequestAddition=256 \
  --set config.cpuLimitMultiplier=2.5 \
  --set config.cpuLimitAddition=200 \
  --set config.memoryLimitMultiplier=2.0 \
  --set config.memoryLimitAddition=512 \
  --set config.maxCpuLimit=8000 \
  --set config.maxMemoryLimit=16384
```

Or using a values file:

```yaml
# values-custom.yaml
resizeInterval: 1m
logLevel: info
config:
  cpuRequestMultiplier: 1.5
  cpuRequestAddition: 100      # Add 100m base CPU
  memoryRequestMultiplier: 1.3
  memoryRequestAddition: 256    # Add 256MB base memory
  cpuLimitMultiplier: 2.5
  cpuLimitAddition: 200         # Add 200m to limits
  memoryLimitMultiplier: 2.0
  memoryLimitAddition: 512      # Add 512MB to limits
  maxCpuLimit: 8000
  maxMemoryLimit: 16384
  minCpuRequest: 50
  minMemoryRequest: 256
```

```bash
helm install right-sizer ./helm -f values-custom.yaml
```

## Configuration Scenarios

### Conservative Production Configuration
High stability with generous resource allocation and comprehensive monitoring:

```yaml
# Resource multipliers - generous for stability
CPU_REQUEST_MULTIPLIER: "1.5"
CPU_REQUEST_ADDITION: "100"      # Base 100m for system overhead
MEMORY_REQUEST_MULTIPLIER: "1.4"
MEMORY_REQUEST_ADDITION: "256"    # Base 256MB for runtime
CPU_LIMIT_MULTIPLIER: "2.5"
CPU_LIMIT_ADDITION: "200"        # Extra headroom for spikes
MEMORY_LIMIT_MULTIPLIER: "2.0"
MEMORY_LIMIT_ADDITION: "512"     # Extra memory buffer

# Boundaries - higher minimums, reasonable maximums
MIN_CPU_REQUEST: "50"
MIN_MEMORY_REQUEST: "256"
MAX_CPU_LIMIT: "8000"
MAX_MEMORY_LIMIT: "16384"

# Safety and reliability
SAFETY_THRESHOLD: "0.3"  # Only 30% change allowed
MAX_RETRIES: "5"
RETRY_INTERVAL: "5s"
RESIZE_INTERVAL: "5m"    # Conservative interval

# Enhanced features
POLICY_BASED_SIZING: "true"
AUDIT_ENABLED: "true"
METRICS_ENABLED: "true"

# Security
ADMISSION_CONTROLLER: "true"
LOG_LEVEL: "info"
```

### Aggressive Cost Optimization
Minimal resource allocation for development/testing environments:

```yaml
# Resource multipliers - minimal overhead
CPU_REQUEST_MULTIPLIER: "1.1"
CPU_REQUEST_ADDITION: "0"        # No additional CPU
MEMORY_REQUEST_MULTIPLIER: "1.1"
MEMORY_REQUEST_ADDITION: "0"      # No additional memory
CPU_LIMIT_MULTIPLIER: "1.5"
CPU_LIMIT_ADDITION: "0"          # Keep costs minimal
MEMORY_LIMIT_MULTIPLIER: "1.5"
MEMORY_LIMIT_ADDITION: "0"       # Keep costs minimal

# Boundaries - lower caps for cost control
MAX_CPU_LIMIT: "2000"    # 2 cores max
MAX_MEMORY_LIMIT: "4096" # 4GB max
MIN_CPU_REQUEST: "10"
MIN_MEMORY_REQUEST: "32"

# Faster iteration for development
RESIZE_INTERVAL: "30s"
SAFETY_THRESHOLD: "0.6"  # Allow larger changes

# Minimal monitoring overhead
METRICS_ENABLED: "false"
AUDIT_ENABLED: "false"
LOG_LEVEL: "warn"
DRY_RUN: "true"  # Safe for experimentation
```

### High Performance Enterprise
Maximum performance with comprehensive observability:

```yaml
# Resource multipliers - performance focused
CPU_REQUEST_MULTIPLIER: "1.8"
MEMORY_REQUEST_MULTIPLIER: "1.5"
CPU_LIMIT_MULTIPLIER: "3.0"
MEMORY_LIMIT_MULTIPLIER: "2.5"

# Boundaries - high capacity for performance
MIN_CPU_REQUEST: "100"
MIN_MEMORY_REQUEST: "512"
MAX_CPU_LIMIT: "32000"   # 32 cores max
MAX_MEMORY_LIMIT: "65536" # 64GB max

# Fast response with safety
RESIZE_INTERVAL: "30s"
SAFETY_THRESHOLD: "0.4"
MAX_RETRIES: "3"
RETRY_INTERVAL: "3s"

# Full enterprise features
POLICY_BASED_SIZING: "true"
ADMISSION_CONTROLLER: "true"
AUDIT_ENABLED: "true"
METRICS_ENABLED: "true"
CUSTOM_METRICS: "network_rx_bytes,network_tx_bytes,disk_io"
HISTORY_DAYS: "30"

# Comprehensive monitoring
LOG_LEVEL: "info"
METRICS_PROVIDER: "prometheus"
PROMETHEUS_URL: "http://prometheus.monitoring:9090"
```

### Policy-Based Configuration Example
Advanced configuration with sophisticated policy rules:

```yaml
# Enable policy engine
POLICY_BASED_SIZING: "true"
HISTORY_DAYS: "14"

# Base multipliers - policies will override these
CPU_REQUEST_MULTIPLIER: "1.2"
MEMORY_REQUEST_MULTIPLIER: "1.2"

# Safety settings
SAFETY_THRESHOLD: "0.35"
MAX_RETRIES: "5"

# Full observability
AUDIT_ENABLED: "true"
METRICS_ENABLED: "true"
ADMISSION_CONTROLLER: "true"

# Custom metrics for advanced decisions
CUSTOM_METRICS: "jvm_memory_used,redis_memory_usage,db_connections"

# Namespace filtering for policy application
KUBE_NAMESPACE_INCLUDE: "production,staging,critical"
KUBE_NAMESPACE_EXCLUDE: "kube-system,monitoring,logging"
```

## Log Levels

The operator supports four log levels for controlling output verbosity:

### Debug
- Most verbose - shows all logs including detailed debugging information
- Shows metrics calculations, pod evaluations, and internal operations
- Useful for troubleshooting and development
- Example: `LOG_LEVEL=debug`

### Info (Default)
- Standard verbosity - shows informational messages
- Includes configuration loading, resource adjustments, and general operations
- Recommended for normal operation
- Example: `LOG_LEVEL=info`

### Warn
- Reduced verbosity - shows warnings and errors only
- Includes issues that don't prevent operation but should be noted
- Good for production environments where less logging is desired
- Example: `LOG_LEVEL=warn`

### Error
- Minimal verbosity - shows errors only
- Only critical issues that prevent proper operation
- Suitable for quiet production environments
- Example: `LOG_LEVEL=error`

## Resize Interval Examples

The `RESIZE_INTERVAL` accepts various time duration formats:

```yaml
RESIZE_INTERVAL: "10s"   # Every 10 seconds (fast - for testing)
RESIZE_INTERVAL: "30s"   # Every 30 seconds (default)
RESIZE_INTERVAL: "1m"    # Every minute
RESIZE_INTERVAL: "5m"    # Every 5 minutes (recommended for production)
RESIZE_INTERVAL: "1h"    # Every hour (slow - for stable workloads)
```

## Files Modified

1. **Created**:
   - `config/config.go` - Configuration management package with RESIZE_INTERVAL and LOG_LEVEL
   - `logger/logger.go` - Logger package with configurable log levels
   - `examples/config-scenarios.yaml` - Example configurations
   - `test-config.sh` - Configuration testing script

2. **Modified**:
   - `main.go` - Added configuration loading, logger initialization, and log level support
   - `controllers/adaptive_rightsizer.go` - Use config for calculations, RESIZE_INTERVAL, and logger
   - `controllers/deployment_rightsizer.go` - Use config for calculations, RESIZE_INTERVAL, and logger
   - `controllers/inplace_rightsizer.go` - Use config for calculations, RESIZE_INTERVAL, and logger
   - `controllers/nondisruptive_rightsizer.go` - Use config for calculations, RESIZE_INTERVAL, and logger
   - `controllers/rightsizer_controller.go` - Use config for calculations and logger
   - `deployment.yaml` - Added environment variables
   - `helm/values.yaml` - Added configuration section
   - `helm/templates/deployment.yaml` - Added env vars
   - `helm/templates/_helpers.tpl` - Added helper templates
   - `README.md` - Added configuration documentation

## Testing the Configuration

1. **Build the operator**:
   ```bash
   go build -o right-sizer main.go
   ```

2. **Test configuration loading**:
   ```bash
   export CPU_REQUEST_MULTIPLIER=1.5
   export MEMORY_REQUEST_MULTIPLIER=1.3
   ./right-sizer
   ```

3. **Verify in logs**:
   Look for configuration output at startup:
   ```
   📋 Configuration Loaded:
      CPU Request Multiplier: 1.50
      Memory Request Multiplier: 1.30
      CPU Limit Multiplier: 2.00
      Memory Limit Multiplier: 2.00
      Resize Interval: 1m
      Log Level: info
   ```

## Policy-Based Resource Sizing

The operator supports sophisticated policy-based resource allocation with configurable rules that can be applied based on various pod characteristics.

### Policy Configuration

Policies are defined in ConfigMaps and loaded by the operator. Each policy rule includes:

- **Selectors**: Criteria for matching pods (namespace, labels, annotations, regex patterns)
- **Actions**: Resource modifications to apply (multipliers, fixed values, constraints)
- **Priority**: Evaluation order (higher priority rules override lower ones)
- **Schedule**: Time-based activation (business hours, weekends, etc.)

### Example Policy ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: right-sizer-policies
  namespace: right-sizer-system
data:
  rules.yaml: |
    - name: high-priority-production
      priority: 150
      enabled: true
      selectors:
        namespaces: ["production"]
        labels:
          priority: high
      actions:
        cpuMultiplier: 1.8
        memoryMultiplier: 1.6
        minMemory: "1Gi"
```

See [examples/policy-rules-example.yaml](examples/policy-rules-example.yaml) for comprehensive examples.

## Security & Compliance Configuration

### Admission Controller Setup

```yaml
# Enable admission webhook
ADMISSION_CONTROLLER: "true"
ADMISSION_WEBHOOK_PORT: "8443"
ADMISSION_CERT_PATH: "/etc/certs/tls.crt"
ADMISSION_KEY_PATH: "/etc/certs/tls.key"

# Validation settings
SAFETY_THRESHOLD: "0.3"  # Strict change limits
AUDIT_ENABLED: "true"    # Track all decisions
```

### Audit Logging Configuration

```yaml
# Enable comprehensive audit logging
AUDIT_ENABLED: "true"

# Audit log retention (via Helm values)
observability:
  auditConfig:
    retentionDays: 90
    maxFileSize: 104857600  # 100MB
    enableFileLog: true
    enableEventLog: true
```

## Observability Configuration

### Metrics and Monitoring

```yaml
# Enable Prometheus metrics
METRICS_ENABLED: "true"
METRICS_PORT: "9090"

# Comprehensive monitoring
HISTORY_DAYS: "30"
CUSTOM_METRICS: "network_utilization,disk_io,jvm_memory"
```

### Health Checks and Circuit Breakers

```yaml
# Retry and circuit breaker settings
MAX_RETRIES: "5"
RETRY_INTERVAL: "3s"
SAFETY_THRESHOLD: "0.3"

# Health check endpoints are enabled by default
# Available at :8081/healthz and :8081/readyz
```

## Best Practices

### Configuration Management
1. **Start Conservative**: Begin with higher multipliers and safety thresholds
2. **Environment-Specific**: Use different configurations for dev/staging/production
3. **Policy-Driven**: Leverage policy engine for complex scenarios
4. **Version Control**: Store configurations in Git with proper versioning
5. **Validate Changes**: Use dry-run mode to test configuration changes

### Security & Compliance
6. **Enable Audit Logging**: Track all resource changes for compliance
7. **Use Admission Controllers**: Validate changes before they're applied
8. **Implement RBAC**: Follow least-privilege principles
9. **Monitor Security Events**: Set up alerting for security-related events
10. **Certificate Management**: Use cert-manager for webhook certificates

### Observability & Monitoring
11. **Monitor Metrics**: Set up Prometheus scraping and Grafana dashboards
12. **Configure Alerts**: Alert on safety threshold violations and failures
13. **Log Analysis**: Use structured logging for better troubleshooting
14. **Performance Tracking**: Monitor processing duration and circuit breaker state
15. **Historical Analysis**: Retain enough historical data for trend analysis

### Operational Excellence
16. **Graceful Degradation**: Configure circuit breakers for reliability
17. **Resource Boundaries**: Set appropriate min/max limits for your cluster
18. **Namespace Filtering**: Be explicit about which namespaces to monitor
19. **Safety Thresholds**: Prevent excessive resource changes
20. **Documentation**: Maintain runbooks and troubleshooting guides

## Troubleshooting

### Configuration Issues

**Configuration Not Applied**
- Check operator logs for configuration loading messages
- Verify environment variables are set correctly in deployment
- Ensure operator pod has been restarted after configuration changes
- Validate ConfigMap changes are properly mounted

**Invalid Values**
- The operator logs warnings for invalid configuration values
- Falls back to defaults if parsing fails
- Check logs for validation errors: `Configuration validation failed`
- Use `kubectl logs` to see detailed error messages

**Policy Rules Not Working**
- Verify policy ConfigMap is in the correct namespace
- Check policy rule syntax and selectors
- Review policy evaluation logs with `LOG_LEVEL=debug`
- Ensure `POLICY_BASED_SIZING=true` is set

### Performance Issues

**High Resource Usage**
- Check metrics collection frequency and retention
- Verify circuit breaker status and retry patterns
- Monitor admission webhook response times
- Review audit log volume and rotation

**Slow Pod Processing**
- Increase `RESIZE_INTERVAL` for less frequent checks
- Optimize policy rules to reduce evaluation overhead
- Check metrics provider response times
- Monitor Kubernetes API rate limiting

### Security and Admission Controller

**Admission Webhook Failures**
- Verify TLS certificates are valid and not expired
- Check webhook service is accessible from API server
- Review admission webhook logs: `kubectl logs -l app=right-sizer`
- Validate webhook configuration and selectors

**Certificate Issues**
- Ensure cert-manager is properly configured
- Check certificate expiration dates
- Verify CA bundle is correctly configured in webhook
- Test certificate chain validation

**RBAC Permissions**
- Verify service account has required permissions
- Check for missing RBAC rules in cluster roles
- Review admission controller permissions
- Validate metrics collection permissions

### Operational Issues

**Circuit Breaker Triggered**
- Check circuit breaker state in metrics
- Review error patterns that triggered the breaker
- Adjust failure threshold or recovery timeout
- Monitor retry attempt metrics

**Audit Log Problems**
- Check audit log file permissions and disk space
- Verify log rotation is working properly
- Review audit event volume and retention policy
- Check Kubernetes event creation permissions

**Metrics Collection Failures**
- Verify metrics-server or Prometheus accessibility
- Check custom metrics endpoint availability
- Review metrics provider configuration
- Monitor metrics collection duration

### Diagnostic Commands

```bash
# Check operator status and configuration
kubectl get pods -l app=right-sizer -o wide
kubectl logs -l app=right-sizer --tail=100 -f

# Review configuration
kubectl describe configmap right-sizer-config
kubectl get secret right-sizer-admission-certs

# Check metrics and health
kubectl port-forward svc/right-sizer-operator 9090:9090
curl http://localhost:9090/metrics | grep rightsizer
curl http://localhost:8081/healthz

# Review policy applications and events
kubectl get events --field-selector reason=PolicyApplied
kubectl get events --field-selector reason=ResourceValidation

# Check admission webhook
kubectl describe validatingadmissionwebhook right-sizer-resource-validator
kubectl get endpoints right-sizer-admission-webhook

# Review audit logs (if mounted)
kubectl exec deployment/right-sizer-operator -- tail -f /var/log/right-sizer/audit.log

# Circuit breaker and retry status
kubectl port-forward svc/right-sizer-operator 9090:9090
curl http://localhost:9090/metrics | grep -E "(retry|circuit)"
```

## Migration Guide

If you're upgrading from a version without configurable multipliers:

1. The default values match the previous hardcoded values:
   - CPU/Memory request multiplier: 1.2 (20% buffer)
   - CPU/Memory limit multiplier: 2.0 (2x burst)
   - Max CPU: 4000 millicores
   - Max Memory: 8192 MB
   - Resize Interval: 30 seconds
   - Log Level: info

2. No configuration changes are required - the operator will work with defaults

3. To customize, add environment variables to your deployment

4. For Helm users, update your values file with the new `config` section
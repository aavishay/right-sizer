# Right-Sizer Configuration Guide

## Overview

The right-sizer operator now supports comprehensive configuration through environment variables, allowing you to customize resource sizing behavior and operational settings without rebuilding the application. This guide documents all available configuration options and provides examples for different use cases.

## Configuration Changes Summary

### New Features Added

1. **Config Package** (`config/config.go`)
   - Centralized configuration management
   - Environment variable parsing with validation
   - Default values with override capability
   - Global configuration singleton

2. **Environment Variables Support**
   - All multipliers are now configurable
   - Resource limits and minimums are configurable
   - Resize interval is configurable (default 30s)
   - Log levels for controlling verbosity
   - Configuration is loaded at startup and logged

3. **Updated Controllers**
   - All controllers now use the centralized configuration
   - Removed hardcoded values for multipliers and limits
   - Consistent configuration across all sizing strategies

## Environment Variables

### Resource Calculation Multipliers

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `CPU_REQUEST_MULTIPLIER` | `1.2` | Multiplier applied to actual CPU usage to calculate CPU requests | `1.5` = 50% buffer |
| `MEMORY_REQUEST_MULTIPLIER` | `1.2` | Multiplier applied to actual memory usage to calculate memory requests | `1.3` = 30% buffer |
| `CPU_LIMIT_MULTIPLIER` | `2.0` | Multiplier applied to CPU requests to calculate CPU limits | `3.0` = 3x burst capacity |
| `MEMORY_LIMIT_MULTIPLIER` | `2.0` | Multiplier applied to memory requests to calculate memory limits | `2.5` = 2.5x burst capacity |
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
  --set config.memoryRequestMultiplier=1.3 \
  --set config.cpuLimitMultiplier=2.5 \
  --set config.memoryLimitMultiplier=2.0 \
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
  memoryRequestMultiplier: 1.3
  cpuLimitMultiplier: 2.5
  memoryLimitMultiplier: 2.0
  maxCpuLimit: 8000
  maxMemoryLimit: 16384
  minCpuRequest: 50
  minMemoryRequest: 256
```

```bash
helm install right-sizer ./helm -f values-custom.yaml
```

## Configuration Scenarios

### Conservative (Production)
- Higher multipliers for stability
- More headroom for unexpected spikes
- Higher minimum values

```yaml
CPU_REQUEST_MULTIPLIER: "1.5"
MEMORY_REQUEST_MULTIPLIER: "1.4"
CPU_LIMIT_MULTIPLIER: "2.5"
MEMORY_LIMIT_MULTIPLIER: "2.0"
MIN_CPU_REQUEST: "50"
MIN_MEMORY_REQUEST: "256"
```

### Aggressive (Cost Optimization)
- Lower multipliers to reduce resource allocation
- Suitable for development/staging
- Lower maximum caps

```yaml
CPU_REQUEST_MULTIPLIER: "1.1"
MEMORY_REQUEST_MULTIPLIER: "1.1"
CPU_LIMIT_MULTIPLIER: "1.5"
MEMORY_LIMIT_MULTIPLIER: "1.5"
MAX_CPU_LIMIT: "2000"
MAX_MEMORY_LIMIT: "4096"
```

### High Performance
- Higher multipliers for consistent performance
- Large burst capacity
- Higher resource caps

```yaml
CPU_REQUEST_MULTIPLIER: "1.8"
MEMORY_REQUEST_MULTIPLIER: "1.5"
CPU_LIMIT_MULTIPLIER: "3.0"
MEMORY_LIMIT_MULTIPLIER: "2.5"
MAX_CPU_LIMIT: "16000"
MAX_MEMORY_LIMIT: "32768"
RESIZE_INTERVAL: "30s"
LOG_LEVEL: "info"
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

## Best Practices

1. **Start Conservative**: Begin with higher multipliers and gradually reduce based on observed behavior
2. **Monitor Metrics**: Watch actual usage vs. allocated resources to fine-tune multipliers
3. **Environment-Specific**: Use different configurations for dev/staging/production
4. **Document Changes**: Keep track of configuration changes and their impact
5. **Test Thoroughly**: Test configuration changes in non-production environments first
6. **Tune Resize Interval**: Use shorter intervals (30s-1m) for dynamic workloads, longer (5m-1h) for stable ones
7. **Adjust Log Level**: Use `debug` for troubleshooting, `info` for normal operation, `error` for production

## Troubleshooting

### Configuration Not Applied
- Check operator logs for configuration loading messages
- Verify environment variables are set correctly
- Ensure operator pod has been restarted after configuration changes

### Invalid Values
- The operator logs warnings for invalid configuration values
- Falls back to defaults if parsing fails
- Check logs for lines like: `Warning: Invalid CPU_REQUEST_MULTIPLIER value`

### Resize Interval Too Short
- Very short intervals (< 10s) may cause excessive API calls
- Recommended minimum: 30s for production
- For testing: 10s is acceptable

### Log Level Not Working
- Ensure LOG_LEVEL is set to valid values: `debug`, `info`, `warn`, `error`
- Check that the logger is initialized before use
- Debug logs will only appear when LOG_LEVEL=debug

### Unexpected Resource Allocations
- Verify actual usage metrics are being collected correctly
- Check that multipliers are appropriate for your workload patterns
- Review minimum and maximum boundaries

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
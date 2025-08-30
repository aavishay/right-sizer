# Configuring Log Levels in Right Sizer

## Overview

The Right Sizer operator supports multiple log levels to control the verbosity of output. You can configure the log level to reduce noise in production or increase verbosity for debugging.

## Available Log Levels

The following log levels are supported (from most to least verbose):

- `debug` - Detailed debugging information
- `info` - General informational messages (default)
- `warn` - Warning messages and errors
- `error` - Only error messages

## Methods to Configure Log Level

### Method 1: Via RightSizerConfig CRD (Runtime Configuration)

The log level can be changed at runtime by updating the `RightSizerConfig` resource:

```bash
# Patch existing config to set log level to warn
kubectl patch rightsizerconfig default --type='merge' -p '{"spec":{"observabilityConfig":{"logLevel":"warn"}}}'
```

Or edit the full config:

```yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: default
spec:
  observabilityConfig:
    logLevel: "warn"  # Options: debug, info, warn, error
    logFormat: "json" # Options: json, text
    # ... other observability settings
```

Apply the configuration:

```bash
kubectl apply -f rightsizerconfig.yaml
```

**Note:** After changing the log level via CRD, the operator will apply the new level on the next reconciliation loop (typically within 30 seconds).

### Method 2: Via Helm Values (Deployment Time)

When deploying with Helm, you can set the log level in your values file:

```yaml
# custom-values.yaml
defaultConfig:
  observability:
    logLevel: "warn"
    logFormat: "json"
```

Deploy or upgrade with the custom values:

```bash
# Initial installation
helm install right-sizer ./helm -f custom-values.yaml \
  --namespace right-sizer-system \
  --create-namespace

# Upgrade existing installation
helm upgrade right-sizer ./helm -f custom-values.yaml \
  --namespace right-sizer-system
```

Or set directly via command line:

```bash
helm install right-sizer ./helm \
  --set defaultConfig.observability.logLevel=warn \
  --namespace right-sizer-system \
  --create-namespace
```

### Method 3: Via Environment Variable (Advanced)

For more immediate log level control, you can set an environment variable in the deployment:

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
        env:
        - name: LOG_LEVEL
          value: "warn"
```

Or patch the deployment directly:

```bash
kubectl set env deployment/right-sizer LOG_LEVEL=warn -n right-sizer-system
```

## Current Limitations and Workarounds

### Startup Logs

Currently, some initialization logs are emitted at INFO level before the CRD configuration is loaded. This includes:

- Controller initialization messages
- CRD setup messages
- Initial configuration loading

**Workaround:** To suppress all INFO logs from startup, use the environment variable method (Method 3) which takes effect immediately when the container starts.

### Controller Runtime Logs

The operator uses both custom logging and controller-runtime logging. Some logs from the controller-runtime framework may still appear at INFO level even when the custom logger is set to WARN.

**Workaround:** Set the controller-runtime log level using the `--zap-log-level` flag:

```yaml
# In the deployment spec
containers:
- name: right-sizer
  args:
  - --zap-log-level=warn
```

## Examples

### Example 1: Production Setup (Minimal Logging)

```yaml
# production-values.yaml
defaultConfig:
  observability:
    logLevel: "warn"
    logFormat: "json"
    enableAuditLog: true
    enableEvents: true
```

Deploy:

```bash
helm upgrade right-sizer ./helm -f production-values.yaml -n right-sizer-system
```

### Example 2: Development Setup (Verbose Logging)

```yaml
# dev-values.yaml
defaultConfig:
  observability:
    logLevel: "debug"
    logFormat: "text"  # Human-readable format
    enableAuditLog: false
    enableEvents: true
```

Deploy:

```bash
helm upgrade right-sizer ./helm -f dev-values.yaml -n right-sizer-system
```

### Example 3: Quick Debugging

Temporarily enable debug logging without redeploying:

```bash
# Enable debug logging
kubectl patch rightsizerconfig default --type='merge' \
  -p '{"spec":{"observabilityConfig":{"logLevel":"debug"}}}'

# Watch logs
kubectl logs -f -n right-sizer-system -l app.kubernetes.io/name=right-sizer

# When done, restore to warn level
kubectl patch rightsizerconfig default --type='merge' \
  -p '{"spec":{"observabilityConfig":{"logLevel":"warn"}}}'
```

## Verifying Log Level

### Check Current Configuration

```bash
# Check the configured log level in CRD
kubectl get rightsizerconfig default -o jsonpath='{.spec.observabilityConfig.logLevel}'

# Check environment variables in the pod
kubectl get deployment right-sizer -n right-sizer-system -o jsonpath='{.spec.template.spec.containers[0].env}'
```

### Monitor Log Output

```bash
# Count log entries by level
kubectl logs -n right-sizer-system -l app.kubernetes.io/name=right-sizer --tail=100 | \
  grep -oE '\[(DEBUG|INFO|WARN|ERROR)\]' | sort | uniq -c

# Watch only WARN and ERROR messages
kubectl logs -f -n right-sizer-system -l app.kubernetes.io/name=right-sizer | \
  grep -E '\[WARN\]|\[ERROR\]'
```

## Best Practices

1. **Production**: Use `warn` or `error` level to reduce log volume
2. **Staging**: Use `info` level for moderate verbosity
3. **Development/Debugging**: Use `debug` level for maximum detail
4. **Log Format**: Use `json` for structured logging in production (better for log aggregation tools)
5. **Audit Logs**: Keep audit logging enabled even with higher log levels for compliance

## Troubleshooting

### Logs Still Show INFO After Setting to WARN

**Possible Causes:**

1. **Reconciliation Delay**: Wait 30 seconds for the configuration to be applied
2. **Pod Not Restarted**: Some changes may require a pod restart:
   ```bash
   kubectl rollout restart deployment/right-sizer -n right-sizer-system
   ```
3. **Startup Logs**: Initial logs before config load will always be INFO level

### No Effect When Changing Log Level

**Check:**

1. Verify the CRD was updated:
   ```bash
   kubectl get rightsizerconfig default -o yaml | grep logLevel
   ```

2. Check for reconciliation errors:
   ```bash
   kubectl logs -n right-sizer-system -l app.kubernetes.io/name=right-sizer | grep -i error
   ```

3. Ensure the operator has permission to read the CRD:
   ```bash
   kubectl auth can-i get rightsizerconfigs --as=system:serviceaccount:right-sizer-system:right-sizer
   ```

## Related Configuration

Other observability settings that work with log levels:

- `logFormat`: Choose between "json" and "text" output
- `enableAuditLog`: Enable/disable audit logging
- `enableEvents`: Enable/disable Kubernetes event generation
- `enableMetricsExport`: Enable/disable Prometheus metrics
- `metricsPort`: Port for metrics endpoint

See the [Observability Configuration Guide](OBSERVABILITY.md) for more details.
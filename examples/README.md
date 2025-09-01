# Right-Sizer Examples

This directory contains example configurations for the right-sizer operator using Custom Resource Definitions (CRDs). All examples use the CRD-based configuration system and are designed to work with the current version of the operator.

## Prerequisites

- Kubernetes cluster (v1.33+ recommended for in-place resize support)
- Right-sizer operator installed in the `right-sizer` namespace
- CRDs installed (`RightSizerConfig` and `RightSizerPolicy`)

## Available Examples

### Core CRD Examples

#### 1. **config-global-settings.yaml** - Global Configuration Examples
Demonstrates various `RightSizerConfig` configurations for different environments:
- Full-featured configuration with all available options
- Minimal configuration for simple deployments
- Development environment with aggressive optimization
- Production environment with conservative settings

```bash
kubectl apply -f config-global-settings.yaml
```

#### 2. **policies-workload-types.yaml** - Resource Sizing Policies
Shows how to create `RightSizerPolicy` resources for different workload types:
- Production web applications
- Development environments
- Batch processing jobs
- Stateful databases
- Machine learning workloads
- Microservices mesh
- Cross-namespace optimization

```bash
kubectl apply -f policies-workload-types.yaml
```

#### 3. **config-namespace-filtering.yaml** - Namespace Filtering
Examples of configuring namespace inclusion/exclusion:
- Include specific namespaces only
- Exclude system namespaces
- Label-based namespace selection
- Combining multiple filtering strategies

```bash
kubectl apply -f config-namespace-filtering.yaml
```

### Feature-Specific Examples

#### 4. **config-scaling-thresholds.yaml** - Scaling Thresholds
Demonstrates configuring CPU and memory scaling thresholds:
- Scale-up thresholds (when to increase resources)
- Scale-down thresholds (when to decrease resources)
- Different thresholds for different environments
- Per-resource threshold configuration

```bash
kubectl apply -f config-scaling-thresholds.yaml
```

#### 5. **config-rate-limiting.yaml** - Rate Limiting
Shows how to configure rate limiting for resource adjustments:
- Maximum changes per time period
- Cooldown periods between adjustments
- Concurrent resize limits
- Pod restart limits

```bash
kubectl apply -f config-rate-limiting.yaml
```

### Helm Configuration

#### 6. **helm-values-custom.yaml** - Helm Values
Example Helm values file for customizing the operator deployment:
- Custom image configuration
- Log level settings
- Observability configuration
- Controller-specific settings

```bash
helm install right-sizer ../helm -f helm-values-custom.yaml
```

## Usage Guide

### Basic Workflow

1. **Install the operator** (if not already installed):
   ```bash
   helm install right-sizer ../helm --namespace right-sizer --create-namespace
   ```

2. **Apply a global configuration**:
   ```bash
   kubectl apply -f config-global-settings.yaml
   ```

3. **Create policies for your workloads**:
   ```bash
   kubectl apply -f policies-workload-types.yaml
   ```

4. **Monitor the operator**:
   ```bash
   kubectl logs -n right-sizer -l app=right-sizer -f
   ```

### Testing an Example

To test with a specific configuration:

1. Apply the configuration:
   ```bash
   kubectl apply -f config-scaling-thresholds.yaml
   ```

2. Deploy a test workload:
   ```bash
   kubectl create deployment nginx --image=nginx --replicas=3
   kubectl set resources deployment nginx --requests=cpu=100m,memory=128Mi --limits=cpu=500m,memory=512Mi
   ```

3. Watch the operator adjust resources:
   ```bash
   kubectl get pods -w
   ```

4. Check the pod resources:
   ```bash
   kubectl describe pod nginx-xxx | grep -A4 "Requests:"
   ```

### Customizing Examples

All examples can be customized for your environment:

1. **Namespace**: All examples use the `right-sizer` namespace by default
2. **Resource limits**: Adjust min/max values based on your cluster capacity
3. **Thresholds**: Tune scaling thresholds based on workload characteristics
4. **Intervals**: Modify check intervals based on workload volatility

## Configuration Hierarchy

The right-sizer operator uses the following configuration hierarchy (highest priority first):

1. **Pod annotations** (if enabled in policy)
2. **RightSizerPolicy** matching the pod
3. **RightSizerConfig** global settings
4. **Operator defaults**

## Best Practices

1. **Start with conservative settings** in production
2. **Use dry-run mode** to preview changes before applying
3. **Monitor metrics** to validate sizing decisions
4. **Set appropriate rate limits** to prevent thrashing
5. **Use policies** to handle different workload types
6. **Test in development** before applying to production

## Troubleshooting

If examples don't work as expected:

1. **Check CRDs are installed**:
   ```bash
   kubectl get crd | grep rightsizer
   ```

2. **Verify operator is running**:
   ```bash
   kubectl get pods -n right-sizer
   ```

3. **Check operator logs**:
   ```bash
   kubectl logs -n right-sizer -l app=right-sizer --tail=50
   ```

4. **Validate configuration**:
   ```bash
   kubectl apply --dry-run=server -f <example-file.yaml>
   ```

## Archived Examples

Older examples that used ConfigMaps, environment variables, or annotations have been moved to the `archive/` directory. These are no longer supported but kept for reference and migration purposes.

## Contributing

When adding new examples:

1. Use only CRD-based configuration
2. Include comprehensive comments
3. Test with current operator version
4. Use the `right-sizer` namespace
5. Follow the naming convention: `feature-description.yaml`

## Support

For issues or questions:
- Check the [main documentation](../docs/)
- Review operator logs for error messages
- Ensure your Kubernetes version is compatible
- Verify CRDs match the operator version
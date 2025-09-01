# Memory Metrics Testing Guide

## Overview

This guide documents the comprehensive memory metrics testing capabilities for the Right-Sizer operator. The test suite validates memory collection, processing, and optimization features across different scenarios using Minikube.

## Test Scripts

### 1. Comprehensive Memory Metrics Test (`memory-metrics-minikube-test.sh`)

Full-featured test suite that validates all aspects of memory metrics functionality.

#### Features
- Complete memory metrics validation
- Multiple workload patterns testing
- Memory pressure simulation
- Prometheus metrics verification
- Recommendation accuracy testing
- Detailed reporting with JSON output

#### Usage

```bash
# Basic execution
./tests/memory-metrics-minikube-test.sh

# Custom namespace and profile
./tests/memory-metrics-minikube-test.sh \
  --namespace my-operator \
  --profile memory-test \
  --metrics-interval 60

# With cleanup after test
./tests/memory-metrics-minikube-test.sh --cleanup

# Help and options
./tests/memory-metrics-minikube-test.sh --help
```

#### Options

| Option | Description | Default |
|--------|-------------|---------|
| `--namespace NAME` | Operator namespace | `right-sizer` |
| `--test-namespace NAME` | Test workloads namespace | `memory-test` |
| `--profile NAME` | Minikube profile name | `right-sizer-memory` |
| `--metrics-interval SEC` | Metrics collection interval | `30` |
| `--cleanup` | Delete Minikube profile after tests | `false` |

### 2. Quick Memory Test (`quick-memory-test.sh`)

Lightweight script for rapid memory metrics validation.

#### Features
- Fast deployment and testing
- Minimal resource requirements
- Stress testing capabilities
- Real-time metrics monitoring
- Quick validation cycles

#### Usage

```bash
# Full test from scratch
./tests/quick-memory-test.sh

# Skip building (use existing image)
./tests/quick-memory-test.sh --skip-build

# Use existing operator deployment
./tests/quick-memory-test.sh --skip-deploy

# Keep operator after test
./tests/quick-memory-test.sh --keep-operator

# Cleanup only
./tests/quick-memory-test.sh --cleanup-only
```

## Memory Metrics Validated

### Core Metrics

1. **Memory Usage (`rightsizer_memory_usage_bytes`)**
   - Total memory consumption
   - Per-container tracking
   - Historical trends

2. **Working Set (`rightsizer_memory_working_set_bytes`)**
   - Active memory pages
   - Real-time usage patterns
   - Optimization targets

3. **RSS (Resident Set Size) (`rightsizer_memory_rss_bytes`)**
   - Physical memory allocation
   - Non-swapped memory
   - System impact assessment

4. **Cache Memory (`rightsizer_memory_cache_bytes`)**
   - File system cache
   - Buffer cache
   - Performance optimization

5. **Swap Usage (`rightsizer_memory_swap_bytes`)**
   - Swap space utilization
   - Memory pressure indicators
   - Performance degradation signals

### Derived Metrics

- **Memory Utilization Percentage**
- **Memory Pressure Index**
- **Recommendation Accuracy**
- **Resource Efficiency Score**

## Test Scenarios

### 1. Basic Memory Collection
Validates that the operator correctly collects memory metrics from pods.

```yaml
# Test pod with defined memory limits
resources:
  requests:
    memory: "128Mi"
  limits:
    memory: "256Mi"
```

### 2. Memory Stress Testing
Tests operator behavior under various memory pressure conditions.

```yaml
# Stress test configurations
- Low pressure: 50% utilization
- Medium pressure: 75% utilization  
- High pressure: 90% utilization
- OOM conditions: >100% utilization
```

### 3. Memory Leak Detection
Simulates gradual memory increase to test leak detection capabilities.

```bash
# Memory leak simulation
while true; do
  data="${data}XXXXXX"
  sleep 10
done
```

### 4. Multi-Container Pods
Tests memory aggregation across multiple containers.

```yaml
# Pod with multiple containers
containers:
- name: app
  resources:
    memory: "128Mi"
- name: sidecar
  resources:
    memory: "64Mi"
```

### 5. Deployment Scaling
Validates memory metrics during pod scaling operations.

```yaml
# Deployment with replicas
spec:
  replicas: 3
  template:
    spec:
      containers:
      - resources:
          memory: "128Mi"
```

## Prerequisites

### Minikube Configuration

```bash
# Start Minikube with adequate resources
minikube start \
  --memory=6144 \
  --cpus=4 \
  --kubernetes-version=v1.28.0 \
  --addons=metrics-server

# Verify metrics-server
kubectl get deployment metrics-server -n kube-system
```

### Required Tools

- `kubectl` - Kubernetes CLI
- `minikube` - Local Kubernetes cluster
- `docker` - Container runtime
- `curl` - HTTP client
- `jq` - JSON processor

## Test Execution Flow

### 1. Environment Setup
```bash
# Create test profile
minikube start -p right-sizer-memory

# Enable metrics-server
minikube addons enable metrics-server
```

### 2. Operator Deployment
```bash
# Build and load image
docker build -t right-sizer:test .
minikube image load right-sizer:test

# Deploy CRDs and operator
kubectl apply -f helm/crds/
```

### 3. Test Workload Deployment
```bash
# Deploy test pods with various memory profiles
kubectl apply -f tests/fixtures/memory-workloads.yaml
```

### 4. Metrics Collection
```bash
# Wait for metrics collection
sleep 30

# Verify metrics
kubectl top pods -n test-namespace
```

### 5. Validation
```bash
# Check operator logs
kubectl logs -n right-sizer deployment/right-sizer

# Verify Prometheus metrics
curl http://localhost:8080/metrics | grep memory
```

## Expected Results

### Successful Test Indicators

✅ **Metrics Collection**
- All memory metrics types collected
- Accurate values matching `kubectl top`
- Consistent updates at configured intervals

✅ **Memory Recommendations**
- Appropriate sizing suggestions
- Respect for min/max boundaries
- Consideration of usage patterns

✅ **Prometheus Integration**
- All memory metrics exported
- Correct labels and values
- Queryable time series data

✅ **Memory Pressure Handling**
- Detection of high memory usage
- Appropriate alerting/logging
- Preventive recommendations

### Common Issues and Solutions

#### Issue: Metrics Not Available
```bash
# Solution: Wait for metrics-server initialization
kubectl wait --for=condition=available \
  deployment/metrics-server \
  -n kube-system \
  --timeout=300s
```

#### Issue: Memory Metrics Missing
```bash
# Solution: Enable memory metrics in configuration
kubectl edit rightsizerconfig -n right-sizer
# Set: spec.metricsConfig.memoryMetrics.enabled: true
```

#### Issue: OOM Kills During Testing
```bash
# Solution: Increase Minikube memory
minikube stop
minikube config set memory 8192
minikube start
```

## Test Reports

### Log Files
Test execution logs are saved to:
```
./test-logs/memory-metrics-test-YYYYMMDD-HHMMSS.log
```

### JSON Reports
Structured test results in JSON format:
```
./test-logs/memory-metrics-report-YYYYMMDD-HHMMSS.json
```

### Report Structure
```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "test_suite": "memory-metrics",
  "environment": {
    "minikube_profile": "right-sizer-memory",
    "namespace": "right-sizer"
  },
  "results": {
    "total": 25,
    "passed": 23,
    "failed": 2,
    "success_rate": 92
  },
  "details": [...]
}
```

## Continuous Integration

### GitHub Actions Integration
```yaml
name: Memory Metrics Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v2
    - name: Start Minikube
      run: minikube start --driver=docker
    - name: Run Memory Tests
      run: ./tests/memory-metrics-minikube-test.sh
```

### Jenkins Pipeline
```groovy
pipeline {
  agent any
  stages {
    stage('Memory Tests') {
      steps {
        sh './tests/quick-memory-test.sh'
      }
    }
  }
}
```

## Advanced Testing

### Custom Memory Patterns
Create custom workload patterns for specific scenarios:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: memory-patterns
data:
  pattern1.yaml: |
    # Gradual increase pattern
    memory_mb: [50, 100, 150, 200]
    interval_sec: 30
  
  pattern2.yaml: |
    # Spike pattern
    memory_mb: [50, 50, 200, 50]
    interval_sec: 15
```

### Performance Benchmarking
```bash
# Run performance benchmark
time ./tests/memory-metrics-minikube-test.sh \
  --metrics-interval 10 \
  --profile benchmark
```

### Load Testing
```bash
# Deploy multiple test namespaces
for i in {1..10}; do
  kubectl create namespace test-$i
  kubectl apply -f workload.yaml -n test-$i
done
```

## Troubleshooting

### Debug Mode
```bash
# Enable debug logging
export LOG_LEVEL=debug
./tests/memory-metrics-minikube-test.sh
```

### Manual Verification
```bash
# Check metrics endpoint directly
kubectl port-forward -n right-sizer \
  deployment/right-sizer 8080:8080

# In another terminal
curl http://localhost:8080/metrics | grep memory
```

### Operator Logs
```bash
# Live logs
kubectl logs -n right-sizer deployment/right-sizer -f

# Filter memory-related logs
kubectl logs -n right-sizer deployment/right-sizer | \
  grep -i "memory\|mem\|oom"
```

## Contributing

### Adding New Test Cases

1. Create test function in script:
```bash
test_new_memory_scenario() {
  print_header "Testing New Scenario"
  # Test implementation
  log_test_result "New Test" "PASS" "Details"
}
```

2. Add to main execution flow:
```bash
main() {
  # ... existing tests
  test_new_memory_scenario
}
```

3. Document in this guide

### Reporting Issues

When reporting test failures, include:
- Test script version
- Minikube version (`minikube version`)
- Kubernetes version (`kubectl version`)
- Test logs from `./test-logs/`
- Operator logs (`kubectl logs -n right-sizer deployment/right-sizer`)

## Best Practices

1. **Resource Allocation**: Ensure Minikube has sufficient memory (minimum 4GB)
2. **Test Isolation**: Use separate namespaces for different test scenarios
3. **Cleanup**: Always cleanup test resources after completion
4. **Metrics Interval**: Allow sufficient time for metrics collection (minimum 30s)
5. **Validation**: Cross-verify metrics with `kubectl top` commands

## Related Documentation

- [Right-Sizer Configuration Guide](../CONFIGURATION.md)
- [Metrics Provider Documentation](../docs/metrics-providers.md)
- [Troubleshooting Guide](../docs/troubleshooting.md)
- [API Reference](../docs/api-reference.md)

## License

This testing suite is part of the Right-Sizer project and follows the same license terms.
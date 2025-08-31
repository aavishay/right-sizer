# Testing Right-Sizer with Minikube

This guide provides comprehensive instructions for testing the Right-Sizer operator with minikube, including unit tests, integration tests, and resource calculation validation.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Test Suites](#test-suites)
- [Running Tests](#running-tests)
- [Test Scenarios](#test-scenarios)
- [Understanding Test Results](#understanding-test-results)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### Required Tools

- **Go** (1.21+): For running unit tests
- **Minikube** (1.30+): For Kubernetes cluster
- **kubectl**: For cluster interaction
- **Docker** or **Podman**: Container runtime
- **Helm** (optional): For deployment

### Installation

```bash
# Install minikube (macOS)
brew install minikube

# Install minikube (Linux)
curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64
sudo install minikube-linux-amd64 /usr/local/bin/minikube

# Verify installation
minikube version
kubectl version --client
go version
```

## Quick Start

Run all tests with a single command:

```bash
# Quick unit tests (no cluster required)
./scripts/quick-test.sh -a

# Full integration tests with minikube
./scripts/test-minikube.sh

# Resource calculation tests only
./scripts/test-calculations.sh
```

## Test Suites

### 1. Unit Tests (No Cluster Required)

Located in `go/controllers/` and `go/metrics/`:

- **Resource Calculations**: CPU/Memory request and limit calculations
- **Edge Cases**: Problematic scenarios and memory limit issues
- **Metrics**: Aggregation and conversion logic

### 2. Integration Tests (Requires Minikube)

Located in `go/tests/integration/`:

- **Deployment Tests**: Operator deployment and configuration
- **Pod Resizing**: In-place resize functionality
- **Metrics Collection**: Integration with metrics-server

### 3. Performance Tests

- **Stress Testing**: High-load scenarios
- **Scaling Tests**: Multiple pod management
- **Resource Efficiency**: Optimization validation

## Running Tests

### Unit Tests Only

```bash
# Run all unit tests
cd go
go test ./...

# Run specific test suite
go test ./controllers -v -run TestCPURequestCalculations

# Run with coverage
go test ./... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Quick Test Script

The `quick-test.sh` script provides a convenient way to run tests:

```bash
# Show help
./scripts/quick-test.sh -h

# Run quick tests
./scripts/quick-test.sh -q

# Run edge case tests
./scripts/quick-test.sh -e

# Run metrics tests
./scripts/quick-test.sh -m

# Run all tests with verbose output
./scripts/quick-test.sh -a -v

# Run specific test
./scripts/quick-test.sh -s TestMemoryLimitCalculations

# Generate coverage report
./scripts/quick-test.sh -a -c
```

### Minikube Integration Tests

```bash
# Start minikube and run full test suite
./scripts/test-minikube.sh

# Keep cluster running after tests
KEEP_CLUSTER=true ./scripts/test-minikube.sh

# Use specific Kubernetes version
KUBERNETES_VERSION=v1.31.0 ./scripts/test-minikube.sh

# Increase resources
MINIKUBE_MEMORY=8192 MINIKUBE_CPUS=4 ./scripts/test-minikube.sh
```

### Resource Calculation Tests

```bash
# Run focused calculation tests
./scripts/test-calculations.sh

# This script will:
# 1. Create test pods with various resource configurations
# 2. Monitor resource adjustments
# 3. Validate calculation logic
# 4. Generate a test report
```

## Test Scenarios

### CPU Request Calculations

Tests various CPU usage patterns and multipliers:

- **Standard calculation**: `usage * 1.2 + addition`
- **Minimum enforcement**: Ensures minimum CPU (10m)
- **Zero usage handling**: Proper handling of idle containers
- **High usage**: Tests with multi-core usage
- **Fractional values**: Precision in calculations

### Memory Request Calculations

Tests memory allocation strategies:

- **Standard calculation**: `usage * 1.3 + addition`
- **Minimum enforcement**: Ensures minimum memory (64Mi)
- **Gigabyte scale**: Large memory allocations
- **Buffer calculations**: Safety margins

### Memory Limit Edge Cases

Critical scenarios that could cause issues:

| Scenario | Risk Level | Description |
|----------|------------|-------------|
| Limit = Request | **CRITICAL** | Certain OOM, no buffer |
| <10% buffer | **HIGH** | Insufficient spike headroom |
| 10-20% buffer | **MEDIUM** | Minimal safety margin |
| 20-50% buffer | **LOW** | Adequate for most workloads |
| >50% buffer | **SAFE** | Generous headroom |

### Workload-Specific Ratios

Recommended memory limit to request ratios:

| Workload Type | Min Ratio | Recommended | Max Ratio |
|---------------|-----------|-------------|-----------|
| Go services | 1.1x | 1.2x | 1.4x |
| Node.js apps | 1.2x | 1.3x | 1.5x |
| Java apps | 1.3x | 1.5x | 2.0x |
| Python ML | 1.3x | 1.5x | 2.0x |
| Databases | 1.15x | 1.25x | 1.5x |
| Cache servers | 1.2x | 1.3x | 1.5x |
| Batch jobs | 1.05x | 1.1x | 1.3x |

### Burst Pattern Tests

Tests different memory usage patterns:

- **Constant usage**: Stable workloads (1.2x buffer)
- **Periodic spikes**: Regular patterns (1.6x buffer)
- **Unpredictable bursts**: Random spikes (2.2x buffer)
- **GC spikes**: Garbage collection patterns (1.4x buffer)
- **Startup spikes**: Initial resource needs (2.1x buffer)

## Understanding Test Results

### Successful Test Output

```
✓ CPU Request Calculations - PASSED
✓ Memory Request Calculations - PASSED
✓ CPU Limit Calculations - PASSED
✓ Memory Limit Calculations - PASSED
```

### Warning Indicators

```
⚠ WARNING: Memory limit too close to request - Only 5.0% overhead
⚠ WARNING: Insufficient headroom for memory spikes
```

### Critical Issues

```
✗ CRITICAL: Memory limit equals request - certain OOM
✗ CRITICAL: CPU limit below request - invalid configuration
```

### Test Report

After running `test-calculations.sh`, a report is generated:

```
test-report-YYYYMMDD-HHMMSS.txt
```

This includes:
- Test environment details
- Test cases executed
- Key findings
- Recommendations

## Test Coverage

Current test coverage includes:

- **CPU Calculations**: 15 test scenarios
- **Memory Calculations**: 17 test scenarios
- **Edge Cases**: 7 problematic scenarios
- **Workload Types**: 8 different patterns
- **Burst Patterns**: 6 different patterns
- **Container Configs**: 5 different setups
- **Validation Rules**: 7 validation scenarios

## Troubleshooting

### Common Issues

#### Minikube Won't Start

```bash
# Reset minikube
minikube delete -p right-sizer-test
minikube start -p right-sizer-test --driver=docker

# Check Docker daemon
docker ps
systemctl status docker  # Linux
```

#### Metrics Server Not Ready

```bash
# Check metrics-server status
kubectl get pods -n kube-system | grep metrics

# Restart metrics-server
kubectl delete pod -n kube-system -l k8s-app=metrics-server

# Test metrics API
kubectl top nodes
kubectl top pods --all-namespaces
```

#### Tests Failing

```bash
# Run with verbose output
./scripts/quick-test.sh -v -a

# Check specific test
go test -v ./controllers -run TestMemoryLimitCalculations

# Clean go cache
go clean -testcache
```

#### Resource Constraints

```bash
# Increase minikube resources
minikube stop -p right-sizer-test
minikube config set memory 4096 -p right-sizer-test
minikube config set cpus 2 -p right-sizer-test
minikube start -p right-sizer-test
```

### Debug Commands

```bash
# Check operator logs
kubectl logs -n right-sizer-system deployment/right-sizer -f

# Check pod events
kubectl describe pod <pod-name> -n <namespace>

# Check metrics for a pod
kubectl top pod <pod-name> -n <namespace>

# Get pod resource specifications
kubectl get pod <pod-name> -n <namespace> -o yaml | grep -A10 resources
```

## Key Findings from Tests

### Critical Insights

1. **Memory Limit = Request is Dangerous**: Always maintain at least 5-10% buffer
2. **Java Applications Need Extra Memory**: 30-50% overhead for non-heap memory
3. **Unpredictable Workloads Need Large Buffers**: 2x or more for safety
4. **Scale Down Should Use Conservative Multipliers**: 1.1x instead of full multiplier
5. **Container Architecture Matters**: Sidecars and init containers affect total resources

### Best Practices

1. **Minimum Buffer**: Always maintain 10%+ between request and limit
2. **Workload Profiling**: Use appropriate multipliers for application type
3. **Monitor Near-Limit Usage**: Track pods approaching limits
4. **Regular Testing**: Run tests after configuration changes
5. **Production Validation**: Test with actual workload patterns

## Contributing Tests

To add new tests:

1. **Unit Tests**: Add to `go/controllers/*_test.go` or `go/metrics/*_test.go`
2. **Integration Tests**: Add to `go/tests/integration/`
3. **Test Scenarios**: Update `scripts/test-calculations.sh`
4. **Documentation**: Update this README with new test cases

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Test
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      - name: Run Tests
        run: |
          cd go
          go test ./... -v -cover
```

### GitLab CI Example

```yaml
test:
  stage: test
  image: golang:1.21
  script:
    - cd go
    - go test ./... -v -cover
  coverage: '/coverage: \d+.\d+%/'
```

## Summary

The Right-Sizer test suite provides comprehensive validation of resource calculation logic with special attention to:

- **Edge cases** that could cause production issues
- **Memory limit configurations** that may lead to OOM
- **Different workload patterns** and their resource needs
- **Proper scaling decisions** based on usage patterns
- **Validation** of resource configurations

Regular testing ensures the operator makes safe and efficient resource adjustments while avoiding common pitfalls in container resource management.
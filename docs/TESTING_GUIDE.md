# Testing Guide for Right-Sizer

This comprehensive guide covers all testing aspects of the Right-Sizer operator, from unit tests to end-to-end testing on real Kubernetes clusters.

---

## Table of Contents

- [Quick Start](#quick-start)
- [Prerequisites](#prerequisites)
- [Testing Types](#testing-types)
- [Unit Testing](#unit-testing)
- [Integration Testing](#integration-testing)
- [End-to-End Testing](#end-to-end-testing)
- [Minikube Testing](#minikube-testing)
- [CI/CD Testing](#cicd-testing)
- [Performance Testing](#performance-testing)
- [Test Coverage](#test-coverage)
- [Troubleshooting](#troubleshooting)

---

## Quick Start

```bash
# Run all tests (unit + integration)
make test

# Run unit tests only
cd go && go test -v ./...

# Run with coverage
cd go && go test -v -cover ./...

# Run linting
cd go && golangci-lint run

# Run E2E tests on Minikube
./tests/test-minikube-basic.sh

# Run with GitHub Actions locally
act -j test --container-architecture linux/amd64
```

---

## Prerequisites

### Required Tools

- **Go**: 1.25+
- **Docker**: For building images
- **Kubernetes**: 1.33+ (for in-place resizing)
- **kubectl**: Configured with cluster access
- **Helm**: 3.0+ for deployments
- **Minikube**: For local testing (optional)
- **golangci-lint**: For code quality checks
- **act**: For testing GitHub Actions locally (optional)

### Installation

```bash
# Install Go dependencies
cd go && go mod download

# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.1

# Install act (for GitHub Actions testing)
brew install act  # macOS
# or
curl https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash  # Linux
```

---

## Testing Types

### 1. **Unit Tests**
- Test individual functions and components
- No external dependencies required
- Fast execution (~2 seconds)
- Coverage target: 80%+

### 2. **Integration Tests**
- Test component interactions
- May require test database or mock services
- Medium execution time (~10 seconds)
- Requires build tag: `-tags=integration`

### 3. **End-to-End Tests**
- Test complete workflows on real clusters
- Requires Kubernetes cluster
- Slower execution (2-5 minutes)
- Validates real-world scenarios

### 4. **Performance Tests**
- Load testing with multiple pods
- Resource usage monitoring
- Scalability validation
- Benchmark tests included

---

## Unit Testing

### Running Unit Tests

```bash
# Run all unit tests
cd go && go test ./...

# Run with verbose output
go test -v ./...

# Run specific package
go test -v ./config

# Run specific test
go test -v -run TestShouldSkipPod ./...

# Run with race detection
go test -race ./...

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
open coverage.html  # View in browser
```

### Test Organization

```
go/
├── main_test.go           # Main package tests
├── admission/
│   └── webhook_test.go    # Webhook tests
├── api/
│   └── server_test.go     # API server tests
├── audit/
│   └── audit_test.go      # Audit logging tests
├── config/
│   └── config_test.go     # Configuration tests
├── controllers/
│   └── adaptive_test.go   # Controller tests
├── metrics/
│   └── provider_test.go   # Metrics provider tests
└── predictor/
    └── predictor_test.go  # ML predictor tests
```

### Writing Unit Tests

```go
// Example unit test structure
func TestResourceCalculation(t *testing.T) {
    testCases := []struct {
        name     string
        input    resource.Quantity
        expected resource.Quantity
    }{
        {
            name:     "normal usage",
            input:    resource.MustParse("100m"),
            expected: resource.MustParse("150m"),
        },
    }

    for _, tc := range testCases {
        t.Run(tc.name, func(t *testing.T) {
            result := calculateResource(tc.input)
            assert.Equal(t, tc.expected, result)
        })
    }
}
```

### Current Test Results (v0.2.1)

| Package | Coverage | Status | Notes |
|---------|----------|--------|-------|
| main | 85% | ✅ Pass | Core logic |
| admission | 78% | ✅ Pass | Webhook validation |
| api | 82% | ✅ Pass | REST API |
| audit | 90% | ✅ Pass | Logging system |
| config | 95% | ✅ Pass | Configuration mgmt |
| controllers | 75% | ✅ Pass | K8s controllers |
| metrics | 88% | ✅ Pass | Metrics collection |
| predictor | 70% | ✅ Pass | ML predictions |

---

## Integration Testing

### Setup

```bash
# Integration tests require build tag
cd tests
go test -v -tags=integration ./integration

# With specific timeout
go test -v -tags=integration -timeout=120s ./integration

# Run with environment variables
RUN_INTEGRATION_TESTS=true go test -v -tags=integration ./integration
```

### Integration Test Structure

```go
//go:build integration
// +build integration

package integration

import (
    "testing"
    "sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestIntegrationSuite(t *testing.T) {
    // Setup test environment
    testEnv := &envtest.Environment{
        CRDDirectoryPaths: []string{"../crds"},
    }

    // Start test environment
    cfg, err := testEnv.Start()
    require.NoError(t, err)
    defer testEnv.Stop()

    // Run tests...
}
```

### Current Limitations

- Integration tests require `envtest` binaries
- Not all integration tests are fully implemented
- Some tests have compilation issues (being fixed)

---

## End-to-End Testing

### E2E Test Script

The comprehensive E2E test script validates:
- Operator deployment
- Health endpoints
- Pod resizing
- Resource limits
- Batch processing
- Audit logging
- Skip annotations

### Running E2E Tests

```bash
# Basic Minikube test
./tests/test-minikube-basic.sh

# Comprehensive E2E test
./tests/test-e2e-minikube.sh

# Test with specific namespace
NAMESPACE=my-namespace ./tests/test-e2e-minikube.sh

# Test with custom timeout
TIMEOUT=600 ./tests/test-e2e-minikube.sh
```

### E2E Test Coverage

| Test Area | Status | Description |
|-----------|--------|-------------|
| Deployment | ✅ Pass | Operator deployment and CRDs |
| Health Probes | ✅ Pass | Liveness/readiness endpoints |
| Pod Detection | ✅ Pass | Finds and monitors pods |
| CPU Resizing | ✅ Pass | Reduces oversized CPU |
| Memory Prediction | ✅ Pass | ML-based memory sizing |
| Batch Processing | ✅ Pass | Handles multiple pods |
| Skip Annotations | ✅ Pass | Respects disable flags |
| Audit Logging | ✅ Pass | Tracks all changes |
| Policy Application | ✅ Pass | Applies custom policies |

---

## Minikube Testing

### Setup Minikube Cluster

```bash
# Start Minikube with required K8s version
minikube start \
  --profile=right-sizer \
  --kubernetes-version=v1.34.0 \
  --memory=4096 \
  --cpus=2 \
  --driver=docker

# Enable metrics-server
minikube addons enable metrics-server

# Verify cluster
kubectl get nodes
kubectl version
```

### Deploy Right-Sizer

```bash
# Using Makefile
make mk-deploy

# Or manually
docker build -t right-sizer:test -f Dockerfile.minikube .
minikube image load right-sizer:test
helm install right-sizer ./helm \
  --namespace right-sizer \
  --create-namespace \
  --set image.repository=right-sizer \
  --set image.tag=test \
  --set image.pullPolicy=Never
```

### Test Scenarios

```bash
# 1. Deploy oversized pod
kubectl apply -f tests/test-deployment-oversized.yaml

# 2. Watch operator logs
kubectl logs -n right-sizer -f deploy/right-sizer-rightsizer

# 3. Check resource changes
kubectl get pod -n test-rightsizer -o yaml | grep -A5 resources

# 4. Run load test
for i in {1..10}; do
  kubectl run test-pod-$i --image=nginx \
    --requests="cpu=100m,memory=128Mi" \
    --limits="cpu=200m,memory=256Mi"
done

# 5. Clean up
kubectl delete ns test-rightsizer
```

### Verification Steps

```bash
# Check operator status
kubectl get all -n right-sizer

# Verify metrics endpoint
kubectl port-forward -n right-sizer svc/right-sizer-rightsizer 9090:9090 &
curl http://localhost:9090/metrics | grep rightsizer

# Check for errors
kubectl logs -n right-sizer deploy/right-sizer-rightsizer | grep ERROR

# Verify CRDs
kubectl get crd | grep rightsizer
```

---

## CI/CD Testing

### GitHub Actions

The project uses GitHub Actions for automated testing:

```yaml
# .github/workflows/test.yml
name: Test
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: golangci-lint run
        working-directory: go
      - run: go test -v ./...
        working-directory: go
```

### Local GitHub Actions Testing

```bash
# Test with act
act -j test

# With specific architecture
act -j test --container-architecture linux/amd64

# List available workflows
act -l

# Run specific workflow
act -W .github/workflows/test.yml

# With secrets
act -j test -s GITHUB_TOKEN=your-token
```

### Pre-commit Hooks

```bash
# Install pre-commit
pip install pre-commit

# Install hooks
pre-commit install

# Run manually
pre-commit run --all-files

# Skip hooks (emergency)
git commit --no-verify
```

---

## Performance Testing

### Benchmarks

```bash
# Run benchmarks
cd go && go test -bench=. -benchmem ./...

# Run specific benchmark
go test -bench=BenchmarkResourceCalculation -benchmem

# With CPU profiling
go test -bench=. -cpuprofile=cpu.prof
go tool pprof cpu.prof

# With memory profiling
go test -bench=. -memprofile=mem.prof
go tool pprof mem.prof
```

### Load Testing

```bash
# Deploy many pods
./tests/scripts/deploy-load-test.sh 100

# Monitor operator performance
kubectl top pod -n right-sizer

# Check processing rate
kubectl logs -n right-sizer deploy/right-sizer-rightsizer | \
  grep "Rightsizing run completed" | \
  tail -10

# Measure API response times
while true; do
  time curl -s http://localhost:9090/metrics > /dev/null
  sleep 1
done
```

### Performance Targets

| Metric | Target | Current | Status |
|--------|--------|---------|--------|
| Startup Time | < 30s | 15s | ✅ |
| Memory Usage | < 256Mi | 128Mi | ✅ |
| CPU Usage (idle) | < 50m | 20m | ✅ |
| Pod Processing | 100/min | 120/min | ✅ |
| API Response | < 100ms | 50ms | ✅ |

---

## Test Coverage

### Generating Coverage Report

```bash
# Generate coverage for all packages
cd go
go test -coverprofile=coverage.out ./...

# Generate HTML report
go tool cover -html=coverage.out -o coverage.html

# View coverage by function
go tool cover -func=coverage.out

# Coverage with race detection
go test -race -coverprofile=coverage.out ./...
```

### Coverage Goals

- **Overall**: 80%+ ✅ (Currently ~82%)
- **Critical Paths**: 90%+
- **New Code**: 85%+
- **API Endpoints**: 95%+

### Improving Coverage

Priority areas for coverage improvement:
1. Error handling paths
2. Edge cases in resource calculations
3. Webhook validation scenarios
4. Policy engine logic
5. Metric collection edge cases

---

## Troubleshooting

### Common Test Issues

#### 1. **Tests Failing Locally**

```bash
# Clear test cache
go clean -testcache

# Update dependencies
go mod tidy
go mod download

# Check Go version
go version  # Should be 1.25+
```

#### 2. **Integration Tests Not Running**

```bash
# Ensure build tag is included
go test -tags=integration ./...

# Set environment variable
RUN_INTEGRATION_TESTS=true go test -tags=integration ./...

# Install envtest binaries (if needed)
go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
setup-envtest use 1.33
```

#### 3. **Minikube Tests Failing**

```bash
# Check Minikube status
minikube status

# Verify Kubernetes version
kubectl version --short

# Check metrics-server
kubectl get deploy -n kube-system metrics-server

# Review operator logs
kubectl logs -n right-sizer deploy/right-sizer-rightsizer --tail=100
```

#### 4. **Coverage Report Issues**

```bash
# Ensure all packages are included
go test -coverprofile=coverage.out ./...

# Exclude test files from coverage
go test -coverprofile=coverage.out -coverpkg=./... ./...

# Generate detailed report
go test -v -covermode=count -coverprofile=coverage.out ./...
```

#### 5. **Act/GitHub Actions Issues**

```bash
# Use specific architecture
act --container-architecture linux/amd64

# Increase resources
act --container-options "--memory=4g --cpus=2"

# Use different image
act --platform ubuntu-latest=nektos/act-environments-ubuntu:22.04
```

### Debug Commands

```bash
# Verbose test output
go test -v -count=1 ./...

# Show test execution time
go test -v -timeout 30s ./...

# Parallel test execution
go test -parallel 4 ./...

# List tests without running
go test -list . ./...
```

---

## Best Practices

### Test Writing Guidelines

1. **Use Table-Driven Tests**: Group similar test cases
2. **Test Names**: Use descriptive names (TestFeature_Scenario_Expected)
3. **Isolation**: Each test should be independent
4. **Cleanup**: Always clean up resources
5. **Assertions**: Use clear assertion messages
6. **Mocking**: Mock external dependencies
7. **Coverage**: Aim for critical path coverage

### Test Organization

```
tests/
├── unit/           # Unit tests
├── integration/    # Integration tests
├── e2e/           # End-to-end tests
├── fixtures/      # Test data
├── helpers/       # Test utilities
└── scripts/       # Test automation
```

### Continuous Improvement

- Review test failures in CI/CD
- Add tests for bug fixes
- Refactor tests alongside code
- Monitor test execution time
- Update test documentation

---

## Related Documentation

- [Architecture Review](./ARCHITECTURE_REVIEW.md)
- [Implementation Status](./IMPLEMENTATION_STATUS.md)
- [Troubleshooting Guide](./troubleshooting-k8s.md)
- [Runtime Testing Guide](./RUNTIME_TESTING_GUIDE.md)
- [GitHub Actions Testing](./github-actions-testing.md)

---

**Last Updated:** October 2024
**Version:** 0.2.1
**Maintainer:** Right-Sizer Team

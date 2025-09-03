# Right Sizer Test Suite

This directory contains all tests for the Right Sizer Kubernetes operator, organized by test type and purpose.

## Directory Structure

```
tests/
├── unit/                    # Unit tests for individual components
│   ├── controllers/         # Controller unit tests
│   ├── metrics/            # Metrics provider unit tests
│   └── main_test.go        # Main application unit tests
├── integration/            # Integration tests
│   └── integration_test.go # End-to-end integration tests
├── rbac/                   # RBAC-specific tests
│   ├── test-rbac-suite.sh # Comprehensive RBAC test suite
│   └── rbac-integration-test.sh # RBAC integration tests
├── workloads/              # Test workload deployments for Right-Sizer validation
│   ├── basic/              # Basic test deployments
│   │   └── test-deployment.yaml
│   ├── redis/              # Redis cluster test workloads
│   │   └── redis-cluster.yaml
│   └── mongodb/            # MongoDB replica set test workloads
│       ├── mongodb-cluster.yaml
│       ├── mongodb-noauth.yaml
│       └── mongodb-load-simple.yaml
├── fixtures/               # Test fixtures and sample resources
│   ├── nginx-deployment.yaml
│   ├── stress-test-pods.yaml
│   ├── test-aggressive.yaml
│   ├── test-correct.yaml
│   ├── test-deployment.yaml
│   ├── test-pods.yaml
│   ├── test-resize-pod.yaml
│   ├── test-simple.yaml
│   ├── root-test-deployment.yaml
│   └── validate-config.yaml
├── scripts/                # Test helper scripts
│   └── minikube-test.sh   # Minikube-specific tests
├── run-all-tests.sh        # Main test runner
├── test_additions.sh       # Test for addition-based calculations
└── README.md              # This file
```

## Test Types

### Unit Tests

Unit tests focus on individual components and functions in isolation.

**Location:** `tests/unit/`

**Run unit tests:**
```bash
# Run all unit tests
cd tests/unit
go test ./...

# Run with coverage
go test -cover ./...

# Run specific test
go test ./controllers -run TestRightSizerController

# Verbose output
go test -v ./...
```

### Integration Tests

Integration tests verify that components work together correctly in a real or simulated Kubernetes environment.

**Location:** `tests/integration/`

**Run integration tests:**
```bash
# Run integration tests
cd tests/integration
go test

# Run with specific kubeconfig
KUBECONFIG=~/.kube/config go test

# Run against specific cluster
go test -cluster-endpoint=https://my-cluster:6443
```

### RBAC Tests

RBAC tests verify that the operator has the correct permissions to function properly.

**Location:** `tests/rbac/`

**Run RBAC tests:**
```bash
# Run comprehensive RBAC test suite
./tests/rbac/test-rbac-suite.sh

# Run with verbose output
./tests/rbac/test-rbac-suite.sh --verbose

# Run specific namespace
./tests/rbac/test-rbac-suite.sh -n my-namespace

# Output as JSON for CI/CD
./tests/rbac/test-rbac-suite.sh --output json > rbac-results.json
```

### Smoke Tests

Quick validation tests to ensure basic functionality.

**Run smoke tests:**
```bash
./tests/run-all-tests.sh --smoke
```

### Workload Tests

Workload tests deploy real applications to validate Right-Sizer's optimization capabilities with production-like workloads.

**Location:** `tests/workloads/`

#### Basic Workloads

Simple test deployments for basic Right-Sizer functionality:

```bash
# Deploy basic test workloads
kubectl apply -f tests/workloads/basic/test-deployment.yaml

# Watch Right-Sizer optimize the resources
kubectl logs -n right-sizer -l app.kubernetes.io/name=right-sizer -f
```

#### Redis Cluster Tests

Deploy a Redis master-replica cluster with load generators:

```bash
# Deploy Redis cluster with load testing
kubectl apply -f tests/workloads/redis/redis-cluster.yaml

# Check optimization progress
kubectl top pods -n redis
kubectl get pods -n redis -o custom-columns=NAME:.metadata.name,CPU_REQ:.spec.containers[0].resources.requests.cpu,MEM_REQ:.spec.containers[0].resources.requests.memory

# Clean up
kubectl delete -f tests/workloads/redis/redis-cluster.yaml
```

**Components:**
- 1 Redis master
- 3 Redis replicas
- 2 Load generators creating continuous read/write operations
- 1 Benchmark pod running redis-benchmark

#### MongoDB Tests

Deploy MongoDB replica set with various configurations:

```bash
# Deploy MongoDB without authentication (for testing)
kubectl apply -f tests/workloads/mongodb/mongodb-noauth.yaml

# Deploy MongoDB with authentication
kubectl apply -f tests/workloads/mongodb/mongodb-cluster.yaml

# Deploy simplified MongoDB load generator
kubectl apply -f tests/workloads/mongodb/mongodb-load-simple.yaml

# Monitor Right-Sizer adjustments
kubectl logs -n right-sizer -l app.kubernetes.io/name=right-sizer --tail=50 | grep mongodb

# Clean up
kubectl delete namespace mongodb
```

**Components:**
- 3-node MongoDB replica set
- Load generators for read/write operations
- Initialization jobs for replica set configuration

#### Running Workload Tests

```bash
# Deploy all workload tests
for workload in tests/workloads/*/*.yaml; do
  kubectl apply -f "$workload"
done

# Monitor Right-Sizer's optimization
watch kubectl top pods -A

# Generate load test report
kubectl logs -n right-sizer -l app.kubernetes.io/name=right-sizer | grep "Successfully resized" > workload-test-results.log

# Clean up all test workloads
kubectl delete namespace redis mongodb default
```

## Test Fixtures

Test fixtures are located in `tests/fixtures/` and contain sample Kubernetes resources for testing:

- **nginx-deployment.yaml**: Basic nginx deployment for testing
- **stress-test-pods.yaml**: CPU/memory stress test pods
- **test-aggressive.yaml**: Aggressive resource sizing test
- **test-correct.yaml**: Correct resource configuration test
- **test-deployment.yaml**: Generic test deployment
- **test-pods.yaml**: Standalone test pods
- **test-resize-pod.yaml**: Pod resize functionality test
- **test-simple.yaml**: Simple test configuration
- **validate-config.yaml**: Configuration validation test

## Running Tests

### All Tests

Run the complete test suite:

```bash
# Run all tests (unit, integration, smoke)
./tests/run-all-tests.sh

# Run all tests including Minikube tests
./tests/run-all-tests.sh --all

# Run with verbose output and coverage
./tests/run-all-tests.sh --verbose --coverage
```

### Specific Test Types

```bash
# Unit tests only
./tests/run-all-tests.sh --unit

# Integration tests only
./tests/run-all-tests.sh --integration

# Smoke tests only
./tests/run-all-tests.sh --smoke

# Minikube tests
./tests/run-all-tests.sh --minikube
```

### Test Additions Feature

Test the CPU and memory addition feature:

```bash
# Run addition feature tests
./tests/test_additions.sh

# Run with cleanup
./tests/test_additions.sh --cleanup
```

### Environment Variables

Configure test behavior with environment variables:

```bash
# Skip certain test types
export RUN_UNIT=false
export RUN_INTEGRATION=true
export RUN_SMOKE=true

# Enable verbose output
export VERBOSE=true

# Generate coverage reports
export COVERAGE=true

# Run tests
./tests/run-all-tests.sh
```

## Writing New Tests

### Go Unit Tests

Create unit tests following Go conventions:

```go
// tests/unit/mycomponent/mycomponent_test.go
package mycomponent_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestMyFunction(t *testing.T) {
    // Arrange
    input := "test"
    expected := "TEST"
    
    // Act
    result := MyFunction(input)
    
    // Assert
    assert.Equal(t, expected, result)
}
```

### Integration Tests

Create integration tests that interact with a Kubernetes cluster:

```go
// tests/integration/feature_test.go
package integration_test

import (
    "context"
    "testing"
    "k8s.io/client-go/kubernetes"
)

func TestFeatureIntegration(t *testing.T) {
    // Setup client
    client := setupKubernetesClient(t)
    
    // Deploy test resources
    deployTestResources(t, client)
    defer cleanupTestResources(t, client)
    
    // Test feature
    testFeature(t, client)
}
```

### Shell Test Scripts

Create shell scripts for complex test scenarios:

```bash
#!/bin/bash
# tests/my-test.sh

set -e

# Source test helpers
source "$(dirname "$0")/test-helpers.sh"

# Test function
test_my_feature() {
    log_info "Testing my feature..."
    
    # Test logic here
    kubectl apply -f fixtures/my-test.yaml
    
    # Assertions
    assert_pod_running "my-pod" "default"
}

# Run test
test_my_feature
```

## CI/CD Integration

### GitHub Actions

```yaml
# .github/workflows/test.yml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Setup Go
        uses: actions/setup-go@v2
        with:
          go-version: 1.25
      
      - name: Run Unit Tests
        run: |
          cd tests
          ./run-all-tests.sh --unit --coverage
      
      - name: Upload Coverage
        uses: codecov/codecov-action@v2
        with:
          file: ./coverage.out
```

### GitLab CI

```yaml
# .gitlab-ci.yml
test:
  stage: test
  script:
    - cd tests
    - ./run-all-tests.sh --unit --coverage
  artifacts:
    reports:
      coverage_report:
        coverage_format: cobertura
        path: coverage.xml
```

### Jenkins

```groovy
// Jenkinsfile
pipeline {
    agent any
    stages {
        stage('Test') {
            steps {
                sh 'cd tests && ./run-all-tests.sh --all --verbose'
            }
        }
    }
    post {
        always {
            junit 'tests/test-results.xml'
            publishHTML target: [
                reportDir: 'tests',
                reportFiles: 'coverage.html',
                reportName: 'Coverage Report'
            ]
        }
    }
}
```

## Test Best Practices

1. **Isolation**: Each test should be independent and not rely on other tests
2. **Cleanup**: Always clean up test resources after tests complete
3. **Naming**: Use descriptive test names that explain what is being tested
4. **Documentation**: Add comments explaining complex test logic
5. **Assertions**: Use clear assertions with helpful error messages
6. **Timeouts**: Set appropriate timeouts for long-running tests
7. **Parallelization**: Design tests to run in parallel when possible
8. **Fixtures**: Use consistent test fixtures across related tests
9. **Mocking**: Mock external dependencies for unit tests
10. **Coverage**: Aim for >80% code coverage

## Troubleshooting

### Common Issues

#### Tests Can't Connect to Cluster

```bash
# Check cluster connectivity
kubectl cluster-info

# Set correct kubeconfig
export KUBECONFIG=~/.kube/config

# Verify permissions
kubectl auth can-i create pods
```

#### Test Timeouts

```bash
# Increase timeout for slow clusters
export TEST_TIMEOUT=300s

# Run with extended timeout
go test -timeout 30m
```

#### Coverage Not Generated

```bash
# Ensure coverage tools are installed
go install golang.org/x/tools/cmd/cover@latest

# Run with explicit coverage output
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

#### RBAC Test Failures

```bash
# Verify service account exists
kubectl get sa -n right-sizer-system

# Check RBAC permissions
kubectl auth can-i --list \
  --as=system:serviceaccount:right-sizer-system:right-sizer
```

## Test Reports

Test results and reports are generated in the following locations:

- **Coverage Report**: `coverage.html`
- **Test Results (JSON)**: `test-results.json`
- **RBAC Report**: `rbac-test-results.json`
- **JUnit XML**: `junit.xml` (when using --junit flag)

## Contributing

When adding new tests:

1. Place tests in the appropriate directory
2. Follow existing naming conventions
3. Update this README if adding new test categories
4. Ensure tests pass locally before submitting PR
5. Include test output in PR description
6. Add integration with CI/CD if needed

## License

Tests are part of the Right Sizer project and follow the same AGPL-3.0 license.
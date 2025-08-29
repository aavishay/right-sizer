# Right-Sizer Tests

This directory contains all tests, test fixtures, and testing utilities for the Right-Sizer Kubernetes operator.

## 📁 Directory Structure

```
test/
├── fixtures/          # Test manifests and sample deployments
├── integration/       # Integration tests
├── scripts/          # Test automation scripts
└── README.md         # This file
```

## 🧪 Test Types

### Unit Tests
Unit tests are located alongside the source code in the `go/` directory:
- `go/controllers/rightsizer_test.go` - Controller unit tests
- `go/metrics/provider_test.go` - Metrics provider tests
- `go/main_test.go` - Main function tests

Run unit tests:
```bash
cd go && go test ./... -v
```

### Integration Tests
Integration tests validate the operator's behavior in a real Kubernetes environment:
- `integration/integration_test.go` - Full integration test suite

Run integration tests:
```bash
cd test/integration && go test -v
```

## 🎯 Test Fixtures

### Pod Manifests

#### `fixtures/test-deployment.yaml`
Simple nginx deployment for basic functionality testing:
- 2 replicas with minimal resources
- Used for smoke testing

#### `fixtures/stress-test-pods.yaml`
Resource-intensive pods for resize testing:
- **stress-test-pod**: CPU and memory stress testing
- **nginx-test-pod**: Low initial resources for resize demonstration
- **busybox-test-pod**: Periodic CPU load generation

#### `fixtures/test-pods.yaml`
Various pod configurations for comprehensive testing:
- Different resource configurations
- Multiple container scenarios
- System namespace pods

#### `fixtures/test-resize-pod.yaml`
Pod with explicit resize policies:
- Tests in-place resize without restart
- Validates resize policy compliance

#### `fixtures/test-aggressive.yaml`
Aggressive resource configuration testing:
- High resource multipliers
- Tests maximum limits
- Validates safety thresholds

## 🚀 Quick Start

### 1. Basic Smoke Test
```bash
# Deploy the operator and run basic tests
./test/scripts/test.sh
```

### 2. Minikube Full Test
```bash
# Run comprehensive tests on Minikube
./test/scripts/minikube-full-test.sh
```

### 3. In-Place Resize Test
```bash
# Test Kubernetes 1.33+ in-place resize functionality
./test/scripts/test-inplace-resize.sh
```

## 📜 Test Scripts

### Core Test Scripts

| Script | Description |
|--------|-------------|
| `test.sh` | Main test runner - executes all unit tests |
| `minikube-test.sh` | Minikube-specific test suite |
| `test-inplace-resize.sh` | Tests in-place pod resizing |
| `validate-all-config.sh` | Validates all configuration options |

### Configuration Test Scripts

| Script | Description |
|--------|-------------|
| `test-config.sh` | Tests configuration loading and validation |
| `quick-test-config.sh` | Quick configuration smoke test |
| `test-interval-loglevel.sh` | Tests resize interval and log level settings |
| `test-minikube-config.sh` | Minikube-specific configuration tests |

### Deployment Scripts

| Script | Description |
|--------|-------------|
| `minikube-deploy.sh` | Deploy operator to Minikube |
| `minikube-helm-deploy.sh` | Deploy using Helm chart |
| `minikube-cleanup.sh` | Clean up test resources |
| `minikube-config-test.sh` | Test configuration in Minikube |

## 🔧 Running Tests

### Prerequisites
- Go 1.24+
- Kubernetes cluster (Minikube recommended for local testing)
- kubectl configured
- Helm 3+ (for Helm deployment tests)

### Run All Tests
```bash
# From project root
make test

# Or directly
./test/scripts/test.sh
```

### Run Specific Test Suite
```bash
# Unit tests only
cd go && go test ./... -v

# Integration tests only
cd test/integration && go test -v

# Minikube tests
./test/scripts/minikube-full-test.sh
```

### Test with Coverage
```bash
cd go && go test ./... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## 🐛 Debugging Tests

### Enable Verbose Logging
```bash
# Set log level to debug
export LOG_LEVEL=debug
./test/scripts/test.sh
```

### Run Single Test
```bash
# Run specific test function
cd go && go test -v -run TestResourceCalculation ./controllers
```

### Check Test Pods
```bash
# View test pods
kubectl get pods -l test=right-sizer

# Check pod resources
kubectl describe pod <test-pod-name>

# View resize events
kubectl get events --field-selector reason=Resized
```

## 📊 Test Coverage Goals

- **Unit Tests**: > 80% code coverage
- **Integration Tests**: All critical paths covered
- **E2E Tests**: Major user scenarios validated

## 🔄 Continuous Integration

Tests are automatically run on:
- Every pull request
- Commits to main branch
- Release tags

## 📝 Writing New Tests

### Adding Unit Tests
1. Create `*_test.go` file alongside source code
2. Follow Go testing conventions
3. Use table-driven tests where appropriate
4. Mock external dependencies

### Adding Integration Tests
1. Add test cases to `integration/integration_test.go`
2. Create fixtures in `test/fixtures/`
3. Update test scripts if needed
4. Document test scenarios

### Test Fixtures Guidelines
- Use minimal resources for faster testing
- Include comments explaining test scenarios
- Name resources clearly (e.g., `test-<scenario>-pod`)
- Set appropriate labels for easy cleanup

## 🚨 Common Issues

### Metrics Server Not Available
```bash
# Install metrics-server in Minikube
minikube addons enable metrics-server
```

### RBAC Permissions
```bash
# Apply RBAC configuration
kubectl apply -f deploy/kubernetes/rbac.yaml
```

### Pod Resize Not Supported
Ensure Kubernetes version 1.27+ with in-place resize feature enabled.

## 📚 Additional Resources

- [Kubernetes Testing Best Practices](https://kubernetes.io/docs/reference/using-api/client-libraries/)
- [Go Testing Documentation](https://golang.org/pkg/testing/)
- [Minikube Documentation](https://minikube.sigs.k8s.io/docs/)
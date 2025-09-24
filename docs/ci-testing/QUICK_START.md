# Right-Sizer CI Testing Quick Start Guide

## Prerequisites

Before running CI tests locally, ensure you have the following installed:

- **Go 1.25+**: Required for running tests
- **Docker**: Required for container builds and integration tests
- **Kubernetes CLI (kubectl)**: Required for Kubernetes integration tests
- **Helm 3.x**: Required for Helm chart testing
- **Minikube** (optional): For local Kubernetes testing
- **Pre-commit**: For running pre-commit hooks

## Quick Setup

### 1. Clone the Repository

```bash
git clone https://github.com/your-org/right-sizer.git
cd right-sizer
```

### 2. Install Dependencies

```bash
# Install Go dependencies
cd go
go mod download
cd ..

# Install pre-commit hooks
pip install pre-commit
pre-commit install

# Install testing tools
go install golang.org/x/vuln/cmd/govulncheck@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### 3. Verify Installation

```bash
# Check Go version
go version

# Check Docker
docker version

# Check Kubernetes CLI
kubectl version --client

# Check Helm
helm version
```

## Running Tests - The Fast Way

### Basic Test Suite (< 1 minute)

```bash
# Run unit tests only
make test

# Or directly with Go
cd go && go test ./... && cd ..
```

### Standard Test Suite (< 5 minutes)

```bash
# Run unit tests with coverage
make test-coverage

# Run with race detection
cd go && go test -race ./... && cd ..
```

### Comprehensive Test Suite (< 10 minutes)

```bash
# Run all tests including integration
make test-all

# Or use the test script
./tests/run-all-tests.sh --coverage --integration
```

## Common Testing Scenarios

### Scenario 1: Pre-Push Testing

Before pushing code, run this minimal set:

```bash
# Format and lint check
cd go && go fmt ./... && go vet ./... && cd ..

# Run quick tests
make test

# Check for security issues
make security-scan
```

### Scenario 2: Feature Development

When developing a new feature:

```bash
# Run tests for specific package
go test -v ./controllers/...

# Run with coverage to check your tests
go test -cover ./controllers/...

# Run integration tests if needed
make test-integration
```

### Scenario 3: Debugging Test Failures

```bash
# Run specific test with verbose output
go test -v -run TestSpecificFunction ./controllers/

# Run with race detection
go test -race -v ./controllers/

# Generate coverage report to see untested code
go test -coverprofile=coverage.out ./controllers/
go tool cover -html=coverage.out
```

### Scenario 4: Pull Request Validation

Simulate CI checks locally:

```bash
# Run pre-commit hooks
pre-commit run --all-files

# Run full test suite
./tests/run-all-tests.sh -c -v -i

# Build Docker image
make docker-build

# Lint Helm chart
make helm-lint
```

## Testing with Minikube

### Quick Minikube Setup

```bash
# Start Minikube
make mk-start

# Build and deploy to Minikube
make mk-deploy

# Run tests against Minikube
make mk-test

# Check logs
make mk-logs

# Cleanup
make mk-clean
```

### Manual Minikube Testing

```bash
# Start Minikube with specific resources
minikube start --memory=4096 --cpus=2

# Build image for Minikube
eval $(minikube docker-env)
docker build -t right-sizer:test .

# Deploy to Minikube
kubectl apply -f deploy/

# Run test workloads
kubectl apply -f tests/workloads/test-deployment.yaml

# Watch the right-sizer in action
kubectl logs -f deployment/right-sizer -n right-sizer
```

## CI Testing Checklist

Use this checklist before pushing code:

- [ ] **Code Formatting**: `go fmt ./...`
- [ ] **Linting**: `golangci-lint run`
- [ ] **Unit Tests**: `go test ./...`
- [ ] **Race Detection**: `go test -race ./...`
- [ ] **Coverage**: `go test -cover ./...` (aim for >80%)
- [ ] **Security Scan**: `govulncheck ./...`
- [ ] **Build Check**: `go build ./...`
- [ ] **Docker Build**: `docker build -t test .`
- [ ] **Helm Lint**: `helm lint helm/`

## Quick Fixes for Common Issues

### Issue: Tests Timing Out

```bash
# Increase timeout
go test -timeout 30m ./...
```

### Issue: Race Condition Detected

```bash
# Debug with detailed race output
GORACE="log_path=race.log" go test -race ./...
cat race.log
```

### Issue: Low Test Coverage

```bash
# Find untested code
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
# Look for red (uncovered) sections
```

### Issue: Integration Tests Failing

```bash
# Check if services are running
kubectl get pods --all-namespaces
kubectl describe pod <pod-name>

# Check logs
kubectl logs -n right-sizer deployment/right-sizer
```

### Issue: Pre-commit Hooks Failing

```bash
# Run specific hook
pre-commit run go-fmt --all-files

# Skip hooks temporarily (not recommended)
git commit --no-verify

# Update hooks
pre-commit autoupdate
```

## Environment Variables for Testing

```bash
# Enable verbose output
export VERBOSE=true

# Enable coverage
export COVERAGE=true

# Set custom timeout
export TIMEOUT=30m

# Enable integration tests
export INTEGRATION=true

# Set parallel test count
export PARALLEL=8

# Run tests
./tests/run-all-tests.sh
```

## Test Output Examples

### Successful Test Run
```
=== RUN   TestRightSizer
--- PASS: TestRightSizer (0.05s)
=== RUN   TestRightSizer/CreatePod
--- PASS: TestRightSizer/CreatePod (0.02s)
=== RUN   TestRightSizer/UpdatePod
--- PASS: TestRightSizer/UpdatePod (0.03s)
PASS
ok      github.com/right-sizer/go/controllers  0.156s
```

### Test with Coverage
```
=== RUN   TestRightSizer
--- PASS: TestRightSizer (0.05s)
PASS
coverage: 85.3% of statements
ok      github.com/right-sizer/go/controllers  0.156s
```

### Failed Test
```
=== RUN   TestRightSizer
    rightsizer_test.go:45: expected pod to be resized, but it wasn't
--- FAIL: TestRightSizer (0.05s)
FAIL
exit status 1
FAIL    github.com/right-sizer/go/controllers  0.156s
```

## Quick Performance Check

```bash
# Run benchmarks for performance testing
go test -bench=. -benchmem ./controllers/

# Sample output:
# BenchmarkResize-8     1000    1050 ns/op    256 B/op    5 allocs/op
```

## Getting Help

### Check Test Logs
```bash
# Verbose test output
go test -v ./...

# Save test output
go test -v ./... 2>&1 | tee test.log
```

### Debug Specific Test
```bash
# Use delve debugger
dlv test ./controllers/ -- -test.run TestSpecificFunction
```

### Resources
- Run `make help` for all available commands
- Check `docs/ci-testing/README.md` for detailed guide
- Review `.github/workflows/` for CI configuration
- Join project Slack channel for support

## Next Steps

After getting familiar with quick testing:

1. Read the full [CI Testing Guide](README.md)
2. Set up your IDE for testing (see [IDE_SETUP.md](IDE_SETUP.md))
3. Learn about [advanced testing scenarios](ADVANCED_TESTING.md)
4. Contribute test improvements via pull requests

---

*Remember: Good tests are the foundation of reliable software. Test early, test often!*

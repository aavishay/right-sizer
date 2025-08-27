# Build Guide

## Quick Start

```bash
# Build the binary
./make build

# Run tests
./make test

# Build Docker image
./make docker

# Build for Minikube
./make minikube-build
```

## Prerequisites

- Go 1.22+
- Docker (for container builds)
- Kubernetes 1.33+ (for in-place resize feature)
- Helm 3.x (for Helm deployments)

## Build Commands

| Command | Description |
|---------|-------------|
| `./make build` | Build the binary |
| `./make test` | Run unit tests |
| `./make test-coverage` | Run tests with coverage report |
| `./make test-integration` | Run integration tests (requires cluster) |
| `./make test-all` | Run comprehensive test suite |
| `./make clean` | Clean build artifacts |
| `./make docker` | Build Docker image |
| `./make minikube-build` | Build Docker image in Minikube |
| `./make helm-deploy` | Deploy with Helm |
| `./make help` | Show all available commands |

## Development Workflow

```bash
# Setup development environment
./scripts/dev.sh setup

# Watch for changes and auto-rebuild
./scripts/dev.sh watch

# Run pre-commit checks
./scripts/dev.sh precommit

# Start local Minikube cluster
./scripts/dev.sh start-cluster
```

## Testing

### Test Commands

| Command | Description |
|---------|-------------|
| `./make test` | Run unit tests |
| `./make test-coverage` | Run tests with coverage report |
| `./make test-integration` | Run integration tests |
| `./make test-all` | Run all tests with coverage and benchmarks |
| `./scripts/test.sh -h` | Show detailed test options |

### Test Types

- **Unit Tests**: Fast tests for individual components
- **Integration Tests**: End-to-end tests requiring a running cluster  
- **Coverage Tests**: Generate HTML coverage reports
- **Benchmark Tests**: Performance testing

### Running Tests

```bash
# Basic unit tests
./make test

# Tests with coverage
./make test-coverage

# Integration tests (requires kubectl access)
INTEGRATION_TESTS=true ./make test-integration

# Watch mode for development
./scripts/test.sh -w
```

## Environment Variables

- `IMAGE_TAG` - Docker image tag (default: `latest`)
- `IMAGE_NAME` - Docker image name (default: `right-sizer`)
- `NAMESPACE` - Kubernetes namespace (default: `default`)

## Examples

```bash
# Build with custom image tag
IMAGE_TAG=v1.0.0 ./make docker

# Deploy to specific namespace
NAMESPACE=production ./make helm-deploy

# Run comprehensive test suite
./make test-all

# Verify deployment works
kubectl get pods -l app=right-sizer
kubectl logs -l app=right-sizer --tail=20
```

### Testing the Operator

```bash
# Deploy test workload
kubectl apply -f examples/in-place-resize-demo.yaml

# Watch for resize operations
kubectl get events --sort-by='.lastTimestamp' | grep resize

# Check pod resources before and after
kubectl get pod demo-app-xxx -o jsonpath='{.spec.containers[0].resources}'

# Verify no restarts during resize (in-place feature)
kubectl get pod demo-app-xxx -o jsonpath='{.status.containerStatuses[0].restartCount}'
```

## Minikube Development

```bash
# Start Minikube (with K8s 1.33+ for in-place resize)
minikube start --kubernetes-version=v1.33.1

# Build and deploy
./make minikube-build
./make helm-deploy

# Test deployment
helm test right-sizer
```

## Files

- `./make` - Build script wrapper
- `scripts/make.sh` - Main build script
- `scripts/dev.sh` - Development helper script
- `Dockerfile` - Container image definition
- `helm/` - Helm chart
- `test/` - Integration tests
- `*_test.go` - Unit tests
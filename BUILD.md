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
| `./make test` | Run tests |
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

# Run full test suite
./make full-test
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
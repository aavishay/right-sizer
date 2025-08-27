# Makefile to Scripts Migration Guide

## Overview

We've migrated from a traditional Makefile to a more portable shell script-based build system. This provides better cross-platform compatibility, more detailed output, and enhanced functionality.

## Migration Benefits

- ✅ **Better Portability**: Works on any system with bash (no need for make)
- ✅ **Enhanced Output**: Colored output with clear status indicators
- ✅ **More Features**: Additional commands for Minikube and Helm workflows
- ✅ **Better Help**: Comprehensive help with examples and environment variables
- ✅ **Easier Debugging**: Shell scripts are easier to debug than Makefiles

## Command Mapping

### Basic Commands

| Old Makefile Command | New Script Command | Description |
|---------------------|-------------------|-------------|
| `make build` | `./make build` | Build the binary |
| `make clean` | `./make clean` | Clean build artifacts |
| `make test` | `./make test` | Run tests |
| `make fmt` | `./make fmt` | Format code |
| `make lint` | `./make lint` | Run linters |
| `make docker` | `./make docker` | Build Docker image |
| `make run` | `./make run` | Build and run locally |

### New Commands

The script system includes several new commands not available in the Makefile:

| New Command | Description |
|------------|-------------|
| `./make minikube-build` | Build Docker image in Minikube environment |
| `./make helm-deploy` | Deploy using Helm chart |
| `./make helm-upgrade` | Upgrade Helm deployment |
| `./make helm-uninstall` | Uninstall Helm deployment |
| `./make quick-test` | Run format, test, and build |
| `./make full-test` | Run complete test suite |
| `./make version` | Show version information |
| `./make test-coverage` | Run tests with coverage report |

## Usage Examples

### Old Way (Makefile)
```bash
make build
make test
make docker
```

### New Way (Scripts)
```bash
./make build
./make test
./make docker
```

### With Environment Variables

```bash
# Old way
IMAGE_TAG=v1.0.0 make docker

# New way (same syntax)
IMAGE_TAG=v1.0.0 ./make docker

# Or export for multiple commands
export IMAGE_TAG=v1.0.0
./make docker
./make docker-push
```

### Minikube Workflow

Old way (manual):
```bash
eval $(minikube docker-env)
make docker
```

New way (automated):
```bash
./make minikube-build
```

### Helm Deployment

Old way (manual):
```bash
helm install right-sizer ./helm \
  --set image.repository=right-sizer \
  --set image.tag=latest
```

New way (automated):
```bash
./make helm-deploy

# Or with custom namespace
NAMESPACE=production ./make helm-deploy
```

## File Locations

- **Main script**: `scripts/make.sh` - The main build script with all logic
- **Wrapper**: `make` - Simple wrapper in project root for convenience
- **Old Makefile**: `Makefile.old` - Preserved for reference

## Help System

The new script includes a comprehensive help system:

```bash
./make help
# or
./make --help
# or
./make -h
# or just
./make
```

This shows:
- All available commands grouped by category
- Environment variables that can be set
- Usage examples
- Color-coded output for better readability

## Environment Variables

The script respects these environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `IMAGE_TAG` | `latest` | Docker image tag |
| `IMAGE_NAME` | `right-sizer` | Docker image name |
| `NAMESPACE` | `default` | Kubernetes namespace for Helm |
| `RELEASE_NAME` | `right-sizer` | Helm release name |
| `GO` | `go` | Go binary path |
| `DOCKER` | `docker` | Docker binary path |
| `KUBECTL` | `kubectl` | Kubectl binary path |

## Troubleshooting

### Permission Denied

If you get a permission denied error:
```bash
chmod +x make scripts/make.sh
```

### Command Not Found

Make sure you're in the project root and use `./make` (with the `./` prefix).

### Make Command Still Works

If you have muscle memory for `make`, you can create an alias:
```bash
alias make='./make'
```

## Rollback

If you need to revert to the Makefile:
```bash
mv Makefile.old Makefile
# Then use make commands as before
```

## Benefits Summary

1. **No Dependencies**: Works without `make` installed
2. **Better Error Messages**: Clear error reporting with colored output
3. **More Functionality**: Includes Minikube and Helm workflows
4. **Cross-Platform**: Works on Linux, macOS, and WSL/Git Bash on Windows
5. **Easier to Extend**: Adding new commands is straightforward
6. **Better Documentation**: Built-in help with examples

## Questions?

Run `./make help` to see all available commands and options.

---

*Migration completed: August 27, 2025*
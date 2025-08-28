# Project Structure

This document describes the organization and structure of the right-sizer operator project.

## Directory Layout

```
right-sizer/
├── go/                  # Go source code and modules
│   ├── main.go          # Application entry point
│   ├── go.mod           # Go module definition
│   ├── go.sum           # Go module checksums
│   ├── config/          # Configuration management
│   │   └── config.go    # Environment variable parsing and validation
│   ├── controllers/     # Kubernetes controller implementations
│   │   ├── adaptive_rightsizer.go      # Adaptive sizing with pod restarts
│   │   ├── deployment_rightsizer.go    # Deployment-focused controller
│   │   ├── inplace_rightsizer.go       # K8s 1.33+ in-place resizing
│   │   ├── nondisruptive_rightsizer.go # Non-disruptive resizing
│   │   └── rightsizer_controller.go    # Base controller logic
│   ├── logger/          # Logging package
│   │   └── logger.go    # Structured logging with levels
│   ├── metrics/         # Metrics collection
│   │   ├── metrics_server.go   # Kubernetes metrics-server integration
│   │   ├── prometheus.go       # Prometheus integration
│   │   └── types.go           # Common types and interfaces
│   └── test/            # Integration test resources
│       └── integration_test.go  # End-to-end tests
├── helm/                # Helm chart for Kubernetes deployment
│   ├── templates/       # Kubernetes manifest templates
│   └── values.yaml      # Default configuration values
├── deploy/              # Deployment resources
│   ├── kubernetes/      # Raw Kubernetes manifests
│   │   ├── deployment.yaml  # Operator deployment
│   │   └── rbac.yaml       # RBAC configuration
│   └── scripts/         # Deployment automation scripts
│       └── deploy.sh    # Quick deployment script
├── docs/                # Additional documentation
│   ├── KUBERNETES-1.33-UPGRADE.md  # K8s 1.33+ features guide
│   ├── LICENSE-SUMMARY.md          # License quick reference
│   └── MINIKUBE-TESTING.md         # Minikube testing guide
├── examples/            # Example configurations
│   ├── config-scenarios.yaml       # Various configuration examples
│   ├── full-configuration.yaml     # Complete config reference
│   └── in-place-resize-demo.yaml   # In-place resize demonstration
├── scripts/             # Build and deployment scripts
│   ├── make.sh          # Main build script (operates on go/ directory)
│   ├── test.sh          # Comprehensive test script
│   └── dev.sh           # Development helper script
├── test/                # Test resources (deprecated, moved to go/test/)
├── .dockerignore        # Docker build exclusions
├── .gitignore          # Git exclusions
├── BUILD.md            # Build and development guide
├── CONFIGURATION.md     # Configuration guide
├── CONTRIBUTING.md      # Contribution guidelines
├── COPYRIGHT           # Copyright notice
├── Dockerfile          # Container image definition (builds from go/)
├── make                # Build script wrapper (./make <command>)
├── LICENSE             # AGPL-3.0 license text
├── NOTICE              # Third-party notices
├── PROJECT-STRUCTURE.md # This file
└── README.md           # Project overview
```

## Core Components

### `/go`
Main Go source code directory containing all Go modules and packages:
- **Root**: `main.go`, `go.mod`, `go.sum` - Application entry point and dependencies
- **Packages**: All Go packages organized in subdirectories

### `/go/config`
Configuration management package that handles:
- Environment variable parsing (including CSV for namespace include/exclude)
- Default value management
- Configuration validation
- Global configuration singleton

### `/go/controllers`
Kubernetes controller implementations with different strategies:
- **Adaptive**: Traditional approach with potential pod restarts
- **In-Place**: Kubernetes 1.33+ native resize support
- **Non-Disruptive**: Minimizes service interruption
- **Deployment**: Focuses on deployment-level management

### `/go/logger`
Custom logging implementation featuring:
- Multiple log levels (debug, info, warn, error)
- Color-coded output for terminals
- Structured logging format
- Context-aware prefixes

### `/go/metrics`
Metrics collection supporting multiple backends:
- Kubernetes metrics-server (default)
- Prometheus integration
- Common interface for extensibility

### `/go/test`
Integration tests and test utilities:
- End-to-end operator testing
- Kubernetes cluster integration
- Test data and mock implementations

## Deployment Options

### Kubernetes Manifests
Raw Kubernetes YAML files in `/deploy/kubernetes/`:
```bash
kubectl apply -f deploy/kubernetes/
```

### Helm Chart
Production-ready Helm chart in `/helm/`:
```bash
helm install right-sizer ./helm
```

### Docker Image
Build from Dockerfile (builds from `go/` directory):
```bash
make docker
```

## Development Workflow

### Building
```bash
make build              # Build binary (from go/ directory)
make docker            # Build Docker image (from go/ directory)  
make clean             # Clean artifacts (including go/vendor, go/coverage.*)
```

### Testing
```bash
make test              # Run unit tests (in go/ directory)
make test-coverage     # Generate coverage report (go/coverage.html)
make test-integration  # Run integration tests
make test-all          # Run comprehensive test suite
```

### Code Quality
```bash
make fmt               # Format code
make lint              # Run linters
```

## Configuration

The operator is configured via environment variables:

| Category | Variables |
|----------|-----------|
| **Resource Multipliers** | `CPU_REQUEST_MULTIPLIER`, `MEMORY_REQUEST_MULTIPLIER`, `CPU_LIMIT_MULTIPLIER`, `MEMORY_LIMIT_MULTIPLIER` |
| **Resource Boundaries** | `MAX_CPU_LIMIT`, `MAX_MEMORY_LIMIT`, `MIN_CPU_REQUEST`, `MIN_MEMORY_REQUEST` |
| **Operational** | `RESIZE_INTERVAL`, `LOG_LEVEL`, `METRICS_PROVIDER`, `DRY_RUN` |
| **Feature Flags** | `ENABLE_INPLACE_RESIZE` |

See [CONFIGURATION.md](CONFIGURATION.md) for complete details.

## Testing Structure

### `/go/test`
Integration tests located within the Go module:
- End-to-end operator testing
- Kubernetes cluster integration tests
- Test utilities and mock implementations
- Benchmark tests for performance validation

### `/scripts`
Build and development scripts:
- **make.sh**: Main build script (operates on go/ directory)
- **test.sh**: Comprehensive test script with coverage and integration options
- **dev.sh**: Development helper script for common workflows

## Examples

The `/examples` directory contains:
- **config-scenarios.yaml**: Different configuration patterns
- **full-configuration.yaml**: Complete reference with all options
- **in-place-resize-demo.yaml**: Kubernetes 1.33+ feature demonstration

## Documentation

| File | Purpose |
|------|---------|
| `README.md` | Project overview and quick start |
| `BUILD.md` | Build and development guide |
| `CONFIGURATION.md` | Detailed configuration guide |
| `CONTRIBUTING.md` | Contribution guidelines and development setup |
| `PROJECT-STRUCTURE.md` | This file - project organization |
| `docs/KUBERNETES-1.33-UPGRADE.md` | Kubernetes 1.33+ features |
| `docs/LICENSE-SUMMARY.md` | AGPL-3.0 license summary |
| `docs/MINIKUBE-TESTING.md` | Local testing with Minikube |

## License

This project is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0).
- License text: [LICENSE](LICENSE)
- Copyright: [COPYRIGHT](COPYRIGHT)
- Third-party notices: [NOTICE](NOTICE)

## Conventions

### File Naming
- Go files: `snake_case.go`
- YAML files: `kebab-case.yaml`
- Scripts: `kebab-case.sh`
- Documentation: `UPPER-CASE.md` for top-level, `lower-case.md` for subdirectories

### Code Organization
- All Go code located in `go/` directory
- One package per directory within `go/`
- Interfaces defined in `types.go`
- Implementation in descriptive files
- Tests alongside implementation (e.g., `go/controllers/rightsizer_test.go`)
- Integration tests in `go/test/`

### Git Workflow
- Feature branches: `feature/description`
- Bug fixes: `fix/description`
- Documentation: `docs/description`
- Commit messages follow conventional commits

## Getting Started

1. **Clone the repository**
   ```bash
   git clone https://github.com/yourusername/right-sizer.git
   cd right-sizer
   ```

2. **Build the project** (builds from `go/` directory)
   ```bash
   make build
   ```

3. **Run tests** (operates on `go/` directory)
   ```bash
   make test
   ```

4. **Deploy to Kubernetes**
   ```bash
   make helm-deploy
   ```

For detailed instructions, see [README.md](README.md).

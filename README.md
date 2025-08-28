# right-sizer Operator

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

A Kubernetes operator for automatic pod resource right-sizing with support for Kubernetes 1.33+ in-place pod resizing.

## Key Features

- **In-Place Pod Resizing (Kubernetes 1.33+)**: Dynamically adjust pod resources without restarts using the new resize subresource
- **Automatic Right-Sizing**: Continuously monitors and adjusts pod resources based on actual usage
- **Multiple Strategies**: Supports various right-sizing strategies including adaptive, non-disruptive, and in-place
- **Metrics-Driven**: Uses Prometheus metrics to make informed sizing decisions

---

## Helm Chart Usage

You can deploy the right-sizer operator using the provided Helm chart.

### Install

```sh
helm install right-sizer ./helm
```

### Upgrade

```sh
helm upgrade right-sizer ./helm
```

### Uninstall

```sh
helm uninstall right-sizer
```

### Configuration

You can override values in `values.yaml` using `--set` or by editing the file.

Example:

```sh
helm install right-sizer ./helm \
  --set image.repository=myrepo/right-sizer \
  --set image.tag=v1.2.3 \
  --set prometheusUrl=http://prometheus.mynamespace.svc:9090
```

### Helm Values

| Name                | Description                       | Default                                  |
|---------------------|-----------------------------------|------------------------------------------|
| `image.repository`  | Docker image repository           | `right-sizer`                            |
| `image.tag`         | Docker image tag                  | `latest`                                 |
| `image.pullPolicy`  | Image pull policy                 | `Always`                                 |
| `prometheusUrl`     | Prometheus endpoint for metrics   | `http://prometheus:9090`                 |
| `namespaceInclude`  | Comma-separated list of namespaces to include (e.g., `default,kube-system`) | `default` |
| `namespaceExclude`  | Comma-separated list of namespaces to exclude (e.g., `test,dev`) | `kube-system` |
| `resources.requests.cpu`    | Pod CPU request            | `100m`                                   |
| `resources.requests.memory` | Pod memory request         | `128Mi`                                  |
| `resources.limits.cpu`      | Pod CPU limit              | `500m`                                   |
| `resources.limits.memory`   | Pod memory limit           | `512Mi`                                  |

---

## Kubernetes 1.33+ In-Place Resize Support

Starting with Kubernetes 1.33, the operator uses the native resize subresource to perform true in-place pod resizing without restarts. This feature allows for:

- **Zero-Downtime Updates**: Resources are adjusted while pods continue running
- **Immediate Application**: Changes take effect without pod recreation
- **Improved Efficiency**: No disruption to running workloads

### Requirements

- Kubernetes 1.33 or later
- kubectl 1.33+ for manual operations
- In-place resize feature gate enabled (enabled by default in 1.33+)

### How It Works

The operator uses the Kubernetes resize subresource API:

```bash
kubectl patch pod <pod-name> --subresource resize --patch '{"spec": {"containers": [{"name": "<container>", "resources": {...}}]}}'
```

The operator automatically detects if your cluster supports this feature and will use it when available, falling back to traditional methods on older clusters.

---

## Agent Mode

The right-sizer operator runs in agent mode by default when deployed as a Kubernetes Deployment (via manifests or Helm). In agent mode, the operator continuously watches pods and their parent controllers, periodically fetching metrics and updating resource requests/limits in the background.

**No manual intervention is needed after deployment.**

### Customizing Agent Interval

To change how often the operator reconciles (e.g., every 5 minutes instead of 10), update the `Interval` value in your controller configuration:

```go
rightsizer := &InPlaceRightSizer{
    Client:          mgr.GetClient(),
    MetricsProvider: provider,
    Interval:        5 * time.Minute, // Adjust this value
}
```

---

## Configuration

The right-sizer operator can be configured using environment variables to customize resource sizing behavior:

### Resource Multipliers

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `CPU_REQUEST_MULTIPLIER` | `1.2` | Multiplier applied to CPU usage to calculate CPU requests |
| `MEMORY_REQUEST_MULTIPLIER` | `1.2` | Multiplier applied to memory usage to calculate memory requests |
| `CPU_LIMIT_MULTIPLIER` | `2.0` | Multiplier applied to CPU requests to calculate CPU limits |
| `MEMORY_LIMIT_MULTIPLIER` | `2.0` | Multiplier applied to memory requests to calculate memory limits |
| `KUBE_NAMESPACE_INCLUDE` | (empty) | Comma-separated list of namespaces to monitor. Default: monitors all namespaces |
| `KUBE_NAMESPACE_EXCLUDE` | (empty) | Comma-separated list of namespaces to exclude. Default: excludes none |
### Namespace Filtering

You can restrict monitoring to specific namespaces using:

- `KUBE_NAMESPACE_INCLUDE`: Only monitor pods in these namespaces (CSV).
- `KUBE_NAMESPACE_EXCLUDE`: Exclude pods in these namespaces (CSV).

If both are set, only namespaces in the include list are monitored.

### Resource Boundaries

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `MAX_CPU_LIMIT` | `4000` | Maximum CPU limit in millicores (4000 = 4 cores) |
| `MAX_MEMORY_LIMIT` | `8192` | Maximum memory limit in MB (8192 = 8GB) |
| `MIN_CPU_REQUEST` | `10` | Minimum CPU request in millicores |
| `MIN_MEMORY_REQUEST` | `64` | Minimum memory request in MB |

### Other Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `METRICS_PROVIDER` | `kubernetes` | Metrics source: `kubernetes` (metrics-server) or `prometheus` |
| `PROMETHEUS_URL` | `http://prometheus:9090` | Prometheus URL (when using Prometheus provider) |
| `ENABLE_INPLACE_RESIZE` | `true` | Enable in-place pod resizing for Kubernetes 1.33+ |
| `DRY_RUN` | `false` | If `true`, only log recommendations without applying changes |
| `RESIZE_INTERVAL` | `30s` | How often to check and resize resources (e.g., `30s`, `1m`, `5m`, `1h`) |
| `LOG_LEVEL` | `info` | Logging verbosity: `debug`, `info`, `warn`, `error` |

For detailed configuration options, see [CONFIGURATION.md](CONFIGURATION.md).

## Quick Start

### Prerequisites

- Go 1.22+
- Docker
- Kubernetes 1.33+ (for in-place resize feature)
- Minikube (for local development)

### Local Development

**Note**: All Go source code is located in the `go/` directory.

#### 1. Setup Environment

```bash
# Start Minikube with Kubernetes 1.33+ for in-place resize support
minikube start --kubernetes-version=v1.33.1

# Optional: Use development helper script
./scripts/dev.sh setup
```

#### 2. Build and Deploy

```bash
# Build the operator binary (builds from go/ directory)
./make build

# Build Docker image in Minikube's registry
./make minikube-build

# Deploy using Helm
./make helm-deploy
```

#### 3. Verify Deployment

```bash
# Check operator status
kubectl get pods -l app=right-sizer
kubectl logs -l app=right-sizer -f

# Test with sample workload
kubectl apply -f examples/sample-deployment.yaml

# Watch resize operations
kubectl get events --sort-by='.lastTimestamp' | grep resize
```

#### 4. Development Workflow

```bash
# Auto-rebuild on changes
./scripts/dev.sh watch

# Run tests
./make test

# Clean rebuild
./make clean && ./make build
```

For detailed build instructions, see [BUILD.md](BUILD.md).

**Project Structure**: Go source code is organized in the `go/` directory, with build scripts operating from the project root.

## Documentation

- [Configuration Guide](CONFIGURATION.md) - All configuration options
- [Build Guide](BUILD.md) - Build and deployment instructions
- [Contributing](CONTRIBUTING.md) - How to contribute
- [Project Structure](PROJECT-STRUCTURE.md) - Code organization

## License

This project is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**. See [LICENSE](LICENSE) for details.
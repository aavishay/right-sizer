# right-sizer Operator

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

A Kubernetes operator for automatic pod resource right-sizing with support for Kubernetes 1.33+ in-place pod resizing.

## Key Features

- **In-Place Pod Resizing (Kubernetes 1.33+)**: Dynamically adjust pod resources without restarts using the new resize subresource
- **Automatic Right-Sizing**: Continuously monitors and adjusts pod resources based on actual usage
- **Multiple Strategies**: Supports various right-sizing strategies including adaptive, non-disruptive, and in-place
- **Metrics-Driven**: Uses Prometheus metrics to make informed sizing decisions

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

### Example Deployment Configuration

```yaml
env:
  # Configure operational settings
  - name: RESIZE_INTERVAL
    value: "1m"      # Check every minute
  - name: LOG_LEVEL
    value: "info"    # Set logging level
  # Configure resource multipliers
  - name: CPU_REQUEST_MULTIPLIER
    value: "1.5"
  - name: MEMORY_REQUEST_MULTIPLIER
    value: "1.3"
```

## License

This project is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**.

The AGPL is a copyleft license that requires anyone who distributes the software or runs it on a server to provide the source code to users, including for any modifications. This ensures that improvements and modifications to the right-sizer operator remain open source and benefit the community.

For more details, see the [LICENSE](LICENSE) file or visit [https://www.gnu.org/licenses/agpl-3.0.html](https://www.gnu.org/licenses/agpl-3.0.html).

### Key License Terms

- ✅ **Commercial use**: You can use this software commercially
- ✅ **Modification**: You can modify the software
- ✅ **Distribution**: You can distribute the software
- ✅ **Patent use**: Provides an express grant of patent rights from contributors
- ⚠️ **Network use is distribution**: If you run a modified version on a server, you must provide the source code
- ⚠️ **Same license**: Modifications must be released under the same AGPL-3.0 license
- ⚠️ **Disclose source**: Source code must be made available when distributing

### Contributing

By contributing to this project, you agree to license your contributions under the AGPL-3.0 license.
    value: "debug"   # More verbose logging
  # Use more conservative multipliers
  - name: CPU_REQUEST_MULTIPLIER
    value: "1.5"
  - name: MEMORY_REQUEST_MULTIPLIER
    value: "1.3"
  # Allow higher burst capacity
  - name: CPU_LIMIT_MULTIPLIER
    value: "3.0"
  - name: MEMORY_LIMIT_MULTIPLIER
    value: "2.5"
  # Set higher maximums for resource-intensive workloads
  - name: MAX_CPU_LIMIT
    value: "8000"  # 8 cores
  - name: MAX_MEMORY_LIMIT
    value: "16384" # 16GB
```

### Using Helm

When deploying with Helm, configure these values in your `values.yaml` or via `--set`:

```bash
helm install right-sizer ./helm \
  --set resizeInterval=1m \
  --set logLevel=debug \
  --set config.cpuRequestMultiplier=1.5 \
  --set config.memoryRequestMultiplier=1.3 \
  --set config.maxCpuLimit=8000 \
  --set config.maxMemoryLimit=16384
```

### Verifying In-Place Resize

To verify that pods are being resized without restarts:

```bash
# Check pod restart count before resize
kubectl get pod <pod-name> -o jsonpath='{.status.containerStatuses[0].restartCount}'

# Watch operator logs
kubectl logs -l app=right-sizer -f

# After resize, check restart count again (should be unchanged)
kubectl get pod <pod-name> -o jsonpath='{.status.containerStatuses[0].restartCount}'

# View pod events to see resize operations
kubectl describe pod <pod-name>
```

---

## Local Development & Minikube Workflow

This guide walks you through compiling, building, deploying, and checking the status of the right-sizer operator on Minikube.

---

### 1. Start and Connect to Minikube

```sh
# Start Minikube with Kubernetes 1.33+ for in-place resize support
minikube start --kubernetes-version=v1.33.1
kubectl config use-context minikube
```

---

### 2. Compile the Operator (Go)

From the project root:

```sh
./make build
```

---

### 3. Build the Docker Image (inside Minikube)

Use Minikube’s Docker daemon so the image is available inside the cluster:

```sh
./make minikube-build
```

---

### 4. Deploy to Minikube

#### a. Using Kubernetes Manifests

Update your deployment manifest to use `right-sizer:latest` (no Docker Hub prefix):

```yaml
# deployment.yaml
...
image: right-sizer:latest
imagePullPolicy: IfNotPresent
...
```

Apply RBAC and deployment:

```sh
kubectl apply -f rbac.yaml
kubectl apply -f deployment.yaml
```

#### b. Using Helm

```sh
helm install right-sizer ./helm \
  --set image.repository=right-sizer \
  --set image.tag=latest \
  --set image.pullPolicy=IfNotPresent
```

---

### 5. Check Operator Status

```sh
kubectl get deployments
kubectl get pods
kubectl describe pod -l app=right-sizer
kubectl logs -l app=right-sizer
```

---

## Troubleshooting

- If the pod image cannot be pulled, ensure you built the image inside Minikube's Docker daemon (`eval $(minikube docker-env)`).
- Check logs for errors: `kubectl logs -l app=right-sizer`
- Make sure RBAC permissions are applied before deploying the operator.
- For in-place resize issues:
  - Verify Kubernetes version: `kubectl version` (must be 1.33+)
  - Check if resize subresource is available: `kubectl api-resources | grep pods/resize`
  - Review pod events for resize operations: `kubectl describe pod <pod-name>`
  - Ensure pods don't have `restartPolicy: Always` in resize policy if you want zero-downtime updates

---

## Useful Links

- [Minikube Documentation](https://minikube.sigs.k8s.io/docs/)
- [Helm Documentation](https://helm.sh/docs/)
- [Kubernetes RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
- [Kubernetes 1.33 In-Place Pod Resize](https://kubernetes.io/blog/2025/05/16/kubernetes-v1-33-in-place-pod-resize-beta/)
- [Dynamic Pod Resizing Without Restarts](https://medium.com/@anbu.gn/kubernetes-1-33-dynamic-pod-resizing-without-restarts-2ece42f0c193)

---
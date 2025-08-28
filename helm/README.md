# right-sizer Helm Chart

A Kubernetes operator for automatic pod resource right-sizing.

## Usage

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

## Configuration

You can override values in `values.yaml` using `--set` or by editing the file.

Example:

```sh
helm install right-sizer ./helm \
  --set image.repository=myrepo/right-sizer \
  --set image.tag=v1.2.3 \
  --set prometheusUrl=http://prometheus.mynamespace.svc:9090
```

## Values

| Name                | Description                       | Default                                  |
|---------------------|-----------------------------------|------------------------------------------|
| `image.repository`  | Docker image repository           | `your-dockerhub-username/right-sizer`    |
| `image.tag`         | Docker image tag                  | `latest`                                 |
| `image.pullPolicy`  | Image pull policy                 | `Always`                                 |
| `prometheusUrl`     | Prometheus endpoint for metrics   | `http://prometheus:9090`                 |
| `namespaceInclude`  | Comma-separated namespaces to include | `default`                           |
| `namespaceExclude`  | Comma-separated namespaces to exclude | `kube-system`                        |
| `resources.requests.cpu`    | Pod CPU request            | `100m`                                   |
| `resources.requests.memory` | Pod memory request         | `128Mi`                                  |
| `resources.limits.cpu`      | Pod CPU limit              | `500m`                                   |
| `resources.limits.memory`   | Pod memory limit           | `512Mi`                                  |

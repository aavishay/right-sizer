# 🚀 Right-Sizer Minikube Deployment Guide

This guide provides step-by-step instructions for deploying all Right-Sizer components to a local Minikube cluster.

## 📋 Prerequisites

Before starting, ensure you have the following tools installed:

- **Minikube** (v1.30+): [Installation Guide](https://minikube.sigs.k8s.io/docs/start/)
- **kubectl** (v1.25+): [Installation Guide](https://kubernetes.io/docs/tasks/tools/)
- **Docker** (v20.10+): [Installation Guide](https://docs.docker.com/get-docker/)
- **Helm** (v3.0+): [Installation Guide](https://helm.sh/docs/intro/install/)
- **Git**: For cloning the repository

### System Requirements

- **Memory**: Minimum 4GB RAM available for Minikube
- **CPU**: Minimum 2 CPU cores
- **Disk Space**: At least 20GB free space

## 🎯 Quick Start

For the fastest deployment, use our automated script:

```bash
# Clone the repository
git clone https://github.com/aavishay/right-sizer.git
cd right-sizer

# Run the deployment script
./deploy-minikube.sh
```

The script will:
- ✅ Check prerequisites
- ✅ Create/configure Minikube cluster with Kubernetes 1.33
- ✅ Enable in-place pod resizing feature
- ✅ Install metrics server
- ✅ Build and load Docker images
- ✅ Deploy Right-Sizer operator
- ✅ Deploy Right-Sizer dashboard (from separate repository)
- ✅ Create test workloads
- ✅ Set up port forwarding

## 📖 Manual Deployment Steps

If you prefer manual deployment or need to customize the installation:

### 1. Start Minikube Cluster

```bash
# Create a new Minikube cluster with in-place resizing enabled
minikube start \
  --profile right-sizer-cluster \
  --kubernetes-version=v1.33.1 \
  --memory=4096 \
  --cpus=2 \
  --driver=docker \
  --feature-gates=InPlacePodVerticalScaling=true \
  --extra-config=kubelet.feature-gates=InPlacePodVerticalScaling=true \
  --extra-config=kube-apiserver.feature-gates=InPlacePodVerticalScaling=true

# Set kubectl context
kubectl config use-context right-sizer-cluster
```

### 2. Enable Metrics Server

```bash
# Enable metrics-server addon
minikube addons enable metrics-server -p right-sizer-cluster

# Verify metrics server is running
kubectl wait --for=condition=available --timeout=300s \
  deployment/metrics-server -n kube-system
```

### 3. Build and Load Docker Images

```bash
# Configure Docker to use Minikube's Docker daemon
eval $(minikube -p right-sizer-cluster docker-env)

# Build Right-Sizer operator image
cd right-sizer
docker build -t right-sizer:latest -f Dockerfile .

# Build Dashboard image (from separate repository)
# Clone the dashboard repository if not already present
if [ ! -d "../right-sizer-dashboard" ]; then
  git clone https://github.com/your-org/right-sizer-dashboard.git ../right-sizer-dashboard
fi
cd ../right-sizer-dashboard
docker build -t right-sizer-dashboard:latest -f Dockerfile .
cd ../right-sizer
```

### 4. Create Namespace

```bash
# Create namespace
kubectl create namespace right-sizer

# Label namespace
kubectl label namespace right-sizer \
  monitoring=enabled \
  right-sizer=enabled
```

### 5. Deploy Right-Sizer Operator

```bash
# Install using Helm
helm install right-sizer ./helm \
  --namespace right-sizer \
  --set image.repository=right-sizer \
  --set image.tag=latest \
  --set image.pullPolicy=IfNotPresent \
  --wait

# Verify deployment
kubectl get deployment right-sizer -n right-sizer
kubectl logs -f deployment/right-sizer -n right-sizer
```

### 6. Deploy Dashboard

The dashboard is maintained in a separate repository: [right-sizer-dashboard](https://github.com/your-org/right-sizer-dashboard)

```bash
# Option 1: Use pre-built deployment manifest from dashboard repo
kubectl apply -f https://raw.githubusercontent.com/your-org/right-sizer-dashboard/main/deploy/kubernetes/dashboard-minikube.yaml

# Option 2: Build and deploy locally
# Clone dashboard repository
git clone https://github.com/your-org/right-sizer-dashboard.git ../right-sizer-dashboard
cd ../right-sizer-dashboard

# Build dashboard image
eval $(minikube -p right-sizer-cluster docker-env)
docker build -t right-sizer-dashboard:latest -f Dockerfile .

# Deploy to cluster
kubectl apply -f deploy/kubernetes/dashboard-minikube.yaml

cd ../right-sizer
```

### 7. Deploy Test Workloads

```bash
# Apply test workloads
kubectl apply -f examples/test-workloads.yaml

# Or create a simple test deployment
kubectl create deployment nginx-test \
  --image=nginx:alpine \
  --replicas=3 \
  -n default

kubectl set resources deployment nginx-test \
  --requests=cpu=100m,memory=128Mi \
  --limits=cpu=500m,memory=512Mi \
  -n default
```

## 🌐 Accessing Components

### Dashboard Access

```bash
# Port-forward for local access
kubectl port-forward -n right-sizer service/right-sizer-dashboard 3000:80
# Access at: http://localhost:3000

# For more dashboard configuration options, see:
# https://github.com/your-org/right-sizer-dashboard/blob/main/deploy/README.md
```

**Option 2: NodePort**
```bash
# Get Minikube IP
minikube ip -p right-sizer-cluster
# Access at: http://<MINIKUBE_IP>:30080
```

**Option 3: Minikube Service**
```bash
minikube service right-sizer-dashboard -n right-sizer -p right-sizer-cluster
```

### Metrics Endpoint

```bash
kubectl port-forward -n right-sizer service/right-sizer 8081:8081
# Access at: http://localhost:8081/metrics
```

## 🔍 Verification & Testing

### 1. Check Component Status

```bash
# Check all components
kubectl get all -n right-sizer

# Check operator logs
kubectl logs -f deployment/right-sizer -n right-sizer

# Check dashboard logs (if deployed)
kubectl logs -f deployment/right-sizer-dashboard -n right-sizer

# Check CRDs
kubectl get crd | grep rightsizer

# For dashboard troubleshooting, see:
# https://github.com/your-org/right-sizer-dashboard/blob/main/deploy/README.md#troubleshooting
```

### 2. Monitor Resource Optimization

```bash
# Watch pod resources
kubectl top pods -A --watch

# View optimization events
kubectl get events -A | grep right-sizer

# Check specific pod resources
kubectl describe pod <pod-name> -n <namespace> | grep -A 5 "Resources:"
```

### 3. Test In-Place Resizing

```bash
# Create a test pod
kubectl run test-pod --image=nginx:alpine \
  --requests=cpu=100m,memory=128Mi \
  --limits=cpu=200m,memory=256Mi

# Wait for optimization (usually 30-60 seconds)
sleep 60

# Check if resources were adjusted
kubectl get pod test-pod -o jsonpath='{.spec.containers[0].resources}'
```

## 📊 Dashboard Features

Once deployed, the dashboard provides:

- **Real-time Metrics**: CPU and memory usage visualization
- **Optimization Events**: History of resource adjustments
- **Savings Analysis**: Resource and cost savings metrics
- **Namespace Overview**: Per-namespace optimization status
- **Workload Insights**: Individual workload optimization details

## 🛠️ Configuration Options

### Customize Optimization Behavior

Create a custom configuration:

```yaml
# right-sizer-config.yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerConfig
metadata:
  name: default
  namespace: right-sizer
spec:
  enabled: true
  mode: balanced  # Options: aggressive, balanced, conservative
  dryRun: false
  resizeInterval: "30s"
  defaultResourceStrategy:
    cpu:
      requestMultiplier: 1.1
      limitMultiplier: 1.5
      minRequest: "10m"
      maxLimit: "2000m"
    memory:
      requestMultiplier: 1.1
      limitMultiplier: 1.3
      minRequest: "32Mi"
      maxLimit: "4Gi"
```

Apply configuration:
```bash
kubectl apply -f right-sizer-config.yaml
```

### Namespace-Specific Policies

```yaml
# namespace-policy.yaml
apiVersion: rightsizer.io/v1alpha1
kind: RightSizerPolicy
metadata:
  name: production-policy
  namespace: right-sizer
spec:
  enabled: true
  priority: 10
  mode: conservative
  targetRef:
    namespaces: ["production"]
  resourceStrategy:
    cpu:
      requestMultiplier: 1.2
      targetUtilization: 70
    memory:
      requestMultiplier: 1.2
      targetUtilization: 80
```

## 🧹 Cleanup

To remove the deployment:

```bash
# Delete Right-Sizer components
helm uninstall right-sizer -n right-sizer
kubectl delete namespace right-sizer

# Delete test workloads
kubectl delete namespace test-workloads

# Stop Minikube cluster
minikube stop -p right-sizer-cluster

# Delete Minikube cluster (optional)
minikube delete -p right-sizer-cluster
```

## 🐛 Troubleshooting

### Common Issues

#### 1. Metrics Server Not Working
```bash
# Restart metrics server
kubectl rollout restart deployment/metrics-server -n kube-system

# Check metrics server logs
kubectl logs -f deployment/metrics-server -n kube-system
```

#### 2. Pod Resizing Not Happening
- Verify Kubernetes version is 1.33+
- Check if InPlacePodVerticalScaling feature gate is enabled
- Review operator logs for errors
- Ensure pods have resource requests/limits defined

#### 3. Dashboard Not Loading
- Check dashboard pod status
- Verify port forwarding is active
- Check browser console for errors
- Ensure metrics endpoint is accessible

#### 4. Images Not Found
- Verify Docker daemon is using Minikube's context
- Rebuild images with correct tags
- Check image pull policy is set to IfNotPresent

### Debug Commands

```bash
# Get detailed component status
kubectl describe deployment right-sizer -n right-sizer

# Check events for errors
kubectl get events -n right-sizer --sort-by='.lastTimestamp'

# View resource usage
kubectl top nodes
kubectl top pods -A

# SSH into Minikube VM
minikube ssh -p right-sizer-cluster

# Check Docker images inside Minikube
minikube ssh -p right-sizer-cluster -- docker images
```

## 📚 Additional Resources

- [Right-Sizer Documentation](../README.md)
- [Configuration Guide](./CONFIGURATION.md)
- [API Documentation](./api/openapi.yaml)
- [Troubleshooting Guide](./TROUBLESHOOTING.md)
- [Examples](../examples/)

## 💡 Tips for Local Development

1. **Use watch mode for logs**: `kubectl logs -f deployment/right-sizer -n right-sizer`
2. **Enable verbose logging**: Set `logLevel: debug` in values.yaml
3. **Test with different workloads**: Use the examples in `/examples` directory
4. **Monitor metrics**: Keep Prometheus metrics endpoint open for debugging
5. **Use dry-run mode**: Test configuration changes with `dryRun: true`

## 🤝 Support

If you encounter issues:

1. Check the [FAQ](./FAQ.md)
2. Review [GitHub Issues](https://github.com/aavishay/right-sizer/issues)
3. Join our [Community Slack](#)
4. Contact maintainers at maintainers@right-sizer.dev

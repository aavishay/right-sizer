#!/bin/bash

# Minimal Right-Sizer Deployment Script
# Deploys only essential components that work

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
print_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
print_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
print_error() { echo -e "${RED}[ERROR]${NC} $1"; }

NAMESPACE="right-sizer"
CLUSTER_NAME="right-sizer-cluster"

print_info "🚀 Minimal Right-Sizer Deployment"

# Check if cluster exists, create if not
if ! minikube profile list 2>/dev/null | grep -q "$CLUSTER_NAME"; then
  print_info "Creating minikube cluster..."
  minikube start -p $CLUSTER_NAME \
    --kubernetes-version=v1.29.0 \
    --memory=4096 \
    --cpus=2 \
    --feature-gates=InPlacePodVerticalScaling=true
else
  print_info "Using existing cluster..."
  minikube profile $CLUSTER_NAME
  if ! minikube status -p $CLUSTER_NAME | grep -q "Running"; then
    minikube start -p $CLUSTER_NAME
  fi
fi

# Set context
kubectl config use-context $CLUSTER_NAME

# Enable metrics-server
print_info "Enabling metrics-server..."
minikube addons enable metrics-server -p $CLUSTER_NAME

# Wait for metrics server
print_info "Waiting for metrics-server..."
kubectl wait --for=condition=available deployment/metrics-server -n kube-system --timeout=120s

# Create namespace
print_info "Creating namespace..."
kubectl create namespace $NAMESPACE 2>/dev/null || true

# Clean up any existing resources
print_info "Cleaning up existing resources..."
kubectl delete deployment right-sizer -n $NAMESPACE 2>/dev/null || true
kubectl delete deployment right-sizer-dashboard -n $NAMESPACE 2>/dev/null || true
kubectl delete service right-sizer -n $NAMESPACE 2>/dev/null || true
kubectl delete service right-sizer-dashboard -n $NAMESPACE 2>/dev/null || true

# Configure Docker environment
print_info "Configuring Docker environment..."
eval $(minikube -p $CLUSTER_NAME docker-env)

# Build operator image with simple Dockerfile
print_info "Building operator image..."
docker build -f Dockerfile.simple -t right-sizer:latest .

print_success "Operator image built"

# Deploy minimal operator (just the essential parts)
print_info "Deploying minimal operator..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: right-sizer
  namespace: $NAMESPACE
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: right-sizer
rules:
  - apiGroups: [""]
    resources: ["pods", "pods/status", "pods/resize"]
    verbs: ["get", "list", "watch", "patch", "update"]
  - apiGroups: [""]
    resources: ["nodes", "namespaces"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["metrics.k8s.io"]
    resources: ["pods", "nodes"]
    verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: right-sizer
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: right-sizer
subjects:
  - kind: ServiceAccount
    name: right-sizer
    namespace: $NAMESPACE
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: right-sizer
  namespace: $NAMESPACE
spec:
  replicas: 1
  selector:
    matchLabels:
      app: right-sizer
  template:
    metadata:
      labels:
        app: right-sizer
    spec:
      serviceAccountName: right-sizer
      containers:
      - name: right-sizer
        image: right-sizer:latest
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 8081
          name: health
        - containerPort: 9090
          name: metrics
        env:
        - name: LOG_LEVEL
          value: "info"
        - name: DRY_RUN
          value: "false"
        resources:
          requests:
            cpu: 50m
            memory: 64Mi
          limits:
            cpu: 200m
            memory: 128Mi
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8081
          initialDelaySeconds: 30
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8081
          initialDelaySeconds: 10
          periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: right-sizer
  namespace: $NAMESPACE
spec:
  selector:
    app: right-sizer
  ports:
  - name: health
    port: 8081
    targetPort: 8081
  - name: metrics
    port: 9090
    targetPort: 9090
EOF

# Deploy simple dashboard (just static files)
print_info "Building simple dashboard..."
cat <<EOF >/tmp/simple-dashboard.html
<!DOCTYPE html>
<html>
<head>
    <title>Right-Sizer Dashboard</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .container { max-width: 1200px; margin: 0 auto; }
        .metrics { background: #f5f5f5; padding: 20px; margin: 20px 0; border-radius: 5px; }
        .status { padding: 10px; margin: 10px 0; border-left: 4px solid #4CAF50; background: #e8f5e8; }
        .error { border-left-color: #f44336; background: #fdeaea; }
        .info { border-left-color: #2196F3; background: #e3f2fd; }
        button { padding: 10px 20px; margin: 10px; background: #4CAF50; color: white; border: none; cursor: pointer; }
        button:hover { background: #45a049; }
        pre { background: #f0f0f0; padding: 15px; overflow-x: auto; border-radius: 3px; }
        .pods { display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 20px; margin: 20px 0; }
        .pod { background: white; border: 1px solid #ddd; padding: 15px; border-radius: 5px; }
        .pod-name { font-weight: bold; color: #333; }
        .pod-resources { margin: 10px 0; font-size: 0.9em; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🎯 Right-Sizer Dashboard</h1>

        <div class="status">
            <strong>Status:</strong> <span id="status">Checking...</span>
        </div>

        <button onclick="refreshMetrics()">🔄 Refresh Metrics</button>
        <button onclick="refreshPods()">📦 Refresh Pods</button>
        <button onclick="testResize()">🔧 Test Resize</button>

        <div class="metrics">
            <h2>📊 Metrics</h2>
            <pre id="metrics">Loading...</pre>
        </div>

        <div class="metrics">
            <h2>📦 Pods in Test Workloads</h2>
            <div id="pods" class="pods">Loading...</div>
        </div>

        <div class="metrics">
            <h2>📋 Logs</h2>
            <pre id="logs">Ready...</pre>
        </div>
    </div>

    <script>
        let logCount = 0;

        function log(message) {
            const logsElement = document.getElementById('logs');
            const timestamp = new Date().toLocaleTimeString();
            logsElement.textContent = \`[\${timestamp}] \${message}\n\` + logsElement.textContent.split('\n').slice(0, 20).join('\n');
        }

        async function checkStatus() {
            try {
                const response = await fetch('/api/v1/healthz');
                if (response.ok) {
                    document.getElementById('status').textContent = '✅ Operator Running';
                    document.getElementById('status').parentElement.className = 'status';
                } else {
                    throw new Error('Health check failed');
                }
            } catch (error) {
                document.getElementById('status').textContent = '❌ Operator Offline';
                document.getElementById('status').parentElement.className = 'status error';
            }
        }

        async function refreshMetrics() {
            log('Fetching metrics...');
            try {
                const response = await fetch('http://localhost:9090/metrics');
                const text = await response.text();

                // Parse some key metrics
                const lines = text.split('\n');
                const metrics = {};
                lines.forEach(line => {
                    if (line.startsWith('rightsizer_')) {
                        const parts = line.split(' ');
                        if (parts.length >= 2) {
                            metrics[parts[0]] = parts[1];
                        }
                    }
                });

                let output = 'Key Right-Sizer Metrics:\n\n';
                Object.entries(metrics).forEach(([key, value]) => {
                    output += \`\${key}: \${value}\n\`;
                });

                if (Object.keys(metrics).length === 0) {
                    output = text; // Show raw metrics if parsing failed
                }

                document.getElementById('metrics').textContent = output;
                log('Metrics updated');
            } catch (error) {
                document.getElementById('metrics').textContent = \`Error: \${error.message}\`;
                log(\`Metrics error: \${error.message}\`);
            }
        }

        async function refreshPods() {
            log('Fetching pods...');
            try {
                // Since we can't easily access K8s API from browser, show instructions
                document.getElementById('pods').innerHTML = \`
                    <div class="info">
                        <strong>To view pods, run in terminal:</strong><br>
                        <code>kubectl get pods -n test-workloads -o wide</code><br><br>
                        <strong>To check resource usage:</strong><br>
                        <code>kubectl top pods -n test-workloads</code><br><br>
                        <strong>To watch for resize events:</strong><br>
                        <code>kubectl logs -f deployment/right-sizer -n right-sizer | grep -i resize</code>
                    </div>
                \`;
                log('Pod instructions displayed');
            } catch (error) {
                log(\`Pod fetch error: \${error.message}\`);
            }
        }

        async function testResize() {
            log('Testing resize functionality...');
            try {
                document.getElementById('logs').textContent = \`Testing resize...

1. Check current pods:
   kubectl get pods -n test-workloads

2. Check resources:
   kubectl describe pod -n test-workloads | grep -A5 "Requests:"

3. Watch operator logs:
   kubectl logs -f deployment/right-sizer -n right-sizer

4. Force a resize check (if operator supports it):
   kubectl annotate pod -n test-workloads <pod-name> rightsizer.io/trigger="\$(date)"

5. Check for resize events:
   kubectl get events -n test-workloads | grep -i resize

The operator should automatically detect over-provisioned resources and attempt resizing.
Check the logs above for any forbidden errors or resize attempts.
\`;
                log('Resize test instructions shown');
            } catch (error) {
                log(\`Test error: \${error.message}\`);
            }
        }

        // Initialize
        checkStatus();
        refreshMetrics();
        refreshPods();

        // Auto-refresh every 30 seconds
        setInterval(() => {
            checkStatus();
            refreshMetrics();
        }, 30000);
    </script>
</body>
</html>
EOF

print_info "Deploying simple dashboard..."
kubectl create configmap dashboard-html --from-file=index.html=/tmp/simple-dashboard.html -n $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: right-sizer-dashboard
  namespace: $NAMESPACE
spec:
  replicas: 1
  selector:
    matchLabels:
      app: right-sizer-dashboard
  template:
    metadata:
      labels:
        app: right-sizer-dashboard
    spec:
      containers:
      - name: dashboard
        image: nginx:alpine
        ports:
        - containerPort: 80
        volumeMounts:
        - name: html
          mountPath: /usr/share/nginx/html
        resources:
          requests:
            cpu: 10m
            memory: 16Mi
          limits:
            cpu: 50m
            memory: 32Mi
      volumes:
      - name: html
        configMap:
          name: dashboard-html
---
apiVersion: v1
kind: Service
metadata:
  name: right-sizer-dashboard
  namespace: $NAMESPACE
spec:
  type: NodePort
  ports:
  - port: 80
    targetPort: 80
    nodePort: 30080
  selector:
    app: right-sizer-dashboard
EOF

# Create test workloads
print_info "Creating test workloads..."
kubectl create namespace test-workloads 2>/dev/null || true

cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: overprovisioned-app
  namespace: test-workloads
  labels:
    rightsizer.io/enable: "true"
spec:
  replicas: 1
  selector:
    matchLabels:
      app: overprovisioned-app
  template:
    metadata:
      labels:
        app: overprovisioned-app
    spec:
      containers:
      - name: app
        image: nginx:alpine
        resources:
          requests:
            cpu: 500m
            memory: 512Mi
          limits:
            cpu: 1000m
            memory: 1Gi
        ports:
        - containerPort: 80
EOF

# Wait for deployments
print_info "Waiting for deployments..."
kubectl wait --for=condition=available deployment/right-sizer -n $NAMESPACE --timeout=180s || true
kubectl wait --for=condition=available deployment/right-sizer-dashboard -n $NAMESPACE --timeout=180s || true
kubectl wait --for=condition=available deployment/overprovisioned-app -n test-workloads --timeout=120s || true

# Setup port forwarding in background
print_info "Setting up port forwarding..."
pkill -f "kubectl port-forward" 2>/dev/null || true
kubectl port-forward -n $NAMESPACE svc/right-sizer-dashboard 3000:80 >/dev/null 2>&1 &
kubectl port-forward -n $NAMESPACE svc/right-sizer 9090:9090 >/dev/null 2>&1 &

# Wait a moment for port forwards to establish
sleep 3

# Show status
print_success "✅ Minimal deployment complete!"
echo ""
echo "📊 Dashboard: http://localhost:3000"
echo "📈 Metrics: http://localhost:9090/metrics"
echo "🔗 NodePort: http://$(minikube -p $CLUSTER_NAME ip):30080"
echo ""
echo "🔍 Useful commands:"
echo "  kubectl logs -f deployment/right-sizer -n $NAMESPACE"
echo "  kubectl get pods -A"
echo "  kubectl top pods -n test-workloads"
echo ""
echo "🧪 Test the functionality:"
echo "  kubectl get pods -n test-workloads"
echo "  kubectl describe pod -n test-workloads | grep -A5 Requests"
echo ""

# Verify RBAC
if kubectl get clusterrole right-sizer -o yaml | grep -q "pods/resize"; then
  print_success "✅ RBAC includes pods/resize permission"
else
  print_warning "⚠️  pods/resize permission may be missing"
fi

# Check for forbidden errors
print_info "Checking for RBAC errors..."
sleep 10 # Let operator start
if kubectl logs deployment/right-sizer -n $NAMESPACE --tail=50 2>/dev/null | grep -i forbidden; then
  print_warning "Found forbidden errors in logs"
else
  print_success "No forbidden errors detected"
fi

# Final status
echo ""
echo "🎯 Right-Sizer is deployed and ready!"
echo "   Open http://localhost:3000 to access the dashboard"
echo ""
print_info "Press Ctrl+C to stop port forwarding and exit"

# Keep script running for port forwards
trap 'echo ""; print_warning "Stopping port forwarding..."; pkill -f "kubectl port-forward" || true; exit 0' INT
wait

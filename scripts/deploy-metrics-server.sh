#!/bin/bash

# Metrics Server Deployment and Verification Script for Minikube
# This script ensures metrics-server is properly deployed and working

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color

# Configuration
PROFILE="${MINIKUBE_PROFILE:-right-sizer}"
TIMEOUT="${METRICS_TIMEOUT:-120}"
STANDALONE_DEPLOY="${STANDALONE_DEPLOY:-false}"

# Functions
print_header() {
  echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════${NC}"
  echo -e "${BLUE} $1${NC}"
  echo -e "${BLUE}═══════════════════════════════════════════════════════════════${NC}"
}

print_success() {
  echo -e "${GREEN}✓${NC} $1"
}

print_error() {
  echo -e "${RED}✗${NC} $1"
}

print_warning() {
  echo -e "${YELLOW}⚠${NC} $1"
}

print_info() {
  echo -e "${CYAN}ℹ${NC} $1"
}

check_minikube() {
  if ! minikube -p "$PROFILE" status &>/dev/null; then
    print_error "Minikube profile '$PROFILE' is not running"
    print_info "Start it with: minikube start -p $PROFILE"
    exit 1
  fi
}

deploy_addon_metrics_server() {
  print_header "Deploying Metrics Server via Minikube Addon"

  # Check if already enabled
  if minikube -p "$PROFILE" addons list | grep -q "metrics-server.*enabled"; then
    print_info "Metrics-server addon is already enabled"
  else
    print_info "Enabling metrics-server addon..."
    minikube -p "$PROFILE" addons enable metrics-server
    print_success "Metrics-server addon enabled"
  fi

  # Wait for deployment
  print_info "Waiting for metrics-server deployment..."
  kubectl wait --for=condition=available deployment/metrics-server \
    -n kube-system \
    --timeout="${TIMEOUT}s" 2>/dev/null || {
    print_warning "Deployment not ready yet, checking pods..."
  }

  # Wait for pod to be ready
  print_info "Waiting for metrics-server pod to be ready..."
  local retries=0
  local max_retries=$((TIMEOUT / 5))

  while [ $retries -lt $max_retries ]; do
    if kubectl get pods -n kube-system -l k8s-app=metrics-server \
      -o jsonpath='{.items[0].status.phase}' 2>/dev/null | grep -q "Running"; then
      print_success "Metrics-server pod is running"
      break
    fi
    echo -n "."
    sleep 5
    retries=$((retries + 1))
  done

  if [ $retries -eq $max_retries ]; then
    print_error "Metrics-server pod did not become ready in time"
    kubectl describe pods -n kube-system -l k8s-app=metrics-server
    return 1
  fi
}

deploy_standalone_metrics_server() {
  print_header "Deploying Standalone Metrics Server"

  print_info "Applying official metrics-server manifest..."

  # Create a custom metrics-server configuration for Minikube
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    k8s-app: metrics-server
  name: metrics-server
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    k8s-app: metrics-server
    rbac.authorization.k8s.io/aggregate-to-admin: "true"
    rbac.authorization.k8s.io/aggregate-to-edit: "true"
    rbac.authorization.k8s.io/aggregate-to-view: "true"
  name: system:aggregated-metrics-reader
rules:
- apiGroups:
  - metrics.k8s.io
  resources:
  - pods
  - nodes
  verbs:
  - get
  - list
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    k8s-app: metrics-server
  name: system:metrics-server
rules:
- apiGroups:
  - ""
  resources:
  - nodes/metrics
  - nodes/stats
  - nodes/proxy
  - pods
  - nodes
  verbs:
  - get
  - list
  - watch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  labels:
    k8s-app: metrics-server
  name: metrics-server-auth-reader
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: extension-apiserver-authentication-reader
subjects:
- kind: ServiceAccount
  name: metrics-server
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  labels:
    k8s-app: metrics-server
  name: metrics-server:system:auth-delegator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:auth-delegator
subjects:
- kind: ServiceAccount
  name: metrics-server
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  labels:
    k8s-app: metrics-server
  name: system:metrics-server
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:metrics-server
subjects:
- kind: ServiceAccount
  name: metrics-server
  namespace: kube-system
---
apiVersion: v1
kind: Service
metadata:
  labels:
    k8s-app: metrics-server
  name: metrics-server
  namespace: kube-system
spec:
  ports:
  - name: https
    port: 443
    protocol: TCP
    targetPort: https
  selector:
    k8s-app: metrics-server
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    k8s-app: metrics-server
  name: metrics-server
  namespace: kube-system
spec:
  selector:
    matchLabels:
      k8s-app: metrics-server
  strategy:
    rollingUpdate:
      maxUnavailable: 0
  template:
    metadata:
      labels:
        k8s-app: metrics-server
    spec:
      containers:
      - args:
        - --cert-dir=/tmp
        - --secure-port=10250
        - --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname
        - --kubelet-use-node-status-port
        - --metric-resolution=15s
        - --kubelet-insecure-tls
        image: registry.k8s.io/metrics-server/metrics-server:v0.8.0
        imagePullPolicy: IfNotPresent
        livenessProbe:
          failureThreshold: 3
          httpGet:
            path: /livez
            port: https
            scheme: HTTPS
          periodSeconds: 10
        name: metrics-server
        ports:
        - containerPort: 10250
          name: https
          protocol: TCP
        readinessProbe:
          failureThreshold: 3
          httpGet:
            path: /readyz
            port: https
            scheme: HTTPS
          initialDelaySeconds: 20
          periodSeconds: 10
        resources:
          requests:
            cpu: 100m
            memory: 200Mi
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          runAsUser: 1000
          seccompProfile:
            type: RuntimeDefault
        volumeMounts:
        - mountPath: /tmp
          name: tmp-dir
      nodeSelector:
        kubernetes.io/os: linux
      priorityClassName: system-cluster-critical
      serviceAccountName: metrics-server
      volumes:
      - emptyDir: {}
        name: tmp-dir
---
apiVersion: apiregistration.k8s.io/v1
kind: APIService
metadata:
  labels:
    k8s-app: metrics-server
  name: v1beta1.metrics.k8s.io
spec:
  group: metrics.k8s.io
  groupPriorityMinimum: 100
  insecureSkipTLSVerify: true
  service:
    name: metrics-server
    namespace: kube-system
  version: v1beta1
  versionPriority: 100
EOF

  print_success "Metrics-server manifest applied"

  # Wait for deployment
  print_info "Waiting for metrics-server deployment..."
  kubectl wait --for=condition=available deployment/metrics-server \
    -n kube-system \
    --timeout="${TIMEOUT}s"
}

verify_metrics_server() {
  print_header "Verifying Metrics Server"

  # Check deployment status
  print_info "Checking deployment status..."
  local deployment_status=$(kubectl get deployment metrics-server -n kube-system \
    -o jsonpath='{.status.conditions[?(@.type=="Available")].status}' 2>/dev/null)

  if [ "$deployment_status" = "True" ]; then
    print_success "Deployment is available"
  else
    print_error "Deployment is not available"
    kubectl describe deployment metrics-server -n kube-system
    return 1
  fi

  # Check pod status
  print_info "Checking pod status..."
  local pod_count=$(kubectl get pods -n kube-system -l k8s-app=metrics-server \
    --field-selector=status.phase=Running --no-headers 2>/dev/null | wc -l)

  if [ "$pod_count" -gt 0 ]; then
    print_success "Metrics-server pod is running"
    kubectl get pods -n kube-system -l k8s-app=metrics-server
  else
    print_error "No running metrics-server pods found"
    kubectl get pods -n kube-system -l k8s-app=metrics-server
    return 1
  fi

  # Check API service
  print_info "Checking metrics API service..."
  if kubectl get apiservice v1beta1.metrics.k8s.io -o jsonpath='{.status.conditions[?(@.type=="Available")].status}' | grep -q "True"; then
    print_success "Metrics API service is available"
  else
    print_warning "Metrics API service might not be ready yet"
  fi

  # Wait for metrics to be available
  print_info "Waiting for metrics to be available (this may take a minute)..."
  local retries=0
  local max_retries=12 # 60 seconds with 5-second intervals

  while [ $retries -lt $max_retries ]; do
    if kubectl top nodes &>/dev/null; then
      print_success "Metrics are available!"
      break
    fi
    echo -n "."
    sleep 5
    retries=$((retries + 1))
  done

  if [ $retries -eq $max_retries ]; then
    print_warning "Metrics might not be available yet. This is normal for the first minute."
    print_info "Try running 'kubectl top nodes' in a minute"
  fi
}

test_metrics() {
  print_header "Testing Metrics Collection"

  print_info "Testing node metrics..."
  if kubectl top nodes 2>/dev/null; then
    print_success "Node metrics working:"
    kubectl top nodes
  else
    print_warning "Node metrics not yet available"
  fi

  echo ""
  print_info "Testing pod metrics in kube-system..."
  if kubectl top pods -n kube-system --no-headers 2>/dev/null | head -5; then
    print_success "Pod metrics working"
  else
    print_warning "Pod metrics not yet available"
  fi

  # Test specific namespaces if they exist
  if kubectl get namespace right-sizer &>/dev/null; then
    echo ""
    print_info "Testing metrics for right-sizer namespace..."
    kubectl top pods -n right-sizer 2>/dev/null || print_warning "Metrics not yet available for right-sizer"
  fi

  if kubectl get namespace test-workloads &>/dev/null; then
    echo ""
    print_info "Testing metrics for test-workloads namespace..."
    kubectl top pods -n test-workloads 2>/dev/null || print_warning "Metrics not yet available for test-workloads"
  fi
}

troubleshoot_metrics() {
  print_header "Troubleshooting Metrics Server"

  print_info "Checking metrics-server logs..."
  kubectl logs -n kube-system -l k8s-app=metrics-server --tail=20

  print_info "Checking metrics-server events..."
  kubectl get events -n kube-system --field-selector involvedObject.name=metrics-server --sort-by='.lastTimestamp' | tail -10

  print_info "Checking API service status..."
  kubectl get apiservice v1beta1.metrics.k8s.io -o yaml | grep -A 10 "status:"
}

show_usage() {
  cat <<EOF
Usage: $0 [OPTIONS]

Deploy and verify metrics-server for Kubernetes metrics collection.

OPTIONS:
    --profile NAME       Minikube profile name (default: right-sizer)
    --standalone        Deploy standalone instead of using Minikube addon
    --timeout SECONDS   Timeout for deployment (default: 120)
    --troubleshoot      Show troubleshooting information
    --test-only         Only test existing metrics-server
    --help              Show this help message

EXAMPLES:
    $0                          # Deploy using Minikube addon
    $0 --standalone             # Deploy standalone metrics-server
    $0 --test-only              # Test existing deployment
    $0 --troubleshoot           # Debug issues

EOF
}

# Main execution
main() {
  local action="deploy"

  # Parse arguments
  while [[ $# -gt 0 ]]; do
    case $1 in
    --profile)
      PROFILE="$2"
      shift 2
      ;;
    --standalone)
      STANDALONE_DEPLOY="true"
      shift
      ;;
    --timeout)
      TIMEOUT="$2"
      shift 2
      ;;
    --troubleshoot)
      action="troubleshoot"
      shift
      ;;
    --test-only)
      action="test"
      shift
      ;;
    --help | -h)
      show_usage
      exit 0
      ;;
    *)
      print_error "Unknown option: $1"
      show_usage
      exit 1
      ;;
    esac
  done

  print_header "Metrics Server Deployment Script"
  print_info "Profile: $PROFILE"
  print_info "Timeout: ${TIMEOUT}s"

  # Check Minikube
  check_minikube

  case "$action" in
  deploy)
    if [ "$STANDALONE_DEPLOY" = "true" ]; then
      deploy_standalone_metrics_server
    else
      deploy_addon_metrics_server
    fi
    verify_metrics_server
    test_metrics

    print_header "Deployment Complete!"
    print_success "Metrics-server is ready for use"
    echo ""
    print_info "Useful commands:"
    echo "  kubectl top nodes                    # View node resource usage"
    echo "  kubectl top pods -A                  # View all pod resource usage"
    echo "  kubectl top pods -n <namespace>      # View namespace pod usage"
    echo ""
    ;;

  test)
    verify_metrics_server
    test_metrics
    ;;

  troubleshoot)
    verify_metrics_server
    troubleshoot_metrics
    ;;
  esac
}

# Run main function
main "$@"

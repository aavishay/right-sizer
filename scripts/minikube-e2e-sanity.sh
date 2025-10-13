#!/usr/bin/env bash
set -u

# Minikube E2E Sanity Tests (lightweight)
# Usage: ./scripts/minikube-e2e-sanity.sh
# Exits with 0 if all checks pass, non-zero otherwise.

FAIL=0
NAMESPACE=${1:-right-sizer}
MINIKUBE_PROFILE=${MINIKUBE_PROFILE:-right-sizer}
MINIKUBE_IP=${MINIKUBE_IP:-$(minikube -p "$MINIKUBE_PROFILE" ip 2>/dev/null || true)}

echo "Minikube profile: $MINIKUBE_PROFILE"

function check_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "ERROR: required command not found: $1"
    FAIL=1
  fi
}

check_cmd kubectl
check_cmd minikube
check_cmd curl
check_cmd jq || true

# 1. Minikube status
echo "\n== Cluster status =="
minikube profile list | grep -E "^$MINIKUBE_PROFILE\b" || true

if ! minikube -p "$MINIKUBE_PROFILE" status >/dev/null 2>&1; then
  echo "ERROR: minikube profile $MINIKUBE_PROFILE not running"
  FAIL=1
fi

# 2. Right-Sizer pods
echo "\n== Right-Sizer pods in namespace $NAMESPACE =="
if ! kubectl get ns "$NAMESPACE" >/dev/null 2>&1; then
  echo "ERROR: namespace $NAMESPACE not found"
  FAIL=1
else
  kubectl get pods -n "$NAMESPACE" -o wide || true
  NOT_READY=$(kubectl get pods -n "$NAMESPACE" --no-headers 2>/dev/null | awk '{print $2" " $1}' | grep -v "1/1" || true)
  if [ -n "$NOT_READY" ]; then
    echo "WARNING: some pods are not ready:\n$NOT_READY"
    FAIL=1
  fi
fi

# 3. Health endpoints (port-forward briefly)
echo "\n== Health endpoints =="
POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=right-sizer -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
# Try alternate label value (some deployments use 'rightsizer')
if [ -z "$POD" ]; then
  POD=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=rightsizer -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
fi
# Fall back to first pod in namespace if label lookup fails
if [ -z "$POD" ]; then
  POD=$(kubectl get pods -n "$NAMESPACE" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
fi
if [ -z "$POD" ]; then
  echo "ERROR: right-sizer pod not found in $NAMESPACE"
  FAIL=1
else
  echo "Found pod: $POD"
  kubectl port-forward -n "$NAMESPACE" pods/$POD 8081:8081 >/dev/null 2>&1 &
  PF_PID=$!
  sleep 1
  if curl -s http://localhost:8081/healthz | grep -qi "ok"; then
    echo "Health OK"
  else
    echo "ERROR: health endpoint failed"
    FAIL=1
  fi
  if curl -s http://localhost:8081/readyz | grep -qi "ok"; then
    echo "Ready OK"
  else
    echo "ERROR: ready endpoint failed"
    FAIL=1
  fi
  kill $PF_PID 2>/dev/null || true
fi

# 4. API endpoints
echo "\n== API endpoints =="
if [ -n "$POD" ]; then
  kubectl port-forward -n "$NAMESPACE" pods/$POD 8082:8082 >/dev/null 2>&1 &
  API_PID=$!
  sleep 1
  if curl -s http://localhost:8082/api/health | jq . >/dev/null 2>&1; then
    echo "API health OK"
  else
    echo "WARNING: API health may be unavailable"
  fi
  kill $API_PID 2>/dev/null || true
fi

# 5. Metrics
echo "\n== Metrics =="
if [ -n "$POD" ]; then
  kubectl port-forward -n "$NAMESPACE" pods/$POD 9090:9090 >/dev/null 2>&1 &
  MET_PID=$!
  sleep 1
  MET_COUNT=$(curl -s http://localhost:9090/metrics | grep -c "^rightsizer_" || true)
  echo "Found $MET_COUNT rightsizer_* metrics"
  kill $MET_PID 2>/dev/null || true
fi

# 6. CRDs
echo "\n== CRDs and CRs =="
CRDS=$(kubectl get crd | grep rightsizer || true)
if [ -z "$CRDS" ]; then
  echo "WARNING: rightsizer CRDs not found"
  FAIL=1
else
  echo "$CRDS"
fi

# 7. Summary
echo "\n== Summary =="
if [ "$FAIL" -eq 0 ]; then
  echo "All sanity checks passed"
else
  echo "Some checks failed. See errors above"
fi

exit $FAIL

# Right Sizer RBAC Troubleshooting Guide

## Overview

This guide provides detailed troubleshooting steps for resolving RBAC (Role-Based Access Control) issues with the Right Sizer operator. Use this guide when encountering permission-related errors or when the operator fails to perform expected operations due to authorization issues.

## Table of Contents

- [Quick Diagnosis](#quick-diagnosis)
- [Common RBAC Errors](#common-rbac-errors)
- [Diagnostic Commands](#diagnostic-commands)
- [Step-by-Step Troubleshooting](#step-by-step-troubleshooting)
- [Advanced Debugging](#advanced-debugging)
- [Emergency Fixes](#emergency-fixes)
- [Prevention Best Practices](#prevention-best-practices)

## Quick Diagnosis

### Automated Check

Run the verification script for immediate diagnosis:

```bash
# Quick check with default settings
./scripts/rbac/verify-permissions.sh

# Verbose output for detailed analysis
./scripts/rbac/verify-permissions.sh --verbose

# Check specific namespace
./scripts/rbac/verify-permissions.sh -n my-namespace
```

### Manual Quick Check

```bash
# Set your namespace and service account
NAMESPACE="right-sizer-system"
SA="right-sizer"

# Check if service account exists
kubectl get sa $SA -n $NAMESPACE

# List all permissions for the service account
kubectl auth can-i --list --as=system:serviceaccount:$NAMESPACE:$SA

# Check specific critical permissions
kubectl auth can-i patch pods --as=system:serviceaccount:$NAMESPACE:$SA
kubectl auth can-i get pods.metrics.k8s.io --as=system:serviceaccount:$NAMESPACE:$SA
```

## Common RBAC Errors

### 1. Service Account Not Found

**Error Message:**
```
Error from server (Forbidden): serviceaccounts "right-sizer" is forbidden: User "system:serviceaccount:right-sizer-system:right-sizer" cannot get resource "serviceaccounts" in API group "" in the namespace "right-sizer-system"
```

**Diagnosis:**
```bash
# Check if service account exists
kubectl get sa -n right-sizer-system

# Check if namespace exists
kubectl get namespace right-sizer-system
```

**Solution:**
```bash
# Create namespace and service account
kubectl create namespace right-sizer-system
kubectl create serviceaccount right-sizer -n right-sizer-system

# Or use the fix script
./scripts/rbac/apply-rbac-fix.sh --force
```

### 2. Metrics API Forbidden

**Error Message:**
```
E0123 12:34:56.789012   1 reflector.go:138] Failed to list *v1beta1.PodMetrics: pods.metrics.k8s.io is forbidden: User "system:serviceaccount:right-sizer-system:right-sizer" cannot list resource "pods" in API group "metrics.k8s.io" at the cluster scope
```

**Diagnosis:**
```bash
# Check if metrics-server is installed
kubectl get deployment metrics-server -n kube-system

# Check if metrics API is available
kubectl api-versions | grep metrics

# Test metrics API access
kubectl top nodes
kubectl top pods -A
```

**Solution:**
```bash
# Install metrics-server if missing
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# Grant metrics permissions
kubectl apply -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: right-sizer-metrics-reader
rules:
- apiGroups: ["metrics.k8s.io"]
  resources: ["pods", "nodes", "podmetrics", "nodemetrics"]
  verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: right-sizer-metrics-reader
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: right-sizer-metrics-reader
subjects:
- kind: ServiceAccount
  name: right-sizer
  namespace: right-sizer-system
EOF
```

### 3. Pod Resize Forbidden (Kubernetes 1.27+)

**Error Message:**
```
Error resizing pod: pods "my-app-7b9f8c5d7-abc123" is forbidden: User "system:serviceaccount:right-sizer-system:right-sizer" cannot patch resource "pods/resize" in API group "" in the namespace "default"
```

**Diagnosis:**
```bash
# Check Kubernetes version
kubectl version --short

# Check if resize subresource is available
kubectl api-resources | grep resize

# Test resize permission
kubectl auth can-i patch pods/resize --as=system:serviceaccount:right-sizer-system:right-sizer
```

**Solution:**
```bash
# Add resize permissions
kubectl patch clusterrole right-sizer --type='json' -p='[
  {
    "op": "add",
    "path": "/rules/-",
    "value": {
      "apiGroups": [""],
      "resources": ["pods/resize"],
      "verbs": ["patch", "update"]
    }
  }
]'
```

### 4. Workload Controller Update Forbidden

**Error Message:**
```
deployments.apps "my-app" is forbidden: User "system:serviceaccount:right-sizer-system:right-sizer" cannot patch resource "deployments" in API group "apps" in the namespace "production"
```

**Diagnosis:**
```bash
# Check deployment permissions
kubectl auth can-i patch deployments.apps --as=system:serviceaccount:right-sizer-system:right-sizer -n production

# List all apps permissions
kubectl auth can-i --list --as=system:serviceaccount:right-sizer-system:right-sizer | grep apps
```

**Solution:**
```bash
# Grant workload controller permissions
kubectl patch clusterrole right-sizer --type='json' -p='[
  {
    "op": "add",
    "path": "/rules/-",
    "value": {
      "apiGroups": ["apps"],
      "resources": ["deployments", "statefulsets", "daemonsets", "replicasets"],
      "verbs": ["get", "list", "watch", "patch", "update"]
    }
  }
]'
```

### 5. Admission Webhook Registration Failed

**Error Message:**
```
Error creating webhook: validatingwebhookconfigurations.admissionregistration.k8s.io is forbidden: User "system:serviceaccount:right-sizer-system:right-sizer" cannot create resource "validatingwebhookconfigurations" in API group "admissionregistration.k8s.io" at the cluster scope
```

**Solution:**
```bash
# Grant webhook permissions
kubectl patch clusterrole right-sizer --type='json' -p='[
  {
    "op": "add",
    "path": "/rules/-",
    "value": {
      "apiGroups": ["admissionregistration.k8s.io"],
      "resources": ["validatingwebhookconfigurations", "mutatingwebhookconfigurations"],
      "verbs": ["get", "list", "watch", "create", "update", "patch", "delete"]
    }
  }
]'
```

## Diagnostic Commands

### Check Service Account Configuration

```bash
# View service account details
kubectl describe sa right-sizer -n right-sizer-system

# Check token secret
kubectl get secret -n right-sizer-system | grep right-sizer-token

# Verify token is mounted in pod
kubectl describe pod -n right-sizer-system -l app=right-sizer | grep -A 5 "Mounts:"
```

### Inspect ClusterRole and Bindings

```bash
# View ClusterRole
kubectl describe clusterrole right-sizer

# View ClusterRoleBinding
kubectl describe clusterrolebinding right-sizer

# Export current RBAC configuration
kubectl get clusterrole right-sizer -o yaml > current-clusterrole.yaml
kubectl get clusterrolebinding right-sizer -o yaml > current-clusterrolebinding.yaml

# Check for duplicate or conflicting roles
kubectl get clusterrole | grep -i right-sizer
kubectl get clusterrolebinding | grep -i right-sizer
```

### Test Specific Permissions

```bash
# Create a test script for all permissions
cat > test-permissions.sh << 'EOF'
#!/bin/bash
NAMESPACE="${1:-right-sizer-system}"
SA="${2:-right-sizer}"
AUTH="--as=system:serviceaccount:$NAMESPACE:$SA"

echo "Testing permissions for $SA in namespace $NAMESPACE"
echo "================================================"

# Core resources
for resource in pods nodes events namespaces configmaps secrets; do
  echo -n "Checking $resource: "
  kubectl auth can-i list $resource $AUTH && echo "✓" || echo "✗"
done

# Metrics
echo -n "Checking metrics.k8s.io/pods: "
kubectl auth can-i list pods.metrics.k8s.io $AUTH && echo "✓" || echo "✗"

# Workloads
for resource in deployments statefulsets daemonsets; do
  echo -n "Checking apps/$resource: "
  kubectl auth can-i patch $resource.apps $AUTH && echo "✓" || echo "✗"
done
EOF

chmod +x test-permissions.sh
./test-permissions.sh
```

### Monitor RBAC Events

```bash
# Watch for RBAC-related events
kubectl get events -A --field-selector reason=FailedAuthorization -w

# Check audit logs (if enabled)
kubectl logs -n kube-system -l component=kube-apiserver | grep -i "forbidden\|denied" | tail -20

# Monitor operator logs for permission errors
kubectl logs -n right-sizer-system deployment/right-sizer -f | grep -i "forbidden\|unauthorized\|denied"
```

## Step-by-Step Troubleshooting

### Phase 1: Verify Basic Setup

1. **Check namespace exists:**
   ```bash
   kubectl get namespace right-sizer-system
   ```

2. **Check service account exists:**
   ```bash
   kubectl get sa right-sizer -n right-sizer-system
   ```

3. **Check ClusterRole exists:**
   ```bash
   kubectl get clusterrole right-sizer
   ```

4. **Check ClusterRoleBinding exists:**
   ```bash
   kubectl get clusterrolebinding right-sizer
   ```

### Phase 2: Validate RBAC Configuration

1. **Verify ClusterRoleBinding references:**
   ```bash
   kubectl get clusterrolebinding right-sizer -o jsonpath='{.roleRef.name}' | grep -q right-sizer
   kubectl get clusterrolebinding right-sizer -o jsonpath='{.subjects[0].name}' | grep -q right-sizer
   ```

2. **Check for typos in namespace:**
   ```bash
   kubectl get clusterrolebinding right-sizer -o jsonpath='{.subjects[0].namespace}'
   ```

3. **Validate Role has required permissions:**
   ```bash
   kubectl get clusterrole right-sizer -o yaml | grep -E "resources:|verbs:"
   ```

### Phase 3: Test Critical Permissions

1. **Test pod operations:**
   ```bash
   kubectl auth can-i patch pods --as=system:serviceaccount:right-sizer-system:right-sizer
   kubectl auth can-i get pods/status --as=system:serviceaccount:right-sizer-system:right-sizer
   ```

2. **Test metrics access:**
   ```bash
   kubectl auth can-i list pods.metrics.k8s.io --as=system:serviceaccount:right-sizer-system:right-sizer
   ```

3. **Test workload updates:**
   ```bash
   kubectl auth can-i patch deployments.apps --as=system:serviceaccount:right-sizer-system:right-sizer
   ```

### Phase 4: Apply Fixes

If issues are found:

```bash
# Option 1: Quick fix with script
./scripts/rbac/apply-rbac-fix.sh --force

# Option 2: Reinstall with Helm
helm upgrade --install right-sizer ./helm \
  --namespace right-sizer-system \
  --create-namespace

# Option 3: Manual fix
kubectl apply -f helm/templates/rbac.yaml
```

## Advanced Debugging

### Enable Verbose Logging

```bash
# Increase operator log level
kubectl set env deployment/right-sizer \
  -n right-sizer-system \
  LOG_LEVEL=debug \
  RBAC_DEBUG=true

# Restart to apply
kubectl rollout restart deployment/right-sizer -n right-sizer-system
```

### Use Impersonation for Testing

```bash
# Test as the service account
kubectl --as=system:serviceaccount:right-sizer-system:right-sizer \
  get pods -A

# Interactive testing
kubectl run test-pod --rm -it \
  --image=bitnami/kubectl:latest \
  --overrides='{"spec":{"serviceAccountName":"right-sizer"}}' \
  -n right-sizer-system \
  -- bash
```

### Analyze with kubectl-who-can

```bash
# Install kubectl-who-can plugin
kubectl krew install who-can

# Check who can perform actions
kubectl who-can patch pods
kubectl who-can get pods.metrics.k8s.io
```

## Emergency Fixes

### Complete RBAC Reset

```bash
#!/bin/bash
# WARNING: This will completely reset Right Sizer RBAC

NAMESPACE="right-sizer-system"
RELEASE="right-sizer"

# Delete all RBAC resources
kubectl delete clusterrole $RELEASE --ignore-not-found
kubectl delete clusterrolebinding $RELEASE --ignore-not-found
kubectl delete role -n $NAMESPACE $RELEASE-namespace --ignore-not-found
kubectl delete rolebinding -n $NAMESPACE $RELEASE-namespace --ignore-not-found
kubectl delete sa -n $NAMESPACE $RELEASE --ignore-not-found

# Wait for cleanup
sleep 5

# Reapply RBAC
./scripts/rbac/apply-rbac-fix.sh --force

# Restart operator
kubectl rollout restart deployment/$RELEASE -n $NAMESPACE
```

### Temporary Elevated Permissions

```bash
# DANGER: Only for debugging - remove after fixing!
kubectl create clusterrolebinding right-sizer-debug \
  --clusterrole=cluster-admin \
  --serviceaccount=right-sizer-system:right-sizer

# Remember to delete after debugging
kubectl delete clusterrolebinding right-sizer-debug
```

## Prevention Best Practices

### Regular Validation

Create a CronJob to regularly validate RBAC:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: rbac-validator
  namespace: right-sizer-system
spec:
  schedule: "0 */6 * * *"  # Every 6 hours
  jobTemplate:
    spec:
      template:
        spec:
          serviceAccountName: right-sizer
          containers:
          - name: validator
            image: bitnami/kubectl:latest
            command:
            - /bin/bash
            - -c
            - |
              echo "Validating RBAC permissions..."
              kubectl auth can-i --list | grep -E "pods|metrics|deployments" || exit 1
              echo "RBAC validation successful"
          restartPolicy: OnFailure
```

### Monitor RBAC Changes

```bash
# Set up alerts for RBAC modifications
kubectl get clusterrole right-sizer -o yaml | sha256sum > /tmp/rbac-checksum

# Check for changes
kubectl get clusterrole right-sizer -o yaml | sha256sum --check /tmp/rbac-checksum
```

### Documentation

Always document custom RBAC modifications:

```yaml
# custom-rbac-additions.yaml
# Date: 2024-01-01
# Author: Admin
# Reason: Added custom resource permissions for integration
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: right-sizer-custom
  annotations:
    description: "Custom permissions for XYZ integration"
rules:
- apiGroups: ["custom.io"]
  resources: ["customresources"]
  verbs: ["get", "list", "watch"]
```

## Getting Help

If you're still experiencing issues after following this guide:

1. **Collect diagnostic information:**
   ```bash
   ./scripts/rbac/verify-permissions.sh --verbose > rbac-diagnosis.txt
   kubectl get events -A --sort-by='.lastTimestamp' | tail -50 >> rbac-diagnosis.txt
   kubectl logs -n right-sizer-system deployment/right-sizer --tail=100 >> rbac-diagnosis.txt
   ```

2. **Check the documentation:**
   - [Main RBAC Guide](./RBAC.md)
   - [Configuration Guide](../CONFIGURATION.md)
   - [Kubernetes RBAC Documentation](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)

3. **Open an issue** with:
   - Kubernetes version (`kubectl version`)
   - Right Sizer version
   - Output from diagnostic commands
   - Specific error messages

## Quick Reference Card

```bash
# Essential Commands
./scripts/rbac/verify-permissions.sh          # Check all permissions
./scripts/rbac/apply-rbac-fix.sh --force      # Fix RBAC issues
kubectl auth can-i --list --as=system:serviceaccount:right-sizer-system:right-sizer  # List all permissions
kubectl logs -n right-sizer-system deployment/right-sizer | grep -i forbidden  # Check for permission errors

# Common Fixes
kubectl apply -f helm/templates/rbac.yaml     # Reapply RBAC
helm upgrade right-sizer ./helm --reuse-values # Update with Helm
kubectl rollout restart deployment/right-sizer -n right-sizer-system  # Restart operator
```

# Right Sizer RBAC Configuration Guide

## Overview

The Right Sizer operator requires specific Role-Based Access Control (RBAC) permissions to monitor, analyze, and optimize resource allocations in your Kubernetes cluster. This document provides a comprehensive guide to understanding, configuring, and troubleshooting RBAC permissions for the Right Sizer.

## Table of Contents

- [Required Permissions](#required-permissions)
- [Installation](#installation)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)
- [Security Best Practices](#security-best-practices)
- [Version Compatibility](#version-compatibility)

## Required Permissions

### Core Kubernetes Resources

#### Pod Operations
The Right Sizer needs extensive pod permissions for monitoring and resizing:

```yaml
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch", "patch", "update"]
  
- apiGroups: [""]
  resources: ["pods/status"]
  verbs: ["get", "list", "watch", "patch", "update"]
  
- apiGroups: [""]
  resources: ["pods/resize"]  # For in-place resize (K8s 1.27+)
  verbs: ["patch", "update"]
```

**Why needed:**
- Monitor pod resource usage and status
- Apply resource adjustments
- Perform in-place resizing without pod restarts (when supported)

#### Node Information
Access to node information for capacity planning:

```yaml
- apiGroups: [""]
  resources: ["nodes", "nodes/status"]
  verbs: ["get", "list", "watch"]
```

**Why needed:**
- Validate available cluster capacity
- Ensure resource changes don't exceed node limits
- Consider node-level constraints in optimization decisions

### Metrics APIs

#### Metrics Server Integration
The Right Sizer requires access to metrics for resource usage data:

```yaml
# Metrics API v1 (current)
- apiGroups: ["metrics.k8s.io"]
  resources: ["pods", "nodes", "podmetrics", "nodemetrics"]
  verbs: ["get", "list", "watch"]

# Custom and external metrics (optional)
- apiGroups: ["custom.metrics.k8s.io", "external.metrics.k8s.io"]
  resources: ["*"]
  verbs: ["get", "list", "watch"]
```

**Why needed:**
- Fetch real-time CPU and memory usage
- Make data-driven resizing decisions
- Support for custom metrics when available

### Workload Controllers

#### Deployments, StatefulSets, and DaemonSets
Permissions to manage workload controllers:

```yaml
- apiGroups: ["apps"]
  resources: ["deployments", "statefulsets", "daemonsets", "replicasets"]
  verbs: ["get", "list", "watch", "patch", "update"]
  
- apiGroups: ["apps"]
  resources: ["deployments/status", "statefulsets/status", "daemonsets/status", "replicasets/status"]
  verbs: ["get", "list", "watch"]
  
- apiGroups: ["apps"]
  resources: ["deployments/scale", "statefulsets/scale", "replicasets/scale"]
  verbs: ["get", "patch", "update"]
```

**Why needed:**
- Update resource specifications in pod templates
- Monitor workload status
- Coordinate with scaling operations

### Autoscaling Integration

#### HPA and VPA Compatibility
Avoid conflicts with autoscalers:

```yaml
- apiGroups: ["autoscaling"]
  resources: ["horizontalpodautoscalers"]
  verbs: ["get", "list", "watch"]
  
- apiGroups: ["autoscaling.k8s.io"]
  resources: ["verticalpodautoscalers"]
  verbs: ["get", "list", "watch"]
```

**Why needed:**
- Detect existing autoscaling configurations
- Avoid resource adjustment conflicts
- Coordinate with VPA recommendations

### Resource Constraints

#### Quotas and Limits
Respect namespace and cluster constraints:

```yaml
- apiGroups: [""]
  resources: ["resourcequotas", "limitranges"]
  verbs: ["get", "list", "watch"]
  
- apiGroups: ["policy"]
  resources: ["poddisruptionbudgets"]
  verbs: ["get", "list", "watch"]
```

**Why needed:**
- Ensure resource changes comply with quotas
- Respect limit ranges
- Consider PodDisruptionBudgets during updates

### Storage Resources

#### Persistent Volume Claims
Monitor storage usage:

```yaml
- apiGroups: [""]
  resources: ["persistentvolumeclaims", "persistentvolumes"]
  verbs: ["get", "list", "watch"]
  
- apiGroups: ["storage.k8s.io"]
  resources: ["storageclasses"]
  verbs: ["get", "list", "watch"]
```

**Why needed:**
- Consider storage constraints in optimization
- Monitor PVC usage patterns
- Understand storage provisioning capabilities

### Networking

#### Services and Network Policies
Network configuration awareness:

```yaml
- apiGroups: [""]
  resources: ["services", "endpoints"]
  verbs: ["get", "list", "watch"]
  
- apiGroups: [""]
  resources: ["services"]
  verbs: ["create", "update", "patch"]  # For webhook services
  
- apiGroups: ["networking.k8s.io"]
  resources: ["networkpolicies"]
  verbs: ["get", "list", "watch"]
```

**Why needed:**
- Create webhook services (if admission control is enabled)
- Understand service dependencies
- Respect network segmentation

### Custom Resources

#### Right Sizer CRDs
Manage Right Sizer configurations:

```yaml
- apiGroups: ["rightsizer.io"]
  resources: ["rightsizerpolicies", "rightsizerconfigs"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
  
- apiGroups: ["rightsizer.io"]
  resources: ["rightsizerpolicies/status", "rightsizerconfigs/status"]
  verbs: ["get", "update", "patch"]
```

**Why needed:**
- Store and manage optimization policies
- Track configuration state
- Update resource status

### Admission Webhooks

#### Webhook Configuration
For admission control features:

```yaml
- apiGroups: ["admissionregistration.k8s.io"]
  resources: ["validatingwebhookconfigurations", "mutatingwebhookconfigurations"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

**Why needed:**
- Register admission webhooks
- Validate resource changes
- Mutate pod specifications

### Namespace-Scoped Permissions

#### Leader Election and Configuration
Permissions within the operator's namespace:

```yaml
# ConfigMaps and Secrets for configuration and TLS
- apiGroups: [""]
  resources: ["configmaps", "secrets"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

# Leader election
- apiGroups: ["coordination.k8s.io"]
  resources: ["leases"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

**Why needed:**
- Store operator configuration
- Manage TLS certificates for webhooks
- Coordinate multiple operator instances

## Installation

### Using Helm

The recommended way to install RBAC is through the Helm chart:

```bash
# Install with Helm
helm install right-sizer ./helm \
  --namespace right-sizer-system \
  --create-namespace

# Upgrade existing installation
helm upgrade right-sizer ./helm \
  --namespace right-sizer-system \
  --reuse-values
```

### Manual Installation

For manual installation or customization:

```bash
# Apply RBAC manifest directly
kubectl apply -f helm/templates/rbac.yaml

# Or use the automated script
./scripts/rbac/apply-rbac-fix.sh
```

### Custom Service Account

To use a custom service account name:

```yaml
# values.yaml
serviceAccount:
  create: true
  name: "custom-right-sizer-sa"
```

## Verification

### Automated Verification

Use the provided verification script:

```bash
# Run comprehensive permission check
./scripts/rbac/verify-permissions.sh

# Check specific namespace
NAMESPACE=my-namespace ./scripts/rbac/verify-permissions.sh
```

### Manual Verification

Check individual permissions:

```bash
# Set variables
NAMESPACE="right-sizer-system"
SA="right-sizer"

# Check core permissions
kubectl auth can-i list nodes \
  --as=system:serviceaccount:$NAMESPACE:$SA

kubectl auth can-i patch pods \
  --as=system:serviceaccount:$NAMESPACE:$SA

kubectl auth can-i get pods.metrics.k8s.io \
  --as=system:serviceaccount:$NAMESPACE:$SA

# Check all permissions for the service account
kubectl auth can-i --list \
  --as=system:serviceaccount:$NAMESPACE:$SA
```

### Viewing Applied RBAC

Inspect the current RBAC configuration:

```bash
# View ClusterRole
kubectl describe clusterrole right-sizer

# View ClusterRoleBinding
kubectl describe clusterrolebinding right-sizer

# View ServiceAccount
kubectl describe serviceaccount right-sizer -n right-sizer-system

# Export current RBAC
kubectl get clusterrole right-sizer -o yaml > current-rbac.yaml
```

## Troubleshooting

### Common Permission Errors

#### 1. Metrics API Access Denied

**Error:**
```
pods.metrics.k8s.io is forbidden: User "system:serviceaccount:right-sizer-system:right-sizer" cannot list resource "pods" in API group "metrics.k8s.io"
```

**Solution:**
- Ensure metrics-server is installed
- Verify metrics API permissions are granted
- Check both v1 and v1beta1 API versions

```bash
# Check metrics-server
kubectl get deployment metrics-server -n kube-system

# Verify API availability
kubectl api-versions | grep metrics
```

#### 2. Node List Forbidden

**Error:**
```
nodes is forbidden: User "system:serviceaccount:right-sizer-system:right-sizer" cannot list resource "nodes" in API group "" at the cluster scope
```

**Solution:**
- Ensure ClusterRole (not Role) is used for node permissions
- Verify ClusterRoleBinding is correctly configured

```bash
# Re-apply RBAC
kubectl apply -f helm/templates/rbac.yaml

# Restart deployment
kubectl rollout restart deployment/right-sizer -n right-sizer-system
```

#### 3. Pod Resize Not Working

**Error:**
```
pods "my-pod" is forbidden: User cannot patch resource "pods/resize"
```

**Solution:**
- Verify Kubernetes version supports resize (1.27+)
- Check resize subresource permissions

```bash
# Check K8s version
kubectl version --short

# Verify resize API
kubectl api-resources | grep resize
```

### Debug RBAC Issues

Enable verbose logging to debug permission issues:

```bash
# Increase log verbosity
kubectl set env deployment/right-sizer \
  -n right-sizer-system \
  LOG_LEVEL=debug

# View logs
kubectl logs -n right-sizer-system \
  deployment/right-sizer \
  --tail=100 -f | grep -i "forbidden\|denied\|unauthorized"
```

## Security Best Practices

### Principle of Least Privilege

The Right Sizer RBAC follows the principle of least privilege:

1. **No wildcard permissions** on core resources
2. **Read-only access** where write is not required
3. **Namespace-scoped** permissions where possible
4. **Explicit resource lists** instead of wildcards

### Restricted Permissions

The Right Sizer does **NOT** have permissions to:

- Delete pods or workloads
- Modify RBAC configurations (except its own webhooks)
- Access sensitive resources like secrets (except in its namespace)
- Create or delete namespaces
- Modify cluster-critical components

### Audit and Compliance

Monitor Right Sizer actions:

```bash
# Enable audit logging for the service account
kubectl get events --all-namespaces \
  --field-selector involvedObject.kind=ServiceAccount,involvedObject.name=right-sizer

# Review authentication logs
kubectl logs -n kube-system \
  -l component=kube-apiserver \
  | grep "right-sizer"
```

### Network Security

Implement network policies to restrict the Right Sizer:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: right-sizer-network-policy
  namespace: right-sizer-system
spec:
  podSelector:
    matchLabels:
      app: right-sizer
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector: {}  # Allow from pods in same namespace
    ports:
    - protocol: TCP
      port: 8443  # Webhook port
  egress:
  - to:
    - namespaceSelector: {}  # Allow to all namespaces
    ports:
    - protocol: TCP
      port: 443  # Kubernetes API
  - to:
    - podSelector:
        matchLabels:
          app: metrics-server
    ports:
    - protocol: TCP
      port: 443  # Metrics server
```

## Version Compatibility

### Kubernetes Version Requirements

| Kubernetes Version | Right Sizer Support | Notes |
|-------------------|-------------------|-------|
| 1.27+ | Full Support | In-place resize available |
| 1.24 - 1.26 | Supported | No in-place resize |
| 1.21 - 1.23 | Limited Support | Basic functionality only |
| < 1.21 | Not Supported | Missing required APIs |

### API Version Compatibility

The Right Sizer RBAC supports multiple API versions for compatibility:

- **Metrics API**: Both `v1` and `v1beta1`
- **Autoscaling**: `v1`, `v2`, `v2beta1`, `v2beta2`
- **Admission**: `v1` and `v1beta1`

### Feature Gates

Certain features require specific Kubernetes feature gates:

```yaml
# For in-place resize (K8s 1.27+)
InPlacePodVerticalScaling: true

# For ephemeral containers (debugging)
EphemeralContainers: true
```

## Additional Resources

- [Kubernetes RBAC Documentation](https://kubernetes.io/docs/reference/access-authn-authz/rbac/)
- [Right Sizer Configuration Guide](../CONFIGURATION.md)
- [Troubleshooting Guide](./TROUBLESHOOTING.md)
- [Security Best Practices](./SECURITY.md)

## Support

If you encounter RBAC issues not covered in this guide:

1. Run the verification script: `./scripts/rbac/verify-permissions.sh`
2. Check the [troubleshooting guide](./TROUBLESHOOTING.md)
3. Open an issue with the verification script output
4. Contact support with your cluster version and RBAC configuration
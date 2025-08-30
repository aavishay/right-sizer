# Troubleshooting Guide

This guide provides solutions for common issues encountered with the Right-Sizer operator.

## Table of Contents

- [Common Issues](#common-issues)
  - [RBAC Permission Errors](#rbac-permission-errors)
  - [Audit Log Permission Errors](#audit-log-permission-errors)
  - [Controller-Runtime Logger Warning](#controller-runtime-logger-warning)
  - [In-Place Resize Not Working](#in-place-resize-not-working)
  - [Metrics Server Not Available](#metrics-server-not-available)
- [Diagnostic Commands](#diagnostic-commands)
- [Monitoring and Debugging](#monitoring-and-debugging)
- [Recovery Procedures](#recovery-procedures)

## Common Issues

### RBAC Permission Errors

#### Symptoms
Common permission errors you might encounter:
```
nodes is forbidden: User "system:serviceaccount:right-sizer-system:right-sizer" cannot list resource "nodes" in API group "" at the cluster scope
```
```
pods.metrics.k8s.io is forbidden: User "system:serviceaccount:right-sizer-system:right-sizer" cannot list resource "pods" in API group "metrics.k8s.io"
```
```
unable to fetch pod metrics: Unauthorized
```

#### Cause
The service account lacks necessary permissions to access Kubernetes resources. This can happen when:
- The RBAC configuration is outdated
- Metrics API permissions are missing
- The cluster uses a different metrics API version (v1 vs v1beta1)
- Required subresources (like pods/resize) are not granted

#### Solution

##### Quick Fix Using Scripts
We provide automated scripts to fix RBAC issues:

```bash
# Apply RBAC fixes automatically
./scripts/rbac/apply-rbac-fix.sh

# Verify all permissions are correctly set
./scripts/rbac/verify-permissions.sh
```

##### Fix Using Helm
If you deployed using Helm, upgrade with the updated RBAC:
```bash
# Update RBAC permissions using Helm
helm upgrade right-sizer ./helm --reuse-values --namespace right-sizer-system --force
```

##### Manual Fix
Apply the complete RBAC configuration manually:
```bash
# Apply the updated RBAC manifest
kubectl apply -f helm/templates/rbac.yaml

# Restart the deployment to pick up new permissions
kubectl rollout restart deployment/right-sizer -n right-sizer-system
```

##### Verify Permissions
Check that the service account has all required permissions:
```bash
# Check critical permissions
kubectl auth can-i list nodes --as=system:serviceaccount:right-sizer-system:right-sizer
kubectl auth can-i list pods --as=system:serviceaccount:right-sizer-system:right-sizer
kubectl auth can-i patch pods --as=system:serviceaccount:right-sizer-system:right-sizer
kubectl auth can-i get pods.metrics.k8s.io --as=system:serviceaccount:right-sizer-system:right-sizer
```

The updated RBAC configuration includes:
- **Core Resources**: pods, nodes, events, namespaces (with all subresources)
- **Metrics API**: Both v1 and v1beta1 versions for compatibility
- **Workload Controllers**: deployments, statefulsets, daemonsets, replicasets (including scale subresource)
- **Autoscaling**: HPA and VPA resources
- **Resource Constraints**: quotas, limitranges, PodDisruptionBudgets
- **Storage**: PVCs, PVs, StorageClasses
- **Networking**: services, endpoints, NetworkPolicies
- **Scheduling**: PriorityClasses
- **Custom Resources**: RightSizerPolicies and RightSizerConfigs
- **Pod Resize**: pods/resize subresource for in-place resizing (K8s 1.27+)
# Troubleshooting Guide

This guide provides solutions for common issues encountered with the Right-Sizer operator.

## Table of Contents

- [Common Issues](#common-issues)
  - [RBAC Permission Errors](#rbac-permission-errors)
  - [Audit Log Permission Errors](#audit-log-permission-errors)
  - [Controller-Runtime Logger Warning](#controller-runtime-logger-warning)
  - [In-Place Resize Not Working](#in-place-resize-not-working)
  - [Metrics Server Not Available](#metrics-server-not-available)
  - [Pods Being Restarted Instead of Resized](#pods-being-restarted-instead-of-resized)
  - [Excessive Log Spam for No-Op Operations](#excessive-log-spam-for-no-op-operations)
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

### Pods Being Restarted Instead of Resized

#### Symptoms
- Pods are being recreated with new ReplicaSets when resources are adjusted
- Rolling updates triggered on Deployments/StatefulSets
- Service disruptions during resource optimization
- Multiple old ReplicaSets shown in deployment history:
```
OldReplicaSets: demo-nginx-59dbbffc4d (0/0 replicas created), demo-nginx-56dd6c9bfb (0/0 replicas created)...
```

#### Cause
This critical issue occurred in versions prior to 0.1.1 where the RightSizerPolicy controller was updating Deployment/StatefulSet/DaemonSet resources directly, which triggers rolling updates and pod restarts.

#### Solution

##### Upgrade to Version 0.1.1 or Later
The issue has been fixed in commit `b74390a`. The operator now:
- Never updates workload controllers (Deployments, StatefulSets, DaemonSets)
- Only performs in-place pod resizing directly
- Guarantees zero-downtime resource optimization

##### Verify the Fix
Check that pods are being resized without restarts:
```bash
# Monitor a pod for restarts
kubectl get pod <pod-name> -n <namespace> --watch

# Check restart count
kubectl describe pod <pod-name> -n <namespace> | grep "Restart Count"

# Verify deployment template hasn't changed
kubectl get deployment <deployment-name> -n <namespace> -o jsonpath='{.spec.template.spec.containers[0].resources}' | jq
```

##### Prevention
- Always use in-place resize when available (Kubernetes 1.27+)
- Never modify deployment specs for resource updates
- Monitor pod restart counts in production

### Excessive Log Spam for No-Op Operations

#### Symptoms
- Repeated log entries showing no actual changes:
```
Successfully resized pod (CPU only: 108m→108m, memory decrease skipped)
Found 1 resources needing adjustment
[REPEATED EVERY 30 SECONDS]
```
- Logs cluttered with operations that don't modify resources
- Difficulty troubleshooting actual issues

#### Cause
Prior to version 0.1.1 (fixed in commit `d9ecfb6`), the operator would:
- Attempt to resize pods even when resources were already at target values
- Log success messages for no-op operations
- Process pods that couldn't be modified due to Kubernetes limitations

#### Solution

##### Upgrade to Version 0.1.1 or Later
The fix includes:
- Comprehensive no-op detection before API calls
- Skipping operations where neither CPU nor memory would change
- Suppressing logs for skipped operations
- Comparing actual pod resources to detect true changes

##### Verify Clean Logging
Monitor logs to ensure only actual changes are logged:
```bash
# Watch operator logs
kubectl logs -n right-sizer deploy/right-sizer -f

# Check for repeated entries
kubectl logs -n right-sizer deploy/right-sizer --since=5m | grep "108m→108m" | wc -l
```

##### Best Practices
- Set appropriate resize intervals to reduce unnecessary checks
- Use namespace filters to skip pods that shouldn't be touched
- Enable debug logging only when troubleshooting
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
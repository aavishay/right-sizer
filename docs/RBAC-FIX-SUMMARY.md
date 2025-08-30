# Right Sizer RBAC Permissions Fix - Summary

## Overview

This document summarizes the comprehensive RBAC (Role-Based Access Control) permissions fix implemented for the Right Sizer Kubernetes operator to resolve authorization issues preventing proper operation.

## Issues Identified

### 1. Core Permission Errors
- **Node Access Denied**: Service account could not list or get nodes
- **Metrics API Forbidden**: Unable to access pod and node metrics from metrics-server
- **Pod Resize Subresource**: Missing permissions for in-place pod resizing (K8s 1.27+)

### 2. API Version Compatibility
- Metrics API has evolved from v1beta1 to v1 in newer Kubernetes versions
- RBAC configuration only referenced generic API groups without version specificity
- Missing explicit permissions for `podmetrics` and `nodemetrics` resources

### 3. Missing Resource Permissions
- Storage resources (PVCs, PVs, StorageClasses) not included
- Network policies permissions absent
- PriorityClasses for scheduling information missing
- Scale subresource for workload controllers not granted

## Fixes Implemented

### 1. Enhanced RBAC Configuration

#### Updated Metrics API Permissions
```yaml
# Added comprehensive metrics permissions
- apiGroups: ["metrics.k8s.io"]
  resources: ["pods", "nodes", "podmetrics", "nodemetrics"]
  verbs: ["get", "list", "watch"]  # Added 'watch' verb
```

#### Added Storage Permissions
```yaml
# Storage resources for validation
- apiGroups: [""]
  resources: ["persistentvolumeclaims", "persistentvolumes"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["storage.k8s.io"]
  resources: ["storageclasses"]
  verbs: ["get", "list", "watch"]
```

#### Added Scheduling and Networking
```yaml
# Scheduling priorities
- apiGroups: ["scheduling.k8s.io"]
  resources: ["priorityclasses"]
  verbs: ["get", "list", "watch"]

# Network policies
- apiGroups: ["networking.k8s.io"]
  resources: ["networkpolicies"]
  verbs: ["get", "list", "watch"]
```

#### Scale Subresource Permissions
```yaml
# Workload controller scaling
- apiGroups: ["apps"]
  resources: ["deployments/scale", "statefulsets/scale", "replicasets/scale"]
  verbs: ["get", "patch", "update"]
```

### 2. Automation Scripts Created

#### Verification Script
**Path**: `scripts/rbac/verify-permissions.sh`
- Comprehensive permission checking for all required resources
- Validates both cluster-scoped and namespace-scoped permissions
- Provides detailed pass/fail report for each permission
- Supports custom namespace and service account testing

#### Fix Application Script
**Path**: `scripts/rbac/apply-rbac-fix.sh`
- Automatically applies RBAC fixes via Helm or kubectl
- Restarts deployments to pick up new permissions
- Performs quick verification after application
- Supports both Helm-managed and standalone deployments

#### Integration Test Script
**Path**: `tests/rbac/rbac-integration-test.sh`
- End-to-end RBAC testing in real Kubernetes environment
- Tests positive and negative permissions
- Validates practical scenarios with test deployments
- Generates comprehensive test report

### 3. Documentation Updates

#### New Documentation
- **`docs/RBAC.md`**: Comprehensive RBAC guide with detailed permission explanations
- **`docs/RBAC-FIX-SUMMARY.md`**: This summary document
- **`CHANGELOG.md`**: Added changelog entry for RBAC fixes

#### Updated Documentation
- **`docs/TROUBLESHOOTING.md`**: Enhanced RBAC troubleshooting section
- **`README.md`**: Added RBAC quick fix instructions

### 4. Helm Chart Updates

**File**: `helm/templates/rbac.yaml`
- Added missing permissions for metrics, storage, networking
- Enhanced compatibility with different Kubernetes versions
- Improved permission granularity following least-privilege principle
- Added comprehensive comments explaining each permission set

## How to Apply the Fixes

### Quick Fix (Recommended)
```bash
# Apply RBAC fixes automatically
./scripts/rbac/apply-rbac-fix.sh

# Verify permissions are correct
./scripts/rbac/verify-permissions.sh
```

### Using Helm
```bash
# Upgrade existing installation
helm upgrade right-sizer ./helm \
  --namespace right-sizer-system \
  --reuse-values \
  --force

# Fresh installation
helm install right-sizer ./helm \
  --namespace right-sizer-system \
  --create-namespace
```

### Manual Application
```bash
# Apply RBAC manifest directly
kubectl apply -f helm/templates/rbac.yaml

# Restart deployment to pick up new permissions
kubectl rollout restart deployment/right-sizer -n right-sizer-system

# Check rollout status
kubectl rollout status deployment/right-sizer -n right-sizer-system
```

## Verification

### Quick Verification
```bash
# Check critical permissions
kubectl auth can-i list nodes \
  --as=system:serviceaccount:right-sizer-system:right-sizer

kubectl auth can-i patch pods \
  --as=system:serviceaccount:right-sizer-system:right-sizer

kubectl auth can-i get pods.metrics.k8s.io \
  --as=system:serviceaccount:right-sizer-system:right-sizer
```

### Comprehensive Testing
```bash
# Run full integration test
./tests/rbac/rbac-integration-test.sh

# Run with verbose output
./tests/rbac/rbac-integration-test.sh --verbose

# Keep test resources for debugging
./tests/rbac/rbac-integration-test.sh --no-cleanup
```

## Key Improvements

### 1. Security Enhancements
- Maintains principle of least privilege
- No wildcard permissions on critical resources
- Explicit resource and verb specifications
- Namespace-scoped permissions where appropriate

### 2. Compatibility Improvements
- Support for both metrics.k8s.io v1 and v1beta1
- Kubernetes 1.21+ through 1.33+ compatibility
- Feature detection for optional resources (VPA, resize subresource)
- Graceful handling of missing APIs

### 3. Operational Benefits
- Automated verification and fixing procedures
- Comprehensive documentation and troubleshooting guides
- Integration tests for validation
- Clear error messages and resolution steps

## Impact

### Before Fix
- Service account lacked permissions to access nodes and metrics
- Pod resizing operations failed with authorization errors
- Unable to properly monitor and optimize resources
- Errors in logs: "forbidden", "cannot list resource", "unauthorized"

### After Fix
- Full access to required Kubernetes resources
- Successful metrics collection from all sources
- In-place pod resizing works (when supported)
- Clean operation with no permission errors

## Monitoring

### Check for Permission Errors
```bash
# Monitor logs for permission issues
kubectl logs -n right-sizer-system deployment/right-sizer -f | \
  grep -i "forbidden\|denied\|unauthorized"

# Check events for RBAC issues
kubectl get events -n right-sizer-system \
  --field-selector reason=Forbidden
```

### Audit Service Account Actions
```bash
# List all permissions for service account
kubectl auth can-i --list \
  --as=system:serviceaccount:right-sizer-system:right-sizer

# Check specific resource access
kubectl auth can-i '*' pods \
  --as=system:serviceaccount:right-sizer-system:right-sizer
```

## Best Practices

1. **Regular Verification**: Run `verify-permissions.sh` after cluster upgrades
2. **Version Compatibility**: Check Kubernetes version compatibility before deployment
3. **Audit Logging**: Enable audit logging to track service account actions
4. **Least Privilege**: Review and adjust permissions based on actual usage
5. **Documentation**: Keep RBAC documentation updated with any changes

## Troubleshooting

If permission issues persist after applying fixes:

1. **Verify RBAC Application**
   ```bash
   kubectl get clusterrole right-sizer -o yaml
   kubectl get clusterrolebinding right-sizer -o yaml
   ```

2. **Check Service Account**
   ```bash
   kubectl get sa right-sizer -n right-sizer-system -o yaml
   ```

3. **Test Specific Permissions**
   ```bash
   ./scripts/rbac/verify-permissions.sh
   ```

4. **Review Deployment Logs**
   ```bash
   kubectl logs -n right-sizer-system deployment/right-sizer --tail=50
   ```

5. **Run Integration Tests**
   ```bash
   ./tests/rbac/rbac-integration-test.sh --verbose
   ```

## Related Files

- **RBAC Configuration**: `helm/templates/rbac.yaml`
- **Verification Script**: `scripts/rbac/verify-permissions.sh`
- **Fix Script**: `scripts/rbac/apply-rbac-fix.sh`
- **Integration Test**: `tests/rbac/rbac-integration-test.sh`
- **Documentation**: `docs/RBAC.md`, `docs/TROUBLESHOOTING.md`
- **Changelog**: `CHANGELOG.md`

## Support

For additional help or to report issues:

1. Run the verification script and save output
2. Check the troubleshooting guide
3. Open an issue with verification script output and cluster details
4. Contact support with specific error messages and environment information

## Conclusion

The RBAC permissions fix ensures the Right Sizer operator has all necessary permissions to effectively monitor, analyze, and optimize Kubernetes workloads while maintaining security best practices. The comprehensive testing and documentation provide confidence in the solution's reliability and maintainability.
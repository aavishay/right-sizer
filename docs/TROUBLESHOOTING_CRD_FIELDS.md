# Troubleshooting: CRD Field Validation Issues

## Problem Description

If you're seeing errors like these in your right-sizer pod logs:

```
{"level":"info","ts":1756578764.9487,"caller":"log/warning_handler.go:64","msg":"unknown field \"spec.defaultResourceStrategy\"","controller":"rightsizerconfig","controller
{"level":"info","ts":1756578764.9487605,"caller":"log/warning_handler.go:64","msg":"unknown field \"spec.globalConstraints\"","controller":"rightsizerconfig","controllerGro
{"level":"info","ts":1756578764.9487658,"caller":"log/warning_handler.go:64","msg":"unknown field \"spec.metricsConfig\"","controller":"rightsizerconfig","controllerGroup":
{"level":"info","ts":1756578764.948769,"caller":"log/warning_handler.go:64","msg":"unknown field \"spec.namespaceConfig\"","controller":"rightsizerconfig","controllerGroup"
{"level":"info","ts":1756578764.9487715,"caller":"log/warning_handler.go:64","msg":"unknown field \"spec.notificationConfig\"","controller":"rightsizerconfig","controllerGr
{"level":"info","ts":1756578764.9487746,"caller":"log/warning_handler.go:64","msg":"unknown field \"spec.observabilityConfig\"","controller":"rightsizerconfig","controllerG
{"level":"info","ts":1756578764.948777,"caller":"log/warning_handler.go:64","msg":"unknown field \"spec.operatorConfig\"","controller":"rightsizerconfig","controllerGroup":
{"level":"info","ts":1756578764.9487793,"caller":"log/warning_handler.go:64","msg":"unknown field \"spec.securityConfig\"","controller":"rightsizerconfig","controllerGroup"
{"level":"info","ts":1756578764.948781,"caller":"log/warning_handler.go:64","msg":"unknown field \"status.lastAppliedTime\"","controller":"rightsizerconfig","controllerGrou
{"level":"info","ts":1756578764.9487832,"caller":"log/warning_handler.go:64","msg":"unknown field \"status.systemHealth\"","controller":"rightsizerconfig","controllerGroup"
```

This indicates that you're using simplified CRD definitions that use `x-kubernetes-preserve-unknown-fields: true` instead of properly structured field schemas.

## Root Cause

There are two versions of CRD definitions in earlier versions:

1. **Simplified versions** (`rightsizerconfig.yaml`, `rightsizerpolicy.yaml`) - These use `x-kubernetes-preserve-unknown-fields: true` which prevents field validation
2. **Full versions** (`rightsizer.io_rightsizerconfigs.yaml`, `rightsizer.io_rightsizerpolicies.yaml`) - These have complete field schemas with proper validation

The controller expects the fields to be properly defined according to the full schema, but if the simplified CRDs are installed, Kubernetes treats the fields as unstructured data, causing the "unknown field" warnings.

## Quick Fix

### Method 1: Use the Fix Script

We provide a script to automatically fix this issue:

```bash
# Fix the CRDs
./scripts/fix-crd-fields.sh

# Fix CRDs in a custom namespace
./scripts/fix-crd-fields.sh --namespace my-namespace

# Dry run to see what would be done
./scripts/fix-crd-fields.sh --dry-run
```

The script will:
1. Backup your existing CRDs and resources
2. Replace the simplified CRDs with the full versions
3. Restart the operator to apply changes
4. Verify the fix worked

### Method 2: Manual Fix

1. **Apply the correct CRD definitions:**
   ```bash
   kubectl apply -f helm/crds/rightsizer.io_rightsizerconfigs.yaml
   kubectl apply -f helm/crds/rightsizer.io_rightsizerpolicies.yaml
   ```

2. **Wait for CRDs to be established:**
   ```bash
   kubectl wait --for=condition=Established --timeout=60s \
     crd/rightsizerconfigs.rightsizer.io \
     crd/rightsizerpolicies.rightsizer.io
   ```

3. **Restart the operator:**
   ```bash
   kubectl rollout restart deployment/right-sizer -n right-sizer-system
   kubectl rollout status deployment/right-sizer -n right-sizer-system
   ```

4. **Verify the fix:**
   ```bash
   # Check operator logs for errors
   kubectl logs -n right-sizer-system -l app.kubernetes.io/name=right-sizer --tail=50
   ```

## Prevention

### For New Installations

Always use the correct CRD files when installing:

```bash
# Use the install script
./scripts/install-crds.sh

# Or manually install the correct files
kubectl apply -f helm/crds/rightsizer.io_rightsizerconfigs.yaml
kubectl apply -f helm/crds/rightsizer.io_rightsizerpolicies.yaml
```

### For Helm Installations

When installing with Helm, the correct CRDs should be installed automatically:

```bash
helm install right-sizer ./helm \
  --namespace right-sizer-system \
  --create-namespace
```

## Verification

### Check CRD Schema

Verify that your CRDs are using the correct schema:

```bash
# This should NOT show "x-kubernetes-preserve-unknown-fields: true"
kubectl get crd rightsizerconfigs.rightsizer.io -o yaml | grep "x-kubernetes-preserve-unknown-fields"

# Check that fields are properly defined
kubectl explain rightsizerconfig.spec.defaultResourceStrategy
kubectl explain rightsizerconfig.spec.globalConstraints
```

### Check Operator Logs

After fixing, verify no more "unknown field" errors:

```bash
kubectl logs -n right-sizer-system -l app.kubernetes.io/name=right-sizer --tail=100 | grep -i "unknown field"
```

## Backup Recovery

If you used the fix script, your backups are stored in `/tmp/right-sizer-crd-backup-[timestamp]`.

To restore from backup:

```bash
# Find your backup directory
ls -la /tmp/right-sizer-crd-backup-*

# Restore CRDs
kubectl apply -f /tmp/right-sizer-crd-backup-[timestamp]/rightsizerconfigs.yaml
kubectl apply -f /tmp/right-sizer-crd-backup-[timestamp]/rightsizerpolicies.yaml

# Restore resources if needed
kubectl apply -f /tmp/right-sizer-crd-backup-[timestamp]/rightsizerconfigs-resources.yaml
kubectl apply -f /tmp/right-sizer-crd-backup-[timestamp]/rightsizerpolicies-resources.yaml
```

## Impact

These "unknown field" warnings are generally non-fatal but can cause:

- Confusion in logs
- Potential issues with field validation
- Some features may not work as expected if fields aren't properly recognized

The warnings don't prevent the operator from functioning, but fixing them ensures proper field validation and cleaner logs.

## Additional Help

If you continue to see issues after applying the fix:

1. Check that you're using the latest version of the operator
2. Ensure no conflicting CRD definitions exist
3. Check for any custom modifications to the CRDs
4. Review the operator deployment configuration

For further assistance, please provide:
- Operator version: `kubectl get deployment -n right-sizer-system right-sizer -o jsonpath='{.spec.template.spec.containers[0].image}'`
- CRD version: `kubectl get crd rightsizerconfigs.rightsizer.io -o jsonpath='{.metadata.annotations.controller-gen\.kubebuilder\.io/version}'`
- Full operator logs: `kubectl logs -n right-sizer-system -l app.kubernetes.io/name=right-sizer --tail=200`

# Kubernetes 1.33 In-Place Pod Resize Implementation

## Overview

This document summarizes the upgrade of the right-sizer operator to support Kubernetes 1.33's native in-place pod resize feature. This feature allows pods to have their CPU and memory resources adjusted without requiring a restart, providing true zero-downtime resource optimization.

## Key Changes Implemented

### 1. Core Implementation Updates

#### **InPlaceRightSizer Controller (`controllers/inplace_rightsizer.go`)**
- Updated to use the Kubernetes `resize` subresource API introduced in v1.33
- Added `ClientSet` and `RestConfig` fields for direct API access
- Implemented `applyInPlaceResize()` method that uses the resize subresource:
  ```go
  _, err = r.ClientSet.CoreV1().Pods(pod.Namespace).Patch(
      ctx,
      pod.Name,
      types.StrategicMergePatchType,
      patchData,
      metav1.PatchOptions{},
      "resize", // Specifying the resize subresource
  )
  ```
- Added fallback mechanism for clusters without resize support
- Enhanced pod filtering to check for resize policy support
- Improved resource calculation to handle per-container metrics

### 2. Dependency Updates

#### **go.mod**
- Updated Kubernetes libraries to v0.31.0 (compatible with K8s 1.33):
  - `k8s.io/api v0.31.0`
  - `k8s.io/apimachinery v0.31.0`
  - `k8s.io/client-go v0.31.0`
- Updated controller-runtime to v0.19.0
- Updated all transitive dependencies for compatibility

### 3. RBAC Permissions

#### **rbac.yaml** and **Helm Chart RBAC**
Added new permissions required for in-place resize:
- `pods/resize` resource with `patch` verb (critical for K8s 1.33)
- `pods/status` for reading resize status
- `events` for creating resize events
- Additional resources for comprehensive monitoring:
  - `horizontalpodautoscalers` - to avoid conflicts with HPA
  - `verticalpodautoscalers` - to avoid conflicts with VPA
  - `poddisruptionbudgets` - for safe operations
  - `resourcequotas`, `limitranges` - to respect namespace limits

### 4. Documentation Updates

#### **README.md**
- Added comprehensive section on Kubernetes 1.33+ support
- Included verification steps for in-place resize
- Updated Minikube instructions to use K8s v1.33.1
- Added troubleshooting section for resize-specific issues

### 5. Testing and Examples

#### **test-inplace-resize.sh**
Created comprehensive test script that:
- Verifies Kubernetes version and resize subresource availability
- Creates test deployment with initial resources
- Performs in-place resize using the resize subresource
- Verifies no pod restart occurred
- Checks pod events and resize status

#### **examples/in-place-resize-demo.yaml**
Created detailed example showing:
- Deployment with resize policy configuration
- ConfigMap for right-sizer configuration
- Examples of pods that should/shouldn't be resized
- HPA integration example
- Verification scripts for monitoring resize operations

## Technical Details

### How In-Place Resize Works

1. **Detection**: The operator checks if pods support in-place resize by examining their resize policies
2. **Metrics Collection**: Fetches current resource usage from metrics API
3. **Calculation**: Determines optimal resources with configurable buffers
4. **Application**: Uses the resize subresource to patch pod resources
5. **Verification**: Monitors resize status without pod restarts

### Resize Patch Structure

```json
{
  "spec": {
    "containers": [
      {
        "name": "container-name",
        "resources": {
          "requests": {
            "cpu": "150m",
            "memory": "192Mi"
          },
          "limits": {
            "cpu": "300m",
            "memory": "384Mi"
          }
        }
      }
    ]
  }
}
```

## Verification Steps

### 1. Check Kubernetes Version
```bash
kubectl version --short
# Should show v1.33.0 or higher
```

### 2. Verify Resize Subresource
```bash
kubectl api-resources | grep pods/resize
# Should show: pods/resize  true  Pod
```

### 3. Test Manual Resize
```bash
kubectl patch pod <pod-name> --subresource resize --patch \
  '{"spec": {"containers": [{"name": "<container>", "resources": {...}}]}}'
```

### 4. Monitor Operator Logs
```bash
kubectl logs -l app=right-sizer -f
# Look for: "Successfully resized pod ... using resize subresource (no restart)"
```

### 5. Verify No Restart
```bash
# Before resize
kubectl get pod <pod-name> -o jsonpath='{.status.containerStatuses[0].restartCount}'

# After resize (should be same value)
kubectl get pod <pod-name> -o jsonpath='{.status.containerStatuses[0].restartCount}'
```

## Benefits

1. **Zero Downtime**: Pods continue serving traffic during resize
2. **Faster Optimization**: No need to wait for pod recreation
3. **Preserved State**: In-memory state and connections maintained
4. **Better User Experience**: No service disruptions
5. **Resource Efficiency**: Immediate resource adjustments

## Compatibility

- **Kubernetes 1.33+**: Full in-place resize support via resize subresource
- **Kubernetes < 1.33**: Falls back to traditional patch (may cause restarts)
- **Minikube**: Tested with Minikube using `--kubernetes-version=v1.33.1`
- **Cloud Providers**: Compatible with any K8s 1.33+ cluster

## Known Limitations

1. Some containers may still restart if they don't support dynamic resource changes
2. Init containers cannot be resized in-place
3. Ephemeral containers are not supported for resize
4. Some resource changes may require container cooperation to take full effect

## Migration Guide

For existing right-sizer deployments:

1. **Update Kubernetes cluster** to v1.33 or higher
2. **Update kubectl** to v1.33+ for manual operations
3. **Apply new RBAC** permissions (especially `pods/resize`)
4. **Rebuild and deploy** the updated operator
5. **Monitor logs** to confirm resize subresource is being used

## Testing Checklist

- [x] Updated Go dependencies to Kubernetes v0.31.0
- [x] Implemented resize subresource API calls
- [x] Added fallback for older Kubernetes versions
- [x] Updated RBAC with resize permissions
- [x] Created comprehensive test script
- [x] Added example configurations
- [x] Updated documentation
- [x] Verified with kubectl v1.33.4

## References

- [Kubernetes v1.33 Release Notes](https://kubernetes.io/blog/2024/12/18/kubernetes-v1-33-release/)
- [In-Place Pod Resize KEP](https://github.com/kubernetes/enhancements/tree/master/keps/sig-node/1287-in-place-update-pod-resources)
- [Kubernetes Blog: In-Place Pod Resize Beta](https://kubernetes.io/blog/2025/05/16/kubernetes-v1-33-in-place-pod-resize-beta/)
- [Dynamic Pod Resizing Without Restarts](https://medium.com/@anbu.gn/kubernetes-1-33-dynamic-pod-resizing-without-restarts-2ece42f0c193)

## Support

For issues or questions about the Kubernetes 1.33 in-place resize feature:

1. Check the operator logs: `kubectl logs -l app=right-sizer`
2. Verify resize subresource availability: `kubectl api-resources | grep resize`
3. Test with the provided script: `./test-inplace-resize.sh`
4. Review pod events: `kubectl describe pod <pod-name>`

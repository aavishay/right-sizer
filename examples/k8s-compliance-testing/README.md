# Kubernetes 1.33+ In-Place Resize Compliance Testing Framework

This directory contains a comprehensive testing framework to verify that the right-sizer operator complies with Kubernetes 1.33+ in-place pod resizing specifications.

## 📋 Overview

The Kubernetes 1.33+ release introduced in-place pod resizing capabilities that allow changing container resource requests and limits without restarting pods. This testing framework ensures that right-sizer properly implements all required and recommended features from the [official Kubernetes documentation](https://kubernetes.io/docs/tasks/configure-pod-container/resize-container-resources/).

### 🎯 What This Framework Tests

#### ✅ **MANDATORY Requirements (K8s Spec)**
- **Resize Subresource Usage** - Uses `kubectl patch --subresource=resize` API
- **Container Resize Policies** - Supports `NotRequired` and `RestartContainer` policies  
- **Pod Resize Status Conditions** - Sets `PodResizePending` and `PodResizeInProgress` conditions
- **QoS Class Preservation** - Maintains original QoS class during resize operations
- **Memory Decrease Handling** - Best-effort for `NotRequired`, guaranteed for `RestartContainer`
- **Resource Validation** - Validates limits >= requests, positive values, node capacity
- **Error Handling** - Proper handling of infeasible resizes with appropriate status

#### 🟡 **RECOMMENDED Requirements**
- **ObservedGeneration Tracking** - Tracks `metadata.generation` in status and conditions
- **Deferred Resize Retry Logic** - Retry mechanism for temporarily impossible resizes
- **Priority-based Retry** - Higher priority pods get retry preference

#### 🔧 **Right-Sizer Integration**
- **Operator Integration** - Right-sizer uses proper K8s resize APIs
- **Policy Management** - Automatic resize policy injection
- **Workload Processing** - Deployments, StatefulSets, DaemonSets support

## 🚀 Quick Start

### Prerequisites

1. **Kubernetes Cluster (1.33+)**
   ```bash
   kubectl version --short
   # Server Version: v1.33.0+ required
   ```

2. **Feature Gates Enabled**
   ```yaml
   # Required feature gate
   InPlacePodVerticalScaling: true
   ```

3. **kubectl Client (1.32+)**
   ```bash
   kubectl version --client --short
   # Client Version: v1.32.0+ required for --subresource=resize
   ```

4. **Right-sizer Operator** (optional for integration tests)
   ```bash
   helm install right-sizer ../../helm \
     --set config.updateResizePolicy=true \
     --set config.patchResizePolicy=true
   ```

### Running Tests

#### Option 1: Quick Compliance Check
```bash
# Run basic compliance verification
cd right-sizer/
./scripts/check-k8s-compliance.sh
```

#### Option 2: Comprehensive Test Suite
```bash
# Run full compliance test suite
cd right-sizer/examples/k8s-compliance-testing/
./run-compliance-tests.sh
```

#### Option 3: Makefile Targets
```bash
# From right-sizer root directory
make test-k8s-compliance        # Quick compliance check
make test-compliance-full       # Full test suite
make test-inplace-resize        # In-place resize specific tests
```

#### Option 4: Individual Test Categories
```bash
# Unit tests
cd right-sizer/tests/unit/
go test -v -run TestResizePolicyValidation
go test -v -run TestQoSClassPreservationValidation

# Integration tests
cd right-sizer/tests/integration/
go test -v -tags=integration -run TestK8sSpecCompliance
go test -v -tags=integration -run TestK8sInPlaceResizeCompliance
```

## 📁 Framework Components

### Test Files

| File | Purpose |
|------|---------|
| `test-pods.yaml` | Test pod manifests with various resize configurations |
| `run-compliance-tests.sh` | Comprehensive test execution script |
| `README.md` | This documentation |

### Supporting Infrastructure

| File | Purpose |
|------|---------|
| `../../tests/integration/k8s_spec_compliance_check.go` | Go integration tests |
| `../../tests/integration/k8s_inplace_resize_compliance_test.go` | Detailed compliance tests |
| `../../tests/unit/resize_policy_validation_test.go` | Unit tests for validation logic |
| `../../scripts/check-k8s-compliance.sh` | Quick compliance check script |

## 🧪 Test Scenarios

### 1. Basic In-Place Resize Tests
```yaml
# Test Pod: basic-resize-test
- CPU resize with NotRequired policy (no restart)
- Memory resize with NotRequired policy (no restart)
- Resource validation and application
```

### 2. Container Restart Policy Tests
```yaml
# Test Pod: mixed-policy-test
- CPU: NotRequired policy → no container restart
- Memory: RestartContainer policy → container restart
- Mixed resource changes → restart if any requires it
```

### 3. QoS Class Preservation Tests
```yaml
# Test Pods: guaranteed-qos-test, burstable-qos-test, besteffort-qos-test
- Guaranteed QoS: requests = limits maintained
- Burstable QoS: requests < limits maintained  
- BestEffort QoS: no resources → cannot add resources
- Invalid QoS transitions → rejected
```

### 4. Memory Decrease Handling Tests
```yaml
# Test Pods: memory-decrease-test, memory-decrease-restart-test
- NotRequired policy: best-effort memory decrease
- RestartContainer policy: guaranteed memory decrease with restart
- OOM kill prevention validation
```

### 5. Error Handling Tests
```yaml
# Test Pod: large-resources-test  
- Infeasible resource requests → PodResizePending condition
- Node capacity validation → proper error messages
- Invalid configurations → rejected with clear errors
```

### 6. Multi-Container Tests
```yaml
# Test Pod: multi-container-test
- Independent container resize policies
- Mixed restart policies in same pod
- Sidecar container handling
```

### 7. Right-Sizer Integration Tests
```yaml
# Test Resources: rightsizer-integration-test deployment
- Automatic resize policy injection
- Workload optimization with proper K8s APIs
- Policy-based resource adjustments
```

## 📊 Understanding Test Results

### Compliance Levels

| Score | Level | Description |
|-------|-------|-------------|
| 80-100% | 🟢 **HIGHLY COMPLIANT** | Minor issues, production ready |
| 60-79% | 🟡 **PARTIALLY COMPLIANT** | Several features missing |
| 0-59% | 🔴 **NON_COMPLIANT** | Major features missing |

### Result Interpretation

```bash
# Example output
🔍 Kubernetes 1.33+ In-Place Resize Compliance Check
================================================================

✅ PASS: Resize Subresource Support - Right-sizer correctly uses resize subresource API
✅ PASS: Container Resize Policy - NotRequired and RestartContainer policies work correctly  
❌ FAIL: Pod Resize Status Conditions - PodResizePending/PodResizeInProgress not implemented
⚠️  WARN: QoS Class Preservation - Basic validation present but not comprehensive
✅ PASS: Memory Decrease Handling - Proper handling based on restart policy

Overall Compliance Score: 75%
Status: PARTIALLY COMPLIANT - Several features need implementation
```

### JSON Report Format

Tests generate detailed JSON reports:

```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "test_environment": {
    "kubernetes_version": "v1.33.0",
    "kubectl_version": "v1.33.0"
  },
  "summary": {
    "total_tests": 15,
    "passed_tests": 10,
    "failed_tests": 3,
    "warnings": 2,
    "compliance_percentage": 75
  },
  "test_results": [
    {
      "test": "basic-resize",
      "status": "PASS", 
      "message": "CPU and memory resize successful"
    }
  ],
  "compliance_status": "PARTIALLY_COMPLIANT"
}
```

## 🔧 Manual Testing Examples

### Example 1: Test Basic CPU Resize

```bash
# Apply test pod
kubectl apply -f test-pods.yaml

# Wait for pod to be ready
kubectl wait --for=condition=Ready pod/basic-resize-test -n k8s-compliance-test

# Check initial resources
kubectl get pod basic-resize-test -n k8s-compliance-test -o yaml | grep -A 10 "resources:"

# Perform CPU resize
kubectl patch pod basic-resize-test -n k8s-compliance-test --subresource resize --patch \
  '{"spec":{"containers":[{"name":"test-container", "resources":{"requests":{"cpu":"150m"}, "limits":{"cpu":"300m"}}}]}}'

# Verify resize was applied
kubectl get pod basic-resize-test -n k8s-compliance-test -o yaml | grep -A 10 "resources:"

# Check container was not restarted (for NotRequired policy)
kubectl get pod basic-resize-test -n k8s-compliance-test -o jsonpath='{.status.containerStatuses[0].restartCount}'
```

### Example 2: Test Memory Resize with Restart

```bash
# Apply pod with RestartContainer memory policy
kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: memory-restart-test
spec:
  containers:
  - name: test-container
    image: registry.k8s.io/pause:3.8
    resources:
      requests: {cpu: "100m", memory: "128Mi"}
      limits: {cpu: "200m", memory: "256Mi"}
    resizePolicy:
    - {resourceName: memory, restartPolicy: RestartContainer}
EOF

# Get initial restart count
INITIAL_RESTART_COUNT=$(kubectl get pod memory-restart-test -o jsonpath='{.status.containerStatuses[0].restartCount}')

# Resize memory
kubectl patch pod memory-restart-test --subresource resize --patch \
  '{"spec":{"containers":[{"name":"test-container", "resources":{"requests":{"memory":"256Mi"}, "limits":{"memory":"512Mi"}}}]}}'

# Wait and check if container was restarted
sleep 10
NEW_RESTART_COUNT=$(kubectl get pod memory-restart-test -o jsonpath='{.status.containerStatuses[0].restartCount}')

if [ "$NEW_RESTART_COUNT" -gt "$INITIAL_RESTART_COUNT" ]; then
  echo "✅ Container was restarted as expected"
else
  echo "❌ Container was not restarted"
fi
```

### Example 3: Test Infeasible Resize

```bash
# Attempt to resize beyond node capacity
kubectl patch pod basic-resize-test -n k8s-compliance-test --subresource resize --patch \
  '{"spec":{"containers":[{"name":"test-container", "resources":{"requests":{"cpu":"1000"}, "limits":{"cpu":"2000"}}}]}}'

# Check for PodResizePending condition
kubectl get pod basic-resize-test -n k8s-compliance-test -o yaml | grep -A 5 "conditions:"

# Expected output should include:
# - type: PodResizePending
#   status: "True"
#   reason: Infeasible
#   message: "Node didn't have enough capacity"
```

## 🐛 Troubleshooting

### Common Issues

#### 1. "unknown subresource 'resize'" Error
```bash
# Check kubectl version
kubectl version --client --short
# Upgrade to kubectl 1.32+ if needed

# Check K8s server version
kubectl version --short
# Ensure server is 1.33+
```

#### 2. Feature Gate Not Enabled
```bash
# Check if InPlacePodVerticalScaling is enabled
kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{": kubelet="}{.status.nodeInfo.kubeletVersion}{"\n"}{end}'

# For kind clusters, recreate with feature gate:
kind create cluster --config - <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: KubeletConfiguration
    featureGates:
      InPlacePodVerticalScaling: true
EOF
```

#### 3. Tests Fail Due to Resource Constraints
```bash
# Check node resources
kubectl describe nodes

# Check resource quotas
kubectl describe quota -A

# Use smaller test resource values
kubectl patch pod basic-resize-test --subresource resize --patch \
  '{"spec":{"containers":[{"name":"test-container", "resources":{"requests":{"cpu":"50m"}, "limits":{"cpu":"100m"}}}]}}'
```

#### 4. Right-Sizer Not Processing Pods
```bash
# Check right-sizer logs
kubectl logs -l app.kubernetes.io/name=right-sizer -n right-sizer-system

# Verify right-sizer configuration
kubectl get rightsizerconfig default -o yaml

# Check if namespace is included
kubectl get rightsizerconfig default -o jsonpath='{.spec.namespaceConfig}'
```

## 🚀 CI/CD Integration

### GitHub Actions

The framework includes GitHub Actions workflow (`.github/workflows/k8s-compliance-test.yml`) that:
- Tests multiple K8s versions (1.32, 1.33)
- Runs on PR and push events
- Generates compliance reports
- Posts results as PR comments

### Integration with Existing CI

```bash
# Add to your CI pipeline
- name: K8s Compliance Check
  run: |
    cd right-sizer/examples/k8s-compliance-testing/
    ./run-compliance-tests.sh
    
- name: Upload Compliance Report
  uses: actions/upload-artifact@v3
  with:
    name: k8s-compliance-report
    path: compliance-test-report-*.json
```

## 📚 Additional Resources

- [Kubernetes 1.33+ In-Place Resize Documentation](https://kubernetes.io/docs/tasks/configure-pod-container/resize-container-resources/)
- [Right-Sizer K8s Compliance Report](../../K8S_INPLACE_RESIZE_COMPLIANCE_REPORT.md)
- [Right-Sizer Installation Guide](../../INSTALLATION_GUIDE.md)
- [Right-Sizer Configuration Reference](../../helm/values.yaml)

## 🤝 Contributing

To add new test scenarios:

1. **Add test pod definition** to `test-pods.yaml`
2. **Add test function** to `run-compliance-tests.sh`
3. **Add Go test** to integration test files
4. **Update documentation** in this README

### Example: Adding a New Test

```bash
# 1. Add pod to test-pods.yaml
---
apiVersion: v1
kind: Pod
metadata:
  name: my-new-test
  namespace: k8s-compliance-test
  labels:
    test-case: "my-test-case"
spec:
  # ... pod specification

# 2. Add test function to run-compliance-tests.sh
test_my_new_feature() {
  print_test "My New Feature Test"
  
  local pod_name="my-new-test"
  
  # Test implementation
  if kubectl wait --for=condition=Ready pod/$pod_name -n $TEST_NAMESPACE --timeout=60s; then
    print_pass "My new feature works correctly" "my-new-feature"
  else
    print_fail "My new feature failed" "my-new-feature"
  fi
}

# 3. Add to main() function
main() {
  # ... existing tests
  test_my_new_feature
  # ...
}
```

## 📄 License

This testing framework is part of the right-sizer project and is licensed under the GNU Affero General Public License v3.0. See [LICENSE](../../LICENSE) for details.

---

**Happy Testing! 🎉**

For questions or issues, please open an issue in the [right-sizer repository](https://github.com/aavishay/right-sizer).
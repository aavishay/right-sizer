# Right Sizer RBAC Scripts

This directory contains scripts for managing and troubleshooting RBAC (Role-Based Access Control) permissions for the Right Sizer operator.

## Scripts Overview

### 1. verify-permissions.sh

**Purpose:** Quickly verify that the Right Sizer operator has all required RBAC permissions.

**Usage:**
```bash
# Basic verification with defaults
./verify-permissions.sh

# Verify specific namespace and service account
./verify-permissions.sh -n my-namespace -s my-service-account

# Verbose output for debugging
./verify-permissions.sh --verbose

# Skip optional checks
./verify-permissions.sh --no-metrics --no-custom-resources
```

**Options:**
- `-n, --namespace NAME` - Namespace where Right Sizer is installed (default: right-sizer-system)
- `-s, --service-account NAME` - Service account name (default: right-sizer)
- `-v, --verbose` - Show detailed permission checks
- `--no-metrics` - Skip metrics API checks
- `--no-custom-resources` - Skip custom resource checks
- `-h, --help` - Show help message

**Exit Codes:**
- `0` - All permissions verified successfully
- `1` - Some permissions are missing or warnings exist
- `2` - Critical permissions are missing

### 2. apply-rbac-fix.sh

**Purpose:** Apply or fix RBAC permissions for the Right Sizer operator.

**Usage:**
```bash
# Apply RBAC with defaults
./apply-rbac-fix.sh

# Force reapply (removes existing RBAC first)
./apply-rbac-fix.sh --force

# Dry run to see what would be applied
./apply-rbac-fix.sh --dry-run

# Apply using Helm
./apply-rbac-fix.sh --use-helm

# Custom namespace and service account
./apply-rbac-fix.sh -n my-namespace -s my-service-account
```

**Options:**
- `-n, --namespace NAME` - Namespace to install Right Sizer (default: right-sizer-system)
- `-s, --service-account NAME` - Service account name (default: right-sizer)
- `-r, --release-name NAME` - Helm release name (default: right-sizer)
- `--dry-run` - Show what would be done without making changes
- `-f, --force` - Force cleanup and reapply RBAC resources
- `--no-verify` - Skip verification after applying RBAC
- `--use-helm` - Use Helm to apply the entire chart
- `-h, --help` - Show help message

**Exit Codes:**
- `0` - RBAC applied successfully
- `1` - Failed to apply RBAC

### 3. test-rbac-suite.sh

**Purpose:** Comprehensive test suite for all RBAC permissions with detailed reporting.

**Note:** This script has been moved to `tests/rbac/test-rbac-suite.sh`

**Usage:**
```bash
# Run complete test suite
../../tests/rbac/test-rbac-suite.sh

# Verbose output with details
../../tests/rbac/test-rbac-suite.sh --verbose

# Output as JSON for automation
../../tests/rbac/test-rbac-suite.sh --output json > results.json

# Test with custom namespace
../../tests/rbac/test-rbac-suite.sh -n my-namespace -t test-namespace

# Keep test resources for inspection
../../tests/rbac/test-rbac-suite.sh --skip-cleanup
```

**Options:**
- `-n, --namespace NAME` - Namespace where Right Sizer is installed
- `-s, --service-account NAME` - Service account name
- `-t, --test-namespace NAME` - Namespace for test resources
- `-v, --verbose` - Show detailed test output
- `--skip-cleanup` - Don't clean up test resources
- `-o, --output FORMAT` - Output format (terminal, json, junit)
- `-h, --help` - Show help message

**Exit Codes:**
- `0` - All tests passed
- `1` - Some non-critical tests failed
- `2` - Critical tests failed

## Common Workflows

### Initial Setup

```bash
# 1. Apply RBAC configuration
./apply-rbac-fix.sh

# 2. Verify permissions
./verify-permissions.sh

# 3. Run comprehensive tests
../../tests/rbac/test-rbac-suite.sh
```

### Troubleshooting Permission Issues

```bash
# 1. Quick diagnosis
./verify-permissions.sh --verbose

# 2. If issues found, force reapply
./apply-rbac-fix.sh --force

# 3. Run comprehensive test suite
../../tests/rbac/test-rbac-suite.sh --verbose
```

### Upgrading Right Sizer

```bash
# 1. Check current permissions
./verify-permissions.sh > before-upgrade.txt

# 2. Perform upgrade (using Helm)
./apply-rbac-fix.sh --use-helm

# 3. Verify new permissions
./verify-permissions.sh > after-upgrade.txt

# 4. Compare results
diff before-upgrade.txt after-upgrade.txt
```

### CI/CD Integration

```bash
#!/bin/bash
# Example CI/CD pipeline script

# Apply RBAC in dry-run mode first
if ! ./apply-rbac-fix.sh --dry-run; then
  echo "RBAC dry-run failed"
  exit 1
fi

# Apply RBAC
./apply-rbac-fix.sh --no-verify

# Run verification
if ! ./verify-permissions.sh; then
  echo "RBAC verification failed"
  exit 1
fi

# Run comprehensive tests with JSON output
../../tests/rbac/test-rbac-suite.sh --output json > rbac-test-results.json

# Check test results
if [ $? -ne 0 ]; then
  echo "RBAC tests failed"
  cat rbac-test-results.json | jq '.failed'
  exit 1
fi

echo "RBAC configuration successful"
```

## Environment Variables

All scripts support environment variables as alternatives to command-line flags:

```bash
# Set namespace
export NAMESPACE=my-namespace

# Set service account
export SERVICE_ACCOUNT=my-service-account

# Enable verbose output
export VERBOSE=true

# Run scripts with environment variables
./verify-permissions.sh
./apply-rbac-fix.sh
```

## Required Permissions Overview

The Right Sizer operator requires the following categories of permissions:

1. **Core Resources**
   - Pods (get, list, watch, patch, update)
   - Nodes (get, list, watch)
   - Events (create, patch, update)
   - Namespaces (get, list, watch)

2. **Metrics APIs**
   - metrics.k8s.io resources
   - Custom and external metrics (optional)

3. **Workload Controllers**
   - Deployments, StatefulSets, DaemonSets, ReplicaSets
   - Scale subresources

4. **Resource Constraints**
   - ResourceQuotas, LimitRanges
   - PodDisruptionBudgets

5. **Storage**
   - PersistentVolumeClaims, PersistentVolumes
   - StorageClasses

6. **Networking**
   - Services, Endpoints
   - NetworkPolicies

7. **Admission Webhooks**
   - ValidatingWebhookConfigurations
   - MutatingWebhookConfigurations

8. **Custom Resources**
   - rightsizer.io resources (if CRDs installed)

## Troubleshooting

### Common Issues

#### Service Account Not Found
```bash
# Check if namespace exists
kubectl get namespace right-sizer-system

# Create namespace if missing
kubectl create namespace right-sizer-system

# Force reapply RBAC
./apply-rbac-fix.sh --force
```

#### Metrics API Not Available
```bash
# Check if metrics-server is installed
kubectl get deployment metrics-server -n kube-system

# Install metrics-server if missing
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
```

#### Permission Denied Errors
```bash
# Run verbose verification
./verify-permissions.sh --verbose > rbac-diagnosis.txt

# Check specific permission
kubectl auth can-i patch pods --as=system:serviceaccount:right-sizer-system:right-sizer

# Force fix RBAC
./apply-rbac-fix.sh --force
```

### Debug Mode

Enable debug mode for detailed output:

```bash
# Enable debug logging
export DEBUG=true
export VERBOSE=true

# Run scripts with debug output
./verify-permissions.sh 2>&1 | tee debug.log
```

### Getting Help

1. Check the main RBAC documentation: [../../docs/RBAC.md](../../docs/RBAC.md)
2. Review troubleshooting guide: [../../docs/RBAC-TROUBLESHOOTING.md](../../docs/RBAC-TROUBLESHOOTING.md)
3. Run script help: `./script-name.sh --help`
4. Check script exit codes for automated handling

## Best Practices

1. **Regular Verification**
   - Run `verify-permissions.sh` after any cluster changes
   - Include in CI/CD pipelines
   - Set up monitoring alerts for permission failures

2. **Version Control**
   - Track RBAC changes in git
   - Document custom modifications
   - Use consistent naming conventions

3. **Security**
   - Follow principle of least privilege
   - Regularly audit permissions
   - Remove temporary elevated permissions

4. **Automation**
   - Use JSON output for automated processing
   - Integrate with monitoring systems
   - Create scheduled verification jobs

## Contributing

When modifying these scripts:

1. Test changes in a development cluster first
2. Update this README with new features
3. Maintain backward compatibility
4. Add error handling for new checks
5. Update help text in scripts

## License

These scripts are part of the Right Sizer project and are licensed under the GNU Affero General Public License v3.0 (AGPL-3.0).
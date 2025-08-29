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
```
nodes is forbidden: User "system:serviceaccount:right-sizer-system:right-sizer" cannot list resource "nodes" in API group "" at the cluster scope
```

#### Cause
The service account lacks necessary permissions to access Kubernetes resources.

#### Solution
Apply the enhanced RBAC configuration:
```bash
# Update RBAC permissions using Helm
helm upgrade right-sizer ./helm --reuse-values --namespace right-sizer-system
```

This grants permissions for:
- Node operations (get, list, watch) for capacity validation
- Pod operations including the resize subresource
- Metrics access for resource usage monitoring
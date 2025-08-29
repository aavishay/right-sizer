# Right-Sizer Troubleshooting Guide

## Overview

This guide provides comprehensive troubleshooting steps for the enhanced right-sizer operator, covering common issues, diagnostic procedures, and resolution strategies for all components including policy engine, admission controller, audit logging, and observability features.

## Quick Diagnostic Checklist

Before diving into specific issues, run through this quick checklist:

1. **Basic Health Check**
   ```bash
   kubectl get pods -l app=right-sizer -o wide
   kubectl logs -l app=right-sizer --tail=50
   curl -f http://localhost:8081/healthz  # via port-forward
   ```

2. **Configuration Validation**
   ```bash
   kubectl describe configmap right-sizer-config
   kubectl get secret right-sizer-admission-certs -o yaml
   ```

3. **Metrics Availability**
   ```bash
   kubectl port-forward svc/right-sizer-operator 9090:9090
   curl http://localhost:9090/metrics | grep rightsizer_
   ```

4. **Recent Events**
   ```bash
   kubectl get events --sort-by='.lastTimestamp' | head -20
   kubectl get events --field-selector reason=ResourceChange
   ```

## Common Issues and Solutions

### 1. Operator Pod Not Starting

#### Symptoms
- Pod stuck in `Pending`, `CrashLoopBackOff`, or `Error` state
- No logs visible or startup errors in logs

#### Diagnostic Steps
```bash
kubectl describe pod -l app=right-sizer
kubectl get events --field-selector involvedObject.kind=Pod
kubectl logs -l app=right-sizer --previous  # Check previous container logs
```

#### Common Causes and Solutions

**Image Pull Errors**
```yaml
# Check image name and tag in deployment
kubectl get deployment right-sizer-operator -o yaml | grep image:
# Verify image exists and is accessible
docker pull right-sizer:latest
```

**Resource Constraints**
```bash
# Check node resources
kubectl describe node <node-name>
# Adjust resource requests/limits
kubectl patch deployment right-sizer-operator -p '{"spec":{"template":{"spec":{"containers":[{"name":"operator","resources":{"requests":{"memory":"128Mi","cpu":"100m"}}}]}}}}'
```

**RBAC Issues**
```bash
# Verify service account and permissions
kubectl get serviceaccount right-sizer-operator
kubectl describe clusterrolebinding right-sizer-operator
# Check if service account can access required resources
kubectl auth can-i get pods --as=system:serviceaccount:right-sizer-system:right-sizer-operator
```

**Configuration Errors**
```bash
# Check for invalid configuration values
kubectl logs -l app=right-sizer | grep -i "configuration validation failed"
# Fix invalid environment variables
kubectl set env deployment/right-sizer-operator CPU_REQUEST_MULTIPLIER=1.2
```

### 2. Pods Not Being Resized

#### Symptoms
- Operator is running but pods aren't getting resized
- No resource change events visible
- Pods continue using original resource specifications

#### Diagnostic Steps
```bash
# Check if pods are being processed
kubectl logs -l app=right-sizer | grep "Processing pods"
# Look for skip reasons
kubectl logs -l app=right-sizer | grep -i "skip\|ignore"
# Check metrics for processed pods
curl http://localhost:9090/metrics | grep rightsizer_pods_processed_total
```

#### Common Causes and Solutions

**Namespace Filtering**
```bash
# Check namespace include/exclude settings
kubectl get deployment right-sizer-operator -o yaml | grep -E "NAMESPACE_(INCLUDE|EXCLUDE)"
# Verify target pods are in monitored namespaces
kubectl get pods -A | grep <your-pod-name>
```

**Pod Annotations/Labels**
```bash
# Check for skip annotations
kubectl get pod <pod-name> -o yaml | grep -A 5 -B 5 "rightsizer"
# Remove skip annotations if needed
kubectl annotate pod <pod-name> rightsizer.io/disable-
```

**Metrics Collection Issues**
```bash
# Verify metrics-server is running
kubectl get pods -n kube-system | grep metrics-server
# Check if pod has metrics available
kubectl top pod <pod-name>
# Test Prometheus connection (if using Prometheus provider)
kubectl exec -it deployment/right-sizer-operator -- wget -qO- http://prometheus:9090/api/v1/query?query=up
```

**Policy Rules Blocking**
```bash
# Check policy evaluation logs
kubectl logs -l app=right-sizer | grep -i policy
# Review policy ConfigMap
kubectl get configmap right-sizer-policies -o yaml
# Test with policy engine disabled
kubectl set env deployment/right-sizer-operator POLICY_BASED_SIZING=false
```

**Safety Threshold Violations**
```bash
# Check for safety threshold violations
curl http://localhost:9090/metrics | grep rightsizer_safety_threshold_violations
# Adjust safety threshold
kubectl set env deployment/right-sizer-operator SAFETY_THRESHOLD=0.6
```

### 3. Admission Controller Issues

#### Symptoms
- Pod creation/updates failing with admission webhook errors
- Webhook timeout errors
- Certificate validation failures

#### Diagnostic Steps
```bash
# Check webhook configuration
kubectl describe validatingadmissionwebhook right-sizer-resource-validator
# Test webhook service accessibility
kubectl get svc right-sizer-admission-webhook
kubectl get endpoints right-sizer-admission-webhook
# Check webhook pod logs
kubectl logs -l app=right-sizer | grep -i webhook
```

#### Common Causes and Solutions

**Certificate Issues**
```bash
# Check certificate validity
kubectl get secret right-sizer-admission-certs -o yaml | grep tls.crt | base64 -d | openssl x509 -text -noout
# Verify certificate expiration
kubectl get secret right-sizer-admission-certs -o yaml | grep tls.crt | base64 -d | openssl x509 -enddate -noout
# Regenerate certificates if expired
# (Use cert-manager or manual certificate generation)
```

**Service Connectivity**
```bash
# Test webhook endpoint from cluster
kubectl run test-pod --rm -it --image=busybox -- wget -qO- https://right-sizer-admission-webhook.right-sizer-system.svc:443/validate
# Check DNS resolution
kubectl run test-pod --rm -it --image=busybox -- nslookup right-sizer-admission-webhook.right-sizer-system.svc
```

**Webhook Configuration**
```bash
# Verify webhook is pointing to correct service
kubectl get validatingadmissionwebhook right-sizer-resource-validator -o yaml | grep -A 10 clientConfig
# Check webhook failure policy
kubectl patch validatingadmissionwebhook right-sizer-resource-validator -p '{"webhooks":[{"name":"validate.rightsizer.io","failurePolicy":"Ignore"}]}'
```

**Port and Firewall Issues**
```bash
# Verify webhook port configuration
kubectl get deployment right-sizer-operator -o yaml | grep -A 5 -B 5 "containerPort.*8443"
# Test port forwarding
kubectl port-forward deployment/right-sizer-operator 8443:8443
curl -k https://localhost:8443/validate
```

### 4. Policy Engine Problems

#### Symptoms
- Policies not being applied
- Unexpected resource calculations
- Policy evaluation errors

#### Diagnostic Steps
```bash
# Check policy engine status
kubectl logs -l app=right-sizer | grep -i "policy.*initialized"
# Review policy evaluation
kubectl logs -l app=right-sizer | grep -i "policy.*applied\|rule.*matched"
# Check policy ConfigMap
kubectl get configmap right-sizer-policies -o yaml
```

#### Common Causes and Solutions

**Policy ConfigMap Issues**
```bash
# Verify ConfigMap exists and is mounted
kubectl get configmap right-sizer-policies
kubectl describe pod -l app=right-sizer | grep -A 5 -B 5 configmap
# Check YAML syntax
kubectl get configmap right-sizer-policies -o yaml | grep -A 50 rules.yaml
```

**Policy Rule Syntax Errors**
```bash
# Look for policy validation errors
kubectl logs -l app=right-sizer | grep -i "policy.*invalid\|rule.*error"
# Test individual policy rules
# Create a simple test policy and deploy incrementally
```

**Selector Matching Issues**
```bash
# Debug selector matching
kubectl logs -l app=right-sizer | grep -i "selector.*match"
# Check pod labels and annotations
kubectl get pod <pod-name> -o yaml | grep -A 10 -B 10 labels
# Test regex patterns
echo "test-pod-name" | grep -E "^test-.*"
```

**Priority and Rule Conflicts**
```bash
# Check rule evaluation order
kubectl logs -l app=right-sizer -f | grep -i "priority\|rule.*applied"
# Review policy priorities
kubectl get configmap right-sizer-policies -o yaml | grep -E "name:|priority:"
```

### 5. Metrics and Observability Issues

#### Symptoms
- Missing metrics in Prometheus
- Metrics endpoint not accessible
- Incomplete or incorrect metric values

#### Diagnostic Steps
```bash
# Check metrics endpoint
kubectl port-forward svc/right-sizer-operator 9090:9090
curl http://localhost:9090/metrics | head -50
# Verify service monitor (if using Prometheus Operator)
kubectl get servicemonitor right-sizer-operator -o yaml
# Check metrics in Prometheus
curl "http://localhost:9090/api/v1/query?query=rightsizer_pods_processed_total"
```

#### Common Causes and Solutions

**Metrics Server Configuration**
```bash
# Verify metrics are enabled
kubectl get deployment right-sizer-operator -o yaml | grep METRICS_ENABLED
# Check metrics port configuration
kubectl get svc right-sizer-operator -o yaml | grep -A 5 -B 5 9090
```

**Prometheus Scraping Issues**
```bash
# Check service monitor labels
kubectl get servicemonitor right-sizer-operator -o yaml | grep -A 5 -B 5 labels
# Verify Prometheus can reach the service
kubectl get endpoints right-sizer-operator
# Check Prometheus configuration
kubectl logs -n monitoring deployment/prometheus | grep right-sizer
```

**Metrics Collection Problems**
```bash
# Check for metrics collection errors
kubectl logs -l app=right-sizer | grep -i "metrics.*error\|collection.*failed"
# Test individual metric endpoints
curl http://localhost:9090/metrics | grep rightsizer_pods_processed_total
```

### 6. Audit Logging Issues

#### Symptoms
- Missing audit logs
- Audit log rotation not working
- Events not being created in Kubernetes

#### Diagnostic Steps
```bash
# Check audit logging configuration
kubectl get deployment right-sizer-operator -o yaml | grep AUDIT_ENABLED
# Verify log files (if using file logging)
kubectl exec deployment/right-sizer-operator -- ls -la /var/log/right-sizer/
# Check Kubernetes events
kubectl get events --field-selector source=right-sizer
```

#### Common Causes and Solutions

**File System Issues**
```bash
# Check disk space and permissions
kubectl exec deployment/right-sizer-operator -- df -h /var/log/right-sizer/
kubectl exec deployment/right-sizer-operator -- ls -la /var/log/right-sizer/
# Fix permissions if needed
kubectl exec deployment/right-sizer-operator -- chmod 755 /var/log/right-sizer/
```

**Volume Mount Problems**
```bash
# Verify PVC and volume mounts
kubectl get pvc right-sizer-audit-logs
kubectl describe pod -l app=right-sizer | grep -A 10 -B 5 "audit-logs"
```

**Event Creation Permissions**
```bash
# Check RBAC for event creation
kubectl auth can-i create events --as=system:serviceaccount:right-sizer-system:right-sizer-operator
# Review cluster role permissions
kubectl get clusterrole right-sizer-operator -o yaml | grep -A 5 -B 5 events
```

### 7. Circuit Breaker and Retry Issues

#### Symptoms
- Operations failing repeatedly
- Circuit breaker stuck in OPEN state
- Excessive retry attempts

#### Diagnostic Steps
```bash
# Check circuit breaker metrics
curl http://localhost:9090/metrics | grep -E "rightsizer.*(retry|circuit)"
# Review retry attempt logs
kubectl logs -l app=right-sizer | grep -i "retry\|circuit"
# Check failure patterns
kubectl logs -l app=right-sizer | grep -i "error\|failed" | tail -20
```

#### Common Causes and Solutions

**Circuit Breaker Configuration**
```bash
# Check current circuit breaker state
curl http://localhost:9090/metrics | grep rightsizer_circuit_breaker_state
# Adjust circuit breaker thresholds
kubectl set env deployment/right-sizer-operator MAX_RETRIES=5
kubectl set env deployment/right-sizer-operator RETRY_INTERVAL=3s
```

**Underlying Service Issues**
```bash
# Check Kubernetes API server health
kubectl get --raw="/healthz"
# Verify metrics provider connectivity
kubectl exec deployment/right-sizer-operator -- wget -qO- http://prometheus:9090/api/v1/label/__name__/values
```

### 8. Performance Issues

#### Symptoms
- High CPU/memory usage
- Slow pod processing
- Timeouts during operations

#### Diagnostic Steps
```bash
# Check resource usage
kubectl top pod -l app=right-sizer
# Review processing duration metrics
curl http://localhost:9090/metrics | grep rightsizer_processing_duration
# Check for resource constraints
kubectl describe pod -l app=right-sizer | grep -A 5 -B 5 "resource\|limit"
```

#### Common Causes and Solutions

**Resource Constraints**
```bash
# Increase operator resources
kubectl patch deployment right-sizer-operator -p '{"spec":{"template":{"spec":{"containers":[{"name":"operator","resources":{"requests":{"memory":"512Mi","cpu":"200m"},"limits":{"memory":"1Gi","cpu":"500m"}}}]}}}}'
```

**Configuration Tuning**
```bash
# Increase processing interval for less frequent checks
kubectl set env deployment/right-sizer-operator RESIZE_INTERVAL=5m
# Reduce historical data retention
kubectl set env deployment/right-sizer-operator HISTORY_DAYS=7
```

## Advanced Debugging

### Debug Mode Configuration

Enable comprehensive debugging:

```bash
# Set debug logging
kubectl set env deployment/right-sizer-operator LOG_LEVEL=debug

# Enable dry run for safe testing
kubectl set env deployment/right-sizer-operator DRY_RUN=true

# Increase logging verbosity for specific components
kubectl logs -l app=right-sizer -f | grep -E "(policy|validation|metric|audit)"
```

### Network Troubleshooting

Test network connectivity between components:

```bash
# Test from operator pod to metrics server
kubectl exec deployment/right-sizer-operator -- wget -qO- http://metrics-server.kube-system:443/apis/metrics.k8s.io/v1beta1/nodes

# Test admission webhook connectivity
kubectl exec deployment/right-sizer-operator -- wget -qO- https://right-sizer-admission-webhook:443/health

# Test Prometheus connectivity
kubectl exec deployment/right-sizer-operator -- wget -qO- http://prometheus:9090/api/v1/label/__name__/values
```

### Configuration Validation

Validate configuration files before applying:

```bash
# Test YAML syntax
yamllint examples/policy-rules-example.yaml

# Validate Kubernetes resources
kubectl apply --dry-run=client -f examples/advanced-configuration.yaml

# Check Helm template rendering
helm template right-sizer ./helm --debug
```

### Performance Profiling

Enable performance profiling for detailed analysis:

```bash
# Enable Go profiling (requires code modification)
# kubectl port-forward deployment/right-sizer-operator 6060:6060
# go tool pprof http://localhost:6060/debug/pprof/profile

# Monitor resource usage over time
kubectl top pod -l app=right-sizer --containers

# Check garbage collection metrics
curl http://localhost:9090/metrics | grep go_gc_
```

## Emergency Procedures

### Complete Operator Reset

If the operator is completely broken:

```bash
# 1. Stop the operator
kubectl scale deployment right-sizer-operator --replicas=0

# 2. Clear any stuck webhooks
kubectl delete validatingadmissionwebhook right-sizer-resource-validator --ignore-not-found=true

# 3. Reset configuration to defaults
kubectl delete configmap right-sizer-config
kubectl apply -f examples/default-configuration.yaml

# 4. Restart with minimal configuration
kubectl set env deployment/right-sizer-operator DRY_RUN=true POLICY_BASED_SIZING=false ADMISSION_CONTROLLER=false
kubectl scale deployment right-sizer-operator --replicas=1
```

### Admission Controller Bypass

If admission controller is blocking critical operations:

```bash
# Temporarily set failure policy to Ignore
kubectl patch validatingadmissionwebhook right-sizer-resource-validator -p '{"webhooks":[{"name":"validate.rightsizer.io","failurePolicy":"Ignore"}]}'

# Or completely disable admission controller
kubectl delete validatingadmissionwebhook right-sizer-resource-validator

# Disable in operator
kubectl set env deployment/right-sizer-operator ADMISSION_CONTROLLER=false
```

### Data Recovery

Recover from data corruption or loss:

```bash
# Restore audit logs from backup
kubectl cp backup-audit.log right-sizer-operator-pod:/var/log/right-sizer/audit.log

# Rebuild metrics cache
kubectl delete pod -l app=right-sizer  # Force restart to rebuild cache

# Reset policy engine
kubectl delete configmap right-sizer-policies
kubectl apply -f examples/policy-rules-example.yaml
```

## Prevention and Monitoring

### Proactive Monitoring

Set up monitoring for early issue detection:

```bash
# Key metrics to monitor
rightsizer_pods_processed_total
rightsizer_processing_duration_seconds
rightsizer_safety_threshold_violations_total
rightsizer_circuit_breaker_state

# Recommended alerts
# - Processing duration > 30s
# - Circuit breaker OPEN for > 5 minutes
# - Safety threshold violations > 10/hour
# - Pod processing errors > 5% of total
```

### Health Checks

Configure comprehensive health monitoring:

```bash
# Kubernetes readiness/liveness probes (already configured)
curl http://localhost:8081/healthz
curl http://localhost:8081/readyz

# Custom health checks
curl http://localhost:9090/metrics | grep -c rightsizer_
kubectl get events --field-selector source=right-sizer --since=1h
```

### Log Management

Set up proper log management:

```bash
# Configure log rotation
kubectl patch deployment right-sizer-operator -p '{"spec":{"template":{"spec":{"containers":[{"name":"operator","env":[{"name":"LOG_ROTATION_SIZE","value":"100MB"},{"name":"LOG_RETENTION_DAYS","value":"30"}]}]}}}}'

# Set up log aggregation (Fluentd/ELK)
# Add log shipping sidecar or configure node-level log collection
```

## Getting Help

### Information to Collect

When reporting issues, collect this information:

```bash
# Operator status and configuration
kubectl get deployment right-sizer-operator -o yaml > operator-deployment.yaml
kubectl get configmap right-sizer-config -o yaml > operator-config.yaml
kubectl get secret right-sizer-admission-certs -o yaml > operator-certs.yaml

# Logs and events
kubectl logs -l app=right-sizer --since=1h > operator-logs.txt
kubectl get events --sort-by='.lastTimestamp' --since=1h > cluster-events.txt

# Metrics and health status
curl http://localhost:9090/metrics > operator-metrics.txt
curl http://localhost:8081/healthz > operator-health.txt

# Cluster information
kubectl version --client --output=yaml > kubectl-version.yaml
kubectl get nodes -o wide > cluster-nodes.txt
```

### Support Channels

- **GitHub Issues**: Report bugs and feature requests
- **Community Forum**: Ask questions and share solutions
- **Documentation**: Check latest documentation for updates
- **Helm Chart**: Review Helm chart issues separately

Remember to sanitize any sensitive information before sharing logs or configurations.
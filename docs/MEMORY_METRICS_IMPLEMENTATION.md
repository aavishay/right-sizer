# Memory Metrics Implementation

## Overview

This document describes the implementation of custom Prometheus metrics export and memory pressure specific logging for the Right-Sizer operator. These features enhance observability and provide detailed insights into memory usage patterns and pressure conditions.

## Implementation Summary

### 1. Custom Prometheus Metrics Export ✅

#### Features Implemented

##### Core Memory Metrics
- `rightsizer_pod_memory_usage_bytes` - Current memory usage in bytes
- `rightsizer_pod_memory_working_set_bytes` - Working set memory (active pages)
- `rightsizer_pod_memory_rss_bytes` - Resident Set Size memory
- `rightsizer_pod_memory_cache_bytes` - Cache memory usage
- `rightsizer_pod_memory_swap_bytes` - Swap memory usage

##### Memory Limits and Requests
- `rightsizer_pod_memory_limit_bytes` - Configured memory limits
- `rightsizer_pod_memory_request_bytes` - Configured memory requests

##### Memory Utilization Metrics
- `rightsizer_memory_utilization_percentage` - Memory usage as percentage of limit
- `rightsizer_memory_request_utilization` - Usage relative to requests
- `rightsizer_memory_limit_utilization` - Usage relative to limits

##### Memory Pressure Metrics
- `rightsizer_memory_pressure_events_total` - Counter of pressure events
- `rightsizer_memory_pressure_level` - Current pressure level (0-4 scale)
- `rightsizer_memory_oom_kill_events_total` - OOM kill event counter
- `rightsizer_memory_throttling_events_total` - Throttling event counter

##### Memory Trend Metrics
- `rightsizer_memory_trend_slope` - Memory usage trend indicator
- `rightsizer_memory_peak_usage_bytes` - Peak memory observed
- `rightsizer_memory_average_usage_bytes` - Average memory over time window

##### Memory Efficiency Metrics
- `rightsizer_memory_waste_bytes` - Allocated but unused memory
- `rightsizer_memory_efficiency_score` - Efficiency score (0-100)

##### Memory Recommendation Metrics
- `rightsizer_memory_recommendation_bytes` - Recommended memory allocation
- `rightsizer_memory_recommendation_ratio` - Ratio of recommended to current

#### Prometheus Server Configuration

The implementation includes a dedicated Prometheus metrics server on port 9090 with the following endpoints:

- `/metrics` - Main Prometheus metrics endpoint with all custom metrics
- `/metrics/memory` - Memory-specific metrics summary
- `/metrics/health` - Metrics server health check

### 2. Memory Pressure Specific Logging ✅

#### Pressure Level Detection

The system detects five levels of memory pressure:

| Level | Threshold | Action |
|-------|-----------|--------|
| NONE | < 70% | Normal operation |
| LOW | 70-80% | Informational logging |
| MEDIUM | 80-90% | Warning logs |
| HIGH | 90-95% | Alert with recommendations |
| CRITICAL | > 95% | Critical alerts, immediate action |

#### Log Types Implemented

##### Memory Pressure Logs
```
[MEMORY_PRESSURE] HIGH detected for namespace/pod/container - Usage: 230.5Mi/256.0Mi (90.0% utilization)
[MEMORY_CRITICAL] Pod namespace/pod at risk of OOM kill - Immediate action required
[MEMORY_HIGH] Pod namespace/pod approaching memory limit - Consider increasing allocation
```

##### Memory Trend Logs
```
[MEMORY_TREND] INCREASING trend for namespace/pod/container - Slope: 2.5, Peak: 245Mi, Avg: 200Mi
[MEMORY_LEAK] Potential memory leak detected in namespace/pod/container - Rapid increase in memory usage
```

##### Memory Allocation Logs
```
[MEMORY_ALLOCATION] namespace/pod/container - Requested: 128Mi, Allocated: 256Mi, Used: 180Mi (70.3% efficiency)
```

##### Memory Recommendation Logs
```
[MEMORY_RECOMMENDATION] namespace/pod/container - Current: 128Mi, Recommended: 256Mi (100.0% change) - Reason: Based on p95 usage: 230Mi
```

##### Memory Resize Operation Logs
```
[MEMORY_RESIZE] Successfully up memory for namespace/pod/container
[MEMORY_RESIZE] Failed to down memory for namespace/pod/container: insufficient resources
```

##### Memory Pattern Analysis Logs
```
[MEMORY_PATTERN] namespace/pod/container - Stable memory usage pattern detected
[MEMORY_PATTERN] namespace/pod/container - High variance in memory usage - Consider buffer
[MEMORY_PATTERN] namespace/pod/container - Erratic memory usage pattern - Investigate application behavior
```

## Architecture

### Components

#### 1. MemoryMetrics Structure (`go/metrics/memory_metrics.go`)
Central structure holding all Prometheus metric collectors for memory-related metrics.

#### 2. PodMemoryController (`go/controllers/pod_controller_memory.go`)
Enhanced pod controller that:
- Collects memory metrics from metrics-server
- Analyzes memory patterns
- Detects pressure conditions
- Generates recommendations
- Applies in-place resizing

#### 3. Memory Monitoring Service
Background service that:
- Tracks memory usage history
- Calculates trends and patterns
- Detects potential memory leaks
- Triggers alerts on pressure conditions

#### 4. Prometheus Exporter
Dedicated server on port 9090 that:
- Exposes all memory metrics in Prometheus format
- Provides memory-specific endpoints
- Includes health checks

## Configuration

### Environment Variables

```yaml
ENABLE_MEMORY_METRICS: "true"           # Enable memory metrics collection
ENABLE_MEMORY_PRESSURE_LOGGING: "true"  # Enable pressure logging
MEMORY_PRESSURE_THRESHOLD: "0.8"        # Pressure detection threshold
MEMORY_HISTORY_SIZE: "100"              # Number of samples to keep
MEMORY_TREND_WINDOW: "5m"               # Time window for trend analysis
```

### Deployment Configuration

```yaml
ports:
- containerPort: 8080
  name: metrics         # Standard metrics
- containerPort: 8081
  name: health         # Health checks
- containerPort: 9090
  name: prometheus     # Prometheus metrics

annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "9090"
  prometheus.io/path: "/metrics"
```

## Usage

### Viewing Memory Metrics

1. **Via Prometheus Endpoint**
```bash
# Port-forward to the operator
kubectl port-forward -n right-sizer svc/right-sizer-metrics 9090:9090

# Query metrics
curl http://localhost:9090/metrics | grep memory
```

2. **Via kubectl logs**
```bash
# View memory pressure events
kubectl logs -n right-sizer deployment/right-sizer | grep MEMORY_PRESSURE

# View all memory-related logs
kubectl logs -n right-sizer deployment/right-sizer | grep MEMORY
```

### Prometheus Queries

Example queries for monitoring:

```promql
# Memory utilization percentage
rightsizer_memory_utilization_percentage{namespace="production"}

# Memory pressure events rate
rate(rightsizer_memory_pressure_events_total[5m])

# Memory efficiency score
rightsizer_memory_efficiency_score{pod=~"app-.*"}

# Memory waste in bytes
rightsizer_memory_waste_bytes{namespace="default"}

# Memory recommendation ratio
rightsizer_memory_recommendation_ratio{recommendation_type="request"}
```

### Grafana Dashboard

The metrics can be visualized in Grafana with panels for:
- Memory usage trends
- Pressure level heatmap
- Efficiency scores
- Recommendation tracking
- OOM event counters

## Integration Points

### With Existing Components

1. **Health Checker** - Reports memory subsystem health
2. **Audit Logger** - Records memory resize operations
3. **Resource Validator** - Validates memory recommendations
4. **Retry Manager** - Handles resize operation retries

### With External Systems

1. **Prometheus** - Scrapes metrics on port 9090
2. **Grafana** - Visualizes memory metrics
3. **AlertManager** - Triggers alerts on pressure conditions
4. **Metrics Server** - Source of memory usage data

## Testing

### Unit Tests
- Memory metrics calculation accuracy
- Pressure level detection logic
- Trend analysis algorithms
- Pattern recognition

### Integration Tests
- Metrics export verification
- Pressure event triggering
- Resize operation execution
- Log message formatting

### End-to-End Tests
Run the test scripts to verify:
```bash
# Quick memory test
./tests/quick-memory-test.sh

# Comprehensive test
./tests/memory-metrics-minikube-test.sh

# Implementation verification
./test-memory-implementation.sh
```

## Benefits

### Observability
- Complete visibility into memory usage patterns
- Early detection of memory issues
- Historical trend analysis
- Memory leak detection

### Proactive Management
- Prevent OOM kills through early detection
- Optimize memory allocation based on actual usage
- Reduce memory waste
- Improve resource efficiency

### Troubleshooting
- Detailed logs for memory events
- Pattern analysis for debugging
- Pressure level tracking
- Resize operation auditing

## Future Enhancements

### Planned Features
1. **Machine Learning** - Predictive memory usage modeling
2. **Anomaly Detection** - Automatic detection of unusual patterns
3. **Cost Optimization** - Memory cost tracking and optimization
4. **Multi-cluster** - Aggregated metrics across clusters
5. **Custom Alerts** - User-defined pressure thresholds
6. **Memory Profiles** - Application-specific memory patterns

### Integration Opportunities
1. **VPA Integration** - Coordinate with Vertical Pod Autoscaler
2. **HPA Coordination** - Memory-based horizontal scaling
3. **Cluster Autoscaler** - Node scaling based on memory pressure
4. **Cost Management** - Integration with FinOps tools

## Conclusion

The memory metrics implementation provides comprehensive observability and management capabilities for memory resources in Kubernetes. With custom Prometheus metrics and detailed logging, operators can proactively manage memory allocation, prevent OOM conditions, and optimize resource utilization.

The implementation follows Kubernetes best practices and integrates seamlessly with existing monitoring and observability tools, providing a production-ready solution for memory management in containerized environments.
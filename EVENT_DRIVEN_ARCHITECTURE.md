# Event-Driven Right-Sizer Architecture

## Overview

This implementation transforms the right-sizer operator into a lightweight, event-driven, API-first system designed to integrate with a centralized SaaS dashboard. The architecture emphasizes real-time streaming, stateless operation, and secure communication.

## Architecture Components

### 1. Event-Driven Foundation (`events/`)

**Event Types** (`events/types.go`):
- Comprehensive event taxonomy covering pod lifecycle, resource exhaustion, optimization opportunities
- Structured event format with correlation IDs, severity levels, and rich metadata
- Support for event filtering by type, namespace, severity, and tags

**Event Bus** (`events/bus.go`):
- Thread-safe event distribution system
- Subscription management with filtering capabilities
- Asynchronous event publishing with timeout protection
- Channel-based subscriptions for streaming APIs

**Streaming API** (`events/streaming.go`):
- WebSocket-based real-time event streaming
- Connection management with authentication support
- Event filtering and buffering
- Health monitoring and connection statistics

### 2. Remediation Engine (`remediation/`)

**Action Framework** (`remediation/engine.go`):
- Pluggable action handlers for different remediation types
- Risk assessment and approval workflows
- Safety checks and execution tracking
- Support for dry-run mode and audit logging

**Action Types**:
- Pod restarts for crash loops and OOM conditions
- Resource updates for optimization
- Scaling operations for capacity management
- Custom extensible action handlers

### 3. gRPC API (`api/grpc/`)

**Service Definition** (`api/grpc/v1/rightsizer.proto`):
- Cluster information and health status
- Metrics querying with filtering
- Real-time event streaming
- Remediation action execution and status tracking

**Server Implementation** (`api/grpc/server.go`):
- Full gRPC server with TLS support
- Authentication and authorization
- Metrics aggregation and reporting
- Action execution and status tracking

### 4. Event-Driven Controller (`controllers/`)

**Stateless Design** (`controllers/event_driven_controller.go`):
- React to Kubernetes events rather than polling
- Anomaly detection with configurable thresholds
- Automatic remediation triggering
- Event correlation and analysis

**Key Features**:
- OOM kill detection and response
- Crash loop analysis and remediation
- Resource utilization monitoring
- Optimization opportunity identification

## Integration Points

### Dashboard Communication

The operator provides multiple APIs for dashboard integration:

1. **WebSocket Streaming** (Port 8082):
   - Real-time event feed
   - Configurable filtering
   - Connection management

2. **gRPC API** (Port 9090):
   - Cluster information
   - Metrics querying
   - Action execution
   - Status monitoring

3. **REST API** (Metrics endpoint on 8080):
   - Prometheus metrics
   - Health checks

### Event Flow

```
Kubernetes Events -> Event-Driven Controller -> Event Bus -> {
  - Dashboard via WebSocket
  - Dashboard via gRPC streaming
  - Remediation Engine
  - Audit logging
}
```

### Security

- TLS encryption for all external communication
- Authentication tokens for API access
- RBAC integration with Kubernetes
- Audit trail for all actions

## Configuration

The system supports comprehensive configuration through:

- Environment variables for cluster identification
- ConfigMaps for operational parameters
- CRDs for advanced policies
- Command-line flags for deployment options

Key configuration areas:
- Cluster identification (ID, name, environment)
- Event filtering and routing
- Remediation policies and safety checks
- API endpoints and security settings

## Deployment

### Minimal Operator Mode

For SaaS architecture, the operator runs with minimal resources:

```yaml
resources:
  requests:
    memory: "64Mi"
    cpu: "50m"
  limits:
    memory: "128Mi"
    cpu: "100m"
```

### Multi-Cluster Support

Each cluster runs an independent operator instance that:
- Streams events to centralized dashboard
- Executes remediation actions locally
- Maintains cluster-specific policies
- Reports health and status

## Benefits

1. **Lightweight**: Minimal resource footprint per cluster
2. **Real-time**: Immediate event streaming and response
3. **Scalable**: Centralized intelligence, distributed execution
4. **Secure**: End-to-end encryption and authentication
5. **Observable**: Comprehensive event tracking and correlation
6. **Extensible**: Pluggable actions and custom integrations

## Usage Examples

### Starting the Event-Driven Operator

```bash
./main_event_driven \
  --config=/etc/rightsizer/config.yaml \
  --enable-grpc=true \
  --grpc-bind-address=:9090 \
  --metrics-bind-address=:8080
```

### Dashboard Integration

The dashboard can connect via:
- WebSocket: `ws://operator:8082/events`
- gRPC: `operator:9090`
- Metrics: `http://operator:8080/metrics`

### Event Filtering

WebSocket clients can filter events:
```json
{
  "eventTypes": ["pod.oom_killed", "resource.exhaustion"],
  "namespaces": ["production"],
  "severities": ["warning", "critical"]
}
```

## Future Enhancements

- AI-powered anomaly detection
- Predictive scaling based on trends
- Advanced correlation analysis
- Custom action webhooks
- Multi-cloud support

This architecture provides a solid foundation for a modern, cloud-native resource management platform with centralized intelligence and distributed execution.

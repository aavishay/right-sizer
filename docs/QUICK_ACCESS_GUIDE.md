# Quick Access Guide

## 🌐 Web Access
```bash
# Grafana Dashboard (Primary Interface)
open http://192.168.49.2:32000
# Credentials: admin / admin123
```

## 🔌 API Access
```bash
# Port forward Right-Sizer APIs
kubectl port-forward -n right-sizer svc/right-sizer 8082:8082 &

# Test main endpoints
curl http://localhost:8082/api/health           # Health status
curl http://localhost:8082/api/metrics/live     # Live metrics
curl http://localhost:8082/api/pods/count       # Pod count
curl http://localhost:8082/api/system/support   # System info
```

## 🏥 Health Monitoring
```bash
# Port forward health endpoints
kubectl port-forward -n right-sizer svc/right-sizer 8081:8081 &

# Check health
curl http://localhost:8081/healthz    # Liveness
curl http://localhost:8081/readyz     # Readiness
```

## 📊 Direct Metrics Access
```bash
# Port forward metrics endpoint
kubectl port-forward -n right-sizer svc/right-sizer 9090:9090 &

# View Prometheus metrics
curl http://localhost:9090/metrics | grep rightsizer
```

## ⚙️ Management Commands
```bash
# Check deployment status
kubectl get pods -n right-sizer
kubectl get pods -n monitoring
kubectl get pods -n demo

# View configurations
kubectl get rightsizerconfigs -A
kubectl describe rightsizerconfig -n right-sizer

# Check logs
kubectl logs -n right-sizer -l app.kubernetes.io/name=right-sizer -f

# View resource usage
kubectl top pods -n demo
```

## 🛑 Shutdown
```bash
# Stop port forwards
killall kubectl

# Remove demo workloads
kubectl delete namespace demo

# Remove monitoring
helm uninstall monitoring -n monitoring

# Remove right-sizer
helm uninstall right-sizer -n right-sizer

# Stop minikube
minikube stop
```

## 📝 Configuration Files
- `.env` - Environment variables
- `k8s/monitoring-simple.yaml` - Prometheus/Grafana configuration
- `k8s/demo-workload.yaml` - Sample applications
- `scripts/e2e-sanity-tests.sh` - Comprehensive test suite

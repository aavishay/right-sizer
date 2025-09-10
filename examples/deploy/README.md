# Right-Sizer Deployment & Workload Examples

This directory contains curated example manifests and helper scripts demonstrating different deployment modes, data collection patterns, and test scenarios for the Right-Sizer operator.

The manifests are intentionally verbose and scenario‑oriented rather than minimal. You can copy/adapt them into your own GitOps or Helm values flows.

---

## Directory Goals

1. Provide ready-to-run examples for:
   - Local / Minikube experimentation
   - Mock vs real metrics/event collection
   - Quick functional smoke tests
   - Stress / load simulation
2. Demonstrate best practices for:
   - Progressive rollout
   - Separation of mock vs production-like assets
   - Observability and verification flows

---

## Manifest Overview

| File | Purpose |
|------|---------|
| `right-sizer-deployment.yaml` | Baseline operator deployment (typical defaults) without special data modes. |
| `right-sizer-no-mock.yaml` | Operator deployment explicitly disabling mock data sources (forces real metrics / cluster behavior). |
| `right-sizer-real-data.yaml` | Example enabling “real” telemetry/event flows (pair with real events collectors below). |
| `api-proxy.yaml` | Local cluster (e.g. Minikube) API proxy for dashboard or lightweight in-cluster demos (mock / hybrid endpoints). |
| `api-proxy-real.yaml` | API proxy variant preferring *real* event & metrics endpoints (fallback only if unavailable). |
| `optimization-events-minikube.yaml` | Simple in-cluster mock optimization events source for Minikube / dev. |
| `optimization-events-server.yaml` | More featureful optimization events server variant (extended fields or aggregation). |
| `real-events-collector.yaml` | Baseline events collector (e.g. tailing real workloads / pod events). |
| `real-events-collector-v2.yaml` | Next-gen / experimental events collector with richer schema or filters. |
| `real-test-workloads.yaml` | A larger realistic workload mix (multiple deployments, resource profiles). |
| `test-workloads.yaml` | Standard functional workload set (balanced + burst + idle pods). |
| `test-workloads-quick.yaml` | Lightweight subset for very fast smoke tests (low pod count). |
| `stress-test.yaml` | High‑churn / scaling scenario for resilience & performance evaluation. |
| `dashboard-minikube.yaml` | Deploys the dashboard resources adapted for Minikube (NodePort or ClusterIP bridging). |
| `deploy-minikube.sh` | Helper script to bootstrap operator + example workloads on Minikube. |
| `verify-deployment.sh` | Script that runs post‑deployment checks (pods ready, metrics endpoint live, basic API responses). |

---

## Choosing a Starting Point

| Goal | Use These |
|------|-----------|
| Just run the operator | `right-sizer-deployment.yaml` |
| Force real metrics only | `right-sizer-no-mock.yaml` |
| Explore event + metrics flow | `right-sizer-real-data.yaml` + `real-events-collector*.yaml` |
| Dashboard demo | `dashboard-minikube.yaml` + `api-proxy*.yaml` |
| Quick smoke test | `test-workloads-quick.yaml` |
| Full functional exercise | `test-workloads.yaml` |
| Edge / resource pressure | `stress-test.yaml` |
| Compare collectors | `real-events-collector.yaml` vs `real-events-collector-v2.yaml` |
| Local iterative dev | `deploy-minikube.sh` + `verify-deployment.sh` |

---

## Typical Workflow (Fast Path)

```bash
# 1. Deploy base operator (mock-friendly)
kubectl apply -f right-sizer-deployment.yaml

# 2. Add quick workloads
kubectl apply -f test-workloads-quick.yaml

# 3. (Optional) Add optimization events mock
kubectl apply -f optimization-events-minikube.yaml

# 4. Verify status
./verify-deployment.sh
```

---

## Moving to Realistic Data

```bash
# Switch to non-mock / real metrics emphasis
kubectl apply -f right-sizer-no-mock.yaml

# Add real event collectors
kubectl apply -f real-events-collector.yaml
# or experimental:
kubectl apply -f real-events-collector-v2.yaml

# Add richer workload mix
kubectl apply -f real-test-workloads.yaml
```

---

## Stress Testing

```bash
kubectl apply -f stress-test.yaml
# Monitor operator CPU + reconciliation latency
kubectl top pods -n right-sizer
```

Recommended to pair with cluster metrics pipeline (Prometheus / metrics-server) and watch the operator's scaling decisions.

---

## Dashboard / UI Demo

```bash
# Deploy API proxy (mock + fallback)
kubectl apply -f api-proxy.yaml
# Or real-first:
kubectl apply -f api-proxy-real.yaml

# Deploy dashboard in Minikube
kubectl apply -f dashboard-minikube.yaml

# Use `kubectl port-forward` or `minikube service` to access
```

---

## Scripted Minikube Bootstrap

`deploy-minikube.sh` encapsulates:
1. Namespace creation
2. Operator deployment
3. Workload + events backend seeding
4. Optional dashboard wiring

You can safely modify it for CI ephemeral clusters.

---

## Verification

`verify-deployment.sh` performs:
- Namespace / pod readiness checks
- Metrics endpoint HTTP probe
- Basic API endpoint (e.g. `/metrics`, `/healthz`) validation
- Optional event source presence

Extend it by:
- Adding JSON schema validation of returned event documents
- Checking metric presence with `grep` filters

---

## Naming Conventions

| Pattern | Meaning |
|---------|--------|
| `*-real-*` | Prefers real or production-like signal sources. |
| `*-no-mock*` | Disables mock generators explicitly. |
| `*-minikube*` | Adjusted for local cluster defaults (ports, storage class, service type). |
| `*-collector*` | A pod/service that gathers or synthesizes events. |
| `test-workloads*` | Workload sets (baseline or reduced). |
| `stress-*` | High-pressure scenarios. |
| `*-v2` | Experimental or iterative upgrade over baseline manifest. |

---

## Customization Tips

1. Namespace Isolation
   Wrap each manifest set with `kubectl -n your-namespace` or add `metadata.namespace` blocks.

2. Resource Tuning
   For stress scenarios, scale `replicas` *and* widen CPU/memory request ranges to test resizing boundaries.

3. Policy Synergy
   After deploying workloads, apply `RightSizerPolicy` CRDs to observe difference in recommendations.

4. Cleanup
   ```
   kubectl delete -f examples/deploy/test-workloads.yaml
   kubectl delete -f examples/deploy/right-sizer-deployment.yaml
   ```

---

## When to Use Helm Instead

If you:
- Need consistent environment promotion (dev -> staging -> prod)
- Want templated overrides rather than raw YAML
- Rely on CI pipelines

…prefer the Helm chart (see project root docs). These raw manifests are best for:
- Education
- Quick iteration
- Isolated reproduction of a reported issue

---

## Safe Removal

If you build a slimmer distribution artifact:
- Keep only: `right-sizer-deployment.yaml`, one workloads file, and verification script.
- Archive or delete others—none are required for core operator functionality.

---

## Contributing New Examples

1. Keep names descriptive and scenario-based.
2. Add a short header comment block at top of new manifest:
   ```
   # Scenario: High-latency network simulation for adaptation
   # Focus: CPU under-utilization + memory bursts
   ```
3. Update the table in this README.

PRs that add examples without README updates may be requested to revise.

---

## Questions / Enhancements

Open a Git issue with:
- Example name
- Use case
- Whether it's mock / real / hybrid
- Any supporting policy CRDs

---

Happy sizing 🚀

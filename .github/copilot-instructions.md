## 🤖 AI Coding Agent Instructions for `right-sizer`

Concise, project-specific guidance to be productive quickly. Focus on actual patterns in this repo (Go 1.25 Kubernetes operator). Keep changes small, align with existing structure, and validate with tests.

### 1. Big Picture
Right-Sizer is a Kubernetes operator that performs zero-downtime in-place pod resource resizing (CPU & memory) for K8s 1.33+. Core loop: controllers watch CRDs (`RightSizerConfig`, `RightSizerPolicy`) + Pods → collect metrics (Metrics Server / Prometheus) → compute sizing (adaptive + prediction) → apply two-step in-place resize with `resizePolicy: NotRequired` (CPU first, then memory). Prediction subsystem augments decisions conservatively (only increases if prediction > current calculation with sufficient confidence).

### 2. Key Directories & Roles
- `go/main.go`: Bootstrap, capability detection (pods/resize, metrics), controller & webhook setup, leader election.
- `go/config/`: Global config (singleton `Load()`), CRD override precedence: CRD > env > defaults. Add new fields here and update controller logic.
- `go/controllers/`: Reconciliation + resizing logic (`adaptive_rightsizer.go`, `inplace_rightsizer.go`). Large file = many responsibilities—follow existing helper patterns when extending.
- `go/policy/`: Policy evaluation & priority resolution.
- `go/metrics/`: Provider abstraction; default metrics-server, optional Prometheus.
- `go/predictor/`: Pluggable algorithms (linear regression, exponential smoothing, moving average) with confidence thresholds.
- `go/admission/`: Webhooks (validation/mutation) that can inject resize policies.
- `helm/`: Deployment artifacts (CRDs in `helm/crds/`). Keep CRD schema changes synchronized.

### 3. Resource Resize Pattern (Do Not Break)
Two-step in-place sequence: (1) apply/ensure resizePolicy per container; (2) patch CPU only; (3) patch Memory only. Memory decreases are guarded & may be skipped. Maintain logging semantics (emoji prefix patterns) for observability. When modifying update logic, keep partial success behavior (proceed to memory even if CPU patch partly fails unless fatal).

### 4. Configuration Conventions
Add new tuning knobs as explicit fields in `config.Config` with default in `Load()`. Controllers reconcile CRD → call config update; avoid hidden globals. Respect rate limiting (QPS/Burst in `main.go`). Feature flags: e.g., `PredictionEnabled`. Cluster identity fields (`ClusterID`, `ClusterName`, `Environment`, `Version`) derive from env vars (`CLUSTER_ID`, `CLUSTER_NAME`, `ENVIRONMENT`, `OPERATOR_VERSION`) set early—use for event/metric tagging, don’t recompute. Follow existing naming (PascalCase in struct, descriptive comments). Hot reload implies thread-safe reads; avoid mutating slices in place—copy before changing.

### 5. Metrics & Observability
Expose new metrics using Prometheus client but register defensively (see `registerOnce` pattern in `main.go`). Use label sets consistent with existing metrics (`namespace`, `resource`, `type`). Health endpoints auto-provided by controller-runtime; add deeper checks by extending `health.NewOperatorHealthChecker()`.

### 6. Prediction Integration Rules
In adaptive calculations: only adopt prediction result when `prediction.Confidence >= cfg.PredictionConfidenceThreshold` and predicted value > base. Never downscale purely on prediction. If adding algorithms, implement strategy interface, add name to `PredictionMethods`, update confidence logic and tests.

### 7. Testing Workflow
- Unit tests: `go test -v ./...` from `go/`.
- Coverage: `go test -cover ./...` (HTML in `build/coverage/coverage.html`).
- E2E (local minikube): use Makefile targets `mk-start`, `mk-build-image`, `mk-deploy`, `mk-test` for rapid cycle.
Add tests near implementation file (`controllers/resize_policy_test.go`, `controllers/resize_test.go`). For large controller changes, isolate logic into small pure functions first and test them separately.

### 8. Safe Change Checklist (Internal Pattern)
1. Update config struct & defaults.
2. Adjust CRD (if spec surface changes) in `helm/crds/` + regenerate docs/examples.
3. Add validation hooks (if new constraints) in `go/validation/`.
4. Add metrics (optional) with guarded registration.
5. Write focused unit tests (happy path + edge: missing metrics-server, low confidence prediction, memory decrease denial).
6. Run `go test` + (optionally) minikube Makefile flow for integration.

### 9. Error Handling & Resilience Patterns
Use structured logging via `logger` package (Info/Warn/Success). Prefer returning `(string, error)` where existing update functions do to attach operation summary. Retry/circuit logic lives in `go/retry/`; use it rather than ad-hoc sleeps. Rate limiting: respect `MaxConcurrentReconciles` (don’t spawn uncontrolled goroutines inside reconciler).

### 10. Extending Policies
When adding policy attributes: update CRD schema + policy reconciliation to compute effective strategy. Preserve priority resolution (higher number = higher priority). Never silently override conflicting fields; log at Warning level.

### 11. Admission Webhook Mutations
If injecting new defaults into pods, follow existing JSON patch generation style and ensure idempotency (skip if field already set). Resize policy injection must not restart pods; keep `NotRequired`. Consider multi-container edge cases (index alignment).

### 12. Common Edge Cases To Test
- Cluster without metrics-server (expect degraded mode log & conservative sizing).
- Prediction disabled (`PredictionEnabled=false`).
- Pods lacking `pods/resize` subresource (fallback behavior—do not panic).
- Memory decrease attempt below safe threshold.
- CRD update race (config reload while resizing). Use copies of config during calculation.

### 13. Performance Considerations
Avoid O(N*containers) repeated API GETs: batch gather pod state once per resize cycle. Cache metrics where feasible; do not add synchronous sleeps unless essential. Keep per-pod decision latency <100ms; offload heavy computation outside hot reconcile path.

### 14. Adding New Metrics Provider
Implement provider interface (mirror pattern in `go/metrics/`), register selection via RightSizerConfig, ensure zeroed metrics fallback on errors (do not propagate failures). Log a single warning then rate-limit subsequent messages.

### 15. Style & Dependencies
Stick to stdlib + existing libraries in `go.mod`. Avoid introducing heavy new deps without justification. Keep new files small (<300 lines) or refactor large monolith segments.

### 16. Release & Version Tagging
Binary version comes from ldflags (`main.Version`). If changing build info fields, update Makefile `LDFLAGS` and Dockerfile build args together.

### 17. When Unsure
Search similar patterns in `adaptive_rightsizer.go` before creating new abstractions. Prefer extraction (ResourceCalculator style) to duplicating blocks. Ask (via PR description) if changing public CRD fields or resize semantics.

---
Questions or unclear conventions? Surface them early—maintainers favor small iterative PRs over large rewrites.

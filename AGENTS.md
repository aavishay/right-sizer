# AGENTS.md - Agentic Developer Guide

## Build, Lint & Test Commands

**Go Operator** (`right-sizer/`):
- **Run all tests**: `cd right-sizer && make test`
- **Run single test**: `cd right-sizer && go test -race -run TestName -v ./path/to/package`
- **Coverage**: `cd right-sizer && make test-coverage` (90% required)
- **Lint & Format**: `cd right-sizer && make lint` and `go fmt ./...`
- **Build binary**: `cd right-sizer && make build`
- **E2E testing**: `cd right-sizer && make mk-test` (requires minikube)

**Dashboard** (`right-sizer-dashboard-a4df5c80/`):
- **All tests**: `npm test` (backend + frontend)
- **Backend tests**: `npm --workspace=backend test`
- **Frontend tests**: `npm --workspace=frontend test`
- **E2E tests**: `npm run test:e2e` or `SANITY_MODE=true npm run test:e2e`
- **Lint**: `npm run lint` (backend + frontend)
- **Typecheck**: `npm run typecheck:backend`
- **Build**: `npm run build` (frontend + backend)

## Code Style & Project Patterns

- **Go Style**: Follow `.golangci.yml` rules - `gofmt -s`, 140-char lines, Go 1.25 idioms
- **Go Imports**: `goimports` with `right-sizer` local prefix, stdlib → external → local grouping
- **Go Naming**: camelCase variables, PascalCase exports, `(value, error)` return tuples
- **Go Error Handling**: `fmt.Errorf("%w", err)` for wrapping, structured logging with `go-logr/zapr`
- **Go Patterns**: Thread-safe config singleton (RWMutex), three-phase resize: NotRequired flag → CPU → Memory
- **TypeScript**: Strict mode enabled, Zod validation, Prisma ORM for database

## Critical Integration Rules

- **Database**: Hostname must be `timescaledb` (not `postgres`)
- **Kubernetes**: Use `right-sizer` namespace, `v1alpha1` CRDs, CDK8s deployments
- **Concurrency**: No global state without RWMutex protection
- **Auth**: JWT validation with `decoded.userId || decoded.id` fallback
- **Frontend**: Always use `apiClient`, never bare `fetch()`

## Architecture Summary

- **Dual-project**: Go operator (right-sizer/) + React dashboard (right-sizer-dashboard-a4df5c80/)
- **Operator**: Bootstrap in main.go, controllers in adaptive_rightsizer.go, critical resize pattern
- **Dashboard**: CDK8s preferred, hostname `timescaledb`, JWT validation with fallback
- **Workflows**: `make compose-up` for full stack, `npm run deploy:dev` for CDK8s

**References**: `.github/copilot-instructions.md`, `.golangci.yml`, `Makefile`

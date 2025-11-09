# Pre-Commit Hooks Setup Guide

This guide walks you through setting up and using pre-commit hooks for the right-sizer project.

## Prerequisites

- Python 3.7+ installed
- Git repository initialized
- Docker (for some optional hooks like hadolint)

## Installation

### Step 1: Install pre-commit

```bash
# Using pip (recommended)
pip install pre-commit

# Using Homebrew (macOS)
brew install pre-commit

# Using conda
conda install -c conda-forge pre-commit
```

### Step 2: Run the setup script

```bash
cd right-sizer
bash scripts/setup-precommit.sh
```

Or manually run:

```bash
cd right-sizer
pre-commit install --hook-type pre-commit
pre-commit install --hook-type commit-msg
```

### Step 3: Verify installation

```bash
ls -la .git/hooks/
```

You should see `pre-commit` and `pre-commit-msg` files.

## How It Works

Pre-commit hooks automatically run code quality checks **before each commit**. If any check fails, the commit is blocked until issues are fixed.

### Available Hooks

#### Standard Hooks (from `pre-commit/pre-commit-hooks`)
- ✅ **trailing-whitespace**: Remove trailing whitespace
- ✅ **end-of-file-fixer**: Ensure files end with a newline
- ✅ **check-yaml**: Validate YAML files
- ✅ **check-merge-conflict**: Detect merge conflict markers
- ✅ **check-added-large-files**: Prevent large files (>500KB)
- ✅ **mixed-line-ending**: Normalize line endings to LF

#### Go-Specific Hooks
- ✅ **go-fmt**: Auto-format Go code with `gofmt -s`
- ✅ **go-vet**: Run `go vet` analysis
- ✅ **go-mod-tidy**: Tidy Go modules
- ✅ **go-test**: Run all tests with race detection
- ✅ **go-build**: Ensure code compiles
- ✅ **go-security-scan**: Run vulnerability checks with `govulncheck`
- ✅ **go-integration-test**: Run integration tests (optional)
- ✅ **check-go-mod**: Verify module integrity

#### Infrastructure Hooks
- ✅ **yamllint**: Lint YAML files
- ✅ **shellcheck**: Lint shell scripts
- ✅ **helm-lint**: Validate Helm charts
- ✅ **detect-secrets**: Detect accidentally committed secrets
- ✅ **gosec**: Security scanner for Go code
- ✅ **hadolint**: Lint Dockerfiles (requires Docker)

## Common Commands

### Run pre-commit on all files
```bash
cd right-sizer
pre-commit run --all-files
```

### Run specific hook
```bash
pre-commit run go-fmt --all-files
pre-commit run go-test --all-files
```

### Skip pre-commit on a commit
```bash
git commit --no-verify
```

### Update hook definitions
```bash
pre-commit autoupdate
```

### View hook configuration
```bash
cat .pre-commit-config.yaml
```

## Workflow Example

1. Make code changes
2. Stage your changes: `git add .`
3. Try to commit: `git commit -m "Your message"`
4. Pre-commit runs automatically:
   - If all checks pass ✅ → Commit succeeds
   - If any check fails ❌ → Commit blocked
5. Fix any issues found
6. Re-run the commit

## Troubleshooting

### Hook failed: "go: command not found"
Ensure Go is in your PATH:
```bash
go version
export PATH=$PATH:$(go env GOPATH)/bin
```

### Hook failed: "golangci-lint not found"
Install golangci-lint:
```bash
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Hook failed: "helm not found"
Install Helm:
```bash
# macOS
brew install helm

# Linux
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
```

### Hook timeout
Some hooks may timeout on large codebases. You can:
- Skip the hook: `pre-commit run --hook-stage push`
- Increase timeout in `.pre-commit-config.yaml`
- Run manually: `cd go && make test`

### Permanently disable a hook
Edit `.pre-commit-config.yaml` and remove the hook entry, or set `stages: []`

## Best Practices

1. **Run tests locally before pushing**: Don't rely solely on CI/CD
2. **Use `--no-verify` sparingly**: Only for emergency fixes
3. **Keep hooks fast**: Disable slow hooks like `hadolint` if needed
4. **Regular updates**: Run `pre-commit autoupdate` monthly
5. **Team consistency**: Share `.pre-commit-config.yaml` with your team

## CI/CD Integration

Pre-commit hooks also run in GitHub Actions when you push to branches.

See `.github/workflows/pr-checks.yml` for automated checks on pull requests.

## Additional Resources

- [Pre-commit documentation](https://pre-commit.com/)
- [Pre-commit hooks registry](https://pre-commit.com/hooks.html)
- [Go best practices](https://golang.org/doc/effective_go)
- [Helm best practices](https://helm.sh/docs/chart_best_practices/)

---

**Last Updated**: November 7, 2025
**Maintainer**: Right-Sizer Development Team

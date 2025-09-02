# Security Policy

## Supported Versions

We release patches for security vulnerabilities. Which versions are eligible for receiving such patches depends on the CVSS v3.0 Rating:

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |
| < 0.1   | :x:                |

## Reporting a Vulnerability

If you discover a security vulnerability within Right-Sizer, please send an e-mail to the maintainers. All security vulnerabilities will be promptly addressed.

Please do not open issues for anything you think might have a security implication.

### What to Include in Your Report

- Type of issue (e.g., buffer overflow, SQL injection, cross-site scripting, etc.)
- Full paths of source file(s) related to the manifestation of the issue
- The location of the affected source code (tag/branch/commit or direct URL)
- Any special configuration required to reproduce the issue
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the issue, including how an attacker might exploit the issue

## Security Update Process

1. The reported security issue is acknowledged within 48 hours
2. A fix is developed and tested
3. A security advisory is prepared
4. The fix is released along with the advisory
5. The security advisory is published

## Dependency Management

### Regular Updates

We regularly update dependencies to ensure we're using versions with the latest security patches:

- **Monthly**: Review and update patch versions of dependencies
- **Quarterly**: Review and update minor versions where appropriate
- **As Needed**: Immediate updates for critical security vulnerabilities

### Dependency Scanning

Our CI/CD pipeline includes:

1. **Trivy Scanner**: Scans Docker images for vulnerabilities
2. **Go Vulnerability Database**: Checks Go dependencies for known vulnerabilities
3. **GitHub Dependabot**: Automated dependency updates and security alerts

### Current Security Measures

#### Replace Directives

We use Go module replace directives to ensure specific versions of critical dependencies:

```go
replace golang.org/x/net => golang.org/x/net v0.17.0
replace golang.org/x/sys => golang.org/x/sys v0.17.0
replace golang.org/x/tools => golang.org/x/tools v0.17.0
```

These replacements ensure we're using versions that have addressed known vulnerabilities.

## Recent Security Updates

### CVE-2023-39325 (Fixed)
- **Component**: golang.org/x/net
- **Severity**: HIGH
- **Fixed Version**: v0.17.0
- **Description**: HTTP/2 rapid reset vulnerability that could cause excessive work
- **Resolution**: Updated from v0.10.0 to v0.17.0

## Container Security

### Base Image

We use `gcr.io/distroless/static:nonroot` as our base image for production containers:
- Minimal attack surface
- No shell or package managers
- Runs as non-root user
- Regular security updates from Google

### Build Process

Our Docker build process includes:
- Multi-stage builds to minimize final image size
- No secrets or sensitive data in images
- Version information embedded at build time
- Vulnerability scanning before push

## Kubernetes Security

### RBAC

The Right-Sizer operator uses minimal RBAC permissions:
- Only accesses required resources
- Namespace-scoped where possible
- No wildcard permissions
- Regular audit of permissions

### Pod Security

Recommended security context for deployment:

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 65532
  fsGroup: 65532
  readOnlyRootFilesystem: true
  capabilities:
    drop:
      - ALL
```

### Network Policies

Consider implementing network policies to restrict traffic:
- Allow ingress only on metrics port (9090)
- Allow egress only to Kubernetes API server
- Deny all other traffic by default

## Security Best Practices

### For Developers

1. **Never commit secrets**: Use Kubernetes secrets or external secret management
2. **Validate all inputs**: Sanitize and validate all user inputs
3. **Use structured logging**: Avoid logging sensitive information
4. **Keep dependencies updated**: Regularly update Go modules
5. **Run security scans**: Use `govulncheck` and `trivy` before commits

### For Operators

1. **Use latest versions**: Always deploy the latest stable version
2. **Enable audit logging**: Monitor and audit Right-Sizer activities
3. **Implement RBAC**: Use least-privilege principles
4. **Monitor metrics**: Watch for unusual patterns or behaviors
5. **Regular updates**: Apply security patches promptly

## Security Scanning Commands

### Local Development

```bash
# Scan Go dependencies for vulnerabilities
cd go && go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...

# Scan Docker image
trivy image docker.io/aavishay/right-sizer:latest

# Check for outdated dependencies
go list -u -m all
```

### CI/CD Pipeline

Our GitHub Actions workflows include:
- Trivy scanning on every build
- SBOM generation for supply chain security
- Security results uploaded to GitHub Security tab

## Compliance

### CIS Benchmarks

Right-Sizer follows CIS Kubernetes Benchmark recommendations:
- Non-root containers
- Read-only root filesystem
- No privileged containers
- Resource limits defined
- Security contexts enforced

### Supply Chain Security

- SBOM (Software Bill of Materials) generated for each release
- Signed container images (planned)
- Reproducible builds
- Dependency verification

## Security Contact

For security-related inquiries or to report vulnerabilities:
- Open a security advisory on GitHub (preferred)
- Email: security@right-sizer.io (monitored)

## Acknowledgments

We appreciate the security research community and will acknowledge researchers who responsibly disclose vulnerabilities.

## Resources

- [OWASP Kubernetes Security Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Kubernetes_Security_Cheat_Sheet.html)
- [CIS Kubernetes Benchmark](https://www.cisecurity.org/benchmark/kubernetes)
- [Kubernetes Security Best Practices](https://kubernetes.io/docs/concepts/security/)
- [Go Security Best Practices](https://golang.org/doc/security/)
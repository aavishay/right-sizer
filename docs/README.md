# Right-Sizer Documentation

This directory contains comprehensive documentation for the Right-Sizer Kubernetes operator.

## 📖 Documentation Index

### Getting Started
- **[Installation Guide](installation-guide.md)** - Complete setup and deployment instructions
- **[Troubleshooting K8s](troubleshooting-k8s.md)** - Kubernetes deployment issues and solutions

### Development & Testing
- **[Testing Guide](TESTING_GUIDE.md)** - Comprehensive testing documentation (unit, integration, E2E)
- **[Runtime Testing Guide](RUNTIME_TESTING_GUIDE.md)** - Runtime testing and validation procedures
- **[GitHub Actions Testing](github-actions-testing.md)** - Local CI/CD testing with act
- **[Act Testing Summary](act-testing-summary.md)** - Test results and validation status
- **[Code Review Checklist](code-review-checklist.md)** - Comprehensive review guidelines
- **[Coverage Improvements](coverage-improvements.md)** - Test coverage analysis and enhancements

### Architecture & Implementation
- **[Prediction System](prediction-system.md)** - AI/ML-based resource prediction architecture
- **[Feature Flag Implementation](feature-flag-implementation.md)** - Feature gate system design
- **[Resize Policy Implementation](resize-policy-implementation.md)** - Policy engine and resource sizing
- **[Minikube Deployment](minikube-deployment.md)** - Local development environment setup

### Project Management
- **[Review Summary](review-summary.md)** - Code review status and focus areas
- **[Changelog](changelog.md)** - Version history and release notes
- **[Reorganization Summary](reorganization-summary.md)** - Repository structure changes and improvements
- **[Self-Protection Fix](self-protection-fix.md)** - Fix for preventing operator from resizing itself

## 🚀 Quick Navigation

### For New Users
1. Start with [Installation Guide](installation-guide.md)
2. Review [Troubleshooting K8s](troubleshooting-k8s.md) for common issues
3. Check [Minikube Deployment](minikube-deployment.md) for local testing

### For Developers
1. Start with [Testing Guide](TESTING_GUIDE.md) for comprehensive testing
2. Review [Code Review Checklist](code-review-checklist.md)
3. Set up [GitHub Actions Testing](github-actions-testing.md)
4. Understand [Feature Flag Implementation](feature-flag-implementation.md)
5. Study [Prediction System](prediction-system.md) architecture

### For Reviewers
1. Check [Review Summary](review-summary.md) for current status
2. Use [Code Review Checklist](code-review-checklist.md) as guide
3. Review [Coverage Improvements](coverage-improvements.md) for test status

## 📁 Repository Structure

```
docs/
├── README.md                           # This file - Documentation index
├── TESTING_GUIDE.md                   # Comprehensive testing documentation
├── RUNTIME_TESTING_GUIDE.md           # Runtime testing procedures
├── installation-guide.md               # Setup and deployment instructions
├── troubleshooting-k8s.md             # Kubernetes deployment issues
├── github-actions-testing.md          # CI/CD testing guide
├── act-testing-summary.md             # Test validation results
├── code-review-checklist.md           # Review guidelines
├── coverage-improvements.md           # Test coverage analysis
├── prediction-system.md               # AI/ML architecture
├── feature-flag-implementation.md     # Feature gates system
├── resize-policy-implementation.md    # Policy engine design
├── minikube-deployment.md             # Local development setup
├── review-summary.md                  # Code review status
├── changelog.md                       # Version history
├── reorganization-summary.md          # Repository structure changes
├── self-protection-fix.md             # Self-protection implementation
├── ARM64_DEPLOYMENT_SUCCESS.md        # ARM64 deployment guide
├── METRICS_SERVER_DEPLOYMENT.md       # Metrics server setup
├── ci-testing/                        # CI/CD testing guides
│   ├── README.md                      # CI testing overview
│   ├── QUICK_START.md                 # Quick testing guide
│   ├── ADVANCED_TESTING.md            # Advanced testing scenarios
│   └── IDE_SETUP.md                   # IDE configuration
└── releases/                          # Release documentation
    └── v0.2.0.md                      # v0.2.0 release notes
```

## 🔧 Scripts & Tools

The repository also includes helpful scripts in the `scripts/` directory:
- **`scripts/test-github-actions.sh`** - GitHub Actions testing automation

## 📋 Documentation Standards

### File Naming Convention
- Use lowercase with hyphens: `feature-name.md`
- Be descriptive and specific
- Group related content logically

### Content Structure
- Start with clear title and purpose
- Include table of contents for long documents
- Use consistent markdown formatting
- Add examples and code snippets where helpful
- Include troubleshooting sections

### Maintenance
- Keep documentation up-to-date with code changes
- Review and update during code reviews
- Version documentation with releases
- Archive outdated information

## 🤝 Contributing to Documentation

1. **Follow the style guide** - Consistent formatting and structure
2. **Update the index** - Add new documents to this README
3. **Cross-reference** - Link related documents
4. **Test examples** - Ensure all code examples work
5. **Review process** - Documentation changes go through PR review

## 📞 Support

For questions about the documentation:
- Check the relevant troubleshooting guide first
- Review the code review checklist for process questions
- Consult the installation guide for setup issues
- Use the GitHub Actions testing guide for CI/CD questions

---

*Last updated: October 2024*
*Documentation version: 1.1*

# Documentation Index

## Overview

This directory contains all documentation for the Right-Sizer Kubernetes Operator. The documentation is organized by category to help you quickly find the information you need.

## 📚 Documentation Structure

### Getting Started
- **[README.md](../README.md)** - Main project documentation and quick start guide
- **[CONFIGURATION.md](CONFIGURATION.md)** - Complete configuration reference
- **[CONFIGURATION-CRD.md](CONFIGURATION-CRD.md)** - Custom Resource Definition configuration guide

### Development
- **[BUILD.md](BUILD.md)** - Build instructions and development setup
- **[CONTRIBUTING.md](CONTRIBUTING.md)** - Contribution guidelines and development process
- **[CHANGELOG.md](CHANGELOG.md)** - Version history and release notes

### Security & Permissions
- **[RBAC.md](RBAC.md)** - Comprehensive RBAC documentation and troubleshooting

### Feature Documentation
- **[MEMORY_METRICS_IMPLEMENTATION.md](MEMORY_METRICS_IMPLEMENTATION.md)** - Memory metrics collection and processing
- **[DUPLICATE_LOGGING_FIX.md](DUPLICATE_LOGGING_FIX.md)** - Logging improvements and duplicate reduction
- **[INFO_PREFIX_REMOVAL.md](INFO_PREFIX_REMOVAL.md)** - Logger enhancement for cleaner output

## 🗂️ Document Categories

### Configuration Guides
| Document | Description |
|----------|-------------|
| [CONFIGURATION.md](CONFIGURATION.md) | Complete configuration options including environment variables, Helm values, and runtime settings |
| [CONFIGURATION-CRD.md](CONFIGURATION-CRD.md) | RightSizerConfig and RightSizerPolicy CRD specifications |

### Operational Guides
| Document | Description |
|----------|-------------|
| [BUILD.md](BUILD.md) | Building from source, Docker images, and deployment methods |
| [RBAC.md](RBAC.md) | Required permissions, ServiceAccount setup, and security best practices |

### Development Guides
| Document | Description |
|----------|-------------|
| [CONTRIBUTING.md](CONTRIBUTING.md) | How to contribute, coding standards, and PR process |
| [CHANGELOG.md](CHANGELOG.md) | Release history, breaking changes, and migration guides |

### Technical Deep-Dives
| Document | Description |
|----------|-------------|
| [MEMORY_METRICS_IMPLEMENTATION.md](MEMORY_METRICS_IMPLEMENTATION.md) | Architecture of memory metrics collection and scaling decisions |
| [DUPLICATE_LOGGING_FIX.md](DUPLICATE_LOGGING_FIX.md) | How duplicate logging was identified and resolved |
| [INFO_PREFIX_REMOVAL.md](INFO_PREFIX_REMOVAL.md) | Logger improvements for better readability |

## 🔍 Quick Reference

### For Operators
- Start with [CONFIGURATION.md](CONFIGURATION.md) for deployment options
- Review [RBAC.md](RBAC.md) for security setup
- Check [CHANGELOG.md](CHANGELOG.md) before upgrades

### For Developers
- Read [CONTRIBUTING.md](CONTRIBUTING.md) before starting
- Follow [BUILD.md](BUILD.md) for local development
- Review technical documentation for component understanding

### For Troubleshooting
- [RBAC.md](RBAC.md) - Permission errors
- [DUPLICATE_LOGGING_FIX.md](DUPLICATE_LOGGING_FIX.md) - Log verbosity issues
- [MEMORY_METRICS_IMPLEMENTATION.md](MEMORY_METRICS_IMPLEMENTATION.md) - Memory scaling issues

## 📋 Documentation Standards

### File Naming
- Use UPPERCASE for top-level guides (e.g., `BUILD.md`)
- Use descriptive names with underscores for technical docs (e.g., `MEMORY_METRICS_IMPLEMENTATION.md`)
- Keep names concise but meaningful

### Content Structure
Each document should include:
1. **Title** - Clear document title
2. **Overview** - Brief description of content
3. **Table of Contents** - For documents > 200 lines
4. **Main Content** - Organized with clear headers
5. **Examples** - Practical examples where applicable
6. **Troubleshooting** - Common issues and solutions
7. **References** - Links to related documentation

### Markdown Guidelines
- Use ATX-style headers (`#`, `##`, etc.)
- Include code blocks with language specification
- Use tables for structured data
- Add links to related documents
- Include diagrams where helpful

## 🔄 Maintenance

### Adding New Documentation
1. Place the document in this `docs/` directory
2. Update this INDEX.md with the new document
3. Add appropriate cross-references in related docs
4. Update the main README.md if necessary

### Updating Existing Documentation
1. Keep CHANGELOG.md updated with changes
2. Mark deprecated sections clearly
3. Maintain backward compatibility notes
4. Update examples to reflect current best practices

## 📝 Documentation TODO

- [ ] Add architecture diagrams
- [ ] Create troubleshooting decision tree
- [ ] Add performance tuning guide
- [ ] Create migration guides for major versions
- [ ] Add more real-world examples

## 🆘 Getting Help

If you can't find what you need:
1. Check the [main README](../README.md)
2. Search existing [GitHub issues](https://github.com/your-org/right-sizer/issues)
3. Join our community discussions
4. Open a documentation issue with your question

## 📜 License

All documentation is provided under the same license as the Right-Sizer project. See [LICENSE](../LICENSE) for details.
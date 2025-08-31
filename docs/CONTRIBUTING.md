# Contributing to right-sizer

Thank you for your interest in contributing to the right-sizer operator! We welcome contributions from the community and are grateful for any help you can provide.

## License Agreement

By contributing to this project, you agree that your contributions will be licensed under the GNU Affero General Public License v3.0 (AGPL-3.0), the same license as the project. Please read the [LICENSE](LICENSE) file for full details.

**Important**: All contributions must be compatible with the AGPL-3.0 license. This means:
- Your contributions will be open source
- Any modifications must also be licensed under AGPL-3.0
- If you use this code in a network service, you must provide the source code to users

## How to Contribute

### 1. Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork locally:
   ```bash
   git clone https://github.com/yourusername/right-sizer.git
   cd right-sizer
   ```

### 2. Create a Branch

Create a branch for your feature or bugfix:
```bash
git checkout -b feature/your-feature-name
# or
git checkout -b fix/your-bugfix-name
```

### 3. Make Your Changes

1. **Add License Headers**: All new Go source files must include the AGPL-3.0 license header:
   ```go
   // Copyright (C) 2024 right-sizer contributors
   //
   // This program is free software: you can redistribute it and/or modify
   // it under the terms of the GNU Affero General Public License as published by
   // the Free Software Foundation, either version 3 of the License, or
   // (at your option) any later version.
   //
   // This program is distributed in the hope that it will be useful,
   // but WITHOUT ANY WARRANTY; without even the implied warranty of
   // MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   // GNU Affero General Public License for more details.
   //
   // You should have received a copy of the GNU Affero General Public License
   // along with this program.  If not, see <https://www.gnu.org/licenses/>.
   ```

2. **Follow Code Standards**:
   - Use `gofmt` to format your code
   - Follow Go best practices and idioms
   - Add comments for exported functions and types
   - Keep functions focused and small
   - Write clear, self-documenting code

3. **Update Documentation**:
   - Update README.md if you're adding new features
   - Update CONFIGURATION.md for new configuration options
   - Add inline documentation for complex logic

### 4. Test Your Changes

1. **Run Unit Tests**:
   ```bash
   go test ./...
   ```

2. **Run Integration Tests**:
   ```bash
   # Start Minikube
   minikube start
   
   # Build and test
   ./test-config.sh
   ./test-interval-loglevel.sh
   ```

3. **Test with Different Configurations**:
   - Test with minimum configuration
   - Test with aggressive settings
   - Test with conservative settings
   - Verify backward compatibility

### 5. Commit Your Changes

1. **Write Clear Commit Messages**:
   ```
   <type>(<scope>): <subject>
   
   <body>
   
   <footer>
   ```
   
   Types:
   - `feat`: New feature
   - `fix`: Bug fix
   - `docs`: Documentation changes
   - `style`: Code style changes (formatting, etc.)
   - `refactor`: Code refactoring
   - `test`: Test additions or changes
   - `chore`: Build process or auxiliary tool changes

   Example:
   ```
   feat(config): add support for custom resource multipliers
   
   - Add CPU_REQUEST_MULTIPLIER environment variable
   - Add MEMORY_REQUEST_MULTIPLIER environment variable
   - Update documentation with new configuration options
   
   Closes #123
   ```

2. **Sign Your Commits** (optional but recommended):
   ```bash
   git commit -s -m "Your commit message"
   ```

### 6. Push and Create Pull Request

1. Push your branch:
   ```bash
   git push origin feature/your-feature-name
   ```

2. Create a Pull Request on GitHub with:
   - Clear title describing the change
   - Detailed description of what was changed and why
   - Reference to any related issues
   - Test results or screenshots if applicable

## Development Guidelines

### Code Quality

- **Error Handling**: Always check and handle errors appropriately
- **Logging**: Use the structured logger with appropriate log levels
- **Configuration**: Use environment variables for configuration
- **Resources**: Be mindful of resource usage and clean up properly
- **Concurrency**: Handle concurrent operations safely

### Testing Requirements

All contributions should include:
1. Unit tests for new functions
2. Integration tests for new features
3. Documentation of test scenarios
4. Performance impact analysis for resource-intensive changes

### Documentation

- Update relevant documentation files
- Add code comments for complex logic
- Include examples for new features
- Update configuration documentation

## Code Review Process

1. **Automated Checks**: All PRs must pass:
   - Build verification
   - Unit tests
   - Code formatting checks
   - License header verification

2. **Manual Review**: Maintainers will review:
   - Code quality and style
   - Test coverage
   - Documentation completeness
   - Compatibility with project goals

3. **Feedback**: Be prepared to:
   - Address review comments
   - Make requested changes
   - Discuss design decisions

## Reporting Issues

When reporting issues, please include:
1. Kubernetes version
2. right-sizer version
3. Configuration (environment variables)
4. Steps to reproduce
5. Expected behavior
6. Actual behavior
7. Relevant logs

## Security Issues

For security issues, please email the maintainers directly rather than opening a public issue. We take security seriously and will respond promptly.

## Questions or Need Help?

- Open a GitHub issue for bugs or feature requests
- Start a GitHub Discussion for general questions
- Check existing issues and discussions first

## Recognition

Contributors will be recognized in:
- The NOTICE file
- Release notes
- Project documentation

Thank you for contributing to right-sizer and helping make Kubernetes resource management better for everyone!

## Legal Notice

By contributing to this repository, you agree that your contributions will be licensed under the GNU Affero General Public License v3.0 (AGPL-3.0). You represent that you have the right to license your contributions under these terms.
feat: migrate to CRD-based configuration, remove env vars and policy rules

BREAKING CHANGE: Configuration is now managed entirely through CRDs instead of environment variables

## Major Changes

### Configuration System Overhaul
- Replaced all environment variable configuration with CRD-based system
- Removed hardcoded policy rules and policy engine
- Configuration now dynamically loaded from RightSizerConfig CRD
- Added thread-safe configuration updates

### Core Changes

#### config/config.go
- Removed all environment variable loading logic
- Added UpdateFromCRD() method for CRD-based updates
- Added thread-safe mutex protection
- Added configuration source tracking
- Added Clone() method for safe config copies
- Removed parseCSV and related env var helper functions

#### main.go
- Removed environment variable configuration loading
- Removed policy engine initialization
- Configuration initialized with defaults, updated via CRD
- Simplified startup process
- Added CRD controller initialization

#### controllers/rightsizerconfig_controller.go
- Updated to properly apply configuration from CRD
- Added dynamic metrics provider switching
- Added feature component management
- Fixed type handling for non-pointer CRD fields
- Added proper status updates

### Helm Chart Updates

#### templates/deployment.yaml
- Removed all configuration environment variables
- Only kept essential pod metadata env vars
- Added webhook certificate volume mounts

#### values.yaml
- Restructured for CRD-based configuration
- Added defaultConfig section for RightSizerConfig
- Added examplePolicies section
- Updated feature gates to boolean values
- Removed env var based configuration

#### New Templates
- Added templates/config/rightsizerconfig.yaml for default config
- Added templates/policies/example-policies.yaml for example policies

### Documentation
- Created comprehensive CRD_CONFIGURATION.md guide
- Added migration guide from env vars to CRDs
- Added usage examples and best practices
- Added troubleshooting section

### Testing
- Created unit tests for new configuration system
- Added thread safety tests
- Created integration test script for CRD configuration
- Moved tests to proper tests/ directory structure

### Project Structure
- Moved test files to tests/ directory
- Cleaned up temporary files
- Updated go.mod dependencies

## Benefits
- Dynamic configuration updates without operator restart
- GitOps compatible configuration as code
- Namespace-scoped configuration support
- Policy-based resource management
- CRD schema validation
- Auditable configuration changes
- No secrets in environment variables

## Migration Notes
Users migrating from the previous version need to:
1. Create RightSizerConfig CRD with their previous settings
2. Remove environment variables from deployment
3. Apply new Helm chart with createDefaultConfig=true

Fixes: #RBAC-001
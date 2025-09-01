# Archived Examples

This directory contains example files that have been archived because they don't use the current CRD-based configuration system for the right-sizer operator.

## Why These Files Were Archived

The right-sizer operator has evolved to use Custom Resource Definitions (CRDs) for all configuration:
- `RightSizerConfig` - For global operator configuration
- `RightSizerPolicy` - For defining resource sizing policies

These archived examples use older configuration methods that are no longer supported:
- ConfigMaps for configuration
- Environment variables in Deployments
- Direct deployment manifests
- Annotations-based configuration

## Archived Files

### Configuration Examples (Non-CRD)
- **`addition-based-sizing.yaml`** - Used ConfigMaps to configure resource additions and multipliers
- **`advanced-configuration.yaml`** - Deployment-based configuration with environment variables
- **`config-scenarios.yaml`** - Various deployment scenarios with environment variable configuration
- **`full-configuration.yaml`** - Complete deployment manifest with all configuration options
- **`in-place-resize-demo.yaml`** - Deployment example for testing in-place resize feature

### Meta Files
- **`policy-rules-example.yaml`** - Contains CRD definitions themselves (not examples of using them)

## Current Configuration Method

All configuration should now be done using CRDs. Please refer to the active examples in the parent directory:

- `crd-config.yaml` - RightSizerConfig examples
- `crd-policies.yaml` - RightSizerPolicy examples
- `crd-namespace-filters.yaml` - Namespace filtering configuration
- `rightsizerconfig-with-thresholds.yaml` - Scaling threshold configuration
- `rate-limiting-config.yaml` - Rate limiting configuration

## Migration Guide

If you were using the old configuration method:

1. **From ConfigMap**: Convert your ConfigMap settings to a `RightSizerConfig` CRD
   - Multipliers → `defaultResourceStrategy.cpu/memory.requestMultiplier`
   - Additions → `defaultResourceStrategy.cpu/memory.requestAddition`

2. **From Environment Variables**: Move environment variables to CRD fields
   - `CPU_REQUEST_MULTIPLIER` → `defaultResourceStrategy.cpu.requestMultiplier`
   - `MEMORY_LIMIT_MULTIPLIER` → `defaultResourceStrategy.memory.limitMultiplier`

3. **From Annotations**: Create policies instead
   - Pod annotations → `RightSizerPolicy` with appropriate selectors

## Reference Value

These archived files may still be useful for:
- Understanding the evolution of the operator
- Extracting configuration concepts
- Reference for migration from older versions

## Archive Date

Files archived on: 2024 (during CRD standardization cleanup)
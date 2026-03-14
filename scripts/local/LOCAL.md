# GoRepos Local Testing Framework

This directory contains the local testing framework for GoRepos functionality.

## Core Testing Infrastructure

- **[../test_local.ps1](../test_local.ps1)** - Shared testing logic used by all test environments
- **[grtest1.ps1](grtest1.ps1)** - Test Environment 1 (Default OneDrive Setup)
- **[grtest2.ps1](grtest2.ps1)** - Test Environment 2 (Custom Setup)
- **[grtest3.ps1](grtest3.ps1)** - Example of additional test environment

## Test Environment Details

| Environment | Config Location | Base Path | Purpose |
|-------------|----------------|-----------|---------|
| Test Env 1 | `C:\Users\bence\OneDrive\Documents\gorepos\gorepos.yaml` | `C:\Data\GIT\grtest\1` | Default user configuration discovery |
| Test Env 2 | `C:\Data\GIT\grtest\gorepos.yaml` | `C:\Data\GIT\grtest\2` | Custom configuration paths |
| Test Env 3 | `C:\Data\GIT\grtest\example.yaml` | `C:\Data\GIT\grtest\example` | Example extensibility |

## Test Sequence

The test framework runs commands in this sequence:

1. **Setup** (if `-Setup` flag) - Run `gorepos setup --force`
2. **Help Commands** (unless `-SkipHelp`) - Test all command help functions
3. **Validate** (unless `-SkipValidate`) - Run `gorepos validate`
4. **Status** (unless `-SkipStatus`) - Run `gorepos status`
5. **Graph** (unless `-SkipGraph`) - Run `gorepos graph`
6. **Groups** (if `-Groups`) - Run `gorepos groups`
7. **Clone** (if `-Clone`) - Run `gorepos clone`
8. **Update** (if `-Update`) - Run `gorepos update`

## Usage Examples

### Basic Testing

```powershell
# Test Environment 1 with setup
.\grtest1.ps1 -Setup

# Test Environment 2 with all features
.\grtest2.ps1 -Setup -Groups -VerboseOutput

# Skip help tests, run minimal validation
.\grtest1.ps1 -SkipHelp -SkipStatus -SkipGraph
```

### Advanced Testing Scenarios

```powershell
# Full test suite with verbose output
.\grtest1.ps1 -Setup -Groups -Clone -Update -VerboseOutput

# Fast validation only
.\grtest2.ps1 -SkipHelp -SkipStatus -SkipGraph

# Setup and basic operations
.\grtest1.ps1 -Setup -SkipHelp
```

### Running from Scripts Directory

```powershell
# From scripts directory - Test Environment 1 with setup
.\local\grtest1.ps1 -Setup

# From scripts directory - Test Environment 2 with all features
.\local\grtest2.ps1 -Setup -Groups -VerboseOutput

# From scripts directory - Skip help tests, run minimal validation
.\local\grtest1.ps1 -SkipHelp -SkipStatus -SkipGraph
```

## Available Test Flags

| Flag | Description |
|------|-------------|
| `-Setup` | Run gorepos setup --force |
| `-SkipHelp` | Skip help command tests |
| `-SkipValidate` | Skip gorepos validate |
| `-SkipStatus` | Skip gorepos status |
| `-SkipGraph` | Skip gorepos graph |
| `-Groups` | Run gorepos groups |
| `-Clone` | Run gorepos clone |
| `-Update` | Run gorepos update |
| `-VerboseOutput` | Enable verbose output for all commands |

## Creating Additional Test Environments

To create a new test environment, copy [grtest3.ps1](grtest3.ps1) and modify:

1. **Config File Path** - Where the configuration will be stored
2. **Base Path** - Repository base directory
3. **Test Name** - Descriptive name for the environment

Example:
```powershell
$ConfigFile = "C:\MyCustom\Path\gorepos.yaml"
$BasePath = "C:\MyCustom\Repos"
$TestName = "My Custom Test Environment"
```

## Architecture

### Shared Core Logic

The [../test_local.ps1](../test_local.ps1) script provides all common testing functionality:

- **Binary Discovery** - Automatic detection of built binaries from `dist/VERSION`
- **PATH Management** - Adds binary directory to PATH for clean command execution
- **Test Orchestration** - Configurable test sequence with granular control
- **Visual Feedback** - Comprehensive progress reporting with emoji indicators
- **Error Handling** - Detailed error reporting and exit code management

### Environment-Specific Wrappers

Each `grtest*.ps1` file is a lightweight wrapper that:

1. Defines environment-specific configuration (config file path, base path, test name)
2. Passes all parameters to the shared test script
3. Provides environment isolation for parallel testing

### Test Results

The framework provides comprehensive feedback:

- ✅ **Success indicators** for passed tests
- ❌ **Error indicators** for failed tests  
- ℹ️ **Information** about test progress
- 🔵 **Section headers** for test phases
- **Summary statistics** at completion

## Integration with Build System

These test scripts automatically:

1. Read version from `dist/VERSION` file
2. Locate the correct binary in `dist/windows-amd64/gorepos-{version}/`
3. Add binary directory to PATH for clean execution
4. Handle cross-platform path resolution
5. Provide detailed error reporting

### Prerequisites

Run the build system first:
```powershell
# From project root
.\scripts\build.ps1
```

Then run any test environment:
```powershell
# From scripts/local directory
.\grtest1.ps1 -Setup

# Or from project root
.\scripts\local\grtest1.ps1 -Setup
```

## Example Workflows

### Development Testing
```powershell
# Quick validation during development
.\grtest1.ps1 -SkipHelp -SkipStatus -SkipGraph

# Setup new environment and validate
.\grtest2.ps1 -Setup -SkipHelp
```

### Comprehensive Testing
```powershell
# Full test suite for Environment 1
.\grtest1.ps1 -Setup -Groups -Clone -Update -VerboseOutput

# Parallel testing of both environments
.\grtest1.ps1 -Setup -Groups &
.\grtest2.ps1 -Setup -Groups &
```

### CI/CD Integration
```powershell
# Automated testing pipeline
.\grtest1.ps1 -Setup -SkipHelp
.\grtest2.ps1 -Setup -SkipHelp
.\grtest3.ps1 -Setup -SkipHelp
```

The testing framework is designed for maximum flexibility while maintaining simplicity and reliability across different development scenarios.
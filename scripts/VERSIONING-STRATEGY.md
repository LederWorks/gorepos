# GoRepos Versioning Strategy

This document outlines the flexible versioning approach for the GoRepos project, designed to support both local development and CI/CD workflows.

## 🎯 Overview

GoRepos implements a **context-aware versioning system** that automatically selects the appropriate versioning strategy based on the development context:

- **Local Development**: Content-based versioning (independent of git state)
- **CI/CD Pipeline**: Git-based semantic versioning
- **Manual Override**: Custom version strings for testing

## 🔄 Versioning Modes

### 1. Git-Based Versioning (Default)

**Usage**: `./scripts/build.ps1` or `./scripts/build.sh`

**Strategy**:
1. `git describe --tags` (if tags exist) → `v1.2.3` or `v1.2.3-5-g1a2b3c4`
2. `git commit hash + timestamp` → `5f75f00-20260121-1430`
3. Fallback timestamp → `dev-20260121-1430`

**Best For**:
- CI/CD pipelines
- Official releases
- Git workflow integration
- Traceability to specific commits

**Example Outputs**:
```bash
v1.2.3                    # Tagged release
v1.2.3-5-g1a2b3c4        # 5 commits after v1.2.3
5f75f00-20260121-1430    # Untagged commit
5f75f00-dirty            # Uncommitted changes
```

### 2. Content-Based Versioning

**Usage**: `./scripts/build.ps1 -ContentHash` or `./scripts/build.sh --content-hash`

**Strategy**:
- SHA256 hash of all Go source files + `go.mod` + `go.sum`
- Format: `content-[8-char-hash]-[timestamp]`
- Independent of git history and uncommitted changes

**Best For**:
- Local development iterations
- Testing code changes without git commits
- Reproducible builds regardless of git state
- Development workflow optimization

**Example Outputs**:
```bash
content-a1b2c3d4-20260121-1430    # Content hash + timestamp
content-e5f6g7h8-20260121-1445    # Different content, same time
```

**Benefits**:
- ✅ Same code = same version (reproducible)
- ✅ Independent of git commit history
- ✅ Works without committing changes
- ✅ Fast iteration cycles
- ✅ No git pollution with temporary commits

### 3. Manual Version Override

**Usage**: `./scripts/build.ps1 -Version "custom-1.0"` or `./scripts/build.sh -v "custom-1.0"`

**Best For**:
- Testing specific version scenarios
- Custom deployment workflows
- Integration testing
- Special release builds

## 🏗️ Implementation Details

### Content Hash Calculation

The content-based versioning system:

1. **Scans for Source Files**:
   ```
   *.go files (recursive, excluding vendor/)
   go.mod
   go.sum (if present)
   ```

2. **Creates Composite Hash**:
   - Concatenates: `path:content` for each file
   - Calculates SHA256 hash
   - Takes first 8 characters (lowercase)

3. **Adds Timestamp**:
   - Format: `YYYYMMDD-HHMM`
   - Ensures uniqueness even for identical content

### Cross-Platform Compatibility

Both PowerShell (`build.ps1`) and Bash (`build.sh`) scripts implement identical functionality:

| Feature | PowerShell | Bash |
|---------|------------|------|
| Content hashing | ✅ SHA256 | ✅ sha256sum |
| Git integration | ✅ | ✅ |
| Timestamp format | ✅ | ✅ |
| Error handling | ✅ | ✅ |

## 📋 Usage Examples

### Local Development Workflow

```bash
# Quick iteration with content-based versioning
./scripts/build.sh --content-hash --target linux

# Output: content-a1b2c3d4-20260121-1430
# Same content = same version, regardless of git state
```

### CI/CD Pipeline Workflow

```bash
# Production build with git-based versioning
./scripts/build.sh --target all

# Output: v1.2.3 (if tagged) or 5f75f00-20260121-1430
# Traceable to specific git commit
```

### Testing Scenarios

```bash
# Test with custom version
./scripts/build.sh --version "test-feature-x" --target windows

# Rapid development cycle
./scripts/build.ps1 -ContentHash -Target current -Verbose
```

## 🚀 GitHub Actions Integration

For GitHub Actions workflows, use git-based versioning for proper release management:

```yaml
name: Build and Release

on:
  push:
    tags: ['v*']
  pull_request:

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      # Use git-based versioning (default)
      - name: Build for all platforms
        run: ./scripts/build.sh --target all --clean
        
      # For releases, use tag-based versioning
      - name: Build release
        if: startsWith(github.ref, 'refs/tags/v')
        run: ./scripts/build.sh --target all --version "${{ github.ref_name }}"
```

## 🔍 Debugging Version Detection

### Check Current Version Strategy

```bash
# Git-based (shows detection process)
./scripts/build.sh --verbose

# Content-based (shows hash calculation)
./scripts/build.sh --content-hash --verbose
```

### Troubleshooting

| Issue | Solution |
|-------|----------|
| `fatal: not a git repository` | Use `--content-hash` or `--version` |
| Content hash always changes | Check for auto-generated files |
| Version not updating | Verify git working directory |
| Build fails | Check Go installation and PATH |

## 📚 Best Practices

### Local Development
- Use `--content-hash` for rapid iteration
- Content-based versions are safe for testing
- No need to commit every small change

### CI/CD Integration
- Use git-based versioning (default)
- Tag releases with semantic versioning (`v1.2.3`)
- Let the build system auto-detect versions

### Team Workflows
- Document which versioning strategy to use
- Use content-hash for local dev, git for releases
- Consider automation for version bumping

### Version Management
- Tag releases: `git tag v1.2.3`
- Use semantic versioning for releases
- Content versions are ephemeral (development only)

## 🔧 Advanced Configuration

### Environment Detection

The build system can automatically detect context:

```bash
# Future enhancement: auto-detect CI environment
if [[ "$CI" == "true" ]]; then
    # Use git-based versioning
    VERSION_STRATEGY="git"
else
    # Use content-based for local dev
    VERSION_STRATEGY="content"
fi
```

### Custom Hash Algorithms

The content hashing can be extended for specific needs:

```bash
# Future: configurable hash algorithms
HASH_ALGO="${HASH_ALGO:-sha256}"  # sha1, md5, blake2b
```

## 🎉 Summary

This dual-mode versioning system provides:

- **Flexibility**: Choose the right strategy for your context
- **Productivity**: Fast local development with content hashing
- **Traceability**: Git-based versions for production releases
- **Simplicity**: Automatic detection with manual overrides
- **Consistency**: Cross-platform implementation

The system adapts to your workflow rather than forcing a specific approach, supporting both rapid development cycles and production release management.
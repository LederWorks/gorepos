# GoRepos Automatic Versioning Guide

This guide explains how to set up automatic versioning that creates new versions whenever your Go code changes.

## Current Enhanced Versioning

The build scripts now use an improved versioning strategy:

### Version Detection Priority
1. **Git Tags**: `git describe --tags --always --dirty` (e.g., `v1.2.3` or `v1.2.3-5-g1234abc-dirty`)
2. **Commit Hash + Timestamp**: `abc1234-20260121-1430` (when no tags exist)
3. **Development Timestamp**: `dev-20260121-1430` (fallback)

### Version Components
- **Git Hash**: Short commit hash (7 chars)
- **Timestamp**: `YYYYMMDD-HHMM` for uniqueness
- **Dirty Flag**: `-dirty` when uncommitted changes exist

## Automatic Versioning Strategies

### 1. Git Tag-Based Versioning (Production)

Create semantic version tags that automatically increment:

```bash
# Create version tags
git tag v1.0.0
git tag v1.1.0
git tag v1.2.0

# Build will use latest tag + commits
# Example output: v1.2.0-3-g1234abc (3 commits after v1.2.0)
```

### 2. Pre-Commit Hook (Automatic Tagging)

Create `.git/hooks/pre-commit` to auto-increment versions:

```bash
#!/bin/bash
# Auto-increment patch version on commit

# Get current version
current_version=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")

# Extract version parts
if [[ $current_version =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    major=${BASH_REMATCH[1]}
    minor=${BASH_REMATCH[2]}
    patch=${BASH_REMATCH[3]}
    
    # Increment patch version
    new_patch=$((patch + 1))
    new_version="v$major.$minor.$new_patch"
    
    # Only tag if there are actual changes to Go files
    if git diff --cached --name-only | grep -E '\.(go|mod|sum)$' > /dev/null; then
        git tag "$new_version"
        echo "Auto-tagged new version: $new_version"
    fi
fi
```

### 3. CI/CD Integration

For automated builds, set up version tagging in CI:

```yaml
# GitHub Actions example
name: Build and Release
on:
  push:
    branches: [main]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
      with:
        fetch-depth: 0  # Get full git history for versioning
    
    - name: Auto-tag version
      run: |
        # Auto-increment version based on commit messages
        if git log --format=%s -n 1 | grep -i "breaking\|major"; then
          # Major version bump
          npm version major --no-git-tag-version
        elif git log --format=%s -n 1 | grep -i "feat\|feature"; then
          # Minor version bump  
          npm version minor --no-git-tag-version
        else
          # Patch version bump
          npm version patch --no-git-tag-version
        fi
        
        NEW_VERSION="v$(node -p "require('./package.json').version")"
        git tag "$NEW_VERSION"
        git push origin "$NEW_VERSION"
    
    - name: Build all platforms
      run: ./scripts/build.sh --target all --clean
```

### 4. Development Workflow

For day-to-day development with automatic versioning:

```bash
# 1. Make code changes
vim cmd/gorepos/main.go

# 2. Commit changes (triggers version detection)
git add .
git commit -m "feat: add new command"

# 3. Build - version automatically includes commit hash + timestamp
./scripts/build.ps1

# Output: Built binary with version like "abc1234-20260121-1430"
```

### 5. Manual Version Override

For specific releases:

```bash
# Override version for release builds
./scripts/build.ps1 -Version "v1.0.0-rc1" -Target all

# Creates folders like: gorepos-v1.0.0-rc1
```

## Version-Based Output Structure

With automatic versioning, your dist structure becomes:

```
dist/
├── windows-amd64/
│   ├── gorepos-abc1234-20260121-1430/    # Development build
│   ├── gorepos-v1.0.0/                   # Tagged release
│   └── gorepos-v1.0.1-2-gdef5678/        # Post-release commits
├── linux-amd64/
│   ├── gorepos-abc1234-20260121-1430/
│   └── gorepos-v1.0.0/
└── VERSION                               # Current build version
```

## Benefits

### Automatic Version Detection
- ✅ **Code changes = new versions**: Every commit gets a unique version
- ✅ **No manual intervention**: Build script handles version detection
- ✅ **Development tracking**: Timestamp + hash for development builds  
- ✅ **Release management**: Git tags for official releases
- ✅ **Dirty state detection**: Shows uncommitted changes

### Distribution Ready
- ✅ **Clear identification**: Each build folder shows exact version
- ✅ **Easy packaging**: Zip `gorepos-v1.0.0` folders directly
- ✅ **Version comparison**: Easy to see what's newer/older
- ✅ **Rollback support**: Keep multiple versions available

## Best Practices

### Commit Message Conventions
Use conventional commits for automatic version bumping:

```bash
feat: add new command          # Minor version bump
fix: resolve build issue       # Patch version bump  
feat!: breaking API change     # Major version bump
docs: update README           # No version bump
```

### Release Workflow
```bash
# 1. Development builds (automatic)
git commit -m "feat: new feature"
./scripts/build.ps1  # Creates gorepos-abc1234-timestamp

# 2. Release candidates
git tag v1.0.0-rc1
./scripts/build.ps1  # Creates gorepos-v1.0.0-rc1

# 3. Final releases
git tag v1.0.0
./scripts/build.ps1  # Creates gorepos-v1.0.0
```

This system ensures every code change results in a uniquely identifiable version, making tracking, distribution, and debugging much easier.
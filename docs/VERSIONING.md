# GoRepos Versioning Guide

This document is the single source of truth for versioning in GoRepos.

GoRepos supports a context-aware versioning model that works for local development, CI/CD, and manual release flows.

## Overview

The build scripts support three versioning modes:

1. Git-based versioning (default)
2. Content-based versioning (`--content-hash` / `-ContentHash`)
3. Manual version override (`--version` / `-Version`)

Use content-based versions for local iteration and git-based versions for CI/CD and releases.

## Version Detection Priority

When no explicit version is provided, build scripts resolve versions in this order:

1. Git describe: `git describe --tags --always --dirty`
2. Commit hash + timestamp: `abc1234-20260121-1430`
3. Development fallback: `dev-20260121-1430`

Version components:

- Git hash: short commit hash (7 chars)
- Timestamp: `YYYYMMDD-HHMM`
- Dirty flag: `-dirty` when uncommitted changes exist

## Versioning Modes

### 1. Git-Based Versioning (Default)

Usage:

```bash
./scripts/build.sh
./scripts/build.ps1
```

Best for:

- CI/CD pipelines
- Official releases
- Traceability to exact commits

Example outputs:

```text
v1.2.3
v1.2.3-5-g1a2b3c4
5f75f00-20260121-1430
5f75f00-20260121-1430-dirty
```

### 2. Content-Based Versioning

Usage:

```bash
./scripts/build.sh --content-hash
./scripts/build.ps1 -ContentHash
```

Strategy:

- Hashes all `*.go` files (excluding `vendor/`) plus `go.mod` and optional `go.sum`
- Uses SHA-256
- Produces `[8-char-hash]-local`

Best for:

- Local development iterations
- Testing changes before commit
- Reproducible local builds independent of git metadata

Example outputs:

```text
a1b2c3d4-local
e5f6g7h8-local
```

Benefits:

- Same code content yields the same version
- No need to create temporary commits
- Fast feedback loops during development

### 3. Manual Version Override

Usage:

```bash
./scripts/build.sh --version "v1.0.0-rc1"
./scripts/build.ps1 -Version "v1.0.0-rc1"
```

Best for:

- Release candidates
- Integration testing
- Custom deployment workflows

## Build Output Structure

Build output is versioned per target platform and architecture:

```text
dist/
├── windows-amd64/
│   ├── gorepos-abc1234-20260121-1430/
│   ├── gorepos-v1.0.0/
│   └── gorepos-v1.0.1-2-gdef5678/
├── linux-amd64/
│   ├── gorepos-abc1234-20260121-1430/
│   └── gorepos-v1.0.0/
└── VERSION
```

This makes it easy to keep multiple builds, compare versions, and package artifacts.

## Local Development Workflow

```bash
# Build a local reproducible binary tied to file content
./scripts/build.sh --content-hash --target darwin

# Example output version: e159dd62-local
```

Recommended:

- Use `--content-hash` for daily development
- Use default git-based mode for CI/release builds

## Release Workflow

```bash
# 1. Commit changes
git add .
git commit -m "feat: add new command"

# 2. Optional release candidate
git tag v1.3.0-rc1

# 3. Final release tag
git tag v1.3.0

# 4. Build release artifacts
./scripts/build.sh --target all --clean
```

## CI/CD Integration

For CI builds, use git-based versioning (default) and ensure full history is fetched.

```yaml
name: Build and Release

on:
  push:
    branches: [main]
    tags: ['v*']
  pull_request:

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Build all platforms
        run: ./scripts/build.sh --target all --clean
```

If you want auto-tagging, keep it explicit and repository-specific rather than relying on non-Go tooling.

## Optional Auto-Tagging Hook

You can automate patch increments locally with a pre-commit hook.

```bash
#!/bin/bash
set -euo pipefail

current_version=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")

if [[ $current_version =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
	major=${BASH_REMATCH[1]}
	minor=${BASH_REMATCH[2]}
	patch=${BASH_REMATCH[3]}
	new_patch=$((patch + 1))
	new_version="v$major.$minor.$new_patch"

	if git diff --cached --name-only | grep -E '\.(go|mod|sum)$' > /dev/null; then
		git tag "$new_version"
		echo "Auto-tagged new version: $new_version"
	fi
fi
```

Note: team-level release tagging is usually better handled in CI or release automation.

## Troubleshooting

| Issue | Fix |
|---|---|
| `fatal: not a git repository` | Use `--content-hash` or provide `--version` |
| Version did not change | Verify files changed and correct working directory |
| Content hash changes unexpectedly | Check for generated files affecting sources |
| Build fails | Verify Go installation and PATH |

## Best Practices

- Use Conventional Commits to keep release intent clear
- Tag releases with semantic versions (`vMAJOR.MINOR.PATCH`)
- Prefer content-based mode for local experiments
- Prefer git-based mode for traceable CI artifacts
- Keep release tagging centralized (CI or release workflow)

## Summary

The GoRepos versioning system balances productivity and traceability:

- Local: content-based, reproducible, commit-independent
- CI/Release: git-based, semantic, traceable
- Special cases: manual overrides

This gives stable, identifiable build artifacts across platforms and workflows.

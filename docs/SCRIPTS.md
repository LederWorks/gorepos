# GoRepos Build and Test Scripts

This directory contains the build and local test scripts used by the GoRepos project.

Current scripts:

- `build.sh`
- `build.ps1`
- `test_local.ps1`

## Build Scripts

Both build scripts support cross-platform builds, optional tests, version embedding, selective cleaning, and versioned output folders.

Default behavior in both scripts:

- Target: `all`
- Architecture: `all`
- Output: `dist`
- Version: auto-detected unless explicitly set

### build.sh (Bash)

Usage:

```bash
./scripts/build.sh [options]
```

Options:

- `-t`, `--target` (`windows|linux|darwin|all`, default `all`)
- `-a`, `--arch` (`amd64|arm64|all`, default `all`)
- `-o`, `--output` (default `dist`)
- `-v`, `--version` (manual version override)
- `--content-hash` (use content-based local version)
- `-c`, `--clean` (clean output for selected targets)
- `--test` (run `go test ./...` before building)
- `--verbose`
- `-h`, `--help`

Examples:

```bash
# Build all targets
./scripts/build.sh

# Build Linux amd64, run tests, and clean target output first
./scripts/build.sh --target linux --arch amd64 --test --clean

# Build with explicit version
./scripts/build.sh --version "v1.0.0" --target darwin --arch arm64
```

### build.ps1 (PowerShell)

Usage:

```powershell
.\scripts\build.ps1 [-Target <value>] [-Arch <value>] [-Output <value>] [-Version <value>] [-ContentHash] [-Clean] [-Test] [-Verbose]
```

Parameters:

- `-Target` (`windows|linux|darwin|all`, default `all`)
- `-Arch` (`amd64|arm64|all`, default `all`)
- `-Output` (default `dist`)
- `-Version` (manual version override)
- `-ContentHash` (use content-based local version)
- `-Clean` (clean output for selected targets)
- `-Test` (run `go test ./...` before building)
- `-Verbose`

Examples:

```powershell
# Build all targets
.\scripts\build.ps1

# Build Linux amd64, run tests, and clean target output first
.\scripts\build.ps1 -Target linux -Arch amd64 -Test -Clean

# Build with explicit version
.\scripts\build.ps1 -Version "v1.0.0" -Target windows -Arch amd64
```

## Versioning

Version selection order (when no manual version is provided):

1. Git describe: `git describe --tags --always --dirty`
2. Git hash + timestamp: `<hash>-YYYYMMDD-HHMM` (plus optional `-dirty`)
3. Fallback: `dev-YYYYMMDD-HHMM`

Content-hash mode (`--content-hash` / `-ContentHash`):

- Hashes Go source plus `go.mod` and optional `go.sum`
- Produces `<8-char-hash>-local`

For complete versioning guidance, see [docs/VERSIONING.md](../docs/VERSIONING.md).

## Output Layout

Build output is written to `dist` by default:

```text
dist/
  <os>-<arch>/
    gorepos-<version>/
      gorepos[.exe]
  VERSION
```

Example:

```text
dist/
  darwin-arm64/
    gorepos-e159dd62-local/
      gorepos
  windows-amd64/
    gorepos-v1.2.3/
      gorepos.exe
  VERSION
```

## Selective Cleaning

`--clean` / `-Clean` removes only output folders for the targets being built. It does not blindly delete all builds unless your selection implies all targets.

## Local Test Script

`test_local.ps1` is a reusable local test runner for gorepos command validation.

Highlights:

- Requires `-ConfigFile`, `-BasePath`, and `-TestName`
- Optional switches for setup and command groups
- Reads version from `dist/VERSION`
- Uses the built Windows binary under `dist/windows-amd64/gorepos-<version>/gorepos.exe`

Typical flow:

```powershell
# 1. Build first
.\scripts\build.ps1 -Target windows -Arch amd64

# 2. Run local test suite
.\scripts\test_local.ps1 -ConfigFile "C:\temp\gorepos.yaml" -BasePath "C:\repos" -TestName "Local" -Setup
```

## Prerequisites

- Go `1.24.7` or newer workspace-compatible toolchain (see `go.mod`)
- Git (recommended for git-based versioning)
- Bash for `build.sh`
- PowerShell (pwsh) for `build.ps1` and `test_local.ps1`

## CI/CD Usage

GitHub Actions example:

```yaml
- name: Build GoRepos
  run: ./scripts/build.sh --target all --clean --test
```

Azure DevOps example:

```yaml
- powershell: |
    .\scripts\build.ps1 -Target all -Clean -Test
  displayName: Build GoRepos
```
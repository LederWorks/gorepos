# GoRepos Build & Test Scripts

This directory contains build scripts and testing framework for the GoRepos project.

## Build Scripts

### `build.ps1` (PowerShell - Windows)

PowerShell build script with comprehensive cross-platform compilation support.

**Basic Usage:**
```powershell
.\scripts\build.ps1
```

**Advanced Usage:**
```powershell
# Build for all platforms
.\scripts\build.ps1 -Target all -Clean

# Build for specific platform with tests
.\scripts\build.ps1 -Target linux -Arch amd64 -Test -Verbose

# Build with custom version and output directory
.\scripts\build.ps1 -Version "v1.0.0" -Output "release"
```

**Parameters:**
- `-Target`: Platform (windows, linux, darwin, all) [default: current platform]
- `-Arch`: Architecture (amd64, arm64, all) [default: all architectures]
- `-Output`: Output directory (default: dist)
- `-Version`: Version string to embed
- `-Clean`: Clean output directory first
- `-Test`: Run tests before building
- `-Verbose`: Enable verbose output

### `build.sh` (Bash - Unix/Linux/macOS)

Bash build script with equivalent functionality for Unix-like systems.

**Basic Usage:**
```bash
./scripts/build.sh
```

**Advanced Usage:**
```bash
# Build for all platforms
./scripts/build.sh --target all --clean

# Build for specific platform with tests
./scripts/build.sh -t linux -a amd64 --test --verbose

# Build with custom version
./scripts/build.sh --version "v1.0.0" --output "release"
```

**Options:**
- `-t, --target`: Platform (windows, linux, darwin, all) [default: current platform]
- `-a, --arch`: Architecture (amd64, arm64, all) [default: all architectures]
- `-o, --output`: Output directory (default: dist)
- `-v, --version`: Version string to embed
- `--content-hash`: Use content-based versioning for local development
- `-c, --clean`: Clean output directory first
- `--test`: Run tests before building
- `--verbose`: Enable verbose output
- `-h, --help`: Show help message

## Features

Both scripts provide:

- **Cross-platform compilation**: Build for Windows, Linux, and macOS from any platform
- **Architecture support**: Both amd64 and arm64 architectures
- **Version embedding**: Automatic version detection from git or custom version strings
- **Testing integration**: Optional test execution before building
- **Clean builds**: Option to clean output directory before building
- **Verbose output**: Detailed build information when needed
- **Error handling**: Proper error reporting and exit codes
- **Binary organization**: Organized output with platform-specific naming

## Versioning Strategies

The build system supports two versioning approaches:

### 🔄 Git-Based Versioning (Default)

**Usage**: `./scripts/build.sh` (default behavior)

Automatically detects version from git:
- `v1.2.3` (if tagged)
- `5f75f00-20260121-1430` (commit + timestamp)
- `dev-20260121-1430` (fallback)

**Best for**: CI/CD pipelines, official releases, git workflow integration

### 📦 Content-Based Versioning 

**Usage**: `./scripts/build.sh --content-hash`

Generates version from source code content:
- `content-a1b2c3d4-20260121-1430`
- Same code = same version (reproducible)
- Independent of git commits and uncommitted changes

**Best for**: Local development, rapid iteration, testing without git commits

### Examples

```bash
# Git-based versioning (default)
./scripts/build.sh --target all
# → Version: 5f75f00-20260121-1430

# Content-based versioning
./scripts/build.sh --content-hash --target linux  
# → Version: content-a1b2c3d4-20260121-1430

# Custom version
./scripts/build.sh --version "test-build-1" --target windows
# → Version: test-build-1
```

For detailed versioning strategy documentation, see [`VERSIONING-STRATEGY.md`](./VERSIONING-STRATEGY.md).

## Output Structure

Built binaries are organized in the output directory (default: `dist/`) with the following folder structure:

```
dist/
├── windows-amd64/
│   └── gorepos-v1.0.0/
│       └── gorepos.exe
├── windows-arm64/
│   └── gorepos-v1.0.0/
│       └── gorepos.exe
├── linux-amd64/
│   └── gorepos-v1.0.0/
│       └── gorepos
├── linux-arm64/
│   └── gorepos-v1.0.0/
│       └── gorepos
├── darwin-amd64/
│   └── gorepos-v1.0.0/
│       └── gorepos
├── darwin-arm64/
│   └── gorepos-v1.0.0/
│       └── gorepos
└── VERSION
```

Each platform-architecture combination gets its own folder, with tool-name-version subfolders for easy packaging and distribution. The version folders include the tool name making them easily identifiable when zipped for distribution.

### Selective Cleaning

The `--clean` flag intelligently cleans only what will be built:

- **Specific target/arch**: Only cleans that platform-architecture folder
- **All platforms, specific arch**: Cleans all platform folders for that architecture
- **Specific platform, all arch**: Cleans both architectures for that platform
- **No target/arch specified**: Cleans entire output directory

This preserves other builds and allows incremental building.

### Version Management

Each build creates a version subfolder based on:
- Git version (`git describe --tags --always --dirty`)
- Development timestamp fallback (`dev-YYYYMMDD`)
- Manual version override via command line

This enables:
- **Side-by-side versions**: Multiple versions can coexist
- **Easy packaging**: Each platform-arch-version is self-contained
- **Distribution**: Simple tarball/zip creation per platform (e.g., `gorepos-v1.0.0.zip`)
- **Rollback**: Easy access to previous builds
- **Clear identification**: Tool name in folder makes distribution packages obvious

## Version Detection

Version information is automatically detected using:

1. Git tags and commits (`git describe --tags --always --dirty`)
2. Fallback to development timestamp (`dev-YYYYMMDD`)
3. Manual override via command line parameter

The version is embedded into the binary using Go's `-ldflags "-X main.version=..."`. The `version` variable must be declared in `package main` (see `cmd/gorepos/main.go`).

## Prerequisites

- **Go 1.25.5+**: Required for building (matches `go.mod`)
- **Git** (optional): For automatic version detection
- **PowerShell 5.1+ or PowerShell Core** (for build.ps1)
- **Bash 4.0+** (for build.sh)

## Examples

### Quick Development Build
```bash
# Unix/Linux/macOS - builds all architectures for current platform
./scripts/build.sh

# Windows - builds all architectures for current platform
.\scripts\build.ps1
```

### Platform-Specific Build (All Architectures)
```bash
# Unix/Linux/macOS - Build all architectures for Windows
./scripts/build.sh --target windows

# Windows - Build all architectures for Linux  
.\scripts\build.ps1 -Target linux
```

### Specific Architecture Build
```bash
# Unix/Linux/macOS - Build only amd64 for Windows
./scripts/build.sh --target windows --arch amd64

# Windows - Build only arm64 for Linux
.\scripts\build.ps1 -Target linux -Arch arm64
```

### Release Build (All Platforms)
```bash
# Unix/Linux/macOS
./scripts/build.sh --target all --arch all --clean --test --version "v1.0.0"

# Windows
.\scripts\build.ps1 -Target all -Arch all -Clean -Test -Version "v1.0.0"
```

### Platform-Specific Build
```bash
# Unix/Linux/macOS - Build for Windows
./scripts/build.sh --target windows --arch amd64

# Windows - Build for Linux
.\scripts\build.ps1 -Target linux -Arch amd64
```

## CI/CD Integration

These scripts are designed to be easily integrated into CI/CD pipelines:

```yaml
# GitHub Actions example
- name: Build GoRepos
  run: |
    chmod +x scripts/build.sh
    ./scripts/build.sh --target all --clean --test
```

```yaml
# Azure DevOps example (PowerShell)
- powershell: |
    .\scripts\build.ps1 -Target all -Clean -Test
  displayName: 'Build GoRepos'
```

---

## Testing Framework

The local testing framework for GoRepos is located in the [`local/`](local/) directory.

**Quick Start:**
```powershell
# Build first
.\scripts\build.ps1

# Run basic test
.\scripts\local\grtest1.ps1 -Setup
```

For comprehensive testing documentation, usage examples, and framework details, see [**local/LOCAL.md**](local/LOCAL.md).
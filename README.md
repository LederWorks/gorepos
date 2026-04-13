# GoRepos - Modern Repository Management Tool

A high-performance, parallel repository management tool built in Go.

## Features

- **Parallel Repository Operations** - Clone, update, and check status across hundreds of repositories concurrently
- **YAML-First Configuration** - Hierarchical configuration with recursive includes (local paths and HTTP URLs)
- **External Config Feeding** - Separate configuration repositories for shared, team-wide setups
- **Cross-Platform Support** - Native Windows, macOS, and Linux compatibility
- **Graph-Based Architecture** - In-memory graph of configs, repos, groups, tags, and labels with typed relationships
- **Hierarchical Tags & Labels** - Key-value tags and categorical labels with inheritance from global to repo level
- **Context-Aware Filtering** - Automatically scopes operations to the current working directory subtree

## Installation

### Build from Source

```bash
git clone https://github.com/LederWorks/gorepos.git
cd gorepos
go build -o gorepos cmd/gorepos/main.go
```

Requires Go 1.24+.

### Build Scripts

```bash
# Bash (macOS/Linux) - all platforms
./scripts/build.sh

# Local dev build with content-based versioning
./scripts/build.sh --content-hash --target darwin --arch arm64
```

```powershell
# PowerShell (Windows)
.\scripts\build.ps1

# Local dev build
.\scripts\build.ps1 -ContentHash -Target windows -Arch amd64
```

Build output: `dist/<os>-<arch>/gorepos-<version>/gorepos[.exe]`

## Quick Start

### 1. Initialize User Configuration

```bash
# Interactive setup - creates ~/.gorepos/gorepos.yaml
gorepos setup

# With pre-configured includes
gorepos setup --includes "path/to/team-config.yaml"

# Dry run to preview what would be created
gorepos setup --dry-run
```

### 2. Validate Configuration

```bash
gorepos validate
gorepos validate --config path/to/gorepos.yaml
```

### 3. Repository Operations

```bash
# Show repository status
gorepos status

# Clone missing repositories
gorepos clone

# Update all repositories
gorepos update

# Visualize configuration graph
gorepos graph

# List groups
gorepos groups

# Show repository filesystem hierarchy
gorepos repos
```

## Configuration

### Basic Configuration

```yaml
version: "1.0"

global:
  basePath: "/home/user/repos"
  workers: 10
  timeout: 300s

repositories:
  - name: "my-project"
    path: "org/my-project"
    url: "https://github.com/org/my-project.git"
    branch: "main"
    tags:
      language: "go"
    labels: ["backend", "api"]
```

### Hierarchical Includes

Split configuration across multiple files for team and organizational structure:

```yaml
# gorepos.yaml (entry point)
version: "1.0"

includes:
  - "configs/team-a.yaml"
  - "configs/team-b.yaml"
  - "https://raw.githubusercontent.com/org/shared-config/main/gorepos.yaml"

global:
  basePath: "/workspace"
  tags:
    organization: "my-org"
  labels: ["managed"]
```

Includes are resolved recursively. Circular includes are detected and rejected. Global tags and labels are inherited downward and merged (not replaced) at each level.

### Tags and Labels

**Tags** are key-value metadata (`map[string]interface{}`):
```yaml
tags:
  platform: "github"
  language: "go"
  criticality: "high"
```

**Labels** are categorical strings (`[]string`):
```yaml
labels: ["api", "backend", "production"]
```

Both inherit from global -> config file -> repository level. Repository values supplement parent values.

### Groups

```yaml
groups:
  backend-services: ["api-service", "auth-service", "data-service"]
  all-repos: []  # Empty array = auto-populated from matching labels
```

### Disabling Repositories

```yaml
repositories:
  - name: "archived-project"
    url: "https://github.com/org/archived.git"
    disabled: true  # Kept in config/graph but skipped for git operations
```

## Commands

| Command | Description |
|---------|-------------|
| `setup` | Initialize user configuration with platform-appropriate defaults |
| `validate` | Validate YAML configuration files against the gorepos schema |
| `status` | Show status of all repositories (branch, uncommitted changes) |
| `clone` | Clone missing repositories |
| `update` | Update all repositories (pull latest) |
| `graph` | Display configuration graph with relationships |
| `groups` | List repository groups and their members |
| `repos` | Show repository filesystem hierarchy |

### Global Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--config` | `-c` | Configuration file path | Auto-detected |
| `--parallel` | `-p` | Number of parallel workers | `10` |
| `--verbose` | `-v` | Enable verbose output | `false` |
| `--dry-run` | `-n` | Show what would be done without doing it | `false` |

### Config File Resolution

When `--config` is not specified, gorepos searches in order:
1. `gorepos.yaml` / `.gorepos.yaml` in the current directory
2. `~/.gorepos/gorepos.yaml`
3. `~/.config/gorepos/gorepos.yaml`
4. Platform-specific locations (macOS: `~/Library/Application Support/gorepos/`, Windows: `~/Documents/gorepos/` or OneDrive)

## Architecture

GoRepos processes configuration through a 4-stage pipeline:

1. **Config Loading** - Reads `gorepos.yaml`, recursively resolves `includes:`, detects circular includes, merges global tags/labels down to repo level
2. **Graph Construction** - Converts config into an in-memory graph of typed nodes (`Config`, `Repository`, `Group`, `Tag`, `Label`) connected by typed edges
3. **Context Filtering** - Compares CWD against `basePath`; when inside a managed repo subtree, only that subtree's repos are visible
4. **Command Execution** - Dispatches to command handlers for git operations or display rendering

### Output Example

```
Configuration Tree:
 gorepos.yaml
 +-- configs/lederworks/lederworks.yaml
 |   +-- github/lederworks_github.yaml
 |   |   +-- gorepos-main
 |   |   +-- gorepos-config
 |   |   +-- myrepos
 |   |   o-- myrepos-archive (disabled)
 |   +-- azuredevops/lederworks_ado.yaml
 +-- configs/ledermayer/ledermayer.yaml
     +-- ledermayer-app
     +-- ledermayer-web

Groups:
 +-- lederworks-all (4 repositories)
 +-- ledermayer-all (2 repositories)
```

## External Configuration Repositories

GoRepos is designed for separating the tool from its configuration. Common patterns:

| Repo | Purpose |
|------|---------|
| `gorepos-config` | Educational/example config (ships with gorepos) |
| `gorepos-<org>-gh` | GitHub repos for an organization |
| `gorepos-<org>-ado` | Azure DevOps repos for an organization |

Each config repo is a standalone YAML tree with its own `gorepos.yaml` entry point. The gorepos binary resolves includes at runtime.

## Dependencies

- [cobra](https://github.com/spf13/cobra) - CLI framework
- [yaml.v3](https://gopkg.in/yaml.v3) - YAML parsing
- [validator/v10](https://github.com/go-playground/validator/v10) - Struct-tag validation

## Development

```bash
# Run all tests
go test ./...

# Run tests for a specific package
go test ./pkg/graph/...
go test ./internal/config/...

# Build
go build -o gorepos cmd/gorepos/main.go

# Vet
go vet ./...
```

See [CLAUDE.md](CLAUDE.md) for detailed architecture documentation and conventions.

## License

MIT License - see [LICENSE](LICENSE) file for details.

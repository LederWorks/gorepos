# GoRepos - Modern Repository Management Tool

![GoRepos Logo](https://via.placeholder.com/400x100/2E86AB/FFFFFF?text=GoRepos)

A high-performance, parallel repository management tool built in Go, designed to replace and modernize MyRepos with advanced features and superior performance.

## 🚀 Features

### Core Capabilities
- **Parallel Repository Operations** - Process 500+ repositories concurrently
- **YAML-First Configuration** - Modern, hierarchical configuration system
- **External Config Feeding** - Separate configuration repositories for shared setups
- **Cross-Platform Support** - Native Windows, macOS, and Linux compatibility
- **Graph Database Architecture** - Sophisticated relationship modeling and inheritance
- **Hierarchical Tags & Labels** - Key-value tags and categorical labels with inheritance
- **CLI-First Design** - Comprehensive command-line interface with rich output

### Advanced Features
- **Hierarchical Configuration** - Multi-level includes with circular detection
- **Scope-Aware Inheritance** - Intelligent group and property inheritance
- **Visual Status Indicators** - Clear symbols for enabled/disabled repositories
- **Template System Ready** - Multi-level template and variable inheritance
- **Plugin Architecture** - Extensible relationship modeling for custom operations

## 📦 Installation

### Pre-built Binaries
```bash
# Download latest release (coming soon)
curl -L https://github.com/LederWorks/gorepos/releases/latest/download/gorepos-$(uname -s)-$(uname -m) -o gorepos
chmod +x gorepos
```

### Build from Source
```bash
git clone https://github.com/LederWorks/gorepos.git
cd gorepos
go build -o gorepos cmd/gorepos/main.go
```

## 🛠️ Quick Start

### 1. Initialize Configuration
```bash
# Use external configuration repository
gorepos validate --config https://github.com/LederWorks/gorepos-config/gorepos.yaml

# Or create local configuration
cat > gorepos.yaml << EOF
version: "1.0"
workspace:
  name: "my-repositories"
  base_path: "~/workspace"
repositories:
  - name: "my-project"
    url: "https://github.com/user/my-project.git"
    path: "my-project"
    enabled: true
EOF
```

### 2. Validate Configuration
```bash
gorepos validate --config gorepos.yaml
```

### 3. Repository Operations
```bash
# Show repository status
gorepos status

# Update all repositories
gorepos update

# Clone missing repositories
gorepos clone

# Visualize configuration graph
gorepos graph
```

## 📚 Configuration

### Hierarchical Configuration System
GoRepos supports sophisticated hierarchical configuration with includes:

```yaml
# Main configuration
version: "1.0"
workspace:
  name: "enterprise-repos"
  base_path: "/workspace"

# Include client-specific configurations
includes:
  - "configs/client-a/gorepos.yaml"
  - "configs/client-b/gorepos.yaml"

# Groups for bulk operations
groups:
  all_backends: []  # Auto-includes all repos from hierarchy
  critical_services:
    - "user-service"
    - "payment-service"

repositories:
  - name: "shared-library"
    url: "https://github.com/company/shared-lib.git"
    path: "shared-library"
    enabled: true
```

### External Configuration Repositories
Use separate repositories for shared configurations:

```bash
# Reference external config
gorepos status --config https://raw.githubusercontent.com/LederWorks/gorepos-config/main/gorepos.yaml
```

### Repository Configuration
Each repository can have detailed metadata:

```yaml
# Global configuration with client-level tags
global:
  tags:
    client: "company-name"
    environment: "production"
  labels:
    - "company"
    - "managed"

# Platform-specific configurations
includes:
  - "platforms/github.yaml"  # Adds platform: "github" tag
  - "platforms/gitlab.yaml"  # Adds platform: "gitlab" tag

repositories:
  - name: "backend-api"
    url: "https://github.com/company/backend-api.git"
    path: "backend-api"
    enabled: true
    tags:
      project: "core-services"
      language: "go"
      criticality: "high"
    labels: ["api", "backend", "service"]
```

## 🔧 Commands

### Core Commands
| Command | Description | Example |
|---------|-------------|---------|
| `status` | Show repository status | `gorepos status --dry-run` |
| `update` | Update all repositories | `gorepos update --workers 20` |
| `clone` | Clone missing repositories | `gorepos clone` |
| `validate` | Validate configuration | `gorepos validate` |
| `graph` | Visualize configuration and relationships | `gorepos graph` |
| `groups` | List configured groups | `gorepos groups --verbose` |

### Global Flags
| Flag | Description | Default |
|------|-------------|---------|
| `--config` | Configuration file path | `gorepos.yaml` |
| `--workers` | Number of parallel workers | `10` |
| `--verbose` | Enable verbose output | `false` |
| `--dry-run` | Show what would be done | `false` |

## 🏷️ Tags and Labels

### Hierarchical Organization
GoRepos supports sophisticated hierarchical tagging and labeling:

**Tags** - Key-value pairs for metadata:
- **Client Level**: `client: "company"`, `environment: "production"`
- **Platform Level**: `platform: "github"`, `provider: "aws"`
- **Repository Level**: `project: "api"`, `language: "go"`, `criticality: "high"`

**Labels** - Simple categorical tags:
- **Client Level**: `["company", "managed"]`
- **Platform Level**: `["github", "ci-enabled"]`
- **Repository Level**: `["api", "backend", "microservice"]`

### Tag Inheritance
```yaml
# client.yaml - Client-level tags
global:
  tags:
    client: "acme-corp"
    tier: "enterprise"
  labels: ["acme", "managed"]

# github.yaml - Platform-level tags  
global:
  tags:
    platform: "github"
    ci_system: "github-actions"
  labels: ["github"]

# Final repository inherits:
# - client: "acme-corp" (from client)
# - platform: "github" (from platform)  
# - project: "api" (repository-specific)
```

### Graph Visualization
```bash
gorepos graph
# Shows:
# 🏷️ client = acme-corp (scope: global)
#     Used by: api-service, web-app, database
# 🏷️ platform = github (scope: global)
#     Used by: api-service, web-app
# 🏷️ project = core (scope: repository)
#     Used by: api-service
```

## 📊 Performance

### Parallel Processing
GoRepos processes repositories concurrently for superior performance:

- **MyRepos Sequential**: ~10-15 minutes for 500 repositories
- **GoRepos Parallel**: ~1-3 minutes for 500 repositories (**10x improvement**)

### Configuration Example Output
```
Configuration Tree:
📁 gorepos.yaml ✅
├── 📁 configs/lederworks/gorepos.yaml ✅
│   ├── 📁 configs/lederworks/github/gorepos.yaml ✅
│   │   ├─● gorepos
│   │   ├─● gorepos-config
│   │   ├─● myrepos
│   │   ├─● myrepos-scripts
│   │   └─○ myrepos-archive (disabled)
│   └── 📁 configs/lederworks/azuredevops/gorepos.yaml ✅
└── 📁 configs/ledermayer/gorepos.yaml ✅
    ├─● ledermayer-app
    ├─● ledermayer-web
    └─● ledermayer-docs

Groups Summary:
├─● ledermayer-all (3 repositories)
├─● lederworks-all (5 repositories)
└─● gorepos_managed (2 repositories)
```

## 🏗️ Architecture

### Graph Database System
GoRepos uses a sophisticated graph database architecture:

```go
// Node types in the graph
- Root nodes: Entry points
- Config nodes: Configuration files
- Repository nodes: Git repositories
- Group nodes: Repository collections
- Template nodes: Template definitions

// Relationship types
- parent_child: Hierarchical structure
- includes: Configuration includes
- defines: Entity definitions
- inherits: Property inheritance
- depends_on: Dependencies
- triggers: Event relationships
```

### Hybrid Node Architecture
- **Explicit Nodes**: Configurations and repositories defined in YAML
- **Derived Nodes**: Groups and computed entities with auto-population
- **Metadata Tracking**: Source configuration, inheritance chains, properties
- **Template System**: Multi-level template and variable inheritance

## 🔮 Roadmap

### Phase 1: Core Functionality (95% Complete)
- [x] Parallel repository operations
- [x] YAML configuration system
- [x] Hierarchical includes
- [x] CLI interface
- [x] Graph database architecture
- [x] Visual status indicators
- [x] Group inheritance system
- [x] Cross-platform compatibility
- [ ] Security & credential management

### Phase 2: Advanced Features (Planned)
- [ ] Go template system integration
- [ ] Content management capabilities
- [ ] Plugin system foundation
- [ ] Performance optimization
- [ ] Monitoring and logging

### Phase 3: Enterprise Features (Future)
- [ ] Web interface
- [ ] Vector database integration
- [ ] AI/ML capabilities
- [ ] Role-based access control
- [ ] API integrations

## 📝 Migration from MyRepos

### Configuration Conversion
GoRepos includes tools to migrate from MyRepos `.mrconfig` format:

```bash
# Convert existing .mrconfig to gorepos.yaml
gorepos convert --from-mrconfig ~/.mrconfig --output gorepos.yaml
```

### Command Mapping
| MyRepos | GoRepos | Notes |
|---------|---------|-------|
| `mr status` | `gorepos status` | Parallel execution |
| `mr update` | `gorepos update` | Parallel execution |
| `mr checkout` | `gorepos clone` | Improved UX |
| `mr register` | `gorepos add` | YAML-based |

## 🤝 Contributing

### Development Setup
```bash
# Clone repository
git clone https://github.com/LederWorks/gorepos.git
cd gorepos

# Install dependencies
go mod download

# Run tests
go test ./...

# Build
go build -o gorepos cmd/gorepos/main.go
```

### Code Quality
- Go 1.21+ required
- Follow Go coding standards
- Comprehensive test coverage (>90%)
- Documentation for all public APIs

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🔗 Links

- **Documentation**: [GitHub Wiki](https://github.com/LederWorks/gorepos/wiki)
- **Configuration Examples**: [gorepos-config](https://github.com/LederWorks/gorepos-config)
- **Issue Tracker**: [GitHub Issues](https://github.com/LederWorks/gorepos/issues)
- **Discussions**: [GitHub Discussions](https://github.com/LederWorks/gorepos/discussions)

---

**Built with ❤️ by LederWorks** - Modernizing repository management for the 21st century
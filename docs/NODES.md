# Node System Process Flow Documentation

This document describes how nodes are used throughout the GoRepos system, from configuration loading to display and validation.

## Current Architecture (January 2026)

### Modular Command System
- **Location**: `internal/commands/` package with dedicated command handlers
- **Structure**: Each command has its own focused file following Single Responsibility Principle
- **Commands**: `repos.go`, `validate.go`, `groups.go`, `graph.go`, `status.go`

### Display Package Architecture  
- **Organization**: 5 focused modules for different display strategies
- **Files**:
  - `types.go` - Core display types and interfaces
  - `basic_tree.go` - Simple tree structure display
  - `validation_tree.go` - Configuration validation status display
  - `context_tree.go` - Context-aware filtering logic
  - `groups_tree.go` - File-level groups display functionality

### Config Package Modules
- **Structure**: 8 focused modules with clear separation of concerns
- **Modules**:
  - `types.go` - Core types and constructor (`ConfigLoadResult`, `FileNode`, `Loader`)
  - `loader.go` - Configuration loading with graph integration
  - `validation.go` - Schema validation and business logic validation
  - `merging.go` - Configuration merging and inheritance logic
  - `setup.go` - Setup command and cross-platform path discovery (includes OneDrive support)
  - `display.go` - Tree display and context-aware printing
  - `utils.go` - Helper functions for filtering and tree operations
  - `config.go` - Public API and coordination (entry points)

### Windows Compatibility Features
- **OneDrive Integration**: Smart detection of OneDrive Documents redirection
- **Path Discovery**: Checks both standard and OneDrive Documents locations
- **Cross-Platform**: Platform-specific config path resolution for Windows, macOS, and Linux

## Node Types Overview

### Core Node Types
- **FileNode**: Represents configuration files in the hierarchy
- **Repository**: Individual repository definitions within config files
- **Graph Nodes**: Database representation for analysis and relationships

### Node States
- **Valid/Invalid**: Configuration validation status (✅/❌)
- **Enabled/Disabled**: Repository operational status (●/○)
- **Explicit/Derived**: Origin type (from config vs computed)

## Process Flow: Configuration Loading to Display

```
1. Configuration Discovery
   ├── Scan for config files (gorepos.yaml)
   ├── Follow include chains
   └── Build FileNode hierarchy

2. Configuration Parsing
   ├── YAML parsing per FileNode
   ├── Struct validation (schema)
   ├── Set validation status (✅/❌)
   └── Extract Repository definitions

3. Context Analysis
   ├── Determine current directory context
   ├── Filter repositories by context
   ├── Build contextRepoMap
   └── Calculate scope boundaries

4. Node Filtering (Per Command)
   ├── Status/Update: Show only contextual repos
   ├── Validate: Show all configs in context branch
   ├── Graph: Show analysis of all nodes (NO filtering)
   └── Apply context-aware filtering

5. Display Generation (Command-Specific Strategies)
   ├── Graph Command: Always shows complete analysis
   │   ├── All nodes regardless of context
   │   ├── Complete relationship mapping
   │   └── Full validation status across hierarchy
   ├── Validate Command: Context-branch filtering
   │   ├── Hide unrelated configuration branches
   │   ├── Show all configs within relevant branch
   │   └── Include invalid configs for visibility
   ├── Status/Update Commands: Repository filtering
   │   ├── Filter by current directory context
   │   ├── Show only relevant repositories
   │   └── Apply enabled/disabled filtering
   └── Generate appropriate visualization per strategy
```

## Detailed Node Usage Patterns

### 1. Configuration Loading (`internal/config/config.go`)

**FileNode Creation:**
```go
type FileNode struct {
    Path         string           // Configuration file path
    IsValid      bool            // YAML schema validation result
    Repositories []RepositoryInfo // Repositories defined in this file
    Includes     []FileNode      // Child configuration files
}
```

**Process:**
1. **Discovery**: Scan for `gorepos.yaml` files
2. **Parsing**: Load YAML content into structs
3. **Validation**: Apply schema validation using struct tags
4. **Hierarchy Building**: Create parent-child relationships via includes
5. **Status Assignment**: Mark each node as valid (✅) or invalid (❌)

### 2. Context Filtering (`cmd/gorepos/main.go`)

**Context Determination:**
```go
// Base path context (show everything)
if len(contextRepoNames) >= totalRepos {
    result.PrintConfigTreeWithValidation()
} else {
    // Subdirectory context (filtered view)  
    result.PrintConfigTreeWithValidationContext(contextRepoNames)
}
```

**Filtering Logic:**
- **Base Path**: All nodes visible
- **Subdirectory**: Only nodes with relevant repositories shown
- **Branch Scoping**: Hide unrelated configuration branches

### 3. Command-Specific Node Usage

#### Status Command
- **Purpose**: Show operational repository status
- **Node Filter**: Only repositories in current context
- **Display**: Repository status (Clean/Dirty, Sync status)

#### Validate Command  
- **Purpose**: Show configuration validation results
- **Node Filter**: All configuration files in context branch
- **Display**: Validation status (✅/❌) for each config file
- **Challenge**: Include invalid configs within relevant branches

#### Graph Command
- **Purpose**: Analyze configuration relationships
- **Node Filter**: **NO FILTERING** - Always shows complete analysis
- **Display**: Statistics and relationship mapping for entire hierarchy
- **Behavior**: Intentionally context-independent for debugging/analysis
- **Output**: Always identical regardless of working directory

#### Update Command
- **Purpose**: Synchronize repositories
- **Node Filter**: Only enabled repositories in context
- **Display**: Update progress and results

### 4. Validation Context Filtering

**Problem Solved:**
When running validation from subdirectory, need to:
1. ✅ Hide unrelated branches (e.g., hide `ledermayer` when in `lederworks`)
2. ✅ Show all configs within relevant branch (including invalid ❌)
3. ✅ Maintain proper tree structure and visual indicators

**Implementation:**
```go
// Show node if it has context repositories OR is within context branch
if hasContextRepos || r.isWithinContextBranch(node, contextRepoMap) {
    // Display with validation status
}
```

**Context Branch Logic:**
- Collect all nodes with context repositories
- Check if invalid config shares directory structure with valid configs
- Include if they're in the same configuration branch

## Graph Database Integration

### Node Classification
- **Explicit Nodes**: Directly from configuration files (configs, repositories)
- **Derived Nodes**: Computed from configuration (groups, tags, labels)
- **Config Nodes**: FileNode + Repository (concrete items)
- **Logical Nodes**: Groups + computed relationships (organizational)

### Relationship Types
- **parent_child**: Configuration hierarchy
- **defines**: Config file defines repository
- **includes**: Config file includes other configs
- **tagged_with**: Repository has specific tags
- **labeled_with**: Repository has specific labels

## Visual Representation Patterns

### Tree Display Elements
```
├── config_file.yaml ✅        # Configuration file (valid)
│   ├─● enabled-repo           # Enabled repository  
│   ├─○ disabled-repo          # Disabled repository
│   └── invalid_config.yaml ❌ # Invalid configuration
```

### Context-Aware Display
- **Base Path**: Full tree with all branches
- **Subdirectory**: Filtered tree showing relevant branch only
- **Validation**: All configs in branch, repositories filtered by context

## Key Implementation Files

### Command System (Modular Architecture)
- `internal/commands/repos.go`: Repository filesystem hierarchy display
- `internal/commands/validate.go`: Configuration validation with context filtering
- `internal/commands/groups.go`: Groups command with file-level group display
- `internal/commands/graph.go`: Configuration analysis and relationship mapping
- `internal/commands/status.go`: Repository status checking
- `cmd/gorepos/main.go`: Application entry point and command coordination

### Configuration System (Refactored Modules)
- `internal/config/types.go`: Core types (`ConfigLoadResult`, `FileNode`, `Loader`, `SetupOptions`)
- `internal/config/loader.go`: Configuration loading, graph integration, recursive hierarchy loading
- `internal/config/validation.go`: Schema validation (struct tags) and business logic validation
- `internal/config/merging.go`: Configuration merging, inheritance, and group resolution
- `internal/config/setup.go`: Setup command, cross-platform path discovery, OneDrive support
- `internal/config/display.go`: Tree display methods and context-aware printing
- `internal/config/utils.go`: Filtering utilities, tree operations, context calculations
- `internal/config/config.go`: Public API entry points and coordination

### Display Logic (Modular Components)
- `internal/display/types.go`: Core display types and tree structures
- `internal/display/basic_tree.go`: Simple tree visualization (`printNode`)
- `internal/display/validation_tree.go`: Validation status display (`printNodeWithValidation`)
- `internal/display/context_tree.go`: Context filtering (`printNodeWithValidationContext`)
- `internal/display/groups_tree.go`: File-level group display (`convertToDisplayNodesWithFileGroups`)

### Graph Integration  
- `pkg/graph/builder.go`: Node relationship analysis and graph construction
- `pkg/graph/query.go`: Graph querying and merged configuration retrieval
- `pkg/types/types.go`: Node structure definitions and validation tags

## Architecture Benefits

### Current Design Principles
- **Single Responsibility**: Each file has one clear, focused purpose
- **Maintainability**: Files are kept concise (~200 lines max) and well-organized
- **Clear boundaries**: Distinct separation between loading, validation, display, and setup
- **Testability**: Modular structure enables focused unit testing
- **Cross-platform compatibility**: Smart path resolution for Windows, macOS, and Linux

### Development Advantages
- **Code navigation**: Intuitive file organization makes features easy to find
- **Parallel development**: Multiple developers can work on different modules simultaneously
- **Debugging**: Issues can be isolated to specific modules
- **Feature additions**: New functionality can be added without affecting unrelated code
- **Windows integration**: Seamless OneDrive Documents folder support

## Context Filtering Challenges & Solutions

### Original Problem
```
# From lederworks/ directory - BEFORE
├── lederworks configs ✅
└── ledermayer configs ✅  # Should be hidden!
```

### Solution Applied
```
# From lederworks/ directory - AFTER  
└── lederworks configs ✅
    ├── valid_configs.yaml ✅
    ├── invalid_configs.yaml ❌  # Now properly shown
    └── repos in context
```

**Key Insight**: Context filtering operates at branch level, not individual file level. Within relevant branches, show ALL configurations for complete validation visibility.

## Future Enhancements

### Potential Improvements
1. **Smarter Path Matching**: More sophisticated directory structure analysis
2. **User-Defined Scopes**: Allow custom context definitions beyond directory-based
3. **Performance Optimization**: Cache context calculations for large configurations
4. **Enhanced Validation**: More granular validation status (warnings, errors, etc.)

## Summary

The node system provides a flexible foundation for representing configuration hierarchies with context-aware filtering. The key insight is that different commands require different filtering strategies:

- **Operations** (status, update): Filter by repository context
- **Validation**: Filter by configuration branch context  
- **Analysis** (graph): No filtering for comprehensive view

This allows users to see relevant information based on their current working context while maintaining access to complete validation information within their scope.

# Node System Process Flow Documentation

This document describes how nodes are used throughout the GoRepos system, from configuration loading to display and validation.

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

### Configuration System
- `internal/config/config.go`: FileNode creation and hierarchy management
- `pkg/types/types.go`: Node structure definitions and validation tags

### Display Logic
- `printNodeWithValidation()`: Full display (base path)
- `printNodeWithValidationContext()`: Context-filtered display
- `hasContextRepositories()`: Determines node relevance
- `isWithinContextBranch()`: Includes invalid configs in relevant branches

### Graph Integration  
- `pkg/graph/`: Node relationship analysis
- Graph commands: Statistical analysis and visualization

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

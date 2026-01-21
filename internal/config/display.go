package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PrintConfigTreeWithValidation prints the configuration file hierarchy with validation status
func (r *ConfigLoadResult) PrintConfigTreeWithValidation() {
	if len(r.FileHierarchy) > 0 {
		fmt.Println("Configuration file hierarchy:")
		r.printNodeWithValidation(r.FileHierarchy[0], "", true)
		fmt.Println()
	}
}

// PrintConfigTree prints the configuration file hierarchy as a tree
func (r *ConfigLoadResult) PrintConfigTree() {
	if len(r.FileHierarchy) > 0 {
		fmt.Println("Configuration file hierarchy:")
		r.printNode(r.FileHierarchy[0], "", true)
		fmt.Println()
	}
}

// PrintConfigTreeContext prints the configuration file hierarchy filtered by context repositories
func (r *ConfigLoadResult) PrintConfigTreeContext(contextRepoNames []string) {
	// Create a map for quick lookup of context repository names
	contextRepoMap := make(map[string]bool)
	for _, name := range contextRepoNames {
		contextRepoMap[name] = true
	}

	if len(r.FileHierarchy) > 0 {
		fmt.Printf("Configuration file hierarchy (context: %d repositories):\n", len(contextRepoNames))
		r.printNodeContext(r.FileHierarchy[0], "", true, contextRepoMap)
		fmt.Println()
	}
}

// PrintConfigTreeWithValidationContext prints the configuration file hierarchy with validation status, filtered by context repositories
func (r *ConfigLoadResult) PrintConfigTreeWithValidationContext(contextRepoNames []string) {
	// Create a map for quick lookup of context repository names
	contextRepoMap := make(map[string]bool)
	for _, name := range contextRepoNames {
		contextRepoMap[name] = true
	}

	if len(r.FileHierarchy) > 0 {
		fmt.Printf("Configuration file hierarchy (context: %d repositories):\n", len(contextRepoNames))
		r.printNodeWithValidationContext(r.FileHierarchy[0], "", true, contextRepoMap)
		fmt.Println()
	}
}

// printConfigValidationNode prints config files with validation status, context-aware for invalid configs
func (r *ConfigLoadResult) printConfigValidationNode(node FileNode, prefix string, isLast bool) {
	// Get current working directory for context-aware filtering
	cwd, _ := os.Getwd()

	// Determine if this config should be shown based on context
	showNode := r.isConfigRelevantForValidation(node, cwd)
	if showNode {
		// Print current node with validation status
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		// Get display path (shortened for configs in proper hierarchy)
		displayPath := r.getDisplayPath(node, "", len(prefix) == 0)

		// Add validation status and repository count
		validationSymbol := "✅"
		if !node.IsValid {
			validationSymbol = "❌"
		}

		// Count enabled and total repositories
		totalRepos := len(node.Repositories)
		enabledRepos := 0
		for _, repo := range node.Repositories {
			if !repo.Disabled {
				enabledRepos++
			}
		}

		// Format repository count information
		repoInfo := ""
		if totalRepos > 0 {
			repoInfo = fmt.Sprintf(" (%d repos, %d enabled)", totalRepos, enabledRepos)
		}

		fmt.Printf("%s%s%s %s%s\n", prefix, connector, validationSymbol, displayPath, repoInfo)
	}

	// Print children (only if current node is shown or if child should be shown)
	if len(node.Includes) > 0 {
		childPrefix := prefix
		if showNode {
			if isLast {
				childPrefix += "    "
			} else {
				childPrefix += "│   "
			}
		}

		for i, include := range node.Includes {
			isLastChild := i == len(node.Includes)-1
			r.printConfigValidationNode(include, childPrefix, isLastChild)
		}
	}
}

// isConfigRelevantForValidation determines if a config file should be shown for validation
// Always shows configs within the current context branch, including invalid ones
func (r *ConfigLoadResult) isConfigRelevantForValidation(node FileNode, cwd string) bool {
	// Always show configs if we can't determine context
	if cwd == "" {
		return true
	}

	// Normalize paths
	nodePath := strings.ReplaceAll(node.Path, "\\", "/")
	cwd = strings.ReplaceAll(cwd, "\\", "/")

	// Extract sub-branch from current directory
	// Look for patterns like: /lederworks/github or /ledermayer/github
	cwdParts := strings.Split(cwd, "/")

	// Find current context (client and sub-branch)
	var currentClient, currentSubBranch string
	for i := 0; i < len(cwdParts)-1; i++ {
		if (cwdParts[i] == "lederworks" || cwdParts[i] == "ledermayer") && i+1 < len(cwdParts) {
			currentClient = cwdParts[i]
			if cwdParts[i+1] == "github" || cwdParts[i+1] == "azuredevops" {
				currentSubBranch = cwdParts[i+1]
			}
			break
		}
	}

	// If no specific context, show all configs
	if currentClient == "" {
		return true
	}

	// Find config's client and sub-branch
	nodeParts := strings.Split(nodePath, "/")
	var nodeClient, nodeSubBranch string
	for i := 0; i < len(nodeParts)-1; i++ {
		if (nodeParts[i] == "lederworks" || nodeParts[i] == "ledermayer") && i+1 < len(nodeParts) {
			nodeClient = nodeParts[i]
			if nodeParts[i+1] == "github" || nodeParts[i+1] == "azuredevops" {
				nodeSubBranch = nodeParts[i+1]
			}
			break
		}
	}

	// Different client - hide
	if nodeClient != "" && nodeClient != currentClient {
		return false
	}

	// If node has no sub-branch, it's probably a parent config - show it
	if nodeSubBranch == "" {
		return true
	}

	// If we're in a specific sub-branch context, only show configs from that sub-branch
	if currentSubBranch != "" && nodeSubBranch != currentSubBranch {
		return false
	}

	// Show configs within the current context
	return true
}

// shouldShowFullPath determines if the full path should be shown for a config file
// Returns true only for the user's main config and the base gorepos-config
func (r *ConfigLoadResult) shouldShowFullPath(node FileNode, parentPath string, isTopLevel bool) bool {
	// Always show full path for top-level entry points (user's config)
	if isTopLevel {
		return true
	}

	// Show full path only for the base external config file (gorepos.yaml)
	if filepath.Base(node.Path) == "gorepos.yaml" && !strings.Contains(node.Path, "configs/") {
		return true
	}

	// For all configs under the configs/ directory, use shortened paths
	return false
}

// getDisplayPath returns either the base name or full path based on the relationship
func (r *ConfigLoadResult) getDisplayPath(node FileNode, parentPath string, isTopLevel bool) string {
	if r.shouldShowFullPath(node, parentPath, isTopLevel) {
		return node.Path
	}

	// For config files in the configs directory structure, use ..\filename
	if strings.Contains(node.Path, "configs") {
		return "...\\" + filepath.Base(node.Path)
	}

	// Use standard shortened display for configs in proper parent-child relationship
	nodePath := filepath.Dir(node.Path)
	if parentPath != "" {
		if rel, err := filepath.Rel(parentPath, nodePath); err == nil {
			if rel != "" && rel != "." {
				return "...\\" + rel + "\\" + filepath.Base(node.Path)
			}
		}
	}

	return filepath.Base(node.Path)
}

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// hasContextRepositories checks if a node or its descendants have repositories in context
func (r *ConfigLoadResult) hasContextRepositories(node FileNode, contextRepoMap map[string]bool) bool {
	// Check if this node has any context repositories
	for _, repo := range node.Repositories {
		if contextRepoMap[repo.Name] {
			return true
		}
	}

	// Check if any descendant nodes have context repositories
	for _, include := range node.Includes {
		if r.hasContextRepositories(include, contextRepoMap) {
			return true
		}
	}

	return false
}

// countRepositoriesInNode recursively counts all repositories in a node and its children
func countRepositoriesInNode(node FileNode) int {
	count := len(node.Repositories)
	for _, child := range node.Includes {
		count += countRepositoriesInNode(child)
	}
	return count
}

// printNodeWithValidation recursively prints a file node with validation status
func (r *ConfigLoadResult) printNodeWithValidation(node FileNode, prefix string, isLast bool) {
	r.printNodeWithValidationInternal(node, prefix, isLast, "", len(prefix) == 0, nil)
}

// printNodeWithValidationInternal handles the actual printing logic
func (r *ConfigLoadResult) printNodeWithValidationInternal(node FileNode, prefix string, isLast bool, parentPath string, isTopLevel bool, contextRepoMap map[string]bool) {
	// Print current node with validation status
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	// Get display path (full path if not in proper parent-child relationship)
	displayPath := r.getDisplayPath(node, parentPath, isTopLevel)

	// Add validation status
	validationStatus := ""
	if node.IsValid {
		validationStatus = " ✅"
	} else {
		validationStatus = " ❌"
	}

	fmt.Printf("%s%s%s%s\n", prefix, connector, displayPath, validationStatus)

	// Print repositories defined in this config file
	if len(node.Repositories) > 0 {
		repoPrefix := prefix
		if isLast {
			repoPrefix += "    "
		} else {
			repoPrefix += "│   "
		}

		for i, repo := range node.Repositories {
			isLastRepo := i == len(node.Repositories)-1 && len(node.Includes) == 0

			repoConnector := "├── "
			if isLastRepo {
				repoConnector = "└── "
			}

			statusSymbol := "●"
			if repo.Disabled {
				statusSymbol = "○"
			}

			fmt.Printf("%s%s%s %s\n", repoPrefix, repoConnector, statusSymbol, repo.Name)
		}
	}

	// Print children
	if len(node.Includes) > 0 {
		childPrefix := prefix
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}

		for i, include := range node.Includes {
			isLastChild := i == len(node.Includes)-1
			r.printNodeWithValidationInternal(include, childPrefix, isLastChild, node.Path, false, contextRepoMap)
		}
	}
}

// printNodeWithValidationContext recursively prints a file node with validation status and context filtering
func (r *ConfigLoadResult) printNodeWithValidationContext(node FileNode, prefix string, isLast bool, contextRepoMap map[string]bool) {
	r.printNodeWithValidationContextInternal(node, prefix, isLast, "", len(prefix) == 0, contextRepoMap)
}

// printNodeWithValidationContextInternal handles the actual printing with context
func (r *ConfigLoadResult) printNodeWithValidationContextInternal(node FileNode, prefix string, isLast bool, parentPath string, isTopLevel bool, contextRepoMap map[string]bool) {
	// Filter repositories to only those in context
	contextRepos := []RepositoryInfo{}
	for _, repo := range node.Repositories {
		if contextRepoMap[repo.Name] {
			contextRepos = append(contextRepos, repo)
		}
	}

	// Check if this node or any descendant has context repositories
	hasContextRepos := r.hasContextRepositories(node, contextRepoMap)

	// Also check if this node is within a context branch (for invalid configs)
	isWithinBranch := r.isWithinContextBranch(node, contextRepoMap)

	// Show nodes that have context repos OR are within a context branch
	if hasContextRepos || isWithinBranch {
		// Print current node with validation status
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		// Get display path
		displayPath := r.getDisplayPath(node, parentPath, isTopLevel)

		// Add validation status
		validationStatus := ""
		if node.IsValid {
			validationStatus = " ✅"
		} else {
			validationStatus = " ❌"
		}

		fmt.Printf("%s%s%s%s\n", prefix, connector, displayPath, validationStatus)

		// Print only context repositories in this config file
		if len(contextRepos) > 0 {
			repoPrefix := prefix
			if isLast {
				repoPrefix += "    "
			} else {
				repoPrefix += "│   "
			}

			// Count valid children to determine if repo is last
			validChildCount := 0
			for _, include := range node.Includes {
				if r.hasContextRepositories(include, contextRepoMap) || r.isWithinContextBranch(include, contextRepoMap) {
					validChildCount++
				}
			}

			for i, repo := range contextRepos {
				isLastRepo := i == len(contextRepos)-1 && validChildCount == 0

				repoConnector := "├── "
				if isLastRepo {
					repoConnector = "└── "
				}

				statusSymbol := "●"
				if repo.Disabled {
					statusSymbol = "○"
				}

				fmt.Printf("%s%s%s %s\n", repoPrefix, repoConnector, statusSymbol, repo.Name)
			}
		}

		// Print children that have context repositories
		if len(node.Includes) > 0 {
			childPrefix := prefix
			if isLast {
				childPrefix += "    "
			} else {
				childPrefix += "│   "
			}

			validChildren := []FileNode{}
			for _, include := range node.Includes {
				if r.hasContextRepositories(include, contextRepoMap) || r.isWithinContextBranch(include, contextRepoMap) {
					validChildren = append(validChildren, include)
				}
			}

			for i, include := range validChildren {
				isLastChild := i == len(validChildren)-1
				r.printNodeWithValidationContextInternal(include, childPrefix, isLastChild, node.Path, false, contextRepoMap)
			}
		}
	}
}

// isWithinContextBranch checks if a node is within a branch that has context repositories
// This allows showing invalid config files within relevant branches
func (r *ConfigLoadResult) isWithinContextBranch(node FileNode, contextRepoMap map[string]bool) bool {
	// If the node itself has context repositories, it's definitely in a context branch
	for _, repo := range node.Repositories {
		if contextRepoMap[repo.Name] {
			return true
		}
	}

	// For validation purposes, we need to show invalid configs within relevant branches
	// But also need to respect the current directory context for more granular filtering
	return r.isConfigWithinDirectoryContext(node, contextRepoMap)
}

// isConfigWithinDirectoryContext checks if a config file should be shown based on current directory context
func (r *ConfigLoadResult) isConfigWithinDirectoryContext(node FileNode, contextRepoMap map[string]bool) bool {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}

	// Find all nodes that have context repositories
	contextNodes := []*FileNode{}
	r.collectContextNodes(r.FileHierarchy[0], contextRepoMap, &contextNodes)

	// Check if any context node is within the current directory branch AND
	// the target node is also within the same sub-branch
	for _, contextNode := range contextNodes {
		if r.isNodeWithinDirectoryBranch(*contextNode, cwd) && r.sharesSameBranch(node.Path, contextNode.Path) {
			return true
		}
	}

	return false
}

// isNodeWithinDirectoryBranch checks if a config file is within the current directory branch
func (r *ConfigLoadResult) isNodeWithinDirectoryBranch(node FileNode, cwd string) bool {
	// Normalize paths
	nodePath := strings.ReplaceAll(node.Path, "\\", "/")
	cwd = strings.ReplaceAll(cwd, "\\", "/")

	// Extract the sub-branch from current directory (e.g., "github" from "lederworks/github")
	cwdParts := strings.Split(cwd, "/")

	// Find relevant sub-branch context from current directory
	// Look for patterns like: .../lederworks/github or .../lederworks/azuredevops
	var currentClient, currentSubBranch string
	for i := 0; i < len(cwdParts)-1; i++ {
		if cwdParts[i] == "lederworks" || cwdParts[i] == "ledermayer" {
			currentClient = cwdParts[i]
			if i+1 < len(cwdParts) && (cwdParts[i+1] == "github" || cwdParts[i+1] == "azuredevops") {
				currentSubBranch = cwdParts[i+1]
			}
			break
		}
	}

	// If we couldn't find a specific sub-branch context, allow all within client
	if currentClient == "" {
		return true
	}

	// Find config's client and sub-branch
	nodeParts := strings.Split(nodePath, "/")
	var nodeClient, nodeSubBranch string
	for i := 0; i < len(nodeParts)-1; i++ {
		if nodeParts[i] == "lederworks" || nodeParts[i] == "ledermayer" {
			nodeClient = nodeParts[i]
			if i+1 < len(nodeParts) && (nodeParts[i+1] == "github" || nodeParts[i+1] == "azuredevops") {
				nodeSubBranch = nodeParts[i+1]
			}
			break
		}
	}

	// Must match both client and sub-branch (or be a parent that leads to the sub-branch)
	if nodeClient != currentClient {
		return false
	}

	// If nodeSubBranch is empty, it might be a parent config that includes the sub-branch
	if nodeSubBranch == "" {
		// Allow parent configs for now
		return true
	}

	// Must be within the same sub-branch
	return nodeSubBranch == currentSubBranch
}

// hasContextRepositoryInSameBranch checks if there's a context repository in the same directory branch
func (r *ConfigLoadResult) hasContextRepositoryInSameBranch(root FileNode, targetNode FileNode, contextRepoMap map[string]bool) bool {
	// Find all nodes that have context repositories
	contextNodes := []*FileNode{}
	r.collectContextNodes(root, contextRepoMap, &contextNodes)

	// Check if any context node shares the same directory prefix with target node
	for _, contextNode := range contextNodes {
		if r.sharesSameBranch(targetNode.Path, contextNode.Path) {
			return true
		}
	}

	return false
}

// collectContextNodes recursively collects all nodes that have context repositories
func (r *ConfigLoadResult) collectContextNodes(node FileNode, contextRepoMap map[string]bool, contextNodes *[]*FileNode) {
	// Check if this node has context repositories
	hasContext := false
	for _, repo := range node.Repositories {
		if contextRepoMap[repo.Name] {
			hasContext = true
			break
		}
	}

	if hasContext {
		// Create a copy of the node to avoid pointer issues
		nodeCopy := node
		*contextNodes = append(*contextNodes, &nodeCopy)
	}

	// Recursively check children
	for _, child := range node.Includes {
		r.collectContextNodes(child, contextRepoMap, contextNodes)
	}
}

// sharesSameBranch checks if two file paths are in the same configuration branch
func (r *ConfigLoadResult) sharesSameBranch(path1, path2 string) bool {
	// Normalize paths
	path1 = strings.ReplaceAll(path1, "\\", "/")
	path2 = strings.ReplaceAll(path2, "\\", "/")

	// Extract directory structure to find client/sub-branch patterns
	getBranchInfo := func(path string) (client, subBranch string) {
		parts := strings.Split(path, "/")
		for i := 0; i < len(parts)-1; i++ {
			if parts[i] == "lederworks" || parts[i] == "ledermayer" {
				client = parts[i]
				if i+1 < len(parts) && (parts[i+1] == "github" || parts[i+1] == "azuredevops") {
					subBranch = parts[i+1]
				}
				return
			}
		}
		return
	}

	client1, subBranch1 := getBranchInfo(path1)
	client2, subBranch2 := getBranchInfo(path2)

	// If either doesn't have clear branch info, be more permissive
	if client1 == "" || client2 == "" {
		return true
	}

	// Must have same client
	if client1 != client2 {
		return false
	}

	// If both have sub-branches, they must match
	if subBranch1 != "" && subBranch2 != "" {
		return subBranch1 == subBranch2
	}

	// If one doesn't have sub-branch, it might be a parent - allow
	return true
}

// printNode recursively prints a file node with tree formatting
func (r *ConfigLoadResult) printNode(node FileNode, prefix string, isLast bool) {
	// Print current node
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	displayPath := filepath.Base(node.Path)
	if len(node.Path) > 60 {
		// Show last part of path if too long
		parts := strings.Split(node.Path, string(filepath.Separator))
		if len(parts) > 2 {
			displayPath = "..." + string(filepath.Separator) + strings.Join(parts[len(parts)-2:], string(filepath.Separator))
		}
	}

	fmt.Printf("%s%s%s\n", prefix, connector, displayPath)

	// Print repositories in this config file
	if len(node.Repositories) > 0 {
		repoPrefix := prefix
		if isLast {
			repoPrefix += "    "
		} else {
			repoPrefix += "│   "
		}

		for i, repo := range node.Repositories {
			isLastRepo := i == len(node.Repositories)-1 && len(node.Includes) == 0

			repoConnector := "├── "
			if isLastRepo {
				repoConnector = "└── "
			}

			statusSymbol := "●"
			if repo.Disabled {
				statusSymbol = "○"
			}

			fmt.Printf("%s%s%s %s\n", repoPrefix, repoConnector, statusSymbol, repo.Name)
		}
	}

	// Print children
	if len(node.Includes) > 0 {
		childPrefix := prefix
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}

		for i, include := range node.Includes {
			isLastChild := i == len(node.Includes)-1
			r.printNode(include, childPrefix, isLastChild)
		}
	}
}

// printNodeContext recursively prints a file node with context filtering
func (r *ConfigLoadResult) printNodeContext(node FileNode, prefix string, isLast bool, contextRepoMap map[string]bool) {
	// Check if this node has context repositories or descendant nodes with context repositories
	if !r.hasContextRepositories(node, contextRepoMap) {
		return
	}

	// Print current node
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	displayPath := filepath.Base(node.Path)
	if len(node.Path) > 60 {
		// Show last part of path if too long
		parts := strings.Split(node.Path, string(filepath.Separator))
		if len(parts) > 2 {
			displayPath = "..." + string(filepath.Separator) + strings.Join(parts[len(parts)-2:], string(filepath.Separator))
		}
	}

	fmt.Printf("%s%s%s\n", prefix, connector, displayPath)

	// Print only context repositories in this config file
	contextRepos := []RepositoryInfo{}
	for _, repo := range node.Repositories {
		if contextRepoMap[repo.Name] {
			contextRepos = append(contextRepos, repo)
		}
	}

	if len(contextRepos) > 0 {
		repoPrefix := prefix
		if isLast {
			repoPrefix += "    "
		} else {
			repoPrefix += "│   "
		}

		// Count valid children to determine if repo is last
		validChildCount := 0
		for _, include := range node.Includes {
			if r.hasContextRepositories(include, contextRepoMap) {
				validChildCount++
			}
		}

		for i, repo := range contextRepos {
			isLastRepo := i == len(contextRepos)-1 && validChildCount == 0

			repoConnector := "├── "
			if isLastRepo {
				repoConnector = "└── "
			}

			statusSymbol := "●"
			if repo.Disabled {
				statusSymbol = "○"
			}

			fmt.Printf("%s%s%s %s\n", repoPrefix, repoConnector, statusSymbol, repo.Name)
		}
	}

	// Print children that have context repositories
	if len(node.Includes) > 0 {
		childPrefix := prefix
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}

		validChildren := []FileNode{}
		for _, include := range node.Includes {
			if r.hasContextRepositories(include, contextRepoMap) {
				validChildren = append(validChildren, include)
			}
		}

		for i, include := range validChildren {
			isLastChild := i == len(validChildren)-1
			r.printNodeContext(include, childPrefix, isLastChild, contextRepoMap)
		}
	}
}

package display

import (
	"fmt"
	"path/filepath"
	"strings"
)

// PrintConfigTreeContext prints the configuration file hierarchy filtered by context repositories
func (d *ConfigTreeDisplay) PrintConfigTreeContext(hierarchy []FileNode, contextRepoNames []string) {
	// Create a map for quick lookup of context repository names
	contextRepoMap := make(map[string]bool)
	for _, name := range contextRepoNames {
		contextRepoMap[name] = true
	}

	// Print only the first node as we expect a single root
	if len(hierarchy) > 0 {
		d.printNodeContext(hierarchy[0], "", true, contextRepoMap)
	}
}

// PrintConfigTreeWithValidationContext prints the configuration file hierarchy with validation status, filtered by context repositories
func (d *ConfigTreeDisplay) PrintConfigTreeWithValidationContext(hierarchy []FileNode, contextRepoNames []string) {
	// Create a map for quick lookup of context repository names
	contextRepoMap := make(map[string]bool)
	for _, name := range contextRepoNames {
		contextRepoMap[name] = true
	}

	// Print only the first node as we expect a single root
	if len(hierarchy) > 0 {
		d.printNodeWithValidationContext(hierarchy[0], "", true, contextRepoMap)
	}
}

// printNodeContext recursively prints a file node with context filtering
func (d *ConfigTreeDisplay) printNodeContext(node FileNode, prefix string, isLast bool, contextRepoMap map[string]bool) {
	// Check if this node has context repositories or descendant nodes with context repositories
	if !d.hasContextRepositories(node, contextRepoMap) {
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
			if d.hasContextRepositories(include, contextRepoMap) {
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
			if d.hasContextRepositories(include, contextRepoMap) {
				validChildren = append(validChildren, include)
			}
		}

		for i, include := range validChildren {
			isLastChild := i == len(validChildren)-1
			d.printNodeContext(include, childPrefix, isLastChild, contextRepoMap)
		}
	}
}

// printNodeWithValidationContext recursively prints a file node with validation status and context filtering
func (d *ConfigTreeDisplay) printNodeWithValidationContext(node FileNode, prefix string, isLast bool, contextRepoMap map[string]bool) {
	// Check if this node has context repositories or descendant nodes with context repositories
	if !d.hasContextRepositories(node, contextRepoMap) {
		return
	}

	// Print current node with validation status
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

	// Add validation status
	validationStatus := ""
	if node.IsValid {
		validationStatus = " ✅"
	} else {
		validationStatus = " ❌"
	}

	fmt.Printf("%s%s%s%s\n", prefix, connector, displayPath, validationStatus)

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
			if d.hasContextRepositories(include, contextRepoMap) {
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
			if d.hasContextRepositories(include, contextRepoMap) {
				validChildren = append(validChildren, include)
			}
		}

		for i, include := range validChildren {
			isLastChild := i == len(validChildren)-1
			d.printNodeWithValidationContext(include, childPrefix, isLastChild, contextRepoMap)
		}
	}
}

// hasContextRepositories checks if a node or its descendants have repositories in context
func (d *ConfigTreeDisplay) hasContextRepositories(node FileNode, contextRepoMap map[string]bool) bool {
	// Check if this node has any context repositories
	for _, repo := range node.Repositories {
		if contextRepoMap[repo.Name] {
			return true
		}
	}

	// Check if any descendant nodes have context repositories
	for _, include := range node.Includes {
		if d.hasContextRepositories(include, contextRepoMap) {
			return true
		}
	}

	return false
}

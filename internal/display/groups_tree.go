package display

import (
	"fmt"
	"path/filepath"
	"strings"
)

// PrintConfigTreeWithValidationAndFileGroups prints the configuration file hierarchy with validation status and file-level groups
func (d *ConfigTreeDisplay) PrintConfigTreeWithValidationAndFileGroups(hierarchy []FileNode, contextRepoNames []string) {
	if len(contextRepoNames) > 0 {
		// Create a map for quick lookup of context repository names
		contextRepoMap := make(map[string]bool)
		for _, name := range contextRepoNames {
			contextRepoMap[name] = true
		}

		// Print only the first node as we expect a single root
		if len(hierarchy) > 0 {
			d.printNodeWithValidationAndFileGroups(hierarchy[0], "", true, contextRepoMap)
		}
	} else {
		// Show full tree with validation and file groups
		for i, node := range hierarchy {
			isLast := i == len(hierarchy)-1
			d.printNodeWithValidationAndFileGroups(node, "", isLast, nil)
		}
	}
}

// printNodeWithValidationAndFileGroups recursively prints a file node with validation status and file-level groups
func (d *ConfigTreeDisplay) printNodeWithValidationAndFileGroups(node FileNode, prefix string, isLast bool, contextRepoMap map[string]bool) {
	// Print current node
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	validationSymbol := "✅"
	if !node.IsValid {
		validationSymbol = "❌"
	}

	displayPath := filepath.Base(node.Path)
	if len(node.Path) > 60 {
		// Show last part of path if too long
		parts := strings.Split(node.Path, string(filepath.Separator))
		if len(parts) > 2 {
			displayPath = "..." + string(filepath.Separator) + strings.Join(parts[len(parts)-2:], string(filepath.Separator))
		}
	}

	fmt.Printf("%s%s%s %s\n", prefix, connector, displayPath, validationSymbol)

	// Print groups defined in this file
	if len(node.FileGroups) > 0 {
		groupPrefix := prefix
		if isLast {
			groupPrefix += "    "
		} else {
			groupPrefix += "│   "
		}

		groupNames := make([]string, 0, len(node.FileGroups))
		for groupName := range node.FileGroups {
			groupNames = append(groupNames, groupName)
		}

		for i, groupName := range groupNames {
			repos := node.FileGroups[groupName]
			isLastGroup := i == len(groupNames)-1 && len(node.Repositories) == 0 && len(node.Includes) == 0

			groupConnector := "├── "
			if isLastGroup {
				groupConnector = "└── "
			}

			// Filter repositories in this group by context if needed
			var filteredRepos []string
			if contextRepoMap != nil {
				for _, repo := range repos {
					if contextRepoMap[repo] {
						filteredRepos = append(filteredRepos, repo)
					}
				}
			} else {
				filteredRepos = repos
			}

			if len(filteredRepos) > 0 || contextRepoMap == nil {
				fmt.Printf("%s%s📁 %s (%d repos): [%s]\n", groupPrefix, groupConnector, groupName, len(filteredRepos), strings.Join(filteredRepos, ", "))
			}
		}
	}

	// Print repositories in this config file
	if len(node.Repositories) > 0 {
		repoPrefix := prefix
		if isLast {
			repoPrefix += "    "
		} else {
			repoPrefix += "│   "
		}

		var contextRepos []RepositoryInfo
		if contextRepoMap != nil {
			// Filter repositories by context
			for _, repo := range node.Repositories {
				if contextRepoMap[repo.Name] {
					contextRepos = append(contextRepos, repo)
				}
			}
		} else {
			contextRepos = node.Repositories
		}

		// Calculate how many valid children we'll have for proper tree formatting
		validChildCount := 0
		for _, include := range node.Includes {
			if contextRepoMap == nil || d.hasContextRepositories(include, contextRepoMap) {
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

	// Print children with validation and file groups
	if len(node.Includes) > 0 {
		childPrefix := prefix
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}

		var validChildren []FileNode
		if contextRepoMap != nil {
			// Filter children by context
			for _, include := range node.Includes {
				if d.hasContextRepositories(include, contextRepoMap) {
					validChildren = append(validChildren, include)
				}
			}
		} else {
			validChildren = node.Includes
		}

		for i, include := range validChildren {
			isLastChild := i == len(validChildren)-1
			d.printNodeWithValidationAndFileGroups(include, childPrefix, isLastChild, contextRepoMap)
		}
	}
}

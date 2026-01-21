package display

import (
	"fmt"
	"path/filepath"
	"strings"
)

// PrintConfigTreeWithValidation prints the configuration file hierarchy with validation status
func (d *ConfigTreeDisplay) PrintConfigTreeWithValidation(hierarchy []FileNode) {
	for i, node := range hierarchy {
		isLast := i == len(hierarchy)-1
		d.printNodeWithValidation(node, "", isLast)
	}
}

// printNodeWithValidation recursively prints a file node with validation status
func (d *ConfigTreeDisplay) printNodeWithValidation(node FileNode, prefix string, isLast bool) {
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
			d.printNodeWithValidation(include, childPrefix, isLastChild)
		}
	}
}
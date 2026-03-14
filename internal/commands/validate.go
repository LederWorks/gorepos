package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LederWorks/gorepos/internal/config"
)

// ValidateCommand handles the validation command
type ValidateCommand struct {
	configFile string
	verbose    bool
}

// NewValidateCommand creates a new validate command handler
func NewValidateCommand() *ValidateCommand {
	return &ValidateCommand{}
}

// Execute runs the validate command
func (v *ValidateCommand) Execute(configFile string, verbose bool) error {
	v.configFile = configFile
	v.verbose = verbose

	loader := config.NewLoader()

	// Get config file path
	configPath := configFile
	if configPath == "" {
		var err error
		configPath, err = config.GetConfigPath()
		if err != nil {
			return err
		}
	}

	fmt.Printf("Validating configuration file: %s\n", configPath)
	fmt.Println()

	// Load and validate configuration
	result, err := loader.LoadConfigWithDetails(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Configuration validation failed: %v\n", err)
		return err
	}

	// Show configuration file hierarchy with validation status
	// This focuses purely on config file locations and validation, not repository locations
	fmt.Println("Configuration File Hierarchy:")
	fmt.Println(strings.Repeat("=", 40))
	v.printConfigValidationHierarchy(result.FileHierarchy)

	fmt.Printf("\nℹ️  Use 'gorepos repos' to see repository filesystem hierarchy\n")

	if verbose {
		fmt.Printf("Configuration files validated (%d files):\n", len(result.ProcessedFiles))
		for _, file := range result.ProcessedFiles {
			fmt.Printf("  - %s\n", file)
		}
		fmt.Printf("Schema validation passed for all included files\n")
		fmt.Printf("Configuration logic validation passed\n")
	}
	return nil
}

// printConfigValidationHierarchy prints the configuration file hierarchy
func (v *ValidateCommand) printConfigValidationHierarchy(hierarchy []config.FileNode) {
	for i, node := range hierarchy {
		isLast := i == len(hierarchy)-1
		v.printConfigValidationNode(node, "", isLast)
	}
}

// printConfigValidationNode prints config files with validation status
func (v *ValidateCommand) printConfigValidationNode(node config.FileNode, prefix string, isLast bool) {
	// Print current node with validation status
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	// Status indicator
	status := "✅"
	if !node.IsValid {
		status = "❌"
	}

	// Display with shortened path for config files
	displayPath := v.getShortPath(node.Path)
	fmt.Printf("%s%s%s %s", prefix, connector, status, displayPath)

	// Show repository count if there are repositories
	if len(node.Repositories) > 0 {
		enabledCount := 0
		for _, repo := range node.Repositories {
			if !repo.Disabled {
				enabledCount++
			}
		}
		fmt.Printf(" (%d repos, %d enabled)", len(node.Repositories), enabledCount)
	}

	fmt.Println()

	// Print includes
	if len(node.Includes) > 0 {
		newPrefix := prefix
		if isLast {
			newPrefix += "    "
		} else {
			newPrefix += "│   "
		}

		for i, include := range node.Includes {
			includeIsLast := i == len(node.Includes)-1
			v.printConfigValidationNode(include, newPrefix, includeIsLast)
		}
	}
}

// getShortPath returns a shortened version of the path for config files
func (v *ValidateCommand) getShortPath(fullPath string) string {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fullPath
	}

	// Try to get relative path
	if relPath, err := filepath.Rel(cwd, fullPath); err == nil && !strings.HasPrefix(relPath, "..") {
		return relPath
	}

	// For absolute paths, show "...\\" prefix with last part
	dir := filepath.Dir(fullPath)
	base := filepath.Base(fullPath)

	if len(dir) > 50 {
		return fmt.Sprintf("...\\%s", base)
	}

	return fullPath
}

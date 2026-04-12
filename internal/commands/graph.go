package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LederWorks/gorepos/internal/config"
	"github.com/LederWorks/gorepos/internal/display"
)

// GraphCommand handles the graph command functionality
type GraphCommand struct{}

// NewGraphCommand creates a new graph command handler
func NewGraphCommand() *GraphCommand {
	return &GraphCommand{}
}

// Execute runs the graph command
func (c *GraphCommand) Execute(cfgFile string, verbose bool) error {
	result, err := LoadConfigWithVerbose(cfgFile, verbose)
	if err != nil {
		return err
	}

	// Get the current working directory for context
	cwd, err := os.Getwd()
	var contextRepoNames []string

	if err == nil {
		// Normalize paths for comparison
		basePath := filepath.ToSlash(result.Config.Global.BasePath)
		currentPath := filepath.ToSlash(cwd)

		// Check if we're within the basePath
		if strings.HasPrefix(currentPath, basePath) {
			// Extract the relative path
			relPath := strings.TrimPrefix(currentPath, basePath)
			relPath = strings.TrimPrefix(relPath, "/")

			if relPath != "" {
				// We're in a subdirectory, get context repository names for filtering
				contextRepoNames = GetContextRepositoryNames(result.Config.Repositories, basePath, currentPath)
			}
		}
	}

	fmt.Println("Configuration Dependency Graph:")
	fmt.Println(strings.Repeat("=", 40))

	// Use the display package to show the configuration tree
	display := display.NewConfigTreeDisplay()

	// Convert config FileNode to display FileNode
	displayNodes := c.convertToDisplayNodes(result.FileHierarchy)

	if len(contextRepoNames) > 0 {
		// Show context-filtered tree
		fmt.Printf("Context: %s\n", cwd)
		fmt.Printf("Filtered by %d repositories in current context\n", len(contextRepoNames))
		fmt.Println()

		display.PrintConfigTreeWithValidationContext(displayNodes, contextRepoNames)
	} else {
		// Show full tree with validation
		display.PrintConfigTreeWithValidation(displayNodes)
	}

	if verbose {
		fmt.Printf("\nConfiguration Statistics:\n")
		fmt.Printf("- Total files processed: %d\n", len(result.ProcessedFiles))
		fmt.Printf("- Total repositories: %d\n", len(result.Config.Repositories))
		fmt.Printf("- Total groups: %d\n", len(result.Config.Groups))

		if len(contextRepoNames) > 0 {
			fmt.Printf("- Context repositories: %d\n", len(contextRepoNames))
			fmt.Printf("- Context: %s\n", cwd)
		}
	}

	return nil
}

// convertToDisplayNodes converts config FileNode to display FileNode
func (c *GraphCommand) convertToDisplayNodes(nodes []config.FileNode) []display.FileNode {
	var result []display.FileNode
	for _, node := range nodes {
		displayNode := display.FileNode{
			Path:         node.Path,
			Repositories: c.convertRepositoryInfo(node.Repositories),
			IsValid:      node.IsValid,
			Includes:     c.convertToDisplayNodes(node.Includes),
		}
		result = append(result, displayNode)
	}
	return result
}

// convertRepositoryInfo converts config RepositoryInfo to display RepositoryInfo
func (c *GraphCommand) convertRepositoryInfo(repos []config.RepositoryInfo) []display.RepositoryInfo {
	var result []display.RepositoryInfo
	for _, repo := range repos {
		displayRepo := display.RepositoryInfo{
			Name:     repo.Name,
			Disabled: repo.Disabled,
		}
		result = append(result, displayRepo)
	}
	return result
}

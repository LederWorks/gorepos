package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LederWorks/gorepos/internal/config"
	"github.com/LederWorks/gorepos/internal/display"
	"github.com/LederWorks/gorepos/pkg/types"
	"gopkg.in/yaml.v3"
)

// GroupsCommand handles the groups command functionality
type GroupsCommand struct{}

// NewGroupsCommand creates a new groups command handler
func NewGroupsCommand() *GroupsCommand {
	return &GroupsCommand{}
}

// Execute runs the groups command
func (c *GroupsCommand) Execute(cfgFile string, verbose bool) error {
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

	fmt.Println("Configuration Dependency Graph with Groups:")
	fmt.Println(strings.Repeat("=", 40))

	// Parse groups defined in each file by loading each file individually
	fileGroups, err := c.parseGroupsPerFile(result)
	if err != nil {
		return err
	}

	// Use the display package to show the configuration tree with group information
	display := display.NewConfigTreeDisplay()

	// Convert config FileNode to display FileNode with file-level groups
	displayNodes := c.convertToDisplayNodesWithFileGroups(result.FileHierarchy, fileGroups, result.Config)

	if len(contextRepoNames) > 0 {
		// Show context-filtered tree
		fmt.Printf("Context: %s\n", cwd)
		fmt.Printf("Filtered by %d repositories in current context\n", len(contextRepoNames))
		fmt.Println()

		display.PrintConfigTreeWithValidationAndFileGroups(displayNodes, contextRepoNames)
	} else {
		// Show full tree with validation and groups
		display.PrintConfigTreeWithValidationAndFileGroups(displayNodes, nil)
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

// parseGroupsPerFile loads each configuration file individually to determine which groups are defined in each file
func (c *GroupsCommand) parseGroupsPerFile(result *config.ConfigLoadResult) (map[string]map[string][]string, error) {
	fileGroups := make(map[string]map[string][]string)

	// Parse each processed file individually to extract its groups
	for _, filePath := range result.ProcessedFiles {
		groups, err := c.parseGroupsFromFile(filePath)
		if err != nil {
			// Continue processing other files if one fails
			continue
		}
		if len(groups) > 0 {
			fileGroups[filePath] = groups
		}
	}

	return fileGroups, nil
}

// parseGroupsFromFile reads a single YAML file and extracts only the groups defined in it
func (c *GroupsCommand) parseGroupsFromFile(filePath string) (map[string][]string, error) {
	var config struct {
		Groups map[string][]string `yaml:"groups,omitempty"`
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return config.Groups, nil
}

// convertToDisplayNodesWithFileGroups converts config FileNode to display FileNode with file-level groups
func (c *GroupsCommand) convertToDisplayNodesWithFileGroups(nodes []config.FileNode, fileGroups map[string]map[string][]string, cfg *types.Config) []display.FileNode {
	var result []display.FileNode
	for _, node := range nodes {
		// Get groups defined in this specific file
		groups := fileGroups[node.Path]

		displayNode := display.FileNode{
			Path:         node.Path,
			Repositories: c.convertRepositoryInfo(node.Repositories),
			IsValid:      node.IsValid,
			Includes:     c.convertToDisplayNodesWithFileGroups(node.Includes, fileGroups, cfg),
			FileGroups:   groups,
		}
		result = append(result, displayNode)
	}
	return result
}

// convertRepositoryInfo converts config RepositoryInfo to display RepositoryInfo without group data
func (c *GroupsCommand) convertRepositoryInfo(repos []config.RepositoryInfo) []display.RepositoryInfo {
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

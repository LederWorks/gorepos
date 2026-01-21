package config

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/LederWorks/gorepos/pkg/graph"
	"github.com/LederWorks/gorepos/pkg/types"
	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

// ConfigLoadResult contains configuration and loading metadata
type ConfigLoadResult struct {
	Config         *types.Config
	ProcessedFiles []string
	FileHierarchy  []FileNode
}

// RepositoryInfo tracks repository name and status
type RepositoryInfo struct {
	Name     string
	Disabled bool
}

// FileNode represents a configuration file in the include hierarchy
type FileNode struct {
	Path         string
	Repositories []RepositoryInfo // Repository info with name and enabled/disabled status
	IsValid      bool             // Whether this config file is valid
	Includes     []FileNode
}

// Loader implements the ConfigLoader interface
type Loader struct {
	defaultTimeout time.Duration
	validator      *validator.Validate
}

// NewLoader creates a new configuration loader
func NewLoader() *Loader {
	return &Loader{
		defaultTimeout: 30 * time.Second,
		validator:      validator.New(),
	}
}

// LoadConfig loads configuration from a local file
func (l *Loader) LoadConfig(path string) (*types.Config, error) {
	// Use graph-based loading for scope-aware inheritance
	return l.LoadConfigWithGraph(path)
}

// LoadConfigWithGraph loads configuration using dependency graph for scope-aware inheritance
func (l *Loader) LoadConfigWithGraph(path string) (*types.Config, error) {
	// Build repository graph
	builder := graph.NewGraphBuilder()
	graphQuery, err := builder.BuildGraph(path)
	if err != nil {
		return nil, fmt.Errorf("failed to build repository graph: %w", err)
	}

	// Get merged configuration (inheritance is calculated during build)
	config := graphQuery.GetMergedConfig()

	// Validate configuration
	if err := l.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// LoadConfigLegacy loads configuration using the original flat merging approach (kept for compatibility)
func (l *Loader) LoadConfigLegacy(path string) (*types.Config, error) {
	result, err := l.LoadConfigWithDetails(path)
	if err != nil {
		return nil, err
	}
	return result.Config, nil
}

// LoadConfigWithDetails loads configuration and returns detailed loading information
func (l *Loader) LoadConfigWithDetails(path string) (*ConfigLoadResult, error) {
	if path == "" {
		return nil, fmt.Errorf("configuration path is required")
	}

	processedFiles := make([]string, 0)
	fileHierarchy := make([]FileNode, 0)
	config, rootNode, err := l.loadConfigRecursiveWithHierarchy(path, make(map[string]bool), &processedFiles)
	if err != nil {
		return nil, err
	}

	fileHierarchy = append(fileHierarchy, *rootNode)

	// Final validation only happens at the root level after all includes are processed
	if err := l.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("final configuration validation failed: %w", err)
	}

	// Apply final group inheritance for root-level empty groups after all merging is complete
	l.applyRootGroupInheritance(config)

	return &ConfigLoadResult{
		Config:         config,
		ProcessedFiles: processedFiles,
		FileHierarchy:  fileHierarchy,
	}, nil
}

// loadConfigRecursiveWithHierarchy loads configuration with hierarchy tracking
func (l *Loader) loadConfigRecursiveWithHierarchy(path string, visited map[string]bool, processedFiles *[]string) (*types.Config, *FileNode, error) {
	// Convert to absolute path for cycle detection
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get absolute path for %s: %w", path, err)
	}

	// Check for circular includes
	if visited[absPath] {
		return nil, nil, fmt.Errorf("circular include detected: %s", absPath)
	}
	visited[absPath] = true
	defer delete(visited, absPath)

	// Track this file as processed
	*processedFiles = append(*processedFiles, absPath)

	// Create file node for hierarchy
	node := &FileNode{
		Path:         absPath,
		Repositories: make([]RepositoryInfo, 0),
		IsValid:      false, // Will be set to true only if content is meaningful
		Includes:     make([]FileNode, 0),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var config types.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		node.IsValid = false
		return nil, nil, fmt.Errorf("failed to parse YAML config %s: %w", path, err)
	}

	// Validate the configuration using struct validation tags
	node.IsValid = l.validateConfigStruct(&config)

	// Add repository names to the node for hierarchy display
	for _, repo := range config.Repositories {
		node.Repositories = append(node.Repositories, RepositoryInfo{
			Name:     repo.Name,
			Disabled: repo.Disabled,
		})
	}

	// Process includes
	if len(config.Includes) > 0 {
		baseDir := filepath.Dir(path)
		for _, includePath := range config.Includes {
			// Resolve include path relative to current config file
			var fullIncludePath string
			if filepath.IsAbs(includePath) {
				fullIncludePath = includePath
			} else {
				fullIncludePath = filepath.Join(baseDir, includePath)
			}

			// Load included configuration
			includedConfig, includedNode, err := l.loadConfigRecursiveWithHierarchy(fullIncludePath, visited, processedFiles)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to load included config %s: %w", fullIncludePath, err)
			}

			// Add to hierarchy
			node.Includes = append(node.Includes, *includedNode)

			// Merge included configuration into current config
			config = l.mergeConfigs(&config, includedConfig)
		}
	}

	// Set default values
	l.setDefaults(&config)

	// No validation here - only at the root level
	return &config, node, nil
}

// LoadRemoteConfig loads configuration from a remote URL
func (l *Loader) LoadRemoteConfig(url string) (*types.Config, error) {
	if url == "" {
		return nil, fmt.Errorf("remote configuration URL is required")
	}

	client := &http.Client{
		Timeout: l.defaultTimeout,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remote config from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch remote config: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read remote config response: %w", err)
	}

	var config types.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse remote YAML config: %w", err)
	}

	// Set default values
	l.setDefaults(&config)

	// Validate configuration
	if err := l.ValidateConfig(&config); err != nil {
		return nil, fmt.Errorf("remote configuration validation failed: %w", err)
	}

	return &config, nil
}

// ValidateConfig validates the configuration structure
func (l *Loader) ValidateConfig(config *types.Config) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Validate version
	if config.Version == "" {
		return fmt.Errorf("config version is required")
	}

	// Validate global settings
	if config.Global.Workers < 1 {
		return fmt.Errorf("global.workers must be at least 1")
	}

	if config.Global.Workers > 100 {
		return fmt.Errorf("global.workers cannot exceed 100")
	}

	if config.Global.Timeout < time.Second {
		return fmt.Errorf("global.timeout must be at least 1 second")
	}

	// Validate repositories (only if they exist)
	repoNames := make(map[string]bool)
	for i, repo := range config.Repositories {
		if repo.Name == "" {
			return fmt.Errorf("repository[%d]: name is required", i)
		}

		if repoNames[repo.Name] {
			return fmt.Errorf("repository[%d]: duplicate name '%s'", i, repo.Name)
		}
		repoNames[repo.Name] = true

		if repo.Path == "" {
			return fmt.Errorf("repository[%d] (%s): path is required", i, repo.Name)
		}

		if repo.URL == "" {
			return fmt.Errorf("repository[%d] (%s): URL is required", i, repo.Name)
		}

		// Validate path is absolute or relative to basePath
		if !filepath.IsAbs(repo.Path) && config.Global.BasePath == "" {
			return fmt.Errorf("repository[%d] (%s): relative path requires global.basePath to be set", i, repo.Name)
		}
	}

	return nil
}

// mergeConfigs merges an included configuration into the main configuration
func (l *Loader) mergeConfigs(main *types.Config, included *types.Config) types.Config {
	result := *main // Start with main config

	// Merge global settings (main takes precedence)
	if main.Global.BasePath == "" && included.Global.BasePath != "" {
		result.Global.BasePath = included.Global.BasePath
	}
	if main.Global.Workers == 0 && included.Global.Workers != 0 {
		result.Global.Workers = included.Global.Workers
	}
	if main.Global.Timeout == 0 && included.Global.Timeout != 0 {
		result.Global.Timeout = included.Global.Timeout
	}

	// Merge environment variables
	if result.Global.Environment == nil {
		result.Global.Environment = make(map[string]string)
	}
	for key, value := range included.Global.Environment {
		if _, exists := result.Global.Environment[key]; !exists {
			result.Global.Environment[key] = value
		}
	}

	// Merge repositories (included first, then main to allow overrides)
	repoMap := make(map[string]types.Repository)

	// Add included repositories first
	for _, repo := range included.Repositories {
		repoMap[repo.Name] = repo
	}

	// Add main repositories (overwrites included if same name)
	for _, repo := range main.Repositories {
		repoMap[repo.Name] = repo
	}

	// Convert back to slice
	result.Repositories = make([]types.Repository, 0, len(repoMap))
	for _, repo := range repoMap {
		result.Repositories = append(result.Repositories, repo)
	}

	// Merge groups (no inheritance during merge phase)
	if result.Groups == nil {
		result.Groups = make(map[string][]string)
	}
	for groupName, repos := range included.Groups {
		if _, exists := result.Groups[groupName]; !exists {
			result.Groups[groupName] = repos
		}
	}

	// Merge templates
	if result.Templates == nil {
		result.Templates = make(map[string]interface{})
	}
	for templateName, template := range included.Templates {
		if _, exists := result.Templates[templateName]; !exists {
			result.Templates[templateName] = template
		}
	}

	return result
}

// applyRootGroupInheritance populates all empty groups with all repositories after full merge is complete
func (l *Loader) applyRootGroupInheritance(config *types.Config) {
	if config.Groups == nil {
		return
	}

	// Collect all repository names from the final merged configuration
	allRepoNames := make([]string, 0, len(config.Repositories))
	for _, repo := range config.Repositories {
		allRepoNames = append(allRepoNames, repo.Name)
	}

	// Populate all empty groups with all repository names
	for groupName, repos := range config.Groups {
		if len(repos) == 0 {
			config.Groups[groupName] = append([]string{}, allRepoNames...)
		}
	}
}

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
	if len(r.FileHierarchy) > 0 {
		// Check if we're showing all repositories (i.e., at base path)
		// Count total repositories in configuration
		totalRepos := 0
		r.countRepositoriesInNode(r.FileHierarchy[0], &totalRepos)

		// If context includes all repositories, show full tree
		if len(contextRepoNames) >= totalRepos {
			fmt.Println("Configuration file hierarchy:")
			r.printNode(r.FileHierarchy[0], "", true)
			fmt.Println()
		} else {
			// Create a map for fast lookup
			contextRepoMap := make(map[string]bool)
			for _, repoName := range contextRepoNames {
				contextRepoMap[repoName] = true
			}

			fmt.Println("Configuration file hierarchy:")
			r.printNodeContext(r.FileHierarchy[0], "", true, contextRepoMap)
			fmt.Println()
		}
	}
}

// PrintConfigTreeWithValidationContext prints the configuration file hierarchy with validation status, filtered by context repositories
func (r *ConfigLoadResult) PrintConfigTreeWithValidationContext(contextRepoNames []string) {
	if len(r.FileHierarchy) > 0 {
		// Check if we're showing all repositories (i.e., at base path)
		// Count total repositories in configuration
		totalRepos := 0
		r.countRepositoriesInNode(r.FileHierarchy[0], &totalRepos)

		// If context includes all repositories, show full tree with validation
		if len(contextRepoNames) >= totalRepos {
			fmt.Println("Configuration file hierarchy:")
			r.printNodeWithValidation(r.FileHierarchy[0], "", true)
			fmt.Println()
		} else {
			// Create a map for fast lookup
			contextRepoMap := make(map[string]bool)
			for _, repoName := range contextRepoNames {
				contextRepoMap[repoName] = true
			}

			fmt.Println("Configuration file hierarchy:")
			r.printNodeWithValidationContext(r.FileHierarchy[0], "", true, contextRepoMap)
			fmt.Println()
		}
	}
}

// printConfigValidationNode prints config files with validation status, context-aware for invalid configs
func (r *ConfigLoadResult) printConfigValidationNode(node FileNode, prefix string, isLast bool) {
	// Get current working directory for context-aware filtering
	cwd, err := os.Getwd()

	// Determine if this config should be shown based on context
	shouldShow := true
	if err == nil {
		// Check if we're in a specific context that should filter this config
		shouldShow = r.isConfigRelevantForValidation(node, cwd)
	}

	if shouldShow {
		// Print current config file with validation status
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		// Get display path (full path if not in proper parent-child relationship)
		displayPath := r.getDisplayPath(node, "", len(prefix) == 0)

		// Add validation status
		validationStatus := ""
		if node.IsValid {
			validationStatus = " ✅"
		} else {
			validationStatus = " ❌"
		}

		fmt.Printf("%s%s%s%s\n", prefix, connector, displayPath, validationStatus)

		// Print children
		for i, include := range node.Includes {
			isLastChild := i == len(node.Includes)-1
			childPrefix := prefix
			if isLast {
				childPrefix += "    "
			} else {
				childPrefix += "│   "
			}
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
	cwdNorm := strings.ReplaceAll(cwd, "\\", "/")
	nodePath := strings.ReplaceAll(node.Path, "\\", "/")
	nodeDir := filepath.Dir(nodePath)

	// Extract sub-branch from current directory
	cwdParts := strings.Split(cwdNorm, "/")
	nodeDirParts := strings.Split(nodeDir, "/")

	// Find current context (client and sub-branch)
	var cwdClient, cwdSubBranch string
	for i := 0; i < len(cwdParts)-1; i++ {
		if cwdParts[i] == "lederworks" || cwdParts[i] == "ledermayer" {
			cwdClient = cwdParts[i]
			if i+1 < len(cwdParts) {
				cwdSubBranch = cwdParts[i+1]
			}
			break
		}
	}

	// If no specific context, show all configs
	if cwdClient == "" {
		return true
	}

	// Find config's client and sub-branch
	var nodeClient, nodeSubBranch string
	for i := 0; i < len(nodeDirParts)-1; i++ {
		if nodeDirParts[i] == "configs" && i+1 < len(nodeDirParts) {
			nodeClient = nodeDirParts[i+1]
			if i+2 < len(nodeDirParts) {
				nodeSubBranch = nodeDirParts[i+2]
			}
			break
		}
	}

	// Different client - hide
	if nodeClient != "" && nodeClient != cwdClient {
		return false
	}

	// If node has no sub-branch, it's probably a parent config - show it
	if nodeSubBranch == "" {
		return true
	}

	// If we're in a specific sub-branch context, only show configs from that sub-branch
	if cwdSubBranch != "" && nodeSubBranch != cwdSubBranch {
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
	if strings.Contains(node.Path, "gorepos-config") && !strings.Contains(node.Path, "configs") {
		return true
	}

	// For all configs under the configs/ directory, use shortened paths
	return false
}

// getDisplayPath returns either the base name or full path based on the relationship
func (r *ConfigLoadResult) getDisplayPath(node FileNode, parentPath string, isTopLevel bool) string {
	if r.shouldShowFullPath(node, parentPath, isTopLevel) {
		// Show full path for configs in different filesystem locations
		return node.Path
	}

	// For config files in the configs directory structure, use ...\filename
	if strings.Contains(node.Path, "configs") && !isTopLevel {
		return "...\\" + filepath.Base(node.Path)
	}

	// Use standard shortened display for configs in proper parent-child relationship
	displayPath := filepath.Base(node.Path)
	if len(node.Path) > 60 {
		// Show last part of path if too long
		parts := strings.Split(node.Path, string(filepath.Separator))
		if len(parts) > 2 {
			displayPath = "..." + string(filepath.Separator) + strings.Join(parts[len(parts)-2:], string(filepath.Separator))
		}
	}
	return displayPath
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
	if len(node.Repositories) > 0 && contextRepoMap == nil {
		repoPrefix := prefix
		if isLast {
			repoPrefix += "    "
		} else {
			repoPrefix += "│   "
		}
		for i, repoInfo := range node.Repositories {
			isLastRepo := i == len(node.Repositories)-1 && len(node.Includes) == 0
			repoConnector := "├─"
			if isLastRepo {
				repoConnector = "└─"
			}

			// Use different symbols for enabled/disabled repositories
			repoSymbol := "●" // ● for enabled
			if repoInfo.Disabled {
				repoSymbol = "○" // ○ for disabled
			}

			fmt.Printf("%s%s%s %s\n", repoPrefix, repoConnector, repoSymbol, repoInfo.Name)
		}
	}

	// Print children
	for i, include := range node.Includes {
		isLastChild := i == len(node.Includes)-1
		childPrefix := prefix
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}
		r.printNodeWithValidationInternal(include, childPrefix, isLastChild, node.Path, false, contextRepoMap)
	}
}

// printNodeWithValidationContext recursively prints a file node with validation status and context filtering
func (r *ConfigLoadResult) printNodeWithValidationContext(node FileNode, prefix string, isLast bool, contextRepoMap map[string]bool) {
	r.printNodeWithValidationContextInternal(node, prefix, isLast, "", len(prefix) == 0, contextRepoMap)
}

// printNodeWithValidationContextInternal handles the actual printing with context
func (r *ConfigLoadResult) printNodeWithValidationContextInternal(node FileNode, prefix string, isLast bool, parentPath string, isTopLevel bool, contextRepoMap map[string]bool) {
	// Filter repositories to only those in context
	var contextRepositories []RepositoryInfo
	for _, repoInfo := range node.Repositories {
		if contextRepoMap[repoInfo.Name] {
			contextRepositories = append(contextRepositories, repoInfo)
		}
	}

	// Check if this node or any descendant has context repositories
	hasContextRepos := r.hasContextRepositories(node, contextRepoMap)

	// Also check if this node is within a context branch (for invalid configs)
	inContextBranch := r.isWithinContextBranch(node, contextRepoMap)

	// Show nodes that have context repos OR are within a context branch
	if hasContextRepos || inContextBranch {
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

		// Print repositories in context
		if len(contextRepositories) > 0 {
			repoPrefix := prefix
			if isLast {
				repoPrefix += "    "
			} else {
				repoPrefix += "│   "
			}

			// Count includes that will be shown
			includeCount := 0
			for _, include := range node.Includes {
				if r.hasContextRepositories(include, contextRepoMap) || r.isWithinContextBranch(include, contextRepoMap) {
					includeCount++
				}
			}

			for i, repoInfo := range contextRepositories {
				isLastRepo := i == len(contextRepositories)-1 && includeCount == 0
				repoConnector := "├─"
				if isLastRepo {
					repoConnector = "└─"
				}

				// Use different symbols for enabled/disabled repositories
				repoSymbol := "●" // ● for enabled
				if repoInfo.Disabled {
					repoSymbol = "○" // ○ for disabled
				}

				fmt.Printf("%s%s%s %s\n", repoPrefix, repoConnector, repoSymbol, repoInfo.Name)
			}
		}

		// Process includes - show all includes within context branches (including invalid configs)
		var validIncludes []FileNode
		for _, include := range node.Includes {
			// Show include if it has context repos OR if it's within a context branch
			if r.hasContextRepositories(include, contextRepoMap) || r.isWithinContextBranch(include, contextRepoMap) {
				validIncludes = append(validIncludes, include)
			}
		}

		for i, include := range validIncludes {
			isLastChild := i == len(validIncludes)-1
			childPrefix := prefix
			if isLast {
				childPrefix += "    "
			} else {
				childPrefix += "│   "
			}
			r.printNodeWithValidationContextInternal(include, childPrefix, isLastChild, node.Path, false, contextRepoMap)
		}
	}
}

// isWithinContextBranch checks if a node is within a branch that has context repositories
// This allows showing invalid config files within relevant branches
func (r *ConfigLoadResult) isWithinContextBranch(node FileNode, contextRepoMap map[string]bool) bool {
	// If the node itself has context repositories, it's definitely in a context branch
	if r.hasContextRepositories(node, contextRepoMap) {
		return true
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
		// Fallback to previous behavior if we can't get cwd
		return r.hasContextRepositoryInSameBranch(r.FileHierarchy[0], node, contextRepoMap)
	}

	// Find all nodes that have context repositories
	var contextNodes []FileNode
	r.collectContextNodes(r.FileHierarchy[0], contextRepoMap, &contextNodes)

	// Check if any context node is within the current directory branch AND
	// the target node is also within the same sub-branch
	for _, contextNode := range contextNodes {
		if r.isNodeWithinDirectoryBranch(contextNode, cwd) && r.isNodeWithinDirectoryBranch(node, cwd) {
			return true
		}
	}

	return false
}

// isNodeWithinDirectoryBranch checks if a config file is within the current directory branch
func (r *ConfigLoadResult) isNodeWithinDirectoryBranch(node FileNode, cwd string) bool {
	// Normalize paths
	cwdNorm := strings.ReplaceAll(cwd, "\\", "/")
	nodePath := strings.ReplaceAll(node.Path, "\\", "/")
	nodeDir := filepath.Dir(nodePath)

	// Extract the sub-branch from current directory (e.g., "github" from "lederworks/github")
	cwdParts := strings.Split(cwdNorm, "/")
	nodeDirParts := strings.Split(nodeDir, "/")

	// Find relevant sub-branch context from current directory
	// Look for patterns like: .../lederworks/github or .../lederworks/azuredevops
	var cwdSubBranch string
	var cwdClient string

	for i := 0; i < len(cwdParts)-1; i++ {
		if cwdParts[i] == "lederworks" || cwdParts[i] == "ledermayer" {
			cwdClient = cwdParts[i]
			if i+1 < len(cwdParts) {
				cwdSubBranch = cwdParts[i+1]
			}
			break
		}
	}

	// If we couldn't find a specific sub-branch context, allow all within client
	if cwdSubBranch == "" {
		// Just check if it's the same client
		for _, part := range nodeDirParts {
			if part == cwdClient {
				return true
			}
		}
		return false
	}

	// Check if the node is within the same client and sub-branch
	var nodeClient string
	var nodeSubBranch string

	for i := 0; i < len(nodeDirParts)-1; i++ {
		if nodeDirParts[i] == "configs" && i+1 < len(nodeDirParts) {
			nodeClient = nodeDirParts[i+1]
			if i+2 < len(nodeDirParts) {
				nodeSubBranch = nodeDirParts[i+2]
			}
			break
		}
	}

	// Must match both client and sub-branch (or be a parent that leads to the sub-branch)
	if nodeClient != cwdClient {
		return false
	}

	// If nodeSubBranch is empty, it might be a parent config that includes the sub-branch
	if nodeSubBranch == "" {
		// Check if this is a parent config that would lead to our sub-branch
		// This is more complex - for now, allow parent configs
		return true
	}

	// Must be within the same sub-branch
	return nodeSubBranch == cwdSubBranch
}

// hasContextRepositoryInSameBranch checks if there's a context repository in the same directory branch
func (r *ConfigLoadResult) hasContextRepositoryInSameBranch(root FileNode, targetNode FileNode, contextRepoMap map[string]bool) bool {
	// Find all nodes that have context repositories
	var contextNodes []FileNode
	r.collectContextNodes(root, contextRepoMap, &contextNodes)

	// Check if any context node shares the same directory prefix with target node
	targetPath := targetNode.Path

	for _, contextNode := range contextNodes {
		if r.sharesSameBranch(contextNode.Path, targetPath) {
			return true
		}
	}

	return false
}

// collectContextNodes recursively collects all nodes that have context repositories
func (r *ConfigLoadResult) collectContextNodes(node FileNode, contextRepoMap map[string]bool, contextNodes *[]FileNode) {
	// If this node has context repositories, add it to the list
	// BUT exclude root-level config files that are just including other configs
	// We want to compare only against actual client branch configs
	if r.hasContextRepositories(node, contextRepoMap) {
		// Only include if it's not a root gorepos.yaml file
		if !strings.HasSuffix(node.Path, "gorepos.yaml") || strings.Contains(node.Path, "configs") {
			*contextNodes = append(*contextNodes, node)
		}
	}

	// Recursively check children
	for _, include := range node.Includes {
		r.collectContextNodes(include, contextRepoMap, contextNodes)
	}
}

// sharesSameBranch checks if two file paths are in the same configuration branch
func (r *ConfigLoadResult) sharesSameBranch(contextPath, targetPath string) bool {
	// Normalize paths by removing file names and getting directories
	contextDir := filepath.Dir(contextPath)
	targetDir := filepath.Dir(targetPath)

	// Special case: Check if they are in completely different top-level branches
	// Extract the client/organization part from the path (lederworks vs ledermayer)
	contextParts := strings.Split(contextDir, string(filepath.Separator))
	targetParts := strings.Split(targetDir, string(filepath.Separator))

	// Find the "configs" index to identify the client branch
	contextConfigsIndex := -1
	targetConfigsIndex := -1

	for i, part := range contextParts {
		if part == "configs" {
			contextConfigsIndex = i
			break
		}
	}

	for i, part := range targetParts {
		if part == "configs" {
			targetConfigsIndex = i
			break
		}
	}

	// If both have configs directory and there's a next element (client name)
	if contextConfigsIndex != -1 && targetConfigsIndex != -1 &&
		contextConfigsIndex+1 < len(contextParts) && targetConfigsIndex+1 < len(targetParts) {

		contextClient := contextParts[contextConfigsIndex+1]
		targetClient := targetParts[targetConfigsIndex+1]

		// They're in the same branch only if they have the same client name
		if contextClient == targetClient {
			return true
		} else {
			// Different clients (lederworks vs ledermayer)
			return false
		}
	}

	// Fallback to prefix matching for other cases
	commonPrefixLength := 0
	minLength := len(contextParts)
	if len(targetParts) < minLength {
		minLength = len(targetParts)
	}

	for i := 0; i < minLength; i++ {
		if contextParts[i] == targetParts[i] {
			commonPrefixLength++
		} else {
			break
		}
	}

	// They share the same branch if they have a significant common prefix
	return commonPrefixLength >= 3
}

// printNode recursively prints a file node with tree formatting
func (r *ConfigLoadResult) printNode(node FileNode, prefix string, isLast bool) {
	// Print current node
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	// Get relative path for cleaner display
	displayPath := filepath.Base(node.Path)
	if len(node.Path) > 60 {
		// Show last part of path if too long
		parts := strings.Split(node.Path, string(filepath.Separator))
		if len(parts) > 2 {
			displayPath = "..." + string(filepath.Separator) + strings.Join(parts[len(parts)-2:], string(filepath.Separator))
		}
	}

	fmt.Printf("%s%s%s\n", prefix, connector, displayPath)

	// Print repositories defined in this config file
	if len(node.Repositories) > 0 {
		repoPrefix := prefix
		if isLast {
			repoPrefix += "    "
		} else {
			repoPrefix += "│   "
		}
		for i, repoInfo := range node.Repositories {
			isLastRepo := i == len(node.Repositories)-1 && len(node.Includes) == 0
			repoConnector := "├─"
			if isLastRepo {
				repoConnector = "└─"
			}

			// Use different symbols for enabled/disabled repositories
			repoSymbol := "●" // ● for enabled
			if repoInfo.Disabled {
				repoSymbol = "○" // ○ for disabled
			}

			fmt.Printf("%s%s%s %s\n", repoPrefix, repoConnector, repoSymbol, repoInfo.Name)
		}
	}

	// Print children
	for i, include := range node.Includes {
		isLastChild := i == len(node.Includes)-1
		childPrefix := prefix
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}
		r.printNode(include, childPrefix, isLastChild)
	}
}

// setDefaults sets default values for configuration
func (l *Loader) setDefaults(config *types.Config) {
	// Set default version
	if config.Version == "" {
		config.Version = "1.0"
	}

	// Set default global settings
	if config.Global.Workers == 0 {
		config.Global.Workers = 10
	}

	if config.Global.Timeout == 0 {
		config.Global.Timeout = 5 * time.Minute
	}

	// Set default branch for repositories
	for i := range config.Repositories {
		if config.Repositories[i].Branch == "" {
			config.Repositories[i].Branch = "main"
		}
	}
}

// GetConfigPath attempts to find a configuration file
func GetConfigPath() (string, error) {
	// Check common configuration file names
	candidates := []string{
		"gorepos.yaml",
		"gorepos.yml",
		".gorepos.yaml",
		".gorepos.yml",
	}

	// Check in current directory first
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			abs, err := filepath.Abs(candidate)
			if err != nil {
				return candidate, nil
			}
			return abs, nil
		}
	}

	// Check user configuration directories
	userConfigPaths := getUserConfigPaths()
	for _, configPath := range userConfigPaths {
		for _, candidate := range candidates {
			path := filepath.Join(configPath, candidate)
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}

	// Check in user home directory as fallback
	homeDir, err := os.UserHomeDir()
	if err == nil {
		for _, candidate := range candidates {
			path := filepath.Join(homeDir, candidate)
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf(`no configuration file found

GoRepos looks for configuration files in the following locations:
  1. Current directory: gorepos.yaml, gorepos.yml, .gorepos.yaml, .gorepos.yml
  2. User config directory: %s
  3. Home directory: %s

To get started, run:
  gorepos setup

This will create a user configuration file with appropriate defaults for your platform.

You can also:
  1. Create a gorepos.yaml file manually in any of the above locations
  2. Use --config flag to specify a custom configuration file path
  3. See examples at: https://github.com/LederWorks/gorepos-config`,
		strings.Join(userConfigPaths, ", "), homeDir)
}

// getUserConfigPaths returns platform-appropriate user configuration directories
func getUserConfigPaths() []string {
	var paths []string

	switch runtime.GOOS {
	case "windows":
		// Windows: Try to find actual Documents folder (OneDrive-aware)
		// Try OneDrive Documents first
		if oneDriveDoc := os.Getenv("OneDriveCommercial"); oneDriveDoc != "" {
			paths = append(paths, filepath.Join(oneDriveDoc, "Documents", "gorepos"))
		}
		if oneDrivePersonal := os.Getenv("OneDrive"); oneDrivePersonal != "" {
			paths = append(paths, filepath.Join(oneDrivePersonal, "Documents", "gorepos"))
		}
		// Standard Documents folder
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			paths = append(paths, filepath.Join(userProfile, "Documents", "gorepos"))
		}
		// Fallback: %APPDATA%/gorepos
		if appData := os.Getenv("APPDATA"); appData != "" {
			paths = append(paths, filepath.Join(appData, "gorepos"))
		}
	default:
		// Unix-based systems: ~/.gorepos.d
		if homeDir, err := os.UserHomeDir(); err == nil {
			paths = append(paths, filepath.Join(homeDir, ".gorepos.d"))
		}
		// XDG config directory: ~/.config/gorepos
		if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
			paths = append(paths, filepath.Join(xdgConfig, "gorepos"))
		} else if homeDir, err := os.UserHomeDir(); err == nil {
			paths = append(paths, filepath.Join(homeDir, ".config", "gorepos"))
		}
	}

	return paths
}

// getDefaultUserConfigPath returns the primary user config directory for setup
func getDefaultUserConfigPath() (string, error) {
	paths := getUserConfigPaths()
	if len(paths) == 0 {
		return "", fmt.Errorf("unable to determine user configuration directory")
	}
	return paths[0], nil
}

// SetupOptions contains configuration options for the setup command
type SetupOptions struct {
	ConfigPath string   // Custom config file path
	BasePath   string   // Custom base path for repositories
	Includes   []string // Include files/URLs to add to configuration
	Force      bool     // Overwrite existing config
}

// RunSetup creates a user configuration file
func RunSetup(options *SetupOptions) error {
	if options == nil {
		options = &SetupOptions{}
	}

	fmt.Println("GoRepos Setup")
	fmt.Println("=============")

	var configFile string
	var configDir string

	if options.ConfigPath != "" {
		// Use custom config path
		if !filepath.IsAbs(options.ConfigPath) {
			return fmt.Errorf("custom config path must be absolute: %s", options.ConfigPath)
		}

		configFile = options.ConfigPath
		if filepath.Ext(configFile) == "" {
			configFile = filepath.Join(configFile, "gorepos.yaml")
		}
		configDir = filepath.Dir(configFile)
	} else {
		// Use default config path
		defaultConfigDir, err := getDefaultUserConfigPath()
		if err != nil {
			return fmt.Errorf("failed to determine default config path: %w", err)
		}

		configDir = defaultConfigDir
		fmt.Printf("Configuration will be created in: %s\n", defaultConfigDir)
		configFile = filepath.Join(configDir, "gorepos.yaml")
	}

	// Create config directory
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", configDir, err)
	}

	// Check if config already exists
	if _, err := os.Stat(configFile); err == nil && !options.Force {
		fmt.Printf("Configuration file already exists: %s\n", configFile)
		fmt.Print("Overwrite? (y/N): ")

		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			response := strings.ToLower(strings.TrimSpace(scanner.Text()))
			if response != "y" && response != "yes" {
				fmt.Println("Setup cancelled.")
				return nil
			}
		}
	}

	// Determine appropriate base path
	var basePath string
	if options.BasePath != "" {
		basePath = options.BasePath
	} else {
		var err error
		basePath, err = getDefaultBasePath()
		if err != nil {
			return fmt.Errorf("failed to determine default base path: %w", err)
		}
	}

	// Create user configuration template
	userConfig := createUserConfigTemplate(basePath, options.Includes)

	// Write configuration file
	file, err := os.Create(configFile)
	if err != nil {
		return fmt.Errorf("failed to create config file %s: %w", configFile, err)
	}
	defer file.Close()

	if _, err := file.WriteString(userConfig); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("✅ Configuration file created: %s\n", configFile)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("1. Edit the configuration file to add your repositories")
	fmt.Println("2. Run 'gorepos validate' to check your configuration")
	fmt.Println("3. Run 'gorepos status' to see repository status")
	fmt.Println()
	fmt.Println("For examples and templates, visit:")
	fmt.Println("  https://github.com/LederWorks/gorepos-config")

	return nil
}

// getDefaultBasePath determines an appropriate default base path for repositories
func getDefaultBasePath() (string, error) {
	// Try common development directories
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	candidates := []string{
		filepath.Join(homeDir, "repositories"),
		filepath.Join(homeDir, "repos"),
		filepath.Join(homeDir, "src"),
		filepath.Join(homeDir, "Development"),
		filepath.Join(homeDir, "dev"),
	}

	// On Windows, also check common patterns
	if runtime.GOOS == "windows" {
		if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
			candidates = append(candidates, []string{
				filepath.Join(userProfile, "Documents", "repositories"),
				filepath.Join(userProfile, "Documents", "GitHub"),
				"C:\\repositories",
				"C:\\src",
			}...)
		}
	}

	// Return the first existing directory, or default to ~/repositories
	for _, candidate := range candidates {
		if stat, err := os.Stat(candidate); err == nil && stat.IsDir() {
			return candidate, nil
		}
	}

	// Default fallback
	return filepath.Join(homeDir, "repositories"), nil
}

// createUserConfigTemplate creates a user configuration template
func createUserConfigTemplate(basePath string, includes []string) string {
	includeSection := ""
	if len(includes) > 0 {
		includeSection = "# Include external configurations\nincludes:\n"
		for _, include := range includes {
			// Escape backslashes for Windows paths in YAML
			escapedInclude := strings.ReplaceAll(include, "\\", "\\\\")
			includeSection += fmt.Sprintf("  - \"%s\"\n", escapedInclude)
		}
		includeSection += "\n"
	} else {
		includeSection = "# Include external configurations (optional)\n# includes:\n#   - \"https://raw.githubusercontent.com/LederWorks/gorepos-config/main/gorepos.yaml\"\n\n"
	}

	return fmt.Sprintf(`# GoRepos User Configuration
# Generated by 'gorepos setup'

version: "1.0"

%s# Global settings for this user
global:
  basePath: "%s"
  # workers: 8
  # timeout: 300s
  # tags:
  #   managed: true
  # labels:
  #   - "user-managed"
  # environment:
  #   GIT_CONFIG_GLOBAL: ""
  # credentials:
  #   sshKeyPath: ""
  #   gitCredHelper: ""

# Add your repositories here (optional)
# repositories:
#   - name: "my-project"
#     path: "my-project"
#     url: "https://github.com/user/my-project.git"
#     branch: "main"
#     tags:
#       type: "app"
#     labels: ["personal"]

# Groups for convenient operations (optional)
# groups:
#   personal: ["my-project"]
#   work: []
`, includeSection, strings.ReplaceAll(basePath, "\\", "\\\\"))
}

// printNodeContext recursively prints a file node with context filtering
func (r *ConfigLoadResult) printNodeContext(node FileNode, prefix string, isLast bool, contextRepoMap map[string]bool) {
	// Filter repositories to only those in context
	var contextRepositories []RepositoryInfo
	for _, repoInfo := range node.Repositories {
		if contextRepoMap[repoInfo.Name] {
			contextRepositories = append(contextRepositories, repoInfo)
		}
	}

	// Filter child nodes to only those with repositories in context (recursive check)
	var contextIncludes []FileNode
	for _, include := range node.Includes {
		if r.hasContextRepositories(include, contextRepoMap) {
			contextIncludes = append(contextIncludes, include)
		}
	}

	// Only show this node if it has repositories in context or context includes
	if len(contextRepositories) > 0 || len(contextIncludes) > 0 {
		// Print current node
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		// Get relative path for cleaner display
		displayPath := filepath.Base(node.Path)
		if len(node.Path) > 60 {
			// Show last part of path if too long
			parts := strings.Split(node.Path, string(filepath.Separator))
			if len(parts) > 2 {
				displayPath = "..." + string(filepath.Separator) + strings.Join(parts[len(parts)-2:], string(filepath.Separator))
			}
		}

		fmt.Printf("%s%s%s\n", prefix, connector, displayPath)

		// Print repositories in context
		if len(contextRepositories) > 0 {
			repoPrefix := prefix
			if isLast {
				repoPrefix += "    "
			} else {
				repoPrefix += "│   "
			}
			for i, repoInfo := range contextRepositories {
				isLastRepo := i == len(contextRepositories)-1 && len(contextIncludes) == 0
				repoConnector := "├─"
				if isLastRepo {
					repoConnector = "└─"
				}

				// Use different symbols for enabled/disabled repositories
				repoSymbol := "●" // ● for enabled
				if repoInfo.Disabled {
					repoSymbol = "○" // ○ for disabled
				}

				fmt.Printf("%s%s%s %s\n", repoPrefix, repoConnector, repoSymbol, repoInfo.Name)
			}
		}

		// Print context includes
		for i, include := range contextIncludes {
			isLastChild := i == len(contextIncludes)-1
			childPrefix := prefix
			if isLast {
				childPrefix += "    "
			} else {
				childPrefix += "│   "
			}
			r.printNodeContext(include, childPrefix, isLastChild, contextRepoMap)
		}
	}
}

// hasContextRepositories checks if a node or its descendants have repositories in context
func (r *ConfigLoadResult) hasContextRepositories(node FileNode, contextRepoMap map[string]bool) bool {
	// Check repositories in this node
	for _, repoInfo := range node.Repositories {
		if contextRepoMap[repoInfo.Name] {
			return true
		}
	}

	// Check repositories in child nodes (recursive)
	for _, include := range node.Includes {
		if r.hasContextRepositories(include, contextRepoMap) {
			return true
		}
	}

	return false
}

// validateConfigStruct validates configuration using struct validation tags
func (l *Loader) validateConfigStruct(config *types.Config) bool {
	validate := validator.New()

	// For empty files (zero-value config), they should be considered invalid
	if config.Version == "" && len(config.Repositories) == 0 && len(config.Includes) == 0 &&
		len(config.Groups) == 0 && len(config.Templates) == 0 &&
		(config.Global.BasePath == "" && config.Global.Workers == 0) {
		return false
	}

	// If there's any content, validate it using partial validation rules
	// We create a relaxed version of validation for include files
	err := l.validatePartialConfig(validate, config)
	return err == nil
}

// validatePartialConfig applies relaxed validation rules suitable for include files
func (l *Loader) validatePartialConfig(validate *validator.Validate, config *types.Config) error {
	// Version is required if specified
	if config.Version != "" {
		if err := validate.Var(config.Version, "oneof=1.0"); err != nil {
			return err
		}
	}

	// Validate repositories if present (but don't require them)
	for _, repo := range config.Repositories {
		if err := validate.Struct(&repo); err != nil {
			return err
		}
	}

	// Validate global config if present (with relaxed rules)
	if config.Global.Workers > 0 {
		if err := validate.Var(config.Global.Workers, "min=1,max=100"); err != nil {
			return err
		}
	}

	if config.Global.Timeout != 0 {
		if err := validate.Var(config.Global.Timeout, "min=1s"); err != nil {
			return err
		}
	}

	return nil
}

// countRepositoriesInNode recursively counts all repositories in a node and its children
func (r *ConfigLoadResult) countRepositoriesInNode(node FileNode, count *int) {
	*count += len(node.Repositories)
	for _, include := range node.Includes {
		r.countRepositoriesInNode(include, count)
	}
}

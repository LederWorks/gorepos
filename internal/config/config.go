package config

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
		IsValid:      true, // Assume valid unless validation fails
		Includes:     make([]FileNode, 0),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var config types.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, nil, fmt.Errorf("failed to parse YAML config %s: %w", path, err)
	}

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

	// Validate repositories
	if len(config.Repositories) == 0 {
		return fmt.Errorf("at least one repository must be configured")
	}

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

// printNodeWithValidation recursively prints a file node with validation status
func (r *ConfigLoadResult) printNodeWithValidation(node FileNode, prefix string, isLast bool) {
	// Print current node with validation status
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
		r.printNodeWithValidation(include, childPrefix, isLastChild)
	}
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

	// Check in user home directory
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
  2. Home directory: %s/gorepos.yaml, %s/.gorepos.yaml

To get started:
  1. Create a gorepos.yaml file in your home directory or current directory
  2. Use --config flag to specify a custom configuration file path
  3. See examples at: https://github.com/LederWorks/gorepos-config

Example basic configuration:
---
version: "1.0"
global:
  basePath: "%s"
  workers: 5
  timeout: 300s
repositories:
  - name: "example"
    path: "example"
    url: "https://github.com/user/example.git"
    branch: "main"`, homeDir, homeDir, filepath.Join(homeDir, "repositories"))
}

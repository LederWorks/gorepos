package config

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/LederWorks/gorepos/pkg/graph"
	"github.com/LederWorks/gorepos/pkg/types"
	"gopkg.in/yaml.v3"
)

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
	visited := make(map[string]bool)
	var processedFiles []string
	config, _, err := l.loadConfigRecursiveWithHierarchy(path, visited, &processedFiles)
	return config, err
}

// LoadConfigWithDetails loads configuration and returns detailed loading information
func (l *Loader) LoadConfigWithDetails(path string) (*ConfigLoadResult, error) {
	visited := make(map[string]bool)
	var processedFiles []string
	config, hierarchy, err := l.loadConfigRecursiveWithHierarchy(path, visited, &processedFiles)
	if err != nil {
		return nil, err
	}

	// Create result with hierarchy
	result := &ConfigLoadResult{
		Config:         config,
		ProcessedFiles: processedFiles,
		FileHierarchy:  []FileNode{*hierarchy},
	}

	// Final validation only happens at the root level after all includes are processed
	if err := l.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	// Apply final group inheritance for root-level empty groups after all merging is complete
	l.applyRootGroupInheritance(config)

	// Set default values after loading and merging
	l.setDefaults(config)

	return result, nil
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
		return nil, nil, fmt.Errorf("circular include detected: %s", path)
	}
	visited[absPath] = true
	defer delete(visited, absPath)

	// Track this file as processed
	*processedFiles = append(*processedFiles, absPath)

	// Create file node for hierarchy
	node := &FileNode{
		Path:         absPath,
		Repositories: []RepositoryInfo{},
		IsValid:      true,
		Includes:     []FileNode{},
	}

	// Load main configuration
	data, err := os.ReadFile(path)
	if err != nil {
		// Mark as invalid if file cannot be read
		node.IsValid = false
		return nil, node, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	var config types.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		node.IsValid = false
		return nil, node, fmt.Errorf("failed to parse YAML in %s: %w", path, err)
	}

	// Validate the configuration using struct validation tags
	if err := l.validatePartialConfig(&config); err != nil {
		node.IsValid = false
		// Continue processing but mark as invalid
	}

	// Add repository names to the node for hierarchy display
	for _, repo := range config.Repositories {
		repoInfo := RepositoryInfo{
			Name:     repo.Name,
			Disabled: repo.Disabled,
		}
		node.Repositories = append(node.Repositories, repoInfo)
	}

	// Process includes
	for _, includePath := range config.Includes {
		// Resolve relative paths
		if !filepath.IsAbs(includePath) {
			includePath = filepath.Join(filepath.Dir(path), includePath)
		}

		// Load included config
		includedConfig, includedNode, err := l.loadConfigRecursiveWithHierarchy(includePath, visited, processedFiles)
		if err != nil {
			// Add the invalid node to hierarchy but continue
			node.Includes = append(node.Includes, *includedNode)
			continue
		}

		// Add included node to hierarchy
		node.Includes = append(node.Includes, *includedNode)

		// Merge the included configuration
		config = l.mergeConfigs(&config, includedConfig)
	}

	// Set default values
	l.setDefaults(&config)

	// No validation here - only at the root level
	return &config, node, nil
}

// LoadRemoteConfig loads configuration from a remote URL
func (l *Loader) LoadRemoteConfig(url string) (*types.Config, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remote config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch remote config: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read remote config: %w", err)
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

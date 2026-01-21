package config

import (
	"fmt"

	"github.com/LederWorks/gorepos/pkg/graph"
	"github.com/LederWorks/gorepos/pkg/types"
)

// LoadConfig is the main entry point for loading configuration
func LoadConfig(path string) (*types.Config, error) {
	loader := NewLoader()
	return loader.LoadConfig(path)
}

// LoadConfigWithDetails loads configuration and returns detailed loading information
func LoadConfigWithDetails(path string) (*ConfigLoadResult, error) {
	loader := NewLoader()
	return loader.LoadConfigWithDetails(path)
}

// LoadRemoteConfig loads configuration from a remote URL
func LoadRemoteConfig(url string) (*types.Config, error) {
	loader := NewLoader()
	return loader.LoadRemoteConfig(url)
}

// ValidateConfig validates a configuration
func ValidateConfig(config *types.Config) error {
	loader := NewLoader()
	return loader.ValidateConfig(config)
}

// Legacy methods preserved for compatibility

// LoadConfigWithGraph loads configuration using dependency graph for scope-aware inheritance
func LoadConfigWithGraph(path string) (*types.Config, error) {
	// Build repository graph
	builder := graph.NewGraphBuilder()
	graphQuery, err := builder.BuildGraph(path)
	if err != nil {
		return nil, fmt.Errorf("failed to build repository graph: %w", err)
	}

	// Get merged configuration (inheritance is calculated during build)
	config := graphQuery.GetMergedConfig()

	// Validate configuration
	loader := NewLoader()
	if err := loader.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

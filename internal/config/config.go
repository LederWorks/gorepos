package config

import (
	"context"
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
	// Build repository graph with remote loaders injected to avoid circular imports
	loader := NewLoader()
	builder := graph.NewGraphBuilderWithLoaders(
		func(repoURL, ref, file string) (*types.Config, error) {
			return loader.LoadRemoteConfigViaGit(context.Background(), repoURL, ref, file)
		},
		loader.LoadRemoteConfig,
	)
	graphQuery, err := builder.BuildGraph(path)
	if err != nil {
		return nil, fmt.Errorf("failed to build repository graph: %w", err)
	}

	// Get merged configuration (inheritance is calculated during build)
	config := graphQuery.GetMergedConfig()

	// Apply defaults (workers, timeout, branch, version)
	loader.setDefaults(config)

	// Validate configuration
	if err := loader.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

package config

import (
	"time"

	"github.com/LederWorks/gorepos/pkg/types"
)

// mergeConfigs merges an included configuration into the main configuration
func (l *Loader) mergeConfigs(main *types.Config, included *types.Config) types.Config {
	result := *main // Start with main config

	// Merge global settings (main takes precedence)
	if result.Global.Workers == 0 && included.Global.Workers > 0 {
		result.Global.Workers = included.Global.Workers
	}
	if result.Global.Timeout == 0 && included.Global.Timeout > 0 {
		result.Global.Timeout = included.Global.Timeout
	}
	if result.Global.BasePath == "" && included.Global.BasePath != "" {
		result.Global.BasePath = included.Global.BasePath
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

// setDefaults sets default values for configuration
func (l *Loader) setDefaults(config *types.Config) {
	// Set default global values if not specified
	if config.Global.Workers == 0 {
		config.Global.Workers = 4
	}
	if config.Global.Timeout == 0 {
		config.Global.Timeout = 30 * time.Second
	}

	// Set default branch for repositories if not specified
	for i := range config.Repositories {
		if config.Repositories[i].Branch == "" {
			config.Repositories[i].Branch = "main"
		}
	}

	// Initialize maps if they don't exist
	if config.Groups == nil {
		config.Groups = make(map[string][]string)
	}
	if config.Templates == nil {
		config.Templates = make(map[string]interface{})
	}
	if config.Global.Environment == nil {
		config.Global.Environment = make(map[string]string)
	}
}

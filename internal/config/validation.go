package config

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/LederWorks/gorepos/pkg/types"
)

// ValidateConfig validates the configuration structure
func (l *Loader) ValidateConfig(config *types.Config) error {
	if config == nil {
		return fmt.Errorf("configuration is nil")
	}

	// Validate version
	if config.Version != "" && config.Version != "1.0" {
		return fmt.Errorf("unsupported configuration version: %s", config.Version)
	}

	// Validate global settings
	if config.Global.Workers < 0 {
		return fmt.Errorf("workers must be non-negative")
	}
	if config.Global.Workers > 100 {
		return fmt.Errorf("workers must be less than or equal to 100")
	}
	if config.Global.Timeout < 0 {
		return fmt.Errorf("timeout must be non-negative")
	}

	// Validate repositories (only if they exist)
	if len(config.Repositories) > 0 {
		repoNames := make(map[string]bool)
		for i, repo := range config.Repositories {
			// Check for duplicate repository names
			if repoNames[repo.Name] {
				return fmt.Errorf("duplicate repository name: %s", repo.Name)
			}
			repoNames[repo.Name] = true

			// Validate repository fields
			if strings.TrimSpace(repo.Name) == "" {
				return fmt.Errorf("repository[%d]: name cannot be empty", i)
			}
			if strings.TrimSpace(repo.Path) == "" {
				return fmt.Errorf("repository[%d]: path cannot be empty", i)
			}
			if strings.TrimSpace(repo.URL) == "" {
				return fmt.Errorf("repository[%d]: URL cannot be empty", i)
			}

			// Validate URL format
			if _, err := url.Parse(repo.URL); err != nil {
				return fmt.Errorf("repository[%d]: invalid URL format: %w", i, err)
			}
		}
	}

	return nil
}

// validateConfigStruct validates configuration using struct validation tags
func (l *Loader) validateConfigStruct(config *types.Config) error {
	if err := l.validator.Struct(config); err != nil {
		return fmt.Errorf("configuration structure validation failed: %w", err)
	}
	return nil
}

// validatePartialConfig applies relaxed validation rules suitable for include files
func (l *Loader) validatePartialConfig(config *types.Config) error {
	// For include files, we only validate what's present
	// Don't require all fields to be present

	// Validate version if present
	if config.Version != "" && config.Version != "1.0" {
		return fmt.Errorf("unsupported configuration version: %s", config.Version)
	}

	// Validate repositories if present
	if len(config.Repositories) > 0 {
		repoNames := make(map[string]bool)
		for i, repo := range config.Repositories {
			// Check for duplicate repository names
			if repoNames[repo.Name] {
				return fmt.Errorf("duplicate repository name: %s", repo.Name)
			}
			repoNames[repo.Name] = true

			// Basic field validation
			if strings.TrimSpace(repo.Name) == "" {
				return fmt.Errorf("repository[%d]: name cannot be empty", i)
			}
			if strings.TrimSpace(repo.Path) == "" {
				return fmt.Errorf("repository[%d]: path cannot be empty", i)
			}
			if strings.TrimSpace(repo.URL) == "" {
				return fmt.Errorf("repository[%d]: URL cannot be empty", i)
			}

			// Validate URL format
			if _, err := url.Parse(repo.URL); err != nil {
				return fmt.Errorf("repository[%d]: invalid URL format: %w", i, err)
			}
		}
	}

	return nil
}

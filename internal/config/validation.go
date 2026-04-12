package config

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/LederWorks/gorepos/pkg/types"
)

// ValidateConfig validates the configuration structure
func (l *Loader) ValidateConfig(config *types.Config) error {
	if config == nil {
		return fmt.Errorf("configuration is nil")
	}

	// Validate version (required)
	if config.Version == "" {
		return fmt.Errorf("configuration version is required")
	}
	if config.Version != "1.0" {
		return fmt.Errorf("unsupported configuration version: %s", config.Version)
	}

	// Validate global settings
	if config.Global.Workers < 1 {
		return fmt.Errorf("workers must be at least 1")
	}
	if config.Global.Workers > 100 {
		return fmt.Errorf("workers must be less than or equal to 100")
	}
	if config.Global.Timeout < time.Second {
		return fmt.Errorf("timeout must be at least 1s")
	}

	// Validate repositories (at least one repository or include required)
	if len(config.Repositories) == 0 && len(config.Includes) == 0 {
		return fmt.Errorf("at least one repository or include is required")
	}

	// Validate custom platform entries
	validPlatformTypes := map[string]bool{"github": true, "gitlab": true, "azure": true, "bitbucket": true}
	for i, p := range config.Global.Platforms {
		if strings.TrimSpace(p.Hostname) == "" {
			return fmt.Errorf("global.platforms[%d]: hostname cannot be empty", i)
		}
		if !validPlatformTypes[strings.ToLower(p.Type)] {
			return fmt.Errorf("global.platforms[%d]: type %q is not valid (must be one of: github, gitlab, azure, bitbucket)", i, p.Type)
		}
	}

	// Validate include entries
	for i, inc := range config.Includes {
		if inc.Repo != "" && inc.Path != "" {
			return fmt.Errorf("include[%d]: 'repo' and 'path' are mutually exclusive", i)
		}
		if inc.Repo == "" && inc.Path == "" {
			return fmt.Errorf("include[%d]: must have either 'path' or 'repo'", i)
		}
		if inc.Repo == "" && (inc.Ref != "" || inc.File != "") {
			return fmt.Errorf("include[%d]: 'ref' and 'file' require 'repo' to be set", i)
		}
		// user/email only valid on remote repo includes
		if inc.Repo == "" && (inc.User != "" || inc.Email != "") {
			return fmt.Errorf("include[%d]: 'user' and 'email' require 'repo' to be set (local includes inherit from global credentials)", i)
		}
	}

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

		// Relative paths require basePath
		if !filepath.IsAbs(repo.Path) && config.Global.BasePath == "" {
			return fmt.Errorf("repository[%d]: relative path %q requires global basePath to be set", i, repo.Path)
		}
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

// CollectIdentityWarnings returns non-fatal warnings about missing git identity
// for remote repo includes. Callers can decide how to display them.
func CollectIdentityWarnings(config *types.Config) []string {
	var warnings []string
	hasGlobalIdentity := config.Global.Credentials != nil &&
		(config.Global.Credentials.GitUserName != "" || config.Global.Credentials.GitUserEmail != "")
	for i, inc := range config.Includes {
		if inc.IsRemoteRepo() && inc.User == "" && inc.Email == "" && !hasGlobalIdentity {
			warnings = append(warnings, fmt.Sprintf("include[%d] (%s) has no git identity configured — repos will use system git config", i, inc.Repo))
		}
	}
	return warnings
}

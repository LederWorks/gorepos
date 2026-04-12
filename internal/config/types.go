package config

import (
	"time"

	"github.com/LederWorks/gorepos/pkg/types"
	"github.com/go-playground/validator/v10"
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

// SetupOptions contains options for the setup command
type SetupOptions struct {
	Force    bool
	DryRun   bool
	Path     string   // Custom path for the configuration file
	BasePath string
	Includes []string // Include paths or URLs to embed in the configuration
	Workers  int
	Timeout  time.Duration
	User     string // Git user.name for remote repo includes (non-interactive mode)
	Email    string // Git user.email for remote repo includes (non-interactive mode)
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

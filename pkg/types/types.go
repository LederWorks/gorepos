package types

import (
	"context"
	"time"
)

// Repository represents a single repository configuration
type Repository struct {
	Name        string                 `yaml:"name" validate:"required,min=1"`
	Path        string                 `yaml:"path" validate:"required,min=1"`
	URL         string                 `yaml:"url" validate:"required,url"`
	Branch      string                 `yaml:"branch,omitempty"`
	Commands    map[string]string      `yaml:"commands,omitempty"`
	Environment map[string]string      `yaml:"environment,omitempty"`
	Tags        map[string]interface{} `yaml:"tags,omitempty"`   // Key-value pairs
	Labels      []string               `yaml:"labels,omitempty"` // Simple labels
	Disabled    bool                   `yaml:"disabled,omitempty"`
}

// Config represents the complete configuration structure
type Config struct {
	Version      string                 `yaml:"version" validate:"required,oneof=1.0"`
	Includes     []string               `yaml:"includes,omitempty"`
	Global       GlobalConfig           `yaml:"global" validate:"required"`
	Repositories []Repository           `yaml:"repositories" validate:"required,min=1,dive"`
	Groups       map[string][]string    `yaml:"groups,omitempty"`
	Templates    map[string]interface{} `yaml:"templates,omitempty"`
}

// GlobalConfig contains global settings
type GlobalConfig struct {
	BasePath    string                 `yaml:"basePath"`
	Workers     int                    `yaml:"workers" validate:"min=1,max=100"`
	Timeout     time.Duration          `yaml:"timeout" validate:"min=1s"`
	Environment map[string]string      `yaml:"environment,omitempty"`
	Tags        map[string]interface{} `yaml:"tags,omitempty"`   // Global key-value tags
	Labels      []string               `yaml:"labels,omitempty"` // Global simple labels
	Credentials CredentialConfig       `yaml:"credentials,omitempty"`
}

// CredentialConfig handles credential management
type CredentialConfig struct {
	SSHKeyPath    string `yaml:"sshKeyPath,omitempty"`
	GitCredHelper string `yaml:"gitCredHelper,omitempty"`
	TokenEnvVar   string `yaml:"tokenEnvVar,omitempty"`
}

// Operation represents a repository operation
type Operation struct {
	Repository *Repository
	Command    string
	Args       []string
	Context    context.Context
}

// Result represents the result of a repository operation
type Result struct {
	Repository *Repository
	Operation  string
	Success    bool
	Output     string
	Error      error
	Duration   time.Duration
	StartTime  time.Time
}

// RepositoryManager interface for repository operations
type RepositoryManager interface {
	Clone(ctx context.Context, repo *Repository) error
	Update(ctx context.Context, repo *Repository) error
	Status(ctx context.Context, repo *Repository) (*RepoStatus, error)
	Execute(ctx context.Context, repo *Repository, command string, args ...string) (*Result, error)
	Exists(repo *Repository) bool
}

// RepoStatus represents repository status information
type RepoStatus struct {
	Path             string
	CurrentBranch    string
	IsClean          bool
	UncommittedFiles []string
	AheadBehind      *BranchComparison
}

// BranchComparison shows commits ahead/behind upstream
type BranchComparison struct {
	Ahead  int
	Behind int
}

// Executor interface for parallel operation execution
type Executor interface {
	Execute(ctx context.Context, operations []Operation) <-chan Result
	SetWorkerCount(count int)
	Shutdown(ctx context.Context) error
}

// ConfigLoader interface for configuration loading
type ConfigLoader interface {
	LoadConfig(path string) (*Config, error)
	LoadRemoteConfig(url string) (*Config, error)
	ValidateConfig(config *Config) error
}

package types

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
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
	User        string                 `yaml:"user,omitempty"`   // git user.name set locally after clone
	Email       string                 `yaml:"email,omitempty"`  // git user.email set locally after clone
}

// IncludeEntry represents a single include source — either a simple path/URL string
// or a structured repository reference with ref and file.
type IncludeEntry struct {
	// Path is used for simple string includes (local path or raw URL)
	Path string `yaml:"path,omitempty"`
	// Repo is a git hosting platform repository URL (GitHub, Azure DevOps, GitLab, Bitbucket)
	Repo string `yaml:"repo,omitempty"`
	// Ref is a git ref: branch name, tag, or commit hash (default: repo default branch)
	Ref string `yaml:"ref,omitempty"`
	// File is the path to the config file within the repo (default: "gorepos.yaml")
	File string `yaml:"file,omitempty"`
	// User is the git user.name applied to repos from this config source (remote repo includes only)
	User string `yaml:"user,omitempty"`
	// Email is the git user.email applied to repos from this config source (remote repo includes only)
	Email string `yaml:"email,omitempty"`
}

// UnmarshalYAML handles both plain string and structured mapping forms.
func (e *IncludeEntry) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		e.Path = value.Value
		return nil
	}
	if value.Kind == yaml.MappingNode {
		// Use alias to avoid infinite recursion
		type alias IncludeEntry
		var a alias
		if err := value.Decode(&a); err != nil {
			return err
		}
		*e = IncludeEntry(a)
		return nil
	}
	return fmt.Errorf("include entry must be a string or mapping, got kind %d", value.Kind)
}

// MarshalYAML emits a plain string for simple path entries, or a mapping for structured entries.
func (e IncludeEntry) MarshalYAML() (interface{}, error) {
	if e.Repo == "" && e.Ref == "" && e.File == "" && e.Path != "" {
		return e.Path, nil
	}
	// Use alias to avoid infinite recursion
	type alias IncludeEntry
	return alias(e), nil
}

// IsLocal returns true if this is a local file path include.
func (e *IncludeEntry) IsLocal() bool {
	if e.Repo != "" {
		return false
	}
	return e.Path != "" && !strings.HasPrefix(e.Path, "http://") && !strings.HasPrefix(e.Path, "https://")
}

// IsRemoteRepo returns true if this is a structured repo include.
func (e *IncludeEntry) IsRemoteRepo() bool {
	return e.Repo != ""
}

// IsRawURL returns true if this is a plain HTTP URL include (legacy behavior).
func (e *IncludeEntry) IsRawURL() bool {
	return e.Repo == "" && e.Path != "" && (strings.HasPrefix(e.Path, "http://") || strings.HasPrefix(e.Path, "https://"))
}

// GetFile returns the file path to fetch, defaulting to "gorepos.yaml".
func (e *IncludeEntry) GetFile() string {
	if e.File != "" {
		return e.File
	}
	return "gorepos.yaml"
}

// String returns a human-readable representation for display/logging.
func (e *IncludeEntry) String() string {
	if e.Repo != "" {
		s := e.Repo
		if e.Ref != "" {
			s += " (ref: " + e.Ref + ")"
		}
		if e.File != "" {
			s += " [" + e.File + "]"
		}
		if e.User != "" || e.Email != "" {
			identity := ""
			if e.User != "" {
				identity = e.User
			}
			if e.Email != "" {
				if identity != "" {
					identity += " <" + e.Email + ">"
				} else {
					identity = "<" + e.Email + ">"
				}
			}
			s += " (identity: " + identity + ")"
		}
		return s
	}
	return e.Path
}

// Config represents the complete configuration structure
type Config struct {
	Version      string                 `yaml:"version" validate:"omitempty,oneof=1.0"`
	Includes     []IncludeEntry         `yaml:"includes,omitempty"`
	Global       GlobalConfig           `yaml:"global,omitempty"`
	Repositories []Repository           `yaml:"repositories,omitempty" validate:"dive"`
	Groups       map[string][]string    `yaml:"groups,omitempty"`
	Templates    map[string]interface{} `yaml:"templates,omitempty"`
}

// GlobalConfig contains global settings
type GlobalConfig struct {
	BasePath    string                 `yaml:"basePath,omitempty"`
	Workers     int                    `yaml:"workers,omitempty" validate:"omitempty,min=1,max=100"`
	Timeout     time.Duration          `yaml:"timeout,omitempty" validate:"omitempty,min=1s"`
	Environment map[string]string      `yaml:"environment,omitempty"`
	Tags        map[string]interface{} `yaml:"tags,omitempty"`   // Global key-value tags
	Labels      []string               `yaml:"labels,omitempty"` // Global simple labels
	Credentials *CredentialConfig      `yaml:"credentials,omitempty"`
	Platforms   []PlatformEntry        `yaml:"platforms,omitempty"` // Custom/self-hosted git platform registrations
}

// PlatformEntry registers a custom or self-hosted git hosting platform so that
// gorepos can construct raw-content URLs for it. Use this to support self-hosted
// GitLab, Gitea, Forgejo, or on-premise Azure DevOps instances.
// Hostname is matched case-insensitively against the host portion of include repo URLs.
// Type must be one of: "github", "gitlab", "azure", "bitbucket".
type PlatformEntry struct {
	Hostname string `yaml:"hostname"` // e.g. "gitlab.mycompany.com"
	Type     string `yaml:"type"`     // "github" | "gitlab" | "azure" | "bitbucket"
}

// CredentialConfig handles credential management
type CredentialConfig struct {
	SSHKeyPath    string `yaml:"sshKeyPath,omitempty"`
	GitCredHelper string `yaml:"gitCredHelper,omitempty"`
	TokenEnvVar   string `yaml:"tokenEnvVar,omitempty"`
	GitUserName   string `yaml:"gitUserName,omitempty"`  // default user.name for local repo git config
	GitUserEmail  string `yaml:"gitUserEmail,omitempty"` // default user.email for local repo git config
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
	StatusData *RepoStatus // populated only for "status" operations
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

// ApplyIncludeIdentity stamps include-level User/Email onto repos that lack their own.
// Preserves the precedence: repo.User > include.User > global credentials.
func (c *Config) ApplyIncludeIdentity(user, email string) {
	if user == "" && email == "" {
		return
	}
	for i := range c.Repositories {
		if user != "" && c.Repositories[i].User == "" {
			c.Repositories[i].User = user
		}
		if email != "" && c.Repositories[i].Email == "" {
			c.Repositories[i].Email = email
		}
	}
}

// ConfigLoader interface for configuration loading
type ConfigLoader interface {
	LoadConfig(path string) (*Config, error)
	LoadRemoteConfig(url string) (*Config, error)
	ValidateConfig(config *Config) error
}

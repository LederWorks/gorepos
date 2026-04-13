package config

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/LederWorks/gorepos/pkg/types"
)

// BlockedEnvKeys is the set of environment variable names that are never permitted
// in repo.Environment because they can be used to hijack git subprocess execution.
// The manager's buildEnvironment filters these at runtime; ValidateConfig rejects
// them at load time so users get an early, actionable error.
// Keys are stored upper-cased; callers must upper-case before lookup.
var BlockedEnvKeys = map[string]struct{}{
	"GIT_SSH_COMMAND":       {},
	"GIT_SSH":               {},
	"GIT_PROXY_COMMAND":     {},
	"GIT_EXEC_PATH":         {},
	"GIT_ASKPASS":           {},
	"GIT_TEMPLATE_DIR":      {},
	"LD_PRELOAD":            {},
	"DYLD_INSERT_LIBRARIES": {},
	// PATH can redirect git execution to a malicious binary; always inherit from the OS.
	"PATH": {},
	// GIT_CONFIG_GLOBAL allows overriding the global git config with an attacker-controlled file.
	"GIT_CONFIG_GLOBAL": {},
}

// isAllowedRepoURL returns nil if rawURL is an acceptable repository URL, or an
// error describing why it is not. Accepted forms: SCP git@ syntax, https://, ssh://.
func isAllowedRepoURL(rawURL string) error {
	// SCP-syntax git@github.com:org/repo.git is universally accepted.
	if strings.HasPrefix(rawURL, "git@") {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "https", "ssh":
		return nil
	case "http":
		return fmt.Errorf("http:// URLs are not permitted; use https://")
	default:
		return fmt.Errorf("URL scheme %q is not permitted; use https://, ssh://, or git@host:org/repo syntax", u.Scheme)
	}
}

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

	// Validate global git identity (SEC-H1)
	if config.Global.Credentials != nil {
		creds := config.Global.Credentials
		// Reject not-yet-implemented credential fields (H-4/SEC-M3) so users
		// are not silently left with a broken auth configuration.
		if creds.SSHKeyPath != "" {
			return fmt.Errorf("global credentials: sshKeyPath is not yet implemented; configure SSH via the GIT_SSH_COMMAND environment variable")
		}
		if creds.GitCredHelper != "" {
			return fmt.Errorf("global credentials: gitCredHelper is not yet implemented; set credential.helper via git-config or GIT_CONFIG_GLOBAL")
		}
		if creds.TokenEnvVar != "" {
			return fmt.Errorf("global credentials: tokenEnvVar is not yet implemented; pass the token via a git credential helper or the GITHUB_TOKEN/GITLAB_TOKEN environment variable")
		}
		if name := creds.GitUserName; name != "" {
			if strings.HasPrefix(name, "-") {
				return fmt.Errorf("global credentials: gitUserName %q looks like a flag", name)
			}
			if len(name) > 255 {
				return fmt.Errorf("global credentials: gitUserName exceeds maximum length")
			}
		}
		if email := creds.GitUserEmail; email != "" {
			if strings.HasPrefix(email, "-") {
				return fmt.Errorf("global credentials: gitUserEmail %q looks like a flag", email)
			}
			if len(email) > 255 {
				return fmt.Errorf("global credentials: gitUserEmail exceeds maximum length")
			}
		}
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
		// Validate user/email values when present — reuse the same logic as setup.go
		if inc.User != "" {
			if err := validateUserName(inc.User); err != nil {
				return fmt.Errorf("include[%d]: invalid user: %w", i, err)
			}
		}
		if inc.Email != "" {
			if err := validateEmail(inc.Email); err != nil {
				return fmt.Errorf("include[%d]: invalid email: %w", i, err)
			}
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

		// Validate URL format and scheme (SEC-C3)
		if err := isAllowedRepoURL(repo.URL); err != nil {
			return fmt.Errorf("repository[%d] %q: %w", i, repo.Name, err)
		}

		// Validate git identity fields (SEC-H1)
		if repo.User != "" {
			if strings.HasPrefix(repo.User, "-") {
				return fmt.Errorf("repository[%d] %q: user value %q looks like a flag", i, repo.Name, repo.User)
			}
			if len(repo.User) > 255 {
				return fmt.Errorf("repository[%d] %q: user value exceeds maximum length", i, repo.Name)
			}
		}
		if repo.Email != "" {
			if strings.HasPrefix(repo.Email, "-") {
				return fmt.Errorf("repository[%d] %q: email value %q looks like a flag", i, repo.Name, repo.Email)
			}
			if len(repo.Email) > 255 {
				return fmt.Errorf("repository[%d] %q: email value exceeds maximum length", i, repo.Name)
			}
		}

		// Validate environment keys against denylist (SEC-C1)
		for envKey := range repo.Environment {
			if _, blocked := BlockedEnvKeys[strings.ToUpper(envKey)]; blocked {
				return fmt.Errorf("repository[%d] %q: environment key %q is not permitted", i, repo.Name, envKey)
			}
		}

		// Relative paths require basePath
		if !filepath.IsAbs(repo.Path) && config.Global.BasePath == "" {
			return fmt.Errorf("repository[%d]: relative path %q requires global basePath to be set", i, repo.Path)
		}

		// Validate path containment within basePath (SEC-C2)
		if config.Global.BasePath != "" {
			absBase, err := filepath.Abs(config.Global.BasePath)
			if err != nil {
				return fmt.Errorf("repository[%d]: resolving basePath: %w", i, err)
			}
			var absResolved string
			if filepath.IsAbs(repo.Path) {
				absResolved = filepath.Clean(repo.Path)
			} else {
				absResolved, err = filepath.Abs(filepath.Join(config.Global.BasePath, repo.Path))
				if err != nil {
					return fmt.Errorf("repository[%d]: resolving repo path: %w", i, err)
				}
			}
			sep := string(filepath.Separator)
			if !strings.HasPrefix(absResolved+sep, absBase+sep) {
				return fmt.Errorf("repository[%d] %q: path %q escapes basePath", i, repo.Name, repo.Path)
			}
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

			// Validate URL format and scheme (SEC-C3)
			if err := isAllowedRepoURL(repo.URL); err != nil {
				return fmt.Errorf("repository[%d] %q: %w", i, repo.Name, err)
			}

			// Validate git identity fields (SEC-H1)
			if repo.User != "" {
				if strings.HasPrefix(repo.User, "-") {
					return fmt.Errorf("repository[%d] %q: user value %q looks like a flag", i, repo.Name, repo.User)
				}
				if len(repo.User) > 255 {
					return fmt.Errorf("repository[%d] %q: user value exceeds maximum length", i, repo.Name)
				}
			}
			if repo.Email != "" {
				if strings.HasPrefix(repo.Email, "-") {
					return fmt.Errorf("repository[%d] %q: email value %q looks like a flag", i, repo.Name, repo.Email)
				}
				if len(repo.Email) > 255 {
					return fmt.Errorf("repository[%d] %q: email value exceeds maximum length", i, repo.Name)
				}
			}

			// Validate environment keys against denylist (SEC-C1)
			for envKey := range repo.Environment {
				if _, blocked := BlockedEnvKeys[strings.ToUpper(envKey)]; blocked {
					return fmt.Errorf("repository[%d] %q: environment key %q is not permitted", i, repo.Name, envKey)
				}
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

package config

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

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
	// Build repository graph with remote loaders injected to avoid circular imports
	builder := graph.NewGraphBuilderWithLoaders(l.LoadRemoteConfigViaGit, l.LoadRemoteConfig)
	graphQuery, err := builder.BuildGraph(path)
	if err != nil {
		return nil, fmt.Errorf("failed to build repository graph: %w", err)
	}

	// Get merged configuration (inheritance is calculated during build)
	config := graphQuery.GetMergedConfig()

	// Apply defaults (workers, timeout, branch, version)
	l.setDefaults(config)

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
	// Resolve symlinks for robust circular include detection
	if resolved, err := filepath.EvalSymlinks(absPath); err == nil {
		absPath = resolved
	}
	// If EvalSymlinks fails (e.g., file doesn't exist yet), fall back to the Abs path

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

	// First check if the data is valid YAML with a mapping structure
	var raw interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		node.IsValid = false
		return nil, node, fmt.Errorf("failed to parse YAML in %s: %w", path, err)
	}
	if _, ok := raw.(map[string]interface{}); !ok {
		node.IsValid = false
		return nil, node, fmt.Errorf("invalid config in %s: expected YAML mapping, got %T", path, raw)
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
	for _, include := range config.Includes {
		switch {
		case include.IsRemoteRepo():
			// Structured repo include — fetch via git (uses host's existing auth)
			displayPath := include.String()
			includedConfig, err := l.loadRemoteConfigWithIncludes(include, visited)
			if err != nil {
				remoteNode := &FileNode{
					Path:         displayPath,
					Repositories: []RepositoryInfo{},
					IsValid:      false,
					Includes:     []FileNode{},
				}
				node.Includes = append(node.Includes, *remoteNode)
				return nil, node, fmt.Errorf("failed to load remote include %s: %w", displayPath, err)
			}
			remoteNode := &FileNode{
				Path:         displayPath,
				Repositories: []RepositoryInfo{},
				IsValid:      true,
				Includes:     []FileNode{},
			}
			for _, repo := range includedConfig.Repositories {
				remoteNode.Repositories = append(remoteNode.Repositories, RepositoryInfo{
					Name:     repo.Name,
					Disabled: repo.Disabled,
				})
			}
			node.Includes = append(node.Includes, *remoteNode)
			config = l.mergeConfigs(&config, includedConfig)

		case include.IsRawURL():
			// Plain HTTP URL — legacy behavior
			includedConfig, err := l.loadRemoteConfigWithIncludes(include, visited)
			if err != nil {
				remoteNode := &FileNode{
					Path:         include.Path,
					Repositories: []RepositoryInfo{},
					IsValid:      false,
					Includes:     []FileNode{},
				}
				node.Includes = append(node.Includes, *remoteNode)
				return nil, node, fmt.Errorf("failed to load remote include %s: %w", include.Path, err)
			}
			remoteNode := &FileNode{
				Path:         include.Path,
				Repositories: []RepositoryInfo{},
				IsValid:      true,
				Includes:     []FileNode{},
			}
			for _, repo := range includedConfig.Repositories {
				remoteNode.Repositories = append(remoteNode.Repositories, RepositoryInfo{
					Name:     repo.Name,
					Disabled: repo.Disabled,
				})
			}
			node.Includes = append(node.Includes, *remoteNode)
			config = l.mergeConfigs(&config, includedConfig)

		default:
			// Local file path
			includePath := include.Path
			if !filepath.IsAbs(includePath) {
				includePath = filepath.Join(filepath.Dir(path), includePath)
			}
			// If the path is a directory, look for gorepos.yaml inside it
			if info, err := os.Stat(includePath); err == nil && info.IsDir() {
				includePath = filepath.Join(includePath, "gorepos.yaml")
			}

			includedConfig, includedNode, err := l.loadConfigRecursiveWithHierarchy(includePath, visited, processedFiles)
			if err != nil {
				if includedNode != nil {
					node.Includes = append(node.Includes, *includedNode)
				}
				return nil, node, fmt.Errorf("failed to load include %s: %w", includePath, err)
			}

			node.Includes = append(node.Includes, *includedNode)
			config = l.mergeConfigs(&config, includedConfig)
		}
	}

	// No validation here - only at the root level
	return &config, node, nil
}

// loadRemoteConfigWithIncludes fetches a remote config (via git or HTTP) and recursively
// processes its own includes. Local relative-path sub-includes are skipped since there is
// no local base directory for a remote config. Cycle detection is done via the visited map,
// which uses remote identifiers (URL / "repo@ref:file") as keys — these never collide with
// absolute local paths.
func (l *Loader) loadRemoteConfigWithIncludes(entry types.IncludeEntry, visited map[string]bool) (*types.Config, error) {
	// Build a unique key for cycle detection
	var key string
	var base *types.Config
	var err error

	if entry.IsRemoteRepo() {
		key = entry.Repo + "@" + entry.Ref + ":" + entry.GetFile()
		if visited[key] {
			return nil, fmt.Errorf("circular remote include detected: %s", entry.String())
		}
		visited[key] = true
		defer delete(visited, key)
		base, err = l.LoadRemoteConfigViaGit(entry.Repo, entry.Ref, entry.GetFile())
	} else {
		key = entry.Path
		if visited[key] {
			return nil, fmt.Errorf("circular remote include detected: %s", key)
		}
		visited[key] = true
		defer delete(visited, key)
		base, err = l.LoadRemoteConfig(entry.Path)
	}
	if err != nil {
		return nil, err
	}

	// Apply include-level identity to directly-defined repos BEFORE merging sub-includes.
	// This ensures only repos from this config source get the identity, not repos from
	// its sub-includes (which are separate sources with their own identity).
	base.ApplyIncludeIdentity(entry.User, entry.Email)

	// Recursively process remote sub-includes; skip local paths (unresolvable for remote configs)
	for _, sub := range base.Includes {
		if !sub.IsRemoteRepo() && !sub.IsRawURL() {
			continue // cannot resolve relative local paths from a remote config
		}
		subConfig, err := l.loadRemoteConfigWithIncludes(sub, visited)
		if err != nil {
			return nil, fmt.Errorf("failed to process sub-include %s: %w", sub.String(), err)
		}
		merged := l.mergeConfigs(base, subConfig)
		base = &merged
	}

	return base, nil
}

// LoadRemoteConfig loads configuration from a remote URL
func (l *Loader) LoadRemoteConfig(url string) (*types.Config, error) {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("remote config URL must use http:// or https://, got: %s", url)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch remote config from %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch remote config from %s: HTTP %d", url, resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10 MB limit
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

// LoadRemoteConfigViaGit fetches a config file from a git repository using the host's
// existing git authentication (SSH keys, credential manager, etc.).
// Uses a shallow sparse clone to avoid downloading the full repository.
// Requires git ≥ 2.25 for sparse-checkout --no-cone support.
// Supports branch names, tag names, and commit hashes as ref.
func (l *Loader) LoadRemoteConfigViaGit(repoURL, ref, filePath string) (*types.Config, error) {
	tmpDir, err := os.MkdirTemp("", "gorepos-include-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	isCommitHash := looksLikeCommitHash(ref)

	// Clone with minimal data: depth=1, no blobs, no checkout
	// For commit hashes, we can't use --branch, so we clone without it and fetch separately
	if isCommitHash {
		args := []string{"clone", "--filter=blob:none", "--no-checkout", "--quiet", repoURL, tmpDir}
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			return nil, fmt.Errorf("git clone failed for %s@%s: %w\n%s", repoURL, ref, err, strings.TrimSpace(string(out)))
		}
		// Fetch the specific commit
		if out, err := exec.Command("git", "-C", tmpDir, "fetch", "--depth=1", "origin", ref).CombinedOutput(); err != nil {
			return nil, fmt.Errorf("git fetch commit %s failed: %w\n%s", ref, err, strings.TrimSpace(string(out)))
		}
	} else {
		args := []string{"clone", "--depth=1", "--filter=blob:none", "--no-checkout", "--quiet"}
		if ref != "" {
			args = append(args, "--branch", ref)
		}
		args = append(args, repoURL, tmpDir)
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			return nil, fmt.Errorf("git clone failed for %s@%s: %w\n%s", repoURL, ref, err, strings.TrimSpace(string(out)))
		}
	}

	// Configure sparse checkout to fetch only the config file
	scArgs := []string{"-C", tmpDir, "sparse-checkout", "set", "--no-cone", filePath}
	if out, err := exec.Command("git", scArgs...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git sparse-checkout failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	// Checkout the target ref (FETCH_HEAD for commit hashes, default for branches/tags)
	checkoutRef := ""
	if isCommitHash {
		checkoutRef = "FETCH_HEAD"
	}
	checkoutArgs := []string{"-C", tmpDir, "checkout"}
	if checkoutRef != "" {
		checkoutArgs = append(checkoutArgs, checkoutRef)
	}
	if out, err := exec.Command("git", checkoutArgs...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git checkout failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, filePath))
	if err != nil {
		return nil, fmt.Errorf("config file %q not found in %s@%s", filePath, repoURL, ref)
	}

	var config types.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config from %s: %w", repoURL, err)
	}

	l.setDefaults(&config)

	if err := l.validatePartialConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed for %s: %w", repoURL, err)
	}

	return &config, nil
}

// looksLikeCommitHash returns true if ref appears to be a git commit hash (7-40 hex chars).
func looksLikeCommitHash(ref string) bool {
	if len(ref) < 7 || len(ref) > 40 {
		return false
	}
	for _, c := range ref {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

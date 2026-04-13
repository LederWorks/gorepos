package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LederWorks/gorepos/internal/config"
	"github.com/LederWorks/gorepos/pkg/types"
)

// LoadConfigWithVerbose loads configuration from the given config file path (or auto-detects it),
// optionally printing verbose information. This is the shared implementation used by multiple commands.
func LoadConfigWithVerbose(cfgFile string, verbose bool) (*config.ConfigLoadResult, error) {
	loader := config.NewLoader()

	// Get config file path
	configPath := cfgFile
	if configPath == "" {
		var err error
		configPath, err = config.GetConfigPath()
		if err != nil {
			return nil, err
		}
	}

	if verbose {
		fmt.Printf("Loading configuration from: %s\n", configPath)
		fmt.Println()
	}

	// Load configuration with details
	result, err := loader.LoadConfigWithDetails(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	return result, nil
}

// FilterRepositoriesByContext returns the subset of repositories relevant to the current working
// directory. When CWD is at or outside basePath, all repositories are returned. When CWD is inside
// a subtree, only repositories whose path starts with that relative sub-path are returned.
func FilterRepositoriesByContext(repositories []types.Repository, basePath string) []types.Repository {
	// No context filtering when basePath is unconfigured.
	if basePath == "" {
		return repositories
	}

	cwd, err := os.Getwd()
	if err != nil {
		return repositories
	}

	// Resolve symlinks so macOS /var → /private/var doesn't break prefix matching.
	// Clean paths first to normalize trailing slashes (e.g. "/base/" → "/base").
	normBase := filepath.ToSlash(filepath.Clean(basePath))
	if real, err := filepath.EvalSymlinks(basePath); err == nil {
		normBase = filepath.ToSlash(filepath.Clean(real))
	}
	normCwd := filepath.ToSlash(filepath.Clean(cwd))
	if real, err := filepath.EvalSymlinks(cwd); err == nil {
		normCwd = filepath.ToSlash(filepath.Clean(real))
	}

	// At base path or outside (including false-positive like /base matching /base2) — show everything.
	// Compute the boundary-aware prefix once to avoid duplication.
	normBaseSlash := normBase + "/"
	if normCwd == normBase || !strings.HasPrefix(normCwd, normBaseSlash) {
		return repositories
	}

	relPath := strings.TrimPrefix(normCwd, normBaseSlash)
	relPath = strings.Trim(relPath, "/")
	if relPath == "" {
		return repositories
	}

	var result []types.Repository
	for _, repo := range repositories {
		repoPath := filepath.ToSlash(repo.Path)
		if repoPath == relPath || strings.HasPrefix(repoPath, relPath+"/") {
			result = append(result, repo)
		}
	}
	return result
}

// GetContextRepositoryNames extracts repository names that are relevant to the current directory context.
// basePath and currentPath should already be normalized with filepath.ToSlash.
func GetContextRepositoryNames(repositories []types.Repository, basePath, currentPath string) []string {
	var contextRepos []string

	// Get relative path from basePath
	relPath := strings.TrimPrefix(currentPath, basePath)
	relPath = strings.TrimPrefix(relPath, "/")
	relPath = strings.TrimPrefix(relPath, "\\")

	if relPath == "" {
		// At base path, include all repositories
		for _, repo := range repositories {
			contextRepos = append(contextRepos, repo.Name)
		}
		return contextRepos
	}

	// Normalize path separators
	relPath = filepath.ToSlash(relPath)

	for _, repo := range repositories {
		// Normalize repository path
		repoPath := filepath.ToSlash(repo.Path)
		repoDir := filepath.Dir(repoPath)
		if repoDir == "." {
			repoDir = ""
		}

		// Check if repository is in current context
		if repoDir == "" {
			// Repository at base level
			continue
		}

		if strings.HasPrefix(repoDir, relPath) || strings.HasPrefix(relPath, repoDir) {
			contextRepos = append(contextRepos, repo.Name)
		}
	}

	return contextRepos
}

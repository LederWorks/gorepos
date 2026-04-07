package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/LederWorks/gorepos/internal/config"
	"github.com/LederWorks/gorepos/internal/executor"
	"github.com/LederWorks/gorepos/internal/repository"
	"github.com/LederWorks/gorepos/pkg/types"
)

// StatusCommand handles the status command
type StatusCommand struct {
	configFile string
	verbose    bool
	workers    int
	dryRun     bool
}

// NewStatusCommand creates a new status command handler
func NewStatusCommand() *StatusCommand {
	return &StatusCommand{}
}

// Execute runs the status command
func (s *StatusCommand) Execute(configFile string, verbose bool, workers int, dryRun bool) error {
	s.configFile = configFile
	s.verbose = verbose
	s.workers = workers
	s.dryRun = dryRun

	// Load configuration
	result, err := s.loadConfigWithVerbose()
	if err != nil {
		return err
	}
	cfg := result.Config

	// Get operational repositories (filtered for status operations)
	contextRepos := s.filterRepositoriesByContext(cfg.Repositories, cfg.Global.BasePath)

	// Override workers from command line if provided
	if workers > 0 {
		cfg.Global.Workers = workers
	}

	ctx := context.Background()
	repoManager := repository.NewManager(cfg.Global.BasePath)
	exec := executor.NewPool(cfg.Global.Workers, repoManager)

	fmt.Printf("GoRepos Status (workers: %d)\n", cfg.Global.Workers)
	fmt.Println(strings.Repeat("=", 40))

	// Prepare operations for enabled repositories in current context
	var operations []types.Operation
	enabledRepos := make([]*types.Repository, 0)

	for i := range contextRepos {
		repo := &contextRepos[i]
		if repo.Disabled {
			if verbose {
				fmt.Printf("Skipping disabled repository: %s\n", repo.Name)
			}
			continue
		}
		enabledRepos = append(enabledRepos, repo)
		operations = append(operations, types.Operation{
			Repository: repo,
			Command:    "status",
			Context:    ctx,
		})
	}

	if len(operations) == 0 {
		fmt.Println("No enabled repositories found")
		return nil
	}

	if dryRun {
		fmt.Println("DRY RUN MODE - Would check status of:")
		for _, repo := range enabledRepos {
			fmt.Printf("  - %s (%s)\n", repo.Name, repo.Path)
		}
		return nil
	}

	// Execute status operations and consume results from the parallel channel
	results := exec.Execute(ctx, operations)

	var errs []error
	for result := range results {
		fmt.Printf("\n%s:\n", result.Repository.Name)

		if !result.Success || result.StatusData == nil {
			fmt.Printf("  Error: %v\n", result.Error)
			errs = append(errs, result.Error)
			continue
		}

		status := result.StatusData
		fmt.Printf("  Path: %s\n", status.Path)
		fmt.Printf("  Branch: %s\n", status.CurrentBranch)

		if status.IsClean {
			fmt.Printf("  Status: Clean\n")
		} else {
			fmt.Printf("  Status: %d uncommitted files\n", len(status.UncommittedFiles))
			if s.verbose {
				for _, file := range status.UncommittedFiles {
					fmt.Printf("    - %s\n", file)
				}
			}
		}

		if status.AheadBehind != nil {
			if status.AheadBehind.Ahead > 0 || status.AheadBehind.Behind > 0 {
				fmt.Printf("  Sync: %d ahead, %d behind\n", status.AheadBehind.Ahead, status.AheadBehind.Behind)
			} else {
				fmt.Printf("  Sync: Up to date\n")
			}
		}
	}

	if shutdownErr := exec.Shutdown(ctx); shutdownErr != nil {
		errs = append(errs, shutdownErr)
	}
	return errors.Join(errs...)
}

// loadConfigWithVerbose loads configuration with details
func (s *StatusCommand) loadConfigWithVerbose() (*config.ConfigLoadResult, error) {
	loader := config.NewLoader()

	if s.configFile != "" {
		result, err := loader.LoadConfigWithDetails(s.configFile)
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	// Try to find config file automatically
	configPath, err := config.GetConfigPath()
	if err != nil {
		return nil, fmt.Errorf("no configuration file specified and could not find default config: %w", err)
	}

	result, err := loader.LoadConfigWithDetails(configPath)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// filterRepositoriesByContext filters repositories based on current working directory context
func (s *StatusCommand) filterRepositoriesByContext(repositories []types.Repository, basePath string) []types.Repository {
	cwd, err := os.Getwd()
	if err != nil {
		return repositories // Return all repositories if we can't determine context
	}

	// Normalize paths for comparison
	normBasePath := strings.ReplaceAll(basePath, "\\", "/")
	normCwd := strings.ReplaceAll(cwd, "\\", "/")

	// Check if we're at the base path or outside of it
	if normCwd == normBasePath || !strings.HasPrefix(normCwd, normBasePath) {
		return repositories // Show all repositories when at base path or outside it
	}

	// Extract the relative path from base path
	relPath := strings.TrimPrefix(normCwd, normBasePath)
	relPath = strings.TrimPrefix(relPath, "/")

	// Find repositories that are in the current context (directory or subdirectories)
	var contextRepos []types.Repository
	for _, repo := range repositories {
		normRepoPath := strings.ReplaceAll(repo.Path, "\\", "/")
		if strings.HasPrefix(normRepoPath, relPath) {
			contextRepos = append(contextRepos, repo)
		}
	}

	return contextRepos
}

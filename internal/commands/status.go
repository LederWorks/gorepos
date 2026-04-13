package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	result, err := LoadConfigWithVerbose(s.configFile, s.verbose)
	if err != nil {
		return err
	}
	cfg := result.Config

	// Get operational repositories (filtered for status operations)
	contextRepos := FilterRepositoriesByContext(cfg.Repositories, cfg.Global.BasePath)

	// Override workers from command line if provided
	if workers > 0 {
		cfg.Global.Workers = workers
	}

	ctx := context.Background()
	repoManager := repository.NewManagerWithCredentials(cfg.Global.BasePath, cfg.Global.Credentials)
	exec := executor.NewPool(cfg.Global.Workers, repoManager)

	fmt.Printf("GoRepos Status (workers: %d)\n", cfg.Global.Workers)
	fmt.Println(strings.Repeat("=", 40))

	operations, enabledRepos := s.prepareOperations(contextRepos)

	if len(operations) == 0 {
		fmt.Println("No enabled repositories found")
		return nil
	}

	if dryRun {
		return s.printDryRun(enabledRepos)
	}

	results := exec.Execute(ctx, operations)
	errs := s.processResults(results)

	if shutdownErr := exec.Shutdown(ctx); shutdownErr != nil {
		errs = append(errs, shutdownErr)
	}
	return errors.Join(errs...)
}

// prepareOperations filters disabled repos and builds the operations slice.
func (s *StatusCommand) prepareOperations(contextRepos []types.Repository) ([]types.Operation, []*types.Repository) {
	var operations []types.Operation
	enabledRepos := make([]*types.Repository, 0)

	for i := range contextRepos {
		repo := &contextRepos[i]
		if repo.Disabled {
			if s.verbose {
				fmt.Printf("Skipping disabled repository: %s\n", repo.Name)
			}
			continue
		}
		enabledRepos = append(enabledRepos, repo)
		operations = append(operations, types.Operation{Repository: repo, Command: "status"})
	}

	return operations, enabledRepos
}

// printDryRun lists the repositories that would be checked and returns nil.
func (s *StatusCommand) printDryRun(enabledRepos []*types.Repository) error {
	fmt.Println("DRY RUN MODE - Would check status of:")
	for _, repo := range enabledRepos {
		fmt.Printf("  - %s (%s)\n", repo.Name, repo.Path)
	}
	return nil
}

// processResults drains the results channel, prints each repo status, and collects errors.
func (s *StatusCommand) processResults(results <-chan types.Result) []error {
	var errs []error
	for result := range results {
		fmt.Printf("\n%s:\n", result.Repository.Name)
		if err := s.printResult(result); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// printResult prints a single repository status result. Returns any execution error.
func (s *StatusCommand) printResult(result types.Result) error {
	if !result.Success || result.StatusData == nil {
		if result.Error != nil {
			fmt.Printf("  Error: %v\n", result.Error)
			return result.Error
		}
		return nil
	}

	s.printStatusData(result.StatusData)
	return nil
}

// printStatusData formats and prints the detailed status for a repository.
func (s *StatusCommand) printStatusData(status *types.RepoStatus) {
	fmt.Printf("  Path: %s\n", status.Path)
	fmt.Printf("  Branch: %s\n", status.CurrentBranch)
	s.printCleanStatus(status)
	s.printSyncStatus(status)
}

// printCleanStatus prints whether the working tree is clean or lists uncommitted files.
func (s *StatusCommand) printCleanStatus(status *types.RepoStatus) {
	if status.IsClean {
		fmt.Printf("  Status: Clean\n")
		return
	}
	fmt.Printf("  Status: %d uncommitted files\n", len(status.UncommittedFiles))
	if s.verbose {
		for _, file := range status.UncommittedFiles {
			fmt.Printf("    - %s\n", file)
		}
	}
}

// printSyncStatus prints ahead/behind information if available.
func (s *StatusCommand) printSyncStatus(status *types.RepoStatus) {
	if status.AheadBehind == nil {
		return
	}
	if status.AheadBehind.Ahead > 0 || status.AheadBehind.Behind > 0 {
		fmt.Printf("  Sync: %d ahead, %d behind\n", status.AheadBehind.Ahead, status.AheadBehind.Behind)
	} else {
		fmt.Printf("  Sync: Up to date\n")
	}
}


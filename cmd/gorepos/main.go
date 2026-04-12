package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/LederWorks/gorepos/internal/commands"
	"github.com/LederWorks/gorepos/internal/config"
	"github.com/LederWorks/gorepos/internal/executor"
	"github.com/LederWorks/gorepos/internal/repository"
	"github.com/LederWorks/gorepos/pkg/types"
	"github.com/spf13/cobra"
)

// version is embedded at build time via -ldflags "-X main.version=..."
var version string

var (
	cfgFile string
	workers int
	verbose bool
	dryRun  bool

	// init command flags
	initPath     string
	initBasePath string
	initIncludes []string
	initForce    bool

	// setup command flags
	setupConfigPath string
	setupIncludes   []string
	setupUser       string
	setupEmail      string
)

var rootCmd = &cobra.Command{
	Use:   "gorepos",
	Short: "A modern, high-performance repository management tool",
	Long: `GoRepos is a modern repository management tool that provides:
- Parallel repository operations for superior performance
- YAML-based configuration with external config feeding
- Template system for content management
- Plugin architecture for extensibility`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("git"); err != nil {
			return fmt.Errorf("git is not installed or not in PATH — gorepos requires git")
		}
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of all repositories",
	Long:  "Display the current status of all configured repositories",
	RunE:  runStatus,
}

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update all repositories",
	Long:  "Pull latest changes for all configured repositories",
	RunE:  runUpdate,
}

var reposCmd = &cobra.Command{
	Use:   "repos",
	Short: "Show repository filesystem hierarchy",
	Long:  "Display the filesystem hierarchy of cloned repositories under basePath",
	RunE:  runRepos,
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration files",
	Long:  "Validate YAML configuration files against the GoRepos schema",
	RunE:  runValidate,
}

var cloneCmd = &cobra.Command{
	Use:   "clone",
	Short: "Clone missing repositories",
	Long:  "Clone any repositories that don't exist locally",
	RunE:  runClone,
}

var groupsCmd = &cobra.Command{
	Use:   "groups",
	Short: "List repository groups",
	Long:  "Display all configured groups and their repositories",
	RunE:  runGroups,
}

var graphCmd = &cobra.Command{
	Use:   "graph",
	Short: "Display configuration graph",
	Long:  "Visualize the configuration hierarchy, nodes, and relationships",
	RunE:  runGraph,
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize user configuration",
	Long:  "Create a new user configuration file with platform-appropriate defaults via an interactive wizard",
	RunE:  runInit,
}

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Add configuration sources",
	Long:  "Add include files or URLs to your existing gorepos configuration via an interactive wizard",
	RunE:  runSetup,
}

func init() {
	// Init command flags
	initCmd.Flags().StringVar(&initPath, "path", "", "Custom path for configuration file")
	initCmd.Flags().StringVarP(&initBasePath, "base-path", "b", "", "Custom base path for repositories")
	initCmd.Flags().StringSliceVar(&initIncludes, "includes", nil, "Include files or URLs to embed in initial configuration")
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "Overwrite existing configuration file")

	// Setup command flags
	setupCmd.Flags().StringVar(&setupConfigPath, "path", "", "Path to configuration file to update")
	setupCmd.Flags().StringSliceVar(&setupIncludes, "includes", nil, "Include files or URLs to add (non-interactive)")
	setupCmd.Flags().StringVar(&setupUser, "user", "", "Git user.name for remote repo includes (non-interactive)")
	setupCmd.Flags().StringVar(&setupEmail, "email", "", "Git user.email for remote repo includes (non-interactive)")

	// Add global flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "Configuration file path")
	rootCmd.PersistentFlags().IntVarP(&workers, "parallel", "p", 10, "Number of parallel workers")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "Dry run mode")

	// Add commands
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(reposCmd)
	rootCmd.AddCommand(cloneCmd)
	rootCmd.AddCommand(groupsCmd)
	rootCmd.AddCommand(graphCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(setupCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// loadConfig loads the configuration file using the graph-based path.
// Commands that need the FileHierarchy for display (validate, graph, groups)
// use loadConfigWithVerbose directly.
func loadConfig() (*types.Config, error) {
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

	loader := config.NewLoader()
	return loader.LoadConfig(configPath)
}

// loadConfigWithVerbose loads configuration and optionally shows hierarchy
func loadConfigWithVerbose() (*config.ConfigLoadResult, error) {
	return commands.LoadConfigWithVerbose(cfgFile, verbose)
}

// filterRepositoriesByContext delegates to the shared helper in the commands package.
func filterRepositoriesByContext(repos []types.Repository, basePath string) []types.Repository {
	return commands.FilterRepositoriesByContext(repos, basePath)
}

// runStatus executes the status command
func runStatus(cmd *cobra.Command, args []string) error {
	statusCommand := commands.NewStatusCommand()
	w := 0
	if cmd.Flags().Changed("parallel") {
		w = workers
	}
	return statusCommand.Execute(cfgFile, verbose, w, dryRun)
}

// runUpdate executes the update command
func runUpdate(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Override workers from command line if provided
	if cmd.Flags().Changed("parallel") {
		cfg.Global.Workers = workers
	}

	ctx := context.Background()
	repoManager := repository.NewManagerWithCredentials(cfg.Global.BasePath, cfg.Global.Credentials)
	exec := executor.NewPool(cfg.Global.Workers, repoManager)

	fmt.Printf("GoRepos Update (workers: %d)\n", cfg.Global.Workers)
	fmt.Println(strings.Repeat("=", 40))

	// Filter repositories based on current working directory context
	contextRepos := filterRepositoriesByContext(cfg.Repositories, cfg.Global.BasePath)

	// Prepare operations for enabled repositories that exist in current context
	var operations []types.Operation
	updatedRepos := make([]*types.Repository, 0)

	for i := range contextRepos {
		repo := &contextRepos[i]
		if repo.Disabled {
			if verbose {
				fmt.Printf("Skipping disabled repository: %s\n", repo.Name)
			}
			continue
		}

		if !repoManager.Exists(repo) {
			fmt.Printf("Repository %s does not exist at %s (run 'gorepos clone' first)\n", repo.Name, repo.Path)
			continue
		}

		updatedRepos = append(updatedRepos, repo)
		operations = append(operations, types.Operation{
			Repository: repo,
			Command:    "update",
			Context:    ctx,
		})
	}

	if len(operations) == 0 {
		fmt.Println("No repositories to update")
		return nil
	}

	if dryRun {
		fmt.Println("DRY RUN MODE - Would update:")
		for _, repo := range updatedRepos {
			fmt.Printf("  - %s (%s)\n", repo.Name, repo.Path)
		}
		return nil
	}

	// Execute update operations in parallel
	results := exec.Execute(ctx, operations)
	var errs []error
	for result := range results {
		if result.Success {
			fmt.Printf("[%s] OK (%s)\n", result.Repository.Name, result.Duration.Round(time.Millisecond))
		} else {
			fmt.Printf("[%s] ERROR: %v\n", result.Repository.Name, result.Error)
			errs = append(errs, result.Error)
		}
	}
	if shutdownErr := exec.Shutdown(ctx); shutdownErr != nil {
		errs = append(errs, shutdownErr)
	}
	return errors.Join(errs...)
}

// runClone executes the clone command
func runClone(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// Override workers from command line if provided
	if cmd.Flags().Changed("parallel") {
		cfg.Global.Workers = workers
	}

	ctx := context.Background()
	repoManager := repository.NewManagerWithCredentials(cfg.Global.BasePath, cfg.Global.Credentials)
	exec := executor.NewPool(cfg.Global.Workers, repoManager)

	fmt.Printf("GoRepos Clone (workers: %d)\n", cfg.Global.Workers)
	fmt.Println(strings.Repeat("=", 40))

	// Filter repositories based on current working directory context
	contextRepos := filterRepositoriesByContext(cfg.Repositories, cfg.Global.BasePath)

	// Prepare operations for enabled repositories that don't exist in current context
	var operations []types.Operation
	clonedRepos := make([]*types.Repository, 0)

	for i := range contextRepos {
		repo := &contextRepos[i]
		if repo.Disabled {
			if verbose {
				fmt.Printf("Skipping disabled repository: %s\n", repo.Name)
			}
			continue
		}

		if repoManager.Exists(repo) {
			if verbose {
				fmt.Printf("Repository %s already exists at %s\n", repo.Name, repo.Path)
			}
			continue
		}

		clonedRepos = append(clonedRepos, repo)
		operations = append(operations, types.Operation{
			Repository: repo,
			Command:    "clone",
			Context:    ctx,
		})
	}

	if len(operations) == 0 {
		fmt.Println("No repositories to clone")
		return nil
	}

	if dryRun {
		fmt.Println("DRY RUN MODE - Would clone:")
		for _, repo := range clonedRepos {
			fmt.Printf("  - %s -> %s\n", repo.URL, repo.Path)
		}
		return nil
	}

	// Execute clone operations in parallel
	results := exec.Execute(ctx, operations)
	var errs []error
	for result := range results {
		if result.Success {
			fmt.Printf("[%s] OK (%s)\n", result.Repository.Name, result.Duration.Round(time.Millisecond))
		} else {
			fmt.Printf("[%s] ERROR: %v\n", result.Repository.Name, result.Error)
			errs = append(errs, result.Error)
		}
	}
	if shutdownErr := exec.Shutdown(ctx); shutdownErr != nil {
		errs = append(errs, shutdownErr)
	}
	return errors.Join(errs...)
}

// runValidate executes the validate command
func runValidate(cmd *cobra.Command, args []string) error {
	validateCommand := commands.NewValidateCommand()
	return validateCommand.Execute(cfgFile, verbose)
}

// runRepos executes the repos command
func runRepos(cmd *cobra.Command, args []string) error {
	reposCommand := commands.NewReposCommand()
	return reposCommand.Execute(cfgFile, verbose)
}

// runGroups executes the groups command
func runGroups(cmd *cobra.Command, args []string) error {
	groupsCommand := commands.NewGroupsCommand()
	return groupsCommand.Execute(cfgFile, verbose)
}

// runGraph executes the graph command
func runGraph(cmd *cobra.Command, args []string) error {
	graphCommand := commands.NewGraphCommand()
	return graphCommand.Execute(cfgFile, verbose)
}

// runInit implements the init command
func runInit(cmd *cobra.Command, args []string) error {
	return config.RunInit(config.SetupOptions{
		Path:     initPath,
		BasePath: initBasePath,
		Includes: initIncludes,
		Force:    initForce,
		DryRun:   dryRun,
	})
}

// runSetup implements the setup command
func runSetup(cmd *cobra.Command, args []string) error {
	return config.RunSetup(config.SetupOptions{
		Path:     setupConfigPath,
		Includes: setupIncludes,
		DryRun:   dryRun,
		User:     setupUser,
		Email:    setupEmail,
	})
}

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/LederWorks/gorepos/internal/commands"
	"github.com/LederWorks/gorepos/internal/config"
	"github.com/LederWorks/gorepos/internal/executor"
	"github.com/LederWorks/gorepos/internal/repository"
	"github.com/LederWorks/gorepos/pkg/graph"
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

	// setup command flags
	setupPath     string
	setupBasePath string
	setupIncludes []string
	setupForce    bool
)

var rootCmd = &cobra.Command{
	Use:   "gorepos",
	Short: "A modern, high-performance repository management tool",
	Long: `GoRepos is a modern repository management tool that provides:
- Parallel repository operations for superior performance
- YAML-based configuration with external config feeding
- Template system for content management
- Plugin architecture for extensibility`,
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

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Initialize user configuration",
	Long:  "Create a user-specific configuration file with platform-appropriate defaults",
	RunE:  runSetup,
}

func init() {
	// Setup command flags
	setupCmd.Flags().StringVar(&setupPath, "path", "", "Custom path for configuration file")
	setupCmd.Flags().StringVarP(&setupBasePath, "base-path", "b", "", "Custom base path for repositories")
	setupCmd.Flags().StringSliceVar(&setupIncludes, "includes", nil, "Include files or URLs to add to configuration")
	setupCmd.Flags().BoolVarP(&setupForce, "force", "f", false, "Overwrite existing configuration file")

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
	rootCmd.AddCommand(setupCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// loadConfig loads the configuration file
func loadConfig() (*types.Config, error) {
	result, err := loadConfigWithVerbose()
	if err != nil {
		return nil, err
	}
	return result.Config, nil
}

// loadConfigWithVerbose loads configuration and optionally shows hierarchy
func loadConfigWithVerbose() (*config.ConfigLoadResult, error) {
	loader := config.NewLoader()

	if cfgFile != "" {
		result, err := loader.LoadConfigWithDetails(cfgFile)
		if err != nil {
			return nil, err
		}
		return result, nil
	}

	// Try to find config file automatically
	configPath, err := config.GetConfigPath()
	if err != nil {
		return nil, err
	}

	result, err := loader.LoadConfigWithDetails(configPath)
	if err != nil {
		return nil, err
	}

	if verbose {
		fmt.Printf("Using configuration file: %s\n", configPath)
	}

	return result, nil
}

// filterRepositoriesByContext uses graph-based context awareness to filter repositories
// If CWD is within a managed repository path, only return repos under that path
// If CWD is at base path or outside managed paths, return all repos
func filterRepositoriesByContext(repos []types.Repository, basePath string) []types.Repository {
	cwd, err := os.Getwd()
	if err != nil {
		if verbose {
			fmt.Printf("Warning: Could not get current directory: %v\n", err)
		}
		return repos // Return all repos if we can't determine context
	}

	// Build a graph from repositories for context analysis
	graphImpl := graph.NewRepositoryGraphImpl()
	var repoNodes []*graph.GraphNode

	// Create repository nodes
	for i, repo := range repos {
		repoNode := graph.NewGraphNode(
			fmt.Sprintf("repo:%s", repo.Name),
			graph.NodeTypeRepository,
			repo.Name,
		)
		repoNode.Repository = &repos[i]
		repoNodes = append(repoNodes, repoNode)
		graphImpl.AddNode(repoNode)
	}

	// Use graph-based filtering
	filteredNodes := graphImpl.FilterRepositoriesByGraphContext(basePath, cwd, repoNodes)

	// Convert back to repository array
	var result []types.Repository
	for _, node := range filteredNodes {
		if node.Repository != nil {
			result = append(result, *node.Repository)
		}
	}

	return result
}

// runStatus executes the status command
func runStatus(cmd *cobra.Command, args []string) error {
	statusCmd := commands.NewStatusCommand()
	return statusCmd.Execute(cfgFile, verbose, workers, dryRun)
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
	repoManager := repository.NewManager(cfg.Global.BasePath)
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
	repoManager := repository.NewManager(cfg.Global.BasePath)
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
	validateCmd := commands.NewValidateCommand()
	return validateCmd.Execute(cfgFile, verbose)
}

// runRepos executes the repos command
func runRepos(cmd *cobra.Command, args []string) error {
	reposCmd := commands.NewReposCommand()
	return reposCmd.Execute(cfgFile, verbose)
}

// runGroups executes the groups command
func runGroups(cmd *cobra.Command, args []string) error {
	groupsCmd := commands.NewGroupsCommand()
	return groupsCmd.Execute(cfgFile, verbose)
}

// runGraph executes the graph command
func runGraph(cmd *cobra.Command, args []string) error {
	graphCmd := commands.NewGraphCommand()
	return graphCmd.Execute(cfgFile, verbose)
}

// displayGraph shows comprehensive graph information in a user-friendly format
func displayGraph(graphQuery graph.GraphQuery, contextRepos []types.Repository) {
	fmt.Println("=== Configuration Graph Overview ===")

	// Create context repository map for quick lookup
	contextRepoMap := make(map[string]bool)
	if contextRepos != nil {
		for _, repo := range contextRepos {
			contextRepoMap[repo.Name] = true
		}
	}

	// Show node summary
	fmt.Println("\n--- Node Summary ---")
	for _, nodeType := range []graph.NodeType{
		graph.NodeTypeConfig, graph.NodeTypeRepository, graph.NodeTypeGroup,
		graph.NodeTypeTag, graph.NodeTypeLabel,
	} {
		nodes := graphQuery.GetNodesByType(nodeType)
		// Only show repositories that are in context
		if nodeType == graph.NodeTypeRepository && contextRepos != nil {
			fmt.Printf("%-12s: %d\n", nodeType, len(contextRepos))
		} else {
			fmt.Printf("%-12s: %d\n", nodeType, len(nodes))
		}
	}

	// Show entity classification
	fmt.Println("\n--- Node Classification ---")
	explicit := graphQuery.GetExplicitNodes()
	derived := graphQuery.GetDerivedNodes()
	configEntities := graphQuery.GetConfigEntities()
	logicalEntities := graphQuery.GetLogicalEntities()

	fmt.Printf("%-12s: %d (from configuration files)\n", "Explicit", len(explicit))
	fmt.Printf("%-12s: %d (computed from config)\n", "Derived", len(derived))
	fmt.Printf("%-12s: %d (configs + repositories)\n", "Config", len(configEntities))
	fmt.Printf("%-12s: %d (groups + computed)\n", "Logical", len(logicalEntities))

	// Show relationship summary
	fmt.Println("\n--- Relationship Summary ---")
	for _, relType := range []graph.RelationType{
		graph.RelationParentChild, graph.RelationDefines, graph.RelationIncludes,
		graph.RelationTaggedWith, graph.RelationLabeledWith,
	} {
		relationships := graphQuery.GetRelationshipsByType(relType)
		fmt.Printf("%-12s: %d\n", relType, len(relationships))
	}

	// Show configuration hierarchy
	fmt.Println("\n--- Configuration Hierarchy ---")
	displayConfigHierarchy(graphQuery, contextRepoMap)

	// Show groups with repositories (context-filtered)
	fmt.Println("\n--- Repository Groups ---")
	groups := graphQuery.GetGroupsForDisplay()
	if len(groups) == 0 {
		fmt.Println("No groups defined")
	} else {
		hasContextGroups := false
		for groupName, repos := range groups {
			// Filter repositories in this group to only those in context
			var contextRepos []string
			for _, repo := range repos {
				if len(contextRepoMap) == 0 || contextRepoMap[repo] {
					contextRepos = append(contextRepos, repo)
				}
			}
			// Only show groups that have repositories in current context
			if len(contextRepos) > 0 {
				if !hasContextGroups {
					hasContextGroups = true
				}
				fmt.Printf("\n%s (%d repositories):\n", groupName, len(contextRepos))
				for _, repo := range contextRepos {
					fmt.Printf("  • %s\n", repo)
				}
			}
		}
		if !hasContextGroups {
			fmt.Println("No groups with repositories in current context")
		}
	}

	// Show repository summary
	fmt.Println("\n--- Repository Summary ---")
	repositories := graphQuery.GetNodesByType(graph.NodeTypeRepository)
	if len(repositories) == 0 {
		fmt.Println("No repositories defined")
	} else {
		// Filter repositories to only show those in context
		for _, repoNode := range repositories {
			// Skip repositories not in current context
			if contextRepos != nil && !contextRepoMap[repoNode.Name] {
				continue
			}

			status := "●"
			if repoNode.Repository != nil && repoNode.Repository.Disabled {
				status = "○"
			}

			fmt.Printf("  %s %-30s (scope: %s)\n", status, repoNode.Name, repoNode.GetPathString())
		}
	}

	// Show tags and labels (context-filtered)
	fmt.Println("\n--- Tags and Labels ---")
	displayTagsAndLabels(graphQuery, contextRepoMap)
}

// displayTagsAndLabels shows tags and labels used by repositories in context
func displayTagsAndLabels(graphQuery graph.GraphQuery, contextRepoMap map[string]bool) {
	// Display tags
	tags := graphQuery.GetNodesByType(graph.NodeTypeTag)
	if len(tags) > 0 {
		// Filter tags to only those used by repositories in context
		relevantTags := make([]*graph.GraphNode, 0)
		for _, tagNode := range tags {
			if tagNode.Tag != nil {
				// Check if this tag is used by any repository in context
				taggedWith := graphQuery.GetRelationshipsByType(graph.RelationTaggedWith)
				isRelevant := false
				for _, rel := range taggedWith {
					if rel.To.ID == tagNode.ID {
						// If no context filtering or the tagged entity is in context
						if len(contextRepoMap) == 0 || contextRepoMap[rel.From.Name] {
							isRelevant = true
							break
						}
					}
				}
				if isRelevant {
					relevantTags = append(relevantTags, tagNode)
				}
			}
		}

		if len(relevantTags) > 0 {
			fmt.Printf("\nTags (%d):\n", len(relevantTags))
			for _, tagNode := range relevantTags {
				fmt.Printf("  🏷️  %s = %v (scope: %s)\n", tagNode.Tag.Name, tagNode.Tag.Value, tagNode.Tag.Scope)

				// Show which context entities have this tag
				taggedWith := graphQuery.GetRelationshipsByType(graph.RelationTaggedWith)
				var contextTaggedEntities []string
				for _, rel := range taggedWith {
					if rel.To.ID == tagNode.ID {
						// Only include entities that are in context
						if len(contextRepoMap) == 0 || contextRepoMap[rel.From.Name] {
							contextTaggedEntities = append(contextTaggedEntities, rel.From.Name)
						}
					}
				}
				if len(contextTaggedEntities) > 0 {
					fmt.Printf("      Used by: %s\n", strings.Join(contextTaggedEntities, ", "))
				}
			}
		} else {
			fmt.Println("\nTags: None relevant to current context")
		}
	} else {
		fmt.Println("\nTags: None defined")
	}

	// Display labels
	labels := graphQuery.GetNodesByType(graph.NodeTypeLabel)
	if len(labels) > 0 {
		// Filter labels to only those used by repositories in context
		relevantLabels := make([]*graph.GraphNode, 0)
		for _, labelNode := range labels {
			if labelNode.Label != nil {
				// Check if this label is used by any repository in context
				labeledWith := graphQuery.GetRelationshipsByType(graph.RelationLabeledWith)
				isRelevant := false
				for _, rel := range labeledWith {
					if rel.To.ID == labelNode.ID {
						// If no context filtering or the labeled entity is in context
						if len(contextRepoMap) == 0 || contextRepoMap[rel.From.Name] {
							isRelevant = true
							break
						}
					}
				}
				if isRelevant {
					relevantLabels = append(relevantLabels, labelNode)
				}
			}
		}

		if len(relevantLabels) > 0 {
			fmt.Printf("\nLabels (%d):\n", len(relevantLabels))
			for _, labelNode := range relevantLabels {
				fmt.Printf("  🏷️  %s (scope: %s)\n", labelNode.Label.Name, labelNode.Label.Scope)

				// Show which context entities have this label
				labeledWith := graphQuery.GetRelationshipsByType(graph.RelationLabeledWith)
				var contextLabeledEntities []string
				for _, rel := range labeledWith {
					if rel.To.ID == labelNode.ID {
						// Only include entities that are in context
						if len(contextRepoMap) == 0 || contextRepoMap[rel.From.Name] {
							contextLabeledEntities = append(contextLabeledEntities, rel.From.Name)
						}
					}
				}
				if len(contextLabeledEntities) > 0 {
					fmt.Printf("      Used by: %s\n", strings.Join(contextLabeledEntities, ", "))
				}
			}
		} else {
			fmt.Println("\nLabels: None relevant to current context")
		}
	} else {
		fmt.Println("\nLabels: None defined")
	}
}

// displayConfigHierarchy shows the configuration file hierarchy with context filtering
func displayConfigHierarchy(graphQuery graph.GraphQuery, contextRepoMap map[string]bool) {
	// Get root node
	configNodes := graphQuery.GetNodesByType(graph.NodeTypeConfig)
	if len(configNodes) == 0 {
		fmt.Println("No configuration nodes found")
		return
	}

	// Find root config nodes (level 1)
	var rootConfigs []*graph.GraphNode
	for _, node := range configNodes {
		if node.Level == 1 {
			rootConfigs = append(rootConfigs, node)
		}
	}

	// Display hierarchy for each root config
	for _, rootConfig := range rootConfigs {
		fmt.Printf("└── %s\n", rootConfig.Name)
		displayConfigNode(graphQuery, rootConfig, "    ", contextRepoMap)
	}
}

// displayConfigNode recursively displays a config node and its children with context filtering
func displayConfigNode(graphQuery graph.GraphQuery, node *graph.GraphNode, prefix string, contextRepoMap map[string]bool) {
	// Get children config nodes
	children := graphQuery.GetChildren(node, graph.NodeTypeConfig)

	// Get repositories defined by this config
	repositories := make([]*graph.GraphNode, 0)
	relationships := graphQuery.GetRelationshipsByType(graph.RelationDefines)
	for _, rel := range relationships {
		if rel.From.ID == node.ID && rel.To.Type == graph.NodeTypeRepository {
			// Only include repositories that are in context (if context filtering is enabled)
			if len(contextRepoMap) == 0 || contextRepoMap[rel.To.Name] {
				repositories = append(repositories, rel.To)
			}
		}
	}

	// Display repositories
	for i, repo := range repositories {
		isLast := i == len(repositories)-1 && len(children) == 0
		connector := "├─"
		if isLast {
			connector = "└─"
		}

		status := "●"
		if repo.Repository != nil && repo.Repository.Disabled {
			status = "○"
		}
		fmt.Printf("%s%s%s %s\n", prefix, connector, status, repo.Name)
	}

	// Display child configs
	for i, child := range children {
		isLast := i == len(children)-1
		connector := "├──"
		childPrefix := prefix + "│   "
		if isLast {
			connector = "└──"
			childPrefix = prefix + "    "
		}

		fmt.Printf("%s%s %s/\n", prefix, connector, child.Name)
		displayConfigNode(graphQuery, child, childPrefix, contextRepoMap)
	}
}

// runSetup implements the setup command
func runSetup(cmd *cobra.Command, args []string) error {
	return config.RunSetup(config.SetupOptions{
		Path:     setupPath,
		BasePath: setupBasePath,
		Includes: setupIncludes,
		Force:    setupForce,
	})
}

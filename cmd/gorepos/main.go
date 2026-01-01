package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/LederWorks/gorepos/internal/config"
	"github.com/LederWorks/gorepos/internal/executor"
	"github.com/LederWorks/gorepos/internal/repository"
	"github.com/LederWorks/gorepos/pkg/graph"
	"github.com/LederWorks/gorepos/pkg/types"
	"github.com/spf13/cobra"
)

var (
	cfgFile string
	workers int
	verbose bool
	dryRun  bool
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

func init() {
	// Add global flags
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "Configuration file path")
	rootCmd.PersistentFlags().IntVarP(&workers, "parallel", "p", 10, "Number of parallel workers")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVarP(&dryRun, "dry-run", "n", false, "Dry run mode")

	// Add commands
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(validateCmd)
	rootCmd.AddCommand(cloneCmd)
	rootCmd.AddCommand(groupsCmd)
	rootCmd.AddCommand(graphCmd)
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

// runStatus executes the status command
func runStatus(cmd *cobra.Command, args []string) error {
	// Load configuration with hierarchy details
	result, err := loadConfigWithVerbose()
	if err != nil {
		return err
	}
	cfg := result.Config

	// Always show configuration hierarchy
	result.PrintConfigTree()

	// Override workers from command line if provided
	if cmd.Flags().Changed("parallel") {
		cfg.Global.Workers = workers
	}

	ctx := context.Background()
	repoManager := repository.NewManager(cfg.Global.BasePath)
	exec := executor.NewPool(cfg.Global.Workers)

	fmt.Printf("GoRepos Status (workers: %d)\n", cfg.Global.Workers)
	fmt.Println(strings.Repeat("=", 40))

	// Prepare operations for enabled repositories
	var operations []types.Operation
	enabledRepos := make([]*types.Repository, 0)

	for i := range cfg.Repositories {
		repo := &cfg.Repositories[i]
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

	// Execute status operations
	results := exec.Execute(ctx, operations)

	// Process results
	for result := range results {
		fmt.Printf("\n%s:\n", result.Repository.Name)

		// Get actual repository status using the repository manager
		status, err := repoManager.Status(ctx, result.Repository)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
			continue
		}

		fmt.Printf("  Path: %s\n", status.Path)
		fmt.Printf("  Branch: %s\n", status.CurrentBranch)

		if status.IsClean {
			fmt.Printf("  Status: Clean\n")
		} else {
			fmt.Printf("  Status: %d uncommitted files\n", len(status.UncommittedFiles))
			if verbose {
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

	return exec.Shutdown(ctx)
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
	exec := executor.NewPool(cfg.Global.Workers)

	fmt.Printf("GoRepos Update (workers: %d)\n", cfg.Global.Workers)
	fmt.Println(strings.Repeat("=", 40))

	// Prepare operations for enabled repositories that exist
	var operations []types.Operation
	updatedRepos := make([]*types.Repository, 0)

	for i := range cfg.Repositories {
		repo := &cfg.Repositories[i]
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

	// Execute update operations
	for _, repo := range updatedRepos {
		fmt.Printf("Updating %s...", repo.Name)
		err := repoManager.Update(ctx, repo)
		if err != nil {
			fmt.Printf(" ERROR: %v\n", err)
		} else {
			fmt.Printf(" OK\n")
		}
	}

	return exec.Shutdown(ctx)
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
	exec := executor.NewPool(cfg.Global.Workers)

	fmt.Printf("GoRepos Clone (workers: %d)\n", cfg.Global.Workers)
	fmt.Println(strings.Repeat("=", 40))

	// Prepare operations for enabled repositories that don't exist
	var operations []types.Operation
	clonedRepos := make([]*types.Repository, 0)

	for i := range cfg.Repositories {
		repo := &cfg.Repositories[i]
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

	// Execute clone operations
	for _, repo := range clonedRepos {
		fmt.Printf("Cloning %s...", repo.Name)
		err := repoManager.Clone(ctx, repo)
		if err != nil {
			fmt.Printf(" ERROR: %v\n", err)
		} else {
			fmt.Printf(" OK\n")
		}
	}

	return exec.Shutdown(ctx)
}

// runValidate executes the validate command
func runValidate(cmd *cobra.Command, args []string) error {
	loader := config.NewLoader()

	// Get config file path
	configPath := cfgFile
	if configPath == "" {
		var err error
		configPath, err = config.GetConfigPath()
		if err != nil {
			return err
		}
	}

	fmt.Printf("Validating configuration file: %s\n", configPath)
	fmt.Println()

	// Load and validate configuration
	result, err := loader.LoadConfigWithDetails(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Configuration validation failed: %v\n", err)
		return err
	}

	// Always show configuration hierarchy with validation status
	result.PrintConfigTreeWithValidation()

	if verbose {
		fmt.Printf("Configuration files validated (%d files):\n", len(result.ProcessedFiles))
		for _, file := range result.ProcessedFiles {
			fmt.Printf("  - %s\n", file)
		}
		fmt.Printf("Schema validation passed for all included files\n")
		fmt.Printf("Configuration logic validation passed\n")
	}
	return nil
}

// runGroups executes the groups command
func runGroups(cmd *cobra.Command, args []string) error {
	result, err := loadConfigWithVerbose()
	if err != nil {
		return err
	}
	cfg := result.Config

	if verbose {
		result.PrintConfigTree()
	}

	fmt.Printf("Repository Groups:\n")
	fmt.Println(strings.Repeat("=", 20))

	if len(cfg.Groups) == 0 {
		fmt.Println("No groups defined")
		return nil
	}

	for groupName, repos := range cfg.Groups {
		fmt.Printf("\n%s (%d repositories):\n", groupName, len(repos))
		for _, repoName := range repos {
			// Find the repository to show its status
			var repo *types.Repository
			for i := range cfg.Repositories {
				if cfg.Repositories[i].Name == repoName {
					repo = &cfg.Repositories[i]
					break
				}
			}

			if repo != nil {
				status := "●"
				if repo.Disabled {
					status = "○"
				}
				fmt.Printf("  %s %s\n", status, repoName)
			} else {
				fmt.Printf("  ? %s (not found)\n", repoName)
			}
		}
	}

	return nil
}

// runGraph executes the graph command
func runGraph(cmd *cobra.Command, args []string) error {
	// Get config file path
	configPath := cfgFile
	if configPath == "" {
		var err error
		configPath, err = config.GetConfigPath()
		if err != nil {
			return err
		}
	}

	fmt.Printf("Configuration Graph: %s\n", configPath)
	fmt.Println()

	// Build the repository graph
	builder := graph.NewGraphBuilder()
	graphQuery, err := builder.BuildGraph(configPath)
	if err != nil {
		return fmt.Errorf("failed to build repository graph: %w", err)
	}

	// Display graph information
	displayGraph(graphQuery)

	return nil
}

// displayGraph shows comprehensive graph information in a user-friendly format
func displayGraph(graphQuery graph.GraphQuery) {
	fmt.Println("=== Configuration Graph Overview ===")

	// Show node summary
	fmt.Println("\n--- Node Summary ---")
	for _, nodeType := range []graph.NodeType{
		graph.NodeTypeConfig, graph.NodeTypeRepository, graph.NodeTypeGroup,
		graph.NodeTypeTag, graph.NodeTypeLabel,
	} {
		nodes := graphQuery.GetNodesByType(nodeType)
		fmt.Printf("%-12s: %d\n", nodeType, len(nodes))
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
	displayConfigHierarchy(graphQuery)

	// Show groups with repositories
	fmt.Println("\n--- Repository Groups ---")
	groups := graphQuery.GetGroupsForDisplay()
	if len(groups) == 0 {
		fmt.Println("No groups defined")
	} else {
		for groupName, repos := range groups {
			fmt.Printf("\n%s (%d repositories):\n", groupName, len(repos))
			for _, repo := range repos {
				fmt.Printf("  • %s\n", repo)
			}
		}
	}

	// Show repository summary
	fmt.Println("\n--- Repository Summary ---")
	repositories := graphQuery.GetNodesByType(graph.NodeTypeRepository)
	if len(repositories) == 0 {
		fmt.Println("No repositories defined")
	} else {
		for _, repoNode := range repositories {
			status := "●"
			if repoNode.Repository != nil && repoNode.Repository.Disabled {
				status = "○"
			}
			fmt.Printf("  %s %-30s (scope: %s)\n", status, repoNode.Name, repoNode.GetPathString())
		}
	}

	// Show tags and labels
	fmt.Println("\n--- Tags and Labels ---")
	displayTagsAndLabels(graphQuery)
}

// displayTagsAndLabels shows all tags and labels in the graph
func displayTagsAndLabels(graphQuery graph.GraphQuery) {
	// Display tags
	tags := graphQuery.GetNodesByType(graph.NodeTypeTag)
	if len(tags) > 0 {
		fmt.Printf("\nTags (%d):\n", len(tags))
		for _, tagNode := range tags {
			if tagNode.Tag != nil {
				fmt.Printf("  🏷️  %s = %v (scope: %s)\n", tagNode.Tag.Name, tagNode.Tag.Value, tagNode.Tag.Scope)

				// Show which entities have this tag
				taggedWith := graphQuery.GetRelationshipsByType(graph.RelationTaggedWith)
				var taggedEntities []string
				for _, rel := range taggedWith {
					if rel.To.ID == tagNode.ID {
						taggedEntities = append(taggedEntities, rel.From.Name)
					}
				}
				if len(taggedEntities) > 0 {
					fmt.Printf("      Used by: %s\n", strings.Join(taggedEntities, ", "))
				}
			}
		}
	} else {
		fmt.Println("\nTags: None defined")
	}

	// Display labels
	labels := graphQuery.GetNodesByType(graph.NodeTypeLabel)
	if len(labels) > 0 {
		fmt.Printf("\nLabels (%d):\n", len(labels))
		for _, labelNode := range labels {
			if labelNode.Label != nil {
				fmt.Printf("  🏷️  %s (scope: %s)\n", labelNode.Label.Name, labelNode.Label.Scope)

				// Show which entities have this label
				labeledWith := graphQuery.GetRelationshipsByType(graph.RelationLabeledWith)
				var labeledEntities []string
				for _, rel := range labeledWith {
					if rel.To.ID == labelNode.ID {
						labeledEntities = append(labeledEntities, rel.From.Name)
					}
				}
				if len(labeledEntities) > 0 {
					fmt.Printf("      Used by: %s\n", strings.Join(labeledEntities, ", "))
				}
			}
		}
	} else {
		fmt.Println("\nLabels: None defined")
	}
}

// displayConfigHierarchy shows the configuration file hierarchy
func displayConfigHierarchy(graphQuery graph.GraphQuery) {
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
		displayConfigNode(graphQuery, rootConfig, "    ")
	}
}

// displayConfigNode recursively displays a config node and its children
func displayConfigNode(graphQuery graph.GraphQuery, node *graph.GraphNode, prefix string) {
	// Get children config nodes
	children := graphQuery.GetChildren(node, graph.NodeTypeConfig)

	// Get repositories defined by this config
	repositories := make([]*graph.GraphNode, 0)
	relationships := graphQuery.GetRelationshipsByType(graph.RelationDefines)
	for _, rel := range relationships {
		if rel.From.ID == node.ID && rel.To.Type == graph.NodeTypeRepository {
			repositories = append(repositories, rel.To)
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
		displayConfigNode(graphQuery, child, childPrefix)
	}
}

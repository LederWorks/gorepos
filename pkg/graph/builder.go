package graph

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LederWorks/gorepos/pkg/types"
	"gopkg.in/yaml.v3"
)

// GraphBuilder constructs repository graphs from configuration hierarchies
type GraphBuilder struct {
	visited map[string]bool // Track visited files to prevent cycles
}

// NewGraphBuilder creates a new graph builder
func NewGraphBuilder() *GraphBuilder {
	return &GraphBuilder{
		visited: make(map[string]bool),
	}
}

// BuildGraph constructs a complete repository graph from a root configuration
func (b *GraphBuilder) BuildGraph(rootPath string) (GraphQuery, error) {
	// Initialize graph
	graph := NewRepositoryGraphImpl()

	// Create root node
	rootNode := NewGraphNode("root", NodeTypeRoot, "root")
	rootNode.Level = 0
	rootNode.Path = []string{}
	rootNode.FullPath = "root"

	// Add root node to graph
	if err := graph.AddNode(rootNode); err != nil {
		return nil, fmt.Errorf("failed to add root node: %w", err)
	}
	graph.Root = rootNode

	// Build the configuration hierarchy starting from root
	if err := b.buildConfigHierarchy(rootPath, rootNode, graph); err != nil {
		return nil, fmt.Errorf("failed to build configuration hierarchy: %w", err)
	}

	// Process repositories and groups
	if err := b.processRepositories(graph); err != nil {
		return nil, fmt.Errorf("failed to process repositories: %w", err)
	}

	if err := b.processGroups(graph); err != nil {
		return nil, fmt.Errorf("failed to process groups: %w", err)
	}

	// Process tags and labels
	if err := b.processTagsAndLabels(graph); err != nil {
		return nil, fmt.Errorf("failed to process tags and labels: %w", err)
	}

	// Build indexes for performance
	graph.BuildIndexes()

	// Validate the graph
	if err := graph.ValidateGraph(); err != nil {
		return nil, fmt.Errorf("graph validation failed: %w", err)
	}

	return graph, nil
}

// buildConfigHierarchy recursively builds the configuration hierarchy
func (b *GraphBuilder) buildConfigHierarchy(configPath string, parentNode *GraphNode, graph *RepositoryGraphImpl) error {
	// Convert to absolute path
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path %s: %w", configPath, err)
	}

	// Check for cycles
	if b.visited[absPath] {
		return fmt.Errorf("circular dependency detected: %s", absPath)
	}
	b.visited[absPath] = true
	defer func() { b.visited[absPath] = false }()

	// Load configuration
	config, err := b.loadConfig(absPath)
	if err != nil {
		return fmt.Errorf("failed to load config %s: %w", absPath, err)
	}

	// Create configuration node
	configNode := b.createConfigNode(absPath, config, parentNode)

	// Add config node to graph
	if err := graph.AddNode(configNode); err != nil {
		return fmt.Errorf("failed to add config node: %w", err)
	}

	// Establish parent-child relationship
	parentNode.AddChild(configNode)

	// Add parent-child relationship to graph
	parentChildRel := NewRelationship(
		fmt.Sprintf("pc_%s_%s", parentNode.ID, configNode.ID),
		parentNode,
		configNode,
		RelationParentChild,
	)
	if err := graph.AddRelationship(parentChildRel); err != nil {
		return fmt.Errorf("failed to add parent-child relationship: %w", err)
	}

	// Process includes recursively
	configDir := filepath.Dir(absPath)
	for _, include := range config.Includes {
		includePath := include
		if !filepath.IsAbs(include) {
			includePath = filepath.Join(configDir, include)
		}

		if err := b.buildConfigHierarchy(includePath, configNode, graph); err != nil {
			return fmt.Errorf("failed to build included hierarchy %s: %w", includePath, err)
		}
	}

	return nil
}

// createConfigNode creates a configuration node from a config file
func (b *GraphBuilder) createConfigNode(configPath string, config *types.Config, parentNode *GraphNode) *GraphNode {
	// Generate unique ID based on path
	pathHash := fmt.Sprintf("%x", sha256.Sum256([]byte(configPath)))
	nodeID := fmt.Sprintf("config_%s", pathHash[:8])

	// Extract hierarchy path from file path
	hierarchyPath := b.extractHierarchyPath(configPath)

	// Create node name from the last component of hierarchy path
	nodeName := "root"
	if len(hierarchyPath) > 0 {
		nodeName = hierarchyPath[len(hierarchyPath)-1]
	}

	// Create the node
	configNode := NewGraphNode(nodeID, NodeTypeConfig, nodeName)
	configNode.Config = config
	configNode.SetProperty("file_path", configPath)
	configNode.SetProperty("hierarchy_segments", hierarchyPath)

	// Mark config as explicit entity
	configNode.MarkAsExplicit(nodeID) // Self-referencing for configs

	// Copy templates and variables from the configuration
	if config.Templates != nil {
		for templateName, template := range config.Templates {
			configNode.SetTemplate(templateName, template)
		}
	}

	// Set hierarchy path (exclude the node name itself for path calculation)
	if len(hierarchyPath) > 0 {
		configNode.Path = hierarchyPath[:len(hierarchyPath)-1]
	}

	return configNode
}

// extractHierarchyPath extracts hierarchical path segments from a configuration file path
func (b *GraphBuilder) extractHierarchyPath(configPath string) []string {
	// Normalize path separators
	normalPath := filepath.ToSlash(configPath)

	// Split path into segments
	segments := strings.Split(normalPath, "/")

	// Find the configs directory index
	configsIndex := -1
	for i, segment := range segments {
		if segment == "configs" {
			configsIndex = i
			break
		}
	}

	if configsIndex == -1 {
		return []string{} // Root level
	}

	// Extract hierarchy levels after "configs"
	remaining := segments[configsIndex+1:]

	// Remove .yaml files from the path segments
	var hierarchyPath []string
	for i, segment := range remaining {
		if i == len(remaining)-1 && strings.HasSuffix(segment, ".yaml") {
			// Skip .yaml files at the end
			continue
		}
		hierarchyPath = append(hierarchyPath, segment)
	}

	return hierarchyPath
}

// loadConfig loads and parses a YAML configuration file
func (b *GraphBuilder) loadConfig(configPath string) (*types.Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config types.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// processRepositories creates repository nodes and relationships
func (b *GraphBuilder) processRepositories(graph *RepositoryGraphImpl) error {
	// Process all config nodes
	configNodes := graph.GetNodesByType(NodeTypeConfig)

	for _, configNode := range configNodes {
		if configNode.Config != nil {
			for _, repo := range configNode.Config.Repositories {
				// Create repository node
				repoNode := b.createRepositoryNode(&repo, configNode)

				// Add repository node to graph
				if err := graph.AddNode(repoNode); err != nil {
					return fmt.Errorf("failed to add repository node %s: %w", repoNode.ID, err)
				}

				// Create relationship: config defines repository
				definesRel := NewRelationship(
					fmt.Sprintf("def_%s_%s", configNode.ID, repoNode.ID),
					configNode,
					repoNode,
					RelationDefines,
				)
				if err := graph.AddRelationship(definesRel); err != nil {
					return fmt.Errorf("failed to add defines relationship: %w", err)
				}
			}
		}
	}

	return nil
}

// createRepositoryNode creates a repository node
func (b *GraphBuilder) createRepositoryNode(repo *types.Repository, configNode *GraphNode) *GraphNode {
	// Generate unique ID
	repoID := fmt.Sprintf("repo_%s", repo.Name)

	// Create the node
	repoNode := NewGraphNode(repoID, NodeTypeRepository, repo.Name)
	repoNode.Repository = repo
	repoNode.Path = append([]string{}, configNode.Path...)
	if configNode.Name != "root" {
		repoNode.Path = append(repoNode.Path, configNode.Name)
	}
	repoNode.FullPath = strings.Join(repoNode.Path, "/")
	if repoNode.FullPath == "" {
		repoNode.FullPath = "root"
	}
	repoNode.Level = len(repoNode.Path)

	// Mark repository as explicit config entity
	repoNode.MarkAsExplicit(configNode.ID)

	// Set repository properties
	repoNode.SetProperty("url", repo.URL)
	repoNode.SetProperty("path", repo.Path)
	repoNode.SetProperty("branch", repo.Branch)
	repoNode.SetProperty("disabled", repo.Disabled)

	// Store repository tags and labels for later processing
	if repo.Tags != nil {
		repoNode.SetProperty("tags", repo.Tags)
	}
	if repo.Labels != nil {
		repoNode.SetProperty("labels", repo.Labels)
	}

	// Copy repository-specific templates and variables if any
	if configNode.Config != nil {
		for templateName, template := range configNode.Config.Templates {
			repoNode.SetTemplate(templateName, template)
		}
	}

	return repoNode
}

// processGroups creates group nodes and calculates inheritance
func (b *GraphBuilder) processGroups(graph *RepositoryGraphImpl) error {
	// Process all config nodes for group definitions
	configNodes := graph.GetNodesByType(NodeTypeConfig)

	for _, configNode := range configNodes {
		if configNode.Config != nil && configNode.Config.Groups != nil {
			for groupName, repos := range configNode.Config.Groups {
				// Create group node
				groupNode := b.createGroupNode(groupName, repos, configNode, graph)

				// Add group node to graph
				if err := graph.AddNode(groupNode); err != nil {
					return fmt.Errorf("failed to add group node %s: %w", groupNode.ID, err)
				}

				// Create relationship: config defines group
				definesRel := NewRelationship(
					fmt.Sprintf("def_%s_%s", configNode.ID, groupNode.ID),
					configNode,
					groupNode,
					RelationDefines,
				)
				if err := graph.AddRelationship(definesRel); err != nil {
					return fmt.Errorf("failed to add defines relationship: %w", err)
				}

				// Create relationships: group includes repositories
				allRepos := append([]string{}, groupNode.Group.ExplicitRepos...)
				allRepos = append(allRepos, groupNode.Group.InheritedRepos...)

				for _, repoName := range allRepos {
					if repoNode := graph.AllRepositories[repoName]; repoNode != nil {
						includesRel := NewRelationship(
							fmt.Sprintf("inc_%s_%s", groupNode.ID, repoNode.ID),
							groupNode,
							repoNode,
							RelationIncludes,
						)
						if err := graph.AddRelationship(includesRel); err != nil {
							return fmt.Errorf("failed to add includes relationship: %w", err)
						}
					}
				}
			}
		}
	}

	return nil
}

// createGroupNode creates a group node with inheritance calculation
func (b *GraphBuilder) createGroupNode(groupName string, repos []string, configNode *GraphNode, graph *RepositoryGraphImpl) *GraphNode {
	// Generate unique ID
	groupID := fmt.Sprintf("group_%s_%s", strings.ReplaceAll(configNode.GetPathString(), "/", "_"), groupName)

	// Create the node
	groupNode := NewGraphNode(groupID, NodeTypeGroup, groupName)
	groupNode.Path = append([]string{}, configNode.Path...)
	if configNode.Name != "root" {
		groupNode.Path = append(groupNode.Path, configNode.Name)
	}
	groupNode.FullPath = strings.Join(groupNode.Path, "/")
	if groupNode.FullPath == "" {
		groupNode.FullPath = "root"
	}
	groupNode.Level = len(groupNode.Path)

	// Create group definition
	groupDef := &GroupDefinition{
		Name:          groupName,
		ExplicitRepos: append([]string{}, repos...),
		IsEmpty:       len(repos) == 0,
	}

	// Calculate inherited repositories for empty groups
	if groupDef.IsEmpty {
		groupDef.InheritedRepos = b.calculateInheritedRepos(groupNode, graph)
	}

	groupNode.Group = groupDef

	// Mark group as derived from configuration
	groupNode.MarkAsDerived(configNode.ID)
	groupNode.SetProperty("defining_config", configNode.ID)
	groupNode.SetProperty("is_empty", groupDef.IsEmpty)

	// Copy templates and variables from config if group-specific ones exist
	if configNode.Config != nil {
		for templateName, template := range configNode.Config.Templates {
			// Only set if it's a group-specific template or global
			groupNode.SetTemplate(templateName, template)
		}
	}

	// Add tags
	if groupDef.IsEmpty {
		groupNode.Tags = append(groupNode.Tags, "inherited", "derived")
	} else {
		groupNode.Tags = append(groupNode.Tags, "explicit", "derived")
	}

	return groupNode
}

// processTagsAndLabels creates tag and label nodes and their relationships
func (b *GraphBuilder) processTagsAndLabels(graph *RepositoryGraphImpl) error {
	// Track created tags and labels to avoid duplicates
	createdTags := make(map[string]*GraphNode)
	createdLabels := make(map[string]*GraphNode)

	// Process global tags and labels from config nodes
	configNodes := graph.GetNodesByType(NodeTypeConfig)
	for _, configNode := range configNodes {
		if configNode.Config != nil {
			// Process global tags
			if configNode.Config.Global.Tags != nil {
				for tagName, tagValue := range configNode.Config.Global.Tags {
					tagNode := b.createOrGetTagNode(tagName, tagValue, "global", configNode, createdTags)
					// Only add if not already in graph
					if graph.GetNode(tagNode.ID) == nil {
						if err := graph.AddNode(tagNode); err != nil {
							return fmt.Errorf("failed to add global tag node %s: %w", tagNode.ID, err)
						}
					}
					// Create relationship: config defines tag
					b.createTagRelationship(graph, configNode, tagNode)
				}
			}

			// Process global labels
			if configNode.Config.Global.Labels != nil {
				for _, labelName := range configNode.Config.Global.Labels {
					labelNode := b.createOrGetLabelNode(labelName, "global", configNode, createdLabels)
					// Only add if not already in graph
					if graph.GetNode(labelNode.ID) == nil {
						if err := graph.AddNode(labelNode); err != nil {
							return fmt.Errorf("failed to add global label node %s: %w", labelNode.ID, err)
						}
					}
					// Create relationship: config defines label
					b.createLabelRelationship(graph, configNode, labelNode)
				}
			}
		}
	}

	// Process repository-specific tags and labels
	repoNodes := graph.GetNodesByType(NodeTypeRepository)
	for _, repoNode := range repoNodes {
		if repoNode.Repository != nil {
			// Process repository tags
			if repoNode.Repository.Tags != nil {
				for tagName, tagValue := range repoNode.Repository.Tags {
					tagNode := b.createOrGetTagNode(tagName, tagValue, "repository", repoNode, createdTags)
					// Only add if not already in graph
					if graph.GetNode(tagNode.ID) == nil {
						if err := graph.AddNode(tagNode); err != nil {
							return fmt.Errorf("failed to add repository tag node %s: %w", tagNode.ID, err)
						}
					}
					// Create relationship: repository tagged_with tag
					b.createTagRelationship(graph, repoNode, tagNode)
				}
			}

			// Process repository labels
			if repoNode.Repository.Labels != nil {
				for _, labelName := range repoNode.Repository.Labels {
					labelNode := b.createOrGetLabelNode(labelName, "repository", repoNode, createdLabels)
					// Only add if not already in graph
					if graph.GetNode(labelNode.ID) == nil {
						if err := graph.AddNode(labelNode); err != nil {
							return fmt.Errorf("failed to add repository label node %s: %w", labelNode.ID, err)
						}
					}
					// Create relationship: repository labeled_with label
					b.createLabelRelationship(graph, repoNode, labelNode)
				}
			}
		}
	}

	return nil
}

// createOrGetTagNode creates a tag node or returns existing one
func (b *GraphBuilder) createOrGetTagNode(tagName string, tagValue interface{}, scope string, sourceNode *GraphNode, createdTags map[string]*GraphNode) *GraphNode {
	tagID := fmt.Sprintf("tag_%s_%v", tagName, tagValue)

	if existing, exists := createdTags[tagID]; exists {
		return existing
	}

	tagNode := NewGraphNode(tagID, NodeTypeTag, tagName)
	tagNode.Tag = &TagDefinition{
		Name:       tagName,
		Value:      tagValue,
		Scope:      scope,
		SourceType: "explicit",
	}
	tagNode.MarkAsExplicit(sourceNode.ID)

	// Set tag properties
	tagNode.SetProperty("name", tagName)
	tagNode.SetProperty("value", tagValue)
	tagNode.SetProperty("scope", scope)
	tagNode.SetProperty("source_type", "explicit")

	createdTags[tagID] = tagNode
	return tagNode
}

// createOrGetLabelNode creates a label node or returns existing one
func (b *GraphBuilder) createOrGetLabelNode(labelName string, scope string, sourceNode *GraphNode, createdLabels map[string]*GraphNode) *GraphNode {
	labelID := fmt.Sprintf("label_%s", labelName)

	if existing, exists := createdLabels[labelID]; exists {
		return existing
	}

	labelNode := NewGraphNode(labelID, NodeTypeLabel, labelName)
	labelNode.Label = &LabelDefinition{
		Name:       labelName,
		Scope:      scope,
		SourceType: "explicit",
	}
	labelNode.MarkAsExplicit(sourceNode.ID)

	// Set label properties
	labelNode.SetProperty("name", labelName)
	labelNode.SetProperty("scope", scope)
	labelNode.SetProperty("source_type", "explicit")

	createdLabels[labelID] = labelNode
	return labelNode
}

// createTagRelationship creates a tagged_with relationship
func (b *GraphBuilder) createTagRelationship(graph *RepositoryGraphImpl, fromNode, tagNode *GraphNode) error {
	relID := fmt.Sprintf("tagged_%s_%s", fromNode.ID, tagNode.ID)
	relationship := NewRelationship(relID, fromNode, tagNode, RelationTaggedWith)
	return graph.AddRelationship(relationship)
}

// createLabelRelationship creates a labeled_with relationship
func (b *GraphBuilder) createLabelRelationship(graph *RepositoryGraphImpl, fromNode, labelNode *GraphNode) error {
	relID := fmt.Sprintf("labeled_%s_%s", fromNode.ID, labelNode.ID)
	relationship := NewRelationship(relID, fromNode, labelNode, RelationLabeledWith)
	return graph.AddRelationship(relationship)
}

// calculateInheritedRepos calculates which repositories a group should inherit
func (b *GraphBuilder) calculateInheritedRepos(scopeNode *GraphNode, graph *RepositoryGraphImpl) []string {
	var inheritedRepos []string

	// Get all repositories in scope
	repositories := graph.GetNodesByType(NodeTypeRepository)

	for _, repoNode := range repositories {
		if repoNode.IsInScope(scopeNode) {
			inheritedRepos = append(inheritedRepos, repoNode.Name)
		}
	}

	// Sort for consistent output
	sort.Strings(inheritedRepos)
	return inheritedRepos
}

// GetGroupsForDisplay returns groups formatted for display with proper scoping
func (graph *RepositoryGraphImpl) GetGroupsForDisplay() map[string][]string {
	result := make(map[string][]string)

	// Get all group nodes
	groupNodes := graph.GetNodesByType(NodeTypeGroup)

	// Collect group names to detect conflicts
	groupNameCounts := make(map[string]int)
	for _, groupNode := range groupNodes {
		groupNameCounts[groupNode.Name]++
	}

	// Process each group
	for _, groupNode := range groupNodes {
		if groupNode.Group == nil {
			continue
		}

		// Determine display name
		displayName := groupNode.Name
		if groupNameCounts[groupNode.Name] > 1 && groupNode.GetPathString() != "root" {
			// Add scope prefix for conflicting names
			pathParts := strings.Split(groupNode.GetPathString(), "/")
			if len(pathParts) > 0 && pathParts[len(pathParts)-1] != "root" {
				prefix := pathParts[len(pathParts)-1]
				displayName = prefix + "-" + groupNode.Name
			}
		}

		// Get all repositories for this group
		allRepos := append([]string{}, groupNode.Group.ExplicitRepos...)
		allRepos = append(allRepos, groupNode.Group.InheritedRepos...)

		// Remove duplicates and sort
		repoSet := make(map[string]bool)
		var uniqueRepos []string
		for _, repo := range allRepos {
			if !repoSet[repo] {
				repoSet[repo] = true
				uniqueRepos = append(uniqueRepos, repo)
			}
		}
		sort.Strings(uniqueRepos)

		// Skip empty groups
		if len(uniqueRepos) > 0 {
			result[displayName] = uniqueRepos
		}
	}

	return result
}

// GetMergedConfig returns a flattened configuration for legacy compatibility
func (graph *RepositoryGraphImpl) GetMergedConfig() *types.Config {
	config := &types.Config{
		Global: types.GlobalConfig{
			BasePath: "",
			Workers:  8,
			Timeout:  300,
		},
		Repositories: make([]types.Repository, 0),
		Groups:       make(map[string][]string),
		Templates:    make(map[string]interface{}),
	}

	// Add all repositories
	repositories := graph.GetNodesByType(NodeTypeRepository)
	for _, repoNode := range repositories {
		if repoNode.Repository != nil {
			config.Repositories = append(config.Repositories, *repoNode.Repository)
		}
	}

	// Sort repositories by name for consistent output
	sort.Slice(config.Repositories, func(i, j int) bool {
		return config.Repositories[i].Name < config.Repositories[j].Name
	})

	// Add resolved groups
	groupsForDisplay := graph.GetGroupsForDisplay()
	for groupName, repos := range groupsForDisplay {
		config.Groups[groupName] = repos
	}

	// Merge global settings from configuration nodes (root takes precedence)
	configNodes := graph.GetNodesByType(NodeTypeConfig)
	for _, configNode := range configNodes {
		if configNode.Config != nil {
			if configNode.Level == 1 { // Prefer top-level configurations
				if configNode.Config.Global.BasePath != "" {
					config.Global.BasePath = configNode.Config.Global.BasePath
				}
				if configNode.Config.Global.Workers > 0 {
					config.Global.Workers = configNode.Config.Global.Workers
				}
				if configNode.Config.Global.Timeout > 0 {
					config.Global.Timeout = configNode.Config.Global.Timeout
				}

				// Merge environment variables
				if config.Global.Environment == nil {
					config.Global.Environment = make(map[string]string)
				}
				for key, value := range configNode.Config.Global.Environment {
					config.Global.Environment[key] = value
				}
			}

			// Merge templates
			for templateName, template := range configNode.Config.Templates {
				if config.Templates == nil {
					config.Templates = make(map[string]interface{})
				}
				config.Templates[templateName] = template
			}
		}
	}

	return config
}

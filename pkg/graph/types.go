package graph

import (
	"fmt"
	"strings"

	"github.com/LederWorks/gorepos/pkg/types"
)

// NodeType defines the type of a graph node
type NodeType string

const (
	NodeTypeRoot       NodeType = "root"
	NodeTypeConfig     NodeType = "config"
	NodeTypeRepository NodeType = "repository"
	NodeTypeGroup      NodeType = "group"
	NodeTypeTemplate   NodeType = "template"
	NodeTypeTag        NodeType = "tag"   // Key-value tag nodes
	NodeTypeLabel      NodeType = "label" // Simple label nodes
	// Extensible for future types like deployment, pipeline, etc.
)

// RelationType defines the type of relationship between nodes
type RelationType string

const (
	RelationParentChild RelationType = "parent_child"
	RelationIncludes    RelationType = "includes"
	RelationDefines     RelationType = "defines"
	RelationInherits    RelationType = "inherits"
	RelationDependsOn   RelationType = "depends_on"
	RelationTriggers    RelationType = "triggers"
	RelationTaggedWith  RelationType = "tagged_with"  // Entity has tag
	RelationLabeledWith RelationType = "labeled_with" // Entity has label
	// Extensible for future relationship types
)

// GraphNode represents a single node in the dependency graph
type GraphNode struct {
	// Core identification
	ID   string   `json:"id"`
	Type NodeType `json:"type"`
	Name string   `json:"name"`

	// Hierarchy and relationships
	Level    int          `json:"level"`
	Path     []string     `json:"path"`
	FullPath string       `json:"full_path"`
	Parent   *GraphNode   `json:"-"`
	Children []*GraphNode `json:"-"`
	Tags     []string     `json:"tags"`

	// Node metadata
	IsDerived    bool   `json:"is_derived"`    // true for computed/derived entities
	SourceConfig string `json:"source_config"` // which config defined this
	IsExplicit   bool   `json:"is_explicit"`   // true for explicitly defined entities

	// Content references
	Config     *types.Config     `json:"-"`
	Repository *types.Repository `json:"-"`
	Group      *GroupDefinition  `json:"-"`
	Tag        *TagDefinition    `json:"-"`
	Label      *LabelDefinition  `json:"-"`

	// Properties and extensions
	Properties map[string]interface{} `json:"properties"`
	Templates  map[string]interface{} `json:"templates"` // Templates defined at this level
	Variables  map[string]interface{} `json:"variables"` // Variables for template rendering
}

// GroupDefinition contains group-specific information
type GroupDefinition struct {
	Name           string   `json:"name"`
	ExplicitRepos  []string `json:"explicit_repos"`
	InheritedRepos []string `json:"inherited_repos"`
	IsEmpty        bool     `json:"is_empty"`
}

// TagDefinition contains tag-specific information
type TagDefinition struct {
	Name       string      `json:"name"`
	Value      interface{} `json:"value"`       // The tag value (can be string, bool, number, etc.)
	Scope      string      `json:"scope"`       // "global" or "repository"
	SourceType string      `json:"source_type"` // "explicit" or "inherited"
}

// LabelDefinition contains label-specific information
type LabelDefinition struct {
	Name       string `json:"name"`
	Scope      string `json:"scope"`       // "global" or "repository"
	SourceType string `json:"source_type"` // "explicit" or "inherited"
}

// Relationship represents a connection between two nodes
type Relationship struct {
	ID         string                 `json:"id"`
	From       *GraphNode             `json:"-"` // Exclude from JSON
	To         *GraphNode             `json:"-"` // Exclude from JSON
	FromID     string                 `json:"from_id"`
	ToID       string                 `json:"to_id"`
	Type       RelationType           `json:"type"`
	Properties map[string]interface{} `json:"properties"`
}

// RepositoryGraph represents the complete dependency graph
type RepositoryGraph struct {
	// Core graph data
	Root          *GraphNode               `json:"root"`
	Nodes         map[string]*GraphNode    `json:"nodes"`         // ID -> Node
	Relationships map[string]*Relationship `json:"relationships"` // ID -> Relationship

	// Indexes for fast queries
	NodesByType  map[NodeType][]*GraphNode `json:"-"` // Type -> []Node
	NodesByLevel map[int][]*GraphNode      `json:"-"` // Level -> []Node
	NodesByPath  map[string]*GraphNode     `json:"-"` // Path -> Node
	NodesByTag   map[string][]*GraphNode   `json:"-"` // Tag -> []Node

	// Relationship indexes
	RelationsByType map[RelationType][]*Relationship `json:"-"` // Type -> []Relationship
	RelationsByFrom map[string][]*Relationship       `json:"-"` // FromID -> []Relationship
	RelationsByTo   map[string][]*Relationship       `json:"-"` // ToID -> []Relationship

	// Cached data for performance
	AllRepositories map[string]*GraphNode `json:"-"` // RepoName -> Node
	AllGroups       map[string]*GraphNode `json:"-"` // GroupName -> Node
}

// GraphQuery provides query interface for the graph
type GraphQuery interface {
	// Node queries
	GetNode(id string) *GraphNode
	GetNodesByType(nodeType NodeType) []*GraphNode
	GetNodesByLevel(level int) []*GraphNode
	GetNodesByPath(path string) *GraphNode
	GetNodesByTag(tag string) []*GraphNode
	GetNodesByProperty(key string, value interface{}) []*GraphNode

	// Hierarchy queries
	GetChildren(node *GraphNode, nodeType NodeType) []*GraphNode
	GetDescendants(node *GraphNode, nodeType NodeType) []*GraphNode
	GetAncestors(node *GraphNode) []*GraphNode
	GetSiblings(node *GraphNode) []*GraphNode

	// Relationship queries
	GetRelationships(nodeID string, relationType RelationType) []*Relationship
	GetRelationshipsByType(relationType RelationType) []*Relationship
	GetRelated(node *GraphNode, relationType RelationType) []*GraphNode
	GetIncomingRelations(nodeID string) []*Relationship
	GetOutgoingRelations(nodeID string) []*Relationship

	// Repository-specific queries
	GetRepositoriesInScope(scopeNode *GraphNode) []*GraphNode
	GetGroupsForRepository(repoName string) []*GraphNode
	GetRepositoriesForGroup(groupName string) []*GraphNode

	// Graph operations
	AddNode(node *GraphNode) error
	AddRelationship(rel *Relationship) error
	RemoveNode(id string) error
	RemoveRelationship(id string) error

	// Utility
	BuildIndexes()
	ValidateGraph() error

	// Display and export utilities
	GetGroupsForDisplay() map[string][]string
	GetMergedConfig() *types.Config

	// Node classification queries
	GetExplicitNodes() []*GraphNode
	GetDerivedNodes() []*GraphNode
	GetConfigEntities() []*GraphNode  // Config + Repository nodes
	GetLogicalEntities() []*GraphNode // Groups + derived nodes
}

// NewRepositoryGraph creates a new empty graph
func NewRepositoryGraph() *RepositoryGraph {
	return &RepositoryGraph{
		Nodes:           make(map[string]*GraphNode),
		Relationships:   make(map[string]*Relationship),
		NodesByType:     make(map[NodeType][]*GraphNode),
		NodesByLevel:    make(map[int][]*GraphNode),
		NodesByPath:     make(map[string]*GraphNode),
		NodesByTag:      make(map[string][]*GraphNode),
		RelationsByType: make(map[RelationType][]*Relationship),
		RelationsByFrom: make(map[string][]*Relationship),
		RelationsByTo:   make(map[string][]*Relationship),
		AllRepositories: make(map[string]*GraphNode),
		AllGroups:       make(map[string]*GraphNode),
	}
}

// NewGraphNode creates a new graph node
func NewGraphNode(id string, nodeType NodeType, name string) *GraphNode {
	return &GraphNode{
		ID:         id,
		Type:       nodeType,
		Name:       name,
		Path:       make([]string, 0),
		Children:   make([]*GraphNode, 0),
		Tags:       make([]string, 0),
		Properties: make(map[string]interface{}),
		Templates:  make(map[string]interface{}),
		Variables:  make(map[string]interface{}),
		IsDerived:  false, // Default to explicit
		IsExplicit: true,  // Default to explicit
	}
}

// NewRelationship creates a new relationship
func NewRelationship(id string, from, to *GraphNode, relType RelationType) *Relationship {
	return &Relationship{
		ID:         id,
		From:       from,
		To:         to,
		FromID:     from.ID,
		ToID:       to.ID,
		Type:       relType,
		Properties: make(map[string]interface{}),
	}
}

// AddChild adds a child node and establishes parent-child relationship
func (n *GraphNode) AddChild(child *GraphNode) {
	child.Parent = n
	child.Level = n.Level + 1
	child.Path = append([]string{}, n.Path...)
	if n.Type != NodeTypeRoot {
		child.Path = append(child.Path, n.Name)
	}
	child.FullPath = strings.Join(child.Path, "/")
	if child.FullPath == "" {
		child.FullPath = "root"
	}

	n.Children = append(n.Children, child)
}

// GetPathString returns the full path as a string
func (n *GraphNode) GetPathString() string {
	if n.FullPath == "" {
		n.FullPath = strings.Join(n.Path, "/")
		if n.FullPath == "" {
			n.FullPath = "root"
		}
	}
	return n.FullPath
}

// IsInScope checks if this node is within the scope of another node
func (n *GraphNode) IsInScope(scopeNode *GraphNode) bool {
	// Root scope includes everything
	if scopeNode.Type == NodeTypeRoot {
		return true
	}

	// Get the scope path - this is where the group is defined
	scopePath := scopeNode.GetPathString()
	nodePath := n.GetPathString()

	// For root-level groups (path = "root"), only include root-level repositories
	if scopePath == "root" {
		return nodePath == "root"
	}

	// For hierarchical groups, check if repository is in the same scope or deeper
	return nodePath == scopePath || strings.HasPrefix(nodePath, scopePath+"/")
}

// HasTag checks if the node has a specific tag
func (n *GraphNode) HasTag(tag string) bool {
	for _, t := range n.Tags {
		if t == tag {
			return true
		}
	}
	return false
}

// GetProperty gets a property value with type assertion
func (n *GraphNode) GetProperty(key string) (interface{}, bool) {
	value, exists := n.Properties[key]
	return value, exists
}

// SetProperty sets a property value
func (n *GraphNode) SetProperty(key string, value interface{}) {
	n.Properties[key] = value
}

// SetTemplate sets a template at this node level
func (n *GraphNode) SetTemplate(key string, template interface{}) {
	n.Templates[key] = template
}

// SetVariable sets a variable for template rendering
func (n *GraphNode) SetVariable(key string, value interface{}) {
	n.Variables[key] = value
}

// MarkAsDerived marks this node as derived/computed from configuration
func (n *GraphNode) MarkAsDerived(sourceConfigID string) {
	n.IsDerived = true
	n.IsExplicit = false
	n.SourceConfig = sourceConfigID
}

// MarkAsExplicit marks this node as explicitly defined in configuration
func (n *GraphNode) MarkAsExplicit(sourceConfigID string) {
	n.IsDerived = false
	n.IsExplicit = true
	n.SourceConfig = sourceConfigID
}

// IsComputedEntity returns true if this node represents a computed/derived entity
func (n *GraphNode) IsComputedEntity() bool {
	return n.IsDerived
}

// IsConfigEntity returns true if this node represents actual configuration
func (n *GraphNode) IsConfigEntity() bool {
	return n.Type == NodeTypeConfig || n.Type == NodeTypeRepository
}

// GetEffectiveTemplates returns templates available at this node (including inherited)
func (n *GraphNode) GetEffectiveTemplates() map[string]interface{} {
	effective := make(map[string]interface{})

	// Start from root and work down (inheritance)
	current := n
	var ancestors []*GraphNode

	// Collect ancestors
	for current.Parent != nil {
		ancestors = append([]*GraphNode{current.Parent}, ancestors...)
		current = current.Parent
	}

	// Apply templates from ancestors to current (inheritance chain)
	for _, ancestor := range ancestors {
		for key, value := range ancestor.Templates {
			effective[key] = value
		}
	}

	// Override with this node's templates (most specific wins)
	for key, value := range n.Templates {
		effective[key] = value
	}

	return effective
}

// GetEffectiveVariables returns variables available at this node (including inherited)
func (n *GraphNode) GetEffectiveVariables() map[string]interface{} {
	effective := make(map[string]interface{})

	// Start from root and work down (inheritance)
	current := n
	var ancestors []*GraphNode

	// Collect ancestors
	for current.Parent != nil {
		ancestors = append([]*GraphNode{current.Parent}, ancestors...)
		current = current.Parent
	}

	// Apply variables from ancestors to current (inheritance chain)
	for _, ancestor := range ancestors {
		for key, value := range ancestor.Variables {
			effective[key] = value
		}
	}

	// Override with this node's variables (most specific wins)
	for key, value := range n.Variables {
		effective[key] = value
	}

	return effective
}

// String returns a string representation of the node
func (n *GraphNode) String() string {
	return fmt.Sprintf("%s:%s (%s)", n.Type, n.Name, n.GetPathString())
}

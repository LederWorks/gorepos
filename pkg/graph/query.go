package graph

import (
	"fmt"
	"sort"
)

// RepositoryGraphImpl implements the GraphQuery interface
type RepositoryGraphImpl struct {
	*RepositoryGraph
}

// NewRepositoryGraphImpl creates a new graph implementation
func NewRepositoryGraphImpl() *RepositoryGraphImpl {
	return &RepositoryGraphImpl{
		RepositoryGraph: NewRepositoryGraph(),
	}
}

// GetNode retrieves a node by ID
func (g *RepositoryGraphImpl) GetNode(id string) *GraphNode {
	return g.Nodes[id]
}

// GetNodesByType retrieves all nodes of a specific type
func (g *RepositoryGraphImpl) GetNodesByType(nodeType NodeType) []*GraphNode {
	return g.NodesByType[nodeType]
}

// GetNodesByLevel retrieves all nodes at a specific hierarchy level
func (g *RepositoryGraphImpl) GetNodesByLevel(level int) []*GraphNode {
	return g.NodesByLevel[level]
}

// GetNodesByPath retrieves a node by its path
func (g *RepositoryGraphImpl) GetNodesByPath(path string) *GraphNode {
	return g.NodesByPath[path]
}

// GetNodesByTag retrieves all nodes with a specific tag
func (g *RepositoryGraphImpl) GetNodesByTag(tag string) []*GraphNode {
	return g.NodesByTag[tag]
}

// GetNodesByProperty retrieves nodes with a specific property value
func (g *RepositoryGraphImpl) GetNodesByProperty(key string, value interface{}) []*GraphNode {
	var result []*GraphNode
	for _, node := range g.Nodes {
		if prop, exists := node.GetProperty(key); exists && prop == value {
			result = append(result, node)
		}
	}
	return result
}

// GetChildren retrieves direct children of a specific type
func (g *RepositoryGraphImpl) GetChildren(node *GraphNode, nodeType NodeType) []*GraphNode {
	var result []*GraphNode
	for _, child := range node.Children {
		if child.Type == nodeType {
			result = append(result, child)
		}
	}
	return result
}

// GetDescendants retrieves all descendants of a specific type (recursive)
func (g *RepositoryGraphImpl) GetDescendants(node *GraphNode, nodeType NodeType) []*GraphNode {
	var result []*GraphNode

	// Check direct children
	for _, child := range node.Children {
		if child.Type == nodeType {
			result = append(result, child)
		}
		// Recursively check descendants
		descendants := g.GetDescendants(child, nodeType)
		result = append(result, descendants...)
	}

	return result
}

// GetAncestors retrieves all ancestor nodes
func (g *RepositoryGraphImpl) GetAncestors(node *GraphNode) []*GraphNode {
	var result []*GraphNode
	current := node.Parent

	for current != nil {
		result = append(result, current)
		current = current.Parent
	}

	return result
}

// GetSiblings retrieves all sibling nodes
func (g *RepositoryGraphImpl) GetSiblings(node *GraphNode) []*GraphNode {
	if node.Parent == nil {
		return []*GraphNode{} // Root node has no siblings
	}

	var result []*GraphNode
	for _, sibling := range node.Parent.Children {
		if sibling.ID != node.ID {
			result = append(result, sibling)
		}
	}

	return result
}

// GetRelationships retrieves relationships for a node
func (g *RepositoryGraphImpl) GetRelationships(nodeID string, relationType RelationType) []*Relationship {
	var result []*Relationship

	// Check outgoing relationships
	for _, rel := range g.RelationsByFrom[nodeID] {
		if relationType == "" || rel.Type == relationType {
			result = append(result, rel)
		}
	}

	// Check incoming relationships
	for _, rel := range g.RelationsByTo[nodeID] {
		if relationType == "" || rel.Type == relationType {
			result = append(result, rel)
		}
	}

	return result
}

// GetRelated retrieves nodes related through specific relationship type
func (g *RepositoryGraphImpl) GetRelated(node *GraphNode, relationType RelationType) []*GraphNode {
	var result []*GraphNode

	// Get outgoing relationships
	for _, rel := range g.RelationsByFrom[node.ID] {
		if rel.Type == relationType {
			result = append(result, rel.To)
		}
	}

	// Get incoming relationships
	for _, rel := range g.RelationsByTo[node.ID] {
		if rel.Type == relationType {
			result = append(result, rel.From)
		}
	}

	return result
}

// GetIncomingRelations retrieves incoming relationships
func (g *RepositoryGraphImpl) GetIncomingRelations(nodeID string) []*Relationship {
	return g.RelationsByTo[nodeID]
}

// GetOutgoingRelations retrieves outgoing relationships
func (g *RepositoryGraphImpl) GetOutgoingRelations(nodeID string) []*Relationship {
	return g.RelationsByFrom[nodeID]
}

// GetRelationshipsByType retrieves all relationships of a specific type
func (g *RepositoryGraphImpl) GetRelationshipsByType(relationType RelationType) []*Relationship {
	return g.RelationsByType[relationType]
}

// GetExplicitNodes returns all explicitly defined nodes (from configuration)
func (g *RepositoryGraphImpl) GetExplicitNodes() []*GraphNode {
	var result []*GraphNode
	for _, node := range g.Nodes {
		if node.IsExplicit {
			result = append(result, node)
		}
	}
	return result
}

// GetDerivedNodes returns all derived/computed nodes
func (g *RepositoryGraphImpl) GetDerivedNodes() []*GraphNode {
	var result []*GraphNode
	for _, node := range g.Nodes {
		if node.IsDerived {
			result = append(result, node)
		}
	}
	return result
}

// GetConfigEntities returns all nodes that represent actual configuration (Config + Repository)
func (g *RepositoryGraphImpl) GetConfigEntities() []*GraphNode {
	var result []*GraphNode
	for _, node := range g.Nodes {
		if node.IsConfigEntity() {
			result = append(result, node)
		}
	}
	return result
}

// GetLogicalEntities returns all nodes that represent logical entities (Groups + derived)
func (g *RepositoryGraphImpl) GetLogicalEntities() []*GraphNode {
	var result []*GraphNode
	for _, node := range g.Nodes {
		if !node.IsConfigEntity() {
			result = append(result, node)
		}
	}
	return result
}

// GetRepositoriesInScope retrieves all repositories within a scope node
func (g *RepositoryGraphImpl) GetRepositoriesInScope(scopeNode *GraphNode) []*GraphNode {
	var result []*GraphNode

	for _, repoNode := range g.NodesByType[NodeTypeRepository] {
		if repoNode.IsInScope(scopeNode) {
			result = append(result, repoNode)
		}
	}

	return result
}

// GetGroupsForRepository retrieves all groups that include a repository
func (g *RepositoryGraphImpl) GetGroupsForRepository(repoName string) []*GraphNode {
	var result []*GraphNode

	for _, groupNode := range g.NodesByType[NodeTypeGroup] {
		if groupNode.Group != nil {
			// Check explicit repositories
			for _, repo := range groupNode.Group.ExplicitRepos {
				if repo == repoName {
					result = append(result, groupNode)
					break
				}
			}

			// Check inherited repositories
			for _, repo := range groupNode.Group.InheritedRepos {
				if repo == repoName {
					result = append(result, groupNode)
					break
				}
			}
		}
	}

	return result
}

// GetRepositoriesForGroup retrieves all repositories in a group
func (g *RepositoryGraphImpl) GetRepositoriesForGroup(groupName string) []*GraphNode {
	var result []*GraphNode

	// Find the group node
	for _, groupNode := range g.NodesByType[NodeTypeGroup] {
		if groupNode.Name == groupName && groupNode.Group != nil {
			// Get explicit repositories
			for _, repoName := range groupNode.Group.ExplicitRepos {
				if repoNode := g.AllRepositories[repoName]; repoNode != nil {
					result = append(result, repoNode)
				}
			}

			// Get inherited repositories
			for _, repoName := range groupNode.Group.InheritedRepos {
				if repoNode := g.AllRepositories[repoName]; repoNode != nil {
					// Avoid duplicates
					found := false
					for _, existing := range result {
						if existing.ID == repoNode.ID {
							found = true
							break
						}
					}
					if !found {
						result = append(result, repoNode)
					}
				}
			}
			break
		}
	}

	return result
}

// AddNode adds a node to the graph
func (g *RepositoryGraphImpl) AddNode(node *GraphNode) error {
	if _, exists := g.Nodes[node.ID]; exists {
		return fmt.Errorf("node with ID %s already exists", node.ID)
	}

	g.Nodes[node.ID] = node

	// Add to type index
	g.NodesByType[node.Type] = append(g.NodesByType[node.Type], node)

	// Add to level index
	g.NodesByLevel[node.Level] = append(g.NodesByLevel[node.Level], node)

	// Add to path index
	g.NodesByPath[node.GetPathString()] = node

	// Add to tag indexes
	for _, tag := range node.Tags {
		g.NodesByTag[tag] = append(g.NodesByTag[tag], node)
	}

	// Add to specific type caches
	switch node.Type {
	case NodeTypeRepository:
		g.AllRepositories[node.Name] = node
	case NodeTypeGroup:
		g.AllGroups[node.Name] = node
	}

	return nil
}

// AddRelationship adds a relationship to the graph
func (g *RepositoryGraphImpl) AddRelationship(rel *Relationship) error {
	if _, exists := g.Relationships[rel.ID]; exists {
		return fmt.Errorf("relationship with ID %s already exists", rel.ID)
	}

	g.Relationships[rel.ID] = rel

	// Add to relationship indexes
	g.RelationsByType[rel.Type] = append(g.RelationsByType[rel.Type], rel)
	g.RelationsByFrom[rel.FromID] = append(g.RelationsByFrom[rel.FromID], rel)
	g.RelationsByTo[rel.ToID] = append(g.RelationsByTo[rel.ToID], rel)

	return nil
}

// RemoveNode removes a node from the graph
func (g *RepositoryGraphImpl) RemoveNode(id string) error {
	node, exists := g.Nodes[id]
	if !exists {
		return fmt.Errorf("node with ID %s does not exist", id)
	}

	// Remove from parent's children
	if node.Parent != nil {
		for i, child := range node.Parent.Children {
			if child.ID == id {
				node.Parent.Children = append(node.Parent.Children[:i], node.Parent.Children[i+1:]...)
				break
			}
		}
	}

	// Remove all relationships involving this node
	toRemove := []string{}
	for relID, rel := range g.Relationships {
		if rel.FromID == id || rel.ToID == id {
			toRemove = append(toRemove, relID)
		}
	}
	for _, relID := range toRemove {
		g.RemoveRelationship(relID)
	}

	// Remove from all indexes
	delete(g.Nodes, id)
	g.removeFromTypeIndex(node)
	g.removeFromLevelIndex(node)
	delete(g.NodesByPath, node.GetPathString())
	g.removeFromTagIndexes(node)

	// Remove from type caches
	switch node.Type {
	case NodeTypeRepository:
		delete(g.AllRepositories, node.Name)
	case NodeTypeGroup:
		delete(g.AllGroups, node.Name)
	}

	return nil
}

// RemoveRelationship removes a relationship from the graph
func (g *RepositoryGraphImpl) RemoveRelationship(id string) error {
	rel, exists := g.Relationships[id]
	if !exists {
		return fmt.Errorf("relationship with ID %s does not exist", id)
	}

	delete(g.Relationships, id)

	// Remove from indexes
	g.removeFromRelationshipIndexes(rel)

	return nil
}

// BuildIndexes rebuilds all indexes
func (g *RepositoryGraphImpl) BuildIndexes() {
	// Clear existing indexes
	g.NodesByType = make(map[NodeType][]*GraphNode)
	g.NodesByLevel = make(map[int][]*GraphNode)
	g.NodesByPath = make(map[string]*GraphNode)
	g.NodesByTag = make(map[string][]*GraphNode)
	g.RelationsByType = make(map[RelationType][]*Relationship)
	g.RelationsByFrom = make(map[string][]*Relationship)
	g.RelationsByTo = make(map[string][]*Relationship)
	g.AllRepositories = make(map[string]*GraphNode)
	g.AllGroups = make(map[string]*GraphNode)

	// Rebuild node indexes
	for _, node := range g.Nodes {
		g.NodesByType[node.Type] = append(g.NodesByType[node.Type], node)
		g.NodesByLevel[node.Level] = append(g.NodesByLevel[node.Level], node)
		g.NodesByPath[node.GetPathString()] = node

		for _, tag := range node.Tags {
			g.NodesByTag[tag] = append(g.NodesByTag[tag], node)
		}

		switch node.Type {
		case NodeTypeRepository:
			g.AllRepositories[node.Name] = node
		case NodeTypeGroup:
			g.AllGroups[node.Name] = node
		}
	}

	// Rebuild relationship indexes
	for _, rel := range g.Relationships {
		g.RelationsByType[rel.Type] = append(g.RelationsByType[rel.Type], rel)
		g.RelationsByFrom[rel.FromID] = append(g.RelationsByFrom[rel.FromID], rel)
		g.RelationsByTo[rel.ToID] = append(g.RelationsByTo[rel.ToID], rel)
	}
}

// ValidateGraph validates the graph structure
func (g *RepositoryGraphImpl) ValidateGraph() error {
	// Check that all relationship endpoints exist
	for _, rel := range g.Relationships {
		if _, exists := g.Nodes[rel.FromID]; !exists {
			return fmt.Errorf("relationship %s references non-existent from node %s", rel.ID, rel.FromID)
		}
		if _, exists := g.Nodes[rel.ToID]; !exists {
			return fmt.Errorf("relationship %s references non-existent to node %s", rel.ID, rel.ToID)
		}
	}

	// Check for cycles in parent-child relationships
	visited := make(map[string]bool)
	for _, node := range g.Nodes {
		if !visited[node.ID] {
			if err := g.checkCycles(node, visited, make(map[string]bool)); err != nil {
				return err
			}
		}
	}

	return nil
}

// Helper methods

func (g *RepositoryGraphImpl) removeFromTypeIndex(node *GraphNode) {
	nodes := g.NodesByType[node.Type]
	for i, n := range nodes {
		if n.ID == node.ID {
			g.NodesByType[node.Type] = append(nodes[:i], nodes[i+1:]...)
			break
		}
	}
}

func (g *RepositoryGraphImpl) removeFromLevelIndex(node *GraphNode) {
	nodes := g.NodesByLevel[node.Level]
	for i, n := range nodes {
		if n.ID == node.ID {
			g.NodesByLevel[node.Level] = append(nodes[:i], nodes[i+1:]...)
			break
		}
	}
}

func (g *RepositoryGraphImpl) removeFromTagIndexes(node *GraphNode) {
	for _, tag := range node.Tags {
		nodes := g.NodesByTag[tag]
		for i, n := range nodes {
			if n.ID == node.ID {
				g.NodesByTag[tag] = append(nodes[:i], nodes[i+1:]...)
				break
			}
		}
	}
}

func (g *RepositoryGraphImpl) removeFromRelationshipIndexes(rel *Relationship) {
	// Remove from type index
	rels := g.RelationsByType[rel.Type]
	for i, r := range rels {
		if r.ID == rel.ID {
			g.RelationsByType[rel.Type] = append(rels[:i], rels[i+1:]...)
			break
		}
	}

	// Remove from from index
	rels = g.RelationsByFrom[rel.FromID]
	for i, r := range rels {
		if r.ID == rel.ID {
			g.RelationsByFrom[rel.FromID] = append(rels[:i], rels[i+1:]...)
			break
		}
	}

	// Remove from to index
	rels = g.RelationsByTo[rel.ToID]
	for i, r := range rels {
		if r.ID == rel.ID {
			g.RelationsByTo[rel.ToID] = append(rels[:i], rels[i+1:]...)
			break
		}
	}
}

func (g *RepositoryGraphImpl) checkCycles(node *GraphNode, visited, path map[string]bool) error {
	if path[node.ID] {
		return fmt.Errorf("cycle detected involving node %s", node.ID)
	}

	if visited[node.ID] {
		return nil
	}

	visited[node.ID] = true
	path[node.ID] = true

	for _, child := range node.Children {
		if err := g.checkCycles(child, visited, path); err != nil {
			return err
		}
	}

	path[node.ID] = false
	return nil
}

// PrintDebugInfo prints debug information about the graph
func (g *RepositoryGraphImpl) PrintDebugInfo() {
	fmt.Println("=== Repository Graph Debug Info ===")
	fmt.Printf("Total Nodes: %d\n", len(g.Nodes))
	fmt.Printf("Total Relationships: %d\n", len(g.Relationships))

	fmt.Println("\n--- Nodes by Type ---")
	for nodeType, nodes := range g.NodesByType {
		fmt.Printf("  %s: %d nodes\n", nodeType, len(nodes))
	}

	fmt.Println("\n--- Repository Hierarchy ---")
	repositories := g.NodesByType[NodeTypeRepository]
	sort.Slice(repositories, func(i, j int) bool {
		return repositories[i].GetPathString() < repositories[j].GetPathString()
	})

	for _, repo := range repositories {
		fmt.Printf("  %s: %s\n", repo.Name, repo.GetPathString())
	}

	fmt.Println("\n--- Group Contexts ---")
	groups := g.NodesByType[NodeTypeGroup]
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].GetPathString() < groups[j].GetPathString()
	})

	for _, group := range groups {
		fmt.Printf("  %s (%s):\n", group.Name, group.GetPathString())
		if group.Group != nil {
			fmt.Printf("    IsEmpty: %t\n", group.Group.IsEmpty)
			fmt.Printf("    ExplicitRepos: %v\n", group.Group.ExplicitRepos)
			fmt.Printf("    InheritedRepos: %v\n", group.Group.InheritedRepos)
		}
	}
}

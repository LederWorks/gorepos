package graph

import (
	"testing"
)

// helpers

func newTestGraph() *RepositoryGraphImpl {
	return NewRepositoryGraphImpl()
}

func addNode(t *testing.T, g *RepositoryGraphImpl, id string, typ NodeType, name string) *GraphNode {
	t.Helper()
	n := NewGraphNode(id, typ, name)
	if err := g.AddNode(n); err != nil {
		t.Fatalf("AddNode(%q): %v", id, err)
	}
	return n
}

func addRel(t *testing.T, g *RepositoryGraphImpl, id string, from, to *GraphNode, typ RelationType) *Relationship {
	t.Helper()
	rel := NewRelationship(id, from, to, typ)
	if err := g.AddRelationship(rel); err != nil {
		t.Fatalf("AddRelationship(%q): %v", id, err)
	}
	return rel
}

// --- AddNode ---

func TestAddNode_Success(t *testing.T) {
	g := newTestGraph()
	n := NewGraphNode("n1", NodeTypeRepository, "repo1")
	if err := g.AddNode(n); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.GetNode("n1") == nil {
		t.Error("node should be retrievable after add")
	}
}

func TestAddNode_DuplicateID(t *testing.T) {
	g := newTestGraph()
	addNode(t, g, "n1", NodeTypeRepository, "r1")
	err := g.AddNode(NewGraphNode("n1", NodeTypeRepository, "r2"))
	if err == nil {
		t.Error("expected error for duplicate node ID")
	}
}

func TestAddNode_UpdatesTypeIndex(t *testing.T) {
	g := newTestGraph()
	addNode(t, g, "r1", NodeTypeRepository, "repo1")
	addNode(t, g, "r2", NodeTypeRepository, "repo2")
	addNode(t, g, "c1", NodeTypeConfig, "cfg1")

	repos := g.GetNodesByType(NodeTypeRepository)
	if len(repos) != 2 {
		t.Errorf("expected 2 repo nodes, got %d", len(repos))
	}
	cfgs := g.GetNodesByType(NodeTypeConfig)
	if len(cfgs) != 1 {
		t.Errorf("expected 1 config node, got %d", len(cfgs))
	}
}

func TestAddNode_UpdatesRepositoryCaches(t *testing.T) {
	g := newTestGraph()
	addNode(t, g, "r1", NodeTypeRepository, "myrepo")

	if g.AllRepositories["myrepo"] == nil {
		t.Error("expected AllRepositories to be populated")
	}
}

func TestAddNode_UpdatesGroupCaches(t *testing.T) {
	g := newTestGraph()
	addNode(t, g, "grp1", NodeTypeGroup, "mygroup")

	if g.AllGroups["mygroup"] == nil {
		t.Error("expected AllGroups to be populated")
	}
}

// --- AddRelationship ---

func TestAddRelationship_Success(t *testing.T) {
	g := newTestGraph()
	from := addNode(t, g, "from", NodeTypeConfig, "c")
	to := addNode(t, g, "to", NodeTypeRepository, "r")
	addRel(t, g, "rel1", from, to, RelationDefines)

	rels := g.GetRelationshipsByType(RelationDefines)
	if len(rels) != 1 {
		t.Errorf("expected 1 relationship, got %d", len(rels))
	}
}

func TestAddRelationship_DuplicateID(t *testing.T) {
	g := newTestGraph()
	from := addNode(t, g, "from", NodeTypeConfig, "c")
	to := addNode(t, g, "to", NodeTypeRepository, "r")
	addRel(t, g, "rel1", from, to, RelationDefines)

	err := g.AddRelationship(NewRelationship("rel1", from, to, RelationInherits))
	if err == nil {
		t.Error("expected error for duplicate relationship ID")
	}
}

func TestAddRelationship_IndexesUpdated(t *testing.T) {
	g := newTestGraph()
	from := addNode(t, g, "from", NodeTypeConfig, "c")
	to := addNode(t, g, "to", NodeTypeRepository, "r")
	addRel(t, g, "rel1", from, to, RelationDefines)

	outgoing := g.GetOutgoingRelations("from")
	if len(outgoing) != 1 {
		t.Errorf("expected 1 outgoing relation, got %d", len(outgoing))
	}
	incoming := g.GetIncomingRelations("to")
	if len(incoming) != 1 {
		t.Errorf("expected 1 incoming relation, got %d", len(incoming))
	}
}

// --- RemoveNode ---

func TestRemoveNode_Success(t *testing.T) {
	g := newTestGraph()
	addNode(t, g, "n1", NodeTypeRepository, "r1")
	if err := g.RemoveNode("n1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.GetNode("n1") != nil {
		t.Error("node should be gone after removal")
	}
}

func TestRemoveNode_NonExistent(t *testing.T) {
	g := newTestGraph()
	if err := g.RemoveNode("ghost"); err == nil {
		t.Error("expected error removing non-existent node")
	}
}

func TestRemoveNode_CleansRelationships(t *testing.T) {
	g := newTestGraph()
	from := addNode(t, g, "from", NodeTypeConfig, "c")
	to := addNode(t, g, "to", NodeTypeRepository, "r")
	addRel(t, g, "rel1", from, to, RelationDefines)

	if err := g.RemoveNode("to"); err != nil {
		t.Fatalf("RemoveNode failed: %v", err)
	}
	if _, exists := g.Relationships["rel1"]; exists {
		t.Error("relationship should be removed when endpoint node is removed")
	}
}

func TestRemoveNode_CleansTypeIndex(t *testing.T) {
	g := newTestGraph()
	addNode(t, g, "r1", NodeTypeRepository, "repo1")
	_ = g.RemoveNode("r1")
	repos := g.GetNodesByType(NodeTypeRepository)
	if len(repos) != 0 {
		t.Errorf("expected 0 repo nodes after removal, got %d", len(repos))
	}
}

func TestRemoveNode_CleansRepoCache(t *testing.T) {
	g := newTestGraph()
	addNode(t, g, "r1", NodeTypeRepository, "myrepo")
	_ = g.RemoveNode("r1")
	if g.AllRepositories["myrepo"] != nil {
		t.Error("AllRepositories cache should be cleared after node removal")
	}
}

func TestRemoveNode_RemovesFromParentChildren(t *testing.T) {
	g := newTestGraph()
	parent := addNode(t, g, "parent", NodeTypeConfig, "cfg")
	child := addNode(t, g, "child", NodeTypeRepository, "r1")
	parent.AddChild(child)

	_ = g.RemoveNode("child")
	if len(parent.Children) != 0 {
		t.Errorf("expected parent to have 0 children after removal, got %d", len(parent.Children))
	}
}

// --- RemoveRelationship ---

func TestRemoveRelationship_Success(t *testing.T) {
	g := newTestGraph()
	from := addNode(t, g, "from", NodeTypeConfig, "c")
	to := addNode(t, g, "to", NodeTypeRepository, "r")
	addRel(t, g, "rel1", from, to, RelationDefines)

	if err := g.RemoveRelationship("rel1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, exists := g.Relationships["rel1"]; exists {
		t.Error("relationship should be gone")
	}
}

func TestRemoveRelationship_NonExistent(t *testing.T) {
	g := newTestGraph()
	if err := g.RemoveRelationship("ghost"); err == nil {
		t.Error("expected error removing non-existent relationship")
	}
}

func TestRemoveRelationship_CleansIndexes(t *testing.T) {
	g := newTestGraph()
	from := addNode(t, g, "from", NodeTypeConfig, "c")
	to := addNode(t, g, "to", NodeTypeRepository, "r")
	addRel(t, g, "rel1", from, to, RelationDefines)
	_ = g.RemoveRelationship("rel1")

	if len(g.GetRelationshipsByType(RelationDefines)) != 0 {
		t.Error("type index should be empty after removal")
	}
	if len(g.GetOutgoingRelations("from")) != 0 {
		t.Error("from index should be empty after removal")
	}
	if len(g.GetIncomingRelations("to")) != 0 {
		t.Error("to index should be empty after removal")
	}
}

// --- BuildIndexes ---

func TestBuildIndexes_RebuildAfterDirectMutation(t *testing.T) {
	g := newTestGraph()
	n := NewGraphNode("n1", NodeTypeRepository, "repo1")
	n.Tags = []string{"prod"}
	// Directly insert without going through AddNode to simulate manual mutation
	g.Nodes["n1"] = n
	g.Relationships["rel1"] = &Relationship{
		ID:     "rel1",
		FromID: "n1",
		ToID:   "n1",
		Type:   RelationDefines,
	}

	g.BuildIndexes()

	if len(g.GetNodesByType(NodeTypeRepository)) != 1 {
		t.Error("expected 1 repo node after rebuild")
	}
	if len(g.NodesByTag["prod"]) != 1 {
		t.Error("expected tag index to have 1 node for 'prod'")
	}
	if len(g.GetRelationshipsByType(RelationDefines)) != 1 {
		t.Error("expected 1 relationship after rebuild")
	}
}

// --- GetNode ---

func TestGetNode_ExistsAndMissing(t *testing.T) {
	g := newTestGraph()
	addNode(t, g, "n1", NodeTypeRepository, "r")

	if g.GetNode("n1") == nil {
		t.Error("should find existing node")
	}
	if g.GetNode("missing") != nil {
		t.Error("should return nil for missing node")
	}
}

// --- GetNodesByLevel ---

func TestGetNodesByLevel(t *testing.T) {
	g := newTestGraph()
	root := NewGraphNode("root", NodeTypeRoot, "root")
	_ = g.AddNode(root)
	child := NewGraphNode("child", NodeTypeConfig, "cfg")
	root.AddChild(child)
	_ = g.AddNode(child)

	l0 := g.GetNodesByLevel(0)
	l1 := g.GetNodesByLevel(1)

	if len(l0) != 1 {
		t.Errorf("expected 1 node at level 0, got %d", len(l0))
	}
	if len(l1) != 1 {
		t.Errorf("expected 1 node at level 1, got %d", len(l1))
	}
}

// --- GetNodesByPath ---

func TestGetNodesByPath(t *testing.T) {
	g := newTestGraph()
	n := NewGraphNode("n1", NodeTypeRepository, "r1")
	n.FullPath = "mypath"
	g.Nodes["n1"] = n
	g.BuildIndexes()

	found := g.GetNodesByPath("mypath")
	if found == nil || found.ID != "n1" {
		t.Error("expected to find node by path")
	}
}

// --- GetNodesByTag ---

func TestGetNodesByTag(t *testing.T) {
	g := newTestGraph()
	n := NewGraphNode("n1", NodeTypeRepository, "r1")
	n.Tags = []string{"prod", "backend"}
	g.Nodes["n1"] = n
	g.BuildIndexes()

	tagged := g.GetNodesByTag("prod")
	if len(tagged) != 1 {
		t.Errorf("expected 1 node with tag 'prod', got %d", len(tagged))
	}
	notTagged := g.GetNodesByTag("frontend")
	if len(notTagged) != 0 {
		t.Errorf("expected 0 nodes with tag 'frontend', got %d", len(notTagged))
	}
}

// --- GetNodesByProperty ---

func TestGetNodesByProperty(t *testing.T) {
	g := newTestGraph()
	n1 := addNode(t, g, "n1", NodeTypeRepository, "r1")
	n1.SetProperty("env", "prod")
	n2 := addNode(t, g, "n2", NodeTypeRepository, "r2")
	n2.SetProperty("env", "dev")

	result := g.GetNodesByProperty("env", "prod")
	if len(result) != 1 || result[0].ID != "n1" {
		t.Errorf("expected node n1, got %v", result)
	}
}

// --- GetChildren ---

func TestGetChildren(t *testing.T) {
	g := newTestGraph()
	parent := addNode(t, g, "parent", NodeTypeConfig, "cfg")
	r1 := addNode(t, g, "r1", NodeTypeRepository, "repo1")
	r2 := addNode(t, g, "r2", NodeTypeRepository, "repo2")
	c1 := addNode(t, g, "c1", NodeTypeConfig, "cfg2")

	parent.AddChild(r1)
	parent.AddChild(r2)
	parent.AddChild(c1)

	repos := g.GetChildren(parent, NodeTypeRepository)
	if len(repos) != 2 {
		t.Errorf("expected 2 repo children, got %d", len(repos))
	}
	cfgs := g.GetChildren(parent, NodeTypeConfig)
	if len(cfgs) != 1 {
		t.Errorf("expected 1 config child, got %d", len(cfgs))
	}
}

// --- GetDescendants ---

func TestGetDescendants_Recursive(t *testing.T) {
	g := newTestGraph()
	root := addNode(t, g, "root", NodeTypeRoot, "root")
	mid := addNode(t, g, "mid", NodeTypeConfig, "cfg")
	leaf := addNode(t, g, "leaf", NodeTypeRepository, "repo")

	root.AddChild(mid)
	mid.AddChild(leaf)

	descendants := g.GetDescendants(root, NodeTypeRepository)
	if len(descendants) != 1 || descendants[0].ID != "leaf" {
		t.Errorf("expected leaf as descendant, got %v", descendants)
	}
}

// --- GetAncestors ---

func TestGetAncestors(t *testing.T) {
	g := newTestGraph()
	root := addNode(t, g, "root", NodeTypeRoot, "root")
	mid := addNode(t, g, "mid", NodeTypeConfig, "cfg")
	leaf := addNode(t, g, "leaf", NodeTypeRepository, "repo")

	root.AddChild(mid)
	mid.AddChild(leaf)

	ancestors := g.GetAncestors(leaf)
	if len(ancestors) != 2 {
		t.Errorf("expected 2 ancestors, got %d", len(ancestors))
	}
}

func TestGetAncestors_RootHasNone(t *testing.T) {
	g := newTestGraph()
	root := addNode(t, g, "root", NodeTypeRoot, "root")
	ancestors := g.GetAncestors(root)
	if len(ancestors) != 0 {
		t.Errorf("root should have no ancestors, got %d", len(ancestors))
	}
}

// --- GetSiblings ---

func TestGetSiblings(t *testing.T) {
	g := newTestGraph()
	parent := addNode(t, g, "parent", NodeTypeConfig, "cfg")
	c1 := addNode(t, g, "c1", NodeTypeRepository, "r1")
	c2 := addNode(t, g, "c2", NodeTypeRepository, "r2")
	c3 := addNode(t, g, "c3", NodeTypeRepository, "r3")

	parent.AddChild(c1)
	parent.AddChild(c2)
	parent.AddChild(c3)

	siblings := g.GetSiblings(c1)
	if len(siblings) != 2 {
		t.Errorf("expected 2 siblings, got %d", len(siblings))
	}
}

func TestGetSiblings_RootHasNone(t *testing.T) {
	g := newTestGraph()
	root := addNode(t, g, "root", NodeTypeRoot, "root")
	siblings := g.GetSiblings(root)
	if len(siblings) != 0 {
		t.Errorf("root should have no siblings, got %d", len(siblings))
	}
}

// --- GetRelationships ---

func TestGetRelationships_AllAndFiltered(t *testing.T) {
	g := newTestGraph()
	from := addNode(t, g, "from", NodeTypeConfig, "c")
	to := addNode(t, g, "to", NodeTypeRepository, "r")
	to2 := addNode(t, g, "to2", NodeTypeGroup, "g")

	addRel(t, g, "r1", from, to, RelationDefines)
	addRel(t, g, "r2", from, to2, RelationInherits)

	all := g.GetRelationships("from", "")
	if len(all) != 2 {
		t.Errorf("expected 2 relationships, got %d", len(all))
	}

	filtered := g.GetRelationships("from", RelationDefines)
	if len(filtered) != 1 {
		t.Errorf("expected 1 defines relationship, got %d", len(filtered))
	}
}

// --- GetRelated ---

func TestGetRelated(t *testing.T) {
	g := newTestGraph()
	from := addNode(t, g, "from", NodeTypeConfig, "c")
	to := addNode(t, g, "to", NodeTypeRepository, "r")
	addRel(t, g, "rel1", from, to, RelationDefines)

	related := g.GetRelated(from, RelationDefines)
	if len(related) != 1 || related[0].ID != "to" {
		t.Errorf("expected 'to' as related node, got %v", related)
	}

	// Reverse direction
	related = g.GetRelated(to, RelationDefines)
	if len(related) != 1 || related[0].ID != "from" {
		t.Errorf("expected 'from' as related node (incoming), got %v", related)
	}
}

// --- GetExplicitNodes / GetDerivedNodes ---

func TestGetExplicitAndDerivedNodes(t *testing.T) {
	g := newTestGraph()
	explicit := addNode(t, g, "e1", NodeTypeRepository, "r1")
	explicit.IsExplicit = true
	explicit.IsDerived = false

	derived := addNode(t, g, "d1", NodeTypeGroup, "g1")
	derived.MarkAsDerived("cfg")

	explicitNodes := g.GetExplicitNodes()
	derivedNodes := g.GetDerivedNodes()

	found := false
	for _, n := range explicitNodes {
		if n.ID == "e1" {
			found = true
		}
	}
	if !found {
		t.Error("expected e1 in explicit nodes")
	}

	found = false
	for _, n := range derivedNodes {
		if n.ID == "d1" {
			found = true
		}
	}
	if !found {
		t.Error("expected d1 in derived nodes")
	}
}

// --- GetConfigEntities / GetLogicalEntities ---

func TestGetConfigAndLogicalEntities(t *testing.T) {
	g := newTestGraph()
	addNode(t, g, "cfg", NodeTypeConfig, "c")
	addNode(t, g, "repo", NodeTypeRepository, "r")
	addNode(t, g, "grp", NodeTypeGroup, "g")

	cfgEntities := g.GetConfigEntities()
	if len(cfgEntities) != 2 {
		t.Errorf("expected 2 config entities, got %d", len(cfgEntities))
	}
	logicalEntities := g.GetLogicalEntities()
	if len(logicalEntities) != 1 {
		t.Errorf("expected 1 logical entity, got %d", len(logicalEntities))
	}
}

// --- GetRepositoriesInScope ---

func TestGetRepositoriesInScope_Root(t *testing.T) {
	g := newTestGraph()
	root := addNode(t, g, "root", NodeTypeRoot, "root")
	r1 := addNode(t, g, "r1", NodeTypeRepository, "repo1")
	r1.FullPath = "somepath"
	r2 := addNode(t, g, "r2", NodeTypeRepository, "repo2")
	r2.FullPath = "other"

	repos := g.GetRepositoriesInScope(root)
	if len(repos) != 2 {
		t.Errorf("root scope should include all repos, got %d", len(repos))
	}
}

// --- GetGroupsForRepository ---

func TestGetGroupsForRepository(t *testing.T) {
	g := newTestGraph()
	grpNode := addNode(t, g, "grp1", NodeTypeGroup, "mygroup")
	grpNode.Group = &GroupDefinition{
		Name:          "mygroup",
		ExplicitRepos: []string{"repo1"},
		InheritedRepos: []string{"repo2"},
	}
	// Re-add to trigger type index via rebuild
	g.BuildIndexes()

	groups := g.GetGroupsForRepository("repo1")
	if len(groups) != 1 {
		t.Errorf("expected 1 group for repo1, got %d", len(groups))
	}

	groups = g.GetGroupsForRepository("repo2")
	if len(groups) != 1 {
		t.Errorf("expected 1 group for repo2 (inherited), got %d", len(groups))
	}

	groups = g.GetGroupsForRepository("repo99")
	if len(groups) != 0 {
		t.Errorf("expected 0 groups for unknown repo, got %d", len(groups))
	}
}

// --- GetRepositoriesForGroup ---

func TestGetRepositoriesForGroup(t *testing.T) {
	g := newTestGraph()

	r1 := addNode(t, g, "r1", NodeTypeRepository, "repo1")
	r2 := addNode(t, g, "r2", NodeTypeRepository, "repo2")
	_ = r1
	_ = r2

	grpNode := addNode(t, g, "grp1", NodeTypeGroup, "mygroup")
	grpNode.Group = &GroupDefinition{
		Name:           "mygroup",
		ExplicitRepos:  []string{"repo1"},
		InheritedRepos: []string{"repo2"},
	}
	g.BuildIndexes()

	repos := g.GetRepositoriesForGroup("mygroup")
	if len(repos) != 2 {
		t.Errorf("expected 2 repos in group, got %d", len(repos))
	}
}

func TestGetRepositoriesForGroup_NoDuplicates(t *testing.T) {
	g := newTestGraph()

	addNode(t, g, "r1", NodeTypeRepository, "repo1")

	grpNode := addNode(t, g, "grp1", NodeTypeGroup, "mygroup")
	grpNode.Group = &GroupDefinition{
		Name:           "mygroup",
		ExplicitRepos:  []string{"repo1"},
		InheritedRepos: []string{"repo1"}, // duplicate
	}
	g.BuildIndexes()

	repos := g.GetRepositoriesForGroup("mygroup")
	if len(repos) != 1 {
		t.Errorf("expected 1 (deduped) repo in group, got %d", len(repos))
	}
}

// --- ValidateGraph ---

func TestValidateGraph_Valid(t *testing.T) {
	g := newTestGraph()
	from := addNode(t, g, "from", NodeTypeConfig, "c")
	to := addNode(t, g, "to", NodeTypeRepository, "r")
	addRel(t, g, "rel1", from, to, RelationDefines)

	if err := g.ValidateGraph(); err != nil {
		t.Errorf("expected valid graph, got error: %v", err)
	}
}

func TestValidateGraph_MissingRelationshipEndpoint(t *testing.T) {
	g := newTestGraph()
	from := addNode(t, g, "from", NodeTypeConfig, "c")
	to := NewGraphNode("ghost", NodeTypeRepository, "r")

	// Add relationship bypassing index to point to non-existent node
	rel := NewRelationship("rel1", from, to, RelationDefines)
	g.Relationships["rel1"] = rel

	if err := g.ValidateGraph(); err == nil {
		t.Error("expected validation error for missing endpoint node")
	}
}

func TestValidateGraph_NoCycles(t *testing.T) {
	g := newTestGraph()
	root := addNode(t, g, "root", NodeTypeRoot, "root")
	child := addNode(t, g, "child", NodeTypeConfig, "cfg")
	root.AddChild(child)

	if err := g.ValidateGraph(); err != nil {
		t.Errorf("expected no cycle error, got: %v", err)
	}
}

func TestValidateGraph_WithCycle(t *testing.T) {
	g := newTestGraph()
	n1 := addNode(t, g, "n1", NodeTypeConfig, "c1")
	n2 := addNode(t, g, "n2", NodeTypeConfig, "c2")

	// Create a cycle: n1 -> n2 -> n1 in Children
	n1.Children = append(n1.Children, n2)
	n2.Parent = n1
	n2.Children = append(n2.Children, n1) // cycle back
	n1.Parent = n2

	if err := g.ValidateGraph(); err == nil {
		t.Error("expected cycle detection error")
	}
}

package graph

import (
	"testing"
)

// --- NewGraphNode ---

func TestNewGraphNode_Defaults(t *testing.T) {
	n := NewGraphNode("id1", NodeTypeRepository, "myrepo")

	if n.ID != "id1" {
		t.Errorf("expected ID %q, got %q", "id1", n.ID)
	}
	if n.Type != NodeTypeRepository {
		t.Errorf("expected type %q, got %q", NodeTypeRepository, n.Type)
	}
	if n.Name != "myrepo" {
		t.Errorf("expected name %q, got %q", "myrepo", n.Name)
	}
	if n.IsDerived {
		t.Error("expected IsDerived to be false")
	}
	if !n.IsExplicit {
		t.Error("expected IsExplicit to be true")
	}
	if n.Properties == nil || n.Templates == nil || n.Variables == nil {
		t.Error("maps should be initialized, not nil")
	}
	if n.Path == nil {
		t.Error("Path slice should be initialized")
	}
}

// --- AddChild ---

func TestAddChild_SetsParentAndLevel(t *testing.T) {
	parent := NewGraphNode("parent", NodeTypeConfig, "cfg")
	parent.Name = "cfg"

	child := NewGraphNode("child", NodeTypeRepository, "repo")
	parent.AddChild(child)

	if child.Parent != parent {
		t.Error("child.Parent should point to parent")
	}
	if child.Level != 1 {
		t.Errorf("expected child level 1, got %d", child.Level)
	}
}

func TestAddChild_RootDoesNotAddNameToPath(t *testing.T) {
	root := NewGraphNode("root", NodeTypeRoot, "root")
	child := NewGraphNode("child", NodeTypeConfig, "cfg")
	root.AddChild(child)

	if len(child.Path) != 0 {
		t.Errorf("root should not add its name to child path, got %v", child.Path)
	}
	if child.FullPath != "root" {
		t.Errorf("expected FullPath 'root', got %q", child.FullPath)
	}
}

func TestAddChild_NonRootAddsNameToPath(t *testing.T) {
	root := NewGraphNode("root", NodeTypeRoot, "root")
	parent := NewGraphNode("parent", NodeTypeConfig, "parent-cfg")
	child := NewGraphNode("child", NodeTypeConfig, "child-cfg")

	root.AddChild(parent)
	parent.AddChild(child)

	if len(child.Path) != 1 || child.Path[0] != "parent-cfg" {
		t.Errorf("expected child path [parent-cfg], got %v", child.Path)
	}
	if child.FullPath != "parent-cfg" {
		t.Errorf("expected FullPath 'parent-cfg', got %q", child.FullPath)
	}
}

func TestAddChild_AppendedToParentChildren(t *testing.T) {
	parent := NewGraphNode("p", NodeTypeConfig, "cfg")
	c1 := NewGraphNode("c1", NodeTypeRepository, "r1")
	c2 := NewGraphNode("c2", NodeTypeRepository, "r2")

	parent.AddChild(c1)
	parent.AddChild(c2)

	if len(parent.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(parent.Children))
	}
}

// --- GetPathString ---

func TestGetPathString_Empty(t *testing.T) {
	n := NewGraphNode("n", NodeTypeRepository, "r")
	s := n.GetPathString()
	if s != "root" {
		t.Errorf("expected 'root' for empty path, got %q", s)
	}
}

func TestGetPathString_WithPath(t *testing.T) {
	n := NewGraphNode("n", NodeTypeRepository, "r")
	n.Path = []string{"a", "b"}
	n.FullPath = ""
	s := n.GetPathString()
	if s != "a/b" {
		t.Errorf("expected 'a/b', got %q", s)
	}
}

func TestGetPathString_CachesResult(t *testing.T) {
	n := NewGraphNode("n", NodeTypeRepository, "r")
	n.Path = []string{"x"}
	_ = n.GetPathString()
	n.Path = []string{"y"} // mutate after cache
	s := n.GetPathString()
	// Should return cached value "x", not recompute from "y"
	if s != "x" {
		t.Errorf("expected cached value 'x', got %q", s)
	}
}

// --- IsInScope ---

func TestIsInScope_RootScopeIncludesAll(t *testing.T) {
	root := NewGraphNode("root", NodeTypeRoot, "root")
	n := NewGraphNode("n", NodeTypeRepository, "r")
	n.FullPath = "some/path"

	if !n.IsInScope(root) {
		t.Error("root scope should include all nodes")
	}
}

func TestIsInScope_SameScope(t *testing.T) {
	scope := NewGraphNode("s", NodeTypeConfig, "cfg")
	scope.FullPath = "a/b"

	n := NewGraphNode("n", NodeTypeRepository, "r")
	n.FullPath = "a/b"

	if !n.IsInScope(scope) {
		t.Error("node with same path should be in scope")
	}
}

func TestIsInScope_ChildScope(t *testing.T) {
	scope := NewGraphNode("s", NodeTypeConfig, "cfg")
	scope.FullPath = "a/b"

	n := NewGraphNode("n", NodeTypeRepository, "r")
	n.FullPath = "a/b/c"

	if !n.IsInScope(scope) {
		t.Error("node with deeper path should be in scope")
	}
}

func TestIsInScope_DifferentBranch(t *testing.T) {
	scope := NewGraphNode("s", NodeTypeConfig, "cfg")
	scope.FullPath = "a/b"

	n := NewGraphNode("n", NodeTypeRepository, "r")
	n.FullPath = "a/c"

	if n.IsInScope(scope) {
		t.Error("node on a different branch should not be in scope")
	}
}

func TestIsInScope_RootScopeNode(t *testing.T) {
	// "root" path scope only includes "root" path nodes
	scope := NewGraphNode("s", NodeTypeConfig, "cfg")
	scope.FullPath = "root"

	inScope := NewGraphNode("in", NodeTypeRepository, "r1")
	inScope.FullPath = "root"

	outScope := NewGraphNode("out", NodeTypeRepository, "r2")
	outScope.FullPath = "nested/path"

	if !inScope.IsInScope(scope) {
		t.Error("root-path node should be in root scope")
	}
	if outScope.IsInScope(scope) {
		t.Error("nested node should not be in root scope")
	}
}

// --- HasTag ---

func TestHasTag_Present(t *testing.T) {
	n := NewGraphNode("n", NodeTypeRepository, "r")
	n.Tags = []string{"prod", "backend"}

	if !n.HasTag("prod") {
		t.Error("expected node to have tag 'prod'")
	}
}

func TestHasTag_Absent(t *testing.T) {
	n := NewGraphNode("n", NodeTypeRepository, "r")
	n.Tags = []string{"prod"}

	if n.HasTag("dev") {
		t.Error("expected node to NOT have tag 'dev'")
	}
}

func TestHasTag_EmptyTags(t *testing.T) {
	n := NewGraphNode("n", NodeTypeRepository, "r")
	if n.HasTag("anything") {
		t.Error("expected no tags on fresh node")
	}
}

// --- GetProperty / SetProperty ---

func TestGetProperty_ExistsAndAbsent(t *testing.T) {
	n := NewGraphNode("n", NodeTypeRepository, "r")
	n.SetProperty("key", "value")

	val, ok := n.GetProperty("key")
	if !ok {
		t.Error("expected property to exist")
	}
	if val != "value" {
		t.Errorf("expected 'value', got %v", val)
	}

	_, ok = n.GetProperty("missing")
	if ok {
		t.Error("expected missing property to not exist")
	}
}

// --- MarkAsDerived / MarkAsExplicit ---

func TestMarkAsDerived(t *testing.T) {
	n := NewGraphNode("n", NodeTypeGroup, "g")
	n.MarkAsDerived("cfg-id")

	if !n.IsDerived {
		t.Error("expected IsDerived true")
	}
	if n.IsExplicit {
		t.Error("expected IsExplicit false")
	}
	if n.SourceConfig != "cfg-id" {
		t.Errorf("expected SourceConfig 'cfg-id', got %q", n.SourceConfig)
	}
}

func TestMarkAsExplicit(t *testing.T) {
	n := NewGraphNode("n", NodeTypeGroup, "g")
	n.MarkAsDerived("cfg-id")
	n.MarkAsExplicit("cfg-id2")

	if n.IsDerived {
		t.Error("expected IsDerived false after MarkAsExplicit")
	}
	if !n.IsExplicit {
		t.Error("expected IsExplicit true")
	}
	if n.SourceConfig != "cfg-id2" {
		t.Errorf("expected SourceConfig 'cfg-id2', got %q", n.SourceConfig)
	}
}

// --- IsComputedEntity / IsConfigEntity ---

func TestIsComputedEntity(t *testing.T) {
	n := NewGraphNode("n", NodeTypeGroup, "g")
	if n.IsComputedEntity() {
		t.Error("fresh node should not be computed")
	}
	n.MarkAsDerived("x")
	if !n.IsComputedEntity() {
		t.Error("derived node should be computed")
	}
}

func TestIsConfigEntity(t *testing.T) {
	tests := []struct {
		nodeType NodeType
		expected bool
	}{
		{NodeTypeConfig, true},
		{NodeTypeRepository, true},
		{NodeTypeGroup, false},
		{NodeTypeRoot, false},
		{NodeTypeTag, false},
		{NodeTypeLabel, false},
	}
	for _, tt := range tests {
		n := NewGraphNode("n", tt.nodeType, "x")
		if n.IsConfigEntity() != tt.expected {
			t.Errorf("IsConfigEntity() for %q: expected %v, got %v", tt.nodeType, tt.expected, n.IsConfigEntity())
		}
	}
}

// --- GetEffectiveTemplates ---

func TestGetEffectiveTemplates_NoParent(t *testing.T) {
	n := NewGraphNode("n", NodeTypeRepository, "r")
	n.SetTemplate("tmpl", "value")

	eff := n.GetEffectiveTemplates()
	if eff["tmpl"] != "value" {
		t.Errorf("expected template 'tmpl' with value 'value'")
	}
}

func TestGetEffectiveTemplates_Inheritance(t *testing.T) {
	root := NewGraphNode("root", NodeTypeRoot, "root")
	root.SetTemplate("global-tmpl", "global")

	child := NewGraphNode("child", NodeTypeConfig, "cfg")
	child.SetTemplate("local-tmpl", "local")
	root.AddChild(child)

	eff := child.GetEffectiveTemplates()
	if eff["global-tmpl"] != "global" {
		t.Error("expected inherited 'global-tmpl'")
	}
	if eff["local-tmpl"] != "local" {
		t.Error("expected local 'local-tmpl'")
	}
}

func TestGetEffectiveTemplates_ChildOverridesParent(t *testing.T) {
	root := NewGraphNode("root", NodeTypeRoot, "root")
	root.SetTemplate("tmpl", "parent-value")

	child := NewGraphNode("child", NodeTypeConfig, "cfg")
	child.SetTemplate("tmpl", "child-value")
	root.AddChild(child)

	eff := child.GetEffectiveTemplates()
	if eff["tmpl"] != "child-value" {
		t.Errorf("child should override parent template, expected 'child-value' got %v", eff["tmpl"])
	}
}

// --- GetEffectiveVariables ---

func TestGetEffectiveVariables_Inheritance(t *testing.T) {
	root := NewGraphNode("root", NodeTypeRoot, "root")
	root.SetVariable("env", "production")

	child := NewGraphNode("child", NodeTypeRepository, "r")
	child.SetVariable("version", "1.0")
	root.AddChild(child)

	eff := child.GetEffectiveVariables()
	if eff["env"] != "production" {
		t.Error("expected inherited variable 'env'")
	}
	if eff["version"] != "1.0" {
		t.Error("expected local variable 'version'")
	}
}

func TestGetEffectiveVariables_ChildOverridesParent(t *testing.T) {
	root := NewGraphNode("root", NodeTypeRoot, "root")
	root.SetVariable("key", "parent")

	child := NewGraphNode("child", NodeTypeRepository, "r")
	child.SetVariable("key", "child")
	root.AddChild(child)

	eff := child.GetEffectiveVariables()
	if eff["key"] != "child" {
		t.Errorf("expected 'child', got %v", eff["key"])
	}
}

// --- NewRelationship ---

func TestNewRelationship(t *testing.T) {
	from := NewGraphNode("from", NodeTypeConfig, "c1")
	to := NewGraphNode("to", NodeTypeRepository, "r1")

	rel := NewRelationship("rel1", from, to, RelationDefines)

	if rel.ID != "rel1" {
		t.Errorf("expected ID 'rel1', got %q", rel.ID)
	}
	if rel.From != from || rel.To != to {
		t.Error("From/To pointers not set correctly")
	}
	if rel.FromID != "from" || rel.ToID != "to" {
		t.Errorf("expected FromID 'from' ToID 'to', got %q %q", rel.FromID, rel.ToID)
	}
	if rel.Type != RelationDefines {
		t.Errorf("expected type %q, got %q", RelationDefines, rel.Type)
	}
	if rel.Properties == nil {
		t.Error("Properties map should be initialized")
	}
}

// --- NewRepositoryGraph ---

func TestNewRepositoryGraph_Initialized(t *testing.T) {
	g := NewRepositoryGraph()

	if g.Nodes == nil || g.Relationships == nil {
		t.Error("Nodes and Relationships maps must be initialized")
	}
	if g.NodesByType == nil || g.NodesByLevel == nil || g.NodesByPath == nil || g.NodesByTag == nil {
		t.Error("index maps must be initialized")
	}
	if g.RelationsByType == nil || g.RelationsByFrom == nil || g.RelationsByTo == nil {
		t.Error("relationship index maps must be initialized")
	}
	if g.AllRepositories == nil || g.AllGroups == nil {
		t.Error("cache maps must be initialized")
	}
}

// --- String ---

func TestGraphNode_String(t *testing.T) {
	n := NewGraphNode("n", NodeTypeRepository, "myrepo")
	s := n.String()
	if s == "" {
		t.Error("String() should not be empty")
	}
}

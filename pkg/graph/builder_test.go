package graph

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- helpers ---

func writeYAML(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("writeYAML: %v", err)
	}
	return p
}

const minimalYAML = `
version: "1.0"
global:
  basePath: /tmp/repos
  workers: 5
  timeout: 10s
repositories:
  - name: repo1
    path: org/repo1
    url: https://github.com/example/repo1.git
`

// --- BuildGraph ---

func TestBuildGraph_SingleConfig_CreatesNodes(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, "gorepos.yaml", minimalYAML)

	g, err := NewGraphBuilder().BuildGraph(path)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	// Should have exactly one repository node
	repos := g.GetNodesByType(NodeTypeRepository)
	if len(repos) != 1 {
		t.Errorf("expected 1 repository node, got %d", len(repos))
	}
	if repos[0].Name != "repo1" {
		t.Errorf("expected repo name 'repo1', got %q", repos[0].Name)
	}

	// Should have a config node
	cfgs := g.GetNodesByType(NodeTypeConfig)
	if len(cfgs) == 0 {
		t.Error("expected at least one config node")
	}
}

func TestBuildGraph_MultipleRepos_AllPresent(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, "gorepos.yaml", `
version: "1.0"
global:
  basePath: /tmp/repos
  workers: 2
  timeout: 5s
repositories:
  - name: alpha
    path: org/alpha
    url: https://github.com/example/alpha.git
  - name: beta
    path: org/beta
    url: https://github.com/example/beta.git
  - name: gamma
    path: org/gamma
    url: https://github.com/example/gamma.git
`)

	g, err := NewGraphBuilder().BuildGraph(path)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	repos := g.GetNodesByType(NodeTypeRepository)
	if len(repos) != 3 {
		t.Errorf("expected 3 repository nodes, got %d", len(repos))
	}
}

func TestBuildGraph_WithGroups_GroupNodesPresent(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, "gorepos.yaml", `
version: "1.0"
global:
  basePath: /tmp/repos
  workers: 2
  timeout: 5s
repositories:
  - name: repo1
    path: org/repo1
    url: https://github.com/example/repo1.git
  - name: repo2
    path: org/repo2
    url: https://github.com/example/repo2.git
groups:
  backend:
    - repo1
    - repo2
`)

	g, err := NewGraphBuilder().BuildGraph(path)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	groups := g.GetNodesByType(NodeTypeGroup)
	if len(groups) == 0 {
		t.Error("expected at least one group node")
	}

	found := false
	for _, grp := range groups {
		if grp.Name == "backend" {
			found = true
		}
	}
	if !found {
		t.Error("expected group node named 'backend'")
	}
}

func TestBuildGraph_LocalInclude_BothConfigsLoaded(t *testing.T) {
	dir := t.TempDir()

	included := writeYAML(t, dir, "included.yaml", `
version: "1.0"
global:
  basePath: /tmp/repos
  workers: 2
  timeout: 5s
repositories:
  - name: included-repo
    path: org/included-repo
    url: https://github.com/example/included-repo.git
`)

	root := writeYAML(t, dir, "gorepos.yaml", `
version: "1.0"
global:
  basePath: /tmp/repos
  workers: 5
  timeout: 10s
includes:
  - path: included.yaml
repositories:
  - name: root-repo
    path: org/root-repo
    url: https://github.com/example/root-repo.git
`)
	_ = included

	g, err := NewGraphBuilder().BuildGraph(root)
	if err != nil {
		t.Fatalf("BuildGraph with include: %v", err)
	}

	repos := g.GetNodesByType(NodeTypeRepository)
	if len(repos) != 2 {
		t.Errorf("expected 2 repositories (root + included), got %d", len(repos))
	}
}

func TestBuildGraph_CircularInclude_ReturnsError(t *testing.T) {
	dir := t.TempDir()

	// a.yaml includes b.yaml which includes a.yaml
	writeYAML(t, dir, "b.yaml", `
version: "1.0"
global:
  basePath: /tmp/repos
  workers: 2
  timeout: 5s
includes:
  - path: a.yaml
`)
	writeYAML(t, dir, "a.yaml", `
version: "1.0"
global:
  basePath: /tmp/repos
  workers: 2
  timeout: 5s
includes:
  - path: b.yaml
`)

	_, err := NewGraphBuilder().BuildGraph(filepath.Join(dir, "a.yaml"))
	if err == nil {
		t.Error("expected error for circular include, got nil")
	}
}

func TestBuildGraph_MissingFile_ReturnsError(t *testing.T) {
	_, err := NewGraphBuilder().BuildGraph("/nonexistent/path/gorepos.yaml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

// --- GetMergedConfig ---

func TestGetMergedConfig_ReturnsCorrectVersion(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, "gorepos.yaml", minimalYAML)

	g, err := NewGraphBuilder().BuildGraph(path)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	cfg := g.GetMergedConfig()
	if cfg.Version != "1.0" {
		t.Errorf("expected version '1.0', got %q", cfg.Version)
	}
}

func TestGetMergedConfig_ReturnsGlobalSettings(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, "gorepos.yaml", `
version: "1.0"
global:
  basePath: /srv/repos
  workers: 8
  timeout: 2m
repositories:
  - name: repo1
    path: org/repo1
    url: https://github.com/example/repo1.git
`)

	g, err := NewGraphBuilder().BuildGraph(path)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	cfg := g.GetMergedConfig()
	if cfg.Global.BasePath != "/srv/repos" {
		t.Errorf("expected basePath '/srv/repos', got %q", cfg.Global.BasePath)
	}
	if cfg.Global.Workers != 8 {
		t.Errorf("expected workers=8, got %d", cfg.Global.Workers)
	}
	if cfg.Global.Timeout != 2*time.Minute {
		t.Errorf("expected timeout=2m, got %v", cfg.Global.Timeout)
	}
}

func TestGetMergedConfig_GlobalTagsMerged(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, "gorepos.yaml", `
version: "1.0"
global:
  basePath: /tmp/repos
  workers: 2
  timeout: 5s
  tags:
    env: production
    team: platform
repositories:
  - name: repo1
    path: org/repo1
    url: https://github.com/example/repo1.git
`)

	g, err := NewGraphBuilder().BuildGraph(path)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	cfg := g.GetMergedConfig()
	if cfg.Global.Tags == nil {
		t.Fatal("expected global tags to be populated, got nil")
	}
	if cfg.Global.Tags["env"] != "production" {
		t.Errorf("expected tag env=production, got %v", cfg.Global.Tags["env"])
	}
	if cfg.Global.Tags["team"] != "platform" {
		t.Errorf("expected tag team=platform, got %v", cfg.Global.Tags["team"])
	}
}

func TestGetMergedConfig_GlobalLabelsMerged(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, "gorepos.yaml", `
version: "1.0"
global:
  basePath: /tmp/repos
  workers: 2
  timeout: 5s
  labels:
    - managed
    - critical
repositories:
  - name: repo1
    path: org/repo1
    url: https://github.com/example/repo1.git
`)

	g, err := NewGraphBuilder().BuildGraph(path)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	cfg := g.GetMergedConfig()
	if len(cfg.Global.Labels) != 2 {
		t.Errorf("expected 2 global labels, got %d: %v", len(cfg.Global.Labels), cfg.Global.Labels)
	}
}

func TestGetMergedConfig_ReposAreSorted(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, "gorepos.yaml", `
version: "1.0"
global:
  basePath: /tmp/repos
  workers: 2
  timeout: 5s
repositories:
  - name: zebra
    path: org/zebra
    url: https://github.com/example/zebra.git
  - name: alpha
    path: org/alpha
    url: https://github.com/example/alpha.git
  - name: mango
    path: org/mango
    url: https://github.com/example/mango.git
`)

	g, err := NewGraphBuilder().BuildGraph(path)
	if err != nil {
		t.Fatalf("BuildGraph: %v", err)
	}

	cfg := g.GetMergedConfig()
	names := make([]string, len(cfg.Repositories))
	for i, r := range cfg.Repositories {
		names[i] = r.Name
	}
	expected := []string{"alpha", "mango", "zebra"}
	for i, want := range expected {
		if i >= len(names) || names[i] != want {
			t.Errorf("expected sorted repos %v, got %v", expected, names)
			break
		}
	}
}

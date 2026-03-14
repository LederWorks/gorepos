package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LederWorks/gorepos/pkg/types"
)

// --- helpers ---

func newLoader() *Loader {
	return NewLoader()
}

// validConfig returns a minimal valid config.
func validConfig() *types.Config {
	return &types.Config{
		Version: "1.0",
		Global: types.GlobalConfig{
			BasePath: "/tmp/repos",
			Workers:  5,
			Timeout:  10 * time.Second,
		},
		Repositories: []types.Repository{
			{
				Name: "repo1",
				Path: "repo1",
				URL:  "https://github.com/example/repo1.git",
			},
		},
	}
}

// writeYAML creates a temp YAML file with the given content.
func writeYAML(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("writeYAML: %v", err)
	}
	return p
}

// --- ValidateConfig ---

func TestValidateConfig_Valid(t *testing.T) {
	l := newLoader()
	if err := l.ValidateConfig(validConfig()); err != nil {
		t.Errorf("expected valid config, got: %v", err)
	}
}

func TestValidateConfig_Nil(t *testing.T) {
	l := newLoader()
	if err := l.ValidateConfig(nil); err == nil {
		t.Error("expected error for nil config")
	}
}

func TestValidateConfig_MissingVersion(t *testing.T) {
	l := newLoader()
	c := validConfig()
	c.Version = ""
	if err := l.ValidateConfig(c); err == nil {
		t.Error("expected error for missing version")
	}
}

func TestValidateConfig_WorkersTooLow(t *testing.T) {
	l := newLoader()
	c := validConfig()
	c.Global.Workers = 0
	if err := l.ValidateConfig(c); err == nil {
		t.Error("expected error for workers < 1")
	}
}

func TestValidateConfig_WorkersTooHigh(t *testing.T) {
	l := newLoader()
	c := validConfig()
	c.Global.Workers = 101
	if err := l.ValidateConfig(c); err == nil {
		t.Error("expected error for workers > 100")
	}
}

func TestValidateConfig_TimeoutTooShort(t *testing.T) {
	l := newLoader()
	c := validConfig()
	c.Global.Timeout = 500 * time.Millisecond
	if err := l.ValidateConfig(c); err == nil {
		t.Error("expected error for timeout < 1s")
	}
}

func TestValidateConfig_NoRepositories(t *testing.T) {
	l := newLoader()
	c := validConfig()
	c.Repositories = []types.Repository{}
	if err := l.ValidateConfig(c); err == nil {
		t.Error("expected error for no repositories")
	}
}

func TestValidateConfig_RepoMissingName(t *testing.T) {
	l := newLoader()
	c := validConfig()
	c.Repositories[0].Name = ""
	if err := l.ValidateConfig(c); err == nil {
		t.Error("expected error for repo with no name")
	}
}

func TestValidateConfig_DuplicateRepoName(t *testing.T) {
	l := newLoader()
	c := validConfig()
	c.Repositories = append(c.Repositories, types.Repository{
		Name: "repo1",
		Path: "repo2",
		URL:  "https://github.com/example/repo2.git",
	})
	if err := l.ValidateConfig(c); err == nil {
		t.Error("expected error for duplicate repo names")
	}
}

func TestValidateConfig_RepoMissingPath(t *testing.T) {
	l := newLoader()
	c := validConfig()
	c.Repositories[0].Path = ""
	if err := l.ValidateConfig(c); err == nil {
		t.Error("expected error for repo with no path")
	}
}

func TestValidateConfig_RepoMissingURL(t *testing.T) {
	l := newLoader()
	c := validConfig()
	c.Repositories[0].URL = ""
	if err := l.ValidateConfig(c); err == nil {
		t.Error("expected error for repo with no URL")
	}
}

func TestValidateConfig_RelativePathWithoutBasePath(t *testing.T) {
	l := newLoader()
	c := validConfig()
	c.Global.BasePath = ""
	c.Repositories[0].Path = "relative/path" // relative, no basePath
	if err := l.ValidateConfig(c); err == nil {
		t.Error("expected error for relative path without basePath")
	}
}

func TestValidateConfig_AbsolutePathWithoutBasePath(t *testing.T) {
	l := newLoader()
	c := validConfig()
	c.Global.BasePath = ""
	c.Repositories[0].Path = "/absolute/path"
	if err := l.ValidateConfig(c); err != nil {
		t.Errorf("absolute path without basePath should be valid, got: %v", err)
	}
}

// --- setDefaults ---

func TestSetDefaults_FillsMissingFields(t *testing.T) {
	l := newLoader()
	c := &types.Config{}
	c.Repositories = []types.Repository{{Name: "r", Path: "/p", URL: "u"}}
	l.setDefaults(c)

	if c.Version != "1.0" {
		t.Errorf("expected version '1.0', got %q", c.Version)
	}
	if c.Global.Workers != 10 {
		t.Errorf("expected 10 workers, got %d", c.Global.Workers)
	}
	if c.Global.Timeout != 5*time.Minute {
		t.Errorf("expected 5m timeout, got %v", c.Global.Timeout)
	}
	if c.Repositories[0].Branch != "main" {
		t.Errorf("expected branch 'main', got %q", c.Repositories[0].Branch)
	}
}

func TestSetDefaults_DoesNotOverrideExisting(t *testing.T) {
	l := newLoader()
	c := &types.Config{
		Version: "2.0",
		Global: types.GlobalConfig{
			Workers: 20,
			Timeout: 2 * time.Minute,
		},
		Repositories: []types.Repository{
			{Name: "r", Path: "/p", URL: "u", Branch: "develop"},
		},
	}
	l.setDefaults(c)

	if c.Version != "2.0" {
		t.Errorf("should not override version, got %q", c.Version)
	}
	if c.Global.Workers != 20 {
		t.Errorf("should not override workers, got %d", c.Global.Workers)
	}
	if c.Repositories[0].Branch != "develop" {
		t.Errorf("should not override branch, got %q", c.Repositories[0].Branch)
	}
}

// --- mergeConfigs ---

func TestMergeConfigs_MainTakesPrecedenceForGlobal(t *testing.T) {
	l := newLoader()

	main := &types.Config{
		Global: types.GlobalConfig{
			BasePath: "/main",
			Workers:  5,
			Timeout:  10 * time.Second,
		},
	}
	included := &types.Config{
		Global: types.GlobalConfig{
			BasePath: "/included",
			Workers:  20,
			Timeout:  30 * time.Second,
		},
	}

	result := l.mergeConfigs(main, included)

	if result.Global.BasePath != "/main" {
		t.Errorf("expected main basePath '/main', got %q", result.Global.BasePath)
	}
	if result.Global.Workers != 5 {
		t.Errorf("expected main workers 5, got %d", result.Global.Workers)
	}
}

func TestMergeConfigs_IncludedFillsMissingGlobal(t *testing.T) {
	l := newLoader()

	main := &types.Config{
		Global: types.GlobalConfig{
			Workers: 5,
			Timeout: 10 * time.Second,
		},
	}
	included := &types.Config{
		Global: types.GlobalConfig{
			BasePath: "/from-included",
		},
	}

	result := l.mergeConfigs(main, included)

	if result.Global.BasePath != "/from-included" {
		t.Errorf("expected basePath from included when main is empty, got %q", result.Global.BasePath)
	}
}

func TestMergeConfigs_ReposMainOverridesIncluded(t *testing.T) {
	l := newLoader()

	main := &types.Config{
		Repositories: []types.Repository{
			{Name: "repo1", Path: "/main/repo1", URL: "https://main.git"},
		},
	}
	included := &types.Config{
		Repositories: []types.Repository{
			{Name: "repo1", Path: "/included/repo1", URL: "https://included.git"},
			{Name: "repo2", Path: "/included/repo2", URL: "https://included2.git"},
		},
	}

	result := l.mergeConfigs(main, included)

	// Find repo1
	var r1path string
	for _, r := range result.Repositories {
		if r.Name == "repo1" {
			r1path = r.Path
		}
	}

	if r1path != "/main/repo1" {
		t.Errorf("main repo should override included repo, got path %q", r1path)
	}

	if len(result.Repositories) != 2 {
		t.Errorf("expected 2 repos after merge, got %d", len(result.Repositories))
	}
}

func TestMergeConfigs_EnvVarsMainTakesPrecedence(t *testing.T) {
	l := newLoader()

	main := &types.Config{
		Global: types.GlobalConfig{
			Environment: map[string]string{"KEY": "main-value", "MAIN_ONLY": "yes"},
		},
	}
	included := &types.Config{
		Global: types.GlobalConfig{
			Environment: map[string]string{"KEY": "included-value", "INCLUDED_ONLY": "yes"},
		},
	}

	result := l.mergeConfigs(main, included)

	if result.Global.Environment["KEY"] != "main-value" {
		t.Errorf("main env var should take precedence, got %q", result.Global.Environment["KEY"])
	}
	if result.Global.Environment["INCLUDED_ONLY"] != "yes" {
		t.Errorf("expected included-only env var to be present")
	}
	if result.Global.Environment["MAIN_ONLY"] != "yes" {
		t.Errorf("expected main-only env var to be present")
	}
}

func TestMergeConfigs_GroupsMainTakesPrecedence(t *testing.T) {
	l := newLoader()

	main := &types.Config{
		Groups: map[string][]string{"g1": {"repo1"}},
	}
	included := &types.Config{
		Groups: map[string][]string{"g1": {"repo2"}, "g2": {"repo3"}},
	}

	result := l.mergeConfigs(main, included)

	if len(result.Groups["g1"]) != 1 || result.Groups["g1"][0] != "repo1" {
		t.Errorf("main group should take precedence, got %v", result.Groups["g1"])
	}
	if _, ok := result.Groups["g2"]; !ok {
		t.Error("included-only group should be present")
	}
}

// --- applyRootGroupInheritance ---

func TestApplyRootGroupInheritance_EmptyGroupGetsAllRepos(t *testing.T) {
	l := newLoader()
	c := &types.Config{
		Groups: map[string][]string{
			"all":  {},                   // empty — should get all repos
			"some": {"repo1"},            // already has repos — should not change
		},
		Repositories: []types.Repository{
			{Name: "repo1"},
			{Name: "repo2"},
		},
	}

	l.applyRootGroupInheritance(c)

	if len(c.Groups["all"]) != 2 {
		t.Errorf("expected 2 repos in empty group 'all', got %d", len(c.Groups["all"]))
	}
	if len(c.Groups["some"]) != 1 {
		t.Errorf("non-empty group 'some' should remain unchanged, got %d repos", len(c.Groups["some"]))
	}
}

func TestApplyRootGroupInheritance_NilGroups(t *testing.T) {
	l := newLoader()
	c := &types.Config{Groups: nil}
	// Should not panic
	l.applyRootGroupInheritance(c)
}

// --- LoadConfigWithDetails (file I/O) ---

func TestLoadConfigWithDetails_ValidFile(t *testing.T) {
	dir := t.TempDir()
	content := `
version: "1.0"
global:
  basePath: "/tmp/repos"
  workers: 4
  timeout: 30s
repositories:
  - name: myrepo
    path: /tmp/repos/myrepo
    url: https://github.com/example/myrepo.git
    branch: main
`
	path := writeYAML(t, dir, "gorepos.yaml", content)

	l := newLoader()
	result, err := l.LoadConfigWithDetails(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Config.Version != "1.0" {
		t.Errorf("expected version '1.0', got %q", result.Config.Version)
	}
	if len(result.Config.Repositories) != 1 {
		t.Errorf("expected 1 repo, got %d", len(result.Config.Repositories))
	}
	if len(result.ProcessedFiles) != 1 {
		t.Errorf("expected 1 processed file, got %d", len(result.ProcessedFiles))
	}
}

func TestLoadConfigWithDetails_EmptyPath(t *testing.T) {
	l := newLoader()
	_, err := l.LoadConfigWithDetails("")
	if err == nil {
		t.Error("expected error for empty path")
	}
}

func TestLoadConfigWithDetails_MissingFile(t *testing.T) {
	l := newLoader()
	_, err := l.LoadConfigWithDetails("/nonexistent/path/gorepos.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadConfigWithDetails_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeYAML(t, dir, "bad.yaml", ":::not valid yaml:::")
	l := newLoader()
	_, err := l.LoadConfigWithDetails(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoadConfigWithDetails_WithIncludes(t *testing.T) {
	dir := t.TempDir()

	included := `
version: "1.0"
global:
  workers: 2
  timeout: 10s
repositories:
  - name: included-repo
    path: /tmp/repos/included-repo
    url: https://github.com/example/included.git
`
	includedPath := writeYAML(t, dir, "included.yaml", included)

	main := `
version: "1.0"
global:
  basePath: "/tmp/repos"
  workers: 5
  timeout: 30s
includes:
  - ` + filepath.Base(includedPath) + `
repositories:
  - name: main-repo
    path: /tmp/repos/main-repo
    url: https://github.com/example/main.git
`
	mainPath := writeYAML(t, dir, "main.yaml", main)

	l := newLoader()
	result, err := l.LoadConfigWithDetails(mainPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Config.Repositories) != 2 {
		t.Errorf("expected 2 repos after include merge, got %d", len(result.Config.Repositories))
	}
	if len(result.ProcessedFiles) != 2 {
		t.Errorf("expected 2 processed files, got %d", len(result.ProcessedFiles))
	}
}

func TestLoadConfigWithDetails_CircularInclude(t *testing.T) {
	dir := t.TempDir()

	// a.yaml includes b.yaml, b.yaml includes a.yaml
	aPath := filepath.Join(dir, "a.yaml")
	bPath := filepath.Join(dir, "b.yaml")

	aContent := `
version: "1.0"
includes:
  - b.yaml
global:
  workers: 1
  timeout: 1s
repositories:
  - name: r1
    path: /tmp/r1
    url: https://github.com/example/r1.git
`
	bContent := `
version: "1.0"
includes:
  - a.yaml
global:
  workers: 1
  timeout: 1s
repositories:
  - name: r2
    path: /tmp/r2
    url: https://github.com/example/r2.git
`

	os.WriteFile(aPath, []byte(aContent), 0644)
	os.WriteFile(bPath, []byte(bContent), 0644)

	l := newLoader()
	_, err := l.LoadConfigWithDetails(aPath)
	if err == nil {
		t.Error("expected error for circular include")
	}
}

// --- LoadRemoteConfig ---

func TestLoadRemoteConfig_EmptyURL(t *testing.T) {
	l := newLoader()
	_, err := l.LoadRemoteConfig("")
	if err == nil {
		t.Error("expected error for empty URL")
	}
}

// --- GetConfigPath ---

func TestGetConfigPath_NotFound(t *testing.T) {
	// Change to a temp directory where no config files exist
	orig, _ := os.Getwd()
	defer os.Chdir(orig)

	dir := t.TempDir()
	os.Chdir(dir)

	_, err := GetConfigPath()
	if err == nil {
		t.Error("expected error when no config file found")
	}
}

func TestGetConfigPath_FoundInCurrentDir(t *testing.T) {
	orig, _ := os.Getwd()
	defer os.Chdir(orig)

	dir := t.TempDir()
	os.Chdir(dir)

	// Create a config file
	os.WriteFile(filepath.Join(dir, "gorepos.yaml"), []byte(""), 0644)

	path, err := GetConfigPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(path) != "gorepos.yaml" {
		t.Errorf("expected gorepos.yaml, got %q", path)
	}
}

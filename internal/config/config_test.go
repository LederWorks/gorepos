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

// --- ValidateConfig: platforms ---

func TestValidateConfig_PlatformsValid(t *testing.T) {
	l := newLoader()
	c := validConfig()
	c.Global.Platforms = []types.PlatformEntry{
		{Hostname: "gitlab.mycompany.com", Type: "gitlab"},
		{Hostname: "github.internal.corp", Type: "github"},
	}
	if err := l.ValidateConfig(c); err != nil {
		t.Errorf("expected valid config with custom platforms, got: %v", err)
	}
}

func TestValidateConfig_PlatformEmptyHostname(t *testing.T) {
	l := newLoader()
	c := validConfig()
	c.Global.Platforms = []types.PlatformEntry{
		{Hostname: "", Type: "gitlab"},
	}
	if err := l.ValidateConfig(c); err == nil {
		t.Error("expected error for platform with empty hostname")
	}
}

func TestValidateConfig_PlatformInvalidType(t *testing.T) {
	l := newLoader()
	c := validConfig()
	c.Global.Platforms = []types.PlatformEntry{
		{Hostname: "git.example.com", Type: "forgejo"},
	}
	if err := l.ValidateConfig(c); err == nil {
		t.Error("expected error for platform with unknown type 'forgejo'")
	}
}

func TestValidateConfig_PlatformAllValidTypes(t *testing.T) {
	l := newLoader()
	for _, typ := range []string{"github", "gitlab", "azure", "bitbucket"} {
		c := validConfig()
		c.Global.Platforms = []types.PlatformEntry{{Hostname: "git.example.com", Type: typ}}
		if err := l.ValidateConfig(c); err != nil {
			t.Errorf("type %q should be valid, got: %v", typ, err)
		}
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

	_ = os.WriteFile(aPath, []byte(aContent), 0644)
	_ = os.WriteFile(bPath, []byte(bContent), 0644)

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
	defer func() { _ = os.Chdir(orig) }()

	dir := t.TempDir()
	_ = os.Chdir(dir)

	_, err := GetConfigPath()
	if err == nil {
		t.Error("expected error when no config file found")
	}
}

func TestGetConfigPath_FoundInCurrentDir(t *testing.T) {
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()

	dir := t.TempDir()
	_ = os.Chdir(dir)

	// Create a config file
	_ = os.WriteFile(filepath.Join(dir, "gorepos.yaml"), []byte(""), 0644)

	path, err := GetConfigPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Base(path) != "gorepos.yaml" {
		t.Errorf("expected gorepos.yaml, got %q", path)
	}
}

// --- Include-level identity ---

func TestValidateConfig_IncludeUserEmailRequiresRepo(t *testing.T) {
	l := newLoader()
	c := validConfig()
	c.Includes = []types.IncludeEntry{
		{Path: "./local.yaml", User: "Jane", Email: "jane@co.com"},
	}
	err := l.ValidateConfig(c)
	if err == nil {
		t.Error("expected error for user/email on local include")
	}
}

func TestValidateConfig_IncludeUserEmailOnRepoOK(t *testing.T) {
	l := newLoader()
	c := validConfig()
	c.Includes = []types.IncludeEntry{
		{Repo: "https://github.com/org/repo", User: "Jane", Email: "jane@co.com"},
	}
	err := l.ValidateConfig(c)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCollectIdentityWarnings_NoWarningWithGlobalCreds(t *testing.T) {
	c := validConfig()
	c.Global.Credentials = &types.CredentialConfig{
		GitUserName:  "Jane",
		GitUserEmail: "jane@co.com",
	}
	c.Includes = []types.IncludeEntry{
		{Repo: "https://github.com/org/repo"},
	}
	warnings := CollectIdentityWarnings(c)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings with global credentials, got %v", warnings)
	}
}

func TestCollectIdentityWarnings_NoWarningWithIncludeIdentity(t *testing.T) {
	c := validConfig()
	c.Includes = []types.IncludeEntry{
		{Repo: "https://github.com/org/repo", User: "Jane", Email: "jane@co.com"},
	}
	warnings := CollectIdentityWarnings(c)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings with include identity, got %v", warnings)
	}
}

func TestCollectIdentityWarnings_WarnsWhenNoIdentity(t *testing.T) {
	c := validConfig()
	c.Includes = []types.IncludeEntry{
		{Repo: "https://github.com/org/repo"},
	}
	warnings := CollectIdentityWarnings(c)
	if len(warnings) != 1 {
		t.Errorf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}
}

func TestCollectIdentityWarnings_NoWarningForLocalInclude(t *testing.T) {
	c := validConfig()
	c.Includes = []types.IncludeEntry{
		{Path: "./local.yaml"},
	}
	warnings := CollectIdentityWarnings(c)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for local include, got %v", warnings)
	}
}

// --- ApplyIncludeIdentity ---

func TestApplyIncludeIdentity_StampsReposWithoutOwnIdentity(t *testing.T) {
	c := &types.Config{
		Repositories: []types.Repository{
			{Name: "r1", Path: "r1", URL: "https://x.git"},
			{Name: "r2", Path: "r2", URL: "https://y.git", User: "Existing", Email: "existing@co.com"},
		},
	}
	c.ApplyIncludeIdentity("Jane", "jane@co.com")

	if c.Repositories[0].User != "Jane" || c.Repositories[0].Email != "jane@co.com" {
		t.Errorf("expected include identity on r1, got user=%q email=%q", c.Repositories[0].User, c.Repositories[0].Email)
	}
	if c.Repositories[1].User != "Existing" || c.Repositories[1].Email != "existing@co.com" {
		t.Errorf("repo-level identity should take precedence, got user=%q email=%q", c.Repositories[1].User, c.Repositories[1].Email)
	}
}

func TestApplyIncludeIdentity_NoOpWhenEmpty(t *testing.T) {
	c := &types.Config{
		Repositories: []types.Repository{
			{Name: "r1", Path: "r1", URL: "https://x.git"},
		},
	}
	c.ApplyIncludeIdentity("", "")

	if c.Repositories[0].User != "" || c.Repositories[0].Email != "" {
		t.Error("should not stamp empty identity")
	}
}

// --- buildIncludesSequenceNode ---

func TestBuildIncludesSequenceNode_WithUserEmail(t *testing.T) {
	includes := []types.IncludeEntry{
		{Repo: "https://github.com/org/repo", User: "Jane", Email: "jane@co.com"},
		{Path: "./local.yaml"},
	}
	node := buildIncludesSequenceNode(includes)

	// First entry should be a mapping with user and email
	if len(node.Content) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(node.Content))
	}

	mapNode := node.Content[0]
	// Count the key-value pairs: repo, user, email = 3 pairs = 6 nodes
	if len(mapNode.Content) != 6 {
		t.Errorf("expected 6 content nodes (repo+user+email), got %d", len(mapNode.Content))
	}

	// Check user and email keys are present
	foundUser, foundEmail := false, false
	for i := 0; i < len(mapNode.Content)-1; i += 2 {
		switch mapNode.Content[i].Value {
		case "user":
			foundUser = true
			if mapNode.Content[i+1].Value != "Jane" {
				t.Errorf("expected user=Jane, got %q", mapNode.Content[i+1].Value)
			}
		case "email":
			foundEmail = true
			if mapNode.Content[i+1].Value != "jane@co.com" {
				t.Errorf("expected email=jane@co.com, got %q", mapNode.Content[i+1].Value)
			}
		}
	}
	if !foundUser || !foundEmail {
		t.Errorf("expected user and email keys, foundUser=%v foundEmail=%v", foundUser, foundEmail)
	}
}

// --- IncludeEntry.String() with identity ---

func TestIncludeEntryString_WithUserAndEmail(t *testing.T) {
	e := types.IncludeEntry{
		Repo:  "https://github.com/org/repo",
		Ref:   "main",
		User:  "Jane",
		Email: "jane@co.com",
	}
	s := e.String()
	if s != "https://github.com/org/repo (ref: main) (identity: Jane <jane@co.com>)" {
		t.Errorf("unexpected String(): %q", s)
	}
}

func TestIncludeEntryString_WithUserOnly(t *testing.T) {
	e := types.IncludeEntry{
		Repo: "https://github.com/org/repo",
		User: "Jane",
	}
	s := e.String()
	if s != "https://github.com/org/repo (identity: Jane)" {
		t.Errorf("unexpected String(): %q", s)
	}
}

func TestIncludeEntryString_WithEmailOnly(t *testing.T) {
	e := types.IncludeEntry{
		Repo:  "https://github.com/org/repo",
		Email: "jane@co.com",
	}
	s := e.String()
	if s != "https://github.com/org/repo (identity: <jane@co.com>)" {
		t.Errorf("unexpected String(): %q", s)
	}
}

// --- looksLikeCommitHash ---

func TestLooksLikeCommitHash(t *testing.T) {
	tests := []struct {
		ref    string
		expect bool
	}{
		// Short hashes must return false — platforms cannot fetch commits by abbreviated SHA.
		{"abc123f", false},
		{"ABCDEF1", false},
		{"abc12", false}, // too short
		// Exactly 40 hex chars is the only accepted format.
		{"a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", true},
		{"A1B2C3D4E5F6A1B2C3D4E5F6A1B2C3D4E5F6A1B2", true}, // uppercase hex
		// Non-hex and ref-like strings must return false.
		{"main", false},
		{"v1.0.0", false},
		{"refs/tags/v1.0.0", false},
		{"", false},
	}
	for _, tt := range tests {
		got := looksLikeCommitHash(tt.ref)
		if got != tt.expect {
			t.Errorf("looksLikeCommitHash(%q) = %v, want %v", tt.ref, got, tt.expect)
		}
	}
}

// --- validateUserName tests ---

func TestValidateUserName_AcceptsNormalName(t *testing.T) {
	if err := validateUserName("Jane Doe"); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateUserName_AcceptsUnicode(t *testing.T) {
	if err := validateUserName("Bence Bánó"); err != nil {
		t.Errorf("expected no error for unicode name, got: %v", err)
	}
}

func TestValidateUserName_RejectsGitCommand(t *testing.T) {
	err := validateUserName(`git config user.name "Bence Bánó"`)
	if err == nil {
		t.Error("expected error for pasted git command")
	}
}

func TestValidateUserName_RejectsGitCommandCaseInsensitive(t *testing.T) {
	err := validateUserName(`Git config user.name "X"`)
	if err == nil {
		t.Error("expected error for pasted git command (uppercase)")
	}
}

func TestValidateUserName_RejectsShellChars(t *testing.T) {
	for _, ch := range []string{`"`, "'", "$", ";", "|", "`", "\\"} {
		err := validateUserName("test" + ch + "name")
		if err == nil {
			t.Errorf("expected error for char %q", ch)
		}
	}
}

func TestValidateUserName_RejectsTooLong(t *testing.T) {
	long := ""
	for i := 0; i < 101; i++ {
		long += "a"
	}
	if err := validateUserName(long); err == nil {
		t.Error("expected error for 101-char name")
	}
}

// --- validateEmail tests ---

func TestValidateEmail_AcceptsValid(t *testing.T) {
	valid := []string{
		"jane@example.com",
		"user.name+tag@domain.org",
		"test@sub.domain.co.uk",
	}
	for _, e := range valid {
		if err := validateEmail(e); err != nil {
			t.Errorf("expected %q to be valid, got: %v", e, err)
		}
	}
}

func TestValidateEmail_RejectsMissingAt(t *testing.T) {
	if err := validateEmail("notanemail"); err == nil {
		t.Error("expected error for missing @")
	}
}

func TestValidateEmail_RejectsGitCommand(t *testing.T) {
	err := validateEmail(`git config user.email "x@y.com"`)
	if err == nil {
		t.Error("expected error for pasted git command")
	}
}

func TestValidateEmail_RejectsInvalid(t *testing.T) {
	if err := validateEmail("@nolocal.com"); err == nil {
		t.Error("expected error for missing local part")
	}
}

// --- local directory include tests ---

func TestLocalDirectoryInclude_AppendsGoreposYaml(t *testing.T) {
	// Create a temp dir with a subdirectory containing a gorepos.yaml
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "myconfig")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Write a minimal valid config in the subdirectory
	subConfig := `version: "1.0"
global:
  workers: 2
  timeout: 10s
repositories:
  - name: sub-repo
    path: sub-repo
    url: https://github.com/org/sub-repo
`
	if err := os.WriteFile(filepath.Join(subDir, "gorepos.yaml"), []byte(subConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	// Write a root config that includes the directory path (not the file)
	rootConfig := `version: "1.0"
global:
  basePath: "` + tmpDir + `"
  workers: 5
  timeout: 30s
includes:
  - ` + subDir + `
repositories:
  - name: root-repo
    path: root-repo
    url: https://github.com/org/root-repo
`
	rootFile := filepath.Join(tmpDir, "gorepos.yaml")
	if err := os.WriteFile(rootFile, []byte(rootConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	loader := newLoader()
	result, err := loader.LoadConfigWithDetails(rootFile)
	if err != nil {
		t.Fatalf("expected directory include to work, got: %v", err)
	}
	// Should have both repos merged
	found := false
	for _, r := range result.Config.Repositories {
		if r.Name == "sub-repo" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected sub-repo from directory include to be merged into config")
	}
}

// TestLoadConfigWithDetails_NoWorkersOrTimeout verifies that a config omitting the
// optional workers and timeout fields succeeds via LoadConfigWithDetails.
// Regression test for C-1: setDefaults must run BEFORE ValidateConfig so that the
// validator sees populated values rather than zero values.
func TestLoadConfigWithDetails_NoWorkersOrTimeout(t *testing.T) {
	dir := t.TempDir()
	// Intentionally omit global.workers and global.timeout — they are optional.
	content := `version: "1.0"
global:
  basePath: /tmp/repos
repositories:
  - name: myrepo
    path: myrepo
    url: https://github.com/example/myrepo.git
`
	p := writeYAML(t, dir, "gorepos.yaml", content)

	l := newLoader()
	result, err := l.LoadConfigWithDetails(p)
	if err != nil {
		t.Fatalf("expected success when workers/timeout are omitted, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// setDefaults should have filled in Workers >= 1 and Timeout >= 1s.
	if result.Config.Global.Workers < 1 {
		t.Errorf("expected Workers >= 1 after defaults, got %d", result.Config.Global.Workers)
	}
	if result.Config.Global.Timeout < 1 {
		t.Errorf("expected Timeout >= 1ns after defaults, got %v", result.Config.Global.Timeout)
	}
}

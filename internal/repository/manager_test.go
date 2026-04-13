package repository

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LederWorks/gorepos/pkg/types"
)

// initLocalRepo creates a local git repo with an initial commit and returns its path.
func initLocalRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	run(t, dir, "git", "config", "user.name", "Test")
	// Disable commit signing in test repos (environment may have signing hooks)
	run(t, dir, "git", "config", "commit.gpgsign", "false")
	run(t, dir, "git", "config", "gpg.format", "openpgp")

	// Create initial commit
	f := filepath.Join(dir, "README.md")
	if err := os.WriteFile(f, []byte("hello"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "-c", "commit.gpgsign=false", "commit", "--no-gpg-sign", "-m", "initial")

	return dir
}

// cloneLocalRepo clones src into a new temp dir and returns the clone path.
func cloneLocalRepo(t *testing.T, src string) string {
	t.Helper()
	dest := filepath.Join(t.TempDir(), "clone")
	run(t, "", "git", "clone", src, dest)
	return dest
}

// run runs a command in dir (empty means current dir), failing the test on error.
func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run %s %v in %q: %v\n%s", name, args, dir, err, out)
	}
}

// --- NewManager ---

func TestNewManager(t *testing.T) {
	m := NewManager("/base")
	if m.basePath != "/base" {
		t.Errorf("expected basePath '/base', got %q", m.basePath)
	}
}

// --- getRepoPath ---

func TestGetRepoPath_AbsolutePath(t *testing.T) {
	// Absolute path with no basePath: no containment check, should succeed.
	m := NewManager("")
	repo := &types.Repository{Path: "/absolute/path"}
	got, err := m.getRepoPath(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/absolute/path" {
		t.Errorf("expected '/absolute/path', got %q", got)
	}
}

func TestGetRepoPath_AbsolutePathEscapesBase(t *testing.T) {
	// Absolute path that does not lie inside basePath must be rejected (SEC-C2).
	m := NewManager("/base")
	repo := &types.Repository{Path: "/absolute/path"}
	_, err := m.getRepoPath(repo)
	if err == nil {
		t.Error("expected error when absolute path escapes basePath")
	}
}

func TestGetRepoPath_RelativeWithBasePath(t *testing.T) {
	m := NewManager("/base")
	repo := &types.Repository{Path: "relative"}
	expected := "/base/relative"
	got, err := m.getRepoPath(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestGetRepoPath_RelativeWithoutBasePath(t *testing.T) {
	m := NewManager("")
	repo := &types.Repository{Path: "relative"}
	got, err := m.getRepoPath(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "relative" {
		t.Errorf("expected 'relative', got %q", got)
	}
}

func TestGetRepoPath_TraversalBlocked(t *testing.T) {
	// Path traversal via ../ must be rejected when basePath is set (SEC-C2).
	m := NewManager("/base")
	repo := &types.Repository{Path: "../../etc/passwd"}
	_, err := m.getRepoPath(repo)
	if err == nil {
		t.Error("expected error for path traversal attempt")
	}
}

// --- Exists ---

func TestExists_ExistingRepo(t *testing.T) {
	src := initLocalRepo(t)
	dest := cloneLocalRepo(t, src)

	m := NewManager("")
	repo := &types.Repository{Path: dest}
	if !m.Exists(repo) {
		t.Error("expected Exists to return true for a valid git repo")
	}
}

func TestExists_MissingRepo(t *testing.T) {
	m := NewManager("")
	repo := &types.Repository{Path: "/nonexistent/path/repo"}
	if m.Exists(repo) {
		t.Error("expected Exists to return false for non-existent path")
	}
}

func TestExists_DirectoryWithoutGit(t *testing.T) {
	dir := t.TempDir()
	m := NewManager("")
	repo := &types.Repository{Path: dir}
	if m.Exists(repo) {
		t.Error("expected Exists to return false for dir without .git")
	}
}

// --- Clone ---

func TestClone_Success(t *testing.T) {
	src := initLocalRepo(t)
	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "myclone")

	m := NewManager("")
	repo := &types.Repository{
		Name:   "test",
		Path:   destPath,
		URL:    src,
		Branch: "master",
	}

	if err := m.Clone(context.Background(), repo); err != nil {
		t.Fatalf("Clone failed: %v", err)
	}

	if !m.Exists(repo) {
		t.Error("expected cloned repo to exist")
	}
}

func TestClone_AlreadyExists(t *testing.T) {
	src := initLocalRepo(t)
	dest := cloneLocalRepo(t, src)

	m := NewManager("")
	repo := &types.Repository{
		Name: "test",
		Path: dest,
		URL:  src,
	}

	err := m.Clone(context.Background(), repo)
	if err == nil {
		t.Error("expected error when cloning into existing repo")
	}
}

func TestClone_WithBranch(t *testing.T) {
	src := initLocalRepo(t)
	destDir := t.TempDir()
	destPath := filepath.Join(destDir, "branchclone")

	m := NewManager("")
	repo := &types.Repository{
		Name:   "test",
		Path:   destPath,
		URL:    src,
		Branch: "master",
	}

	if err := m.Clone(context.Background(), repo); err != nil {
		t.Fatalf("Clone with branch failed: %v", err)
	}
	if !m.Exists(repo) {
		t.Error("expected cloned repo to exist")
	}
}

func TestClone_InvalidURL(t *testing.T) {
	destDir := t.TempDir()
	m := NewManager("")
	repo := &types.Repository{
		Name: "test",
		Path: filepath.Join(destDir, "repo"),
		URL:  "/nonexistent/url",
	}

	if err := m.Clone(context.Background(), repo); err == nil {
		t.Error("expected error for invalid URL")
	}
}

// --- Status ---

func TestStatus_CleanRepo(t *testing.T) {
	src := initLocalRepo(t)
	dest := cloneLocalRepo(t, src)

	m := NewManager("")
	repo := &types.Repository{
		Name:   "test",
		Path:   dest,
		Branch: "master",
	}

	status, err := m.Status(context.Background(), repo)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if !status.IsClean {
		t.Error("expected clean repo")
	}
	if status.Path != dest {
		t.Errorf("expected path %q, got %q", dest, status.Path)
	}
}

func TestStatus_DirtyRepo(t *testing.T) {
	src := initLocalRepo(t)
	dest := cloneLocalRepo(t, src)

	// Create an uncommitted file
	if err := os.WriteFile(filepath.Join(dest, "dirty.txt"), []byte("uncommitted"), 0644); err != nil {
		t.Fatalf("write dirty file: %v", err)
	}
	run(t, dest, "git", "add", "dirty.txt")

	m := NewManager("")
	repo := &types.Repository{
		Name:   "test",
		Path:   dest,
		Branch: "master",
	}

	status, err := m.Status(context.Background(), repo)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.IsClean {
		t.Error("expected dirty repo")
	}
}

func TestStatus_NonExistentRepo(t *testing.T) {
	m := NewManager("")
	repo := &types.Repository{
		Name: "test",
		Path: "/nonexistent/repo",
	}
	_, err := m.Status(context.Background(), repo)
	if err == nil {
		t.Error("expected error for non-existent repo")
	}
}

func TestStatus_HasCurrentBranch(t *testing.T) {
	src := initLocalRepo(t)
	dest := cloneLocalRepo(t, src)

	m := NewManager("")
	repo := &types.Repository{
		Name:   "test",
		Path:   dest,
		Branch: "master",
	}

	status, err := m.Status(context.Background(), repo)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.CurrentBranch == "" {
		t.Error("expected non-empty current branch")
	}
}

// --- Update ---

func TestUpdate_NonExistentRepo(t *testing.T) {
	m := NewManager("")
	repo := &types.Repository{
		Name: "test",
		Path: "/nonexistent/repo",
	}
	if err := m.Update(context.Background(), repo); err == nil {
		t.Error("expected error for non-existent repo")
	}
}

func TestUpdate_DirtyRepo(t *testing.T) {
	src := initLocalRepo(t)
	dest := cloneLocalRepo(t, src)

	// Make the repo dirty
	if err := os.WriteFile(filepath.Join(dest, "dirty.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("write dirty: %v", err)
	}
	run(t, dest, "git", "add", "dirty.txt")

	m := NewManager("")
	repo := &types.Repository{
		Name:   "test",
		Path:   dest,
		Branch: "master",
	}

	if err := m.Update(context.Background(), repo); err == nil {
		t.Error("expected error updating dirty repo")
	}
}

func TestUpdate_CleanRepo(t *testing.T) {
	src := initLocalRepo(t)
	dest := cloneLocalRepo(t, src)

	// Configure git user in clone for potential commits
	run(t, dest, "git", "config", "user.email", "test@test.com")
	run(t, dest, "git", "config", "user.name", "Test")

	m := NewManager("")
	repo := &types.Repository{
		Name:   "test",
		Path:   dest,
		URL:    src,
		Branch: "master",
	}

	if err := m.Update(context.Background(), repo); err != nil {
		t.Fatalf("Update failed on clean repo: %v", err)
	}
}

// --- Execute ---

func TestExecute_Success(t *testing.T) {
	src := initLocalRepo(t)
	dest := cloneLocalRepo(t, src)

	m := NewManager("")
	repo := &types.Repository{
		Name: "test",
		Path: dest,
	}

	result, err := m.Execute(context.Background(), repo, "git", "status")
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}
	if result.Output == "" {
		t.Error("expected non-empty output from git status")
	}
	if result.Duration <= 0 {
		t.Error("expected positive duration")
	}
}

func TestExecute_NonExistentRepo(t *testing.T) {
	m := NewManager("")
	repo := &types.Repository{
		Name: "test",
		Path: "/nonexistent",
	}

	result, err := m.Execute(context.Background(), repo, "git", "status")
	if err == nil {
		t.Error("expected error for non-existent repo")
	}
	if result.Success {
		t.Error("expected failure result")
	}
}

func TestExecute_FailingCommand(t *testing.T) {
	src := initLocalRepo(t)
	dest := cloneLocalRepo(t, src)

	m := NewManager("")
	repo := &types.Repository{Name: "test", Path: dest}

	result, _ := m.Execute(context.Background(), repo, "git", "invalid-git-subcommand-xyz")
	if result.Success {
		t.Error("expected failure for invalid git subcommand")
	}
}

// --- buildEnvironment ---

func TestBuildEnvironment_IncludesRepoEnv(t *testing.T) {
	m := NewManager("")
	repo := &types.Repository{
		Name: "test",
		Path: "/tmp/test",
		Environment: map[string]string{
			"MY_CUSTOM_VAR": "custom-value",
		},
	}

	env := m.buildEnvironment(repo)

	found := false
	for _, e := range env {
		if e == "MY_CUSTOM_VAR=custom-value" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected MY_CUSTOM_VAR in environment")
	}
}

func TestBuildEnvironment_InheritsOSEnv(t *testing.T) {
	m := NewManager("")
	repo := &types.Repository{Name: "test", Path: "/tmp/test"}

	env := m.buildEnvironment(repo)

	// Should have at least the OS environment
	if len(env) == 0 {
		t.Error("expected environment to include OS env vars")
	}
}

func TestBuildEnvironment_BlocksDangerousKeys(t *testing.T) {
	// SEC-C1: blocked env keys must not appear in the subprocess environment.
	m := NewManager("")
	repo := &types.Repository{
		Name: "test",
		Path: "/tmp/test",
		Environment: map[string]string{
			"GIT_SSH_COMMAND":       "evil",
			"GIT_SSH":               "evil",
			"GIT_PROXY_COMMAND":     "evil",
			"GIT_EXEC_PATH":         "evil",
			"GIT_ASKPASS":           "evil",
			"GIT_TEMPLATE_DIR":      "evil",
			"LD_PRELOAD":            "evil",
			"DYLD_INSERT_LIBRARIES": "evil",
			"PATH":                  "evil",
			"GIT_CONFIG_GLOBAL":     "evil",
		},
	}

	env := m.buildEnvironment(repo)

	blockedPrefixes := []string{
		"GIT_SSH_COMMAND=", "GIT_SSH=", "GIT_PROXY_COMMAND=",
		"GIT_EXEC_PATH=", "GIT_ASKPASS=", "GIT_TEMPLATE_DIR=",
		"LD_PRELOAD=", "DYLD_INSERT_LIBRARIES=",
		"PATH=", "GIT_CONFIG_GLOBAL=",
	}
	for _, prefix := range blockedPrefixes {
		for _, e := range env {
			if strings.HasPrefix(e, prefix) && strings.HasSuffix(e, "=evil") {
				t.Errorf("dangerous env key leaked into environment: %s", e)
			}
		}
	}
}

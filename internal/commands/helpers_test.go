package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LederWorks/gorepos/pkg/types"
)

func makeTestRepo(name, path string) types.Repository {
	return types.Repository{Name: name, Path: path, URL: "https://github.com/example/" + name + ".git"}
}

// --- FilterRepositoriesByContext ---

func TestFilterRepositoriesByContext_AtBasePath_ReturnsAll(t *testing.T) {
	dir := t.TempDir()
	repos := []types.Repository{
		makeTestRepo("a", "org/a"),
		makeTestRepo("b", "org/b"),
	}

	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	_ = os.Chdir(dir)

	result := FilterRepositoriesByContext(repos, dir)
	if len(result) != 2 {
		t.Errorf("expected 2 repos, got %d", len(result))
	}
}

func TestFilterRepositoriesByContext_OutsideBasePath_ReturnsAll(t *testing.T) {
	basePath := t.TempDir()
	repos := []types.Repository{
		makeTestRepo("a", "org/a"),
		makeTestRepo("b", "org/b"),
	}

	// CWD is a completely different temp dir
	cwd := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	_ = os.Chdir(cwd)

	result := FilterRepositoriesByContext(repos, basePath)
	if len(result) != 2 {
		t.Errorf("expected 2 repos when outside basePath, got %d", len(result))
	}
}

func TestFilterRepositoriesByContext_InSubdir_FiltersToSubtree(t *testing.T) {
	basePath := t.TempDir()
	subDir := filepath.Join(basePath, "org")
	_ = os.MkdirAll(subDir, 0755)

	repos := []types.Repository{
		makeTestRepo("a", "org/a"),
		makeTestRepo("b", "other/b"),
		makeTestRepo("c", "org/c"),
	}

	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	_ = os.Chdir(subDir)

	result := FilterRepositoriesByContext(repos, basePath)
	if len(result) != 2 {
		t.Errorf("expected 2 repos in 'org' subtree, got %d", len(result))
	}
	for _, r := range result {
		if r.Name == "b" {
			t.Errorf("repo 'b' (other/b) should have been filtered out")
		}
	}
}

func TestFilterRepositoriesByContext_EmptyRepos_ReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	_ = os.Chdir(dir)

	result := FilterRepositoriesByContext(nil, dir)
	if len(result) != 0 {
		t.Errorf("expected 0 repos, got %d", len(result))
	}
}

func TestFilterRepositoriesByContext_EmptyBasePath_ReturnsAll(t *testing.T) {
	repos := []types.Repository{
		makeTestRepo("a", "org/a"),
		makeTestRepo("b", "org/b"),
	}
	// When basePath is empty (not configured), no context filtering should occur.
	result := FilterRepositoriesByContext(repos, "")
	if len(result) != 2 {
		t.Errorf("expected all repos when basePath is empty, got %d", len(result))
	}
}

func TestFilterRepositoriesByContext_NoBoundaryFalsePositive(t *testing.T) {
	// relPath="org" must NOT match repo.Path="org2/a" (false-positive prefix match)
	basePath := t.TempDir()
	subDir := filepath.Join(basePath, "org")
	_ = os.MkdirAll(subDir, 0755)

	repos := []types.Repository{
		makeTestRepo("a", "org/a"),
		makeTestRepo("b2", "org2/b"),
	}

	orig, _ := os.Getwd()
	defer func() { _ = os.Chdir(orig) }()
	_ = os.Chdir(subDir)

	result := FilterRepositoriesByContext(repos, basePath)
	if len(result) != 1 || result[0].Name != "a" {
		t.Errorf("expected only repo 'a', got %v", result)
	}
}

// --- GetContextRepositoryNames ---

func TestGetContextRepositoryNames_AtBasePath_ReturnsAll(t *testing.T) {
	repos := []types.Repository{
		makeTestRepo("a", "org/a"),
		makeTestRepo("b", "org/b"),
	}
	names := GetContextRepositoryNames(repos, "/base", "/base")
	if len(names) != 2 {
		t.Errorf("expected 2 names at base path, got %d", len(names))
	}
}

func TestGetContextRepositoryNames_InSubdir_FiltersToMatchingRepos(t *testing.T) {
	repos := []types.Repository{
		makeTestRepo("a", "org/a"),
		makeTestRepo("b", "other/b"),
	}
	names := GetContextRepositoryNames(repos, "/base", "/base/org")
	if len(names) != 1 || names[0] != "a" {
		t.Errorf("expected [a], got %v", names)
	}
}

func TestGetContextRepositoryNames_NoMatch_ReturnsEmpty(t *testing.T) {
	repos := []types.Repository{
		makeTestRepo("a", "org/a"),
	}
	names := GetContextRepositoryNames(repos, "/base", "/base/unrelated")
	if len(names) != 0 {
		t.Errorf("expected empty, got %v", names)
	}
}

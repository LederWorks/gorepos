package display

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout redirects os.Stdout to a buffer during fn, then restores it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}

func makeNode(path string, valid bool, repos ...string) FileNode {
	ri := make([]RepositoryInfo, 0, len(repos))
	for _, r := range repos {
		ri = append(ri, RepositoryInfo{Name: r})
	}
	return FileNode{Path: path, IsValid: valid, Repositories: ri}
}

// --- PrintConfigTree (basic_tree) ---

func TestPrintConfigTree_OutputContainsFileName(t *testing.T) {
	d := NewConfigTreeDisplay()
	node := makeNode("/home/user/.gorepos/gorepos.yaml", true, "repo1", "repo2")

	out := captureStdout(t, func() {
		d.PrintConfigTree([]FileNode{node})
	})

	if !strings.Contains(out, "gorepos.yaml") {
		t.Errorf("expected output to contain 'gorepos.yaml', got:\n%s", out)
	}
}

func TestPrintConfigTree_OutputContainsRepoNames(t *testing.T) {
	d := NewConfigTreeDisplay()
	node := makeNode("/tmp/gorepos.yaml", true, "alpha", "beta")

	out := captureStdout(t, func() {
		d.PrintConfigTree([]FileNode{node})
	})

	if !strings.Contains(out, "alpha") {
		t.Errorf("expected output to contain 'alpha', got:\n%s", out)
	}
	if !strings.Contains(out, "beta") {
		t.Errorf("expected output to contain 'beta', got:\n%s", out)
	}
}

func TestPrintConfigTree_EmptyHierarchy_NoOutput(t *testing.T) {
	d := NewConfigTreeDisplay()
	out := captureStdout(t, func() {
		d.PrintConfigTree(nil)
	})
	if out != "" {
		t.Errorf("expected no output for empty hierarchy, got: %q", out)
	}
}

func TestPrintConfigTree_NestedInclude_BothFilesPresent(t *testing.T) {
	d := NewConfigTreeDisplay()
	child := makeNode("/tmp/included.yaml", true, "child-repo")
	parent := FileNode{
		Path:         "/tmp/gorepos.yaml",
		IsValid:      true,
		Repositories: []RepositoryInfo{{Name: "parent-repo"}},
		Includes:     []FileNode{child},
	}

	out := captureStdout(t, func() {
		d.PrintConfigTree([]FileNode{parent})
	})

	if !strings.Contains(out, "gorepos.yaml") {
		t.Errorf("expected parent file in output, got:\n%s", out)
	}
	if !strings.Contains(out, "included.yaml") {
		t.Errorf("expected included file in output, got:\n%s", out)
	}
}

// --- PrintConfigTreeWithValidation (validation_tree) ---

func TestPrintConfigTreeWithValidation_ValidNodeShowsCheckmark(t *testing.T) {
	d := NewConfigTreeDisplay()
	node := makeNode("/tmp/gorepos.yaml", true)

	out := captureStdout(t, func() {
		d.PrintConfigTreeWithValidation([]FileNode{node})
	})

	if !strings.Contains(out, "✅") {
		t.Errorf("expected ✅ for valid node, got:\n%s", out)
	}
}

func TestPrintConfigTreeWithValidation_InvalidNodeShowsCross(t *testing.T) {
	d := NewConfigTreeDisplay()
	node := makeNode("/tmp/gorepos.yaml", false)

	out := captureStdout(t, func() {
		d.PrintConfigTreeWithValidation([]FileNode{node})
	})

	if !strings.Contains(out, "❌") {
		t.Errorf("expected ❌ for invalid node, got:\n%s", out)
	}
}

// --- PrintConfigTreeWithValidationAndFileGroups (groups_tree) ---

func TestPrintConfigTreeWithValidationAndFileGroups_NoContext_ShowsAllRepos(t *testing.T) {
	d := NewConfigTreeDisplay()
	node := FileNode{
		Path:         "/tmp/gorepos.yaml",
		IsValid:      true,
		Repositories: []RepositoryInfo{{Name: "repo-a"}, {Name: "repo-b"}},
		FileGroups:   map[string][]string{"backend": {"repo-a", "repo-b"}},
	}

	out := captureStdout(t, func() {
		d.PrintConfigTreeWithValidationAndFileGroups([]FileNode{node}, nil)
	})

	if !strings.Contains(out, "gorepos.yaml") {
		t.Errorf("expected config file in output, got:\n%s", out)
	}
}

func TestPrintConfigTreeWithValidationAndFileGroups_WithContext_FiltersRepos(t *testing.T) {
	d := NewConfigTreeDisplay()
	node := FileNode{
		Path:    "/tmp/gorepos.yaml",
		IsValid: true,
		Repositories: []RepositoryInfo{
			{Name: "repo-a"},
			{Name: "repo-b"},
		},
	}

	// Only repo-a is in context
	out := captureStdout(t, func() {
		d.PrintConfigTreeWithValidationAndFileGroups([]FileNode{node}, []string{"repo-a"})
	})

	// Output should be non-empty (the filtered tree is rendered)
	if out == "" {
		t.Error("expected non-empty output with context filtering")
	}
}

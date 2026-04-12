package commands

import (
	"os"
	"testing"
)

// --- ReposCommand ---

func TestReposCommand_ValidConfig_NoError(t *testing.T) {
	dir := t.TempDir()
	path := writeCommandYAML(t, dir, "gorepos.yaml", validCommandConfig)

	orig, _ := os.Getwd()
	defer os.Chdir(orig)

	cmd := NewReposCommand()
	if err := cmd.Execute(path, false); err != nil {
		t.Errorf("expected no error for valid config, got: %v", err)
	}
}

func TestReposCommand_MissingFile_ReturnsError(t *testing.T) {
	cmd := NewReposCommand()
	if err := cmd.Execute("/nonexistent/gorepos.yaml", false); err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestReposCommand_VerboseMode_NoError(t *testing.T) {
	dir := t.TempDir()
	path := writeCommandYAML(t, dir, "gorepos.yaml", validCommandConfig)

	orig, _ := os.Getwd()
	defer os.Chdir(orig)

	cmd := NewReposCommand()
	if err := cmd.Execute(path, true); err != nil {
		t.Errorf("verbose mode should not cause errors: %v", err)
	}
}

func TestReposCommand_NoRepositoriesInContext_NoError(t *testing.T) {
	dir := t.TempDir()
	path := writeCommandYAML(t, dir, "gorepos.yaml", `
version: "1.0"
global:
  basePath: `+dir+`
  workers: 2
  timeout: 5s
repositories:
  - name: repo1
    path: org/repo1
    url: https://github.com/example/repo1.git
`)

	// Change to a subdir that doesn't match any repo path
	subDir := dir + "/unrelated"
	os.MkdirAll(subDir, 0755)
	orig, _ := os.Getwd()
	defer os.Chdir(orig)
	os.Chdir(subDir)

	cmd := NewReposCommand()
	if err := cmd.Execute(path, false); err != nil {
		t.Errorf("expected no error even with no repos in context: %v", err)
	}
}

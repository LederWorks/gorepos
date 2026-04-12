package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func writeCommandYAML(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("writeCommandYAML: %v", err)
	}
	return p
}

const validCommandConfig = `
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

// --- ValidateCommand ---

func TestValidateCommand_ValidConfig_NoError(t *testing.T) {
	dir := t.TempDir()
	path := writeCommandYAML(t, dir, "gorepos.yaml", validCommandConfig)

	cmd := NewValidateCommand()
	if err := cmd.Execute(path, false); err != nil {
		t.Errorf("expected no error for valid config, got: %v", err)
	}
}

func TestValidateCommand_InvalidConfig_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := writeCommandYAML(t, dir, "gorepos.yaml", `
version: "1.0"
global:
  basePath: /tmp/repos
  workers: 5
  timeout: 10s
`)
	// No repositories and no includes — should fail validation

	cmd := NewValidateCommand()
	if err := cmd.Execute(path, false); err == nil {
		t.Error("expected error for config with no repositories or includes")
	}
}

func TestValidateCommand_MissingFile_ReturnsError(t *testing.T) {
	cmd := NewValidateCommand()
	if err := cmd.Execute("/nonexistent/gorepos.yaml", false); err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestValidateCommand_VerboseMode_NoError(t *testing.T) {
	dir := t.TempDir()
	path := writeCommandYAML(t, dir, "gorepos.yaml", validCommandConfig)

	cmd := NewValidateCommand()
	if err := cmd.Execute(path, true); err != nil {
		t.Errorf("verbose mode should not cause errors: %v", err)
	}
}

func TestValidateCommand_WithIncludes_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	writeCommandYAML(t, dir, "included.yaml", `
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
	root := writeCommandYAML(t, dir, "gorepos.yaml", `
version: "1.0"
global:
  basePath: /tmp/repos
  workers: 5
  timeout: 10s
includes:
  - path: included.yaml
`)

	cmd := NewValidateCommand()
	if err := cmd.Execute(root, false); err != nil {
		t.Errorf("expected no error for config with includes only, got: %v", err)
	}
}

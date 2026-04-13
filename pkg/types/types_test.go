package types

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// --- IncludeEntry YAML round-trip ---

func TestIncludeEntry_UnmarshalYAML_PlainString(t *testing.T) {
	input := `"./configs/github.yaml"`
	var entry IncludeEntry
	if err := yaml.Unmarshal([]byte(input), &entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Path != "./configs/github.yaml" {
		t.Errorf("expected Path=%q, got %q", "./configs/github.yaml", entry.Path)
	}
	if entry.Repo != "" || entry.Ref != "" || entry.File != "" {
		t.Errorf("expected Repo/Ref/File to be empty, got %+v", entry)
	}
}

func TestIncludeEntry_UnmarshalYAML_HTTPUrl(t *testing.T) {
	input := `"https://raw.githubusercontent.com/org/repo/main/gorepos.yaml"`
	var entry IncludeEntry
	if err := yaml.Unmarshal([]byte(input), &entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Path != "https://raw.githubusercontent.com/org/repo/main/gorepos.yaml" {
		t.Errorf("unexpected Path: %q", entry.Path)
	}
}

func TestIncludeEntry_UnmarshalYAML_Mapping(t *testing.T) {
	input := `
repo: "https://github.com/org/repo"
ref: "main"
file: "configs/gorepos.yaml"
`
	var entry IncludeEntry
	if err := yaml.Unmarshal([]byte(input), &entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Repo != "https://github.com/org/repo" {
		t.Errorf("unexpected Repo: %q", entry.Repo)
	}
	if entry.Ref != "main" {
		t.Errorf("unexpected Ref: %q", entry.Ref)
	}
	if entry.File != "configs/gorepos.yaml" {
		t.Errorf("unexpected File: %q", entry.File)
	}
}

func TestIncludeEntry_UnmarshalYAML_MappingNoRef(t *testing.T) {
	input := `
repo: "https://github.com/org/repo"
`
	var entry IncludeEntry
	if err := yaml.Unmarshal([]byte(input), &entry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Repo != "https://github.com/org/repo" {
		t.Errorf("unexpected Repo: %q", entry.Repo)
	}
	if entry.Ref != "" {
		t.Errorf("expected empty Ref, got %q", entry.Ref)
	}
}

func TestIncludeEntry_UnmarshalYAML_InvalidKind(t *testing.T) {
	input := `[a, b, c]`
	var entry IncludeEntry
	if err := yaml.Unmarshal([]byte(input), &entry); err == nil {
		t.Error("expected error for sequence node, got nil")
	}
}

func TestIncludeEntry_MarshalYAML_PlainPathEmitsString(t *testing.T) {
	entry := IncludeEntry{Path: "./local.yaml"}
	out, err := yaml.Marshal(entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := string(out)
	// Should marshal as a plain string, not a mapping
	if got != "./local.yaml\n" {
		t.Errorf("expected plain string, got %q", got)
	}
}

func TestIncludeEntry_MarshalYAML_RepoEmitsMapping(t *testing.T) {
	entry := IncludeEntry{Repo: "https://github.com/org/repo", Ref: "main"}
	out, err := yaml.Marshal(entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should marshal as mapping — re-parse to verify
	var back IncludeEntry
	if err := yaml.Unmarshal(out, &back); err != nil {
		t.Fatalf("could not re-parse: %v", err)
	}
	if back.Repo != "https://github.com/org/repo" || back.Ref != "main" {
		t.Errorf("round-trip failed: got %+v", back)
	}
}

// --- IncludeEntry method tests ---

func TestIncludeEntry_IsLocal(t *testing.T) {
	tests := []struct {
		name  string
		entry IncludeEntry
		want  bool
	}{
		{"local relative", IncludeEntry{Path: "./local.yaml"}, true},
		{"local absolute", IncludeEntry{Path: "/abs/path.yaml"}, true},
		{"http url", IncludeEntry{Path: "https://example.com/f.yaml"}, false},
		{"repo entry", IncludeEntry{Repo: "https://github.com/org/repo"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.entry.IsLocal(); got != tc.want {
				t.Errorf("IsLocal() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIncludeEntry_IsRemoteRepo(t *testing.T) {
	e1 := IncludeEntry{Repo: "https://github.com/org/repo"}
	if !e1.IsRemoteRepo() {
		t.Error("expected IsRemoteRepo() = true for entry with Repo")
	}
	e2 := IncludeEntry{Path: "https://example.com/f.yaml"}
	if e2.IsRemoteRepo() {
		t.Error("expected IsRemoteRepo() = false for raw URL entry")
	}
}

func TestIncludeEntry_IsRawURL(t *testing.T) {
	e1 := IncludeEntry{Path: "https://example.com/f.yaml"}
	if !e1.IsRawURL() {
		t.Error("expected IsRawURL() = true for https path")
	}
	e2 := IncludeEntry{Path: "http://example.com/f.yaml"}
	if !e2.IsRawURL() {
		t.Error("expected IsRawURL() = true for http path")
	}
	e3 := IncludeEntry{Path: "./local.yaml"}
	if e3.IsRawURL() {
		t.Error("expected IsRawURL() = false for local path")
	}
	e4 := IncludeEntry{Repo: "https://github.com/org/repo"}
	if e4.IsRawURL() {
		t.Error("expected IsRawURL() = false for repo entry")
	}
}

func TestIncludeEntry_GetFile_DefaultsToGoreposYaml(t *testing.T) {
	entry := IncludeEntry{Repo: "https://github.com/org/repo"}
	if entry.GetFile() != "gorepos.yaml" {
		t.Errorf("expected default file 'gorepos.yaml', got %q", entry.GetFile())
	}
}

func TestIncludeEntry_GetFile_ReturnsCustomFile(t *testing.T) {
	entry := IncludeEntry{Repo: "https://github.com/org/repo", File: "configs/custom.yaml"}
	if entry.GetFile() != "configs/custom.yaml" {
		t.Errorf("expected 'configs/custom.yaml', got %q", entry.GetFile())
	}
}

func TestIncludeEntry_String_PathEntry(t *testing.T) {
	entry := IncludeEntry{Path: "./local.yaml"}
	if entry.String() != "./local.yaml" {
		t.Errorf("unexpected String(): %q", entry.String())
	}
}

func TestIncludeEntry_String_RepoEntry(t *testing.T) {
	entry := IncludeEntry{Repo: "https://github.com/org/repo", Ref: "main", File: "c.yaml"}
	s := entry.String()
	if s == "" {
		t.Error("String() should not be empty for repo entry")
	}
	// Should contain the repo URL
	if len(s) < len("https://github.com/org/repo") {
		t.Errorf("String() too short: %q", s)
	}
}

// --- Config includes list round-trip ---

func TestConfig_Includes_MixedList(t *testing.T) {
	input := `
includes:
  - "./local.yaml"
  - "https://example.com/remote.yaml"
  - repo: "https://github.com/org/repo"
    ref: "main"
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(input), &cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Includes) != 3 {
		t.Fatalf("expected 3 includes, got %d", len(cfg.Includes))
	}
	if !cfg.Includes[0].IsLocal() {
		t.Error("first include should be local")
	}
	if !cfg.Includes[1].IsRawURL() {
		t.Error("second include should be raw URL")
	}
	if !cfg.Includes[2].IsRemoteRepo() {
		t.Error("third include should be remote repo")
	}
}

// --- User/Email YAML round-trip ---

func TestIncludeEntry_UnmarshalYAML_MappingWithUserEmail(t *testing.T) {
	yamlStr := `
repo: "https://github.com/org/repo"
ref: "main"
user: "Jane Dev"
email: "jane@example.com"
`
	var entry IncludeEntry
	if err := yaml.Unmarshal([]byte(yamlStr), &entry); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if entry.Repo != "https://github.com/org/repo" {
		t.Errorf("unexpected repo: %q", entry.Repo)
	}
	if entry.User != "Jane Dev" {
		t.Errorf("unexpected user: %q", entry.User)
	}
	if entry.Email != "jane@example.com" {
		t.Errorf("unexpected email: %q", entry.Email)
	}
}

func TestIncludeEntry_MarshalYAML_RepoWithUserEmailEmitsMapping(t *testing.T) {
	entry := IncludeEntry{
		Repo:  "https://github.com/org/repo",
		User:  "Jane",
		Email: "jane@co.com",
	}
	out, err := yaml.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "user: Jane") {
		t.Errorf("expected 'user: Jane' in output: %s", s)
	}
	if !strings.Contains(s, "email: jane@co.com") {
		t.Errorf("expected 'email: jane@co.com' in output: %s", s)
	}
}

func TestIncludeEntry_YAMLRoundTrip_WithUserEmail(t *testing.T) {
	original := Config{
		Version: "1.0",
		Includes: []IncludeEntry{
			{Repo: "https://github.com/org/repo", Ref: "main", User: "Jane", Email: "jane@co.com"},
			{Path: "./local.yaml"},
		},
	}
	out, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var roundTripped Config
	if err := yaml.Unmarshal(out, &roundTripped); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(roundTripped.Includes) != 2 {
		t.Fatalf("expected 2 includes, got %d", len(roundTripped.Includes))
	}
	if roundTripped.Includes[0].User != "Jane" || roundTripped.Includes[0].Email != "jane@co.com" {
		t.Errorf("user/email lost in round-trip: user=%q email=%q",
			roundTripped.Includes[0].User, roundTripped.Includes[0].Email)
	}
	if roundTripped.Includes[1].Path != "./local.yaml" {
		t.Errorf("local path lost in round-trip: %q", roundTripped.Includes[1].Path)
	}
}

func TestApplyIncludeIdentity_PartialIdentity(t *testing.T) {
	c := &Config{
		Repositories: []Repository{
			{Name: "r1", Path: "r1", URL: "https://x.git"},
			{Name: "r2", Path: "r2", URL: "https://y.git", User: "Existing"},
		},
	}
	c.ApplyIncludeIdentity("Jane", "")

	if c.Repositories[0].User != "Jane" {
		t.Errorf("expected user Jane on r1, got %q", c.Repositories[0].User)
	}
	if c.Repositories[0].Email != "" {
		t.Errorf("expected empty email on r1, got %q", c.Repositories[0].Email)
	}
	if c.Repositories[1].User != "Existing" {
		t.Errorf("repo-level user should not be overridden, got %q", c.Repositories[1].User)
	}
}

package config

import (
	"strings"
	"testing"

	"github.com/LederWorks/gorepos/pkg/types"
)

func TestResolveRawContentURL_GitHub(t *testing.T) {
	tests := []struct {
		name     string
		repoURL  string
		ref      string
		file     string
		wantURL  string
		wantErr  bool
	}{
		{
			name:    "basic with ref",
			repoURL: "https://github.com/Ledermayer/gorepos-ledermayer-gh",
			ref:     "arch-rev",
			file:    "gorepos.yaml",
			wantURL: "https://raw.githubusercontent.com/Ledermayer/gorepos-ledermayer-gh/arch-rev/gorepos.yaml",
		},
		{
			name:    "no ref uses HEAD",
			repoURL: "https://github.com/org/repo",
			ref:     "",
			file:    "gorepos.yaml",
			wantURL: "https://raw.githubusercontent.com/org/repo/HEAD/gorepos.yaml",
		},
		{
			name:    "with .git suffix",
			repoURL: "https://github.com/org/repo.git",
			ref:     "main",
			file:    "gorepos.yaml",
			wantURL: "https://raw.githubusercontent.com/org/repo/main/gorepos.yaml",
		},
		{
			name:    "nested file path",
			repoURL: "https://github.com/org/repo",
			ref:     "main",
			file:    "configs/team.yaml",
			wantURL: "https://raw.githubusercontent.com/org/repo/main/configs/team.yaml",
		},
		{
			name:    "missing repo path",
			repoURL: "https://github.com/org",
			ref:     "main",
			file:    "gorepos.yaml",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveRawContentURL(tt.repoURL, tt.ref, tt.file)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got URL: %s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantURL {
				t.Errorf("got  %s\nwant %s", got, tt.wantURL)
			}
		})
	}
}

func TestResolveRawContentURL_AzureDevOps(t *testing.T) {
	tests := []struct {
		name     string
		repoURL  string
		ref      string
		file     string
		wantContains []string
		wantErr  bool
	}{
		{
			name:    "basic with ref",
			repoURL: "https://dev.azure.com/ADOS-OTPHU-01/gcp-lz/_git/gorepos-gcp-lz",
			ref:     "main",
			file:    "gorepos.yaml",
			wantContains: []string{
				"dev.azure.com/ADOS-OTPHU-01/gcp-lz/_apis/git/repositories/gorepos-gcp-lz/items",
				"path=%2Fgorepos.yaml",
				"versionDescriptor.version=main",
				"versionDescriptor.versionType=branch",
			},
		},
		{
			name:    "no ref omits version descriptor",
			repoURL: "https://dev.azure.com/org/project/_git/repo",
			ref:     "",
			file:    "gorepos.yaml",
			wantContains: []string{
				"dev.azure.com/org/project/_apis/git/repositories/repo/items",
				"path=%2Fgorepos.yaml",
			},
		},
		{
			name:    "missing _git segment",
			repoURL: "https://dev.azure.com/org/project/repo",
			ref:     "main",
			file:    "gorepos.yaml",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveRawContentURL(tt.repoURL, tt.ref, tt.file)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got URL: %s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("URL %s\n  missing: %s", got, want)
				}
			}
			// Ensure no version descriptor when ref is empty
			if tt.ref == "" && strings.Contains(got, "versionDescriptor") {
				t.Errorf("URL should not contain versionDescriptor when ref is empty: %s", got)
			}
		})
	}
}

func TestResolveRawContentURL_GitLab(t *testing.T) {
	got, err := ResolveRawContentURL("https://gitlab.com/org/repo", "develop", "gorepos.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://gitlab.com/org/repo/-/raw/develop/gorepos.yaml"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestResolveRawContentURL_Bitbucket(t *testing.T) {
	got, err := ResolveRawContentURL("https://bitbucket.org/org/repo", "main", "gorepos.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://bitbucket.org/org/repo/raw/main/gorepos.yaml"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestResolveRawContentURL_UnsupportedPlatform(t *testing.T) {
	_, err := ResolveRawContentURL("https://example.com/org/repo", "main", "gorepos.yaml")
	if err == nil {
		t.Error("expected error for unsupported platform")
	}
	if !strings.Contains(err.Error(), "unsupported git hosting platform") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestIsRepoURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://github.com/org/repo", true},
		{"https://dev.azure.com/org/project/_git/repo", true},
		{"https://gitlab.com/org/repo", true},
		{"https://bitbucket.org/org/repo", true},
		{"https://example.com/org/repo", false},
		{"./local-config.yaml", false},
		{"not-a-url", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			if got := IsRepoURL(tt.url); got != tt.want {
				t.Errorf("IsRepoURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

// --- Custom platform support ---

func TestIsRepoURL_CustomPlatform_RecognisedAsRepo(t *testing.T) {
	platforms := []types.PlatformEntry{
		{Hostname: "gitlab.mycompany.com", Type: "gitlab"},
	}
	if !IsRepoURL("https://gitlab.mycompany.com/org/repo", platforms) {
		t.Error("expected custom platform URL to be recognised as repo URL")
	}
}

func TestIsRepoURL_CustomPlatform_UnknownHostStillFails(t *testing.T) {
	platforms := []types.PlatformEntry{
		{Hostname: "gitlab.mycompany.com", Type: "gitlab"},
	}
	if IsRepoURL("https://gitea.other.com/org/repo", platforms) {
		t.Error("expected unregistered custom host to return false")
	}
}

func TestIsRepoURL_CustomPlatform_CaseInsensitiveHostMatch(t *testing.T) {
	platforms := []types.PlatformEntry{
		{Hostname: "GitLab.MyCompany.Com", Type: "gitlab"},
	}
	if !IsRepoURL("https://gitlab.mycompany.com/org/repo", platforms) {
		t.Error("hostname matching should be case-insensitive")
	}
}

func TestResolveRawContentURL_CustomGitLabPlatform(t *testing.T) {
	platforms := []types.PlatformEntry{
		{Hostname: "gitlab.mycompany.com", Type: "gitlab"},
	}
	got, err := ResolveRawContentURL("https://gitlab.mycompany.com/org/repo", "main", "gorepos.yaml", platforms)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://gitlab.mycompany.com/org/repo/-/raw/main/gorepos.yaml"
	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestResolveRawContentURL_CustomGitHubEnterprisePlatform(t *testing.T) {
	platforms := []types.PlatformEntry{
		{Hostname: "github.internal.corp", Type: "github"},
	}
	got, err := ResolveRawContentURL("https://github.internal.corp/org/repo", "main", "gorepos.yaml", platforms)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "raw.githubusercontent.com") {
		t.Errorf("expected GitHub-style raw URL, got: %s", got)
	}
}

func TestResolveRawContentURL_CustomPlatform_UnknownTypeError(t *testing.T) {
	platforms := []types.PlatformEntry{
		{Hostname: "git.example.com", Type: "forgejo"},
	}
	_, err := ResolveRawContentURL("https://git.example.com/org/repo", "main", "gorepos.yaml", platforms)
	if err == nil {
		t.Error("expected error for unknown platform type 'forgejo'")
	}
}

func TestResolveRawContentURL_NoCustomPlatforms_UnsupportedHostStillFails(t *testing.T) {
	var platforms []types.PlatformEntry
	_, err := ResolveRawContentURL("https://example.com/org/repo", "main", "gorepos.yaml", platforms)
	if err == nil {
		t.Error("expected error for unsupported platform even with empty custom list")
	}
}

package config

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/LederWorks/gorepos/pkg/types"
)

// parseSCPOrURL parses either a standard URL or an SCP-style git URL (e.g. git@host:org/repo).
// For SCP URLs it returns a synthetic *url.URL with Scheme="ssh", Host set to the hostname,
// and Path set to "/"+the repo path, so all callers can use url.URL methods uniformly.
// Standard URLs (those containing "://") are forwarded to url.Parse unchanged.
func parseSCPOrURL(rawURL string) (*url.URL, error) {
	// SCP syntax never contains "://" — distinguish from valid URLs like ssh://host/path.
	if !strings.Contains(rawURL, "://") {
		if colonIdx := strings.Index(rawURL, ":"); colonIdx > 0 {
			hostPart := rawURL[:colonIdx]
			// Strip optional user@ prefix (e.g. "git" in "git@github.com:org/repo").
			if atIdx := strings.LastIndex(hostPart, "@"); atIdx >= 0 {
				hostPart = hostPart[atIdx+1:]
			}
			repoPath := rawURL[colonIdx+1:]
			return &url.URL{
				Scheme: "ssh",
				Host:   hostPart,
				Path:   "/" + repoPath,
			}, nil
		}
	}
	return url.Parse(rawURL)
}

// ResolveRawContentURL converts a repository URL + ref + file path into a
// platform-specific raw content URL that can be fetched via HTTP GET.
// An optional list of custom platform entries (from global.platforms) is checked
// before the built-in platform switch, enabling support for self-hosted instances.
// Both standard HTTPS URLs and SCP-style git@ URLs are accepted.
func ResolveRawContentURL(repoURL, ref, filePath string, customPlatforms ...[]types.PlatformEntry) (string, error) {
	u, err := parseSCPOrURL(repoURL)
	if err != nil {
		return "", fmt.Errorf("invalid repo URL: %w", err)
	}

	host := strings.ToLower(u.Hostname())

	// Check caller-supplied custom platforms first
	if len(customPlatforms) > 0 {
		for _, entry := range customPlatforms[0] {
			if strings.ToLower(entry.Hostname) == host {
				return resolveByType(entry.Type, u, ref, filePath)
			}
		}
	}

	switch {
	case host == "github.com":
		return resolveGitHub(u, ref, filePath)
	case host == "dev.azure.com" || strings.HasSuffix(host, ".visualstudio.com"):
		return resolveAzureDevOps(u, ref, filePath)
	case host == "gitlab.com":
		return resolveGitLab(u, ref, filePath)
	case host == "bitbucket.org":
		return resolveBitbucket(u, ref, filePath)
	default:
		return "", fmt.Errorf("unsupported git hosting platform: %s (supported: github.com, dev.azure.com, gitlab.com, bitbucket.org; add custom platforms via global.platforms)", host)
	}
}

// IsRepoURL returns true if the URL belongs to a known git hosting platform.
// An optional list of custom platform entries is checked before the built-in list.
// Both standard HTTPS URLs and SCP-style git@ URLs are accepted.
func IsRepoURL(rawURL string, customPlatforms ...[]types.PlatformEntry) bool {
	u, err := parseSCPOrURL(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())

	// Check caller-supplied custom platforms first
	if len(customPlatforms) > 0 {
		for _, entry := range customPlatforms[0] {
			if strings.ToLower(entry.Hostname) == host {
				return true
			}
		}
	}

	switch {
	case host == "github.com",
		host == "dev.azure.com",
		strings.HasSuffix(host, ".visualstudio.com"),
		host == "gitlab.com",
		host == "bitbucket.org":
		return true
	}
	return false
}

// resolveByType delegates URL resolution to the appropriate resolver for the given platform type.
func resolveByType(platformType string, u *url.URL, ref, filePath string) (string, error) {
	switch strings.ToLower(platformType) {
	case "github":
		return resolveGitHub(u, ref, filePath)
	case "gitlab":
		return resolveGitLab(u, ref, filePath)
	case "azure":
		return resolveAzureDevOps(u, ref, filePath)
	case "bitbucket":
		return resolveBitbucket(u, ref, filePath)
	default:
		return "", fmt.Errorf("unknown platform type %q (valid: github, gitlab, azure, bitbucket)", platformType)
	}
}

// resolveGitHub converts a GitHub repo URL to a raw content URL.
// For github.com:
//
//	Input:  https://github.com/{owner}/{repo}
//	Output: https://raw.githubusercontent.com/{owner}/{repo}/{ref}/{file}
//
// For GitHub Enterprise Server (GHES):
//
//	Input:  https://{ghes-host}/{owner}/{repo}
//	Output: https://{ghes-host}/{owner}/{repo}/raw/{ref}/{file}
func resolveGitHub(u *url.URL, ref, filePath string) (string, error) {
	// Path: /{owner}/{repo} (possibly with .git suffix or trailing slash)
	parts := cleanPathSegments(u.Path)
	if len(parts) < 2 {
		return "", fmt.Errorf("GitHub URL must have at least owner/repo: %s", u.String())
	}

	owner := parts[0]
	repo := strings.TrimSuffix(parts[1], ".git")

	if ref == "" {
		ref = "HEAD"
	}

	hostname := u.Hostname()
	if hostname == "github.com" {
		// Public GitHub: raw content is served from a dedicated CDN host
		return fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s",
			owner, repo, ref, filePath), nil
	}

	// GitHub Enterprise Server: raw content is served from the instance host itself
	return fmt.Sprintf("https://%s/%s/%s/raw/%s/%s",
		hostname, owner, repo, ref, filePath), nil
}

// semverTagPattern matches semver-style version refs such as "v1.0.0", "1.2.3",
// "v2.0.0-beta1". It is used by resolveAzureVersionType to distinguish tag refs
// from branch names that start with "v" (e.g., "versioning", "v-next").
var semverTagPattern = regexp.MustCompile(`^v?\d+(\.\d+)*(-.*)?$`)

// resolveAzureVersionType returns the correct versionDescriptor.versionType value for the
// Azure DevOps Items API based on the ref string.
//
//   - A full 40-hex character string → "commit"
//   - A ref prefixed with "refs/tags/" → "tag"
//   - A ref prefixed with "refs/heads/" → "branch"
//   - A ref matching the semver pattern (e.g. v1.0.0, 1.2.3, v2.0.0-beta1) → "tag"
//   - Anything else → "branch"
func resolveAzureVersionType(ref string) string {
	// Full 40-character hex commit SHA
	if len(ref) == 40 {
		if matched, _ := regexp.MatchString(`^[0-9a-fA-F]{40}$`, ref); matched {
			return "commit"
		}
	}
	// Explicit refs/ namespaced refs
	if strings.HasPrefix(ref, "refs/tags/") {
		return "tag"
	}
	if strings.HasPrefix(ref, "refs/heads/") {
		return "branch"
	}
	// Semver-style tag detection (e.g. v1.0.0, 1.2.3, v2.0.0-beta1).
	// Plain "v" prefix is not sufficient: branch names like "versioning" or "v-next"
	// would be misclassified. Require numeric version components after the optional v.
	if semverTagPattern.MatchString(ref) {
		return "tag"
	}
	return "branch"
}

// resolveAzureDevOps converts an Azure DevOps repo URL to a raw content API URL.
// Input:  https://dev.azure.com/{org}/{project}/_git/{repo}
// Output: https://dev.azure.com/{org}/{project}/_apis/git/repositories/{repo}/items?path=/{file}&...
func resolveAzureDevOps(u *url.URL, ref, filePath string) (string, error) {
	parts := cleanPathSegments(u.Path)

	// Expected: /{org}/{project}/_git/{repo}
	gitIdx := -1
	for i, p := range parts {
		if p == "_git" {
			gitIdx = i
			break
		}
	}
	if gitIdx < 0 || gitIdx < 2 || gitIdx+1 >= len(parts) {
		return "", fmt.Errorf("azure DevOps URL must match /{org}/{project}/_git/{repo}: %s", u.String())
	}

	org := parts[0]
	project := parts[gitIdx-1]
	repo := parts[gitIdx+1]

	// Build the Items API URL
	apiURL := fmt.Sprintf("https://%s/%s/%s/_apis/git/repositories/%s/items",
		u.Hostname(), org, project, repo)

	params := url.Values{}
	params.Set("path", "/"+filePath)
	params.Set("api-version", "7.0")
	params.Set("$format", "text")

	if ref != "" {
		version := strings.TrimPrefix(ref, "refs/tags/")
		params.Set("versionDescriptor.version", version)
		params.Set("versionDescriptor.versionType", resolveAzureVersionType(ref))
	}

	return apiURL + "?" + params.Encode(), nil
}

// resolveGitLab converts a GitLab repo URL to a raw content URL.
// Input:  https://gitlab.com/{owner}/{repo}
// Output: https://gitlab.com/{owner}/{repo}/-/raw/{ref}/{file}
func resolveGitLab(u *url.URL, ref, filePath string) (string, error) {
	parts := cleanPathSegments(u.Path)
	if len(parts) < 2 {
		return "", fmt.Errorf("GitLab URL must have at least owner/repo: %s", u.String())
	}

	owner := parts[0]
	repo := strings.TrimSuffix(parts[1], ".git")

	if ref == "" {
		ref = "HEAD"
	}

	return fmt.Sprintf("https://%s/%s/%s/-/raw/%s/%s",
		u.Hostname(), owner, repo, ref, filePath), nil
}

// resolveBitbucket converts a Bitbucket repo URL to a raw content URL.
// Input:  https://bitbucket.org/{owner}/{repo}
// Output: https://bitbucket.org/{owner}/{repo}/raw/{ref}/{file}
func resolveBitbucket(u *url.URL, ref, filePath string) (string, error) {
	parts := cleanPathSegments(u.Path)
	if len(parts) < 2 {
		return "", fmt.Errorf("bitbucket URL must have at least owner/repo: %s", u.String())
	}

	owner := parts[0]
	repo := strings.TrimSuffix(parts[1], ".git")

	if ref == "" {
		ref = "HEAD"
	}

	return fmt.Sprintf("https://%s/%s/%s/raw/%s/%s",
		u.Hostname(), owner, repo, ref, filePath), nil
}

// cleanPathSegments splits a URL path into non-empty segments.
func cleanPathSegments(path string) []string {
	var segments []string
	for _, s := range strings.Split(strings.Trim(path, "/"), "/") {
		if s != "" {
			segments = append(segments, s)
		}
	}
	return segments
}

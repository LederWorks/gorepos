package repository

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/LederWorks/gorepos/internal/config"
	"github.com/LederWorks/gorepos/pkg/types"
)

// Manager implements the RepositoryManager interface
type Manager struct {
	basePath    string
	credentials *types.CredentialConfig // may be nil
}

// NewManager creates a new repository manager
func NewManager(basePath string) *Manager {
	return &Manager{
		basePath: basePath,
	}
}

// NewManagerWithCredentials creates a repository manager that sets per-repo git identity after clone.
func NewManagerWithCredentials(basePath string, creds *types.CredentialConfig) *Manager {
	return &Manager{
		basePath:    basePath,
		credentials: creds,
	}
}

// Clone clones a repository if it doesn't exist
func (m *Manager) Clone(ctx context.Context, repo *types.Repository) error {
	repoPath, err := m.getRepoPath(repo)
	if err != nil {
		return fmt.Errorf("resolving repository path: %w", err)
	}

	if m.Exists(repo) {
		return fmt.Errorf("repository already exists at %s", repoPath)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(repoPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	args := []string{"clone"}
	if repo.Branch != "" {
		args = append(args, "-b", repo.Branch)
	}
	args = append(args, repo.URL, repoPath)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = m.buildEnvironment(repo)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w\nOutput: %s", err, string(output))
	}

	// Set local git identity (never touches --global config).
	// Use -- to prevent git from interpreting the value as a flag (SEC-H3).
	if user := m.effectiveUser(repo); user != "" {
		if err := exec.CommandContext(ctx, "git", "-C", repoPath, "config", "user.name", "--", user).Run(); err != nil {
			return fmt.Errorf("setting git user.name: %w", err)
		}
	}
	if email := m.effectiveEmail(repo); email != "" {
		if err := exec.CommandContext(ctx, "git", "-C", repoPath, "config", "user.email", "--", email).Run(); err != nil {
			return fmt.Errorf("setting git user.email: %w", err)
		}
	}

	return nil
}

func (m *Manager) effectiveUser(repo *types.Repository) string {
	if repo.User != "" {
		return repo.User
	}
	if m.credentials != nil {
		return m.credentials.GitUserName
	}
	return ""
}

func (m *Manager) effectiveEmail(repo *types.Repository) string {
	if repo.Email != "" {
		return repo.Email
	}
	if m.credentials != nil {
		return m.credentials.GitUserEmail
	}
	return ""
}

// Update updates an existing repository
func (m *Manager) Update(ctx context.Context, repo *types.Repository) error {
	if !m.Exists(repo) {
		repoPath, _ := m.getRepoPath(repo)
		return fmt.Errorf("repository does not exist at %s", repoPath)
	}

	repoPath, err := m.getRepoPath(repo)
	if err != nil {
		return fmt.Errorf("resolving repository path: %w", err)
	}

	// Fetch latest changes from origin without modifying the working tree.
	cmd := exec.CommandContext(ctx, "git", "fetch", "origin")
	cmd.Dir = repoPath
	cmd.Env = m.buildEnvironment(repo)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch failed: %w\nOutput: %s", err, string(output))
	}

	// Refuse to update if there are uncommitted changes — the pull would abort
	// anyway, but an early check gives a clearer message.
	status, err := m.Status(ctx, repo)
	if err != nil {
		return fmt.Errorf("failed to check repository status: %w", err)
	}

	if !status.IsClean {
		return fmt.Errorf("repository has uncommitted changes, cannot update")
	}

	// Fast-forward only: never destroy local commits (H-2).
	// Pull explicitly from origin/<branch> so the configured branch is always
	// used regardless of which branch is currently checked out locally.
	branch := repo.Branch
	if branch == "" {
		branch = "main"
	}
	cmd = exec.CommandContext(ctx, "git", "pull", "--ff-only", "origin", branch)
	cmd.Dir = repoPath
	cmd.Env = m.buildEnvironment(repo)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf(
			"update: fast-forward not possible for %q — local and remote have diverged; manual merge required: %w\nOutput: %s",
			repo.Name, err, string(output),
		)
	}

	return nil
}

// Status returns the current status of a repository
func (m *Manager) Status(ctx context.Context, repo *types.Repository) (*types.RepoStatus, error) {
	if !m.Exists(repo) {
		repoPath, _ := m.getRepoPath(repo)
		return nil, fmt.Errorf("repository does not exist at %s", repoPath)
	}

	repoPath, err := m.getRepoPath(repo)
	if err != nil {
		return nil, fmt.Errorf("resolving repository path: %w", err)
	}
	status := &types.RepoStatus{
		Path: repoPath,
	}

	// Get current branch
	cmd := exec.CommandContext(ctx, "git", "branch", "--show-current")
	cmd.Dir = repoPath
	cmd.Env = m.buildEnvironment(repo)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}
	status.CurrentBranch = strings.TrimSpace(string(output))

	// Check if working tree is clean
	cmd = exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = repoPath
	cmd.Env = m.buildEnvironment(repo)

	output, err = cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	statusOutput := strings.TrimSpace(string(output))
	status.IsClean = statusOutput == ""

	if !status.IsClean {
		lines := strings.Split(statusOutput, "\n")
		for _, line := range lines {
			if line != "" {
				// Extract filename from git status --porcelain output (XY FILENAME from column 3)
				if len(line) >= 3 {
					status.UncommittedFiles = append(status.UncommittedFiles, line[3:])
				}
			}
		}
	}

	// Get ahead/behind info
	targetBranch := repo.Branch
	if targetBranch == "" {
		targetBranch = "main"
	}

	cmd = exec.CommandContext(ctx, "git", "rev-list", "--count", "--left-right", fmt.Sprintf("HEAD...origin/%s", targetBranch))
	cmd.Dir = repoPath
	cmd.Env = m.buildEnvironment(repo)

	output, err = cmd.Output()
	if err == nil {
		parts := strings.Split(strings.TrimSpace(string(output)), "\t")
		if len(parts) == 2 {
			ahead := 0
			behind := 0
			_, _ = fmt.Sscanf(parts[0], "%d", &ahead)
			_, _ = fmt.Sscanf(parts[1], "%d", &behind)

			status.AheadBehind = &types.BranchComparison{
				Ahead:  ahead,
				Behind: behind,
			}
		}
	}

	return status, nil
}

// Execute runs a custom command in the repository directory
func (m *Manager) Execute(ctx context.Context, repo *types.Repository, command string, args ...string) (*types.Result, error) {
	startTime := time.Now()
	result := &types.Result{
		Repository: repo,
		Operation:  command,
		StartTime:  startTime,
	}

	repoPath, err := m.getRepoPath(repo)
	if err != nil {
		result.Error = fmt.Errorf("resolving repository path: %w", err)
		result.Duration = time.Since(startTime)
		return result, result.Error
	}

	if !m.Exists(repo) {
		result.Error = fmt.Errorf("repository does not exist at %s", repoPath)
		result.Duration = time.Since(startTime)
		return result, result.Error
	}

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = repoPath
	cmd.Env = m.buildEnvironment(repo)

	output, err := cmd.CombinedOutput()
	result.Output = string(output)
	result.Duration = time.Since(startTime)

	if err != nil {
		result.Error = err
		result.Success = false
	} else {
		result.Success = true
	}

	return result, nil
}

// Exists checks if a repository exists at the configured path
func (m *Manager) Exists(repo *types.Repository) bool {
	repoPath, err := m.getRepoPath(repo)
	if err != nil {
		return false
	}
	gitDir := filepath.Join(repoPath, ".git")

	_, statErr := os.Stat(gitDir)
	return statErr == nil
}

// getRepoPath returns the resolved path for a repository, verifying that it
// does not escape m.basePath (path traversal protection, SEC-C2).
func (m *Manager) getRepoPath(repo *types.Repository) (string, error) {
	var resolved string
	if filepath.IsAbs(repo.Path) {
		resolved = filepath.Clean(repo.Path)
	} else if m.basePath != "" {
		resolved = filepath.Join(m.basePath, repo.Path)
	} else {
		resolved = repo.Path
	}

	// Containment check: when a basePath is set, the resolved path must stay
	// inside it (prevents both relative ../../ traversal and absolute escapes).
	if m.basePath != "" {
		absBase, err := filepath.Abs(m.basePath)
		if err != nil {
			return "", fmt.Errorf("resolving basePath: %w", err)
		}
		absResolved, err := filepath.Abs(resolved)
		if err != nil {
			return "", fmt.Errorf("resolving repo path: %w", err)
		}
		sep := string(filepath.Separator)
		if !strings.HasPrefix(absResolved+sep, absBase+sep) {
			return "", fmt.Errorf("repository path %q escapes basePath %q", repo.Path, m.basePath)
		}
	}

	return resolved, nil
}

// buildEnvironment builds the environment variables for git commands,
// filtering out keys that could be used to hijack subprocess execution (SEC-C1).
func (m *Manager) buildEnvironment(repo *types.Repository) []string {
	env := os.Environ()
	for key, value := range repo.Environment {
		if _, blocked := config.BlockedEnvKeys[strings.ToUpper(key)]; blocked {
			// Skip dangerous keys silently (validation should have caught them at load time)
			continue
		}
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	return env
}

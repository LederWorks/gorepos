package repository

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/LederWorks/gorepos/pkg/types"
)

// Manager implements the RepositoryManager interface
type Manager struct {
	basePath string
}

// NewManager creates a new repository manager
func NewManager(basePath string) *Manager {
	return &Manager{
		basePath: basePath,
	}
}

// Clone clones a repository if it doesn't exist
func (m *Manager) Clone(ctx context.Context, repo *types.Repository) error {
	repoPath := m.getRepoPath(repo)

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

	return nil
}

// Update updates an existing repository
func (m *Manager) Update(ctx context.Context, repo *types.Repository) error {
	if !m.Exists(repo) {
		return fmt.Errorf("repository does not exist at %s", m.getRepoPath(repo))
	}

	repoPath := m.getRepoPath(repo)

	// Fetch latest changes
	cmd := exec.CommandContext(ctx, "git", "fetch", "origin")
	cmd.Dir = repoPath
	cmd.Env = m.buildEnvironment(repo)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch failed: %w\nOutput: %s", err, string(output))
	}

	// Reset to origin branch if clean
	status, err := m.Status(ctx, repo)
	if err != nil {
		return fmt.Errorf("failed to check repository status: %w", err)
	}

	if !status.IsClean {
		return fmt.Errorf("repository has uncommitted changes, cannot update")
	}

	// Reset to origin branch
	targetBranch := repo.Branch
	if targetBranch == "" {
		targetBranch = "main"
	}

	cmd = exec.CommandContext(ctx, "git", "reset", "--hard", fmt.Sprintf("origin/%s", targetBranch))
	cmd.Dir = repoPath
	cmd.Env = m.buildEnvironment(repo)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git reset failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// Status returns the current status of a repository
func (m *Manager) Status(ctx context.Context, repo *types.Repository) (*types.RepoStatus, error) {
	if !m.Exists(repo) {
		return nil, fmt.Errorf("repository does not exist at %s", m.getRepoPath(repo))
	}

	repoPath := m.getRepoPath(repo)
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
				// Extract filename from git status output
				parts := strings.SplitN(line, " ", 3)
				if len(parts) >= 3 {
					status.UncommittedFiles = append(status.UncommittedFiles, strings.TrimSpace(parts[2]))
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
			fmt.Sscanf(parts[0], "%d", &ahead)
			fmt.Sscanf(parts[1], "%d", &behind)

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

	repoPath := m.getRepoPath(repo)

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
	repoPath := m.getRepoPath(repo)
	gitDir := filepath.Join(repoPath, ".git")

	// Check if it's a git repository
	if stat, err := os.Stat(gitDir); err == nil {
		return stat.IsDir()
	}

	// Check if it's a git worktree
	if _, err := os.Stat(gitDir); err == nil {
		return true
	}

	return false
}

// getRepoPath returns the absolute path for a repository
func (m *Manager) getRepoPath(repo *types.Repository) string {
	if filepath.IsAbs(repo.Path) {
		return repo.Path
	}

	if m.basePath != "" {
		return filepath.Join(m.basePath, repo.Path)
	}

	return repo.Path
}

// buildEnvironment builds the environment variables for git commands
func (m *Manager) buildEnvironment(repo *types.Repository) []string {
	env := os.Environ()

	// Add repository-specific environment variables
	for key, value := range repo.Environment {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	return env
}

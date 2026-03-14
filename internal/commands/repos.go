package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LederWorks/gorepos/internal/config"
	"github.com/LederWorks/gorepos/pkg/types"
)

// GitInfo contains git repository status information
type GitInfo struct {
	Branch      string
	HasUnstaged bool
	HasStaged   bool
	IsClean     bool
	Exists      bool
}

// ReposCommand handles the repository filesystem display command
type ReposCommand struct {
	configFile string
	verbose    bool
	basePath   string
}

// NewReposCommand creates a new repos command handler
func NewReposCommand() *ReposCommand {
	return &ReposCommand{}
}

// Execute runs the repos command
func (r *ReposCommand) Execute(configFile string, verbose bool) error {
	r.configFile = configFile
	r.verbose = verbose

	loader := config.NewLoader()

	// Get config file path
	configPath := configFile
	if configPath == "" {
		var err error
		configPath, err = config.GetConfigPath()
		if err != nil {
			return err
		}
	}

	// Load configuration
	result, err := loader.LoadConfigWithDetails(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Store base path for git operations
	r.basePath = result.Config.Global.BasePath

	// Get current working directory for context
	cwd, err := os.Getwd()
	if err != nil {
		cwd = result.Config.Global.BasePath
	}

	// Show repository filesystem hierarchy based on current context
	fmt.Println("Repository Filesystem Hierarchy:")
	fmt.Println(strings.Repeat("=", 40))

	// Apply context-aware filtering
	contextRepos := r.filterRepositoriesByContext(result.Config.Repositories, result.Config.Global.BasePath)
	if len(contextRepos) > 0 {
		r.printRepositoryTreeSimple(contextRepos, result.Config.Global.BasePath, cwd)
	} else {
		fmt.Println("No repositories in current context")
	}

	if verbose {
		fmt.Printf("\nContext information:\n")
		fmt.Printf("Current directory: %s\n", cwd)
		fmt.Printf("Base path: %s\n", result.Config.Global.BasePath)
		fmt.Printf("Repositories in context: %d\n", len(contextRepos))
	}

	return nil
}

// filterRepositoriesByContext filters repositories based on current working directory context
func (r *ReposCommand) filterRepositoriesByContext(repositories []types.Repository, basePath string) []types.Repository {
	cwd, err := os.Getwd()
	if err != nil {
		return repositories // Return all repositories if we can't determine context
	}

	// Normalize paths for comparison
	normBasePath := strings.ReplaceAll(basePath, "\\", "/")
	normCwd := strings.ReplaceAll(cwd, "\\", "/")

	// Check if we're at the base path or outside of it
	if normCwd == normBasePath || !strings.HasPrefix(normCwd, normBasePath) {
		return repositories // Show all repositories when at base path or outside it
	}

	// Extract the relative path from base path
	relPath := strings.TrimPrefix(normCwd, normBasePath)
	relPath = strings.TrimPrefix(relPath, "/")
	relPath = strings.TrimSuffix(relPath, "/")

	if relPath == "" {
		return repositories
	}

	// Find repositories whose configured path is under the current context directory
	var contextRepos []types.Repository
	for _, repo := range repositories {
		normRepoPath := strings.ReplaceAll(repo.Path, "\\", "/")

		// Include the repo if its path starts with the current relative path
		if strings.HasPrefix(normRepoPath, relPath) {
			contextRepos = append(contextRepos, repo)
		}
	}

	return contextRepos
}

// printRepositoryTreeSimple displays the repository filesystem hierarchy with git status
func (r *ReposCommand) printRepositoryTreeSimple(repos []types.Repository, basePath, currentDir string) {
	// Build directory structure from repository paths
	dirTree := r.buildDirectoryTree(repos, basePath)

	// Display the tree
	fmt.Printf("📁 %s\n", basePath)
	r.printDirectoryTree(dirTree, "", true)
}

// DirectoryNode represents a directory in the filesystem tree
type DirectoryNode struct {
	Name         string
	Repositories []types.Repository
	Subdirs      map[string]*DirectoryNode
	IsLeaf       bool
}

// buildDirectoryTree constructs a directory tree from repository configurations
func (r *ReposCommand) buildDirectoryTree(repos []types.Repository, basePath string) *DirectoryNode {
	root := &DirectoryNode{
		Name:    basePath,
		Subdirs: make(map[string]*DirectoryNode),
	}

	for _, repo := range repos {
		// Use repo.Path to derive the parent directory in the tree.
		// repo.Path is the relative path of the repository under basePath,
		// so its parent directory determines where the repo node is nested.
		normPath := strings.ReplaceAll(repo.Path, "\\", "/")
		parentDir := filepath.ToSlash(filepath.Dir(normPath))
		if parentDir == "." {
			parentDir = ""
		}

		if parentDir == "" {
			// Repository lives directly in the root
			root.Repositories = append(root.Repositories, repo)
			continue
		}

		// Create all intermediate directory nodes, then attach the repository
		// to the leaf directory node.
		parts := strings.Split(parentDir, "/")
		current := root
		for _, part := range parts {
			if part == "" {
				continue
			}
			if current.Subdirs[part] == nil {
				current.Subdirs[part] = &DirectoryNode{
					Name:    part,
					Subdirs: make(map[string]*DirectoryNode),
				}
			}
			current = current.Subdirs[part]
		}
		current.Repositories = append(current.Repositories, repo)
	}

	return root
}

// printDirectoryTree recursively prints the directory tree
func (r *ReposCommand) printDirectoryTree(node *DirectoryNode, prefix string, isLast bool) {
	// Sort subdirectories
	var subdirNames []string
	for name := range node.Subdirs {
		subdirNames = append(subdirNames, name)
	}
	sort.Strings(subdirNames)

	// Sort repositories
	sort.Slice(node.Repositories, func(i, j int) bool {
		return node.Repositories[i].Name < node.Repositories[j].Name
	})

	totalItems := len(subdirNames) + len(node.Repositories)
	itemCount := 0

	// Print subdirectories first
	for _, name := range subdirNames {
		itemCount++
		isLastItem := itemCount == totalItems

		connector := "├── "
		if isLastItem {
			connector = "└── "
		}

		fmt.Printf("%s%s📁 %s/\n", prefix, connector, name)

		// Print subdirectory contents
		newPrefix := prefix
		if isLastItem {
			newPrefix += "    "
		} else {
			newPrefix += "│   "
		}

		r.printDirectoryTree(node.Subdirs[name], newPrefix, true)
	}

	// Then print repositories
	for _, repo := range node.Repositories {
		itemCount++
		isLastItem := itemCount == totalItems

		r.printRepositoryWithGitInfo(repo, prefix, isLastItem)
	}
}

// printRepositoryWithGitInfo displays a repository with git status information
func (r *ReposCommand) printRepositoryWithGitInfo(repo types.Repository, prefix string, isLast bool) {
	connector := "├── "
	if isLast {
		connector = "└── "
	}

	// Repository status indicator (enabled/disabled)
	status := "●" // Enabled
	if repo.Disabled {
		status = "○" // Disabled
	}

	// Repository type indicator (assume git for now)
	typeIcon := "🔗" // Git icon

	// Get git information
	gitInfo := r.getGitInfo(repo)

	// Extract repository directory name from full name
	repoDir := r.getRepositoryDirName(repo.Name)

	// Build display string
	displayStr := fmt.Sprintf("%s%s%s %s %s", prefix, connector, typeIcon, status, repoDir)

	// Add URL in verbose mode first
	if r.verbose && repo.URL != "" {
		displayStr += fmt.Sprintf(" (%s)", repo.URL)
	}

	// Add branch info if repository exists
	if gitInfo.Exists && gitInfo.Branch != "" {
		displayStr += fmt.Sprintf(" (%s)", gitInfo.Branch)
	}

	// Add git status icon only for enabled repositories
	// Disabled repositories use ○ symbol which already indicates they're not active
	if !repo.Disabled {
		gitStatusIcon := r.getGitStatusIcon(gitInfo)
		if gitStatusIcon != "" {
			displayStr += " " + gitStatusIcon
		}
	}

	fmt.Print(displayStr)

	fmt.Println()
}

// getRepositoryDirName returns the repository name for display
func (r *ReposCommand) getRepositoryDirName(repoName string) string {
	// Return the full repository name as configured
	return repoName
}

// getGitInfo retrieves git repository information
func (r *ReposCommand) getGitInfo(repo types.Repository) GitInfo {
	info := GitInfo{}

	// Construct repository path
	repoPath := filepath.Join(r.getBasePath(), repo.Path)
	if repo.Path == "" {
		repoPath = filepath.Join(r.getBasePath(), repo.Name)
	}

	// Check if repository directory exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return info // Repository doesn't exist
	}
	info.Exists = true

	// Get current branch
	info.Branch = r.getCurrentBranch(repoPath)

	// Get git status
	info.HasUnstaged, info.HasStaged, info.IsClean = r.getGitStatus(repoPath)

	return info
}

// getCurrentBranch gets the current git branch
func (r *ReposCommand) getCurrentBranch(repoPath string) string {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return "" // Not a git repository or git not available
	}

	branch := strings.TrimSpace(string(output))
	if branch == "" {
		// Might be in detached HEAD state, try to get commit
		cmd = exec.Command("git", "rev-parse", "--short", "HEAD")
		cmd.Dir = repoPath
		if output, err := cmd.Output(); err == nil {
			return "detached@" + strings.TrimSpace(string(output))
		}
	}

	return branch
}

// getGitStatus checks git status for unstaged and staged files
func (r *ReposCommand) getGitStatus(repoPath string) (hasUnstaged, hasStaged, isClean bool) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return false, false, false // Error getting status
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return false, false, true // Clean repository
	}

	for _, line := range lines {
		if len(line) >= 2 {
			indexStatus := line[0]
			workTreeStatus := line[1]

			// Check for staged files (index status)
			if indexStatus != ' ' && indexStatus != '?' {
				hasStaged = true
			}

			// Check for unstaged files (work tree status)
			if workTreeStatus != ' ' {
				hasUnstaged = true
			}
		}
	}

	isClean = !hasStaged && !hasUnstaged
	return hasUnstaged, hasStaged, isClean
}

// getGitStatusIcon returns appropriate icon based on git status
func (r *ReposCommand) getGitStatusIcon(info GitInfo) string {
	if !info.Exists {
		return "❌" // Repository doesn't exist
	}

	if info.IsClean {
		return "✅" // Clean/committed
	}

	if info.HasUnstaged {
		return "❌" // Has unstaged files
	}

	if info.HasStaged {
		return "🔄" // Has staged files (in progress)
	}

	return "✅" // Default to clean
}

// getBasePath gets the base path from configuration
func (r *ReposCommand) getBasePath() string {
	return r.basePath
}

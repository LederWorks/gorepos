package config

import (
	"bufio"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/LederWorks/gorepos/pkg/types"

	"gopkg.in/yaml.v3"
)

// GetConfigPath attempts to find a configuration file
func GetConfigPath() (string, error) {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	// Check for local config files in current directory
	localConfigs := []string{
		"gorepos.yaml",
		"gorepos.yml",
		".gorepos.yaml",
		".gorepos.yml",
	}

	for _, config := range localConfigs {
		configPath := filepath.Join(cwd, config)
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}
	}

	// Check for user config files
	userConfigPaths := getUserConfigPaths()
	for _, configPath := range userConfigPaths {
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}
	}

	return "", fmt.Errorf("no configuration file found. Run 'gorepos init' to create one")
}

// getUserConfigPaths returns platform-appropriate user configuration directories
func getUserConfigPaths() []string {
	var paths []string

	// Get user home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return paths
	}

	switch runtime.GOOS {
	case "windows":
		// Windows: Check multiple common locations including OneDrive
		paths = append(paths,
			// .gorepos directory (preferred)
			filepath.Join(homeDir, ".gorepos", "gorepos.yaml"),
			filepath.Join(homeDir, ".gorepos", "gorepos.yml"),
			// Standard Documents folder
			filepath.Join(homeDir, "Documents", "gorepos", "gorepos.yaml"),
			filepath.Join(homeDir, "Documents", "gorepos", "gorepos.yml"),
			// OneDrive Documents folder (common with Windows 10/11 OneDrive integration)
			filepath.Join(homeDir, "OneDrive", "Documents", "gorepos", "gorepos.yaml"),
			filepath.Join(homeDir, "OneDrive", "Documents", "gorepos", "gorepos.yml"),
			// Config folder
			filepath.Join(homeDir, ".config", "gorepos", "gorepos.yaml"),
			filepath.Join(homeDir, ".config", "gorepos", "gorepos.yml"),
			// Home directory
			filepath.Join(homeDir, ".gorepos.yaml"),
			filepath.Join(homeDir, ".gorepos.yml"),
		)
	case "darwin":
		// macOS: Follow standard conventions
		paths = append(paths,
			filepath.Join(homeDir, ".gorepos", "gorepos.yaml"),
			filepath.Join(homeDir, ".gorepos", "gorepos.yml"),
			filepath.Join(homeDir, ".config", "gorepos", "gorepos.yaml"),
			filepath.Join(homeDir, ".config", "gorepos", "gorepos.yml"),
			filepath.Join(homeDir, "Library", "Application Support", "gorepos", "gorepos.yaml"),
			filepath.Join(homeDir, "Library", "Application Support", "gorepos", "gorepos.yml"),
			filepath.Join(homeDir, ".gorepos.yaml"),
			filepath.Join(homeDir, ".gorepos.yml"),
		)
	default:
		// Linux and other Unix-like systems
		paths = append(paths,
			filepath.Join(homeDir, ".gorepos", "gorepos.yaml"),
			filepath.Join(homeDir, ".gorepos", "gorepos.yml"),
			filepath.Join(homeDir, ".config", "gorepos", "gorepos.yaml"),
			filepath.Join(homeDir, ".config", "gorepos", "gorepos.yml"),
			filepath.Join(homeDir, ".gorepos.yaml"),
			filepath.Join(homeDir, ".gorepos.yml"),
		)
	}

	return paths
}

// getDefaultUserConfigPath returns the primary user config directory for setup
func getDefaultUserConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	switch runtime.GOOS {
	case "windows":
		// Check if OneDrive Documents exists first (common with Windows 10/11)
		oneDriveDocsPath := filepath.Join(homeDir, "OneDrive", "Documents", "gorepos", "gorepos.yaml")
		if _, err := os.Stat(filepath.Dir(oneDriveDocsPath)); err == nil {
			return oneDriveDocsPath, nil
		}
		// Fall back to standard Documents folder
		return filepath.Join(homeDir, "Documents", "gorepos", "gorepos.yaml"), nil
	case "darwin":
		return filepath.Join(homeDir, ".gorepos", "gorepos.yaml"), nil
	default:
		return filepath.Join(homeDir, ".gorepos", "gorepos.yaml"), nil
	}
}

// RunInit creates a new user configuration file via an interactive wizard
func RunInit(options SetupOptions) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("GoRepos Initial Setup")
	fmt.Println("=====================")
	fmt.Println()

	// Step 1: Configuration location
	configPath, err := resolveConfigPath(options, reader)
	if err != nil {
		return err
	}

	// Check if config already exists
	if !options.Force {
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("configuration file already exists at %s\nUse --force to overwrite, or run 'gorepos setup' to add configuration sources", configPath)
		}
	}

	// Step 2: Base path
	basePath, err := resolveBasePath(options, reader)
	if err != nil {
		return err
	}

	// Step 3: Performance settings
	wizardWorkers, wizardTimeout := resolvePerformanceSettings(options, reader)

	// Silently read global git identity — set as defaults in the generated config
	gitUser := strings.TrimSpace(readGitGlobalConfig("user.name"))
	gitEmail := strings.TrimSpace(readGitGlobalConfig("user.email"))

	// Create configuration content
	configContent := createUserConfigTemplate(basePath, wizardWorkers, wizardTimeout, options.Includes, gitUser, gitEmail)

	fmt.Println()

	// Handle dry-run mode
	if options.DryRun {
		configDir := filepath.Dir(configPath)
		fmt.Printf("[dry-run] Would create directory: %s\n", configDir)
		fmt.Printf("[dry-run] Would create directory: %s\n", basePath)
		fmt.Printf("[dry-run] Would write configuration to: %s\n", configPath)
		fmt.Println()
		fmt.Println(configContent)
		return nil
	}

	// Create directories
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return fmt.Errorf("failed to create base path directory: %w", err)
	}

	// Write configuration file
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("✓ Configuration created at: %s\n", configPath)
	fmt.Printf("✓ Base path: %s\n", basePath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Run 'gorepos setup' to add configuration sources")
	fmt.Println("  2. Run 'gorepos validate' to check your configuration")
	fmt.Println("  3. Run 'gorepos clone' to clone repositories")

	return nil
}

// resolveConfigPath determines the config file path from flags or interactive prompt
func resolveConfigPath(options SetupOptions, reader *bufio.Reader) (string, error) {
	fmt.Println("Step 1/3: Configuration location")

	if options.Path != "" {
		absPath, err := filepath.Abs(options.Path)
		if err != nil {
			return "", fmt.Errorf("failed to resolve config path: %w", err)
		}
		fmt.Printf("  Config file: %s\n", absPath)
		fmt.Println()
		return absPath, nil
	}

	defaultPath, err := getDefaultUserConfigPath()
	if err != nil {
		return "", fmt.Errorf("failed to determine config path: %w", err)
	}

	fmt.Printf("  Default: %s\n", defaultPath)
	fmt.Print("  [Enter custom path or press Enter for default]: ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	fmt.Println()

	if input != "" {
		absPath, err := filepath.Abs(input)
		if err != nil {
			return "", fmt.Errorf("failed to resolve config path: %w", err)
		}
		return absPath, nil
	}
	return defaultPath, nil
}

// resolveBasePath determines the repository base path from flags or interactive prompt
func resolveBasePath(options SetupOptions, reader *bufio.Reader) (string, error) {
	fmt.Println("Step 2/3: Repository base path")
	fmt.Println("  Where should repositories be cloned?")

	if options.BasePath != "" {
		basePath := options.BasePath
		if !filepath.IsAbs(basePath) {
			if absPath, err := filepath.Abs(basePath); err == nil {
				basePath = absPath
			}
		}
		fmt.Printf("  Base path: %s\n", basePath)
		fmt.Println()
		return basePath, nil
	}

	defaultBase, err := getDefaultBasePath()
	if err != nil {
		return "", fmt.Errorf("failed to determine default base path: %w", err)
	}

	fmt.Printf("  Default: %s\n", defaultBase)
	fmt.Print("  [Enter path or press Enter for default]: ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	fmt.Println()

	basePath := defaultBase
	if input != "" {
		basePath = input
	}

	if !filepath.IsAbs(basePath) {
		if absPath, err := filepath.Abs(basePath); err == nil {
			basePath = absPath
		}
	}
	return basePath, nil
}

// resolvePerformanceSettings determines workers and timeout from flags or interactive prompt
func resolvePerformanceSettings(options SetupOptions, reader *bufio.Reader) (int, time.Duration) {
	fmt.Println("Step 3/3: Performance settings")

	workers := options.Workers
	timeout := options.Timeout

	if workers == 0 {
		fmt.Print("  Parallel workers [8]: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			if _, err := fmt.Sscanf(input, "%d", &workers); err != nil || workers < 1 {
				workers = 8
			}
		} else {
			workers = 8
		}
	} else {
		fmt.Printf("  Parallel workers: %d\n", workers)
	}

	if timeout == 0 {
		fmt.Print("  Operation timeout in seconds [300]: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			var seconds int
			if _, err := fmt.Sscanf(input, "%d", &seconds); err != nil || seconds < 1 {
				timeout = 300 * time.Second
			} else {
				timeout = time.Duration(seconds) * time.Second
			}
		} else {
			timeout = 300 * time.Second
		}
	} else {
		fmt.Printf("  Operation timeout: %s\n", timeout)
	}

	fmt.Println()
	return workers, timeout
}

// RunSetup adds configuration sources to an existing config file
func RunSetup(options SetupOptions) error {
	// Determine config path
	configPath, err := findExistingConfig(options.Path)
	if err != nil {
		return err
	}

	// Read existing config
	existingConfig, err := readExistingConfig(configPath)
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("GoRepos Setup — Add Configuration Sources")
	fmt.Println("==========================================")
	fmt.Println()
	fmt.Printf("Current config: %s\n", configPath)
	if existingConfig.Global.BasePath != "" {
		fmt.Printf("Base path: %s\n", existingConfig.Global.BasePath)
	}

	// Show current includes
	if len(existingConfig.Includes) > 0 {
		fmt.Printf("Current includes:\n")
		for _, inc := range existingConfig.Includes {
			fmt.Printf("  - %s\n", inc.String())
		}
	} else {
		fmt.Printf("Current includes: (none)\n")
	}
	fmt.Println()

	// Collect new includes
	var newIncludes []types.IncludeEntry
	if len(options.Includes) > 0 {
		// Non-interactive batch mode — parse strings into IncludeEntry
		for _, inc := range options.Includes {
			entry := parseIncludeString(inc)
			// Apply --user/--email flags to remote repo includes only
			if entry.IsRemoteRepo() {
				if options.User != "" {
					if err := validateUserName(options.User); err != nil {
						return fmt.Errorf("invalid --user value: %w", err)
					}
					entry.User = options.User
				}
				if options.Email != "" {
					if err := validateEmail(options.Email); err != nil {
						return fmt.Errorf("invalid --email value: %w", err)
					}
					entry.Email = options.Email
				}
			}
			if err := validateIncludeEntry(entry); err != nil {
				return fmt.Errorf("invalid include %q: %w", inc, err)
			}
			newIncludes = append(newIncludes, entry)
			fmt.Printf("  Added: %s\n", entry.String())
		}
	} else {
		// Interactive mode
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("💡 Tip: The gorepos-config public repo has examples and tutorials:")
		fmt.Println("   https://github.com/LederWorks/gorepos-config")
		fmt.Println()
		for {
			fmt.Print("Add a configuration source (local path or URL, or 'done' to finish):\n  > ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)

			if input == "" || strings.EqualFold(input, "done") {
				break
			}

			entry, err := promptForInclude(input, reader)
			if err != nil {
				fmt.Printf("  ⚠ Invalid: %v\n\n", err)
				continue
			}

			// Check for duplicates
			if containsInclude(existingConfig.Includes, entry) || containsInclude(newIncludes, entry) {
				fmt.Printf("  ⚠ Already included: %s\n\n", entry.String())
				continue
			}

			newIncludes = append(newIncludes, entry)
			fmt.Printf("  Added: %s\n\n", entry.String())
		}
	}

	if len(newIncludes) == 0 {
		fmt.Println("\nNo sources added.")
		return nil
	}

	// Merge includes
	allIncludes := append(existingConfig.Includes, newIncludes...)

	// Handle dry-run
	if options.DryRun {
		fmt.Println()
		fmt.Printf("[dry-run] Would update: %s\n", configPath)
		fmt.Printf("[dry-run] Includes would be:\n")
		for _, inc := range allIncludes {
			fmt.Printf("  - %s\n", inc.String())
		}
		return nil
	}

	// Update the config file
	if err := updateConfigIncludes(configPath, allIncludes); err != nil {
		return fmt.Errorf("failed to update config: %w", err)
	}

	fmt.Println()
	fmt.Printf("✓ Configuration updated: %s\n", configPath)
	fmt.Printf("  Includes:\n")
	for _, inc := range allIncludes {
		fmt.Printf("    - %s\n", inc.String())
	}
	fmt.Println()
	fmt.Println("Next: Run 'gorepos validate' to verify your configuration.")

	return nil
}

// findExistingConfig locates an existing config file, or returns an error directing user to run init
func findExistingConfig(customPath string) (string, error) {
	if customPath != "" {
		absPath, err := filepath.Abs(customPath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve config path: %w", err)
		}
		if _, err := os.Stat(absPath); err != nil {
			return "", fmt.Errorf("configuration file not found at %s\nRun 'gorepos init' to create one first", absPath)
		}
		return absPath, nil
	}

	// Try to find existing config using standard search
	configPath, err := GetConfigPath()
	if err != nil {
		return "", fmt.Errorf("no configuration file found\nRun 'gorepos init' to create one first")
	}
	return configPath, nil
}

// readExistingConfig reads and parses a config file for the setup wizard
func readExistingConfig(path string) (*configFileData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg configFileData
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// configFileData holds the parts of the config we need for the setup wizard
type configFileData struct {
	Version  string               `yaml:"version"`
	Includes []types.IncludeEntry `yaml:"includes,omitempty"`
	Global   struct {
		BasePath string `yaml:"basePath"`
	} `yaml:"global"`
}

// updateConfigIncludes reads a config file, updates its includes, and writes it back
func updateConfigIncludes(path string, includes []types.IncludeEntry) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse into a yaml.Node to preserve comments and structure
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// The document node wraps a mapping node
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return fmt.Errorf("unexpected YAML structure")
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("expected YAML mapping at root")
	}

	seqNode := buildIncludesSequenceNode(includes)

	// Find or create the "includes" key
	includesUpdated := false
	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == "includes" {
			root.Content[i+1] = seqNode
			includesUpdated = true
			break
		}
	}

	if !includesUpdated {
		// Insert includes after version key (or at beginning)
		insertIdx := 0
		for i := 0; i < len(root.Content)-1; i += 2 {
			if root.Content[i].Value == "version" {
				insertIdx = i + 2
				break
			}
		}

		keyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: "includes",
		}

		newContent := make([]*yaml.Node, 0, len(root.Content)+2)
		newContent = append(newContent, root.Content[:insertIdx]...)
		newContent = append(newContent, keyNode, seqNode)
		newContent = append(newContent, root.Content[insertIdx:]...)
		root.Content = newContent
	}

	// Write back
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// buildIncludesSequenceNode creates a YAML sequence node for includes,
// emitting plain strings for simple path entries and mapping nodes for structured entries.
func buildIncludesSequenceNode(includes []types.IncludeEntry) *yaml.Node {
	seqNode := &yaml.Node{
		Kind: yaml.SequenceNode,
		Tag:  "!!seq",
	}
	for _, inc := range includes {
		if inc.Repo != "" {
			// Structured entry — emit as mapping
			mapNode := &yaml.Node{
				Kind: yaml.MappingNode,
				Tag:  "!!map",
			}
			mapNode.Content = append(mapNode.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "repo"},
				&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: inc.Repo},
			)
			if inc.Ref != "" {
				mapNode.Content = append(mapNode.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "ref"},
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: inc.Ref},
				)
			}
			if inc.File != "" {
				mapNode.Content = append(mapNode.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "file"},
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: inc.File},
				)
			}
			if inc.User != "" {
				mapNode.Content = append(mapNode.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "user"},
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: inc.User},
				)
			}
			if inc.Email != "" {
				mapNode.Content = append(mapNode.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "email"},
					&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: inc.Email},
				)
			}
			seqNode.Content = append(seqNode.Content, mapNode)
		} else {
			// Simple path/URL — emit as scalar
			seqNode.Content = append(seqNode.Content, &yaml.Node{
				Kind:  yaml.ScalarNode,
				Tag:   "!!str",
				Value: inc.Path,
			})
		}
	}
	return seqNode
}

// validateIncludeEntry checks whether an include entry is valid
func validateIncludeEntry(entry types.IncludeEntry) error {
	if entry.Repo != "" {
		// Structured repo entry — validate the URL and platform
		if _, err := url.Parse(entry.Repo); err != nil {
			return fmt.Errorf("invalid repo URL: %w", err)
		}
		if !IsRepoURL(entry.Repo) {
			return fmt.Errorf("unsupported git platform: %s (supported: github.com, dev.azure.com, gitlab.com, bitbucket.org)", entry.Repo)
		}
		// Verify we can resolve it
		if _, err := ResolveRawContentURL(entry.Repo, entry.Ref, entry.GetFile()); err != nil {
			return err
		}
		return nil
	}

	if entry.Path == "" {
		return fmt.Errorf("include must have a path or repo")
	}

	// Check if it's a URL
	if strings.HasPrefix(entry.Path, "http://") || strings.HasPrefix(entry.Path, "https://") {
		if _, err := url.Parse(entry.Path); err != nil {
			return fmt.Errorf("invalid URL: %w", err)
		}
		return nil
	}

	// Local path — resolve and check existence
	absPath, err := filepath.Abs(entry.Path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("file not found: %s", absPath)
	}

	return nil
}

// promptForInclude builds an IncludeEntry from user input, prompting for ref/file if a repo URL is detected
func promptForInclude(input string, reader *bufio.Reader) (types.IncludeEntry, error) {
	// Check if it's a known git platform URL
	if (strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://")) && IsRepoURL(input) {
		entry := types.IncludeEntry{Repo: input}

		fmt.Print("  Git ref — branch name, tag (e.g. v1.0.0), or commit hash (Enter for default branch): ")
		ref, _ := reader.ReadString('\n')
		ref = strings.TrimSpace(ref)
		if ref != "" {
			entry.Ref = ref
		}

		// Check if gorepos.yaml exists at the repo root before asking for file path
		if needsFilePath := checkNeedsFilePath(entry.Repo, entry.Ref); needsFilePath {
			fmt.Print("  File path in repo (Enter for 'gorepos.yaml'): ")
			file, _ := reader.ReadString('\n')
			file = strings.TrimSpace(file)
			if file != "" {
				entry.File = file
			}
		}

		// Prompt for git identity (remote repo includes only)
		globalUser := strings.TrimSpace(readGitGlobalConfig("user.name"))
		globalEmail := strings.TrimSpace(readGitGlobalConfig("user.email"))

		if globalUser != "" {
			fmt.Printf("  Git user.name for repos from this source (Enter to use global default '%s'): ", globalUser)
		} else {
			fmt.Print("  Git user.name for repos from this source (Enter to skip): ")
		}
		user, _ := reader.ReadString('\n')
		user = strings.TrimSpace(user)
		if user != "" {
			if err := validateUserName(user); err != nil {
				return types.IncludeEntry{}, fmt.Errorf("invalid user.name: %w", err)
			}
			entry.User = user
		}

		if globalEmail != "" {
			fmt.Printf("  Git user.email for repos from this source (Enter to use global default '%s'): ", globalEmail)
		} else {
			fmt.Print("  Git user.email for repos from this source (Enter to skip): ")
		}
		email, _ := reader.ReadString('\n')
		email = strings.TrimSpace(email)
		if email != "" {
			if err := validateEmail(email); err != nil {
				return types.IncludeEntry{}, fmt.Errorf("invalid user.email: %w", err)
			}
			entry.Email = email
		}

		if err := validateIncludeEntry(entry); err != nil {
			return types.IncludeEntry{}, err
		}
		return entry, nil
	}

	// Plain path or raw URL
	entry := types.IncludeEntry{Path: input}
	if err := validateIncludeEntry(entry); err != nil {
		return types.IncludeEntry{}, err
	}
	return entry, nil
}

// checkNeedsFilePath checks whether a repo needs an explicit file path by verifying
// if gorepos.yaml exists at the repo root. Returns true if the file was not found
// (meaning the user should be asked for the file path).
func checkNeedsFilePath(repoURL, ref string) bool {
	// Try to resolve the raw content URL for gorepos.yaml at repo root
	checkRef := ref
	if checkRef == "" {
		checkRef = "HEAD" // platform-agnostic default for existence check
	}
	rawURL, err := ResolveRawContentURL(repoURL, checkRef, "gorepos.yaml")
	if err != nil {
		// Can't resolve URL — ask the user
		fmt.Println("  ℹ Could not check for gorepos.yaml at repo root")
		return true
	}

	// Quick HTTP HEAD check with short timeout
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Head(rawURL)
	if err != nil {
		fmt.Println("  ℹ Could not verify gorepos.yaml at repo root (network error)")
		return true
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		fmt.Println("  ✓ Found gorepos.yaml at repo root")
		return false
	case resp.StatusCode == http.StatusNotFound:
		fmt.Println("  ℹ No gorepos.yaml found at repo root")
		return true
	default:
		// 401, 403, 405, 5xx — can't determine, ask the user
		fmt.Printf("  ℹ Could not verify gorepos.yaml at repo root (HTTP %d)\n", resp.StatusCode)
		return true
	}
}

// parseIncludeString converts a CLI string argument into an IncludeEntry,
// auto-detecting repo URLs from known platforms.
func parseIncludeString(s string) types.IncludeEntry {
	if (strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")) && IsRepoURL(s) {
		return types.IncludeEntry{Repo: s}
	}
	return types.IncludeEntry{Path: s}
}

// containsInclude checks if an entry is already in the includes list
func containsInclude(includes []types.IncludeEntry, entry types.IncludeEntry) bool {
	for _, inc := range includes {
		if entry.Repo != "" {
			// Compare by repo URL and ref
			if inc.Repo == entry.Repo && inc.Ref == entry.Ref {
				return true
			}
		} else if inc.Path == entry.Path {
			return true
		}
	}
	return false
}

// getDefaultBasePath determines an appropriate default base path for repositories
func getDefaultBasePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	switch runtime.GOOS {
	case "windows":
		// Check for common Git locations on Windows
		candidates := []string{
			filepath.Join(homeDir, "Documents", "Git"),
			filepath.Join(homeDir, "git"),
			filepath.Join(homeDir, "repos"),
			filepath.Join(homeDir, "projects"),
		}
		// Check if any of these directories exist
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
		// Default to Documents/Git
		return filepath.Join(homeDir, "Documents", "Git"), nil

	case "darwin":
		// macOS: Check common development locations
		candidates := []string{
			filepath.Join(homeDir, "Developer"),
			filepath.Join(homeDir, "Projects"),
			filepath.Join(homeDir, "git"),
			filepath.Join(homeDir, "repos"),
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
		// Default to ~/Developer
		return filepath.Join(homeDir, "Developer"), nil

	default:
		// Linux and other Unix-like: Check common locations
		candidates := []string{
			filepath.Join(homeDir, "git"),
			filepath.Join(homeDir, "repos"),
			filepath.Join(homeDir, "projects"),
			filepath.Join(homeDir, "src"),
		}
		for _, candidate := range candidates {
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
		// Default to ~/git
		return filepath.Join(homeDir, "git"), nil
	}
}

// createUserConfigTemplate creates a user configuration template.
// gitUser and gitEmail are populated from 'git config --global user.name/email' if available.
func createUserConfigTemplate(basePath string, workers int, timeout time.Duration, includes []string, gitUser, gitEmail string) string {
	// Set default values if not provided
	if workers == 0 {
		workers = 8
	}
	if timeout == 0 {
		timeout = 300 * time.Second
	}

	timeoutSeconds := int(timeout.Seconds())

	// Build includes section: use provided includes if any, otherwise comment out a placeholder
	var includesSection string
	if len(includes) > 0 {
		includesSection = "includes:\n"
		for _, inc := range includes {
			includesSection += fmt.Sprintf("  - %q\n", inc)
		}
	} else {
		includesSection = "# includes:\n#   - \"./another-config.yaml\"\n#   - repo: \"https://github.com/org/repo\"\n#     ref: \"main\"\n"
	}

	// Build credentials section — include git identity if detected
	var credentialsSection string
	if gitUser != "" || gitEmail != "" {
		credentialsSection = "  credentials:\n    sshKeyPath: \"\"\n    gitCredHelper: \"\"\n"
		if gitUser != "" {
			credentialsSection += fmt.Sprintf("    gitUserName: %q\n", gitUser)
		}
		if gitEmail != "" {
			credentialsSection += fmt.Sprintf("    gitUserEmail: %q\n", gitEmail)
		}
	} else {
		credentialsSection = "  credentials:\n    sshKeyPath: \"\"\n    gitCredHelper: \"\"\n"
	}

	return fmt.Sprintf(`# GoRepos User Configuration
# Generated by 'gorepos init'

version: "1.0"

# Include additional configuration files (local paths or URLs)
%s
# Global settings for your environment
global:
  # Base path where repositories will be cloned
  basePath: "%s"

  # Number of parallel workers for operations
  workers: %d

  # Timeout for operations (in seconds)
  timeout: %ds

  # Environment variables for git operations
  environment:
    GIT_CONFIG_GLOBAL: ""

  # Credential configuration
%s
# You can define your own repositories here
# repositories:
#   - name: "my-project"
#     path: "my-org/my-project"
#     url: "https://github.com/my-org/my-project.git"
#     branch: "main"

# You can define your own groups here
# groups:
#   my-projects: ["my-project"]
`, includesSection, basePath, workers, timeoutSeconds, credentialsSection)
}

// readGitGlobalConfig reads a git global config value silently (returns empty string on failure).
func readGitGlobalConfig(key string) string {
	out, err := exec.Command("git", "config", "--global", key).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// validateUserName checks that the input is a plausible human name, not a
// pasted git command or something containing shell metacharacters.
func validateUserName(input string) error {
	if strings.HasPrefix(strings.ToLower(input), "git ") {
		return fmt.Errorf("looks like a git command was pasted — enter only the name (e.g. \"Jane Doe\")")
	}
	for _, ch := range []string{`"`, "'", "$", ";", "|", "`", "\\", "<", ">"} {
		if strings.Contains(input, ch) {
			return fmt.Errorf("name contains invalid character %q — enter a plain name (e.g. \"Jane Doe\")", ch)
		}
	}
	if len(input) > 100 {
		return fmt.Errorf("name is too long (%d chars, max 100)", len(input))
	}
	return nil
}

// validateEmail checks that the input looks like a valid email address.
func validateEmail(input string) error {
	if strings.HasPrefix(strings.ToLower(input), "git ") {
		return fmt.Errorf("looks like a git command was pasted — enter only the email address")
	}
	if !strings.Contains(input, "@") {
		return fmt.Errorf("not a valid email address (missing @)")
	}
	if _, err := mail.ParseAddress(input); err != nil {
		return fmt.Errorf("not a valid email address: %w", err)
	}
	return nil
}

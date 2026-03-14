package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
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

	return "", fmt.Errorf("no configuration file found. Run 'gorepos setup' to create one")
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
		return filepath.Join(homeDir, ".config", "gorepos", "gorepos.yaml"), nil
	default:
		return filepath.Join(homeDir, ".config", "gorepos", "gorepos.yaml"), nil
	}
}

// RunSetup creates a user configuration file
func RunSetup(options SetupOptions) error {
	// Determine config path: use custom path if provided, otherwise use platform default
	var configPath string
	if options.Path != "" {
		absPath, err := filepath.Abs(options.Path)
		if err != nil {
			return fmt.Errorf("failed to resolve config path: %w", err)
		}
		configPath = absPath
	} else {
		var err error
		configPath, err = getDefaultUserConfigPath()
		if err != nil {
			return fmt.Errorf("failed to determine config path: %w", err)
		}
	}

	// Check if config already exists
	if !options.Force {
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("configuration file already exists at %s. Use --force to overwrite", configPath)
		}
	}

	// Create directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Determine base path
	basePath := options.BasePath
	if basePath == "" {
		// Interactive prompt for base path
		fmt.Print("Enter the base path for repositories (press Enter for default): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input != "" {
			basePath = input
		} else {
			// Use default base path
			var err error
			basePath, err = getDefaultBasePath()
			if err != nil {
				return fmt.Errorf("failed to determine default base path: %w", err)
			}
		}
	}

	// Convert to absolute path
	if !filepath.IsAbs(basePath) {
		if absPath, err := filepath.Abs(basePath); err == nil {
			basePath = absPath
		}
	}

	// Create base directory if it doesn't exist
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return fmt.Errorf("failed to create base path directory: %w", err)
	}

	// Create configuration content
	configContent := createUserConfigTemplate(basePath, options.Workers, options.Timeout, options.Includes)

	// Write configuration file
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	fmt.Printf("Configuration created successfully at: %s\n", configPath)
	fmt.Printf("Base path set to: %s\n", basePath)
	fmt.Println("\nYou can now run 'gorepos validate' to check your configuration.")

	return nil
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

// createUserConfigTemplate creates a user configuration template
func createUserConfigTemplate(basePath string, workers int, timeout time.Duration, includes []string) string {
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
		includesSection = "# includes:\n#   - \"./another-config.yaml\"\n#   - \"https://example.com/gorepos-config.yaml\"\n"
	}

	return fmt.Sprintf(`# GoRepos User Configuration
# This file contains your personal configuration for gorepos
# Generated by 'gorepos setup'

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
  credentials:
    sshKeyPath: ""
    gitCredHelper: ""

# You can define your own repositories here
# repositories:
#   - name: "my-project"
#     path: "my-org/my-project"
#     url: "https://github.com/my-org/my-project.git"
#     branch: "main"

# You can define your own groups here
# groups:
#   my-projects: ["my-project"]

# Templates for generating repository content (optional)
# Note: Template import feature is planned for future versions
# templates:
#   readme: |
#     # {{ .Name }}
    
#     Repository: {{ .URL }}
#     Branch: {{ .Branch }}
#     Path: {{ .Path }}
`, includesSection, basePath, workers, timeoutSeconds)
}

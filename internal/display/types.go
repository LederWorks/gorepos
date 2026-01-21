package display

// ConfigTreeDisplay handles all configuration tree display functionality
type ConfigTreeDisplay struct{}

// NewConfigTreeDisplay creates a new configuration tree display handler
func NewConfigTreeDisplay() *ConfigTreeDisplay {
	return &ConfigTreeDisplay{}
}

// FileNode represents a configuration file in the include hierarchy
type FileNode struct {
	Path         string
	Repositories []RepositoryInfo
	IsValid      bool
	Includes     []FileNode
	FileGroups   map[string][]string // Groups defined in this specific file
}

// RepositoryInfo tracks repository name and status
type RepositoryInfo struct {
	Name     string
	Disabled bool
}

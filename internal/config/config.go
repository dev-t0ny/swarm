package config

// Config represents the .swarmrc configuration.
type Config struct {
	Symlinks       []string          `yaml:"symlinks"`
	InstallCommand string            `yaml:"install_command"`
	BasePort       int               `yaml:"base_port"`
	Agents         map[string]Agent  `yaml:"agents"`
}

// Agent represents a configured agent type.
type Agent struct {
	Command     string `yaml:"command"`
	MergePrompt string `yaml:"merge_prompt"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Symlinks:       []string{".env", ".env.local"},
		InstallCommand: "npm install",
		BasePort:       3000,
		Agents: map[string]Agent{
			"claude":   {Command: "claude", MergePrompt: "Merge your changes into {base_branch}. Resolve any conflicts. Confirm when done."},
			"opencode": {Command: "opencode", MergePrompt: "Merge into {base_branch}, resolve all conflicts, confirm when done."},
			"codex":    {Command: "codex", MergePrompt: "Merge into {base_branch} and resolve any conflicts."},
		},
	}
}

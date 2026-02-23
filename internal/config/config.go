package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the .swarmrc configuration.
type Config struct {
	Symlinks       []string         `yaml:"symlinks"`
	InstallCommand string           `yaml:"install_command"`
	BasePort       int              `yaml:"base_port"`
	Agents         map[string]Agent `yaml:"agents"`
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

// Load reads the .swarmrc file from the given directory.
// If the file doesn't exist, returns the default config.
// If it does exist, merges it with defaults (user values take precedence).
func Load(repoRoot string) *Config {
	cfg := DefaultConfig()

	data, err := os.ReadFile(filepath.Join(repoRoot, ".swarmrc"))
	if err != nil {
		// File doesn't exist or can't be read — use defaults
		return cfg
	}

	var userCfg Config
	if err := yaml.Unmarshal(data, &userCfg); err != nil {
		// Invalid YAML — use defaults
		return cfg
	}

	// Merge user config over defaults
	if len(userCfg.Symlinks) > 0 {
		cfg.Symlinks = userCfg.Symlinks
	}
	if userCfg.InstallCommand != "" {
		cfg.InstallCommand = userCfg.InstallCommand
	}
	if userCfg.BasePort != 0 {
		cfg.BasePort = userCfg.BasePort
	}
	if len(userCfg.Agents) > 0 {
		for name, agent := range userCfg.Agents {
			cfg.Agents[name] = agent
		}
	}

	return cfg
}

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
	DevCommand     string           `yaml:"dev_command"`
	BasePort       int              `yaml:"base_port"`
	Agents         map[string]Agent `yaml:"agents"`

	// Detected is the auto-detected project type (not from .swarmrc).
	Detected ProjectType `yaml:"-"`
}

// Agent represents a configured agent type.
type Agent struct {
	Command     string `yaml:"command"`
	MergePrompt string `yaml:"merge_prompt"`
}

// Load reads the .swarmrc file from the given directory.
// It first auto-detects the project type and applies preset defaults,
// then merges .swarmrc on top (user values always win).
func Load(repoRoot string) *Config {
	// 1. Detect the project type
	detected := DetectProject(repoRoot)
	preset := GetPreset(detected)

	// 2. Start with preset defaults (or bare defaults for unknown projects)
	cfg := &Config{
		Symlinks:       preset.Symlinks,
		InstallCommand: preset.InstallCommand,
		DevCommand:     preset.DevCommand,
		BasePort:       3000,
		Detected:       detected,
		Agents: map[string]Agent{
			"claude":   {Command: "claude", MergePrompt: "Merge your changes into {base_branch}. Resolve any conflicts. Confirm when done."},
			"opencode": {Command: "opencode", MergePrompt: "Merge into {base_branch}, resolve all conflicts, confirm when done."},
			"codex":    {Command: "codex", MergePrompt: "Merge into {base_branch} and resolve any conflicts."},
		},
	}
	if cfg.Symlinks == nil {
		cfg.Symlinks = []string{".env"}
	}

	// 3. Read .swarmrc if it exists
	data, err := os.ReadFile(filepath.Join(repoRoot, ".swarmrc"))
	if err != nil {
		return cfg
	}

	var userCfg Config
	if err := yaml.Unmarshal(data, &userCfg); err != nil {
		return cfg
	}

	// 4. Merge — user values override detected preset
	if len(userCfg.Symlinks) > 0 {
		cfg.Symlinks = userCfg.Symlinks
	}
	if userCfg.InstallCommand != "" {
		cfg.InstallCommand = userCfg.InstallCommand
	}
	if userCfg.DevCommand != "" {
		cfg.DevCommand = userCfg.DevCommand
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

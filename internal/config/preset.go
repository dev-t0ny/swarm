package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ProjectType identifies the detected project framework/language.
type ProjectType string

const (
	ProjectNextJS  ProjectType = "Next.js"
	ProjectVite    ProjectType = "Vite"
	ProjectNode    ProjectType = "Node.js"
	ProjectGo      ProjectType = "Go"
	ProjectRust    ProjectType = "Rust"
	ProjectPython  ProjectType = "Python"
	ProjectRuby    ProjectType = "Ruby"
	ProjectUnknown ProjectType = ""
)

// Preset holds the auto-detected defaults for a project type.
type Preset struct {
	Type           ProjectType
	InstallCommand string
	DevCommand     string
	Symlinks       []string
}

// knownPresets maps project types to their default configs.
var knownPresets = map[ProjectType]Preset{
	ProjectNextJS: {
		Type:           ProjectNextJS,
		InstallCommand: "npm install",
		DevCommand:     "npm run dev -- --port {port}",
		Symlinks:       []string{".env", ".env.local"},
	},
	ProjectVite: {
		Type:           ProjectVite,
		InstallCommand: "npm install",
		DevCommand:     "npm run dev -- --port {port}",
		Symlinks:       []string{".env", ".env.local"},
	},
	ProjectNode: {
		Type:           ProjectNode,
		InstallCommand: "npm install",
		DevCommand:     "npm run dev -- --port {port}",
		Symlinks:       []string{".env", ".env.local"},
	},
	ProjectGo: {
		Type:           ProjectGo,
		InstallCommand: "",
		DevCommand:     "",
		Symlinks:       []string{".env"},
	},
	ProjectRust: {
		Type:           ProjectRust,
		InstallCommand: "cargo build",
		DevCommand:     "",
		Symlinks:       []string{".env"},
	},
	ProjectPython: {
		Type:           ProjectPython,
		InstallCommand: "pip install -r requirements.txt",
		DevCommand:     "",
		Symlinks:       []string{".env"},
	},
	ProjectRuby: {
		Type:           ProjectRuby,
		InstallCommand: "bundle install",
		DevCommand:     "bundle exec rails server --port {port}",
		Symlinks:       []string{".env"},
	},
}

// DetectProject inspects the repo root and returns the detected project type.
func DetectProject(repoRoot string) ProjectType {
	// Check for package.json first (most common)
	if hasPkgJSON(repoRoot) {
		return detectNodeFramework(repoRoot)
	}

	// Go
	if fileExists(repoRoot, "go.mod") {
		return ProjectGo
	}

	// Rust
	if fileExists(repoRoot, "Cargo.toml") {
		return ProjectRust
	}

	// Python
	if fileExists(repoRoot, "requirements.txt") || fileExists(repoRoot, "pyproject.toml") || fileExists(repoRoot, "setup.py") {
		return ProjectPython
	}

	// Ruby
	if fileExists(repoRoot, "Gemfile") {
		return ProjectRuby
	}

	return ProjectUnknown
}

// GetPreset returns the preset for a given project type.
func GetPreset(pt ProjectType) Preset {
	if p, ok := knownPresets[pt]; ok {
		return p
	}
	return Preset{Type: ProjectUnknown}
}

// detectNodeFramework reads package.json to identify the specific framework.
func detectNodeFramework(repoRoot string) ProjectType {
	data, err := os.ReadFile(filepath.Join(repoRoot, "package.json"))
	if err != nil {
		return ProjectNode
	}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ProjectNode
	}

	allDeps := make(map[string]bool)
	for k := range pkg.Dependencies {
		allDeps[k] = true
	}
	for k := range pkg.DevDependencies {
		allDeps[k] = true
	}

	// Check for specific frameworks (order matters — most specific first)
	if allDeps["next"] {
		return ProjectNextJS
	}
	if allDeps["vite"] {
		return ProjectVite
	}

	// Check scripts for vite usage (some projects only have vite in scripts)
	var scripts struct {
		Scripts map[string]string `json:"scripts"`
	}
	if json.Unmarshal(data, &scripts) == nil {
		for _, cmd := range scripts.Scripts {
			if strings.Contains(cmd, "vite") {
				return ProjectVite
			}
		}
	}

	return ProjectNode
}

func fileExists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

func hasPkgJSON(dir string) bool {
	return fileExists(dir, "package.json")
}

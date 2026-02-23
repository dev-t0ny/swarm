package deps

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Installer handles dependency installation in worktrees.
type Installer struct {
	Command string
}

// NewInstaller creates a new dependency installer.
func NewInstaller(command string) *Installer {
	return &Installer{Command: command}
}

// NeedsInstall checks if the worktree has a package.json (or similar)
// that would require dependency installation.
func (i *Installer) NeedsInstall(worktreeDir string) bool {
	indicators := []string{
		"package.json",
		"Gemfile",
		"requirements.txt",
		"pyproject.toml",
		"Cargo.toml",
		"go.mod",
	}
	for _, f := range indicators {
		if _, err := os.Stat(filepath.Join(worktreeDir, f)); err == nil {
			return true
		}
	}
	return false
}

// Install runs the install command in the given directory.
// Returns the combined output and any error.
func (i *Installer) Install(worktreeDir string) (string, error) {
	if i.Command == "" {
		return "", nil
	}

	parts := strings.Fields(i.Command)
	if len(parts) == 0 {
		return "", nil
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = worktreeDir
	cmd.Env = append(os.Environ(), "CI=true") // Avoid interactive prompts

	output, err := cmd.CombinedOutput()
	return string(output), err
}

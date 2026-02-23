package deps

// Installer handles dependency installation in worktrees.
type Installer struct {
	Command string
}

// NewInstaller creates a new dependency installer.
func NewInstaller(command string) *Installer {
	return &Installer{Command: command}
}

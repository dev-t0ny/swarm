package git

// Manager handles git worktree operations.
type Manager struct {
	RepoRoot   string
	SwarmDir   string
}

// NewManager creates a new worktree manager.
func NewManager(repoRoot string) *Manager {
	return &Manager{
		RepoRoot: repoRoot,
		SwarmDir: repoRoot + "/.swarm",
	}
}

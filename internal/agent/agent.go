package agent

// Type represents a supported AI agent.
type Type struct {
	Name        string
	Command     string
	MergePrompt string
}

// DefaultAgents returns the built-in agent type definitions.
func DefaultAgents() []Type {
	return []Type{
		{Name: "claude", Command: "claude", MergePrompt: "Merge your changes into {base_branch}. Resolve any conflicts. Confirm when done."},
		{Name: "opencode", Command: "opencode", MergePrompt: "Merge into {base_branch}, resolve all conflicts, confirm when done."},
		{Name: "codex", Command: "codex", MergePrompt: "Merge into {base_branch} and resolve any conflicts."},
		{Name: "shell", Command: "", MergePrompt: ""},
	}
}

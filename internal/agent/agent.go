package agent

import (
	"os/exec"
	"strings"
)

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

// AvailableAgents returns only agents whose commands are found on the system.
// Shell is always available.
func AvailableAgents() []Type {
	var available []Type
	for _, a := range DefaultAgents() {
		if a.Name == "shell" {
			available = append(available, a)
			continue
		}
		if _, err := exec.LookPath(a.Command); err == nil {
			available = append(available, a)
		}
	}
	return available
}

// AllAgents returns all agents, marking unavailable ones.
// Returns (agent, isAvailable) pairs.
func AllAgents() []struct {
	Agent     Type
	Available bool
} {
	var result []struct {
		Agent     Type
		Available bool
	}
	for _, a := range DefaultAgents() {
		available := a.Name == "shell"
		if !available {
			_, err := exec.LookPath(a.Command)
			available = err == nil
		}
		result = append(result, struct {
			Agent     Type
			Available bool
		}{Agent: a, Available: available})
	}
	return result
}

// FormatMergePrompt replaces template variables in the merge prompt.
func (t *Type) FormatMergePrompt(baseBranch string) string {
	return strings.ReplaceAll(t.MergePrompt, "{base_branch}", baseBranch)
}

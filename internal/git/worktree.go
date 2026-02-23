package git

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// WorktreeInfo represents an existing worktree.
type WorktreeInfo struct {
	Path   string
	Branch string
}

// Manager handles git worktree operations.
type Manager struct {
	RepoRoot string
	SwarmDir string
}

// NewManager creates a new worktree manager.
func NewManager(repoRoot string) *Manager {
	return &Manager{
		RepoRoot: repoRoot,
		SwarmDir: filepath.Join(repoRoot, ".swarm"),
	}
}

// EnsureSwarmDir creates the .swarm directory if it doesn't exist
// and adds it to .gitignore if not already there.
func (m *Manager) EnsureSwarmDir() error {
	if err := os.MkdirAll(m.SwarmDir, 0755); err != nil {
		return fmt.Errorf("create .swarm directory: %w", err)
	}

	// Ensure .swarm/ is in .gitignore
	gitignorePath := filepath.Join(m.RepoRoot, ".gitignore")
	if !m.isInGitignore(gitignorePath) {
		f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("open .gitignore: %w", err)
		}
		defer f.Close()

		// Add newline before entry if file is non-empty
		info, _ := f.Stat()
		if info != nil && info.Size() > 0 {
			if _, err := f.WriteString("\n"); err != nil {
				return err
			}
		}
		if _, err := f.WriteString("# Swarm worktrees\n.swarm/\n"); err != nil {
			return fmt.Errorf("write .gitignore: %w", err)
		}
	}

	return nil
}

// isInGitignore checks if .swarm/ is already in .gitignore.
func (m *Manager) isInGitignore(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == ".swarm/" || line == ".swarm" {
			return true
		}
	}
	return false
}

// CreateWorktree creates a new git worktree with a new branch.
// Returns the absolute path to the created worktree.
func (m *Manager) CreateWorktree(name string, baseBranch string) (string, string, error) {
	worktreePath := filepath.Join(m.SwarmDir, name)
	branchName := "swarm/" + name

	// Get the current branch if baseBranch is empty
	if baseBranch == "" {
		out, err := exec.Command("git", "-C", m.RepoRoot, "rev-parse", "--abbrev-ref", "HEAD").Output()
		if err != nil {
			return "", "", fmt.Errorf("get current branch: %w", err)
		}
		baseBranch = strings.TrimSpace(string(out))
	}

	// Create the worktree with a new branch
	cmd := exec.Command("git", "-C", m.RepoRoot, "worktree", "add", "-b", branchName, worktreePath, baseBranch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("create worktree: %s: %w", strings.TrimSpace(string(output)), err)
	}

	return worktreePath, branchName, nil
}

// RemoveWorktree removes a worktree and its branch.
func (m *Manager) RemoveWorktree(name string, deleteBranch bool) error {
	worktreePath := filepath.Join(m.SwarmDir, name)
	branchName := "swarm/" + name

	// Remove the worktree
	cmd := exec.Command("git", "-C", m.RepoRoot, "worktree", "remove", "--force", worktreePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If worktree remove fails, try manual cleanup
		_ = os.RemoveAll(worktreePath)
		// Prune stale worktree entries
		_ = exec.Command("git", "-C", m.RepoRoot, "worktree", "prune").Run()
	}
	_ = output

	// Delete the branch if requested
	if deleteBranch {
		_ = exec.Command("git", "-C", m.RepoRoot, "branch", "-D", branchName).Run()
	}

	return nil
}

// ListWorktrees returns all swarm-managed worktrees.
func (m *Manager) ListWorktrees() ([]WorktreeInfo, error) {
	cmd := exec.Command("git", "-C", m.RepoRoot, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}

	var worktrees []WorktreeInfo
	var current WorktreeInfo
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "worktree ") {
			current = WorktreeInfo{Path: strings.TrimPrefix(line, "worktree ")}
		} else if strings.HasPrefix(line, "branch ") {
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		} else if line == "" {
			// Only include worktrees inside .swarm/
			if strings.HasPrefix(current.Path, m.SwarmDir) {
				worktrees = append(worktrees, current)
			}
			current = WorktreeInfo{}
		}
	}
	// Handle last entry if file doesn't end with blank line
	if current.Path != "" && strings.HasPrefix(current.Path, m.SwarmDir) {
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

// RemoveAllWorktrees removes all swarm-managed worktrees and their branches.
func (m *Manager) RemoveAllWorktrees() error {
	worktrees, err := m.ListWorktrees()
	if err != nil {
		return err
	}

	for _, wt := range worktrees {
		name := filepath.Base(wt.Path)
		if err := m.RemoveWorktree(name, true); err != nil {
			// Continue cleanup even if one fails
			continue
		}
	}

	// Remove the .swarm directory itself
	_ = os.RemoveAll(m.SwarmDir)

	return nil
}

// GetCurrentBranch returns the current branch name.
func (m *Manager) GetCurrentBranch() (string, error) {
	out, err := exec.Command("git", "-C", m.RepoRoot, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("get current branch: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dev-t0ny/swarm/internal/tmux"
	"github.com/dev-t0ny/swarm/internal/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "swarm",
	Short: "Parallel AI agent workspace manager",
	Long:  "Swarm manages multiple AI coding agents in parallel using git worktrees and tmux.",
	RunE:  runSwarm,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func runSwarm(cmd *cobra.Command, args []string) error {
	// Step 1: Verify we're in a git repo
	repoRoot, err := findGitRoot()
	if err != nil {
		return fmt.Errorf("not a git repository (or any parent): run swarm from inside a git repo")
	}

	// Step 2: Check tmux is installed
	if !tmux.IsInstalled() {
		return fmt.Errorf("tmux is not installed. Install it with: brew install tmux")
	}

	// Step 3: Determine session name from repo directory name
	repoName := filepath.Base(repoRoot)
	sessionName := "swarm-" + sanitizeSessionName(repoName)
	driver := tmux.NewDriver(sessionName)

	// Step 4: If we're NOT inside tmux, bootstrap into a tmux session
	if !tmux.IsInsideSession() {
		return bootstrapTmux(driver, repoRoot)
	}

	// Step 5: We're inside tmux — get our pane ID and resize to sidebar width
	paneID, err := tmux.GetCurrentPaneID()
	if err != nil {
		return fmt.Errorf("failed to get current pane ID: %w", err)
	}

	// Resize the swarm pane to be a sidebar (30 columns)
	_ = driver.ResizePane(paneID, 34)

	// Step 6: Launch the TUI
	return tui.Run(repoRoot, repoName, driver, paneID)
}

// bootstrapTmux creates a tmux session (or reattaches) and re-execs swarm inside it.
func bootstrapTmux(driver *tmux.Driver, repoRoot string) error {
	if driver.SessionExists() {
		// Session already exists — just attach to it
		fmt.Printf("Reattaching to existing session '%s'...\n", driver.SessionName)
		return driver.AttachSession()
	}

	// Find our own binary path so we can re-exec inside tmux
	selfPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to find swarm executable: %w", err)
	}

	// Create a new detached session
	if err := driver.CreateSession(repoRoot); err != nil {
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	// Send the swarm command into the session so it runs the TUI inside tmux
	panes, err := driver.ListPanes()
	if err != nil || len(panes) == 0 {
		return fmt.Errorf("failed to list panes in new session: %w", err)
	}

	if err := driver.RunInPane(panes[0].ID, selfPath); err != nil {
		return fmt.Errorf("failed to launch swarm in tmux: %w", err)
	}

	// Attach to the session (replaces current process)
	return driver.AttachSession()
}

// findGitRoot finds the root of the current git repository.
func findGitRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// sanitizeSessionName cleans a string for use as a tmux session name.
func sanitizeSessionName(name string) string {
	// tmux session names can't contain dots or colons
	replacer := strings.NewReplacer(".", "-", ":", "-", " ", "-")
	return replacer.Replace(name)
}

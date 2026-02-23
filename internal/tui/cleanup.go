package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	gitpkg "github.com/dev-t0ny/swarm/internal/git"
)

// cleanupState tracks the sub-state of the cleanup flow.
type cleanupState int

const (
	cleanupConfirm cleanupState = iota
	cleanupRunning
)

// cleanupModel holds state for the cleanup dialog.
type cleanupModel struct {
	cursor  int
	state   cleanupState
	message string
}

func newCleanupModel() cleanupModel {
	return cleanupModel{
		state: cleanupConfirm,
	}
}

// --- Messages ---

type cleanupDoneMsg struct {
	err error
}

// updateCleanup handles input for the cleanup dialog.
func (a *App) updateCleanup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch a.cleanup.state {
	case cleanupConfirm:
		switch {
		case msg.String() == "y" || msg.String() == "Y":
			a.cleanup.state = cleanupRunning
			a.cleanup.message = "Cleaning up all agents..."
			return a, a.cleanupAllCmd()
		case msg.String() == "n" || msg.String() == "N" || key.Matches(msg, a.keys.Back):
			a.screen = ScreenDashboard
			a.cleanup = newCleanupModel()
		}
	}
	return a, nil
}

// cleanupAllCmd returns a tea.Cmd that tears down everything.
// Captures all needed state before the closure.
func (a *App) cleanupAllCmd() tea.Cmd {
	agents := make([]AgentInstance, len(a.agents))
	copy(agents, a.agents)
	repoRoot := a.repoRoot
	tmuxDriver := a.tmux
	swarmPaneID := a.swarmPaneID

	return func() tea.Msg {
		// Kill all agent panes (and their dev server panes)
		for _, ag := range agents {
			if ag.DevPaneID != "" {
				_ = tmuxDriver.KillPane(ag.DevPaneID)
			}
			_ = tmuxDriver.KillPane(ag.PaneID)
		}

		// Refocus control pane (only pane left)
		_ = tmuxDriver.SelectPane(swarmPaneID)

		// Remove all worktrees and branches
		gitMgr := gitpkg.NewManager(repoRoot)
		err := gitMgr.RemoveAllWorktrees()

		return cleanupDoneMsg{err: err}
	}
}

// handleCleanupDone processes the result of cleanup.
func (a *App) handleCleanupDone(msg cleanupDoneMsg) (tea.Model, tea.Cmd) {
	// Clear all state
	a.agents = []AgentInstance{}
	a.cursor = 0
	a.ports.ReleaseAll()
	a.nextAgentNum = 1

	// Hide pane borders — back to single pane
	_ = a.tmux.DisablePaneBorders()

	if msg.err != nil {
		a.statusMsg = fmt.Sprintf("Cleanup completed with warnings: %v", msg.err)
	} else {
		a.statusMsg = "All agents cleaned up"
	}

	a.screen = ScreenDashboard
	a.cleanup = newCleanupModel()
	return a, nil
}

// viewCleanup renders the cleanup confirmation dialog.
func (a *App) viewCleanup() string {
	switch a.cleanup.state {
	case cleanupRunning:
		return a.viewCleanupProgress()
	default:
		return a.viewCleanupConfirm()
	}
}

func (a *App) viewCleanupConfirm() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Cleanup All"))
	b.WriteString("\n\n")

	// Show what will be destroyed
	b.WriteString(warningStyle.Render("  This will:"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s Kill %d agent(s)\n", dangerStyle.Render("*"), len(a.agents)))

	devCount := 0
	for _, agent := range a.agents {
		if agent.DevPaneID != "" {
			devCount++
		}
	}
	if devCount > 0 {
		b.WriteString(fmt.Sprintf("  %s Kill %d dev server(s)\n", dangerStyle.Render("*"), devCount))
	}

	b.WriteString(fmt.Sprintf("  %s Remove all worktrees in .swarm/\n", dangerStyle.Render("*")))
	b.WriteString(fmt.Sprintf("  %s Delete all swarm/* branches\n", dangerStyle.Render("*")))
	b.WriteString("\n")

	// List agents that will be affected
	b.WriteString(sectionStyle.Render("  Agents to remove:"))
	b.WriteString("\n")
	for _, agent := range a.agents {
		b.WriteString(fmt.Sprintf("    %s %s (%s)\n", dangerStyle.Render("-"), agent.Name, agent.Branch))
	}
	b.WriteString("\n")

	b.WriteString(fmt.Sprintf("  %s  /  %s\n", keyStyle.Render("[y] confirm"), descStyle.Render("[n] cancel")))

	return b.String()
}

func (a *App) viewCleanupProgress() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Cleanup All"))
	b.WriteString("\n\n")

	icon := warningStyle.Render("◐")
	b.WriteString(fmt.Sprintf("  %s %s", icon, a.cleanup.message))
	b.WriteString("\n")

	return b.String()
}

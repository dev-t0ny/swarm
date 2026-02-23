package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dev-t0ny/swarm/internal/agent"
	"github.com/dev-t0ny/swarm/internal/deps"
	gitpkg "github.com/dev-t0ny/swarm/internal/git"
	"github.com/dev-t0ny/swarm/internal/link"
)

// newAgentState tracks the sub-state of the new agent flow.
type newAgentState int

const (
	newAgentPicking newAgentState = iota
	newAgentCreating
	newAgentInstalling
	newAgentDone
)

// newAgentModel holds state for the new agent dialog.
type newAgentModel struct {
	agents   []agent.Type
	cursor   int
	state    newAgentState
	message  string
}

func newNewAgentModel() newAgentModel {
	allAgents := agent.AllAgents()
	var types []agent.Type
	for _, a := range allAgents {
		types = append(types, a.Agent)
	}
	return newAgentModel{
		agents: types,
		state:  newAgentPicking,
	}
}

// --- Messages for async operations ---

type agentCreatedMsg struct {
	instance AgentInstance
	err      error
}

type depsInstalledMsg struct {
	agentName string
	err       error
}

// updateNewAgent handles input for the new agent dialog.
func (a *App) updateNewAgent(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch a.newAgent.state {
	case newAgentPicking:
		switch {
		case key.Matches(msg, a.keys.Up):
			if a.newAgent.cursor > 0 {
				a.newAgent.cursor--
			}
		case key.Matches(msg, a.keys.Down):
			if a.newAgent.cursor < len(a.newAgent.agents)-1 {
				a.newAgent.cursor++
			}
		case key.Matches(msg, a.keys.Focus): // Enter
			selected := a.newAgent.agents[a.newAgent.cursor]
			a.newAgent.state = newAgentCreating
			a.newAgent.message = fmt.Sprintf("Creating worktree for %s...", selected.Name)
			return a, a.createAgentCmd(selected)
		case key.Matches(msg, a.keys.Back):
			a.screen = ScreenDashboard
			a.newAgent = newNewAgentModel()
		}
	case newAgentCreating, newAgentInstalling:
		// Don't allow input while creating/installing
		if key.Matches(msg, a.keys.Back) {
			// Allow cancel? For now just ignore
		}
	}
	return a, nil
}

// createAgentCmd returns a tea.Cmd that creates a worktree, symlinks, and opens a tmux pane.
func (a *App) createAgentCmd(agentType agent.Type) tea.Cmd {
	return func() tea.Msg {
		agentName := fmt.Sprintf("agent-%d", a.nextAgentNum)

		// 1. Create worktree
		gitMgr := gitpkg.NewManager(a.repoRoot)
		if err := gitMgr.EnsureSwarmDir(); err != nil {
			return agentCreatedMsg{err: fmt.Errorf("ensure .swarm dir: %w", err)}
		}

		worktreePath, branchName, err := gitMgr.CreateWorktree(agentName, "")
		if err != nil {
			return agentCreatedMsg{err: fmt.Errorf("create worktree: %w", err)}
		}

		// 2. Symlink .env files
		linker := link.NewLinker(a.repoRoot, []string{".env", ".env.local"})
		linker.LinkTo(worktreePath)

		// 3. Create tmux pane
		paneID, err := a.tmux.SplitWindowH(a.swarmPaneID, worktreePath)
		if err != nil {
			return agentCreatedMsg{err: fmt.Errorf("create tmux pane: %w", err)}
		}

		// 4. Label the pane so it's clear which worktree it belongs to
		paneTitle := fmt.Sprintf("%s (%s) %s", agentName, agentType.Name, branchName)
		_ = a.tmux.SetPaneTitle(paneID, paneTitle)

		// Enable pane border titles on first agent creation
		_ = a.tmux.EnablePaneTitles()
		// Also title the swarm control pane
		_ = a.tmux.SetPaneTitle(a.swarmPaneID, "swarm")

		// 5. Print a banner in the pane showing context
		_ = a.tmux.PrintBanner(paneID, []string{
			fmt.Sprintf("=== %s ===", agentName),
			fmt.Sprintf("  Agent:    %s", agentType.Name),
			fmt.Sprintf("  Branch:   %s", branchName),
			fmt.Sprintf("  Worktree: %s", worktreePath),
			"===",
		})

		// 6. Launch agent in the pane (if not shell)
		if agentType.Command != "" {
			_ = a.tmux.RunInPane(paneID, agentType.Command)
		}

		instance := AgentInstance{
			Name:      agentName,
			AgentType: agentType.Name,
			Branch:    branchName,
			WorkDir:   worktreePath,
			PaneID:    paneID,
			Status:    AgentRunning,
		}

		return agentCreatedMsg{instance: instance}
	}
}

// installDepsCmd returns a tea.Cmd that runs npm install in a worktree.
func (a *App) installDepsCmd(agentName string, worktreeDir string) tea.Cmd {
	return func() tea.Msg {
		installer := deps.NewInstaller("npm install")
		if installer.NeedsInstall(worktreeDir) {
			_, err := installer.Install(worktreeDir)
			return depsInstalledMsg{agentName: agentName, err: err}
		}
		return depsInstalledMsg{agentName: agentName}
	}
}

// handleAgentCreated processes the result of creating an agent.
func (a *App) handleAgentCreated(msg agentCreatedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		a.statusMsg = fmt.Sprintf("Error: %v", msg.err)
		a.screen = ScreenDashboard
		a.newAgent = newNewAgentModel()
		return a, nil
	}

	a.agents = append(a.agents, msg.instance)
	a.nextAgentNum++
	a.cursor = len(a.agents) - 1

	// Start dependency installation in background
	a.newAgent.state = newAgentInstalling
	a.newAgent.message = fmt.Sprintf("Installing dependencies for %s...", msg.instance.Name)

	return a, a.installDepsCmd(msg.instance.Name, msg.instance.WorkDir)
}

// handleDepsInstalled processes the result of dependency installation.
func (a *App) handleDepsInstalled(msg depsInstalledMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		a.statusMsg = fmt.Sprintf("Deps install warning for %s: %v", msg.agentName, msg.err)
	}

	// Done — return to dashboard
	a.screen = ScreenDashboard
	a.newAgent = newNewAgentModel()
	a.statusMsg = ""
	return a, nil
}

// viewNewAgent renders the new agent dialog.
func (a *App) viewNewAgent() string {
	switch a.newAgent.state {
	case newAgentCreating, newAgentInstalling:
		return a.viewNewAgentProgress()
	default:
		return a.viewNewAgentPicker()
	}
}

func (a *App) viewNewAgentPicker() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("New Agent"))
	b.WriteString("\n\n")
	b.WriteString(descStyle.Render("Select an agent type:"))
	b.WriteString("\n\n")

	allAgents := agent.AllAgents()
	for i, entry := range allAgents {
		name := entry.Agent.Name
		available := entry.Available

		var line string
		if i == a.newAgent.cursor {
			if available {
				line = selectedStyle.Render(fmt.Sprintf(" %s ", name))
			} else {
				line = lipgloss.NewStyle().
					Bold(true).
					Foreground(mutedColor).
					Background(lipgloss.Color("#374151")).
					Padding(0, 1).
					Render(fmt.Sprintf(" %s (not installed) ", name))
			}
		} else {
			if available {
				line = fmt.Sprintf("  %s", activeAgentStyle.Render(name))
			} else {
				line = fmt.Sprintf("  %s %s", idleAgentStyle.Render(name), descStyle.Render("(not installed)"))
			}
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(renderKeyHint("enter", "select"))
	b.WriteString(renderKeyHint("esc", "cancel"))

	return b.String()
}

func (a *App) viewNewAgentProgress() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("New Agent"))
	b.WriteString("\n\n")

	// Show spinner-like message
	icon := warningStyle.Render("◐")
	b.WriteString(fmt.Sprintf("  %s %s", icon, a.newAgent.message))
	b.WriteString("\n")

	return b.String()
}

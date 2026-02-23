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

// agentOption is an agent type with its availability status.
type agentOption struct {
	agent.Type
	Available bool
}

// newAgentModel holds state for the new agent dialog.
type newAgentModel struct {
	agents   []agentOption
	cursor   int
	state    newAgentState
	message  string
}

func newNewAgentModel() newAgentModel {
	allAgents := agent.AllAgents()
	var options []agentOption
	for _, a := range allAgents {
		options = append(options, agentOption{Type: a.Agent, Available: a.Available})
	}
	return newAgentModel{
		agents: options,
		state:  newAgentPicking,
	}
}

// --- Messages for async operations ---

type agentCreatedMsg struct {
	instance   AgentInstance
	linkErrors []string
	err        error
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
			if !selected.Available {
				// Can't select unavailable agents
				return a, nil
			}
			a.newAgent.state = newAgentCreating
			a.newAgent.message = fmt.Sprintf("Creating worktree for %s...", selected.Name)
			return a, a.createAgentCmd(selected.Type)
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
// All mutable App state is captured into local variables before the closure to avoid races.
func (a *App) createAgentCmd(agentType agent.Type) tea.Cmd {
	// Capture all state needed by the closure BEFORE returning the Cmd.
	agentName := fmt.Sprintf("agent-%d", a.nextAgentNum)
	repoRoot := a.repoRoot
	symlinks := a.cfg.Symlinks
	swarmPaneID := a.swarmPaneID
	tmuxDriver := a.tmux
	installCmd := a.cfg.InstallCommand

	// Pre-increment to avoid race if user creates multiple agents rapidly.
	a.nextAgentNum++

	return func() tea.Msg {
		// 1. Create worktree
		gitMgr := gitpkg.NewManager(repoRoot)
		if err := gitMgr.EnsureSwarmDir(); err != nil {
			return agentCreatedMsg{err: fmt.Errorf("ensure .swarm dir: %w", err)}
		}

		worktreePath, branchName, err := gitMgr.CreateWorktree(agentName, "")
		if err != nil {
			return agentCreatedMsg{err: fmt.Errorf("create worktree: %w", err)}
		}

		// 2. Symlink configured files (.env, etc.)
		linker := link.NewLinker(repoRoot, symlinks)
		results := linker.LinkTo(worktreePath)
		var linkErrors []string
		for _, r := range results {
			if r.Error != nil {
				linkErrors = append(linkErrors, fmt.Sprintf("%s: %v", r.File, r.Error))
			}
		}

		// 3. Create tmux pane
		paneID, err := tmuxDriver.SplitWindowH(swarmPaneID, worktreePath)
		if err != nil {
			// Rollback: clean up orphaned worktree
			_ = gitMgr.RemoveWorktree(agentName, true)
			return agentCreatedMsg{err: fmt.Errorf("create tmux pane: %w", err)}
		}

		// 4. Label the pane so it's clear which worktree it belongs to
		paneTitle := fmt.Sprintf("%s (%s) %s", agentName, agentType.Name, branchName)
		_ = tmuxDriver.SetPaneTitle(paneID, paneTitle)

		// Enable pane border titles on first agent creation
		_ = tmuxDriver.EnablePaneTitles()
		// Also title the swarm control pane
		_ = tmuxDriver.SetPaneTitle(swarmPaneID, "swarm")

		// 5. Print a banner in the pane showing context
		_ = tmuxDriver.PrintBanner(paneID, []string{
			fmt.Sprintf("=== %s ===", agentName),
			fmt.Sprintf("  Agent:    %s", agentType.Name),
			fmt.Sprintf("  Branch:   %s", branchName),
			fmt.Sprintf("  Worktree: %s", worktreePath),
			"===",
		})

		// 6. Launch agent in the pane (if not shell)
		if agentType.Command != "" {
			_ = tmuxDriver.RunInPane(paneID, agentType.Command)
		}

		instance := AgentInstance{
			Name:         agentName,
			AgentType:    agentType.Name,
			Branch:       branchName,
			WorkDir:      worktreePath,
			PaneID:       paneID,
			Status:       AgentRunning,
			installCmd:   installCmd,
		}

		return agentCreatedMsg{instance: instance, linkErrors: linkErrors}
	}
}

// installDepsCmd returns a tea.Cmd that runs npm install in a worktree.
// installCommand is captured before the closure to avoid reading App state from goroutine.
func installDepsCmd(agentName string, worktreeDir string, installCommand string) tea.Cmd {
	return func() tea.Msg {
		installer := deps.NewInstaller(installCommand)
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
	a.cursor = len(a.agents) - 1

	// Show symlink warnings if any
	if len(msg.linkErrors) > 0 {
		a.statusMsg = fmt.Sprintf("Symlink warnings: %v", msg.linkErrors)
	}

	// Start dependency installation in background
	a.newAgent.state = newAgentInstalling
	a.newAgent.message = fmt.Sprintf("Installing dependencies for %s...", msg.instance.Name)

	return a, installDepsCmd(msg.instance.Name, msg.instance.WorkDir, msg.instance.installCmd)
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

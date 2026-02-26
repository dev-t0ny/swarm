package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dev-t0ny/swarm/internal/agent"
	gitpkg "github.com/dev-t0ny/swarm/internal/git"
	"github.com/dev-t0ny/swarm/internal/link"
)

// newAgentState tracks the sub-state of the new agent flow.
type newAgentState int

const (
	newAgentPicking newAgentState = iota
	newAgentCreating
)

// agentOption is an agent type with its availability status.
type agentOption struct {
	agent.Type
	Available bool
}

// newAgentModel holds state for the new agent dialog.
type newAgentModel struct {
	agents  []agentOption
	cursor  int
	state   newAgentState
	message string
	spinner spinner.Model
}

func newNewAgentModel() newAgentModel {
	allAgents := agent.AllAgents()
	var options []agentOption
	for _, a := range allAgents {
		options = append(options, agentOption{Type: a.Agent, Available: a.Available})
	}
	return newAgentModel{
		agents:  options,
		state:   newAgentPicking,
		spinner: newSwarmSpinner(),
	}
}

// --- Messages for async operations ---

type agentCreatedMsg struct {
	instance   AgentInstance
	linkErrors []string
	err        error
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
			return a, tea.Batch(a.newAgent.spinner.Tick, a.createAgentCmd(selected.Type))
		case key.Matches(msg, a.keys.Back):
			a.screen = ScreenDashboard
			a.newAgent = newNewAgentModel()
		}
	case newAgentCreating:
		// Don't allow input while creating
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
	tmuxDriver := a.tmux
	swarmPaneID := a.swarmPaneID

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

		// 3. Create a new pane by splitting from the swarm control pane
		paneID, err := tmuxDriver.SplitWindowH(swarmPaneID, worktreePath)
		if err != nil {
			_ = gitMgr.RemoveWorktree(agentName, true)
			return agentCreatedMsg{err: fmt.Errorf("create tmux pane: %w", err)}
		}

		// 4. Apply tiled layout so all panes form a grid, then shrink the swarm pane
		_ = tmuxDriver.ApplyTiledLayout()
		_ = tmuxDriver.SetPaneWidth(swarmPaneID, swarmPaneWidth)

		// 5. Label the pane
		paneTitle := fmt.Sprintf("%s  %s  %s", agentName, agentType.Name, branchName)
		_ = tmuxDriver.SetPaneTitle(paneID, paneTitle)

		// 6. Print a banner in the pane showing context
		_ = tmuxDriver.PrintBanner(paneID, []string{
			fmt.Sprintf("  %s", agentName),
			fmt.Sprintf("  Agent:    %s", agentType.Name),
			fmt.Sprintf("  Branch:   %s", branchName),
			fmt.Sprintf("  Worktree: %s", worktreePath),
			"",
		})

		// 7. Launch agent in the pane (if not shell)
		if agentType.Command != "" {
			_ = tmuxDriver.RunInPane(paneID, agentType.Command)
		}

		// 8. Refocus the swarm control pane so the TUI stays active
		_ = tmuxDriver.SelectPane(swarmPaneID)

		instance := AgentInstance{
			Name:      agentName,
			AgentType: agentType.Name,
			Branch:    branchName,
			WorkDir:   worktreePath,
			PaneID:    paneID,
			Status:    AgentRunning,
		}

		return agentCreatedMsg{instance: instance, linkErrors: linkErrors}
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

	// Enable pane borders when the first agent is created
	if len(a.agents) == 0 {
		_ = a.tmux.EnablePaneBorders()
	}

	a.agents = append(a.agents, msg.instance)
	a.cursor = len(a.agents) - 1

	// Show symlink warnings if any
	if len(msg.linkErrors) > 0 {
		a.statusMsg = fmt.Sprintf("Symlink warnings: %v", msg.linkErrors)
	} else {
		a.statusMsg = ""
	}

	a.screen = ScreenDashboard
	a.newAgent = newNewAgentModel()
	return a, nil
}

// viewNewAgent renders the new agent dialog.
func (a *App) viewNewAgent() string {
	switch a.newAgent.state {
	case newAgentCreating:
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
	b.WriteString("  ")
	b.WriteString(hintBar("enter", "select", "esc", "cancel"))
	b.WriteString("\n")

	return b.String()
}

func (a *App) viewNewAgentProgress() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("New Agent"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("  %s %s", a.newAgent.spinner.View(), descStyle.Render(a.newAgent.message)))
	b.WriteString("\n")

	return b.String()
}

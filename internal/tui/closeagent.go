package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/dev-t0ny/swarm/internal/config"
	gitpkg "github.com/dev-t0ny/swarm/internal/git"
	"github.com/dev-t0ny/swarm/internal/port"
	"github.com/dev-t0ny/swarm/internal/tmux"
)

// closeAction represents the user's choice for closing an agent.
type closeAction int

const (
	closeApply closeAction = iota
	closeKeep
	closeDrop
)

// closeAgentModel holds state for the close agent dialog.
type closeAgentModel struct {
	cursor int
}

func newCloseAgentModel() closeAgentModel {
	return closeAgentModel{}
}

var closeOptions = []struct {
	key    string
	label  string
	desc   string
	action closeAction
}{
	{"a", "Apply", "Tell the agent to merge changes into main", closeApply},
	{"k", "Keep", "Save the branch, close the agent", closeKeep},
	{"x", "Drop", "Throw away all changes", closeDrop},
}

// --- Messages ---

type agentClosedMsg struct {
	agentName string
	action    closeAction
	err       error
}

// updateCloseAgent handles input for the close agent dialog.
func (a *App) updateCloseAgent(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, a.keys.Up):
		if a.closeAgent.cursor > 0 {
			a.closeAgent.cursor--
		}
	case key.Matches(msg, a.keys.Down):
		if a.closeAgent.cursor < len(closeOptions)-1 {
			a.closeAgent.cursor++
		}
	case key.Matches(msg, a.keys.Back):
		a.screen = ScreenDashboard
		a.closeAgent = newCloseAgentModel()
	case msg.String() == "a":
		return a, a.closeAgentCmd(closeApply)
	case msg.String() == "k":
		return a, a.closeAgentCmd(closeKeep)
	case msg.String() == "x":
		return a, a.closeAgentCmd(closeDrop)
	case key.Matches(msg, a.keys.Focus): // Enter
		selected := closeOptions[a.closeAgent.cursor]
		return a, a.closeAgentCmd(selected.action)
	}
	return a, nil
}

// closeAgentCmd returns a tea.Cmd that performs the chosen close action.
// Captures all needed state before the closure.
func (a *App) closeAgentCmd(action closeAction) tea.Cmd {
	if a.cursor >= len(a.agents) {
		return nil
	}
	agentInst := a.agents[a.cursor]
	repoRoot := a.repoRoot
	tmuxDriver := a.tmux
	ports := a.ports
	cfgAgents := a.cfg.Agents

	return func() tea.Msg {
		switch action {
		case closeApply:
			return doCloseApply(agentInst, repoRoot, tmuxDriver, cfgAgents)
		case closeKeep:
			return doCloseKeep(agentInst, tmuxDriver, ports)
		case closeDrop:
			return doCloseDrop(agentInst, repoRoot, tmuxDriver, ports)
		}
		return agentClosedMsg{agentName: agentInst.Name, action: action}
	}
}

// doCloseApply tells the agent to merge its changes.
func doCloseApply(agentInst AgentInstance, repoRoot string, tmuxDriver *tmux.Driver, cfgAgents map[string]config.Agent) agentClosedMsg {
	// Get the base branch
	gitMgr := gitpkg.NewManager(repoRoot)
	baseBranch, err := gitMgr.GetCurrentBranch()
	if err != nil {
		baseBranch = "main"
	}

	// Construct the merge prompt from config or default
	mergePrompt := fmt.Sprintf(
		"Merge your changes from branch %s into %s. Resolve any conflicts intelligently. Confirm when done.",
		agentInst.Branch, baseBranch,
	)
	if cfgAgent, ok := cfgAgents[agentInst.AgentType]; ok && cfgAgent.MergePrompt != "" {
		mergePrompt = strings.ReplaceAll(cfgAgent.MergePrompt, "{base_branch}", baseBranch)
	}

	// Focus the agent's pane and send the merge instruction
	_ = tmuxDriver.SelectPane(agentInst.PaneID)
	_ = tmuxDriver.SendKeys(agentInst.PaneID, mergePrompt)

	return agentClosedMsg{
		agentName: agentInst.Name,
		action:    closeApply,
	}
}

// doCloseKeep kills the pane but preserves the worktree and branch.
func doCloseKeep(agentInst AgentInstance, tmuxDriver *tmux.Driver, ports *port.Allocator) agentClosedMsg {
	// Kill dev server pane if running
	if agentInst.DevPaneID != "" {
		_ = tmuxDriver.KillPane(agentInst.DevPaneID)
		ports.ReleaseByAgent(agentInst.Name)
	}

	// Kill the agent pane
	_ = tmuxDriver.KillPane(agentInst.PaneID)

	return agentClosedMsg{
		agentName: agentInst.Name,
		action:    closeKeep,
	}
}

// doCloseDrop kills everything and deletes the worktree and branch.
func doCloseDrop(agentInst AgentInstance, repoRoot string, tmuxDriver *tmux.Driver, ports *port.Allocator) agentClosedMsg {
	// Kill dev server pane if running
	if agentInst.DevPaneID != "" {
		_ = tmuxDriver.KillPane(agentInst.DevPaneID)
		ports.ReleaseByAgent(agentInst.Name)
	}

	// Kill the agent pane
	_ = tmuxDriver.KillPane(agentInst.PaneID)

	// Remove the worktree and delete the branch
	gitMgr := gitpkg.NewManager(repoRoot)
	_ = gitMgr.RemoveWorktree(agentInst.Name, true)

	return agentClosedMsg{
		agentName: agentInst.Name,
		action:    closeDrop,
	}
}

// handleAgentClosed processes the result of closing an agent.
func (a *App) handleAgentClosed(msg agentClosedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		a.statusMsg = fmt.Sprintf("Error closing %s: %v", msg.agentName, msg.err)
		a.screen = ScreenDashboard
		a.closeAgent = newCloseAgentModel()
		return a, nil
	}

	switch msg.action {
	case closeApply:
		// Mark agent as merging, don't remove it
		for i, agent := range a.agents {
			if agent.Name == msg.agentName {
				a.agents[i].Status = AgentMerging
				break
			}
		}
		a.statusMsg = fmt.Sprintf("%s: merge prompt sent to agent", msg.agentName)

	case closeKeep:
		// Remove from agents list (pane is killed, but branch/worktree remain)
		a.removeAgent(msg.agentName)
		a.statusMsg = fmt.Sprintf("%s: branch preserved, worktree kept", msg.agentName)

	case closeDrop:
		// Remove from agents list (everything deleted)
		a.removeAgent(msg.agentName)
		a.statusMsg = fmt.Sprintf("%s: all changes discarded", msg.agentName)
	}

	a.screen = ScreenDashboard
	a.closeAgent = newCloseAgentModel()

	// Fix cursor if it's now out of bounds
	if a.cursor >= len(a.agents) && a.cursor > 0 {
		a.cursor = len(a.agents) - 1
	}

	return a, nil
}

// removeAgent removes an agent from the agents list by name.
func (a *App) removeAgent(name string) {
	for i, agent := range a.agents {
		if agent.Name == name {
			a.agents = append(a.agents[:i], a.agents[i+1:]...)
			return
		}
	}
}

// viewCloseAgent renders the close agent dialog.
func (a *App) viewCloseAgent() string {
	if a.cursor >= len(a.agents) {
		// Shouldn't happen — guard against it by showing dashboard
		return a.viewDashboard()
	}

	agent := a.agents[a.cursor]
	var b strings.Builder

	b.WriteString(titleStyle.Render("Close Agent"))
	b.WriteString("\n\n")

	// Agent info
	b.WriteString(fmt.Sprintf("  %s %s\n", sectionStyle.Render("Agent:"), agent.Name))
	b.WriteString(fmt.Sprintf("  %s %s\n", sectionStyle.Render("Type:"), agent.AgentType))
	b.WriteString(fmt.Sprintf("  %s %s\n", sectionStyle.Render("Branch:"), agent.Branch))
	b.WriteString("\n")

	b.WriteString(descStyle.Render("  What should happen to this agent's work?"))
	b.WriteString("\n\n")

	for i, opt := range closeOptions {
		var line string
		if i == a.closeAgent.cursor {
			line = selectedStyle.Render(fmt.Sprintf(" [%s] %s ", opt.key, opt.label))
		} else {
			line = fmt.Sprintf("  %s %s", keyStyle.Render("["+opt.key+"]"), opt.label)
		}
		b.WriteString(line)
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("      %s\n", descStyle.Render(opt.desc)))
	}

	b.WriteString("\n")
	b.WriteString(renderKeyHint("enter", "confirm"))
	b.WriteString(renderKeyHint("esc", "cancel"))

	return b.String()
}

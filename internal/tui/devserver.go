package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// devServerState tracks the sub-state of the dev server flow.
type devServerState int

const (
	devServerPicking devServerState = iota
	devServerStarting
)

// devServerModel holds state for the dev server dialog.
type devServerModel struct {
	cursor  int
	state   devServerState
	message string
	spinner spinner.Model
}

func newDevServerModel() devServerModel {
	return devServerModel{
		state:   devServerPicking,
		spinner: newSwarmSpinner(),
	}
}

// --- Messages ---

type devServerStartedMsg struct {
	agentName string
	paneID    string
	port      int
	err       error
}

// updateDevServer handles input for the dev server dialog.
func (a *App) updateDevServer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch a.devServer.state {
	case devServerPicking:
		// If no dev_command is configured, any key goes back
		if a.cfg.DevCommand == "" {
			if key.Matches(msg, a.keys.Back) || key.Matches(msg, a.keys.Focus) {
				a.screen = ScreenDashboard
				a.devServer = newDevServerModel()
			}
			return a, nil
		}

		switch {
		case key.Matches(msg, a.keys.Up):
			if a.devServer.cursor > 0 {
				a.devServer.cursor--
			}
		case key.Matches(msg, a.keys.Down):
			if a.devServer.cursor < len(a.agents)-1 {
				a.devServer.cursor++
			}
		case key.Matches(msg, a.keys.Focus): // Enter
			agent := a.agents[a.devServer.cursor]
			if agent.DevPaneID != "" {
				a.statusMsg = fmt.Sprintf("%s already has a dev server on :%d", agent.Name, agent.DevPort)
				a.screen = ScreenDashboard
				a.devServer = newDevServerModel()
				return a, nil
			}
			a.devServer.state = devServerStarting
			a.devServer.message = fmt.Sprintf("Starting dev server for %s...", agent.Name)
			return a, tea.Batch(a.devServer.spinner.Tick, a.startDevServerCmd(agent.Name, agent.PaneID, agent.WorkDir))
		case key.Matches(msg, a.keys.Back):
			a.screen = ScreenDashboard
			a.devServer = newDevServerModel()
		}
	}
	return a, nil
}

// startDevServerCmd returns a tea.Cmd that starts a dev server for an agent.
// The dev server runs in a vertical split within the agent's window (agent on top, server on bottom).
// Captures immutable references before the closure.
func (a *App) startDevServerCmd(agentName string, agentPaneID string, workDir string) tea.Cmd {
	tmuxDriver := a.tmux
	ports := a.ports
	swarmPaneID := a.swarmPaneID
	devCmdTemplate := a.cfg.DevCommand

	return func() tea.Msg {
		// Allocate a port (thread-safe via mutex)
		allocatedPort, err := ports.Allocate(agentName)
		if err != nil {
			return devServerStartedMsg{agentName: agentName, err: err}
		}

		// Create a vertical split below the agent's pane
		paneID, err := tmuxDriver.SplitWindowV(agentPaneID, workDir)
		if err != nil {
			ports.Release(allocatedPort)
			return devServerStartedMsg{agentName: agentName, err: fmt.Errorf("create dev server pane: %w", err)}
		}

		// Label the dev server pane
		devTitle := fmt.Sprintf("%s dev :%d", agentName, allocatedPort)
		_ = tmuxDriver.SetPaneTitle(paneID, devTitle)

		// Build the dev server command from config template
		devCmd := strings.ReplaceAll(devCmdTemplate, "{port}", fmt.Sprintf("%d", allocatedPort))
		if err := tmuxDriver.RunInPane(paneID, devCmd); err != nil {
			ports.Release(allocatedPort)
			_ = tmuxDriver.KillPane(paneID)
			return devServerStartedMsg{agentName: agentName, err: fmt.Errorf("start dev server: %w", err)}
		}

		// Refocus the swarm control pane so the TUI stays active
		_ = tmuxDriver.SelectPane(swarmPaneID)

		return devServerStartedMsg{
			agentName: agentName,
			paneID:    paneID,
			port:      allocatedPort,
		}
	}
}

// handleDevServerStarted processes the result of starting a dev server.
func (a *App) handleDevServerStarted(msg devServerStartedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		a.statusMsg = fmt.Sprintf("Error: %v", msg.err)
	} else {
		// Update the agent with dev server info
		for i, agent := range a.agents {
			if agent.Name == msg.agentName {
				a.agents[i].DevPaneID = msg.paneID
				a.agents[i].DevPort = msg.port
				break
			}
		}
		a.statusMsg = ""
	}

	a.screen = ScreenDashboard
	a.devServer = newDevServerModel()
	return a, nil
}

// viewDevServer renders the dev server dialog.
func (a *App) viewDevServer() string {
	switch a.devServer.state {
	case devServerStarting:
		return a.viewDevServerProgress()
	default:
		return a.viewDevServerPicker()
	}
}

func (a *App) viewDevServerPicker() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Dev Server"))
	b.WriteString("\n\n")

	// If no dev_command is configured, show setup instructions
	if a.cfg.DevCommand == "" {
		b.WriteString(descStyle.Render("No dev_command configured."))
		b.WriteString("\n\n")
		b.WriteString(descStyle.Render("Add to .swarmrc:"))
		b.WriteString("\n\n")
		b.WriteString(keyStyle.Render("  dev_command: npm run dev -- --port {port}"))
		b.WriteString("\n\n")
		b.WriteString(descStyle.Render("Use {port} as a placeholder for the allocated port."))
		b.WriteString("\n\n")
		b.WriteString("  ")
		b.WriteString(hintBar("esc", "back"))
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString(descStyle.Render("Select agent to attach dev server:"))
	b.WriteString("\n\n")

	for i, agent := range a.agents {
		var line string
		status := ""
		if agent.DevPaneID != "" {
			status = descStyle.Render(fmt.Sprintf(" (already running :%d)", agent.DevPort))
		}

		if i == a.devServer.cursor {
			line = selectedStyle.Render(fmt.Sprintf(" %s %s ", agent.Name, agent.AgentType))
			line += status
		} else {
			line = fmt.Sprintf("  %s %s%s", activeAgentStyle.Render(agent.Name), descStyle.Render(agent.AgentType), status)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(hintBar("enter", "start", "esc", "cancel"))
	b.WriteString("\n")

	return b.String()
}

func (a *App) viewDevServerProgress() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Dev Server"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("  %s %s", a.devServer.spinner.View(), descStyle.Render(a.devServer.message)))
	b.WriteString("\n")

	return b.String()
}



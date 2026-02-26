package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// devServerState tracks the sub-state of the dev server flow.
type devServerState int

const (
	devServerIdle devServerState = iota
	devServerStarting
)

// devServerModel holds state for the dev server progress spinner.
type devServerModel struct {
	state   devServerState
	message string
	spinner spinner.Model
}

func newDevServerModel() devServerModel {
	return devServerModel{
		state:   devServerIdle,
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

type devServerStoppedMsg struct {
	agentName string
	err       error
}

// updateDevServer handles input for the dev server progress screen.
// The picker is gone — "d" on the dashboard directly starts/stops the dev server.
func (a *App) updateDevServer(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Only the starting state is shown as a modal now; back key dismisses it
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

// stopDevServerCmd returns a tea.Cmd that kills the dev server pane for an agent.
func (a *App) stopDevServerCmd(agentName string, devPaneID string) tea.Cmd {
	tmuxDriver := a.tmux
	swarmPaneID := a.swarmPaneID
	ports := a.ports

	return func() tea.Msg {
		_ = tmuxDriver.KillPane(devPaneID)
		_ = tmuxDriver.ApplyTiledLayout()
		_ = tmuxDriver.SetPaneWidth(swarmPaneID, swarmPaneWidth)
		_ = tmuxDriver.SelectPane(swarmPaneID)
		ports.ReleaseByAgent(agentName)
		return devServerStoppedMsg{agentName: agentName}
	}
}

// handleDevServerStopped processes the result of stopping a dev server.
func (a *App) handleDevServerStopped(msg devServerStoppedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		a.statusMsg = fmt.Sprintf("Error stopping dev server: %v", msg.err)
		return a, nil
	}
	for i, agent := range a.agents {
		if agent.Name == msg.agentName {
			a.agents[i].DevPaneID = ""
			a.agents[i].DevPort = 0
			break
		}
	}
	a.statusMsg = ""
	return a, nil
}

// viewDevServer renders the dev server progress spinner.
func (a *App) viewDevServer() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Dev Server"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("  %s %s", a.devServer.spinner.View(), descStyle.Render(a.devServer.message)))
	b.WriteString("\n")

	return b.String()
}



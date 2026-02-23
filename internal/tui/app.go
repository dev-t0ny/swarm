package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dev-t0ny/swarm/internal/port"
	"github.com/dev-t0ny/swarm/internal/tmux"
)

// Screen represents which screen is currently active.
type Screen int

const (
	ScreenDashboard Screen = iota
	ScreenNewAgent
	ScreenDevServer
	ScreenCloseAgent
	ScreenCleanup
)

// AgentStatus represents the current state of an agent.
type AgentStatus int

const (
	AgentRunning AgentStatus = iota
	AgentMerging
	AgentIdle
)

func (s AgentStatus) String() string {
	switch s {
	case AgentRunning:
		return "running"
	case AgentMerging:
		return "merging"
	case AgentIdle:
		return "idle"
	default:
		return "unknown"
	}
}

// AgentInstance represents a running agent in a worktree.
type AgentInstance struct {
	Name       string
	AgentType  string // claude, opencode, codex, shell
	Branch     string
	WorkDir    string
	PaneID     string
	DevPaneID  string // pane ID for dev server, if any
	DevPort    int
	Status     AgentStatus
}

// App is the root bubbletea model.
type App struct {
	screen   Screen
	keys     KeyMap
	width    int
	height   int
	agents   []AgentInstance
	cursor   int
	repoRoot string
	repoName string
	tmux     *tmux.Driver
	swarmPaneID  string
	nextAgentNum int

	// Sub-models for dialogs
	newAgent  newAgentModel
	devServer devServerModel

	// Port allocator
	ports *port.Allocator

	// Error/status message
	statusMsg string
}

// NewApp creates the root TUI application.
func NewApp(repoRoot, repoName string, driver *tmux.Driver, swarmPaneID string) *App {
	return &App{
		screen:       ScreenDashboard,
		keys:         DefaultKeyMap(),
		agents:       []AgentInstance{},
		repoRoot:     repoRoot,
		repoName:     repoName,
		tmux:         driver,
		swarmPaneID:  swarmPaneID,
		nextAgentNum: 1,
		newAgent:     newNewAgentModel(),
		devServer:    newDevServerModel(),
		ports:        port.NewAllocator(3000),
	}
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	// Handle async messages from any screen
	case agentCreatedMsg:
		return a.handleAgentCreated(msg)
	case depsInstalledMsg:
		return a.handleDepsInstalled(msg)
	case devServerStartedMsg:
		return a.handleDevServerStarted(msg)

	case tea.KeyMsg:
		// Global keys that work on any screen
		if key.Matches(msg, a.keys.Quit) {
			return a, tea.Quit
		}

		// Back key returns to dashboard from any sub-screen
		// (but only if we're not in the middle of an async operation)
		if key.Matches(msg, a.keys.Back) && a.screen != ScreenDashboard {
			if a.screen == ScreenNewAgent && (a.newAgent.state == newAgentCreating || a.newAgent.state == newAgentInstalling) {
				// Don't allow back during creation
				return a, nil
			}
			a.screen = ScreenDashboard
			a.statusMsg = ""
			a.newAgent = newNewAgentModel()
			return a, nil
		}

		// Screen-specific handling
		switch a.screen {
		case ScreenDashboard:
			return a.updateDashboard(msg)
		case ScreenNewAgent:
			return a.updateNewAgent(msg)
		case ScreenDevServer:
			return a.updateDevServer(msg)
		}
	}

	return a, nil
}

// updateDashboard handles key events on the dashboard screen.
func (a *App) updateDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, a.keys.Up):
		if a.cursor > 0 {
			a.cursor--
		}
	case key.Matches(msg, a.keys.Down):
		if a.cursor < len(a.agents)-1 {
			a.cursor++
		}
	case key.Matches(msg, a.keys.NewAgent):
		a.screen = ScreenNewAgent
		a.newAgent = newNewAgentModel()
	case key.Matches(msg, a.keys.Focus):
		if len(a.agents) > 0 {
			agent := a.agents[a.cursor]
			_ = a.tmux.SelectPane(agent.PaneID)
		}
	case key.Matches(msg, a.keys.Close):
		if len(a.agents) > 0 {
			a.screen = ScreenCloseAgent
		}
	case key.Matches(msg, a.keys.DevServer):
		if len(a.agents) > 0 {
			a.screen = ScreenDevServer
		}
	case key.Matches(msg, a.keys.Cleanup):
		if len(a.agents) > 0 {
			a.screen = ScreenCleanup
		}
	}
	return a, nil
}

// View implements tea.Model.
func (a *App) View() string {
	switch a.screen {
	case ScreenDashboard:
		return a.viewDashboard()
	case ScreenNewAgent:
		return a.viewNewAgent()
	case ScreenCloseAgent:
		return a.viewCloseAgent()
	case ScreenDevServer:
		return a.viewDevServer()
	case ScreenCleanup:
		return a.viewCleanup()
	default:
		return a.viewDashboard()
	}
}

// viewDashboard renders the main dashboard.
func (a *App) viewDashboard() string {
	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("swarm"))
	b.WriteString("\n")

	// Repo info
	b.WriteString(descStyle.Render(a.repoName))
	b.WriteString("\n\n")

	// Agents section
	b.WriteString(sectionStyle.Render("Agents"))
	b.WriteString("\n")

	if len(a.agents) == 0 {
		b.WriteString(descStyle.Render("  No agents running"))
		b.WriteString("\n")
		b.WriteString(descStyle.Render("  Press 'n' to create one"))
		b.WriteString("\n")
	} else {
		for i, agent := range a.agents {
			// Status indicator
			var indicator string
			var nameStyle lipgloss.Style
			switch agent.Status {
			case AgentRunning:
				indicator = activeAgentStyle.Render("●")
				nameStyle = activeAgentStyle
			case AgentMerging:
				indicator = mergingAgentStyle.Render("◐")
				nameStyle = mergingAgentStyle
			case AgentIdle:
				indicator = idleAgentStyle.Render("○")
				nameStyle = idleAgentStyle
			}

			var line string
			if i == a.cursor {
				line = selectedStyle.Render(fmt.Sprintf(" %s %s %s", "●", agent.Name, agent.AgentType))
			} else {
				line = fmt.Sprintf("  %s %s %s", indicator, nameStyle.Render(agent.Name), descStyle.Render(agent.AgentType))
			}

			b.WriteString(line)

			// Show dev server port if running
			if agent.DevPaneID != "" {
				b.WriteString(descStyle.Render(fmt.Sprintf(" :%d", agent.DevPort)))
			}

			b.WriteString("\n")
		}
	}

	b.WriteString("\n")

	// Keybinding hints
	b.WriteString(renderKeyHint("n", "new agent"))
	if len(a.agents) > 0 {
		b.WriteString(renderKeyHint("enter", "focus"))
		b.WriteString(renderKeyHint("d", "dev server"))
		b.WriteString(renderKeyHint("x", "close"))
		b.WriteString(renderKeyHint("C", "cleanup all"))
	}
	b.WriteString(renderKeyHint("q", "quit"))

	// Status bar
	b.WriteString("\n")
	b.WriteString(statusBarStyle.Render(fmt.Sprintf("Agents: %d", len(a.agents))))

	// Status message
	if a.statusMsg != "" {
		b.WriteString("\n")
		b.WriteString(warningStyle.Render(a.statusMsg))
	}

	return b.String()
}

// viewCloseAgent is a placeholder — will be implemented in Phase 7.
func (a *App) viewCloseAgent() string {
	return dialogStyle.Render("Close Agent (coming soon)\n\nPress esc to go back")
}

// viewCleanup is a placeholder — will be implemented in Phase 8.
func (a *App) viewCleanup() string {
	return dialogStyle.Render("Cleanup (coming soon)\n\nPress esc to go back")
}

func renderKeyHint(k, desc string) string {
	return fmt.Sprintf("  %s %s\n", keyStyle.Render("["+k+"]"), descStyle.Render(desc))
}

// Run starts the TUI application.
func Run(repoRoot, repoName string, driver *tmux.Driver, swarmPaneID string) error {
	app := NewApp(repoRoot, repoName, driver, swarmPaneID)
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

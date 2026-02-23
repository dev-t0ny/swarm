package tui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/dev-t0ny/swarm/internal/config"
	"github.com/dev-t0ny/swarm/internal/port"
	"github.com/dev-t0ny/swarm/internal/tmux"
)

// healthCheckMsg carries the set of pane IDs that are still alive.
type healthCheckMsg struct {
	livePanes map[string]bool
}

// paneDeathWatcher runs a goroutine that blocks on tmux wait-for signals.
// When a pane dies, it queries live panes and sends a healthCheckMsg into the
// bubbletea program. Call stop() to shut down the watcher.
type paneDeathWatcher struct {
	driver *tmux.Driver
	prog   *tea.Program
	stopCh chan struct{}
	once   sync.Once
}

func newPaneDeathWatcher(driver *tmux.Driver, prog *tea.Program) *paneDeathWatcher {
	return &paneDeathWatcher{
		driver: driver,
		prog:   prog,
		stopCh: make(chan struct{}),
	}
}

// start installs the tmux hook and begins the watch loop.
func (w *paneDeathWatcher) start() {
	_ = w.driver.SetPaneDeathHook()
	go w.loop()
}

// stop tears down the hook and unblocks the goroutine.
func (w *paneDeathWatcher) stop() {
	w.once.Do(func() {
		close(w.stopCh)
		_ = w.driver.RemovePaneDeathHook()
	})
}

// loop blocks on WaitForPaneDeath repeatedly, sending health check messages.
func (w *paneDeathWatcher) loop() {
	for {
		// Block until a pane dies (or the hook is removed on shutdown).
		err := w.driver.WaitForPaneDeath()

		// Check if we've been told to stop.
		select {
		case <-w.stopCh:
			return
		default:
		}

		if err != nil {
			// Session gone or tmux error — stop watching.
			return
		}

		// A pane died — query which panes are still alive.
		live, _ := w.driver.LivePaneIDs()
		w.prog.Send(healthCheckMsg{livePanes: live})
	}
}

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
	PaneID     string // tmux pane ID for this agent
	DevPaneID  string // pane ID for dev server split, if any
	DevPort    int
	Status     AgentStatus
	installCmd string // cached install command from config
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
	newAgent   newAgentModel
	devServer  devServerModel
	closeAgent closeAgentModel
	cleanup    cleanupModel

	// Port allocator
	ports *port.Allocator

	// Config from .swarmrc
	cfg *config.Config

	// Event-driven pane death watcher (set after program starts)
	watcher *paneDeathWatcher

	// Error/status message
	statusMsg string
}

// NewApp creates the root TUI application.
func NewApp(repoRoot, repoName string, driver *tmux.Driver, swarmPaneID string, cfg *config.Config) *App {
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
		closeAgent:   newCloseAgentModel(),
		cleanup:      newCleanupModel(),
		ports:        port.NewAllocator(cfg.BasePort),
		cfg:          cfg,
	}
}

// Init implements tea.Model.
func (a *App) Init() tea.Cmd {
	// Health checking is done by the paneDeathWatcher goroutine (event-driven),
	// not by tea.Tick polling. The watcher is started in Run() after we have a
	// reference to the tea.Program.
	return nil
}

// Update implements tea.Model.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		return a, nil

	// Health check — detect externally killed panes
	case healthCheckMsg:
		return a.handleHealthCheck(msg)

	// Handle async messages from any screen
	case agentCreatedMsg:
		return a.handleAgentCreated(msg)
	case depsInstalledMsg:
		return a.handleDepsInstalled(msg)
	case devServerStartedMsg:
		return a.handleDevServerStarted(msg)
	case agentClosedMsg:
		return a.handleAgentClosed(msg)
	case cleanupDoneMsg:
		return a.handleCleanupDone(msg)

	case tea.KeyMsg:
		// Global keys that work on any screen
		if key.Matches(msg, a.keys.Quit) {
			// Stop the pane death watcher before killing panes
			if a.watcher != nil {
				a.watcher.stop()
			}
			// Kill all agent panes before exiting
			for _, ag := range a.agents {
				if ag.DevPaneID != "" {
					_ = a.tmux.KillPane(ag.DevPaneID)
				}
				_ = a.tmux.KillPane(ag.PaneID)
			}
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
		case ScreenCloseAgent:
			return a.updateCloseAgent(msg)
		case ScreenCleanup:
			return a.updateCleanup(msg)
		}
	}

	return a, nil
}

// updateDashboard handles key events on the dashboard screen.
func (a *App) updateDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Number keys 1-9 to select an agent in the list
	k := msg.String()
	if len(k) == 1 && k[0] >= '1' && k[0] <= '9' {
		idx := int(k[0] - '1')
		if idx < len(a.agents) {
			a.cursor = idx
		}
		return a, nil
	}

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
	w := a.width
	h := a.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}

	// Empty dashboard handles its own centering + background
	if a.screen == ScreenDashboard && len(a.agents) == 0 {
		return a.viewDashboardEmpty(w, h)
	}

	// Dashboard renders normally (top-left aligned)
	if a.screen == ScreenDashboard {
		return lipgloss.Place(w, h,
			lipgloss.Left, lipgloss.Top,
			a.viewDashboardActive(w),
		)
	}

	// All other screens render as centered modals
	var inner string
	switch a.screen {
	case ScreenNewAgent:
		inner = a.viewNewAgent()
	case ScreenCloseAgent:
		inner = a.viewCloseAgent()
	case ScreenDevServer:
		inner = a.viewDevServer()
	case ScreenCleanup:
		inner = a.viewCleanup()
	default:
		inner = a.viewDashboardActive(w)
	}

	modal := modalStyle.Render(inner)
	return lipgloss.Place(w, h,
		lipgloss.Center, lipgloss.Center,
		modal,
	)
}

// viewDashboardEmpty renders a centered welcome screen.
func (a *App) viewDashboardEmpty(w, h int) string {
	var b strings.Builder

	b.WriteString(logoStyle.Render(logo))
	b.WriteString("\n")
	b.WriteString(subtitleStyle.Render("parallel AI agent workspace"))
	b.WriteString("\n\n")

	b.WriteString(descStyle.Render("No agents running"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("%s %s\n", keyStyle.Render("[n]"), descStyle.Render("new agent")))
	b.WriteString(fmt.Sprintf("%s %s\n", keyStyle.Render("[q]"), descStyle.Render("quit")))

	// Status message
	if a.statusMsg != "" {
		b.WriteString("\n")
		b.WriteString(warningStyle.Render(a.statusMsg))
	}

	// Center everything
	return lipgloss.Place(w, h,
		lipgloss.Center, lipgloss.Center,
		b.String(),
	)
}

// viewDashboardActive renders the dashboard with the agent list.
func (a *App) viewDashboardActive(w int) string {
	var b strings.Builder

	sepW := w - 4
	if sepW > 50 {
		sepW = 50
	}
	if sepW < 20 {
		sepW = 20
	}

	// Header — compact, one line
	projectTag := ""
	if a.cfg.Detected != "" {
		projectTag = dimStyle.Render(fmt.Sprintf("  %s", string(a.cfg.Detected)))
	}
	header := fmt.Sprintf("  %s %s %s%s  %s",
		keyStyle.Render("swarm"),
		separatorStyle.Render("─"),
		descStyle.Render(a.repoName),
		projectTag,
		descStyle.Render(fmt.Sprintf("%d agent(s)", len(a.agents))),
	)
	b.WriteString(header)
	b.WriteString("\n\n")

	// Agent list
	for i, ag := range a.agents {
		// Number badge
		tabNum := fmt.Sprintf("%d", i+1)
		var badge string
		if i == a.cursor {
			badge = badgeStyle.Render(tabNum)
		} else {
			badge = badgeInactiveStyle.Render(tabNum)
		}

		// Status indicator
		var indicator string
		switch ag.Status {
		case AgentRunning:
			indicator = activeAgentStyle.Render("●")
		case AgentMerging:
			indicator = mergingAgentStyle.Render("◐")
		case AgentIdle:
			indicator = idleAgentStyle.Render("○")
		}

		// Agent name + type
		var nameStr string
		if i == a.cursor {
			nameStr = fmt.Sprintf("%s  %s", keyStyle.Render(ag.Name), descStyle.Render(ag.AgentType))
		} else {
			nameStr = fmt.Sprintf("%s  %s", descStyle.Render(ag.Name), dimStyle.Render(ag.AgentType))
		}

		b.WriteString(fmt.Sprintf("  %s %s %s", badge, indicator, nameStr))

		// Branch
		b.WriteString(branchStyle.Render(fmt.Sprintf("  %s", ag.Branch)))

		// Port
		if ag.DevPaneID != "" {
			b.WriteString(portStyle.Render(fmt.Sprintf("  :%d", ag.DevPort)))
		}

		b.WriteString("\n")
	}

	b.WriteString("\n  ")
	b.WriteString(separator(sepW))
	b.WriteString("\n\n")

	// Keybinding hints — compact two-column layout
	b.WriteString(fmt.Sprintf("  %s %-12s %s %s\n",
		keyStyle.Render("[n]"), "new",
		keyStyle.Render("[x]"), "close"))
	b.WriteString(fmt.Sprintf("  %s %-12s %s %s\n",
		keyStyle.Render("[d]"), "dev server",
		keyStyle.Render("[C]"), "cleanup all"))
	b.WriteString(fmt.Sprintf("  %s %-12s %s %s\n",
		keyStyle.Render("[1-9]"), "select",
		keyStyle.Render("[q]"), "quit"))

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  click a pane to interact with it"))

	// Status message
	if a.statusMsg != "" {
		b.WriteString("\n\n")
		b.WriteString(warningStyle.Render("  " + a.statusMsg))
	}

	return b.String()
}

// handleHealthCheck removes agents whose panes no longer exist in tmux.
// Called when the paneDeathWatcher detects a pane was killed externally.
func (a *App) handleHealthCheck(msg healthCheckMsg) (tea.Model, tea.Cmd) {
	if msg.livePanes == nil || len(a.agents) == 0 {
		return a, nil
	}

	var alive []AgentInstance
	changed := false
	for _, ag := range a.agents {
		if msg.livePanes[ag.PaneID] {
			alive = append(alive, ag)
		} else {
			// Pane is gone — release port if dev server was running
			if ag.DevPaneID != "" {
				a.ports.ReleaseByAgent(ag.Name)
			}
			changed = true
		}
	}

	if changed {
		a.agents = alive
		if a.cursor >= len(a.agents) && a.cursor > 0 {
			a.cursor = len(a.agents) - 1
		}
		if len(a.agents) == 0 {
			_ = a.tmux.DisablePaneBorders()
		}
	}

	return a, nil
}

func renderKeyHint(k, desc string) string {
	return fmt.Sprintf("  %s %s\n", keyStyle.Render("["+k+"]"), descStyle.Render(desc))
}

// Run starts the TUI application.
func Run(repoRoot, repoName string, driver *tmux.Driver, swarmPaneID string, cfg *config.Config) error {
	app := NewApp(repoRoot, repoName, driver, swarmPaneID, cfg)
	p := tea.NewProgram(app, tea.WithAltScreen())

	// Start the event-driven pane death watcher.
	// It needs a reference to the tea.Program to send messages, so we create it here.
	watcher := newPaneDeathWatcher(driver, p)
	app.watcher = watcher
	watcher.start()

	_, err := p.Run()

	// Clean up the watcher (removes tmux hook, unblocks goroutine).
	watcher.stop()

	return err
}

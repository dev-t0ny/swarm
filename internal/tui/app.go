package tui

import (
	tea "github.com/charmbracelet/bubbletea"
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

// App is the root bubbletea model.
type App struct {
	screen Screen
	keys   KeyMap
	width  int
	height int
}

// NewApp creates the root TUI application.
func NewApp() *App {
	return &App{
		screen: ScreenDashboard,
		keys:   DefaultKeyMap(),
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
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return a, tea.Quit
		}
	}
	return a, nil
}

// View implements tea.Model.
func (a *App) View() string {
	return titleStyle.Render("swarm") + "\n\n" +
		descStyle.Render("Parallel AI Agent Workspace Manager") + "\n\n" +
		descStyle.Render("Press 'q' to quit")
}

// Run starts the TUI application.
func Run() error {
	app := NewApp()
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

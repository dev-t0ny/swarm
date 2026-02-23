package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines the keybindings for the TUI.
type KeyMap struct {
	NewAgent  key.Binding
	DevServer key.Binding
	Focus     key.Binding
	Close     key.Binding
	Cleanup   key.Binding
	Quit      key.Binding
	Up        key.Binding
	Down      key.Binding
	Back      key.Binding
}

// DefaultKeyMap returns the default keybindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		NewAgent: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "new agent"),
		),
		DevServer: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "dev server"),
		),
		Focus: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "focus"),
		),
		Close: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "close agent"),
		),
		Cleanup: key.NewBinding(
			key.WithKeys("C"),
			key.WithHelp("C", "cleanup all"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("up/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("down/j", "down"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
	}
}

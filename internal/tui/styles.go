package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("#7C3AED")
	secondaryColor = lipgloss.Color("#A78BFA")
	successColor   = lipgloss.Color("#10B981")
	warningColor   = lipgloss.Color("#F59E0B")
	dangerColor    = lipgloss.Color("#EF4444")
	mutedColor     = lipgloss.Color("#6B7280")
	textColor      = lipgloss.Color("#F9FAFB")

	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(secondaryColor)

	activeAgentStyle = lipgloss.NewStyle().
				Foreground(successColor)

	idleAgentStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	mergingAgentStyle = lipgloss.NewStyle().
				Foreground(warningColor)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(textColor).
			Background(primaryColor).
			Padding(0, 1)

	keyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	descStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginTop(1)

	dialogStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2)
)

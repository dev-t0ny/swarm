package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors — deep space theme
	// No background colors on styles — we inherit the terminal's background.
	primaryColor   = lipgloss.Color("#7C3AED") // vibrant purple
	secondaryColor = lipgloss.Color("#A78BFA") // lighter purple
	accentColor    = lipgloss.Color("#C084FC") // pink-purple for highlights
	successColor   = lipgloss.Color("#34D399") // emerald green
	warningColor   = lipgloss.Color("#FBBF24") // warm amber
	dangerColor    = lipgloss.Color("#F87171") // soft red
	mutedColor     = lipgloss.Color("#6B7280") // gray
	dimColor       = lipgloss.Color("#374151") // darker gray
	textColor      = lipgloss.Color("#F9FAFB") // near-white
	subtextColor   = lipgloss.Color("#9CA3AF") // light gray for descriptions
	surfaceColor   = lipgloss.Color("#161b22") // raised surface for badges

	// ASCII art logo — block style
	logo = "" +
		" \u2584\u2584\u2588\u2588\u2588\u2588\u2588\u2584 \u2588\u2588      \u2588\u2588  \u2584\u2588\u2588\u2588\u2588\u2588\u2584   \u2588\u2588\u2584\u2588\u2588\u2588\u2588  \u2588\u2588\u2588\u2588\u2584\u2588\u2588\u2584\n" +
		"  \u2588\u2588\u2584\u2584\u2584\u2584 \u2580 \u2580\u2588  \u2588\u2588  \u2588\u2580  \u2580 \u2584\u2584\u2584\u2588\u2588   \u2588\u2588\u2580      \u2588\u2588 \u2588\u2588 \u2588\u2588\n" +
		"   \u2580\u2580\u2580\u2580\u2588\u2588\u2584  \u2588\u2588\u2584\u2588\u2588\u2584\u2588\u2588  \u2584\u2588\u2588\u2580\u2580\u2580\u2588\u2588   \u2588\u2588       \u2588\u2588 \u2588\u2588 \u2588\u2588\n" +
		"  \u2588\u2584\u2584\u2584\u2584\u2584\u2588\u2588  \u2580\u2588\u2588  \u2588\u2588\u2580  \u2588\u2588\u2584\u2584\u2584\u2588\u2588\u2588   \u2588\u2588       \u2588\u2588 \u2588\u2588 \u2588\u2588\n" +
		"   \u2580\u2580\u2580\u2580\u2580\u2580    \u2580\u2580  \u2580\u2580    \u2580\u2580\u2580\u2580 \u2580\u2580   \u2580\u2580       \u2580\u2580 \u2580\u2580 \u2580\u2580"

	// Styles — foreground only, no backgrounds (terminal provides the bg)
	logoStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			PaddingLeft(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(subtextColor).
			Italic(true).
			PaddingLeft(2)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1).
			PaddingLeft(1)

	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(secondaryColor)

	activeAgentStyle = lipgloss.NewStyle().
				Foreground(successColor).
				Bold(true)

	idleAgentStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	mergingAgentStyle = lipgloss.NewStyle().
				Foreground(warningColor).
				Bold(true)

	selectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(textColor).
			Background(primaryColor).
			Padding(0, 1)

	keyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(accentColor)

	descStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	dimStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor)

	dangerStyle = lipgloss.NewStyle().
			Foreground(dangerColor)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(subtextColor).
			Background(surfaceColor).
			Bold(true).
			Padding(0, 2)

	separatorStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	// Badges intentionally have backgrounds — they're pill-shaped labels
	badgeStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Background(primaryColor).
			Bold(true).
			Padding(0, 1)

	badgeInactiveStyle = lipgloss.NewStyle().
				Foreground(subtextColor).
				Background(surfaceColor).
				Padding(0, 1)

	branchStyle = lipgloss.NewStyle().
			Foreground(accentColor)

	portStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Bold(true)

	// Modal dialog — rounded border, padding
	modalStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 3)

	// Custom spinner — braille starburst, fast and edgy
	swarmSpinner = spinner.Spinner{
		Frames: []string{"⣼", "⣹", "⢻", "⠿", "⡟", "⣏", "⣧", "⣶"},
		FPS:    time.Second / 12,
	}
)

// newSwarmSpinner creates a styled spinner instance.
func newSwarmSpinner() spinner.Model {
	s := spinner.New(
		spinner.WithSpinner(swarmSpinner),
		spinner.WithStyle(lipgloss.NewStyle().Foreground(secondaryColor)),
	)
	return s
}

// separator returns a horizontal line of the given width.
func separator(width int) string {
	if width <= 0 {
		width = 30
	}
	return separatorStyle.Render(strings.Repeat("─", width))
}

package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// PaneInfo represents a tmux pane.
type PaneInfo struct {
	ID     string
	Index  int
	Width  int
	Height int
	Active bool
}

// WindowInfo represents a tmux window (tab).
type WindowInfo struct {
	ID     string
	Index  int
	Name   string
	Active bool
	PaneID string // ID of the first/main pane in this window
}

// Driver provides methods to interact with tmux sessions and panes.
type Driver struct {
	SessionName string
}

// NewDriver creates a new tmux driver for the given session.
func NewDriver(sessionName string) *Driver {
	return &Driver{SessionName: sessionName}
}

// IsInstalled checks if tmux is available on the system.
func IsInstalled() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// IsInsideSession returns true if we're currently running inside a tmux session.
func IsInsideSession() bool {
	return os.Getenv("TMUX") != ""
}

// CurrentSessionName returns the name of the current tmux session, if any.
func CurrentSessionName() (string, error) {
	out, err := exec.Command("tmux", "display-message", "-p", "#{session_name}").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// SessionExists checks if a session with the given name already exists.
func (d *Driver) SessionExists() bool {
	err := exec.Command("tmux", "has-session", "-t", d.SessionName).Run()
	return err == nil
}

// CreateSession creates a new tmux session.
// The session is created detached so we can attach to it after setup.
func (d *Driver) CreateSession(workdir string) error {
	cmd := exec.Command("tmux", "new-session", "-d", "-s", d.SessionName, "-c", workdir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// AttachSession attaches to the tmux session, replacing the current process.
func (d *Driver) AttachSession() error {
	tmuxPath, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux not found: %w", err)
	}
	return execSyscall(tmuxPath, []string{"tmux", "attach-session", "-t", d.SessionName}, os.Environ())
}

// --- Window (Tab) Management ---

// NewWindow creates a new tmux window (tab) with the given name and working directory.
// Returns the pane ID of the new window's pane.
func (d *Driver) NewWindow(name string, workdir string) (string, error) {
	args := []string{"new-window", "-t", d.SessionName, "-n", name, "-P", "-F", "#{pane_id}"}
	if workdir != "" {
		args = append(args, "-c", workdir)
	}
	out, err := exec.Command("tmux", args...).Output()
	if err != nil {
		return "", fmt.Errorf("new-window: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// SelectWindow switches to a window by its index (0-based).
func (d *Driver) SelectWindow(index int) error {
	target := fmt.Sprintf("%s:%d", d.SessionName, index)
	return exec.Command("tmux", "select-window", "-t", target).Run()
}

// SelectWindowByName switches to a window by its name.
func (d *Driver) SelectWindowByName(name string) error {
	target := fmt.Sprintf("%s:%s", d.SessionName, name)
	return exec.Command("tmux", "select-window", "-t", target).Run()
}

// KillWindow closes a tmux window (tab) and all its panes.
func (d *Driver) KillWindow(windowID string) error {
	return exec.Command("tmux", "kill-window", "-t", windowID).Run()
}

// RenameWindow renames a tmux window.
func (d *Driver) RenameWindow(windowTarget string, name string) error {
	return exec.Command("tmux", "rename-window", "-t", windowTarget, name).Run()
}

// ListWindows returns all windows in the session.
func (d *Driver) ListWindows() ([]WindowInfo, error) {
	format := "#{window_id}:#{window_index}:#{window_name}:#{window_active}:#{pane_id}"
	out, err := exec.Command("tmux", "list-windows", "-t", d.SessionName, "-F", format).Output()
	if err != nil {
		return nil, fmt.Errorf("list-windows: %w", err)
	}

	var windows []WindowInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 5)
		if len(parts) != 5 {
			continue
		}
		var idx int
		fmt.Sscanf(parts[1], "%d", &idx)
		windows = append(windows, WindowInfo{
			ID:     parts[0],
			Index:  idx,
			Name:   parts[2],
			Active: parts[3] == "1",
			PaneID: parts[4],
		})
	}
	return windows, nil
}

// --- Pane Management ---

// SplitWindowH creates a horizontal split (new pane to the right) and returns the new pane ID.
func (d *Driver) SplitWindowH(targetPane string, workdir string) (string, error) {
	args := []string{"split-window", "-h", "-t", targetPane, "-P", "-F", "#{pane_id}"}
	if workdir != "" {
		args = append(args, "-c", workdir)
	}
	out, err := exec.Command("tmux", args...).Output()
	if err != nil {
		return "", fmt.Errorf("split-window horizontal: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// ApplyTiledLayout reflows all panes in the current window into an even grid.
func (d *Driver) ApplyTiledLayout() error {
	target := fmt.Sprintf("%s:0", d.SessionName)
	return exec.Command("tmux", "select-layout", "-t", target, "tiled").Run()
}

// SendKeys sends keystrokes to a specific pane.
func (d *Driver) SendKeys(paneID string, keys string) error {
	return exec.Command("tmux", "send-keys", "-t", paneID, keys, "Enter").Run()
}

// SplitWindowV creates a vertical split (new pane below) within a window and returns the new pane ID.
func (d *Driver) SplitWindowV(targetPane string, workdir string) (string, error) {
	args := []string{"split-window", "-v", "-t", targetPane, "-P", "-F", "#{pane_id}"}
	if workdir != "" {
		args = append(args, "-c", workdir)
	}
	out, err := exec.Command("tmux", args...).Output()
	if err != nil {
		return "", fmt.Errorf("split-window vertical: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// SelectPane focuses a specific pane.
func (d *Driver) SelectPane(paneID string) error {
	return exec.Command("tmux", "select-pane", "-t", paneID).Run()
}

// KillPane closes a specific pane.
func (d *Driver) KillPane(paneID string) error {
	return exec.Command("tmux", "kill-pane", "-t", paneID).Run()
}

// KillSession kills the entire session.
func (d *Driver) KillSession() error {
	return exec.Command("tmux", "kill-session", "-t", d.SessionName).Run()
}

// ListPanes returns all panes in the session.
func (d *Driver) ListPanes() ([]PaneInfo, error) {
	format := "#{pane_id}:#{pane_index}:#{pane_width}:#{pane_height}:#{pane_active}"
	out, err := exec.Command("tmux", "list-panes", "-t", d.SessionName, "-F", format).Output()
	if err != nil {
		return nil, fmt.Errorf("list-panes: %w", err)
	}

	var panes []PaneInfo
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 5)
		if len(parts) != 5 {
			continue
		}
		var idx, w, h int
		fmt.Sscanf(parts[1], "%d", &idx)
		fmt.Sscanf(parts[2], "%d", &w)
		fmt.Sscanf(parts[3], "%d", &h)
		panes = append(panes, PaneInfo{
			ID:     parts[0],
			Index:  idx,
			Width:  w,
			Height: h,
			Active: parts[4] == "1",
		})
	}
	return panes, nil
}

// GetCurrentPaneID returns the ID of the currently active pane.
func GetCurrentPaneID() (string, error) {
	out, err := exec.Command("tmux", "display-message", "-p", "#{pane_id}").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// RunInPane sends a command to a pane for execution.
func (d *Driver) RunInPane(paneID string, command string) error {
	return exec.Command("tmux", "send-keys", "-t", paneID, command, "Enter").Run()
}

// SetPaneTitle sets the title of a tmux pane (visible in pane border).
func (d *Driver) SetPaneTitle(paneID string, title string) error {
	return exec.Command("tmux", "select-pane", "-t", paneID, "-T", title).Run()
}

// --- Session Styling ---

// ConfigureSession sets up tmux styling: status bar, pane borders, and layout behavior.
func (d *Driver) ConfigureSession() error {
	opts := [][]string{
		// Hide the status bar entirely — the TUI shows all info
		{"status", "off"},

		// Pane borders — off by default (only one pane at start)
		// Turned on when agents are spawned via EnablePaneBorders()
		{"pane-border-status", "off"},
		{"pane-border-style", "fg=#2a2e3f"},
		{"pane-active-border-style", "fg=#7c3aed"},
		{"pane-border-lines", "single"},

		// Mouse support — click to focus panes
		{"mouse", "on"},

		// Don't auto-rename windows
		{"allow-rename", "off"},
		{"automatic-rename", "off"},
	}

	for _, opt := range opts {
		if err := exec.Command("tmux", "set-option", "-t", d.SessionName, opt[0], opt[1]).Run(); err != nil {
			return fmt.Errorf("set-option %s: %w", opt[0], err)
		}
	}

	return nil
}

// EnablePaneBorders turns on pane border titles (call when agents are spawned).
func (d *Driver) EnablePaneBorders() error {
	if err := exec.Command("tmux", "set-option", "-t", d.SessionName, "pane-border-status", "top").Run(); err != nil {
		return err
	}
	return exec.Command("tmux", "set-option", "-t", d.SessionName,
		"pane-border-format", "#{?pane_title, #[fg=#a78bfa,bold]#{pane_title}#[default] ,}").Run()
}

// DisablePaneBorders turns off pane border titles (call when all agents are removed).
func (d *Driver) DisablePaneBorders() error {
	return exec.Command("tmux", "set-option", "-t", d.SessionName, "pane-border-status", "off").Run()
}

// PrintBanner prints a message in a pane using printf, escaping single quotes for safety.
func (d *Driver) PrintBanner(paneID string, lines []string) error {
	for _, line := range lines {
		escaped := strings.ReplaceAll(line, "'", "'\\''")
		cmd := fmt.Sprintf("printf '%%s\\n' '%s'", escaped)
		if err := d.RunInPane(paneID, cmd); err != nil {
			return err
		}
	}
	return d.RunInPane(paneID, "echo ''")
}

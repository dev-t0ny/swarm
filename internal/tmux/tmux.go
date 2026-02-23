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

// CreateSession creates a new tmux session and runs the given command in it.
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

// SendKeys sends keystrokes to a specific pane.
func (d *Driver) SendKeys(paneID string, keys string) error {
	return exec.Command("tmux", "send-keys", "-t", paneID, keys, "Enter").Run()
}

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

// SplitWindowV creates a vertical split (new pane below) and returns the new pane ID.
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

// ResizePane resizes a pane to the given width (in columns).
func (d *Driver) ResizePane(paneID string, width int) error {
	return exec.Command("tmux", "resize-pane", "-t", paneID, "-x", fmt.Sprintf("%d", width)).Run()
}

// ResizePaneY resizes a pane to the given height (in rows).
func (d *Driver) ResizePaneY(paneID string, height int) error {
	return exec.Command("tmux", "resize-pane", "-t", paneID, "-y", fmt.Sprintf("%d", height)).Run()
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

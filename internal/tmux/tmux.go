package tmux

// Driver provides methods to interact with tmux sessions and panes.
type Driver struct {
	SessionName string
}

// NewDriver creates a new tmux driver for the given session.
func NewDriver(sessionName string) *Driver {
	return &Driver{SessionName: sessionName}
}

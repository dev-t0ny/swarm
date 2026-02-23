//go:build windows

package tmux

import (
	"fmt"
)

// execSyscall is not supported on Windows.
func execSyscall(path string, args []string, env []string) error {
	return fmt.Errorf("tmux is not supported on Windows")
}

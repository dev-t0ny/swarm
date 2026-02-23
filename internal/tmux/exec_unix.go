//go:build !windows

package tmux

import "syscall"

// execSyscall replaces the current process with the given command.
func execSyscall(path string, args []string, env []string) error {
	return syscall.Exec(path, args, env)
}

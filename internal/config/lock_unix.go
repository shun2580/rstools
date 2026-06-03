//go:build !windows

package config

import (
	"os"
	"syscall"
)

// processExists checks whether a process with the given PID is alive
// by sending signal 0 (no-op probe).
func processExists(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

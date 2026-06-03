//go:build windows

package config

import "golang.org/x/sys/windows"

// processExists checks whether a process with the given PID is alive
// using the Windows OpenProcess API.
func processExists(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	_ = windows.CloseHandle(h)
	return true
}

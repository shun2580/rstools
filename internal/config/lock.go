package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// AcquireLock creates a lock file containing the current PID.
// If a lock file exists, it checks for stale locks (dead process) and removes them.
func AcquireLock(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		if !os.IsExist(err) {
			return fmt.Errorf("ロックファイルの作成に失敗: %w", err)
		}
		// Lock file exists — check if it's stale.
		pid, pidErr := readLockPID(path)
		if pidErr != nil || !processExists(pid) {
			// Stale lock: remove and retry.
			_ = os.Remove(path)
			f, err = os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				return fmt.Errorf("ロックファイルの作成に失敗: %w", err)
			}
		} else {
			return fmt.Errorf("別の rscli プロセス (PID %d) が実行中です。--force-unlock で解除できます", pid)
		}
	}
	_, _ = fmt.Fprintf(f, "%d", os.Getpid())
	return f.Close()
}

// ReleaseLock removes the lock file.
func ReleaseLock(path string) {
	_ = os.Remove(path)
}

// ForceUnlock removes the lock file regardless of owner.
func ForceUnlock(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("ロック解除に失敗: %w", err)
	}
	return nil
}

func readLockPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

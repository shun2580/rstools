package config

import (
	"os"
	"path/filepath"
)

// Dir returns the config directory path, creating it if needed.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "remotestorage-cli")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

// LockFile returns the path to the lock file.
func LockFile() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lock"), nil
}

// SyncStateFile returns the path to sync_state.json.
func SyncStateFile() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "sync_state.json"), nil
}

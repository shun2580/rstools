package sync

import (
	"encoding/json"
	"fmt"
	"os"
)

// FileState records the last-known ETag and mtime for a synced file.
type FileState struct {
	ETag  string `json:"etag"`
	Mtime int64  `json:"mtime"` // Unix timestamp (seconds)
	Size  int64  `json:"size"`
}

// SyncState is the full contents of sync_state.json.
type SyncState struct {
	Files map[string]FileState `json:"files"`
}

// LoadState reads sync_state.json. Returns an empty state if the file does not exist.
func LoadState(path string) (*SyncState, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &SyncState{Files: make(map[string]FileState)}, nil
	}
	if err != nil {
		return nil, err
	}
	var s SyncState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("sync_state.json のパースに失敗しました。--reset-state で初期化できます: %w", err)
	}
	if s.Files == nil {
		s.Files = make(map[string]FileState)
	}
	return &s, nil
}

// Save writes the state atomically using a temp file + os.Rename to prevent corruption.
func (s *SyncState) Save(path string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

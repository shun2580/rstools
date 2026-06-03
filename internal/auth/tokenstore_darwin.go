//go:build darwin

package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/shun2580/rstools/internal/config"
)

// TODO: Use macOS Keychain for token protection.
// Current implementation uses file-based storage as a placeholder.
type fileTokenStore struct {
	path string
}

func newPlatformTokenStore() (TokenStore, error) {
	dir, err := config.Dir()
	if err != nil {
		return nil, err
	}
	return &fileTokenStore{path: filepath.Join(dir, "token.json")}, nil
}

func (s *fileTokenStore) Save(token *Token) error {
	data, err := json.MarshalIndent(token, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(s.path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := os.Chmod(s.path, 0600); err != nil {
		return fmt.Errorf("failed to set token file permissions: %w", err)
	}
	_, err = f.Write(data)
	return err
}

func (s *fileTokenStore) Load() (*Token, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var token Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, err
	}
	return &token, nil
}

func (s *fileTokenStore) Delete() error {
	err := os.Remove(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

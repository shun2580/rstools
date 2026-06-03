package sync

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// IgnoreList matches file paths against .rsignore patterns.
type IgnoreList struct {
	patterns []string
}

// LoadIgnore reads the .rsignore file in dir and merges extra patterns from --exclude flags.
func LoadIgnore(dir string, extra []string) (*IgnoreList, error) {
	il := &IgnoreList{patterns: append([]string(nil), extra...)}

	f, err := os.Open(filepath.Join(dir, ".rsignore"))
	if os.IsNotExist(err) {
		return il, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		il.patterns = append(il.patterns, line)
	}
	return il, scanner.Err()
}

// Match reports whether path matches any ignore pattern.
// Patterns are matched against the base name and the full path.
func (il *IgnoreList) Match(path string) bool {
	name := filepath.Base(path)
	for _, pattern := range il.patterns {
		if ok, _ := filepath.Match(pattern, name); ok {
			return true
		}
		if ok, _ := filepath.Match(pattern, path); ok {
			return true
		}
	}
	return false
}

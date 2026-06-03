package remotestorage

import (
	"net/url"
	"strings"
)

// EncodePath encodes a remoteStorage path for use in URLs,
// preserving slashes as path separators.
func EncodePath(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

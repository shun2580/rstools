package transfer

import (
	"mime"
	"path/filepath"
)

// DetectContentType returns the MIME type for a file path based on its extension.
// Falls back to application/octet-stream for unknown extensions.
func DetectContentType(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return "application/octet-stream"
	}
	t := mime.TypeByExtension(ext)
	if t == "" {
		return "application/octet-stream"
	}
	return t
}

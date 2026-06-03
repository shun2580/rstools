package remotestorage

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Entry represents a single item in a remoteStorage directory listing.
type Entry struct {
	Name        string
	IsDir       bool
	Size        int64
	ContentType string
	ETag        string
	LastModified time.Time
}

// draft-22 directory listing JSON structure.
type dirListing struct {
	Items map[string]dirItem `json:"items"`
}

type dirItem struct {
	ETag          string `json:"ETag"`
	ContentType   string `json:"Content-Type"`
	ContentLength int64  `json:"Content-Length"`
	LastModified  int64  `json:"Last-Modified"` // milliseconds since epoch
}

// ListDir fetches the directory listing for the given path.
// The path must end with "/".
func (c *Client) ListDir(path string) ([]Entry, error) {
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	req, err := http.NewRequest(http.MethodGet, c.url(path), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/ld+json;profile=\"http://www.w3.org/ns/json-ld#expanded\"")
	resp, err := c.do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("directory not found: %s", path)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ls %s returned HTTP %d", path, resp.StatusCode)
	}

	var listing dirListing
	if err := json.NewDecoder(resp.Body).Decode(&listing); err != nil {
		return nil, fmt.Errorf("directory listing parse error: %w", err)
	}

	entries := make([]Entry, 0, len(listing.Items))
	for name, item := range listing.Items {
		e := Entry{
			Name:        name,
			IsDir:       strings.HasSuffix(name, "/"),
			Size:        item.ContentLength,
			ContentType: item.ContentType,
			ETag:        item.ETag,
		}
		if item.LastModified > 0 {
			e.LastModified = time.UnixMilli(item.LastModified)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

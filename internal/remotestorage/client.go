package remotestorage

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client is a remoteStorage HTTP client with HTTPS enforcement.
type Client struct {
	storageURL  string
	accessToken string
	http        *http.Client
}

// NewClient creates a new remoteStorage client.
func NewClient(storageURL, accessToken string, insecure bool) *Client {
	transport := &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: insecure}, //nolint:gosec
		IdleConnTimeout:   60 * time.Second,
	}
	return &Client{
		storageURL:  strings.TrimRight(storageURL, "/"),
		accessToken: accessToken,
		http: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
	}
}

// do executes an authenticated request, enforcing HTTPS.
func (c *Client) do(req *http.Request) (*http.Response, error) {
	if !strings.HasPrefix(req.URL.String(), "https://") {
		return nil, fmt.Errorf("HTTPS required: refusing non-HTTPS URL %s", req.URL)
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("User-Agent", "rscli/0.1")
	return c.http.Do(req)
}

// url builds the full URL for a remoteStorage path.
func (c *Client) url(path string) string {
	return c.storageURL + EncodePath(path)
}

// Get downloads a resource and returns a ReadCloser. Caller must close it.
func (c *Client) Get(path string) (io.ReadCloser, http.Header, error) {
	req, err := http.NewRequest(http.MethodGet, c.url(path), nil)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.do(req)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		resp.Body.Close()
		return nil, nil, fmt.Errorf("not found: %s", path)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, nil, fmt.Errorf("GET %s returned HTTP %d", path, resp.StatusCode)
	}
	return resp.Body, resp.Header, nil
}

// Put uploads a resource via streaming.
func (c *Client) Put(path, contentType string, body io.Reader) error {
	req, err := http.NewRequest(http.MethodPut, c.url(path), body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("PUT %s returned HTTP %d", path, resp.StatusCode)
	}
	return nil
}

// Delete removes a resource.
func (c *Client) Delete(path string) error {
	req, err := http.NewRequest(http.MethodDelete, c.url(path), nil)
	if err != nil {
		return err
	}
	resp, err := c.do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("DELETE %s returned HTTP %d", path, resp.StatusCode)
	}
	return nil
}

package auth

import (
	"crypto/tls"
	"net/http"
	"time"
)

// httpClient returns an HTTP client for auth requests (WebFinger, token exchange).
func httpClient(insecure bool) *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure}, //nolint:gosec
	}
	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
}

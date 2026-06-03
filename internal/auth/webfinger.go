package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

const remoteStorageRel = "http://tools.ietf.org/id/draft-dejong-remotestorage"

// Endpoints holds remoteStorage server endpoints discovered via WebFinger.
type Endpoints struct {
	StorageURL    string
	AuthEndpoint  string
	TokenEndpoint string
	RegEndpoint   string // RFC 7591 dynamic registration endpoint (may be empty)
}

type jrd struct {
	Links []jrdLink `json:"links"`
}

type jrdLink struct {
	Rel        string            `json:"rel"`
	Href       string            `json:"href"`
	Properties map[string]string `json:"properties"`
}

// Discover performs WebFinger discovery for the given user@host address.
func Discover(userHost string, insecure bool) (*Endpoints, error) {
	user, host, err := parseUserHost(userHost)
	if err != nil {
		return nil, err
	}

	client := httpClient(insecure)
	wfURL := fmt.Sprintf("https://%s/.well-known/webfinger?resource=%s&rel=%s",
		host,
		url.QueryEscape("acct:"+user+"@"+host),
		url.QueryEscape(remoteStorageRel),
	)

	resp, err := client.Get(wfURL)
	if err != nil {
		return nil, fmt.Errorf("WebFinger request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("WebFinger returned HTTP %d", resp.StatusCode)
	}

	var j jrd
	if err := json.NewDecoder(resp.Body).Decode(&j); err != nil {
		return nil, fmt.Errorf("WebFinger response parse error: %w", err)
	}

	for _, link := range j.Links {
		if link.Rel != remoteStorageRel {
			continue
		}
		ep := &Endpoints{
			StorageURL:    link.Href,
			AuthEndpoint:  link.Properties["http://tools.ietf.org/html/rfc6749#section-4.2"],
			TokenEndpoint: link.Properties["http://tools.ietf.org/html/rfc6749#section-4.3"],
			RegEndpoint:   link.Properties["http://tools.ietf.org/html/rfc7591"],
		}
		if ep.StorageURL == "" {
			return nil, fmt.Errorf("WebFinger response missing storage URL")
		}
		return ep, nil
	}
	return nil, fmt.Errorf("WebFinger response contains no remoteStorage link")
}

func parseUserHost(userHost string) (user, host string, err error) {
	for i, c := range userHost {
		if c == '@' {
			return userHost[:i], userHost[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("invalid user@host format: %q", userHost)
}

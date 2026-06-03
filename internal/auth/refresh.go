package auth

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// Refresh attempts to renew the access token using the refresh token.
// Returns the updated token, or an error if refresh is not possible.
func Refresh(token *Token, insecure bool) (*Token, error) {
	if token.RefreshToken == "" {
		return nil, fmt.Errorf("no refresh token available")
	}

	vals := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {token.RefreshToken},
		"client_id":     {token.ClientID},
	}

	client := httpClient(insecure)
	resp, err := client.PostForm(token.TokenEndpoint, vals)
	if err != nil {
		return nil, fmt.Errorf("token refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("token refresh returned HTTP %d", resp.StatusCode)
	}

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, fmt.Errorf("token refresh response parse error: %w", err)
	}
	if tr.AccessToken == "" {
		return nil, fmt.Errorf("token refresh returned empty access_token")
	}

	updated := *token
	updated.AccessToken = tr.AccessToken
	if tr.RefreshToken != "" {
		updated.RefreshToken = tr.RefreshToken
	}
	if tr.ExpiresIn > 0 {
		updated.ExpiresAt = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	}
	return &updated, nil
}

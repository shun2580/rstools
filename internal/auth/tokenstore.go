package auth

import "time"

// Token holds OAuth2 tokens and remoteStorage endpoint metadata.
type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Scope        string    `json:"scope"`
	StorageURL   string    `json:"storage_url"`
	AuthEndpoint string    `json:"auth_endpoint"`
	TokenEndpoint string   `json:"token_endpoint"`
	ClientID     string    `json:"client_id"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
}

// IsExpired reports whether the access token has expired.
func (t *Token) IsExpired() bool {
	if t.ExpiresAt.IsZero() {
		return false
	}
	return time.Now().After(t.ExpiresAt.Add(-30 * time.Second))
}

// TokenStore persists and retrieves OAuth2 tokens.
type TokenStore interface {
	Save(token *Token) error
	Load() (*Token, error)
	Delete() error
}

// NewTokenStore returns the platform-appropriate TokenStore implementation.
func NewTokenStore() (TokenStore, error) {
	return newPlatformTokenStore()
}

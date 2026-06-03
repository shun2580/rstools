package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// PKCEChallenge holds a PKCE verifier/challenge pair (RFC 7636, S256).
type PKCEChallenge struct {
	Verifier  string
	Challenge string
	Method    string
}

// NewPKCE generates a cryptographically random PKCE verifier and S256 challenge.
func NewPKCE() (*PKCEChallenge, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	verifier := base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(h[:])
	return &PKCEChallenge{
		Verifier:  verifier,
		Challenge: challenge,
		Method:    "S256",
	}, nil
}

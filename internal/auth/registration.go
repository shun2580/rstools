package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type regRequest struct {
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

type regResponse struct {
	ClientID string `json:"client_id"`
}

// RegisterClient attempts RFC 7591 dynamic client registration.
// Returns the client_id on success, or an error if the server does not support it.
func RegisterClient(regEndpoint, redirectURI string, insecure bool) (string, error) {
	if regEndpoint == "" {
		return "", fmt.Errorf("server does not support dynamic client registration")
	}

	body, _ := json.Marshal(regRequest{
		ClientName:              "remotestorage-cli",
		RedirectURIs:            []string{redirectURI},
		TokenEndpointAuthMethod: "none",
	})

	client := httpClient(insecure)
	resp, err := client.Post(regEndpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("client registration request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("client registration returned HTTP %d", resp.StatusCode)
	}

	var reg regResponse
	if err := json.NewDecoder(resp.Body).Decode(&reg); err != nil {
		return "", fmt.Errorf("client registration response parse error: %w", err)
	}
	if reg.ClientID == "" {
		return "", fmt.Errorf("client registration returned empty client_id")
	}
	return reg.ClientID, nil
}

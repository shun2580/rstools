package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Flow performs the OAuth2 Authorization Code + PKCE flow.
// It starts a local HTTP server, opens the browser, and returns the obtained token.
func Flow(ep *Endpoints, clientID, scope string, insecure bool) (*Token, error) {
	pkce, err := NewPKCE()
	if err != nil {
		return nil, err
	}

	state, err := randomState()
	if err != nil {
		return nil, err
	}

	// Listen on a random port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start local server: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	// Build authorization URL.
	authURL := ep.AuthEndpoint + "?" + url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {scope},
		"state":                 {state},
		"code_challenge":        {pkce.Challenge},
		"code_challenge_method": {pkce.Method},
	}.Encode()

	fmt.Printf("ブラウザで認証してください:\n%s\n\n", authURL)
	_ = openBrowser(authURL)

	// Wait for the callback.
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux}

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != state {
			errCh <- fmt.Errorf("state mismatch: possible CSRF attack")
			http.Error(w, "state mismatch", http.StatusBadRequest)
			return
		}
		if e := q.Get("error"); e != "" {
			errCh <- fmt.Errorf("authorization error: %s", e)
			fmt.Fprintf(w, "<html><body>認証に失敗しました: %s<br>このウィンドウを閉じてください。</body></html>", e)
			return
		}
		code := q.Get("code")
		if code == "" {
			errCh <- fmt.Errorf("authorization code not found in callback")
			http.Error(w, "code missing", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, "<html><body>認証が完了しました。このウィンドウを閉じてください。</body></html>")
		codeCh <- code
	})

	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	var code string
	select {
	case code = <-codeCh:
	case err = <-errCh:
		return nil, err
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("timed out waiting for browser authentication")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)

	// Exchange authorization code for tokens.
	return exchangeCode(ep, clientID, code, redirectURI, pkce.Verifier, insecure)
}

func exchangeCode(ep *Endpoints, clientID, code, redirectURI, verifier string, insecure bool) (*Token, error) {
	vals := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {clientID},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"code_verifier": {verifier},
	}

	client := httpClient(insecure)
	resp, err := client.PostForm(ep.TokenEndpoint, vals)
	if err != nil {
		return nil, fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange returned HTTP %d", resp.StatusCode)
	}

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, fmt.Errorf("token response parse error: %w", err)
	}
	if tr.AccessToken == "" {
		return nil, fmt.Errorf("token response missing access_token")
	}

	token := &Token{
		AccessToken:   tr.AccessToken,
		RefreshToken:  tr.RefreshToken,
		Scope:         tr.Scope,
		StorageURL:    ep.StorageURL,
		AuthEndpoint:  ep.AuthEndpoint,
		TokenEndpoint: ep.TokenEndpoint,
		ClientID:      clientID,
	}
	if tr.ExpiresIn > 0 {
		token.ExpiresAt = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	}
	return token, nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	ExpiresIn    int    `json:"expires_in"`
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func openBrowser(u string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "linux":
		cmd = "xdg-open"
		args = []string{u}
	case "darwin":
		cmd = "open"
		args = []string{u}
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", strings.ReplaceAll(u, "&", "^&")}
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	return exec.Command(cmd, args...).Start()
}

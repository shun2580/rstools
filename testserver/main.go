// testserver is a minimal remoteStorage server for local integration testing.
// It serves HTTPS directly using a self-signed certificate (no reverse proxy needed).
//
// Usage:
//
//	./testserver          # listens on https://localhost:8443
//	EXTERNAL_HOST=localhost:8443 ./testserver
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// ── In-memory storage ─────────────────────────────────────────────────────────

type fileEntry struct {
	Content     []byte
	ContentType string
	ETag        string
	ModTime     time.Time
}

var (
	files   = map[string]fileEntry{}
	filesMu sync.RWMutex

	pendingCodes = map[string]pendingAuth{}
	codesMu      sync.Mutex

	validTokens = map[string]struct{}{}
	tokensMu    sync.Mutex
)

type pendingAuth struct {
	redirectURI string
	state       string
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	host := os.Getenv("EXTERNAL_HOST")
	if host == "" {
		host = "localhost:8443"
	}
	addr := ":8443"

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/webfinger", makeWebFingerHandler(host))
	mux.HandleFunc("/oauth/register", handleRegister)
	mux.HandleFunc("/oauth/authorize", handleAuthorize)
	mux.HandleFunc("/oauth/token", handleToken)
	mux.HandleFunc("/storage/", handleStorage)

	cert, err := generateSelfSignedCert()
	if err != nil {
		log.Fatalf("TLS証明書の生成に失敗: %v", err)
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		},
	}

	log.Printf("testserver (HTTPS) listening on %s  →  https://%s", addr, host)
	log.Printf("接続コマンド: rscli connect --insecure testuser@%s", host)
	log.Fatal(srv.ListenAndServeTLS("", ""))
}

// ── TLS: self-signed certificate generation ───────────────────────────────────

func generateSelfSignedCert() (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("秘密鍵の生成に失敗: %w", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("証明書の生成に失敗: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	privDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER})

	return tls.X509KeyPair(certPEM, privPEM)
}

// ── WebFinger ─────────────────────────────────────────────────────────────────

func makeWebFingerHandler(host string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resource := r.URL.Query().Get("resource")
		user := "testuser"
		if strings.HasPrefix(resource, "acct:") {
			parts := strings.SplitN(strings.TrimPrefix(resource, "acct:"), "@", 2)
			if len(parts) == 2 {
				user = parts[0]
			}
		}
		w.Header().Set("Content-Type", "application/jrd+json")
		json.NewEncoder(w).Encode(map[string]any{
			"links": []map[string]any{{
				"rel":  "http://tools.ietf.org/id/draft-dejong-remotestorage",
				"href": fmt.Sprintf("https://%s/storage/%s", host, user),
				"properties": map[string]string{
					"http://tools.ietf.org/html/rfc6749#section-4.2": fmt.Sprintf("https://%s/oauth/authorize", host),
					"http://tools.ietf.org/html/rfc6749#section-4.3": fmt.Sprintf("https://%s/oauth/token", host),
					"http://tools.ietf.org/html/rfc7591":              fmt.Sprintf("https://%s/oauth/register", host),
				},
			}},
		})
	}
}

// ── OAuth2: Dynamic Client Registration ──────────────────────────────────────

func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"client_id": "rscli-test"})
}

// ── OAuth2: Authorize (auto-approve — no browser interaction needed) ──────────

func handleAuthorize(w http.ResponseWriter, r *http.Request) {
	redirectURI := r.URL.Query().Get("redirect_uri")
	state := r.URL.Query().Get("state")

	code := randomStr(16)
	codesMu.Lock()
	pendingCodes[code] = pendingAuth{redirectURI: redirectURI, state: state}
	codesMu.Unlock()

	http.Redirect(w, r,
		redirectURI+"?"+url.Values{"code": {code}, "state": {state}}.Encode(),
		http.StatusFound)
}

// ── OAuth2: Token exchange ────────────────────────────────────────────────────

func handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	r.ParseForm()

	if gt := r.FormValue("grant_type"); gt == "authorization_code" {
		code := r.FormValue("code")
		codesMu.Lock()
		_, ok := pendingCodes[code]
		delete(pendingCodes, code)
		codesMu.Unlock()
		if !ok {
			http.Error(w, "invalid_grant", http.StatusBadRequest)
			return
		}
	}

	token := randomStr(32)
	tokensMu.Lock()
	validTokens[token] = struct{}{}
	tokensMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"access_token":  token,
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": randomStr(32),
		"scope":         "*:rw",
	})
}

// ── remoteStorage protocol ────────────────────────────────────────────────────

func handleStorage(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	tokensMu.Lock()
	_, valid := validTokens[token]
	tokensMu.Unlock()
	if !valid {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	path := r.URL.Path
	switch r.Method {
	case http.MethodGet:
		if strings.HasSuffix(path, "/") {
			serveDir(w, path)
		} else {
			serveFile(w, path)
		}
	case http.MethodPut:
		putFile(w, r, path)
	case http.MethodDelete:
		deleteFile(w, path)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func serveDir(w http.ResponseWriter, dirPath string) {
	filesMu.RLock()
	defer filesMu.RUnlock()

	items := map[string]any{}
	for p, e := range files {
		if !strings.HasPrefix(p, dirPath) {
			continue
		}
		rel := strings.TrimPrefix(p, dirPath)
		if rel == "" || strings.Contains(rel, "/") {
			continue
		}
		items[rel] = map[string]any{
			"ETag":           e.ETag,
			"Content-Type":   e.ContentType,
			"Content-Length": len(e.Content),
			"Last-Modified":  e.ModTime.UnixMilli(),
		}
	}
	w.Header().Set("Content-Type", "application/ld+json")
	json.NewEncoder(w).Encode(map[string]any{"items": items})
}

func serveFile(w http.ResponseWriter, path string) {
	filesMu.RLock()
	e, ok := files[path]
	filesMu.RUnlock()
	if !ok {
		http.NotFound(w, nil)
		return
	}
	w.Header().Set("Content-Type", e.ContentType)
	w.Header().Set("ETag", e.ETag)
	w.Write(e.Content)
}

func putFile(w http.ResponseWriter, r *http.Request, path string) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusInternalServerError)
		return
	}
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		ct = "application/octet-stream"
	}
	etag := `"` + randomStr(8) + `"`
	filesMu.Lock()
	files[path] = fileEntry{Content: body, ContentType: ct, ETag: etag, ModTime: time.Now()}
	filesMu.Unlock()
	w.Header().Set("ETag", etag)
	w.WriteHeader(http.StatusCreated)
}

func deleteFile(w http.ResponseWriter, path string) {
	filesMu.Lock()
	_, ok := files[path]
	delete(files, path)
	filesMu.Unlock()
	if !ok {
		http.NotFound(w, nil)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func randomStr(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:n]
}

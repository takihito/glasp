package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func TestConfigFromAuthFile(t *testing.T) {
	t.Run("oauth2ClientSettings", func(t *testing.T) {
		root := t.TempDir()
		authFile := filepath.Join(root, ".clasprc.json")
		if err := os.WriteFile(authFile, []byte(`{"oauth2ClientSettings":{"clientId":"cid","clientSecret":"csecret"}}`), 0644); err != nil {
			t.Fatalf("failed to write auth file: %v", err)
		}
		cfg, err := ConfigFromAuthFile(authFile)
		if err != nil {
			t.Fatalf("ConfigFromAuthFile failed: %v", err)
		}
		if cfg.ClientID != "cid" || cfg.ClientSecret != "csecret" {
			t.Fatalf("unexpected credentials: %+v", cfg)
		}
	})

	t.Run("installed-json", func(t *testing.T) {
		root := t.TempDir()
		authFile := filepath.Join(root, ".clasprc.json")
		if err := os.WriteFile(authFile, []byte(`{"installed":{"client_id":"iid","client_secret":"isecret"}}`), 0644); err != nil {
			t.Fatalf("failed to write auth file: %v", err)
		}
		cfg, err := ConfigFromAuthFile(authFile)
		if err != nil {
			t.Fatalf("ConfigFromAuthFile failed: %v", err)
		}
		if cfg.ClientID != "iid" || cfg.ClientSecret != "isecret" {
			t.Fatalf("unexpected credentials: %+v", cfg)
		}
	})

	t.Run("directory-path", func(t *testing.T) {
		root := t.TempDir()
		authFile := filepath.Join(root, ".clasprc.json")
		if err := os.WriteFile(authFile, []byte(`{"clientId":"dirid","clientSecret":"dirsecret"}`), 0644); err != nil {
			t.Fatalf("failed to write auth file: %v", err)
		}
		cfg, err := ConfigFromAuthFile(root)
		if err != nil {
			t.Fatalf("ConfigFromAuthFile failed: %v", err)
		}
		if cfg.ClientID != "dirid" || cfg.ClientSecret != "dirsecret" {
			t.Fatalf("unexpected credentials: %+v", cfg)
		}
	})

	t.Run("missing-credentials", func(t *testing.T) {
		root := t.TempDir()
		authFile := filepath.Join(root, ".clasprc.json")
		if err := os.WriteFile(authFile, []byte(`{"token":{"access_token":"x"}}`), 0644); err != nil {
			t.Fatalf("failed to write auth file: %v", err)
		}
		if _, err := ConfigFromAuthFile(authFile); err == nil {
			t.Fatalf("expected missing credentials error")
		}
	})
}

func TestClientFromAuthFileUsesTokenAccessToken(t *testing.T) {
	root := t.TempDir()
	authFile := filepath.Join(root, ".clasprc.json")
	if err := os.WriteFile(authFile, []byte(`{"token":{"access_token":"abc123","token_type":"Bearer"}}`), 0644); err != nil {
		t.Fatalf("failed to write auth file: %v", err)
	}

	var authHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client, err := ClientFromAuthFile(context.Background(), authFile)
	if err != nil {
		t.Fatalf("ClientFromAuthFile failed: %v", err)
	}
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()
	if authHeader != "Bearer abc123" {
		t.Fatalf("expected Authorization header 'Bearer abc123', got %q", authHeader)
	}
}

func TestClientFromAuthFileRequiresAccessToken(t *testing.T) {
	root := t.TempDir()
	authFile := filepath.Join(root, ".clasprc.json")
	if err := os.WriteFile(authFile, []byte(`{"token":{}}`), 0644); err != nil {
		t.Fatalf("failed to write auth file: %v", err)
	}
	if _, err := ClientFromAuthFile(context.Background(), authFile); err == nil {
		t.Fatalf("expected error when token.access_token is missing")
	}
}

func TestClientFromAuthFileRefreshesAccessToken(t *testing.T) {
	root := t.TempDir()

	var tokenRequests int
	var authHeader string
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer apiServer.Close()

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenRequests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new-token","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenServer.Close()

	authFile := filepath.Join(root, ".clasprc.json")
	content := `{
  "oauth2ClientSettings": {"clientId": "cid", "clientSecret": "csecret"},
  "token": {
    "access_token": "expired-token",
    "refresh_token": "refresh-123",
    "token_type": "Bearer",
    "expiry_date": 1
  }
}`
	if err := os.WriteFile(authFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write auth file: %v", err)
	}

	origGoogleEndpoint := google.Endpoint
	t.Cleanup(func() { google.Endpoint = origGoogleEndpoint })
	google.Endpoint = oauth2.Endpoint{
		AuthURL:  tokenServer.URL + "/auth",
		TokenURL: tokenServer.URL,
	}

	client, err := ClientFromAuthFile(context.Background(), authFile)
	if err != nil {
		t.Fatalf("ClientFromAuthFile failed: %v", err)
	}
	resp, err := client.Get(apiServer.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()

	if tokenRequests == 0 {
		t.Fatalf("expected refresh token endpoint to be called")
	}
	if authHeader != "Bearer new-token" {
		t.Fatalf("expected refreshed bearer token, got %q", authHeader)
	}

	data, err := os.ReadFile(authFile)
	if err != nil {
		t.Fatalf("failed to read updated auth file: %v", err)
	}
	var updated struct {
		Token struct {
			AccessToken string `json:"access_token"`
			ExpiryDate  int64  `json:"expiry_date"`
		} `json:"token"`
	}
	if err := json.Unmarshal(data, &updated); err != nil {
		t.Fatalf("failed to parse updated auth file: %v", err)
	}
	if updated.Token.AccessToken != "new-token" {
		t.Fatalf("expected auth file token.access_token to be updated, got %q", updated.Token.AccessToken)
	}
	if updated.Token.ExpiryDate <= 1 {
		t.Fatalf("expected auth file expiry_date to be refreshed, got %d", updated.Token.ExpiryDate)
	}
}

func TestImportAuthFileSuccess(t *testing.T) {
	root := t.TempDir()
	authFile := filepath.Join(root, ".clasprc.json")
	content := `{
  "oauth2ClientSettings": {"clientId": "cid", "clientSecret": "csecret"},
  "token": {
    "access_token": "my-access-token",
    "refresh_token": "my-refresh-token",
    "token_type": "Bearer",
    "expiry_date": 1700000000000
  }
}`
	if err := os.WriteFile(authFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write auth file: %v", err)
	}

	cacheFile := filepath.Join(root, ".glasp", "access.json")
	if err := ImportAuthFile(authFile, cacheFile); err != nil {
		t.Fatalf("ImportAuthFile failed: %v", err)
	}

	token, clientID, clientSecret, err := loadToken(cacheFile)
	if err != nil {
		t.Fatalf("failed to load saved token: %v", err)
	}
	if token.AccessToken != "my-access-token" {
		t.Errorf("expected access_token 'my-access-token', got %q", token.AccessToken)
	}
	if token.RefreshToken != "my-refresh-token" {
		t.Errorf("expected refresh_token 'my-refresh-token', got %q", token.RefreshToken)
	}
	if token.TokenType != "Bearer" {
		t.Errorf("expected token_type 'Bearer', got %q", token.TokenType)
	}
	// Client credentials should NOT be persisted in cache
	if clientID != "" {
		t.Errorf("expected empty clientID in cache, got %q", clientID)
	}
	if clientSecret != "" {
		t.Errorf("expected empty clientSecret in cache, got %q", clientSecret)
	}
}

func TestImportAuthFileAlreadyLoggedIn(t *testing.T) {
	root := t.TempDir()
	authFile := filepath.Join(root, ".clasprc.json")
	if err := os.WriteFile(authFile, []byte(`{"token":{"access_token":"x"}}`), 0644); err != nil {
		t.Fatalf("failed to write auth file: %v", err)
	}

	glaspDir := filepath.Join(root, ".glasp")
	if err := os.MkdirAll(glaspDir, 0700); err != nil {
		t.Fatalf("failed to create .glasp dir: %v", err)
	}
	cacheFile := filepath.Join(glaspDir, "access.json")
	if err := os.WriteFile(cacheFile, []byte(`{"access_token":"existing"}`), 0600); err != nil {
		t.Fatalf("failed to write existing cache: %v", err)
	}

	err := ImportAuthFile(authFile, cacheFile)
	if err == nil {
		t.Fatal("expected error when cache file already exists")
	}
	if !strings.Contains(err.Error(), "already logged in") {
		t.Fatalf("expected 'already logged in' error, got: %v", err)
	}
}

func TestImportAuthFileNoToken(t *testing.T) {
	root := t.TempDir()
	authFile := filepath.Join(root, ".clasprc.json")
	if err := os.WriteFile(authFile, []byte(`{"token":{}}`), 0644); err != nil {
		t.Fatalf("failed to write auth file: %v", err)
	}

	cacheFile := filepath.Join(root, ".glasp", "access.json")
	err := ImportAuthFile(authFile, cacheFile)
	if err == nil {
		t.Fatal("expected error when no tokens in auth file")
	}
	if !strings.Contains(err.Error(), "no access_token or refresh_token") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestImportAuthFileDefaultTokenType(t *testing.T) {
	root := t.TempDir()
	authFile := filepath.Join(root, ".clasprc.json")
	if err := os.WriteFile(authFile, []byte(`{"token":{"access_token":"tok"}}`), 0644); err != nil {
		t.Fatalf("failed to write auth file: %v", err)
	}

	cacheFile := filepath.Join(root, ".glasp", "access.json")
	if err := ImportAuthFile(authFile, cacheFile); err != nil {
		t.Fatalf("ImportAuthFile failed: %v", err)
	}

	token, _, _, err := loadToken(cacheFile)
	if err != nil {
		t.Fatalf("failed to load token: %v", err)
	}
	if token.TokenType != "Bearer" {
		t.Errorf("expected default token_type 'Bearer', got %q", token.TokenType)
	}
}

func TestClientFromAuthJSONUsesAccessToken(t *testing.T) {
	var authHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	const jsonContent = `{"token":{"access_token":"tok123","token_type":"Bearer"}}`
	client, err := ClientFromAuthJSON(context.Background(), jsonContent)
	if err != nil {
		t.Fatalf("ClientFromAuthJSON failed: %v", err)
	}
	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()
	if authHeader != "Bearer tok123" {
		t.Fatalf("expected Authorization 'Bearer tok123', got %q", authHeader)
	}
}

func TestClientFromAuthJSONRequiresAccessToken(t *testing.T) {
	if _, err := ClientFromAuthJSON(context.Background(), `{"token":{}}`); err == nil {
		t.Fatalf("expected error when access_token is missing")
	}
}

func TestClientFromAuthJSONRejectsEmptyContent(t *testing.T) {
	if _, err := ClientFromAuthJSON(context.Background(), "   "); err == nil {
		t.Fatalf("expected error for empty JSON content")
	}
}

func TestClientFromAuthJSONRefreshesToken(t *testing.T) {
	var authHeader string
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer apiServer.Close()

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"refreshed-tok","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenServer.Close()

	origGoogleEndpoint := google.Endpoint
	t.Cleanup(func() { google.Endpoint = origGoogleEndpoint })
	google.Endpoint = oauth2.Endpoint{
		AuthURL:  tokenServer.URL + "/auth",
		TokenURL: tokenServer.URL,
	}

	const jsonContent = `{
  "oauth2ClientSettings": {"clientId": "cid", "clientSecret": "csecret"},
  "token": {
    "access_token": "old-tok",
    "refresh_token": "ref-tok",
    "token_type": "Bearer",
    "expiry_date": 1
  }
}`
	client, err := ClientFromAuthJSON(context.Background(), jsonContent)
	if err != nil {
		t.Fatalf("ClientFromAuthJSON failed: %v", err)
	}
	resp, err := client.Get(apiServer.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()
	if authHeader != "Bearer refreshed-tok" {
		t.Fatalf("expected refreshed bearer token, got %q", authHeader)
	}
}

func TestClientFromAuthJSONRequiresCredentialsForRefresh(t *testing.T) {
	// No client credentials in payload AND no embedded credentials → must error.
	// Clear all credential sources to make the test environment-independent.
	origID := ldflagsClientID
	origSecret := ldflagsClientSecret
	t.Cleanup(func() {
		ldflagsClientID = origID
		ldflagsClientSecret = origSecret
	})
	ldflagsClientID = ""
	ldflagsClientSecret = ""
	t.Setenv("GLASP_CLIENT_ID", "")
	t.Setenv("GLASP_CLIENT_SECRET", "")

	const jsonContent = `{
  "token": {
    "refresh_token": "ref-tok",
    "token_type": "Bearer"
  }
}`
	if _, err := ClientFromAuthJSON(context.Background(), jsonContent); err == nil {
		t.Fatalf("expected error when clientId/clientSecret are missing for refresh")
	}
}

func TestClientFromAuthJSONFallsBackToEmbeddedCredentials(t *testing.T) {
	// Payload has refresh_token but NO client credentials.
	// Embedded credentials (ldflagsClientID/Secret) are set → should succeed.
	// Clear env vars so the test verifies the ldflags path specifically.
	origID := ldflagsClientID
	origSecret := ldflagsClientSecret
	t.Cleanup(func() {
		ldflagsClientID = origID
		ldflagsClientSecret = origSecret
	})
	ldflagsClientID = "embedded-cid"
	ldflagsClientSecret = "embedded-csecret"
	t.Setenv("GLASP_CLIENT_ID", "")
	t.Setenv("GLASP_CLIENT_SECRET", "")

	var authHeader string
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer apiServer.Close()

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"refreshed-tok","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenServer.Close()

	origGoogleEndpoint := google.Endpoint
	t.Cleanup(func() { google.Endpoint = origGoogleEndpoint })
	google.Endpoint = oauth2.Endpoint{
		AuthURL:  tokenServer.URL + "/auth",
		TokenURL: tokenServer.URL,
	}

	// No clientId/clientSecret in payload — must fall back to ldflagsClientID/Secret.
	const jsonContent = `{
  "token": {
    "access_token": "old-tok",
    "refresh_token": "ref-tok",
    "token_type": "Bearer",
    "expiry_date": 1
  }
}`
	client, err := ClientFromAuthJSON(context.Background(), jsonContent)
	if err != nil {
		t.Fatalf("ClientFromAuthJSON should succeed with embedded credentials fallback: %v", err)
	}
	resp, err := client.Get(apiServer.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()
	if authHeader != "Bearer refreshed-tok" {
		t.Fatalf("expected refreshed bearer token, got %q", authHeader)
	}
}

func TestClientFromAuthFileFallsBackToEmbeddedCredentials(t *testing.T) {
	// Same scenario but via ClientFromAuthFile (--auth flag path).
	// Clear env vars so the test verifies the ldflags path specifically.
	origID := ldflagsClientID
	origSecret := ldflagsClientSecret
	t.Cleanup(func() {
		ldflagsClientID = origID
		ldflagsClientSecret = origSecret
	})
	ldflagsClientID = "embedded-cid"
	ldflagsClientSecret = "embedded-csecret"
	t.Setenv("GLASP_CLIENT_ID", "")
	t.Setenv("GLASP_CLIENT_SECRET", "")

	var authHeader string
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer apiServer.Close()

	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"refreshed-tok","token_type":"Bearer","expires_in":3600}`))
	}))
	defer tokenServer.Close()

	origGoogleEndpoint := google.Endpoint
	t.Cleanup(func() { google.Endpoint = origGoogleEndpoint })
	google.Endpoint = oauth2.Endpoint{
		AuthURL:  tokenServer.URL + "/auth",
		TokenURL: tokenServer.URL,
	}

	root := t.TempDir()
	authFile := filepath.Join(root, ".clasprc.json")
	content := `{
  "token": {
    "access_token": "old-tok",
    "refresh_token": "ref-tok",
    "token_type": "Bearer",
    "expiry_date": 1
  }
}`
	if err := os.WriteFile(authFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write auth file: %v", err)
	}

	client, err := ClientFromAuthFile(context.Background(), authFile)
	if err != nil {
		t.Fatalf("ClientFromAuthFile should succeed with embedded credentials fallback: %v", err)
	}
	resp, err := client.Get(apiServer.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()
	if authHeader != "Bearer refreshed-tok" {
		t.Fatalf("expected refreshed bearer token, got %q", authHeader)
	}
}

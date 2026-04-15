package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func TestTokenCacheFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".clasp.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to write .clasp.json: %v", err)
	}
	subdir := filepath.Join(root, "subdir")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current dir: %v", err)
	}
	if err := os.Chdir(subdir); err != nil {
		t.Fatalf("Failed to change dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	actualPath, err := tokenCacheFile()
	if err != nil {
		t.Fatalf("tokenCacheFile returned an error: %v", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("failed to resolve root path: %v", err)
	}
	if !strings.HasSuffix(filepath.Clean(actualPath), filepath.FromSlash(".glasp/access.json")) {
		t.Errorf("Expected token cache file to end with .glasp/access.json, got %s", actualPath)
	}
	if !strings.HasPrefix(filepath.Clean(actualPath), filepath.Clean(resolvedRoot)) {
		t.Errorf("Expected token cache file to be under %s, got %s", resolvedRoot, actualPath)
	}
}

func TestTokenCacheFileRequiresProject(t *testing.T) {
	root := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current dir: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Failed to change dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	_, err = tokenCacheFile()
	if err == nil {
		t.Fatal("expected error when .clasp.json is missing")
	}
	if !strings.Contains(err.Error(), ".clasp.json") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSaveLoadToken(t *testing.T) {
	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "glasp_test_token_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name()) // Clean up the temp file

	testToken := &oauth2.Token{
		AccessToken:  "test_access_token",
		TokenType:    "Bearer",
		RefreshToken: "test_refresh_token",
		Expiry:       time.Now().Add(time.Hour),
	}

	// Test saving the token (no client credentials stored)
	err = saveToken(tmpFile.Name(), testToken)
	if err != nil {
		t.Fatalf("saveToken failed: %v", err)
	}

	// Test loading the token
	loadedToken, clientID, clientSecret, err := loadToken(tmpFile.Name())
	if err != nil {
		t.Fatalf("loadToken failed: %v", err)
	}

	// Compare loaded token with the original
	if loadedToken.AccessToken != testToken.AccessToken {
		t.Errorf("AccessToken mismatch: expected %s, got %s", testToken.AccessToken, loadedToken.AccessToken)
	}
	if loadedToken.TokenType != testToken.TokenType {
		t.Errorf("TokenType mismatch: expected %s, got %s", testToken.TokenType, loadedToken.TokenType)
	}
	if loadedToken.RefreshToken != testToken.RefreshToken {
		t.Errorf("RefreshToken mismatch: expected %s, got %s", testToken.RefreshToken, loadedToken.RefreshToken)
	}
	// Compare expiry with some tolerance (unix millis loses sub-ms precision)
	if !loadedToken.Expiry.Round(time.Second).Equal(testToken.Expiry.Round(time.Second)) {
		t.Errorf("Expiry mismatch: expected %s, got %s", testToken.Expiry.Round(time.Second), loadedToken.Expiry.Round(time.Second))
	}
	if clientID != "" {
		t.Errorf("expected empty clientID, got %s", clientID)
	}
	if clientSecret != "" {
		t.Errorf("expected empty clientSecret, got %s", clientSecret)
	}
}

func TestLoadTokenNotFound(t *testing.T) {
	// Test loading from a non-existent file
	_, _, _, err := loadToken("non_existent_file.json")
	if err == nil {
		t.Error("loadToken did not return an error for a non-existent file")
	}
	// Check if the error indicates file not found using errors.Is
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Expected 'file not found' error using errors.Is, got %v", err)
	}
}

func TestLoadTokenInvalidFormat(t *testing.T) {
	// Create a temporary file with invalid JSON
	tmpFile, err := os.CreateTemp("", "glasp_invalid_token_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString("this is not json")
	if err != nil {
		t.Fatalf("Failed to write invalid content to temp file: %v", err)
	}
	tmpFile.Close()

	// Test loading from an invalid file
	_, _, _, err = loadToken(tmpFile.Name())
	if err == nil {
		t.Error("loadToken did not return an error for an invalid file format")
	}
	// Check if the error indicates JSON decoding issue (more robust check)
	var jsonErr *json.SyntaxError
	if !strings.Contains(err.Error(), "failed to decode token from file") || !errors.As(err, &jsonErr) {
		t.Errorf("Expected JSON decoding error, got %v", err)
	}
}

func TestOAuthStateValidateReuse(t *testing.T) {
	state := &oauthState{token: "token"}
	if err := state.validate("token"); err != nil {
		t.Fatalf("expected first validate to succeed, got %v", err)
	}
	if err := state.validate("token"); err == nil {
		t.Fatalf("expected reuse error, got nil")
	}
}

func TestLoginWithCachePathInvalidState(t *testing.T) {
	cfg := &oauth2.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "http://example/auth",
			TokenURL: "http://example/token",
		},
	}
	cacheFile := filepath.Join(t.TempDir(), "token.json")

	authURLCh := make(chan string, 1)
	origOpenBrowser := openBrowserFn
	t.Cleanup(func() { openBrowserFn = origOpenBrowser })
	openBrowserFn = func(url string) {
		authURLCh <- url
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := loginWithCachePath(context.Background(), cfg, cacheFile)
		errCh <- err
	}()

	authURL := <-authURLCh
	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("failed to parse auth URL: %v", err)
	}
	if parsed.Query().Get("state") == "" {
		t.Fatalf("expected auth URL to contain state")
	}

	redirectURL := cfg.RedirectURL
	resp, err := http.Get(redirectURL + "?state=wrong&code=abc")
	if err != nil {
		t.Fatalf("failed to call redirect URL: %v", err)
	}
	_ = resp.Body.Close()

	if err := <-errCh; err == nil || !strings.Contains(err.Error(), "invalid state") {
		t.Fatalf("expected invalid state error, got %v", err)
	}
}

func TestLoginWithCachePathCancelled(t *testing.T) {
	cfg := &oauth2.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "http://example/auth",
			TokenURL: "http://example/token",
		},
	}
	cacheFile := filepath.Join(t.TempDir(), "token.json")

	origOpenBrowser := openBrowserFn
	t.Cleanup(func() { openBrowserFn = origOpenBrowser })
	openBrowserFn = func(string) {}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := loginWithCachePath(ctx, cfg, cacheFile); err == nil || !strings.Contains(err.Error(), "authentication cancelled") {
		t.Fatalf("expected authentication cancelled error, got %v", err)
	}
}

func TestShutdownServerMessage(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		msg, isErr := shutdownServerMessage(nil)
		if msg != "" || isErr {
			t.Fatalf("expected empty non-error result, got msg=%q isErr=%v", msg, isErr)
		}
	})

	t.Run("deadline-exceeded", func(t *testing.T) {
		msg, isErr := shutdownServerMessage(context.DeadlineExceeded)
		if msg == "" {
			t.Fatalf("expected message for deadline exceeded")
		}
		if isErr {
			t.Fatalf("expected deadline exceeded to be non-error")
		}
		if !strings.Contains(msg, "authentication already completed") {
			t.Fatalf("unexpected message: %q", msg)
		}
	})

	t.Run("other-error", func(t *testing.T) {
		msg, isErr := shutdownServerMessage(errors.New("boom"))
		if msg == "" {
			t.Fatalf("expected message for generic error")
		}
		if !isErr {
			t.Fatalf("expected generic error to be treated as error")
		}
		if !strings.Contains(msg, "Error shutting down server: boom") {
			t.Fatalf("unexpected message: %q", msg)
		}
	})
}

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

func TestOAuthCredentialsFromPayloadUsesSameSourcePair(t *testing.T) {
	payload := &authFilePayload{}
	payload.OAuth2ClientSettings.ClientID = "A"
	payload.Installed.ClientID = "B"
	payload.Installed.ClientSecret = "C"
	clientID, clientSecret := oauthCredentialsFromPayload(payload)
	if clientID != "B" || clientSecret != "C" {
		t.Fatalf("expected installed pair (B,C), got (%s,%s)", clientID, clientSecret)
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

func TestLoadTokenBackwardCompatOldFormat(t *testing.T) {
	root := t.TempDir()
	cacheFile := filepath.Join(root, "access.json")
	// Old flat format (direct oauth2.Token JSON)
	oldFormat := `{"access_token":"old-tok","token_type":"Bearer","refresh_token":"old-refresh","expiry":"2026-03-20T12:00:00Z"}`
	if err := os.WriteFile(cacheFile, []byte(oldFormat), 0600); err != nil {
		t.Fatalf("failed to write old format file: %v", err)
	}

	token, clientID, clientSecret, err := loadToken(cacheFile)
	if err != nil {
		t.Fatalf("loadToken failed for old format: %v", err)
	}
	if token.AccessToken != "old-tok" {
		t.Errorf("expected access_token 'old-tok', got %q", token.AccessToken)
	}
	if token.RefreshToken != "old-refresh" {
		t.Errorf("expected refresh_token 'old-refresh', got %q", token.RefreshToken)
	}
	if clientID != "" || clientSecret != "" {
		t.Errorf("expected empty credentials for old format, got clientID=%q clientSecret=%q", clientID, clientSecret)
	}
}

func TestPersistingTokenSourceConcurrentTokenCalls(t *testing.T) {
	root := t.TempDir()
	authFile := filepath.Join(root, ".clasprc.json")
	if err := os.WriteFile(authFile, []byte(`{"token":{"access_token":"old","token_type":"Bearer"}}`), 0600); err != nil {
		t.Fatalf("failed to write auth file: %v", err)
	}

	expiry := time.Now().Add(1 * time.Hour)
	fresh := &oauth2.Token{
		AccessToken: "new",
		TokenType:   "Bearer",
		Expiry:      expiry,
	}
	ts := &persistingTokenSource{
		base:         oauth2.StaticTokenSource(fresh),
		authPath:     authFile,
		lastSnapshot: tokenSnapshot{},
		hasLast:      false,
	}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := ts.Token(); err != nil {
				t.Errorf("Token() failed: %v", err)
			}
		}()
	}
	wg.Wait()

	data, err := os.ReadFile(authFile)
	if err != nil {
		t.Fatalf("failed to read auth file: %v", err)
	}
	var updated struct {
		Token struct {
			AccessToken string `json:"access_token"`
		} `json:"token"`
	}
	if err := json.Unmarshal(data, &updated); err != nil {
		t.Fatalf("failed to parse auth file: %v", err)
	}
	if updated.Token.AccessToken != "new" {
		t.Fatalf("expected persisted access token 'new', got %q", updated.Token.AccessToken)
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
	origID := ldflagsClientID
	origSecret := ldflagsClientSecret
	t.Cleanup(func() {
		ldflagsClientID = origID
		ldflagsClientSecret = origSecret
	})
	ldflagsClientID = "embedded-cid"
	ldflagsClientSecret = "embedded-csecret"

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
	origID := ldflagsClientID
	origSecret := ldflagsClientSecret
	t.Cleanup(func() {
		ldflagsClientID = origID
		ldflagsClientSecret = origSecret
	})
	ldflagsClientID = "embedded-cid"
	ldflagsClientSecret = "embedded-csecret"

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

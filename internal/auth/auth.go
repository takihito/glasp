package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/takihito/glasp/internal/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"  // Required for some scopes, e.g., drive.file
	"google.golang.org/api/script/v1" // Google Apps Script API
)

// tokenCacheFile is a variable function to allow patching in tests.
var tokenCacheFile = func() (string, error) {
	projectRoot, err := config.FindProjectRoot()
	if err != nil {
		return "", err
	}
	if projectRoot == "" {
		return "", fmt.Errorf("no .clasp.json found; run this command from a project directory")
	}
	return filepath.Join(projectRoot, ".glasp", "access.json"), nil
}

// saveToken saves the token to a file in .clasprc.json-compatible format.
// Only token data is persisted; OAuth client credentials are never cached
// to avoid creating a long-lived secret store in the project directory.
func saveToken(file string, token *oauth2.Token) error {
	fmt.Printf("Saving credential file to: %s\n", file)
	dir := filepath.Dir(file)
	// When the cache file lives under a .glasp directory, use
	// EnsureGlaspDir so that .claspignore is also kept up to date.
	// Otherwise (e.g. tests passing a bare temp path) just MkdirAll.
	if filepath.Base(dir) == ".glasp" {
		projectRoot := filepath.Dir(dir)
		if err := config.EnsureGlaspDir(projectRoot); err != nil {
			return fmt.Errorf("failed to create token cache directory: %w", err)
		}
	} else {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create token cache directory: %w", err)
		}
	}
	payload := authFilePayload{}
	payload.Token.AccessToken = token.AccessToken
	payload.Token.RefreshToken = token.RefreshToken
	payload.Token.TokenType = token.TokenType
	if !token.Expiry.IsZero() {
		payload.Token.ExpiryDate = token.Expiry.UnixMilli()
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token to file: %w", err)
	}
	return writePrivateFileAtomically(file, append(data, '\n'))
}

// saveTokenExclusive atomically creates a new token cache file, failing if
// the file already exists. This avoids the TOCTOU race of stat-then-write.
func saveTokenExclusive(file string, token *oauth2.Token) error {
	dir := filepath.Dir(file)
	if filepath.Base(dir) == ".glasp" {
		projectRoot := filepath.Dir(dir)
		if err := config.EnsureGlaspDir(projectRoot); err != nil {
			return fmt.Errorf("failed to create token cache directory: %w", err)
		}
	} else {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("failed to create token cache directory: %w", err)
		}
	}
	// O_CREATE|O_EXCL guarantees atomic "create if not exists".
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("already logged in. Please run \"glasp logout\" first")
		}
		return fmt.Errorf("failed to create token cache file: %w", err)
	}
	defer f.Close()

	payload := authFilePayload{}
	payload.Token.AccessToken = token.AccessToken
	payload.Token.RefreshToken = token.RefreshToken
	payload.Token.TokenType = token.TokenType
	if !token.Expiry.IsZero() {
		payload.Token.ExpiryDate = token.Expiry.UnixMilli()
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token to file: %w", err)
	}
	fmt.Printf("Saving credential file to: %s\n", file)
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write token cache file: %w", err)
	}
	return nil
}

// loadToken loads the token and OAuth client credentials from a file.
// It supports both the new .clasprc.json-compatible format and the legacy
// flat oauth2.Token format for backward compatibility.
func loadToken(file string) (token *oauth2.Token, clientID, clientSecret string, err error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, "", "", fmt.Errorf("failed to open token cache file: %w", err)
	}
	var payload authFilePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, "", "", fmt.Errorf("failed to decode token from file: %w", err)
	}
	// New format: token is nested under "token" key
	if strings.TrimSpace(payload.Token.AccessToken) != "" || strings.TrimSpace(payload.Token.RefreshToken) != "" {
		token = tokenFromPayload(&payload)
		if token.TokenType == "" {
			token.TokenType = "Bearer"
		}
		cid, csecret := oauthCredentialsFromPayload(&payload)
		return token, cid, csecret, nil
	}
	// Backward compatibility: try legacy flat oauth2.Token format
	var legacyToken oauth2.Token
	if err := json.Unmarshal(data, &legacyToken); err != nil {
		return nil, "", "", fmt.Errorf("failed to decode token from file: %w", err)
	}
	return &legacyToken, "", "", nil
}

// Package-level variables for ClientID and ClientSecret.
// These can be overridden at build time using ldflags:
// go build -ldflags "-X 'glasp/internal/auth.ldflagsClientID=your_client_id' -X 'glasp/internal/auth.ldflagsClientSecret=your_client_secret'"
var (
	ldflagsClientID     string
	ldflagsClientSecret string
)

// Config returns an OAuth2 configuration.
func Config() (*oauth2.Config, error) {
	var clientID, clientSecret string

	if ldflagsClientID != "" {
		clientID = ldflagsClientID
	}
	if envClientID := os.Getenv("GLASP_CLIENT_ID"); envClientID != "" {
		// Fallback to environment variable
		clientID = envClientID
	}

	if ldflagsClientSecret != "" {
		clientSecret = ldflagsClientSecret
	}
	if envClientSecret := os.Getenv("GLASP_CLIENT_SECRET"); envClientSecret != "" {
		// Fallback to environment variable
		clientSecret = envClientSecret
	}

	var missing []string
	if clientID == "" {
		missing = append(missing, "GLASP_CLIENT_ID")
	}
	if clientSecret == "" {
		missing = append(missing, "GLASP_CLIENT_SECRET")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing OAuth credentials: %s", strings.Join(missing, ", "))
	}

	return buildOAuthConfig(clientID, clientSecret), nil
}

// ConfigFromAuthFile returns an OAuth2 configuration loaded from a .clasprc.json file.
func ConfigFromAuthFile(authPath string) (*oauth2.Config, error) {
	payload, cleanPath, err := loadAuthFilePayload(authPath)
	if err != nil {
		return nil, err
	}
	clientID, clientSecret := oauthCredentialsFromPayload(payload)
	var missing []string
	if clientID == "" {
		missing = append(missing, "clientId")
	}
	if clientSecret == "" {
		missing = append(missing, "clientSecret")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("auth file %s is missing OAuth credentials: %s", cleanPath, strings.Join(missing, ", "))
	}
	return buildOAuthConfig(clientID, clientSecret), nil
}

// ClientFromAuthFile builds an authenticated HTTP client from .clasprc.json.
// If refresh_token exists, this function refreshes token.access_token and persists it.
func ClientFromAuthFile(ctx context.Context, authPath string) (*http.Client, error) {
	return clientFromAuthFileWithRefresh(ctx, authPath)
}

type persistingTokenSource struct {
	base         oauth2.TokenSource
	authPath     string
	mu           sync.Mutex
	lastSnapshot tokenSnapshot
	hasLast      bool
}

type tokenSnapshot struct {
	accessToken  string
	refreshToken string
	tokenType    string
	expiryUnixMs int64
}

func (p *persistingTokenSource) Token() (*oauth2.Token, error) {
	token, err := p.base.Token()
	if err != nil {
		return nil, err
	}
	snapshot := tokenSnapshotFromToken(token)

	p.mu.Lock()
	if p.hasLast && snapshot == p.lastSnapshot {
		p.mu.Unlock()
		return token, nil
	}
	p.mu.Unlock()

	if err := persistAuthToken(p.authPath, token); err != nil {
		log.Printf("Warning: failed to persist refreshed token to %s: %v", p.authPath, err)
	} else {
		p.mu.Lock()
		p.lastSnapshot = snapshot
		p.hasLast = true
		p.mu.Unlock()
	}
	return token, nil
}

type authFilePayload struct {
	ClientID             string `json:"clientId"`
	ClientSecret         string `json:"clientSecret"`
	OAuth2ClientSettings struct {
		ClientID     string `json:"clientId"`
		ClientSecret string `json:"clientSecret"`
	} `json:"oauth2ClientSettings"`
	Installed struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	} `json:"installed"`
	Web struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	} `json:"web"`
	Token struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiryDate   int64  `json:"expiry_date"`
	} `json:"token"`
}

func loadAuthFilePayload(authPath string) (*authFilePayload, string, error) {
	trimmed := strings.TrimSpace(authPath)
	if trimmed == "" {
		return nil, "", fmt.Errorf("auth file path is required")
	}
	cleanPath := filepath.Clean(trimmed)
	info, err := os.Stat(cleanPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to stat auth file %s: %w", cleanPath, err)
	}
	if info.IsDir() {
		cleanPath = filepath.Join(cleanPath, ".clasprc.json")
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read auth file %s: %w", cleanPath, err)
	}

	var payload authFilePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, "", fmt.Errorf("failed to parse auth file %s: %w", cleanPath, err)
	}
	return &payload, cleanPath, nil
}

func oauthCredentialsFromPayload(payload *authFilePayload) (string, string) {
	if payload == nil {
		return "", ""
	}
	ordered := []struct {
		clientID     string
		clientSecret string
	}{
		{
			clientID:     strings.TrimSpace(payload.OAuth2ClientSettings.ClientID),
			clientSecret: strings.TrimSpace(payload.OAuth2ClientSettings.ClientSecret),
		},
		{
			clientID:     strings.TrimSpace(payload.Installed.ClientID),
			clientSecret: strings.TrimSpace(payload.Installed.ClientSecret),
		},
		{
			clientID:     strings.TrimSpace(payload.Web.ClientID),
			clientSecret: strings.TrimSpace(payload.Web.ClientSecret),
		},
		{
			clientID:     strings.TrimSpace(payload.ClientID),
			clientSecret: strings.TrimSpace(payload.ClientSecret),
		},
	}
	for _, pair := range ordered {
		if pair.clientID != "" && pair.clientSecret != "" {
			return pair.clientID, pair.clientSecret
		}
	}
	return "", ""
}

func tokenSnapshotFromToken(token *oauth2.Token) tokenSnapshot {
	if token == nil {
		return tokenSnapshot{}
	}
	snapshot := tokenSnapshot{
		accessToken:  token.AccessToken,
		refreshToken: token.RefreshToken,
		tokenType:    token.TokenType,
	}
	if !token.Expiry.IsZero() {
		snapshot.expiryUnixMs = token.Expiry.UnixMilli()
	}
	return snapshot
}

func persistAuthToken(authPath string, token *oauth2.Token) error {
	if strings.TrimSpace(authPath) == "" {
		return fmt.Errorf("auth file path is required")
	}
	if token == nil {
		return fmt.Errorf("token is nil")
	}
	data, err := os.ReadFile(authPath)
	if err != nil {
		return fmt.Errorf("failed to read auth file %s: %w", authPath, err)
	}
	root := make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("failed to parse auth file %s: %w", authPath, err)
	}

	tokenMap := map[string]any{}
	if rawToken, ok := root["token"]; ok {
		if err := json.Unmarshal(rawToken, &tokenMap); err != nil {
			return fmt.Errorf("failed to parse token in auth file %s: %w", authPath, err)
		}
	}
	tokenMap["access_token"] = token.AccessToken
	if strings.TrimSpace(token.RefreshToken) != "" {
		tokenMap["refresh_token"] = token.RefreshToken
	}
	if strings.TrimSpace(token.TokenType) != "" {
		tokenMap["token_type"] = token.TokenType
	}
	if !token.Expiry.IsZero() {
		tokenMap["expiry_date"] = token.Expiry.UnixMilli()
	}

	tokenJSON, err := json.Marshal(tokenMap)
	if err != nil {
		return fmt.Errorf("failed to marshal token for auth file %s: %w", authPath, err)
	}
	root["token"] = tokenJSON

	updated, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal auth file %s: %w", authPath, err)
	}
	return writePrivateFileAtomically(authPath, append(updated, '\n'))
}

func writePrivateFileAtomically(path string, data []byte) error {
	dir := filepath.Dir(path)
	tempFile, err := os.CreateTemp(dir, ".clasprc.tmp-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file in %s: %w", dir, err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()
	if err := tempFile.Chmod(0600); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("failed to set temp file permissions: %w", err)
	}
	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	// On Windows, os.Rename fails if the destination already exists.
	// Back up the original first so we can restore it if rename fails.
	backupPath := path + ".bak"
	hadOriginal := false
	if _, err := os.Stat(path); err == nil {
		hadOriginal = true
		_ = os.Remove(backupPath) // remove stale backup if any
		if err := os.Rename(path, backupPath); err != nil {
			return fmt.Errorf("failed to back up existing file %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat existing file %s: %w", path, err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		if hadOriginal {
			if restoreErr := os.Rename(backupPath, path); restoreErr != nil {
				return fmt.Errorf("failed to replace auth file %s: %w (also failed to restore backup: %v)", path, err, restoreErr)
			}
		}
		return fmt.Errorf("failed to replace auth file %s: %w", path, err)
	}
	if hadOriginal {
		_ = os.Remove(backupPath)
	}
	return nil
}

func buildOAuthConfig(clientID, clientSecret string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes: []string{
			script.ScriptProjectsScope,
			script.ScriptDeploymentsScope,
			drive.DriveFileScope, // Required to create new projects via Drive API if needed
		},
		Endpoint: google.Endpoint,
	}
}

// openBrowser opens the specified URL in the default browser of the user.
func openBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Printf("Failed to open browser: %v", err)
		log.Printf("Please manually open your browser and go to: %s", url)
	}
}

func generateStateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate state token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

var (
	// テストで差し替えられるよう、関数参照を保持する。
	openBrowserFn        = openBrowser
	generateStateTokenFn = generateStateToken
)

type oauthState struct {
	token string
	used  bool
	mu    sync.Mutex
}

func (s *oauthState) validate(incoming string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.used {
		return fmt.Errorf("state token already used")
	}
	if incoming == "" || incoming != s.token {
		return fmt.Errorf("invalid state in response")
	}
	s.used = true
	return nil
}

// Login performs the OAuth2 login flow using the project-local token cache.
func Login(ctx context.Context, config *oauth2.Config) (*http.Client, error) {
	cacheFile, err := tokenCacheFile()
	if err != nil {
		return nil, fmt.Errorf("failed to get token cache file path: %w", err)
	}
	return loginWithCachePath(ctx, config, cacheFile)
}

// LoginWithCachePath performs the OAuth2 login flow using an explicit cache file path.
func LoginWithCachePath(ctx context.Context, config *oauth2.Config, cacheFile string) (*http.Client, error) {
	if strings.TrimSpace(cacheFile) == "" {
		return nil, fmt.Errorf("token cache file path is required")
	}
	return loginWithCachePath(ctx, config, cacheFile)
}

func loginWithCachePath(ctx context.Context, config *oauth2.Config, cacheFile string) (*http.Client, error) {
	var client *http.Client // Declare client here to be in scope for final return

	// Try to load token from cache
	token, _, _, loadErr := loadToken(cacheFile)
	if loadErr == nil {
		// Attempt to use the cached token. config.TokenSource will automatically
		// refresh the token if it's expired and a refresh token is available.

		tokenSource := config.TokenSource(ctx, token)
		freshToken, err := tokenSource.Token() // This will attempt to refresh if needed
		if err == nil {
			// If a fresh token is successfully obtained (or the cached token was still valid),
			// save it back to cache in case it was refreshed.
			if freshToken.AccessToken != token.AccessToken { // Only save if it's actually a new token
				if saveErr := saveToken(cacheFile, freshToken); saveErr != nil {
					log.Printf("Warning: Failed to save refreshed token: %v", saveErr)
				}
			}
			fmt.Printf("Using cached or refreshed token from %s\n", cacheFile)
			client = oauth2.NewClient(ctx, tokenSource) // Assign to outer scope client
			return client, nil
		}
		log.Printf("Cached token or refresh token is invalid, proceeding with full OAuth flow: %v", err)
		// Fall through to full OAuth flow if refresh failed
	} else {
		log.Printf("No cached token found or failed to load: %v", loadErr)
	}

	// Token not found or invalid, perform full OAuth2 flow

	// Start a local server on a random free port.
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return nil, fmt.Errorf("failed to start local server: %w", err)
	}
	defer listener.Close()

	// Update the RedirectURL with the dynamically chosen port
	port := listener.Addr().(*net.TCPAddr).Port
	config.RedirectURL = fmt.Sprintf("http://localhost:%d/oauth2callback", port)

	stateToken, err := generateStateTokenFn()
	if err != nil {
		return nil, err
	}
	state := &oauthState{token: stateToken}
	authCodeURL := config.AuthCodeURL(stateToken, oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser:\n%s\n", authCodeURL)
	openBrowserFn(authCodeURL)

	// Start a local server to handle the redirect
	type authResult struct {
		code string
		err  error
	}
	resultChan := make(chan authResult, 1)
	sendResult := func(result authResult) {
		select {
		case resultChan <- result:
		default:
		}
	}

	// Use a local ServeMux to avoid polluting the global http handler.
	mux := http.NewServeMux()
	srv := &http.Server{Handler: mux}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if message, isError := shutdownServerMessage(srv.Shutdown(shutdownCtx)); message != "" {
			if isError {
				log.Println(message)
			} else {
				fmt.Println(message)
			}
		}
	}()

	mux.HandleFunc("/oauth2callback", func(w http.ResponseWriter, r *http.Request) {
		incomingState := r.URL.Query().Get("state")
		if err := state.validate(incomingState); err != nil {
			errmsg := "Invalid state in response."
			http.Error(w, errmsg, http.StatusBadRequest)
			log.Println(errmsg)
			sendResult(authResult{err: err})
			return
		}
		log.Printf("Valid state token received")
		code := r.URL.Query().Get("code")
		if code == "" {
			errmsg := "No code in response. "
			if errParam := r.URL.Query().Get("error"); errParam != "" {
				errmsg += fmt.Sprintf("Google returned error: %s", errParam)
			}
			http.Error(w, errmsg, http.StatusBadRequest)
			log.Println(errmsg)
			sendResult(authResult{err: fmt.Errorf("%s", errmsg)})
			return
		}
		log.Printf("Received authorization code (length: %d)", len(code))
		fmt.Fprintf(w, "Authentication successful! You can close this tab.")
		sendResult(authResult{code: code})
	})

	go func() {
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("listen error: %v\n", err)
			sendResult(authResult{err: fmt.Errorf("listen: %w", err)})
		}
	}()
	fmt.Println("Waiting for authentication flow to complete in browser...")

	// Wait for the auth code or an error
	var result authResult
	select {
	case result = <-resultChan:
	case <-ctx.Done():
		return nil, fmt.Errorf("authentication cancelled: %w", ctx.Err())
	}
	if result.err != nil {
		return nil, fmt.Errorf("authentication failed: %w", result.err)
	}

	// Exchange the code for a token
	token, err = config.Exchange(ctx, result.code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange authorization code: %w", err)
	}

	// Save the token
	if err := saveToken(cacheFile, token); err != nil {
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	// Create tokenSource for the newly obtained token
	tokenSource := config.TokenSource(ctx, token)
	client = oauth2.NewClient(ctx, tokenSource) // Assign to outer scope client

	return client, nil
}

func shutdownServerMessage(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "OAuth callback server shutdown timed out; authentication already completed.", false
	}
	return fmt.Sprintf("Error shutting down server: %v", err), true
}

// ImportAuthFile reads a .clasprc.json-style auth file and saves the token
// to cacheFile (.glasp/access.json). It returns an error if cacheFile already exists.
func ImportAuthFile(authPath, cacheFile string) error {
	if strings.TrimSpace(cacheFile) == "" {
		return fmt.Errorf("token cache file path is required")
	}

	payload, cleanPath, err := loadAuthFilePayload(authPath)
	if err != nil {
		return err
	}
	token := tokenFromPayload(payload)
	if strings.TrimSpace(token.AccessToken) == "" && strings.TrimSpace(token.RefreshToken) == "" {
		return fmt.Errorf("auth file %s has no access_token or refresh_token", cleanPath)
	}
	if token.TokenType == "" {
		token.TokenType = "Bearer"
	}
	return saveTokenExclusive(cacheFile, token)
}

// Logout deletes the cached token file.
func Logout() error {
	cacheFile, err := tokenCacheFile()
	if err != nil {
		return fmt.Errorf("failed to get token cache file path: %w", err)
	}

	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		fmt.Printf("No cached token found at %s. Already logged out.\n", cacheFile)
		return nil
	}

	if err := os.Remove(cacheFile); err != nil {
		return fmt.Errorf("failed to remove token cache file %s: %w", cacheFile, err)
	}

	fmt.Printf("Successfully logged out. Removed %s\n", cacheFile)
	return nil
}

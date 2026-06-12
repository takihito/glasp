// Package auth resolves OAuth2 credentials and tokens for the Script API.
//
// File layout:
//   - auth.go        — OAuth config resolution and public client entry points
//   - login_flow.go  — interactive login with local callback server
//   - token_store.go — token cache persistence (.glasp/access.json)
//   - payload.go     — .clasprc.json payload parsing
//   - source.go      — auth source selection and token refresh
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"  // Required for some scopes, e.g., drive.file
	"google.golang.org/api/script/v1" // Google Apps Script API
)

// stdout receives user-facing output (login progress, token cache notices).
// It is a variable so callers and tests can capture or redirect it; warnings
// and diagnostics keep going through the log package.
var stdout io.Writer = os.Stdout

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
		// Environment variable overrides embedded credentials.
		clientID = envClientID
	}

	if ldflagsClientSecret != "" {
		clientSecret = ldflagsClientSecret
	}
	if envClientSecret := os.Getenv("GLASP_CLIENT_SECRET"); envClientSecret != "" {
		// Environment variable overrides embedded credentials.
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

// ClientFromAuthJSON creates an authenticated HTTP client from raw .clasprc.json
// content (e.g. the value of the GLASP_AUTH environment variable).
// Refreshed tokens are not persisted to disk.
func ClientFromAuthJSON(ctx context.Context, jsonContent string) (*http.Client, error) {
	return clientFromAuthJSON(ctx, jsonContent)
}

func clientFromAuthJSON(ctx context.Context, jsonContent string) (*http.Client, error) {
	trimmed := strings.TrimSpace(jsonContent)
	if trimmed == "" {
		return nil, fmt.Errorf("GLASP_AUTH content is empty")
	}
	var payload authFilePayload
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil, fmt.Errorf("failed to parse GLASP_AUTH content: %w", err)
	}
	// persistPath is empty: CI environments are ephemeral so token refresh is
	// not written back to disk.
	return buildClientFromPayload(ctx, &payload, "GLASP_AUTH", "")
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
		fmt.Fprintf(stdout, "No cached token found at %s. Already logged out.\n", cacheFile)
		return nil
	}

	if err := os.Remove(cacheFile); err != nil {
		return fmt.Errorf("failed to remove token cache file %s: %w", cacheFile, err)
	}

	fmt.Fprintf(stdout, "Successfully logged out. Removed %s\n", cacheFile)
	return nil
}

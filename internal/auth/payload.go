package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// authFilePayload models the .clasprc.json variants accepted as auth input:
// clasp's oauth2ClientSettings, Google client-secret files (installed/web),
// flat clientId/clientSecret pairs, and the cached token itself.
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

// oauthCredentialsFromPayload returns the first complete clientId/clientSecret
// pair, preferring clasp's oauth2ClientSettings, then Google client-secret
// formats (installed, web), then flat fields.
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

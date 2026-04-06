package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestTokenRefresh(t *testing.T) {
	// 1. Setup a mock OAuth2 server
	const newAccessToken = "new-access-token"
	const newRefreshToken = "new-refresh-token" // Google might issue a new refresh token
	const expiresInSeconds = 3600

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			t.Errorf("Expected to request '/token', got: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("Failed to parse form: %v", err)
		}
		if r.Form.Get("grant_type") != "refresh_token" {
			t.Errorf("Expected grant_type 'refresh_token', got: %s", r.Form.Get("grant_type"))
		}
		if r.Form.Get("refresh_token") != "test-refresh-token" {
			t.Errorf("Expected refresh_token 'test-refresh-token', got: %s", r.Form.Get("refresh_token"))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  newAccessToken,
			"token_type":    "Bearer",
			"refresh_token": newRefreshToken,
			"expires_in":    expiresInSeconds,
		})
	}))
	defer server.Close()

	// 2. Create a local config object for the test and point it to the mock server
	t.Setenv("GLASP_CLIENT_ID", "test-client-id")
	t.Setenv("GLASP_CLIENT_SECRET", "test-client-secret")
	testConfig, err := Config() // Get a new config object
	if err != nil {
		t.Fatalf("Config failed: %v", err)
	}
	testConfig.Endpoint.TokenURL = server.URL + "/token"

	// 3. Patch tokenCacheFile to use a temporary file
	tmpFile, err := os.CreateTemp("", "glasp_refresh_test_*.json")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	originalTokenCacheFile := tokenCacheFile
	defer func() { tokenCacheFile = originalTokenCacheFile }()
	tokenCacheFile = func() (string, error) {
		return tmpFile.Name(), nil
	}

	// 4. Create and save an expired token
	expiredToken := &oauth2.Token{
		AccessToken:  "expired-access-token",
		TokenType:    "Bearer",
		RefreshToken: "test-refresh-token",
		Expiry:       time.Now().Add(-1 * time.Hour), // Expired
	}
	if err := saveToken(tmpFile.Name(), expiredToken); err != nil {
		t.Fatalf("Failed to save initial token: %v", err)
	}

	// 5. Call Login with context and the testConfig
	issuedAt := time.Now()
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, server.Client())
	client, err := Login(ctx, testConfig) // Pass the testConfig here
	if err != nil {
		t.Fatalf("Login failed during refresh test: %v", err)
	}
	if client == nil {
		t.Fatal("Login returned a nil client")
	}

	// 6. Verify the new token was saved to cache
	loadedToken, _, _, err := loadToken(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load token after refresh: %v", err)
	}

	if loadedToken.AccessToken != newAccessToken {
		t.Errorf("Expected new access token '%s', got '%s'", newAccessToken, loadedToken.AccessToken)
	}
	if loadedToken.RefreshToken != newRefreshToken {
		t.Errorf("Expected new refresh token '%s', got '%s'", newRefreshToken, loadedToken.RefreshToken)
	}
	expiryDelta := loadedToken.Expiry.Sub(issuedAt)
	if expiryDelta < 59*time.Minute || expiryDelta > 61*time.Minute {
		t.Errorf("Expected new expiry to be about 1 hour, got %v", loadedToken.Expiry)
	}
}

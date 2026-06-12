package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/oauth2"
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

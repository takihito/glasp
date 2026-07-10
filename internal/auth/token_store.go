package auth

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/takihito/glasp/internal/config"
	"golang.org/x/oauth2"
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

// ensureTokenCacheDir creates the directory holding the token cache file.
// When the cache file lives under a .glasp directory, EnsureGlaspDir is used
// so that .claspignore is also kept up to date. Otherwise (e.g. tests passing
// a bare temp path) the directory is simply created.
func ensureTokenCacheDir(file string) error {
	dir := filepath.Dir(file)
	if filepath.Base(dir) == ".glasp" {
		projectRoot := filepath.Dir(dir)
		if err := config.EnsureGlaspDir(projectRoot); err != nil {
			return fmt.Errorf("failed to create token cache directory: %w", err)
		}
		return nil
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create token cache directory: %w", err)
	}
	return nil
}

// tokenFileJSON renders the token in .clasprc.json-compatible format.
// Only token data is persisted; OAuth client credentials are never cached
// to avoid creating a long-lived secret store in the project directory.
func tokenFileJSON(token *oauth2.Token) ([]byte, error) {
	payload := authFilePayload{}
	payload.Token.AccessToken = token.AccessToken
	payload.Token.RefreshToken = token.RefreshToken
	payload.Token.TokenType = token.TokenType
	if !token.Expiry.IsZero() {
		payload.Token.ExpiryDate = token.Expiry.UnixMilli()
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal token to file: %w", err)
	}
	return append(data, '\n'), nil
}

// saveToken saves the token to a file, overwriting any existing file.
func saveToken(file string, token *oauth2.Token) error {
	fmt.Fprintf(stdout, "Saving credential file to: %s\n", file)
	if err := ensureTokenCacheDir(file); err != nil {
		return err
	}
	data, err := tokenFileJSON(token)
	if err != nil {
		return err
	}
	return writePrivateFileAtomically(file, data)
}

// saveTokenExclusive atomically creates a new token cache file, failing if
// the file already exists. This avoids the TOCTOU race of stat-then-write.
func saveTokenExclusive(file string, token *oauth2.Token) error {
	if err := ensureTokenCacheDir(file); err != nil {
		return err
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

	data, err := tokenFileJSON(token)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Saving credential file to: %s\n", file)
	if _, err := f.Write(data); err != nil {
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

// persistingTokenSource wraps a TokenSource and writes refreshed tokens back
// to the auth file so subsequent runs reuse them.
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
		slog.Warn("failed to persist refreshed token", "path", p.authPath, "error", err)
	} else {
		p.mu.Lock()
		p.lastSnapshot = snapshot
		p.hasLast = true
		p.mu.Unlock()
	}
	return token, nil
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

// persistAuthToken updates the "token" object inside an existing auth file
// while leaving all other fields untouched.
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

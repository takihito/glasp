package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

var (
	clientFromAuthFileWithRefreshFn = clientFromAuthFileWithRefresh
	clientFromAuthJSONFn            = clientFromAuthJSON
	configFn                        = Config
	loginWithCachePathFn            = LoginWithCachePath
)

// SourceKind identifies where authentication material is loaded from.
type SourceKind string

const (
	SourceKindProjectCache SourceKind = "project_cache"
	SourceKindAuthFile     SourceKind = "auth_file"
	// SourceKindAuthJSON is used when auth credentials are provided as raw
	// JSON content (e.g. via the GLASP_AUTH environment variable) rather
	// than a file path.
	SourceKindAuthJSON SourceKind = "auth_json"
)

// Source represents a resolved authentication source.
type Source struct {
	Kind    SourceKind
	Path    string // used by SourceKindProjectCache and SourceKindAuthFile
	Content string // raw JSON content, used by SourceKindAuthJSON
}

// ResolveAuthSource chooses auth source by CLI input.
// If authPath is provided, .clasprc.json is used; otherwise project-local cache is used.
func ResolveAuthSource(projectRoot, authPath string) (Source, error) {
	trimmedAuthPath := strings.TrimSpace(authPath)
	if trimmedAuthPath != "" {
		return Source{
			Kind: SourceKindAuthFile,
			Path: filepath.Clean(trimmedAuthPath),
		}, nil
	}

	if strings.TrimSpace(projectRoot) == "" {
		return Source{}, fmt.Errorf("project root is required when --auth is not specified")
	}

	return Source{
		Kind: SourceKindProjectCache,
		Path: filepath.Join(projectRoot, ".glasp", "access.json"),
	}, nil
}

// EnsureAccessToken resolves an authenticated HTTP client and refreshes token if needed.
func EnsureAccessToken(ctx context.Context, source Source) (*http.Client, error) {
	switch source.Kind {
	case SourceKindAuthJSON:
		content := strings.TrimSpace(source.Content)
		if content == "" {
			return nil, fmt.Errorf("GLASP_AUTH is empty")
		}
		return clientFromAuthJSONFn(ctx, content)
	case SourceKindAuthFile:
		path := strings.TrimSpace(source.Path)
		if path == "" {
			return nil, fmt.Errorf("auth source path is required")
		}
		return clientFromAuthFileWithRefreshFn(ctx, path)
	case SourceKindProjectCache:
		path := source.Path
		if strings.TrimSpace(path) == "" {
			return nil, fmt.Errorf("auth source path is required")
		}
		oauthConfig, err := configFn()
		if err != nil {
			return nil, err
		}
		return loginWithCachePathFn(ctx, oauthConfig, path)
	default:
		return nil, fmt.Errorf("unsupported auth source kind: %s", source.Kind)
	}
}

func clientFromAuthFileWithRefresh(ctx context.Context, authPath string) (*http.Client, error) {
	payload, cleanPath, err := loadAuthFilePayload(authPath)
	if err != nil {
		return nil, err
	}
	return buildClientFromPayload(ctx, payload, "auth file "+cleanPath, cleanPath)
}

// buildClientFromPayload creates an OAuth client from a parsed authFilePayload.
// source is used in error/log messages (e.g. "auth file /path/.clasprc.json" or "GLASP_AUTH").
// persistPath, when non-empty, causes refreshed tokens to be written back to disk.
func buildClientFromPayload(ctx context.Context, payload *authFilePayload, source, persistPath string) (*http.Client, error) {
	token := tokenFromPayload(payload)
	if token.TokenType == "" {
		token.TokenType = "Bearer"
	}

	clientID, clientSecret := oauthCredentialsFromPayload(payload)
	if strings.TrimSpace(token.RefreshToken) != "" {
		// If the auth payload carries no client credentials, fall back to the
		// credentials embedded at build time (ldflags) or GLASP_CLIENT_ID/SECRET.
		if clientID == "" || clientSecret == "" {
			embedded, err := Config()
			if err != nil {
				return nil, fmt.Errorf("%s has no clientId/clientSecret and embedded credentials are unavailable: %w", source, err)
			}
			clientID = embedded.ClientID
			clientSecret = embedded.ClientSecret
		}
		oauthConfig := buildOAuthConfig(clientID, clientSecret)
		forcedRefreshSeed := &oauth2.Token{
			RefreshToken: token.RefreshToken,
			TokenType:    token.TokenType,
			Expiry:       time.Unix(0, 0),
		}
		refreshed, refreshErr := oauthConfig.TokenSource(ctx, forcedRefreshSeed).Token()
		if refreshErr != nil {
			if strings.TrimSpace(token.AccessToken) == "" {
				return nil, fmt.Errorf("failed to refresh token from %s: %w", source, refreshErr)
			}
			log.Printf("Warning: failed to refresh token from %s, falling back to token.access_token: %v", source, refreshErr)
		} else {
			token = mergeRefreshedToken(token, refreshed)
			if persistPath != "" {
				if err := persistAuthToken(persistPath, token); err != nil {
					log.Printf("Warning: failed to persist refreshed token to %s: %v", persistPath, err)
				}
			}
		}

		var tokenSource oauth2.TokenSource
		if persistPath != "" {
			tokenSource = oauth2.ReuseTokenSource(token, &persistingTokenSource{
				base:         oauthConfig.TokenSource(ctx, token),
				authPath:     persistPath,
				lastSnapshot: tokenSnapshotFromToken(token),
				hasLast:      true,
			})
		} else {
			tokenSource = oauthConfig.TokenSource(ctx, token)
		}
		return oauth2.NewClient(ctx, tokenSource), nil
	}

	if strings.TrimSpace(token.AccessToken) == "" {
		return nil, fmt.Errorf("%s is missing token.access_token", source)
	}
	return oauth2.NewClient(ctx, oauth2.StaticTokenSource(token)), nil
}

func tokenFromPayload(payload *authFilePayload) *oauth2.Token {
	token := &oauth2.Token{
		AccessToken:  strings.TrimSpace(payload.Token.AccessToken),
		RefreshToken: strings.TrimSpace(payload.Token.RefreshToken),
		TokenType:    strings.TrimSpace(payload.Token.TokenType),
	}
	if payload.Token.ExpiryDate > 0 {
		token.Expiry = time.UnixMilli(payload.Token.ExpiryDate)
	}
	return token
}

func mergeRefreshedToken(original, refreshed *oauth2.Token) *oauth2.Token {
	if refreshed == nil {
		return original
	}
	merged := &oauth2.Token{
		AccessToken:  strings.TrimSpace(refreshed.AccessToken),
		RefreshToken: strings.TrimSpace(refreshed.RefreshToken),
		TokenType:    strings.TrimSpace(refreshed.TokenType),
		Expiry:       refreshed.Expiry,
	}
	if merged.AccessToken == "" {
		merged.AccessToken = strings.TrimSpace(original.AccessToken)
	}
	if merged.RefreshToken == "" {
		merged.RefreshToken = strings.TrimSpace(original.RefreshToken)
	}
	if merged.TokenType == "" {
		merged.TokenType = strings.TrimSpace(original.TokenType)
	}
	if merged.TokenType == "" {
		merged.TokenType = "Bearer"
	}
	if merged.Expiry.IsZero() {
		merged.Expiry = original.Expiry
	}
	return merged
}

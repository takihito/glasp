package auth

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

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
		_, err := loginWithCachePath(context.Background(), cfg, cacheFile, false)
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

	if _, err := loginWithCachePath(ctx, cfg, cacheFile, false); err == nil || !strings.Contains(err.Error(), "authentication cancelled") {
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

func TestLoginWithCachePathPKCEEnabled(t *testing.T) {
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
	openBrowserFn = func(u string) { authURLCh <- u }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		_, err := loginWithCachePath(ctx, cfg, cacheFile, true)
		errCh <- err
	}()

	authURL := <-authURLCh
	cancel()
	<-errCh

	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("failed to parse auth URL: %v", err)
	}
	q := parsed.Query()
	if got := q.Get("code_challenge_method"); got != "S256" {
		t.Fatalf("code_challenge_method=%q, want S256", got)
	}
	if strings.TrimSpace(q.Get("code_challenge")) == "" {
		t.Fatalf("expected code_challenge query parameter")
	}
}

func TestLoginWithCachePathPKCEDisabled(t *testing.T) {
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
	openBrowserFn = func(u string) { authURLCh <- u }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		_, err := loginWithCachePath(ctx, cfg, cacheFile, false)
		errCh <- err
	}()

	authURL := <-authURLCh
	cancel()
	<-errCh

	parsed, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("failed to parse auth URL: %v", err)
	}
	q := parsed.Query()
	if v := q.Get("code_challenge"); v != "" {
		t.Fatalf("expected no code_challenge, got %q", v)
	}
	if v := q.Get("code_challenge_method"); v != "" {
		t.Fatalf("expected no code_challenge_method, got %q", v)
	}
}

package main

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/takihito/glasp/internal/config"
)

// writeGlaspConfigRaw writes raw bytes to .glasp/config.json under root,
// creating the .glasp directory first. Used to inject a malformed config.
func writeGlaspConfigRaw(t *testing.T, root string, data []byte) {
	t.Helper()
	if err := config.EnsureGlaspDir(root); err != nil {
		t.Fatalf("EnsureGlaspDir failed: %v", err)
	}
	path := filepath.Join(root, ".glasp", "config.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func TestResolveHTTPTimeout(t *testing.T) {
	t.Run("no-timeout flag returns zero (unlimited)", func(t *testing.T) {
		if got := resolveHTTPTimeout(0, true); got != 0 {
			t.Fatalf("resolveHTTPTimeout(0, true) = %v, want 0", got)
		}
	})

	t.Run("no-timeout overrides positive flag", func(t *testing.T) {
		if got := resolveHTTPTimeout(60, true); got != 0 {
			t.Fatalf("resolveHTTPTimeout(60, true) = %v, want 0", got)
		}
	})

	t.Run("positive flag wins", func(t *testing.T) {
		// Set up a project with a config value to confirm the flag takes priority.
		root := useTempDir(t)
		if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "s"}); err != nil {
			t.Fatalf("SaveClaspConfig failed: %v", err)
		}
		if err := config.SaveGlaspConfig(root, &config.GlaspConfig{TimeoutSeconds: 30}); err != nil {
			t.Fatalf("SaveGlaspConfig failed: %v", err)
		}
		if got := resolveHTTPTimeout(60, false); got != 60*time.Second {
			t.Fatalf("resolveHTTPTimeout(60, false) = %v, want 60s", got)
		}
	})

	t.Run("config value used when no flag", func(t *testing.T) {
		root := useTempDir(t)
		if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "s"}); err != nil {
			t.Fatalf("SaveClaspConfig failed: %v", err)
		}
		if err := config.SaveGlaspConfig(root, &config.GlaspConfig{TimeoutSeconds: 30}); err != nil {
			t.Fatalf("SaveGlaspConfig failed: %v", err)
		}
		if got := resolveHTTPTimeout(0, false); got != 30*time.Second {
			t.Fatalf("resolveHTTPTimeout(0, false) = %v, want 30s", got)
		}
	})

	t.Run("config timeoutSeconds zero falls back to default", func(t *testing.T) {
		root := useTempDir(t)
		if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "s"}); err != nil {
			t.Fatalf("SaveClaspConfig failed: %v", err)
		}
		if err := config.SaveGlaspConfig(root, &config.GlaspConfig{TimeoutSeconds: 0}); err != nil {
			t.Fatalf("SaveGlaspConfig failed: %v", err)
		}
		if got := resolveHTTPTimeout(0, false); got != defaultHTTPTimeout {
			t.Fatalf("resolveHTTPTimeout(0, false) = %v, want %v", got, defaultHTTPTimeout)
		}
	})

	t.Run("no glasp config falls back to default", func(t *testing.T) {
		root := useTempDir(t)
		if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "s"}); err != nil {
			t.Fatalf("SaveClaspConfig failed: %v", err)
		}
		if got := resolveHTTPTimeout(0, false); got != defaultHTTPTimeout {
			t.Fatalf("resolveHTTPTimeout(0, false) = %v, want %v", got, defaultHTTPTimeout)
		}
	})

	t.Run("outside project falls back to default", func(t *testing.T) {
		// useTempDir chdirs into an empty dir with no .clasp.json, so
		// FindProjectRoot returns "".
		_ = useTempDir(t)
		if got := resolveHTTPTimeout(0, false); got != defaultHTTPTimeout {
			t.Fatalf("resolveHTTPTimeout(0, false) = %v, want %v", got, defaultHTTPTimeout)
		}
	})

	t.Run("malformed config falls back to default", func(t *testing.T) {
		root := useTempDir(t)
		if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "s"}); err != nil {
			t.Fatalf("SaveClaspConfig failed: %v", err)
		}
		writeGlaspConfigRaw(t, root, []byte("{ this is not valid json"))
		if got := resolveHTTPTimeout(0, false); got != defaultHTTPTimeout {
			t.Fatalf("resolveHTTPTimeout(0, false) = %v, want %v", got, defaultHTTPTimeout)
		}
	})
}

func TestWithHTTPTimeoutRoundTrip(t *testing.T) {
	t.Run("positive duration is stored and retrieved", func(t *testing.T) {
		ctx := withHTTPTimeout(context.Background(), 42*time.Second)
		if got := httpTimeoutFromCtx(ctx); got != 42*time.Second {
			t.Fatalf("httpTimeoutFromCtx = %v, want 42s", got)
		}
	})

	t.Run("zero duration is not stored", func(t *testing.T) {
		ctx := withHTTPTimeout(context.Background(), 0)
		if got := httpTimeoutFromCtx(ctx); got != 0 {
			t.Fatalf("httpTimeoutFromCtx = %v, want 0", got)
		}
	})

	t.Run("negative duration is not stored", func(t *testing.T) {
		ctx := withHTTPTimeout(context.Background(), -1*time.Second)
		if got := httpTimeoutFromCtx(ctx); got != 0 {
			t.Fatalf("httpTimeoutFromCtx = %v, want 0", got)
		}
	})

	t.Run("missing value returns zero", func(t *testing.T) {
		if got := httpTimeoutFromCtx(context.Background()); got != 0 {
			t.Fatalf("httpTimeoutFromCtx = %v, want 0", got)
		}
	})
}

func TestApplyHTTPTimeout(t *testing.T) {
	t.Run("sets client timeout when present in ctx", func(t *testing.T) {
		ctx := withHTTPTimeout(context.Background(), 15*time.Second)
		client := &http.Client{}
		applyHTTPTimeout(ctx, client)
		if client.Timeout != 15*time.Second {
			t.Fatalf("client.Timeout = %v, want 15s", client.Timeout)
		}
	})

	t.Run("leaves client timeout unchanged when absent", func(t *testing.T) {
		client := &http.Client{Timeout: 5 * time.Second}
		applyHTTPTimeout(context.Background(), client)
		if client.Timeout != 5*time.Second {
			t.Fatalf("client.Timeout = %v, want 5s (unchanged)", client.Timeout)
		}
	})
}

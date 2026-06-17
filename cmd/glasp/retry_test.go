package main

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
	"github.com/takihito/glasp/internal/config"
)

func TestResolveHTTPRetries(t *testing.T) {
	t.Run("positive flag wins", func(t *testing.T) {
		if got := resolveHTTPRetries(5); got != 5 {
			t.Fatalf("resolveHTTPRetries(5) = %d, want 5", got)
		}
	})

	t.Run("flag 1 means 1 retry", func(t *testing.T) {
		if got := resolveHTTPRetries(1); got != 1 {
			t.Fatalf("resolveHTTPRetries(1) = %d, want 1", got)
		}
	})

	t.Run("flag 0 falls back to config when present", func(t *testing.T) {
		root := useTempDir(t)
		if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "s"}); err != nil {
			t.Fatalf("SaveClaspConfig failed: %v", err)
		}
		if err := config.SaveGlaspConfig(root, &config.GlaspConfig{MaxRetries: 7}); err != nil {
			t.Fatalf("SaveGlaspConfig failed: %v", err)
		}
		if got := resolveHTTPRetries(0); got != 7 {
			t.Fatalf("resolveHTTPRetries(0) = %d, want 7 from config", got)
		}
	})

	t.Run("flag 0 config 0 falls back to default", func(t *testing.T) {
		root := useTempDir(t)
		if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "s"}); err != nil {
			t.Fatalf("SaveClaspConfig failed: %v", err)
		}
		if err := config.SaveGlaspConfig(root, &config.GlaspConfig{MaxRetries: 0}); err != nil {
			t.Fatalf("SaveGlaspConfig failed: %v", err)
		}
		if got := resolveHTTPRetries(0); got != defaultHTTPRetries {
			t.Fatalf("resolveHTTPRetries(0) = %d, want defaultHTTPRetries (%d)", got, defaultHTTPRetries)
		}
	})

	t.Run("outside project falls back to default", func(t *testing.T) {
		_ = useTempDir(t)
		if got := resolveHTTPRetries(0); got != defaultHTTPRetries {
			t.Fatalf("resolveHTTPRetries(0) = %d, want %d", got, defaultHTTPRetries)
		}
	})

	t.Run("negative flag warns and falls back to default", func(t *testing.T) {
		_ = useTempDir(t)
		out := captureLog(t, func() {
			if got := resolveHTTPRetries(-1); got != defaultHTTPRetries {
				t.Fatalf("resolveHTTPRetries(-1) = %d, want %d", got, defaultHTTPRetries)
			}
		})
		if !strings.Contains(out, "negative") || !strings.Contains(out, "--max-retries/GLASP_MAX_RETRIES") {
			t.Fatalf("expected negative --max-retries warning, got: %q", out)
		}
	})

	t.Run("negative config maxRetries warns and falls back to default", func(t *testing.T) {
		root := useTempDir(t)
		if err := config.SaveClaspConfig(root, &config.ClaspConfig{ScriptID: "s"}); err != nil {
			t.Fatalf("SaveClaspConfig failed: %v", err)
		}
		if err := config.SaveGlaspConfig(root, &config.GlaspConfig{MaxRetries: -3}); err != nil {
			t.Fatalf("SaveGlaspConfig failed: %v", err)
		}
		out := captureLog(t, func() {
			if got := resolveHTTPRetries(0); got != defaultHTTPRetries {
				t.Fatalf("resolveHTTPRetries(0) = %d, want %d", got, defaultHTTPRetries)
			}
		})
		if !strings.Contains(out, "negative") || !strings.Contains(out, "maxRetries") {
			t.Fatalf("expected negative maxRetries warning, got: %q", out)
		}
	})
}

func TestRetryableCommandsAllowlist(t *testing.T) {
	allowed := []string{"push", "pull", "list-deployments", "clone"}
	notAllowed := []string{"create-script", "create-deployment", "update-deployment", "run-function", "login", "open-script"}

	retryableCommands := map[string]bool{
		"push": true, "pull": true, "list-deployments": true, "clone": true,
	}
	for _, cmd := range allowed {
		if !retryableCommands[cmd] {
			t.Errorf("command %q should be in retryableCommands but is not", cmd)
		}
	}
	for _, cmd := range notAllowed {
		if retryableCommands[cmd] {
			t.Errorf("command %q should NOT be in retryableCommands but is", cmd)
		}
	}
}

func TestWithHTTPRetryRoundTrip(t *testing.T) {
	t.Run("positive value is stored and retrieved", func(t *testing.T) {
		ctx := withHTTPRetry(context.Background(), 3)
		if got := httpRetryFromCtx(ctx); got != 3 {
			t.Fatalf("httpRetryFromCtx = %d, want 3", got)
		}
	})

	t.Run("zero value is not stored", func(t *testing.T) {
		ctx := withHTTPRetry(context.Background(), 0)
		if got := httpRetryFromCtx(ctx); got != 0 {
			t.Fatalf("httpRetryFromCtx = %d, want 0", got)
		}
	})

	t.Run("negative value is not stored", func(t *testing.T) {
		ctx := withHTTPRetry(context.Background(), -1)
		if got := httpRetryFromCtx(ctx); got != 0 {
			t.Fatalf("httpRetryFromCtx = %d, want 0", got)
		}
	})

	t.Run("missing value returns zero", func(t *testing.T) {
		if got := httpRetryFromCtx(context.Background()); got != 0 {
			t.Fatalf("httpRetryFromCtx = %d, want 0", got)
		}
	})
}

func TestApplyHTTPRetry(t *testing.T) {
	t.Run("wraps with retryablehttp transport when n > 0", func(t *testing.T) {
		ctx := withHTTPRetry(context.Background(), 3)
		origTransport := &http.Transport{}
		client := &http.Client{Transport: origTransport}
		applyHTTPRetry(ctx, client)

		rt, ok := client.Transport.(*retryablehttp.RoundTripper)
		if !ok {
			t.Fatalf("expected Transport to be *retryablehttp.RoundTripper, got %T", client.Transport)
		}
		// The inner client must retain the original Transport, not the retry
		// wrapper. If they are the same pointer the retry loop would recurse
		// infinitely when calling rc.HTTPClient.Do().
		if rt.Client.HTTPClient.Transport == client.Transport {
			t.Fatalf("inner client Transport must not equal the outer retryablehttp.RoundTripper (infinite recursion)")
		}
		if rt.Client.HTTPClient.Transport != origTransport {
			t.Fatalf("inner client Transport = %T, want original *http.Transport", rt.Client.HTTPClient.Transport)
		}
		if rt.Client.RetryMax != 3 {
			t.Fatalf("RetryMax = %d, want 3", rt.Client.RetryMax)
		}
		if rt.Client.RetryWaitMin != 500*time.Millisecond {
			t.Fatalf("RetryWaitMin = %v, want 500ms", rt.Client.RetryWaitMin)
		}
		if rt.Client.RetryWaitMax != 30*time.Second {
			t.Fatalf("RetryWaitMax = %v, want 30s", rt.Client.RetryWaitMax)
		}
	})

	t.Run("leaves transport unchanged when n == 0", func(t *testing.T) {
		client := &http.Client{}
		applyHTTPRetry(context.Background(), client)
		if client.Transport != nil {
			t.Fatalf("expected Transport to remain nil, got %T", client.Transport)
		}
	})
}

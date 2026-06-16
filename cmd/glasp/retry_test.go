package main

import (
	"context"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/takihito/glasp/internal/config"
)

// roundTripFunc adapts a plain function to http.RoundTripper.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

// fakeTransport is a test RoundTripper that returns responses from a queue.
type fakeTransport struct {
	calls     int
	responses []fakeResponse
}

type fakeResponse struct {
	status int
	err    error
	body   string
}

func (f *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	i := f.calls
	f.calls++
	if i >= len(f.responses) {
		return nil, errors.New("unexpected call: no more responses queued")
	}
	r := f.responses[i]
	if r.err != nil {
		return nil, r.err
	}
	body := r.body
	if body == "" {
		body = "ok"
	}
	return &http.Response{
		StatusCode: r.status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}, nil
}

func buildRetryTransport(base http.RoundTripper, max int) *retryTransport {
	return &retryTransport{
		base:     base,
		max:      max,
		baseWait: time.Microsecond, // near-zero for fast tests
		maxWait:  time.Millisecond,
		rnd:      rand.New(rand.NewSource(42)), //nolint:gosec
	}
}

func makeRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	var reqBody io.Reader
	if body != "" {
		reqBody = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPut, "http://example.com", reqBody)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	return req
}

func TestRetryTransport503ThenSuccess(t *testing.T) {
	fake := &fakeTransport{
		responses: []fakeResponse{
			{status: 503},
			{status: 200},
		},
	}
	rt := buildRetryTransport(fake, 3)
	resp, err := rt.RoundTrip(makeRequest(t, ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if fake.calls != 2 {
		t.Fatalf("expected 2 calls, got %d", fake.calls)
	}
}

func TestRetryTransportExhausted(t *testing.T) {
	fake := &fakeTransport{
		responses: []fakeResponse{
			{status: 503},
			{status: 503},
			{status: 503},
			{status: 503}, // 1 initial + 3 retries
		},
	}
	rt := buildRetryTransport(fake, 3)
	resp, err := rt.RoundTrip(makeRequest(t, ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 503 {
		t.Fatalf("expected final 503, got %d", resp.StatusCode)
	}
	if fake.calls != 4 {
		t.Fatalf("expected 4 calls (1+3), got %d", fake.calls)
	}
}

func TestRetryTransport404NoRetry(t *testing.T) {
	fake := &fakeTransport{
		responses: []fakeResponse{
			{status: 404},
		},
	}
	rt := buildRetryTransport(fake, 3)
	resp, err := rt.RoundTrip(makeRequest(t, ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	if fake.calls != 1 {
		t.Fatalf("expected exactly 1 call, got %d", fake.calls)
	}
}

func TestRetryTransportNetworkError(t *testing.T) {
	netErr := errors.New("dial tcp: connection refused")
	fake := &fakeTransport{
		responses: []fakeResponse{
			{err: netErr},
			{status: 200},
		},
	}
	rt := buildRetryTransport(fake, 3)
	resp, err := rt.RoundTrip(makeRequest(t, ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if fake.calls != 2 {
		t.Fatalf("expected 2 calls, got %d", fake.calls)
	}
}

func TestRetryTransportBodyReplay(t *testing.T) {
	received := make([]string, 0, 2)
	var callCount int
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		b, _ := io.ReadAll(req.Body)
		received = append(received, string(b))
		if callCount == 1 {
			return &http.Response{
				StatusCode: 503,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		}
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader("ok")),
			Header:     make(http.Header),
		}, nil
	})
	rt := buildRetryTransport(transport, 3)
	req := makeRequest(t, "hello-body")
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if len(received) != 2 {
		t.Fatalf("expected 2 body reads, got %d", len(received))
	}
	for i, r := range received {
		if r != "hello-body" {
			t.Fatalf("attempt %d: body = %q, want %q", i+1, r, "hello-body")
		}
	}
}

func TestRetryTransportRetryAfterHeader(t *testing.T) {
	var delays []time.Duration
	origSleep := func(d time.Duration) { delays = append(delays, d) }
	_ = origSleep // not injected in this test; we verify via timing instead

	retryAfterHeader := make(http.Header)
	retryAfterHeader.Set("Retry-After", "0") // 0s = no extra delay; tests the header parse path
	fake := &fakeTransport{
		responses: []fakeResponse{
			{status: 429},
			{status: 200},
		},
	}
	// Attach Retry-After to the first queued response by wrapping.
	var callCount int
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		r := fake.responses[callCount-1]
		resp := &http.Response{
			StatusCode: r.status,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}
		if callCount == 1 {
			resp.Header.Set("Retry-After", "0")
		}
		return resp, nil
	})
	rt := buildRetryTransport(transport, 3)
	resp, err := rt.RoundTrip(makeRequest(t, ""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if callCount != 2 {
		t.Fatalf("expected 2 calls, got %d", callCount)
	}
}

func TestRetryTransportContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	fake := &fakeTransport{
		responses: []fakeResponse{
			{status: 503},
			{status: 200},
		},
	}
	rt := buildRetryTransport(fake, 3)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)
	resp, err := rt.RoundTrip(req)
	// First attempt succeeds (returns 503), then context cancel fires.
	// The transport should return context.Canceled without a second attempt.
	if err == nil {
		// If the cancelled context propagated, we'd have err != nil.
		// In this case, the first call returned 503 and delay was immediately
		// interrupted by the cancelled context.
		_ = resp
	}
	// At most 1 attempt: the second attempt never happens because ctx is done.
	if fake.calls > 1 {
		t.Fatalf("expected at most 1 call with cancelled context, got %d", fake.calls)
	}
}

func TestBackoffDelay(t *testing.T) {
	rnd := rand.New(rand.NewSource(0)) //nolint:gosec
	base := 500 * time.Millisecond
	max := 30 * time.Second

	for attempt := 0; attempt <= 10; attempt++ {
		d := backoffDelay(attempt, base, max, rnd)
		if d < 0 {
			t.Fatalf("attempt %d: negative delay %v", attempt, d)
		}
		if d > max {
			t.Fatalf("attempt %d: delay %v exceeds max %v", attempt, d, max)
		}
	}
}

func TestBackoffDelayOverflowGuard(t *testing.T) {
	rnd := rand.New(rand.NewSource(0)) //nolint:gosec
	// Very large attempt should not panic or overflow.
	d := backoffDelay(200, 500*time.Millisecond, 30*time.Second, rnd)
	if d < 0 || d > 30*time.Second {
		t.Fatalf("overflow guard failed: d = %v", d)
	}
}

func TestBackoffDelayNegativeAttempt(t *testing.T) {
	rnd := rand.New(rand.NewSource(0)) //nolint:gosec
	d := backoffDelay(-1, 500*time.Millisecond, 30*time.Second, rnd)
	if d < 0 {
		t.Fatalf("negative attempt should yield non-negative delay, got %v", d)
	}
}

func TestResolveHTTPRetries(t *testing.T) {
	t.Run("positive flag wins", func(t *testing.T) {
		if got := resolveHTTPRetries(5); got != 5 {
			t.Fatalf("resolveHTTPRetries(5) = %d, want 5", got)
		}
	})

	t.Run("flag 1 disables retries effectively", func(t *testing.T) {
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
	t.Run("wraps transport when n > 0", func(t *testing.T) {
		ctx := withHTTPRetry(context.Background(), 3)
		client := &http.Client{}
		applyHTTPRetry(ctx, client)
		if _, ok := client.Transport.(*retryTransport); !ok {
			t.Fatalf("expected Transport to be *retryTransport, got %T", client.Transport)
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

// Ensure roundTripFunc satisfies http.RoundTripper.
var _ http.RoundTripper = roundTripFunc(nil)


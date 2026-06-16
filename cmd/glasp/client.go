package main

import (
	"bytes"
	"context"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/takihito/glasp/internal/auth"
	"github.com/takihito/glasp/internal/scriptapi"

	"google.golang.org/api/script/v1"
)

type httpTimeoutCtxKey struct{}
type httpRetryCtxKey struct{}

func withHTTPTimeout(ctx context.Context, d time.Duration) context.Context {
	if d <= 0 {
		return ctx
	}
	return context.WithValue(ctx, httpTimeoutCtxKey{}, d)
}

func httpTimeoutFromCtx(ctx context.Context) time.Duration {
	d, _ := ctx.Value(httpTimeoutCtxKey{}).(time.Duration)
	return d
}

type scriptClient interface {
	CreateProject(ctx context.Context, title, parentID string) (*script.Project, error)
	GetProject(ctx context.Context, scriptID string) (*script.Project, error)
	GetContent(ctx context.Context, scriptID string, versionNumber int64) (*script.Content, error)
	UpdateContent(ctx context.Context, scriptID string, content *script.Content) (*script.Content, error)
	CreateVersion(ctx context.Context, scriptID, description string) (*script.Version, error)
	CreateDeployment(ctx context.Context, scriptID string, deploymentConfig *script.DeploymentConfig) (*script.Deployment, error)
	UpdateDeployment(ctx context.Context, scriptID, deploymentID string, deploymentConfig *script.DeploymentConfig) (*script.Deployment, error)
	ListDeployments(ctx context.Context, scriptID string) ([]*script.Deployment, error)
	RunFunction(ctx context.Context, scriptID, functionName string, params []any, devMode bool) (*script.Operation, error)
}

func newScriptClientWithCachePathAndAuth(ctx context.Context, cacheFile, authPath string) (scriptClient, error) {
	return newScriptClientWithAuthInputs(ctx, cacheFile, authPath)
}

func newScriptClientWithAuthInputs(ctx context.Context, cacheFile, authPath string) (scriptClient, error) {
	var source auth.Source
	switch {
	case strings.TrimSpace(authPath) != "":
		source = auth.Source{
			Kind: auth.SourceKindAuthFile,
			Path: authPath,
		}
	case strings.TrimSpace(os.Getenv("GLASP_AUTH")) != "":
		source = auth.Source{
			Kind:    auth.SourceKindAuthJSON,
			Content: os.Getenv("GLASP_AUTH"),
		}
	case strings.TrimSpace(cacheFile) != "":
		source = auth.Source{
			Kind: auth.SourceKindProjectCache,
			Path: cacheFile,
		}
	default:
		oauthConfig, err := auth.Config()
		if err != nil {
			return nil, err
		}
		httpClient, err := auth.Login(ctx, oauthConfig)
		if err != nil {
			return nil, err
		}
		applyHTTPTimeout(ctx, httpClient)
		applyHTTPRetry(ctx, httpClient)
		return scriptapi.New(ctx, httpClient)
	}

	httpClient, err := auth.EnsureAccessToken(ctx, source)
	if err != nil {
		return nil, err
	}
	applyHTTPTimeout(ctx, httpClient)
	applyHTTPRetry(ctx, httpClient)
	return scriptapi.New(ctx, httpClient)
}

// applyHTTPTimeout sets the Timeout field of httpClient to the value stored in
// ctx by withHTTPTimeout. No-op when no timeout is present in ctx.
func applyHTTPTimeout(ctx context.Context, httpClient *http.Client) {
	if timeout := httpTimeoutFromCtx(ctx); timeout > 0 {
		httpClient.Timeout = timeout
	}
}

func withHTTPRetry(ctx context.Context, n int) context.Context {
	if n <= 0 {
		return ctx
	}
	return context.WithValue(ctx, httpRetryCtxKey{}, n)
}

func httpRetryFromCtx(ctx context.Context) int {
	n, _ := ctx.Value(httpRetryCtxKey{}).(int)
	return n
}

// applyHTTPRetry wraps httpClient.Transport with retryTransport when ctx
// carries a positive retry count. No-op otherwise.
func applyHTTPRetry(ctx context.Context, httpClient *http.Client) {
	n := httpRetryFromCtx(ctx)
	if n <= 0 {
		return
	}
	base := httpClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	httpClient.Transport = &retryTransport{
		base:    base,
		max:     n,
		baseWait: 500 * time.Millisecond,
		maxWait:  30 * time.Second,
		rnd:     rand.New(rand.NewSource(time.Now().UnixNano())), //nolint:gosec
	}
}

// retryTransport is an http.RoundTripper that retries on transient failures
// (network errors, 429, 5xx) with exponential backoff and full jitter.
type retryTransport struct {
	base     http.RoundTripper
	max      int
	baseWait time.Duration
	maxWait  time.Duration
	rnd      *rand.Rand
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Buffer the body once so we can replay it on retry.
	var bodyBytes []byte
	if req.Body != nil && req.Body != http.NoBody {
		if req.GetBody != nil {
			// GetBody is set by http.NewRequest for most body types; use it.
		} else {
			var err error
			bodyBytes, err = io.ReadAll(req.Body)
			req.Body.Close()
			if err != nil {
				return nil, err
			}
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			req.GetBody = func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(bodyBytes)), nil
			}
		}
	}

	var (
		resp *http.Response
		err  error
	)
	for attempt := 0; attempt <= t.max; attempt++ {
		if attempt > 0 {
			delay := backoffDelay(attempt-1, t.baseWait, t.maxWait, t.rnd)
			if resp != nil {
				if d := retryAfterDelay(resp.Header); d > delay {
					delay = d
				}
				// Drain and close the previous response body before the next attempt.
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				resp = nil
			}
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(delay):
			}
			// Restore the body for replay.
			if req.GetBody != nil {
				req.Body, err = req.GetBody()
				if err != nil {
					return nil, err
				}
			}
		}

		resp, err = t.base.RoundTrip(req)
		if err != nil {
			// Network error — retryable.
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			// Transient server error — retryable.
			continue
		}
		// Success or a non-retryable status.
		return resp, nil
	}
	// All attempts exhausted; return whatever we have.
	return resp, err
}

// retryAfterDelay parses the Retry-After header as seconds and returns the
// corresponding duration. Returns 0 if absent, non-numeric, or non-positive.
func retryAfterDelay(h http.Header) time.Duration {
	ra := h.Get("Retry-After")
	if ra == "" {
		return 0
	}
	secs, err := strconv.ParseFloat(ra, 64)
	if err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(secs * float64(time.Second))
}

// backoffDelay returns the wait duration for the given attempt index using
// exponential backoff with full jitter: [0, min(base*2^attempt, max)].
func backoffDelay(attempt int, base, max time.Duration, rnd *rand.Rand) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	// Cap the exponent to avoid int64 overflow (2^62 > math.MaxInt64/ns).
	exp := attempt
	if exp > 62 {
		exp = 62
	}
	d := time.Duration(math.Exp2(float64(exp))) * base
	if d <= 0 || d > max {
		d = max
	}
	if d <= 0 {
		return 0
	}
	return time.Duration(rnd.Int63n(int64(d)))
}

func newProjectScriptClient(ctx context.Context, projectRoot, authPath string, timeout time.Duration, retries int) (scriptClient, error) {
	ctx = withHTTPTimeout(ctx, timeout)
	ctx = withHTTPRetry(ctx, retries)
	source, err := auth.ResolveAuthSource(projectRoot, authPath)
	if err != nil {
		return nil, err
	}
	if source.Kind == auth.SourceKindAuthFile {
		return newScriptClientWithCacheAuthFn(ctx, "", source.Path)
	}
	return newScriptClientWithCacheAuthFn(ctx, source.Path, "")
}

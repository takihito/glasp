package main

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
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

// applyHTTPRetry wraps httpClient with go-retryablehttp when ctx carries a
// positive retry count. No-op otherwise. The oauth2 Transport already set on
// httpClient is preserved as the inner transport so authentication continues
// to work on each retry attempt.
func applyHTTPRetry(ctx context.Context, httpClient *http.Client) {
	n := httpRetryFromCtx(ctx)
	if n <= 0 {
		return
	}
	// Copy the original client so the retryablehttp inner client retains the
	// oauth2 Transport. Without this copy, rc.HTTPClient and httpClient point
	// to the same struct; the subsequent *httpClient = *rc.StandardClient()
	// overwrites that struct with a retryablehttp Transport, causing infinite
	// recursion when the retry loop calls rc.HTTPClient.Do().
	inner := *httpClient
	rc := retryablehttp.NewClient()
	rc.HTTPClient = &inner
	rc.RetryMax = n
	rc.RetryWaitMin = 500 * time.Millisecond
	rc.RetryWaitMax = 30 * time.Second
	rc.Logger = nil // suppress go-retryablehttp's default log output
	// DefaultRetryPolicy retries 429, 5xx, and network errors; skips other 4xx.
	*httpClient = *rc.StandardClient()
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
